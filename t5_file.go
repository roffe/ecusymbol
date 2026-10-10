package symbol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

const (
	LengthT52 = 0x20000
	LengthT55 = 0x40000
)

var T5MagicBytes = []byte{0xFF, 0xFF, 0xF7, 0xFC, 0x00}

type T5File struct {
	data                      []byte
	numberOfSymbols           int
	m_symboltablestartaddress int
	printFunc                 func(string)
	softwareVersion           string
	autoCorrect               bool
	*Collection
}

type T5FileOpt func(*T5File) error

func WithT5PrintFunc(f func(string)) T5FileOpt {
	return func(t5 *T5File) error {
		t5.printFunc = f
		return nil
	}
}

// WithT5AutoCorrectChecksum loads a file whose stored checksum is wrong and
// recalculates it, the way T5Suite opens such a file and repairs it on save.
func WithT5AutoCorrectChecksum() T5FileOpt {
	return func(t5 *T5File) error {
		t5.autoCorrect = true
		return nil
	}
}

func NewT5File(data []byte, opts ...T5FileOpt) (*T5File, error) {
	if len(data) != LengthT52 && len(data) != LengthT55 {
		return nil, ErrInvalidLength
	}

	//if !bytes.HasPrefix(data, T5MagicBytes) {
	//	return nil, ErrMagicBytesNotFound
	//}

	t5 := &T5File{
		data:       data,
		Collection: NewCollection(),
		printFunc: func(s string) {
			log.Println(s)
		},
	}

	for _, opt := range opts {
		if err := opt(t5); err != nil {
			return nil, err
		}
	}

	return t5.init()
}

func (t5 *T5File) init() (*T5File, error) {
	if err := t5.parseData(); err != nil {
		return nil, err
	}
	t5.softwareVersion = t5.findSoftwareVersion()
	err := t5.VerifyChecksum()
	if errors.Is(err, ErrChecksumMismatch) && t5.autoCorrect {
		return t5, t5.UpdateChecksum()
	}
	return t5, err
}

// findSoftwareVersion scans the binary for a software ID matching the
// "XXXXXXXX.NNA$" pattern (8 chars, dot, two digits, alpha, dollar sign)
// and returns the 12 characters preceding the '$'. The byte immediately
// after '$' encodes lock state (0x01 = locked, 0x00 = unlocked).
func (t5 *T5File) findSoftwareVersion() string {
	if i := t5.findSoftwareVersionEnd(); i > 0 {
		return string(t5.data[i-12 : i])
	}
	return ""
}

// findSoftwareVersionEnd returns the index of the '$' terminating the
// software ID, or -1.
func (t5 *T5File) findSoftwareVersionEnd() int {
	const startOffset = 0x1b00 // was 0x2000 for T5.5, 0x1b00 supports T5.2
	n := len(t5.data)
	if n <= startOffset+12 {
		return -1
	}
	for i := startOffset; i < n; i++ {
		if t5.data[i] != '$' || i < 12 {
			continue
		}
		if t5.data[i-4] == '.' &&
			isT5Numeric(t5.data[i-3]) &&
			isT5Numeric(t5.data[i-2]) &&
			isT5Alpha(t5.data[i-1]) {
			return i
		}
	}
	return -1
}

// isT5Alpha matches T5Suite's IsAlpha, which accepts digits as well.
func isT5Alpha(b byte) bool {
	return isT5Numeric(b) || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isT5Numeric(b byte) bool {
	return b >= '0' && b <= '9'
}

func (t5 *T5File) calculateChecksum() (uint32, error) {
	indexOfFirstMarking, err := t5.getIndexOfFirstMarking()
	if err != nil {
		return 0, err
	}
	if indexOfFirstMarking <= 0 {
		return 0, ErrEndOfSymbolTableNotFound
	}
	end := indexOfFirstMarking + 4
	if end > len(t5.data) {
		return 0, fmt.Errorf("end marker %X outside file: %w", end, ErrOffsetOutOfRange)
	}
	var checksum uint32
	for _, b := range t5.data[:end] {
		checksum += uint32(b)
	}
	return checksum, nil
}

func (t5 *T5File) getIndexOfFirstMarking() (int, error) {
	indexOfFirstMarking, err := readEndMarker(t5.data, 0xFE)
	if err != nil {
		return -1, err
	}
	indexOfFirstMarking -= 3 // Adjust for the 3 bytes of the end marker

	if indexOfFirstMarking <= 0 {
		return -1, ErrEndOfSymbolTableNotFound
	}
	return indexOfFirstMarking, nil
}

func (t5 *T5File) UpdateChecksum() error {
	dataLength := len(t5.data)
	if dataLength < 5 { // Minimum length: end marker (1 byte), checksum (4 bytes)
		return ErrInvalidLength
	}

	checksum, err := t5.calculateChecksum()
	if err != nil {
		return err
	}

	binary.BigEndian.PutUint32(t5.data[dataLength-4:], checksum)
	t5.printFunc(fmt.Sprintf("Checksum updated to %08X", checksum))
	return nil
}

// Read the stored checksum from the last 4 bytes
func (t5 *T5File) getChecksum() uint32 {
	dataLength := len(t5.data)
	return binary.BigEndian.Uint32(t5.data[dataLength-4:])
}

func (t5 *T5File) VerifyChecksum() error {
	dataLength := len(t5.data)
	if dataLength < 5 { // Minimum length: end marker (1 byte), checksum (4 bytes)
		return ErrInvalidLength
	}
	checksum, err := t5.calculateChecksum()
	if err != nil {
		return err
	}
	storedChecksum := t5.getChecksum()
	if checksum != storedChecksum {
		t5.printFunc(fmt.Sprintf("checksum: %X storedChecksum: %X\n", checksum, storedChecksum))
		return ErrChecksumMismatch
	}
	t5.printFunc(fmt.Sprintf("Checksum %X OK", checksum))
	return nil
}

// writeBack copies edited symbols into the binary and refreshes the checksum.
// Several symbols can share one flash range (Adapt_ref, Adapt_korr and
// Insp_mat! for example), so only symbols whose cached data differs from the
// binary are written, and every cache is then re-read so the aliases of an
// edited symbol carry the new bytes instead of restoring the old ones on the
// next save.
func (t5 *T5File) writeBack() error {
	var edited []*Symbol
	for _, sym := range t5.Symbols() {
		if sym.Address == 0 || sym.data == nil {
			continue
		}
		end := int(sym.Address) + len(sym.data)
		if end > len(t5.data) {
			return fmt.Errorf("%s @%X+%d: %w", sym.Name, sym.Address, len(sym.data), ErrAddressOutOfRange)
		}
		if !bytes.Equal(sym.data, t5.data[sym.Address:end]) {
			edited = append(edited, sym)
		}
	}
	for _, sym := range edited {
		copy(t5.data[sym.Address:], sym.data)
	}
	for _, sym := range t5.Symbols() {
		if sym.Address != 0 && sym.data != nil {
			copy(sym.data, t5.data[sym.Address:])
		}
	}
	return t5.UpdateChecksum()
}

func (t5 *T5File) Save(filename string) error {
	t5.printFunc(fmt.Sprintf("Saving T5 file to %s", filename))
	if err := t5.writeBack(); err != nil {
		return err
	}
	return os.WriteFile(filename, t5.data, 0o644)
}

// Bytes returns the binary as loaded, without writing back edited symbols.
func (t5 *T5File) Bytes() []byte {
	return t5.data
}

func (t5 *T5File) Byte() ([]byte, error) {
	if err := t5.writeBack(); err != nil {
		return nil, err
	}
	return t5.data, nil
}

func (t5 *T5File) Version() string {
	return t5.softwareVersion
}

func readEndMarker(data []byte, marker byte) (int, error) {
	if len(data) == 0 {
		return 0, errors.New("data slice is empty")
	}

	const dataSize = 0x40000
	dataLength := len(data)
	searchStart := dataLength - 0x100 // Start of the search area
	if searchStart < 0 {
		searchStart = 0 // Ensure searchStart is not negative
	}

	// Search for the marker
	offset := -1
	for i, b := range data[searchStart:] {
		if b == byte(marker) {
			offset = searchStart + i
			//			log.Printf("offset: %X", offset)
			break
		}
	}

	if offset == -1 {
		return 0, errors.New("marker not found")
	}

	// Calculate the result based on bytes preceding the marker, if enough bytes are available
	if offset > 6 {
		hexStr := "00"
		for i := 1; i <= 6; i++ {
			hexStr += string(data[offset-i])
		}
		result, err := hex.DecodeString(hexStr)
		if err != nil {
			return 0, fmt.Errorf("error decoding hex string: %w", err)
		}
		retval := int(int32(binary.BigEndian.Uint32(result)))
		// Adjust retval based on the data size
		if dataLength == dataSize {
			retval -= dataSize
		} else {
			retval -= 0x60000
		}
		return retval, nil
	}
	return 0, errors.New("not enough data before marker for conversion")
}

func (t5 *T5File) parseData() error {
	var symbols []*Symbol

	var state int = -10
	var charcount int
	buff := bytes.NewBuffer(nil)
	// addRecord turns the buffered record into a symbol. T5Suite drops a
	// record it cannot decode and keeps parsing, so do the same.
	addRecord := func(t int) {
		address := t - charcount
		if t5.m_symboltablestartaddress == 0 && t > 0xA000 {
			t5.m_symboltablestartaddress = address
		}
		if sym, err := t5.tosymbol(buff.Bytes()); err != nil {
			t5.printFunc(fmt.Sprintf("skipping symbol record at %X: %v", address, err))
		} else {
			t5.numberOfSymbols++
			symbols = append(symbols, sym)
		}
		buff.Reset()
	}
outer:
	for t, b := range t5.data {
		switch state {
		case -10:
			if b == 0 {
				state++
			}
		case -9:
			if b == 0x0a {
				state++
			} else {
				state = -10
			}
		case -8:
			if b == 0x28 {
				state++
			} else {
				state = -10
			}
		case -7:
			if b == 0x79 {
				state++
			} else {
				state = -10
			}
		case -6:
			if b == 0x00 {
				state++
			} else {
				state = -10
			}
		case -5:
			if b == 0x4E {
				state++
			}
		case -4:
			if b == 0x75 {
				state = 2
			} else {
				state = -5
			}
		case 0:
			if b == 0x0d {
				state++
			}
		case 1:
			if b == 0x0a {
				state++
				charcount = 0
			} else {
				state = 0
			}
		case 2:
			if charcount < 32 {
				if b == 0x0d && t+1 < len(t5.data) && t5.data[t+1] == 0x0a { // start of next symbol
					state = 1
					addRecord(t)
				} else {
					buff.WriteByte(b)
					charcount++
				}
			} else {
				if bytes.HasPrefix(buff.Bytes(), []byte("END$")) {
					break outer
				}
				addRecord(t)
				state = 0
			}
		}
	}
	buff.Reset()

	alh, err := t5.readAddressLookupTable(len(symbols))
	if err != nil {
		return err
	}
	//f, err := os.Create("symbols.txt")
	//if err != nil {
	//	log.Println(err)
	//}
	//defer f.Close()
	for _, sym := range symbols {
		if alt, ok := alh[sym.SramOffset]; ok {
			sym.Address = alt.FlashAddress
			alt.Used = true
			alh[sym.SramOffset] = alt
		}
		if sym.Address == 0 {
			// log.Println(sym.Name, "has no address")
			continue
		}
		end := int(sym.Address) + int(sym.Length)
		if end > len(t5.data) {
			t5.printFunc(fmt.Sprintf("%s: flash range %X+%d outside file, treating as RAM only", sym.Name, sym.Address, sym.Length))
			sym.Address = 0
			continue
		}
		sym.data = bytes.Clone(t5.data[sym.Address:end])
	}

	for _, v := range alh {
		if !v.Used {
			log.Printf("Unused address: %X", v.FlashAddress)
		}
	}

	t5.Add(symbols...)

	t5.printFunc(fmt.Sprintf("Loaded %d symbols from binary", len(symbols)))

	return nil
}

type addressRecord struct {
	FlashAddress uint32
	Used         bool
}

func (t5 *T5File) readAddressLookupTable(numberOfSymbols int) (map[uint32]addressRecord, error) {
	var readstate int = -30
	var lookuptablestartaddress int
	for t, b := range t5.data {
		switch readstate {
		case -30:
			if b == 0x4E {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -29:
			if b == 0x75 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -28:
			if b == 0x48 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -27:
			if b == 0xE7 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -26:
			if b == 0x01 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -25:
			if b == 0x30 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -24:
			if b == 0x26 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -23:
			if b == 0x6F {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -22:
			if b == 0x00 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -21:
			if b == 0x16 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -20:
			if b == 0x3E {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -19:
			if b == 0x2F {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -18:
			if b == 0x00 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -17:
			if b == 0x14 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -16:
			if b == 0x24 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -15:
			if b == 0x6F {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -14:
			if b == 0x00 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -13:
			if b == 0x10 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -12:
			if b == 0x60 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -11:
			if b == 0x00 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -10:
			if b == 0x00 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -9:
			if b == 0x0A {
				readstate = 0
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}

		case 0:
			// waiting for first recognition char 48
			if b == 0x48 {
				lookuptablestartaddress = t
				readstate++
			}
		case 1:
			// waiting for second char 79
			if b == 0x79 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = 0
			}
		case 2:
			// waiting for third char 00
			if b == 0x00 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = 0
			}
		case 3:
			// waiting for last char 04
			if len(t5.data) == 0x20000 {
				if b == 0x06 {
					readstate++
				} else {
					lookuptablestartaddress = 0x00
					readstate = 0
				}
			} else {
				if b == 0x04 {
					readstate++
				} else {
					lookuptablestartaddress = 0x00
					readstate = 0
				}
			}
		}
	}
	binPos := lookuptablestartaddress + 2
	br := bytes.NewReader(t5.data)
	if _, err := br.Seek(int64(binPos), io.SeekStart); err != nil {
		return nil, err
	}

	addressRecords := make(map[uint32]addressRecord)
	for sc := 0; sc < numberOfSymbols; sc++ {
		if binPos >= t5.m_symboltablestartaddress {
			log.Println("pos is greater than symboltablestartaddress")
			break
		}

		var flashAddress uint32
		if err := binary.Read(br, binary.BigEndian, &flashAddress); err != nil {
			return nil, err
		}
		binPos += 4

		// log.Printf("flashAddress: 0x%08X\n", flashAddress)

		// 8x dummy bytes
		dummy := make([]byte, 8)
		if err := binary.Read(br, binary.BigEndian, dummy); err != nil {
			return nil, err
		}
		//		log.Printf("dummy: %X len: %X other: %X", dummy[:2], dummy[2:4], dummy[4:])
		binPos += 8

		var sramAddress uint16
		if err := binary.Read(br, binary.BigEndian, &sramAddress); err != nil {
			return nil, err
		}
		binPos += 2

		// Flash is mapped at 0x40000-0x7FFFF; a T5.2 image fills only the
		// upper half, so reduce modulo the image size like T5Suite does.
		// Entries outside the window are garbage and are dropped.
		switch _, dup := addressRecords[uint32(sramAddress)]; {
		case flashAddress < 0x40000 || flashAddress >= 0x80000:
			log.Printf("lookup entry %X for SRAM %04X outside flash, ignoring", flashAddress, sramAddress)
		case dup: // T5Suite keeps the first entry for an SRAM address
		default:
			addressRecords[uint32(sramAddress)] = addressRecord{
				FlashAddress: flashAddress % uint32(len(t5.data)),
			}
		}

		// Check if there is a nother symbol in the next 16 bytes
		tel := 0
		found := false
		tstate := 0
		for tel < 16 && !found {
			tb, err := br.ReadByte()
			if err != nil {
				return nil, err
			}
			binPos++
			switch tstate {
			case 0:
				if tb == 0x48 {
					tstate++
				}
			case 1:
				if tb == 0x79 {
					found = true
				} else {
					tstate = 0
				}
			}
			tel++
		}
		if !found {
			break
		}

	}
	return addressRecords, nil
}

func (t5 *T5File) tosymbol(data []byte) (*Symbol, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("record too short (%d bytes)", len(data))
	}
	var invalidCharCount int
	for _, b := range data[4:] {
		if b < 10 {
			invalidCharCount++
		}
	}
	if invalidCharCount > 2 {
		return nil, errors.New("too many invalid chars")
	}

	name := CString(data[4:])
	if name == "" {
		return nil, errors.New("empty name")
	}
	length := binary.BigEndian.Uint16(data[2:4])
	typ := T5Types[name]
	factor := GetCorrectionfactor(name)
	switch {
	case strings.HasPrefix(name, "Knock_count_cyl"):
		// Some firmwares store 2/4/6/8 for cyl1..4; T5Suite forces 2 (ParseTrionicFile).
		length = 2
	case strings.HasPrefix(name, "Reg_kon_mat") && length != 0x80:
		// 0x80 bytes is the 8-bit throttle x rpm matrix in %. Later firmware
		// stores a 31-row 16-bit rpm table in 0.1 % (T5Suite isSixteenBitTable,
		// GetMapCorrectionFactor).
		typ &^= CHAR
		factor = 0.1
	}

	return &Symbol{
		Number:           t5.numberOfSymbols,
		SramOffset:       uint32(binary.BigEndian.Uint16(data[0:2])),
		Length:           length,
		Name:             name,
		Type:             typ,
		Correctionfactor: factor,
	}, nil
}

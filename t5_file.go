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
	printFunc                 func(string, ...any)
	*Collection
}

type T5FileOpt func(*T5File) error

func WithT5PrintFunc(f func(string, ...any)) T5FileOpt {
	return func(t5 *T5File) error {
		t5.printFunc = f
		return nil
	}
}

func NewT5File(data []byte, opts ...T5FileOpt) (*T5File, error) {
	if len(data) != LengthT55 {
		return nil, ErrInvalidLength
	}

	//if !bytes.HasPrefix(data, T5MagicBytes) {
	//	return nil, ErrMagicBytesNotFound
	//}

	t5 := &T5File{
		data:       data,
		Collection: NewCollection(),
		printFunc:  log.Printf,
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
	return t5, t5.VerifyChecksum()
}

func (t5 *T5File) calculateChecksum() (uint32, error) {
	indexOfFirstMarking, err := t5.getIndexOfFirstMarking()
	if err != nil {
		return 0, err
	}
	if indexOfFirstMarking <= 0 {
		return 0, ErrEndOfSymbolTableNotFound
	}
	var checksum uint32
	for i := 0; i < indexOfFirstMarking+4; i++ {
		checksum += uint32(t5.data[i])
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
	log.Printf("Checksum updated to %X", t5.data[dataLength-4:])
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
		t5.printFunc("checksum: %X storedChecksum: %X\n", checksum, storedChecksum)
		return ErrChecksumMismatch
	}
	t5.printFunc("Checksum %X OK", checksum)
	return nil
}

func (t5 *T5File) Save(filename string) error {
	if err := t5.UpdateChecksum(); err != nil {
		return err
	}
	return os.WriteFile(filename, t5.data, 0644)
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
				if b == 0x0d && t5.data[t+1] == 0x0a { // start of next symbol
					state = 1
					address := t - charcount
					if t5.m_symboltablestartaddress == 0 && t > 0xA000 {
						t5.m_symboltablestartaddress = address
					}
					sym, err := t5.tosymbol(buff.Bytes())
					if err != nil {
						return err
					}
					t5.numberOfSymbols++
					symbols = append(symbols, sym)

					buff.Reset()
				} else {
					if err := buff.WriteByte(b); err != nil {
						return err
					}
					charcount++
				}
			} else {
				address := t - charcount
				if t5.m_symboltablestartaddress == 0 && t > 0xA000 {
					t5.m_symboltablestartaddress = address
				}
				if bytes.HasPrefix(buff.Bytes(), []byte("END$")) {
					//					log.Println("END$ found")
					break outer
				} else {
					sym, err := t5.tosymbol(buff.Bytes())
					if err != nil {
						return err
					}
					t5.numberOfSymbols++
					symbols = append(symbols, sym)
					buff.Reset()
					state = 0
				}
			}
		}
	}
	buff.Reset()

	alh, err := t5.readAddressLookupTable(len(symbols))
	if err != nil {
		return err
	}
	for _, sym := range symbols {
		if alt, ok := alh[sym.SramOffset]; ok {
			sym.Address = alt.FlashAddress
			alt.Used = true
			alh[sym.SramOffset] = alt
		}
		if sym.Address == 0 {
			//log.Println(sym.Name, "has no address")
			continue
		}
		//log.Printf("SRAM: %X, ADDR: %X, LEN: %d N: %s", sym.SramOffset, sym.Address, sym.Length, sym.Name)
		sym.data = make([]byte, sym.Length)
		if err := binary.Read(bytes.NewReader(t5.data[sym.Address:sym.Address+uint32(sym.Length)]), binary.BigEndian, sym.data); err != nil {
			return err
		}
	}

	for _, v := range alh {
		if !v.Used {
			log.Printf("Unused address: %X", v.FlashAddress)
		}
	}

	t5.Add(symbols...)

	t5.printFunc("Loaded %d symbols from binary", len(symbols))

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

		//log.Printf("flashAddress: 0x%08X\n", flashAddress)

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

		addressRecords[uint32(sramAddress)] = addressRecord{
			FlashAddress: flashAddress - uint32(len(t5.data)),
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
	var invalidCharCount int
	for _, b := range data[4:] {
		if b < 10 {
			invalidCharCount++
		}
	}
	if invalidCharCount > 2 {
		log.Println("Too many invalid chars")
		return nil, fmt.Errorf("too many invalid chars")
	}

	name := CString(data[4:])

	//	log.Println(sym.String())
	return &Symbol{
		Number:           t5.numberOfSymbols,
		SramOffset:       uint32(binary.BigEndian.Uint16(data[0:2])),
		Length:           binary.BigEndian.Uint16(data[2:4]),
		Name:             name,
		Type:             T5Types[name],
		Correctionfactor: GetCorrectionfactor(name),
	}, nil
}

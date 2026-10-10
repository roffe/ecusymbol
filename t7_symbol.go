package symbol

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/roffe/ecusymbol/kmp"
)

func extractUint32(data []byte, start int) uint32 {
	return uint32(data[start])<<24 | uint32(data[start+1])<<16 | uint32(data[start+2])<<8 | uint32(data[start+3])
}

func extractUint16(data []byte, start int) uint16 {
	return uint16(data[start])<<8 | uint16(data[start+1])
}

func NewSymbolFromT7Bytes(data []byte, symbol_number int) *Symbol {
	internall_address := extractUint32(data, 0)

	symbol_length := uint16(0x08)
	if symbol_number != 0 {
		symbol_length = extractUint16(data, 4)
	}

	symbol_mask := extractUint16(data, 6)

	symbol_type := data[8]

	return &Symbol{
		Name:             "Symbol-" + strconv.Itoa(symbol_number),
		Number:           symbol_number,
		Address:          internall_address,
		Length:           symbol_length,
		Mask:             symbol_mask,
		Type:             symbol_type,
		Correctionfactor: 1,
	}
}

// t7ForceUnsigned holds symbols whose type byte in the T7 symbol table claims
// SIGNED but that the ECU actually reads as u16 (VIOS Qair_i.c declares
// "u16 Q_AirInletTab[18]" and the stock calibration tops out at 34000).
// Without this the top table entries show up negative.
// ponytail: plain name list, extend it if another table turns up negative.
var t7ForceUnsigned = map[string]bool{
	"VIOSMAFCal.Q_AirInletTab":  true,
	"VIOSMAFCal.Q_AirInletTab2": true,
}

// fixT7SymbolType clears bogus signedness flags, call after Name is set.
func fixT7SymbolType(s *Symbol) {
	if t7ForceUnsigned[s.Name] {
		s.Type &^= SIGNED
	}
}

func loadT7Symbols(data []byte, cb func(string)) (*Collection, error) {
	//for _, h := range GetAllT7HeaderFields(data) {
	//	switch h.ID {
	//	case 0x91, 0x94, 0x95, 0x97:
	//		cb(h.PrettyString())
	//	}
	//}

	if !isBinaryPackedVersion(data) {
		// return nil, errors.New("non binarypacked not implemented, send your bin to Roffe")
		// log.Println("Not a binarypacked version")
		cb("Not a binarypacked symbol table")
		return nonBinaryPacked(data, cb)

	} else {
		// log.Println("Binary packed version")
		cb("Found binary packed symbol table")
		return binaryPacked(data, cb)

	}
	// return nil, errors.New("not implemented")
}

func nonBinaryPacked(data []byte, cb func(string)) (*Collection, error) {
	symbolListOffset, err := getSymbolListOffSet(data) // 0x15FA in 5168646.BIN
	if err != nil {
		return nil, err
	}
	cb(fmt.Sprintf("Symbol list offset: %X", symbolListOffset))
	var symbolName strings.Builder
	var symbolCount int
	var symbolNames []string
	var symbolInternalPositions []int

outer:
	for pos := symbolListOffset; pos < len(data); pos++ {
		switch data[pos] {
		case 0xFF: // 0xFF used to keep the start of each string 'word' aligned
			continue
		case 0x02:
			break outer
		case 0x00: // 0x00 end of Symbol name string
			symbolNames = append(symbolNames, symbolName.String())
			symbolInternalPositions = append(symbolInternalPositions, pos-len(symbolName.String()))
			symbolName.Reset()
			symbolCount++
		default:
			symbolName.WriteByte(data[pos])
		}
	}

	cb(fmt.Sprintln("Symbols found:", symbolCount))

	searchPattern := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF0}
	searchPattern[12] = byte(symbolInternalPositions[0] >> 8)
	searchPattern[13] = byte(symbolInternalPositions[0])

	// log.Printf("Search pattern: %X", searchPattern)

	addressTableOffset := kmp.BytePatternSearch(data, searchPattern, 0)
	cb(fmt.Sprintf("Address table offset: %X", addressTableOffset))

	if addressTableOffset == -1 {
		return nil, ErrAddressTableOffsetNotFound
	}

	var symb_count int

	symCol := NewCollection()

	for pos := addressTableOffset; pos < len(data); pos += 14 {
		if symb_count >= symbolCount {
			break
		}
		buff := data[pos : pos+14]
		sram_address := binary.BigEndian.Uint32(buff[0:4])
		symbol_length := binary.BigEndian.Uint16(buff[4:6])
		// internal_address := binary.BigEndian.Uint32(buff[10:14])
		sym_type := buff[8]

		// RAM variables keep their RAM address and get no flash data, see
		// readAllT7SymbolsData.
		sym := &Symbol{
			Name:             strings.TrimSpace(symbolNames[symb_count]),
			Number:           symb_count,
			Address:          sram_address,
			Length:           symbol_length,
			Mask:             binary.BigEndian.Uint16(buff[6:8]),
			Type:             sym_type,
			Correctionfactor: GetCorrectionfactor(strings.TrimSpace(symbolNames[symb_count])),
			Unit:             GetUnit(strings.TrimSpace(symbolNames[symb_count])),
		}
		fixT7SymbolType(sym)

		// if sym.Name == "BFuelCal.E85Map" {
		// 	log.Println(sym.String())
		// }

		// if sym.Name == "BFuelCal.Map" {
		// 	log.Printf("all %X", buff)
		// 	log.Printf("sram address: %X", sram_address)
		// 	log.Printf("symbol length: %X", symbol_length)
		// 	log.Printf("internal address: %X", internal_address)
		// 	log.Printf("rest %X", buff[6:10])
		// 	log.Printf("real rom address: %X", real_rom_address)
		// 	log.Println(sym.String())
		// }

		symCol.Add(sym)
		symb_count++
	}
	/*
		for _, sym := range symCol.Symbols() {
			if sym.Address < T7SRAMAddress {
				sym.data, err = readSymbolData(data, sym, 0)
				if err != nil {
					return nil, err
				}
			}
		}
	*/
	//log.Println("Symbols found: ", symb_count)
	cb(fmt.Sprintf("Loaded %d symbols from binary", symb_count))

	if err := readAllT7SymbolsData(data, symCol.Symbols()); err != nil {
		return nil, err
	}
	return symCol, nil
}

// repairZeroedAddresses rebuilds the addresses tuners (MapTun) zero in the
// address table to hide maps. Struct members are contiguous, so a zeroed entry
// is prev.Address+prev.Length — the repair T7Suite does in
// Trionic7File.tryToDecodePackedBinary. That fails for a struct's first member,
// whose prev is the zero-length XxxCalType entry (or another zeroed member), so
// what the forward pass can't resolve is derived backwards from the next member.
// ponytail: a zeroed single-member struct has neither neighbour and stays 0.
func repairZeroedAddresses(symbols []*Symbol) {
	broken := make([]bool, len(symbols))
	for i, s := range symbols {
		broken[i] = s.Address == 0 && s.Length > 0
	}
	for i := 1; i < len(symbols); i++ {
		prev, s := symbols[i-1], symbols[i]
		if !broken[i] || broken[i-1] || prev.Length == 0 {
			continue
		}
		s.Address = prev.Address + uint32(prev.Length)
		if s.Length == 0x240 && s.Address%2 != 0 {
			s.Address++
		}
		broken[i] = false
	}
	for i := len(symbols) - 2; i > 0; i-- {
		next, s := symbols[i+1], symbols[i]
		if !broken[i] || broken[i+1] || next.Length == 0 || next.Address < uint32(s.Length) {
			continue
		}
		s.Address = next.Address - uint32(s.Length)
		if symbols[i-1].Length == 0 && s.Address%2 != 0 { // first member: structs are word aligned
			s.Address--
		}
		broken[i] = false
	}
}

func binaryPacked(data []byte, cb func(string)) (*Collection, error) {
	compressed, addressTableOffset, symbolNameTableOffset, symbolTableLength, err := GetT7Offsets(data, cb)
	if err != nil && !errors.Is(err, ErrSymbolTableNotFound) {
		return nil, err
	}
	// os.WriteFile("compressedSymbolNameTable.bin", data[symbolNameTableOffset:symbolNameTableOffset+symbolTableLength, 0644)

	if addressTableOffset == -1 {
		return nil, ErrAddressTableOffsetNotFound
	}

	var symb_count int
	var symbols []*Symbol

	// os.WriteFile("adresstable.bin", data[addressTableOffset:], 0644)

	// parse addresstable and create symbols with generic names
	for pos := addressTableOffset; pos < len(data)+10; pos += 10 {
		if data[pos] == 0x53 && data[pos+1] == 0x43 { // SC
			break
		}
		sym := NewSymbolFromT7Bytes(data[pos:pos+10], symb_count)
		symbols = append(symbols, sym)
		symb_count++
	}
	repairZeroedAddresses(symbols)
	// log.Println("Symbols found: ", symb_count)
	cb(fmt.Sprintf("Loaded %d symbols from binary", symb_count))

	if compressed {
		if bytes.HasPrefix(data[symbolNameTableOffset:symbolNameTableOffset+symbolTableLength], []byte{0xFF, 0xFF, 0xFF, 0xFF}) {
			return nil, errors.New("compressed symbol table is not present")
		}
		symbolNames, err := ExpandCompressedSymbolNames(data[symbolNameTableOffset : symbolNameTableOffset+symbolTableLength])
		if err != nil {
			return nil, err
		}

		for i := 0; i < len(symbolNames)-1; i++ {
			if i >= len(symbols) {
				cb("Missing symbol in name table " + strconv.Itoa(i))
				continue
			}
			symbols[i].Name = strings.TrimSpace(symbolNames[i])
			symbols[i].Unit = GetUnit(symbols[i].Name)
			symbols[i].Correctionfactor = GetCorrectionfactor(symbols[i].Name)
			fixT7SymbolType(symbols[i])
		}
		if err := readAllT7SymbolsData(data, symbols); err != nil {
			return NewCollection(symbols...), err
		}
		return NewCollection(symbols...), nil
	}

	if symbolTableLength < 0x100 {
		ver, err := determineVersion(data)
		if err != nil {
			if errors.Is(err, ErrVersionNotFound) {
				cb("Could not determine binary version")
				cb("Load symbols from XML")
			} else {
				return nil, fmt.Errorf("could not determine version: %v", err)
			}
		}

		log.Println("Detected version:", ver)

		nameMap, err := xml2map(ver)
		if err != nil {
			return nil, err
		}
		for i, s := range symbols {
			if value, ok := nameMap[s.Number]; ok {
				symbols[i].Name = value
				symbols[i].Unit = GetUnit(s.Name)
				symbols[i].Correctionfactor = GetCorrectionfactor(symbols[i].Name)
				fixT7SymbolType(symbols[i])
			}
		}
	}

	if err := readAllT7SymbolsData(data, symbols); err != nil {
		return nil, err
	}

	return NewCollection(symbols...), nil
}

// t7CalDelta returns the RAM->ROM offset of the calibration sections
// (.STARTOF.(caldata)-.STARTOF.(calconst), see symhead2.h in the T7 source).
// Header field 0x9C carries it; the 4 bytes after the "sYMBOLtABLE\0" marker
// are the same value and serve as fallback.
func t7CalDelta(fileBytes []byte) (uint32, error) {
	if v, err := GetT7HeaderField(fileBytes, 0x9C); err == nil && len(v) == 4 {
		return reverseInt(binary.BigEndian.Uint32(v)), nil
	}
	if m := bytes.Index(fileBytes, []byte("sYMBOLtABLE\x00")); m >= 0 && m+16 <= len(fileBytes) {
		return binary.BigEndian.Uint32(fileBytes[m+12:]), nil
	}
	return 0, errors.New("t7: calibration RAM/ROM delta not found")
}

// t7CalWindow is one calconst->caldata copy the firmware makes at start-up:
// RAM [ram, ram+size) is initialised from flash [flash, flash+size).
type t7CalWindow struct {
	ram, flash, size uint32
}

// t7CalWindows finds the start-up copies in the code. Initdata.c loads the
// three section constants with "move.l #imm,d0" (20 3C imm32) one after the
// other: calconst start, caldata start, calconst size. Only copies whose
// RAM-flash distance is the linker delta from the footer count, the same
// instruction shape occurs in other copy loops.
func t7CalWindows(data []byte, delta uint32) []t7CalWindow {
	type mv struct {
		pos int
		imm uint32
	}
	var mvs []mv
	for i := 0; i+6 <= len(data); i += 2 {
		if data[i] == 0x20 && data[i+1] == 0x3C {
			mvs = append(mvs, mv{i, binary.BigEndian.Uint32(data[i+2:])})
		}
	}
	var out []t7CalWindow
	for i, a := range mvs {
		if a.imm >= uint32(len(data)) {
			continue
		}
		for j := i + 1; j < len(mvs) && mvs[j].pos-a.pos <= 64; j++ {
			if mvs[j].imm != a.imm+delta {
				continue
			}
			for k := j + 1; k < len(mvs) && mvs[k].pos-mvs[j].pos <= 64; k++ {
				if size := mvs[k].imm; size > 0 && a.imm+size <= uint32(len(data)) {
					out = append(out, t7CalWindow{mvs[j].imm, a.imm, size})
				}
				break
			}
			break
		}
	}
	return out
}

// readAllT7SymbolsData points every symbol at its bytes in the binary. ROM
// symbols sit at their address. A RAM symbol has a flash image only when it
// lies in a calibration window (then SramOffset is the RAM-flash distance,
// which Save uses); other RAM variables are live-only and get no data. Symbol
// 0 (SymbolNames / SymbolTableStart) is the symbol table itself and is left
// alone so a writeback can never destroy the table.
func readAllT7SymbolsData(fileBytes []byte, symbols []*Symbol) error {
	var windows []t7CalWindow
	if delta, err := t7CalDelta(fileBytes); err == nil {
		windows = t7CalWindows(fileBytes, delta)
	}
	for _, sym := range symbols {
		if sym.Number == 0 {
			continue
		}
		off := sym.Address
		if sym.Address >= T7SRAMAddress {
			var w *t7CalWindow
			for i := range windows {
				if sym.Address >= windows[i].ram && sym.Address+uint32(sym.Length) <= windows[i].ram+windows[i].size {
					w = &windows[i]
					break
				}
			}
			if w == nil {
				continue
			}
			sym.SramOffset = w.ram - w.flash
			off = sym.Address - sym.SramOffset
		}
		if end := uint64(off) + uint64(sym.Length); end > uint64(len(fileBytes)) {
			log.Printf("symbol data outside file: %s", sym.String())
			continue
		}
		sym.data = fileBytes[off : off+uint32(sym.Length)]
	}
	return nil
}

func reverseInt(v uint32) uint32 {
	return (v >> 24) | ((v >> 8) & 0xFF00) | ((v << 8) & 0xFF0000) | (v << 24)
}

/*
func reverseInt(value uint32) uint32 {
	// input            0x34FCEF00
	// desired output   0x00EFFC34
	var retval uint32
	retval |= (value & 0xFF000000) >> 24
	retval |= ((value & 0x00FF0000) >> 16) << 8
	retval |= ((value & 0x0000FF00) >> 8) << 16
	retval |= (value & 0x000000FF) << 24

	return retval
}
*/

func determineVersion(data []byte) (string, error) {
	switch {
	// The footer stores the version reversed (EU0CF01C -> C10FC0UE), so match both forms.
	case bytes.Contains(data, []byte("EU0CF01O")), bytes.Contains(data, []byte("O10FC0UE")),
		bytes.Contains(data, []byte("EU0BF01O")), bytes.Contains(data, []byte("O10FB0UE")),
		bytes.Contains(data, []byte("EU0AF01O")), bytes.Contains(data, []byte("O10FA0UE")):
		return "EU0CF01O", nil
	// EU06 trace: F01C/Z44C/Z44O share one 4603-entry symbol table (EU0x has 4666).
	case bytes.Contains(data, []byte("EU06F01C")), bytes.Contains(data, []byte("C10F60UE")),
		bytes.Contains(data, []byte("EU06Z44C")), bytes.Contains(data, []byte("C44Z60UE")),
		bytes.Contains(data, []byte("EU06Z44O")), bytes.Contains(data, []byte("O44Z60UE")):
		return "EU06F01C", nil
	// Recovered tables (see EU0xF01C.xml SOURCE/CONFIDENCE fields); one XML per trace.
	case bytes.Contains(data, []byte("EI03F01C")),
		bytes.Contains(data, []byte("C10F30IE")):
		return "EI03F01C", nil
	case bytes.Contains(data, []byte("EU04Z40O")),
		bytes.Contains(data, []byte("O04Z40UE")),
		bytes.Contains(data, []byte("EUO4Z40O")),
		bytes.Contains(data, []byte("O04Z4OUE")):
		return "EU04F01C", nil
	case bytes.Contains(data, []byte("EU03Z36C")),
		bytes.Contains(data, []byte("C63Z30UE")),
		bytes.Contains(data, []byte("EU03Z36O")),
		bytes.Contains(data, []byte("O63Z30UE")):
		return "EU03F01C", nil
	case bytes.Contains(data, []byte("ET07F02C")),
		bytes.Contains(data, []byte("C20F70TE")),
		bytes.Contains(data, []byte("EU02Z30O")),
		bytes.Contains(data, []byte("O03Z20UE")),
		bytes.Contains(data, []byte("EU02Z32O")),
		bytes.Contains(data, []byte("O23Z20UE")):
		return "ET07F02C", nil
	case bytes.Contains(data, []byte("ET06F01C")),
		bytes.Contains(data, []byte("C10F60TE")):
		return "ET06F01C", nil
	case bytes.Contains(data, []byte("EU0AF01C")), bytes.Contains(data, []byte("C10FA0UE")),
		bytes.Contains(data, []byte("EU0BF01C")), bytes.Contains(data, []byte("C10FB0UE")),
		bytes.Contains(data, []byte("EU0CF01C")), bytes.Contains(data, []byte("C10FC0UE")):
		return "EU0AF01C", nil
	// EU09F01O shares an identical symbol table with EU09F01C, so it maps to the same XML.
	// The footer stores the version reversed, hence C10F90UE / O10F90UE / T10F90UE.
	case bytes.Contains(data, []byte("EU09F01C")), bytes.Contains(data, []byte("C10F90UE")),
		bytes.Contains(data, []byte("EU09F01O")), bytes.Contains(data, []byte("O10F90UE")),
		bytes.Contains(data, []byte("EU09F01T")), bytes.Contains(data, []byte("T10F90UE")):
		return "EU09F01C", nil
	}
	return "", ErrVersionNotFound
}

var searchPattern = []byte{0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x00}

// var searchPattern2 = []byte{0x00, 0x08, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
// var searchPattern3 = []byte{0x73, 0x59, 0x4D, 0x42, 0x4F, 0x4C, 0x74, 0x41, 0x42, 0x4C, 0x45, 0x00} // 12
func GetT7Offsets(data []byte, cb func(string)) (bool, int, int, int, error) {
	addressTableOffset := kmp.BytePatternSearch(data, searchPattern, 0x30000)
	if addressTableOffset < 0 {
		return false, -1, 0, 0, ErrAddressTableOffsetNotFound
	}
	addressTableOffset -= 0x06
	cb(fmt.Sprintf("Address table offset: %08X", addressTableOffset))

	if delta, err := t7CalDelta(data); err == nil {
		cb(fmt.Sprintf("Calibration RAM/ROM delta: %08X", delta))
	}

	symbolNameTableOffset := getAddressFromOffset(data, addressTableOffset)
	//	log.Printf("Symbol table offset: %08X", symbolNameTableOffset)
	cb(fmt.Sprintf("Symbol table offset: %08X", symbolNameTableOffset))

	symbolTableLength := getLengthFromOffset(data, addressTableOffset+0x04)
	//	log.Printf("Symbol table length: %08X", symbolTableLength)
	cb(fmt.Sprintf("Symbol table length: %08X", symbolTableLength))

	if symbolTableLength > 0x1000 && symbolNameTableOffset > 0 && symbolNameTableOffset < 0x70000 {
		// compressedSymbolTable := data[symbolNameTableOffset : symbolNameTableOffset+symbolTableLength]
		return true, addressTableOffset, symbolNameTableOffset, symbolTableLength, nil
	}
	return false, addressTableOffset, symbolNameTableOffset, symbolTableLength, ErrSymbolTableNotFound
}

func getLengthFromOffset(data []byte, offset int) int {
	return int(binary.BigEndian.Uint16(data[offset : offset+2]))
}

func getAddressFromOffset(data []byte, offset int) int {
	return int(binary.BigEndian.Uint32(data[offset : offset+4]))
}

func getSymbolListOffSet(data []byte) (int, error) {
	zerocount := 0
	for pos := 0; pos < len(data); pos++ {
		if data[pos] == 0x00 {
			zerocount++
		} else {
			if zerocount < 15 {
				zerocount = 0
			} else {
				return pos, nil
			}
		}
	}
	return -1, errors.New("Symbol list not found")
}

// isBinaryPackedVersion reports whether the footer has a 0x9B field, the
// address of the compressed symbol table.
func isBinaryPackedVersion(data []byte) bool {
	f, err := GetT7HeaderField(data, 0x9B)
	return err == nil && len(f) > 0
}

func GetT7HeaderField(bin []byte, id byte) ([]byte, error) {
	binLength := len(bin)
	var answer []byte
	addr := binLength - 1
	var found bool
	for addr > (binLength - 0x1FF) {
		/* The first byte is the length of the data */
		fieldLength := bin[addr]
		// log.Printf("%3d, %x", lengthField, lengthField)
		if fieldLength == 0x00 || fieldLength == 0xFF {
			break
		}
		addr--

		/* Second byte is an ID field */
		fieldID := bin[addr]
		addr--

		if fieldID == id {
			answer = make([]byte, int(fieldLength))
			answer[fieldLength-1] = 0x00
			// answer[fieldLength] = 0x00
			for i := 0; i < int(fieldLength); i++ {
				answer[i] = bin[addr]
				addr--
			}
			//			log.Printf("0x%02x %d> %q", fieldID, len(answer), string(answer))
			found = true
			// break
			// when this return is commented out, the function will
			// find the last field if there are several (mainly this
			// is for searching for the last VIN field)
			// return 1;
		} else {
			addr -= int(fieldLength)
		}
	}
	if found {
		return answer, nil
	}
	return nil, fmt.Errorf("did not find header for id 0x%02x", id)
}

func GetAllT7HeaderFields(bin []byte) []*T7HeaderField {
	binLength := len(bin)
	addr := binLength - 1

	fields := make([]*T7HeaderField, 0)

	for addr > (binLength - 0x1FF) {
		//		log.Printf("addr: %X", addr)
		/* The first byte is the length of the data */
		fieldLength := bin[addr]
		//		log.Printf("fieldLength %X", fieldLength)
		// log.Printf("%3d, %x", lengthField, lengthField)
		if fieldLength == 0x00 || fieldLength == 0xFF {
			break
		}
		addr--

		fieldID := bin[addr]
		addr--

		fieldData := make([]byte, int(fieldLength))
		fieldData[fieldLength-1] = 0x00

		for i := 0; i < int(fieldLength); i++ {
			fieldData[i] = bin[addr]
			addr--
		}
		fields = append(fields, &T7HeaderField{
			ID:     fieldID,
			Length: fieldLength,
			Data:   fieldData,
		})
		//		log.Printf("0x%02x %d> %q len: %d", fieldID, len(fieldData), string(fieldData), fieldLength)
	}
	return fields
}

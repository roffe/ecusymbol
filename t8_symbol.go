package symbol

import (
	"encoding/binary"
	"fmt"

	"github.com/roffe/ecusymbol/kmp"
)

func loadT8Symbols(fileBytes []byte, cb func(string)) (*Collection, error) {
	//addressTableOffset, err := GetAddrTableOffsetBySymbolTable(fileBytes)
	//if err != nil {
	//	return nil, err
	//}

	addrtaboffset, err := GetEndOfSymbolTable(fileBytes)
	if err != nil {
		return nil, err
	}

	NqNqNqOffset, err := GetFirstNqStringFromOffset(fileBytes, addrtaboffset)
	if err != nil {
		return nil, err
	}

	addressTableOffset := NqNqNqOffset + 21 + 7

	symbtaboffset, err := GetAddressFromOffset(fileBytes, NqNqNqOffset)
	if err != nil {
		return nil, err
	}

	symbtablength, err := GetLengthFromOffset(fileBytes, NqNqNqOffset+4)
	if err != nil {
		return nil, err
	}

	names, err := ExpandCompressedSymbolNames(fileBytes[symbtaboffset : symbtaboffset+symbtablength])
	if err != nil {
		return nil, err
	}

	if err := FindAddressTableOffset(fileBytes); err != nil {
		return nil, err
	}

	symbols, err := ReadAddressTable(fileBytes, addressTableOffset)
	if err != nil {
		return nil, err
	}

	nqCount := CountNq(fileBytes, NqNqNqOffset-2)

	priOffset, err := GetAddressFromOffset(fileBytes, NqNqNqOffset-((nqCount*2)+6))
	if err != nil {
		return nil, err
	}

	secOffset := DetermineSecondaryOffset(fileBytes)
	cb(fmt.Sprintf("Primary Offset: 0x%X", priOffset))
	cb(fmt.Sprintf("Secondary Offset: 0x%X", secOffset))

	for i := range symbols {
		symbols[i].Name = names[i+1]
		symbols[i].Unit = GetUnit(symbols[i].Name)
		symbols[i].Correctionfactor = GetCorrectionfactor(symbols[i].Name)
	}

	syms := NewCollection(symbols...)

	openBin := determineBinaryOpenness(fileBytes, syms)
	if openBin {
		cb("Open binary detected")
	} else {
		cb("Closed binary detected")
	}

	for _, sym := range symbols {
		//origAddress := sym.Address
		var actAddress uint32
		if sym.Address >= 0x100000 {
			if sym.Type != 0xFF && sym.Type&0x22 == 0x02 {
				if openBin {
					// Open binary, offsets are switched up
					if sym.Address+uint32(sym.Length) <= (0x100000+32768) && sym.Address >= uint32(secOffset) {
						// Internal SRAM, use the secondary offset
						sym.SramOffset = uint32(secOffset)
						actAddress = sym.Address - uint32(secOffset)
					} else if sym.Address >= (0x100000+32768) && sym.Address >= uint32(priOffset) {
						// External SRAM, use the primary offset
						sym.SramOffset = uint32(priOffset)
						actAddress = sym.Address - uint32(priOffset)
					}
				} else {
					// Normal binary, use the primary offset
					if sym.Address+uint32(sym.Length) <= (0x100000+32768) && sym.Address >= uint32(priOffset) {
						sym.SramOffset = uint32(priOffset)
						actAddress = sym.Address - uint32(priOffset)
					}
				}
				if actAddress+uint32(sym.Length) <= 0x100000 && actAddress > 0 {
					// Real address must be within range
					sym.Address = actAddress
				}
			}
		}
		//sym.Name = names[i+1]
		//sym.Unit = GetUnit(sym.Name)
		//sym.Correctionfactor = GetCorrectionfactor(sym.Name)

		extractT8SymbolData(sym, fileBytes)
		//sym.Address = origAddress
		//d := extractT8SymbolData2(fileBytes, actAddress, sym.Length)
		//log.Printf("1> % X", d)
		//log.Printf("2> % X", sym.data)
	}

	cb(fmt.Sprintf("End Of Symbol Table: 0x%X", addrtaboffset))
	cb(fmt.Sprintf("NqNqNq Offset: 0x%X", NqNqNqOffset))
	cb(fmt.Sprintf("Symbol Table Offset: 0x%X", symbtaboffset))
	cb(fmt.Sprintf("Symbol Table Length: 0x%X", symbtablength))
	cb(fmt.Sprintf("Real Address Table Offset: 0x%X", addressTableOffset))

	//log.Println("Symbols found: ", symb_count)
	cb(fmt.Sprintf("Loaded %d symbols from binary", len(symbols)))

	return syms, nil
}

/*
   static private void DetermineBinaryOpenness(SymbolCollection symbols, byte[] data)
   {
       const int MinRequiredLevel = 2;
       int level = 0;


       // Determine open/closed by looking at symbol names
       if (DetermineOpen_FromSymbolNames(symbols) == true)
       {
           level++;
       }

       // Determine open/closed by looking at symbol address
       if (DetermineOpen_FromSymbolAddress(symbols) == true)
       {
           level++;
       }

       // This one has extra weight since the address should be present in a LOT of places
       if (DetermineOpen_FromData(data) == false)
       {
           level--;
       }
       else
       {
           level++;
       }

       logger.Debug("Binary openness level: " + level.ToString());
       m_openbin = (level >= MinRequiredLevel);
   }
*/

func determineBinaryOpenness(data []byte, c SymbolCollection) bool {
	const minRequiredLevel = 2
	level := 0
	if determineOpen_FromSymbolNames(c) {
		level++
	}

	if determineOpen_FromSymbolAddress(c) {
		level++
	}

	// This one has extra weight since the address should be present in a LOT of places
	if !determineOpen_FromData(data) {
		level--
	} else {
		level++
	}
	//log.Println("Binary openness level:", level)
	return level >= minRequiredLevel
}

func determineOpen_FromSymbolNames(symbols SymbolCollection) bool {
	for _, sh := range symbols.Symbols() {

		if sh.Address >= T8Length && sh.Length > 0x100 && sh.Length <= 0x400 {
			if sh.Name == "BFuelCal.LambdaOneFacMap" || sh.Name == "KnkFuelCal.fi_MaxOffsetMap" ||
				sh.Name == "AirCtrlCal.RegMap" {
				return true
			}
		}
	}
	return false
}

func determineOpen_FromSymbolAddress(symbols SymbolCollection) bool {
	for _, sh := range symbols.Symbols() {
		if sh.Address >= (0x100000 + 32768) {
			return true
		}
	}
	return false
}

func determineOpen_FromData(data []byte) bool {
	addrPat := []byte{0x20, 0x3C, 0x00, 0x14, 0x00, 0x00}
	addrMsk := []byte{0xf1, 0xbf, 0xff, 0xff, 0xff, 0x00}
	pos := uint32(0x20000)

	dataLen := uint32(len(data))
	maskLen := uint32(len(addrMsk))

	for (pos + maskLen) <= dataLen {
		if MatchPattern(data, pos, addrPat, addrMsk) {
			return true
		}

		pos += 2
	}
	return false
}

func extractT8SymbolData(sym *Symbol, data []byte) {
	if sym.Address < 0x020000 || sym.Address+uint32(sym.Length) > uint32(len(data)) {
		//log.Printf("Symbol %s out of range: 0x%X - 0x%X\n", sym.Name, sym.Address, sym.Address+uint32(sym.Length))
		return
	}
	sym.data = data[sym.Address : sym.Address+uint32(sym.Length)]
}

/*
func extractT8SymbolData2(data []byte, addr uint32, length uint16) []byte {
	if addr < 0x020000 || addr+uint32(length) > uint32(len(data)) {
		//log.Printf("Symbol out of range: 0x%X - 0x%X\n", addr, addr+uint32(length))
		return nil
	}
	return data[addr : addr+uint32(length)]
}
*/

func ReadAddressTable(data []byte, offset int) ([]*Symbol, error) {
	pos := offset - 17
	symbols := make([]*Symbol, 0)
	symb_count := 0
	for {
		symb_count++
		symboldata := data[pos : pos+10]
		pos += 10
		if pos > len(data) {
			break
		}
		if symboldata[9] != 0x00 {
			//log.Printf("End of table found at 0x%X\n", pos)
			break
		} else {
			sym := &Symbol{
				Name:         fmt.Sprintf("Symbol-%d", symb_count),
				Number:       symb_count,
				Address:      uint32(symboldata[2]) | uint32(symboldata[1])<<8 | uint32(symboldata[0])<<16,
				Length:       uint16(symboldata[4]) | uint16(symboldata[3])<<8,
				Mask:         uint16(symboldata[6]) | uint16(symboldata[5])<<8,
				Type:         symboldata[7],
				ExtendedType: symboldata[8],
			}
			symbols = append(symbols, sym)
		}
	}
	return symbols, nil
}

func GetAddressFromOffset(data []byte, offset int) (int, error) {
	if offset < 0 || offset > len(data)-4 {
		return 0, ErrOffsetOutOfRange
	}
	retval := int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
	return retval, nil
}

func GetLengthFromOffset(data []byte, offset int) (int, error) {
	if offset < 0 || offset > len(data)-2 {
		return 0, ErrOffsetOutOfRange
	}
	retval := 0
	retval += int(data[offset]) << 8
	retval += int(data[offset+1])
	return retval, nil
}

func CountNq(data []byte, offset int) int {
	cnt := 0
	if data != nil && len(data) > offset {
		state := 0
		for i := offset; i < len(data) && i > (offset-8) && cnt < 3; i++ {
			switch state {
			case 0:
				if data[i] != 0x4E {
					return cnt
				}
				state++
			case 1:
				if data[i] != 0x71 {
					return cnt
				}
				state = 0
				cnt++
				i -= 4
			}
		}
	}
	return cnt
}

var symPattern = []byte{0x73, 0x59, 0x4D, 0x42, 0x4F, 0x4C, 0x74, 0x41, 0x42, 0x4C, 0x45}

func GetEndOfSymbolTable(data []byte) (int, error) {
	pos := kmp.BytePatternSearch(data, symPattern, 0)
	if pos == -1 {
		return -1, ErrEndOfSymbolTableNotFound
	}
	return pos + len(symPattern) - 1, nil
}

func GetFirstNqStringFromOffset(data []byte, offset int) (int, error) {
	var retval, Nq1, Nq2, Nq3 int
	state := 0
outer:
	for i := offset; i < len(data) && i < offset+0x100; i++ {
		switch state {
		case 0:
			if data[i] == 0x4E {
				state++
			}
		case 1:
			if data[i] == 0x71 {
				state++
			} else {
				state = 0
			}
		case 2:
			Nq1 = i
			if data[i] == 0x4E {
				state++
			} else {
				state = 0
			}
		case 3:
			if data[i] == 0x71 {
				state++
			} else {
				state = 0
			}
		case 4:
			Nq2 = i
			if data[i] == 0x4E {
				state++
			} else {
				state = 0
			}
		case 5:
			if data[i] == 0x71 {
				state++
			} else {
				state = 0
			}
		case 6:
			Nq3 = i
			retval = i
			break outer
		}
	}

	if Nq3 == 0 {
		retval = Nq2
	}

	if retval == 0 {
		retval = Nq1
	}

	return retval, nil
}

func FindAddressTableOffset(data []byte) error {
	pos := kmp.BytePatternSearch(data, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20}, 0x3000)
	if pos == -1 {
		return ErrAddressTableOffsetNotFound
	}
	return nil
}

// readU32 reads a 32-bit unsigned integer from a byte slice at a specified position.
func readU32(data []byte, pos uint32) uint32 {
	return binary.BigEndian.Uint32(data[pos : pos+4])
}

// ReadAddressPair checks for a pattern match and reads two addresses if successful.
func ReadAddressPair(data []byte, pos uint32) (uint32, uint32, bool) {
	addrPat := []byte{0x20, 0x3C, 0x00}
	addrMsk := []byte{0xf1, 0xbf, 0xff}

	var addr1, addr2 uint32

	if MatchPattern(data, pos, addrPat, addrMsk) &&
		MatchPattern(data, pos+6, addrPat, addrMsk) &&
		(pos+12) <= uint32(len(data)) {
		addr1 = readU32(data, pos+2)
		addr2 = readU32(data, pos+8)
		return addr1, addr2, true
	}

	return 0, 0, false
}

// DecodeDataCpy decodes data copy operation based on address pairs.
func DecodeDataCpy(data []byte, pos uint32) int {
	pos += 8

	addr1, addr2, ok := ReadAddressPair(data, pos)
	if ok {
		if addr1 >= 0x100000 && addr1 < 0x108000 && addr2 < 0x100000 {
			return int(addr1 - addr2)
		}
		// Additional logic can be implemented here as needed.
	}
	return 0
}

// DetermineSecondaryOffset determines the secondary offset from the data.
func DetermineSecondaryOffset(data []byte) int {
	initFunc := readU32(data, 0x20004)

	if initFunc >= 0x20008 &&
		initFunc <= (0x100000-6) &&
		(initFunc&1) == 0 {

		if data[initFunc] == 0x4e &&
			data[initFunc+1] == 0xb9 &&
			data[initFunc+2] == 0x00 {

			nextJump := readU32(data, initFunc+2)
			if nextJump >= 0x20008 &&
				nextJump <= (0x100000-6) &&
				(nextJump&1) == 0 {

				if data[nextJump] == 0x4e &&
					data[nextJump+1] == 0xb9 &&
					data[nextJump+2] == 0x00 {

					nextJump = readU32(data, nextJump+2)

					if nextJump >= 0x20008 && (nextJump&1) == 0 {
						return DecodeDataCpy(data, nextJump)
					}
				}
			}
		}
	}
	return 0
}

// MatchPattern is a placeholder function for pattern matching logic.
func MatchPattern(data []byte, pos uint32, pattern []byte, mask []byte) bool {
	found := false

	maskLen := uint32(len(mask))
	dataLen := uint32(len(data))

	if (pos + maskLen) <= dataLen {
		found = true
		for i := uint32(0); i < maskLen; i++ {
			if (data[pos+i] & mask[i]) != (pattern[i] & mask[i]) {
				return false
			}
		}
	}

	return found
}

/*
func ExtractSymbolTable(data []byte) ([]string, error) {
	if bytes.HasPrefix(data, []byte{0xF1, 0x1A, 0x06, 0x5B, 0xA2, 0x6B, 0xCC, 0x6F}) {
		log.Println("Blowfish encrypted symbol table")
	} else {
		//return nil, ErrInvalidSymbolTableHeader
		unpackedLength := int(data[0]) | int(data[1])<<8 | int(data[2])<<16 | int(data[3])<<24
		log.Printf("Unpacked length: 0x%X\n", unpackedLength)
		if unpackedLength <= 0x00FFFFFF {
			log.Println("Decoding packed symbol table")
			return symbol.ExpandCompressedSymbolNames(data)
		}
	}
	return nil, ErrEndOfSymbolTableNotFound
}

func GetAddrTableOffsetBySymbolTable(data []byte) (int, error) {
	addrtaboffset, err := GetEndOfSymbolTable(data)
	if err != nil {
		return -1, err
	}
	log.Printf("End Of Symbol Table: 0x%X\n", addrtaboffset)

	NqNqNqOffset, err := GetFirstNqStringFromOffset(data, addrtaboffset)
	if err != nil {
		return -1, err
	}
	log.Printf("NqNqNq Offset: 0x%X\n", NqNqNqOffset)

	symbtaboffset, err := GetAddressFromOffset(data, NqNqNqOffset)
	if err != nil {
		return -1, err
	}
	log.Printf("Symbol Table Offset: 0x%X\n", symbtaboffset)

	nqCount := CountNq(data, NqNqNqOffset-2)
	log.Printf("Nq count: 0x%X\n", nqCount)

	m_addressoffset, err := GetAddressFromOffset(data, NqNqNqOffset-((nqCount*2)+6))
	if err != nil {
		return -1, err
	}
	log.Printf("Address Offset: 0x%X\n", m_addressoffset)

	symbtablength, err := GetLengthFromOffset(data, NqNqNqOffset+4)
	if err != nil {
		return -1, err
	}
	log.Printf("Symbol Table Length: 0x%X\n", symbtablength)

	if symbtablength < 0x1000 {
		return -1, ErrSymbolTableNotFound
	}

	if symbtaboffset > 0 && symbtaboffset < 0xF0000 {
		return NqNqNqOffset + 21 + 7, nil
	}

	return -1, ErrSymbolTableNotFound
}

func GetStartOfAddressTableOffset(data []byte) (int, error) {
	searchSequence := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20}
	symbolTableOffset := 0x30000
	addressTableOffset := 0
	adrState := 0
outer:
	for i := symbolTableOffset; i < len(data) && addressTableOffset == 0; i++ {
		adrb := data[i]
		switch adrState {
		case 0:
			if adrb == searchSequence[0] {
				adrState++
			}
		case 1:
			if adrb == searchSequence[1] {
				adrState++
			} else {
				adrState = 0
				i--
			}
		case 2:
			if adrb == searchSequence[2] {
				adrState++
			} else {
				adrState = 0
				i -= 2
			}
		case 3:
			if adrb == searchSequence[3] {
				adrState++
			} else {
				adrState = 0
				i -= 3
			}
		case 4:
			if adrb == searchSequence[4] {
				adrState++
			} else {
				adrState = 0
				i -= 4
			}
		case 5:
			if adrb == searchSequence[5] {
				adrState++
			} else {
				adrState = 0
				i -= 5
			}
		case 6:
			if adrb == searchSequence[6] {
				adrState++
			} else {
				adrState = 0
				i -= 6
			}
		case 7:
			if adrb == searchSequence[7] {
				adrState++
			} else {
				adrState = 0
				i -= 7
			}
		case 8:
			if adrb == searchSequence[8] {
				addressTableOffset = i - 1
				break outer
			} else {
				adrState = 0
				i -= 8
			}
		}
	}

	if addressTableOffset == 0 {
		return -1, ErrAddressTableOffsetNotFound
	}

	return addressTableOffset, nil
}
*/

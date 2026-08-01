package symbol

import (
	"encoding/binary"
	"fmt"
	"log"
)

// T7PIAreaNames labels the ids the ECU firmware knows (Ecu_id.c CopyECUID,
// XEolprg.c writePIArea). Anything else is legal too, the footer is just a
// list, so this is display only.
var T7PIAreaNames = map[byte]string{
	0x90: "VIN / chassis id", 0x91: "Part number + alpha code", 0x92: "ECU HW number (immo)",
	0x93: "ECU HW version", 0x94: "ECU SW number", 0x95: "ECU SW version", 0x97: "Engine type",
	0x98: "Tester serial number", 0x99: "Programming date (YYMMDD)", 0x9A: "ECUDiagDataID",
	0x9B: "Symbol table address (LE)", 0x9C: "SRAM offset (LE)", 0xF2: "PI area checksum",
	0xF3: "OBD worst case data", 0xF4: "OBD history counters", 0xF5: "EOL security key L3",
	0xF6: "EOL security seed L3", 0xF7: "EOL security key L1", 0xF8: "EOL security seed L1",
	0xF9: "EOL success flag", 0xFA: "Production traceability", 0xFB: "ROM checksum",
	0xFC: "Top of flash", 0xFD: "Bottom of flash", 0xFE: "Top of program",
}

// piAreaMaxBytes keeps the footer inside 0x7FF00-0x80000, the range txlogger
// writes when flashing. Real bins use 180-212 bytes.
const piAreaMaxBytes = 0xFF

// SetPIArea replaces the whole footer with fields, verbatim and in the given
// order (first entry ends up at the top of flash, like GetHeaders returns
// them). Unlike SetPiArea it does not care which ids exist, so entries can be
// added, edited or removed freely. The list is remembered and re-emitted by
// createPiArea so a later UpdateChecksum does not resurrect removed fields.
func (t7 *T7File) SetPIArea(fields []*T7HeaderField) error {
	total := 0
	seen := make(map[byte]bool, len(fields))
	out := make([]*T7HeaderField, 0, len(fields))
	for _, f := range fields {
		// 0x00/0xFF as a length byte terminates the footer walk, so a field
		// with either of those lengths would truncate everything below it.
		if len(f.Data) == 0 || len(f.Data) >= 0xFF {
			return fmt.Errorf("field 0x%02X: length must be 1..254, got %d", f.ID, len(f.Data))
		}
		if seen[f.ID] {
			return fmt.Errorf("duplicate field id 0x%02X", f.ID)
		}
		seen[f.ID] = true
		total += len(f.Data) + 2
		out = append(out, &T7HeaderField{ID: f.ID, Length: byte(len(f.Data)), Data: f.Data})
	}
	if total > piAreaMaxBytes {
		return fmt.Errorf("PI area is %d bytes, max %d", total, piAreaMaxBytes)
	}

	t7.piArea = out
	t7.clearPiArea()
	t7.createPiArea()

	// re-sync the modelled fields from what we just wrote
	t7.otherFields = nil
	t7.chassisIDCounter = 0
	t7.chassisIDDetected, t7.immocodeDetected = false, false
	t7.loadHeaders()
	return nil
}

// writePiArea emits the raw override list. Checksums are recalculated on save,
// so the stored entries must not pin the old values.
func (t7 *T7File) writePiArea() {
	pos := len(t7.data) - 1
	for _, f := range t7.piArea {
		if len(f.Data) == 4 {
			switch f.ID {
			case 0xF2:
				binary.BigEndian.PutUint32(f.Data, uint32(t7.checksumF2))
			case 0xFB:
				binary.BigEndian.PutUint32(f.Data, uint32(t7.checksumFB))
			}
		}
		pos = t7.writeFooterBytes(pos, f.ID, f.Data)
	}
}

func (t7 *T7File) clearPiArea() {
	const startPosition = 0x07FE00
	if len(t7.data) <= startPosition {
		return
	}
	for i := startPosition; i < len(t7.data); i++ {
		(t7.data)[i] = 0xFF
	}
	log.Println("Footer cleared")
}

func (t7 *T7File) createPiArea() {
	if t7.piArea != nil {
		t7.writePiArea()
		return
	}
	log.Println("Creating new footer")
	pos := len(t7.data) - 1

	pos = t7.writeFooterString(pos, 0x91, t7.partNrAlphaCode)
	pos = t7.writeFooterString(pos, 0x94, t7.ecuSoftwNr)
	pos = t7.writeFooterString(pos, 0x95, t7.softwareVersion)
	pos = t7.writeFooterString(pos, 0x97, t7.engineType)
	pos = t7.writeFooterString(pos, 0x9A, t7.ecuDiagDataID)

	// 0x9C/0x9B are linker fields the ECU never reads, and they are little-endian.
	if t7.symbolTableChecksumDetected {
		pos = t7.writeFooterIntLE(pos, 0x9C, t7.sramOffset)
	}

	if t7.symbolTableMarkerDetected {
		pos = t7.writeFooterIntLE(pos, 0x9B, t7.symbolTableAddress)
	}

	// Everything below here is read by the firmware, so big-endian.
	if t7.f2ChecksumDetected {
		pos = t7.writeFooterInt(pos, 0xF2, t7.checksumF2)
	}

	pos = t7.writeFooterInt(pos, 0xFB, t7.checksumFB)
	pos = t7.writeFooterInt(pos, 0xFC, t7.topOfFlash)
	pos = t7.writeFooterInt(pos, 0xFD, t7.bottomOfFlash)
	pos = t7.writeFooterInt(pos, 0xFE, t7.topOfProgram)
	pos = t7.writeFooterBytes(pos, 0xFA, t7.traceability)
	pos = t7.writeFooterString(pos, 0x92, t7.immobilizerID)
	pos = t7.writeFooterString(pos, 0x93, t7.ecuHardwVersNr)
	pos = t7.writeFooterInt16(pos, 0xF8, t7.securitySeedL1)
	pos = t7.writeFooterInt16(pos, 0xF7, t7.securityKeyL1)
	pos = t7.writeFooterInt16(pos, 0xF6, t7.securitySeedL3)
	pos = t7.writeFooterInt16(pos, 0xF5, t7.securityKeyL3)

	// Unmodelled fields go back between 0xF5 and 0x90, which is where XEolprg.c
	// writes 0xF4/0xF3 and where real bins carry them.
	for _, h := range t7.otherFields {
		pos = t7.writeFooterBytes(pos, h.ID, h.Data)
	}

	pos = t7.writeFooterString(pos, 0x90, t7.chassisID)
	pos = t7.writeFooterString(pos, 0x99, t7.softwareDate)
	pos = t7.writeFooterString(pos, 0x98, t7.testerSerialNr)
	t7.writeFooterBytes(pos, 0xF9, []byte{t7.romChecksumError})

	//	log.Printf("pos: %d, %X", pos, t7.data[0x07FE00:])
}

func (t7 *T7File) writeFooter(pos int, h T7HeaderField) int {
	t7.data[pos] = h.Length
	pos--
	t7.data[pos] = h.ID
	for i := 0; i < int(h.Length); i++ {
		t7.data[pos-int(h.Length)+i] = h.Data[int(h.Length-1)-i]
	}
	pos -= int(h.Length + 1)
	return pos
}

func (t7 *T7File) writeFooterBytes(pos int, id byte, value []byte) int {
	h := T7HeaderField{
		ID:     id,
		Length: byte(len(value)),
		Data:   value,
	}
	return t7.writeFooter(pos, h)
}

func (t7 *T7File) writeFooterString(pos int, id byte, value string) int {
	h := T7HeaderField{
		ID:     id,
		Length: byte(len(value)),
		Data:   []byte(value),
	}
	return t7.writeFooter(pos, h)
}

// writeFooter stores Data in ECU read order, so the byte order of a numeric
// field is just the order we encode it in here. BE32(writeFooterInt) is what the
// firmware reads; LE32(writeFooterIntLE) is only for 0x9B/0x9C.

func (t7 *T7File) writeFooterInt(pos int, id byte, value int) int {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, uint32(value))
	return t7.writeFooter(pos, T7HeaderField{ID: id, Length: 4, Data: data})
}

func (t7 *T7File) writeFooterIntLE(pos int, id byte, value int) int {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, uint32(value))
	return t7.writeFooter(pos, T7HeaderField{ID: id, Length: 4, Data: data})
}

func (t7 *T7File) writeFooterInt16(pos int, id byte, value int) int {
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, uint16(value))
	return t7.writeFooter(pos, T7HeaderField{ID: id, Length: 2, Data: data})
}

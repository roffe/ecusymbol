package symbol

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type T7HeaderField struct {
	ID     byte
	Length byte
	Data   []byte
}

func (h *T7HeaderField) PrettyString() string {
	switch h.Length {
	case 4:
		return fmt.Sprintf("0x%02x %d> 0x%08X  %q", h.ID, len(h.Data), binary.BigEndian.Uint32(h.Data), h.Data)
	case 2:
		return fmt.Sprintf("0x%02x %d> 0x%04X  %q", h.ID, len(h.Data), binary.BigEndian.Uint16(h.Data), h.Data)
	default:
		return fmt.Sprintf("0x%02x> %s", h.ID, string(h.Data))
	}
}

func (h *T7HeaderField) String() string {
	nullIndex := bytes.IndexByte(h.Data, '\x00')
	if nullIndex != -1 {
		return string(h.Data[:nullIndex])
	} else {
		return string(h.Data)
	}
}

// Data is in ECU read order: GetHeaders walks the footer downward, so Data[0] is
// the byte immediately below the id. Every field the firmware itself reads is
// big-endian in that order -- CopyECUID/readPIArea copy it with *ptr2++ = *ptr--
// into a u16/u32 on a big-endian 68332. (Ecu_id.c's "set pointer to low byte"
// comment is wrong for 68k.) 0x9B and 0x9C are the exception: the ECU never
// reads them, they are emitted by the linker for tooling and are little-endian.
//
// Short data returns 0 rather than panicking; a corrupt footer is untrusted input.

func (h *T7HeaderField) BE16() int {
	if len(h.Data) < 2 {
		return 0
	}
	return int(binary.BigEndian.Uint16(h.Data))
}

func (h *T7HeaderField) BE32() int {
	if len(h.Data) < 4 {
		return 0
	}
	return int(binary.BigEndian.Uint32(h.Data))
}

// LE32 is only correct for 0x9B and 0x9C.
func (h *T7HeaderField) LE32() int {
	if len(h.Data) < 4 {
		return 0
	}
	return int(binary.LittleEndian.Uint32(h.Data))
}

func (t7 *T7File) loadHeaders() {
	for _, h := range t7.GetHeaders() {
		switch h.ID {
		case 0x90:
			t7.chassisID = h.String()
			t7.chassisIDDetected = true
			t7.chassisIDCounter++
		case 0x91:
			t7.partNrAlphaCode = h.String()
		case 0x92:
			t7.immobilizerID = h.String()
			t7.immocodeDetected = true
		case 0x93:
			t7.ecuHardwVersNr = h.String()
		case 0x94:
			t7.ecuSoftwNr = h.String()
		case 0x95:
			t7.softwareVersion = h.String()
		case 0x97:
			t7.engineType = h.String()
		case 0x98:
			t7.testerSerialNr = h.String()
		case 0x99:
			t7.softwareDate = h.String()
		case 0x9A:
			t7.ecuDiagDataID = h.String()
		case 0x9B:
			t7.symbolTableAddress = h.LE32()
			t7.symbolTableMarkerDetected = true
		case 0x9C:
			t7.sramOffset = h.LE32()
			t7.symbolTableChecksumDetected = true
		case 0xF2:
			t7.checksumF2 = h.BE32()
			t7.f2ChecksumDetected = true
		case 0xF5:
			t7.securityKeyL3 = h.BE16()
		case 0xF6:
			t7.securitySeedL3 = h.BE16()
		case 0xF7:
			t7.securityKeyL1 = h.BE16()
		case 0xF8:
			t7.securitySeedL1 = h.BE16()
		case 0xF9:
			if len(h.Data) > 0 {
				t7.romChecksumError = h.Data[0]
			}
		case 0xFA:
			t7.traceability = h.Data
		case 0xFB:
			t7.checksumFB = h.BE32()
		case 0xFC:
			t7.topOfFlash = h.BE32()
		case 0xFD:
			t7.bottomOfFlash = h.BE32()
		case 0xFE:
			t7.topOfProgram = h.BE32()
		default:
			// Anything we do not model is kept verbatim and re-emitted by
			// createPiArea. 0xF3/0xF4 (OBDWorstCaseType/OBDHistCountType) are
			// real firmware fields carrying OBD-II history across a reflash;
			// dropping them wipes the ECU's readiness data.
			t7.otherFields = append(t7.otherFields, h)
		}
	}
	if (t7.chassisIDCounter > 1 || !t7.immocodeDetected || !t7.chassisIDDetected) && t7.autoFixFooter {
		t7.clearPiArea()
		t7.createPiArea()
	}
}

func (t7 *T7File) GetHeaders() []*T7HeaderField {
	binLength := len(t7.data)
	addr := binLength - 1

	fields := make([]*T7HeaderField, 0)

	for addr > (binLength - 0x1FF) {
		//		log.Printf("addr: %X", addr)
		/* The first byte is the length of the data */
		fieldLength := t7.data[addr]
		//		log.Printf("fieldLength %X", fieldLength)
		//log.Printf("%3d, %x", lengthField, lengthField)
		if fieldLength == 0x00 || fieldLength == 0xFF {
			break
		}
		addr--

		fieldID := t7.data[addr]
		addr--

		fieldData := make([]byte, int(fieldLength))
		fieldData[fieldLength-1] = 0x00

		for i := 0; i < int(fieldLength); i++ {
			fieldData[i] = t7.data[addr]
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

package symbol

import (
	"bytes"
	"testing"
)

// fieldData returns a footer field's payload in ECU read order (the order
// CopyECUID sees it, walking down from the length byte), or nil if absent.
func fieldData(data []byte, id byte) []byte {
	addr := len(data) - 1
	for addr > len(data)-0x1FF {
		length := data[addr]
		if length == 0x00 || length == 0xFF {
			return nil
		}
		addr--
		fieldID := data[addr]
		addr--
		d := make([]byte, length)
		for i := 0; i < int(length); i++ {
			d[i] = data[addr]
			addr--
		}
		if fieldID == id {
			return d
		}
	}
	return nil
}

// applyRealBinPiArea fills the modelled fields with the widths actually seen in
// production bins: 0x94 is 11 chars, 0x97 is 30.
const realBinChassisID = "YS3EB55A843011337"

func applyRealBinPiArea(t7 *T7File) {
	t7.chassisID = realBinChassisID
	t7.partNrAlphaCode = "55565637 "
	t7.immobilizerID = "123456789012345"
	t7.ecuHardwVersNr = "0000000"
	t7.ecuSoftwNr = "55569109   "
	t7.softwareVersion = "EU0DF25O.55P"
	t7.engineType = "9-5 B235E EC2000 EU (@25Mhz)  "
	t7.testerSerialNr = "0000000000000"
	t7.softwareDate = "050225"
	t7.ecuDiagDataID = "0748"
	t7.traceability = []byte{0x42, 0xFB, 0xFA, 0xFF, 0xFF}
}

// createPiArea's modelled path (the one used for a bin with no SetPIArea
// override) must survive a rebuild -> reparse cycle, which is what
// UpdateChecksum does on every save.
func TestPiAreaRoundTrip(t *testing.T) {
	t7 := &T7File{data: make([]byte, T7Length)}
	applyRealBinPiArea(t7)
	t7.clearPiArea()
	t7.createPiArea()

	fresh := &T7File{data: t7.data}
	fresh.Collection = NewCollection()
	fresh.loadHeaders()
	for _, tc := range []struct {
		name, got, want string
	}{
		{"chassisID", fresh.chassisID, t7.chassisID},
		{"partNrAlphaCode", fresh.partNrAlphaCode, t7.partNrAlphaCode},
		{"immobilizerID", fresh.immobilizerID, t7.immobilizerID},
		{"ecuHardwVersNr", fresh.ecuHardwVersNr, t7.ecuHardwVersNr},
		{"ecuSoftwNr", fresh.ecuSoftwNr, t7.ecuSoftwNr},
		{"softwareVersion", fresh.softwareVersion, t7.softwareVersion},
		{"engineType", fresh.engineType, t7.engineType},
		{"testerSerialNr", fresh.testerSerialNr, t7.testerSerialNr},
		{"softwareDate", fresh.softwareDate, t7.softwareDate},
		{"ecuDiagDataID", fresh.ecuDiagDataID, t7.ecuDiagDataID},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// SetPIArea is the raw editor path: whatever list goes in comes back out,
// including ids the package does not model, and removals survive the
// clear/rebuild that UpdateChecksum does on every save.
func TestSetPIAreaRoundTrip(t *testing.T) {
	t7 := &T7File{data: make([]byte, T7Length)}
	applyRealBinPiArea(t7)
	t7.clearPiArea()
	t7.createPiArea()

	fields := t7.GetHeaders()
	var kept []*T7HeaderField
	for _, f := range fields {
		if f.ID == 0x98 { // remove the tester serial
			continue
		}
		kept = append(kept, f)
	}
	kept = append(kept, &T7HeaderField{ID: 0x42, Data: []byte("hello")}) // brand new id

	if err := t7.SetPIArea(kept); err != nil {
		t.Fatal(err)
	}
	t7.clearPiArea()
	t7.createPiArea() // what UpdateChecksum does

	if got := fieldData(t7.data, 0x98); got != nil {
		t.Errorf("0x98 came back after removal: % X", got)
	}
	if got := fieldData(t7.data, 0x42); !bytes.Equal(got, []byte("hello")) {
		t.Errorf("new field 0x42: got %q", got)
	}
	if got := fieldData(t7.data, 0x90); !bytes.Equal(got, []byte(realBinChassisID)) {
		t.Errorf("0x90 not preserved: %q", got)
	}
}

func TestSetPIAreaValidation(t *testing.T) {
	t7 := &T7File{data: make([]byte, T7Length)}
	for _, tc := range []struct {
		name   string
		fields []*T7HeaderField
	}{
		{"empty field", []*T7HeaderField{{ID: 0x90, Data: nil}}},
		{"duplicate id", []*T7HeaderField{{ID: 0x90, Data: []byte("a")}, {ID: 0x90, Data: []byte("b")}}},
		{"too big", []*T7HeaderField{{ID: 0x90, Data: make([]byte, 200)}, {ID: 0x91, Data: make([]byte, 200)}}},
	} {
		if err := t7.SetPIArea(tc.fields); err == nil {
			t.Errorf("%s: expected an error", tc.name)
		}
	}
}

// Byte patterns below are lifted verbatim from production bins. Every field the
// firmware reads is big-endian in ECU read order; 0x9B/0x9C are little-endian
// because the ECU never reads them (CopyECUID skips them, the linker emits them).
func TestFooterEndianness(t *testing.T) {
	t7 := &T7File{data: make([]byte, T7Length)}
	applyRealBinPiArea(t7)

	t7.topOfFlash = 0x0007FFFF
	t7.bottomOfFlash = 0x00000000
	t7.topOfProgram = 0x0006B5E0
	t7.checksumFB = 0xF1138920
	t7.checksumF2 = 0xD2D622D5
	t7.f2ChecksumDetected = true
	t7.symbolTableAddress = 0x0005F768
	t7.symbolTableMarkerDetected = true
	t7.sramOffset = 0x00F00EB4
	t7.symbolTableChecksumDetected = true
	t7.securitySeedL1 = 0x70F2
	t7.securityKeyL1 = 0x06C5
	t7.securitySeedL3 = 0x7109
	t7.securityKeyL3 = 0xFB45

	t7.clearPiArea()
	t7.createPiArea()

	for _, tc := range []struct {
		id   byte
		want []byte
		why  string
	}{
		{0xFC, []byte{0x00, 0x07, 0xFF, 0xFF}, "TopOffFlash, big-endian"},
		{0xFE, []byte{0x00, 0x06, 0xB5, 0xE0}, "TopOffProgram, big-endian"},
		{0xFB, []byte{0xF1, 0x13, 0x89, 0x20}, "ROMchecksum, big-endian"},
		{0xF2, []byte{0xD2, 0xD6, 0x22, 0xD5}, "PIAreaChecksum, big-endian"},
		{0x9B, []byte{0x68, 0xF7, 0x05, 0x00}, "symboltable address, little-endian"},
		{0x9C, []byte{0xB4, 0x0E, 0xF0, 0x00}, "sram offset, little-endian"},
		{0xF8, []byte{0x70, 0xF2}, "security seed L1, big-endian"},
		{0xF5, []byte{0xFB, 0x45}, "security key L3, big-endian"},
	} {
		got := fieldData(t7.data, tc.id)
		if !bytes.Equal(got, tc.want) {
			t.Errorf("0x%02X (%s): got % X, want % X", tc.id, tc.why, got, tc.want)
		}
	}

	// and the values must survive a reparse
	fresh := &T7File{data: t7.data}
	fresh.Collection = NewCollection()
	fresh.loadHeaders()
	for _, tc := range []struct {
		name      string
		got, want int
	}{
		{"topOfFlash", fresh.topOfFlash, 0x0007FFFF},
		{"topOfProgram", fresh.topOfProgram, 0x0006B5E0},
		{"checksumFB", fresh.checksumFB, 0xF1138920},
		{"symbolTableAddress", fresh.symbolTableAddress, 0x0005F768},
		{"sramOffset", fresh.sramOffset, 0x00F00EB4},
		{"securitySeedL1", fresh.securitySeedL1, 0x70F2},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got 0x%X, want 0x%X", tc.name, tc.got, tc.want)
		}
	}
}

// 0xF3/0xF4 carry OBDWorstCaseType/OBDHistCountType across a reflash. They are
// real firmware fields (FDE_EE0C.47S.V01/Misc.app/Source/Ecu_id.c) that this
// package does not model, so they must be preserved verbatim rather than wiped.
func TestFooterPreservesUnknownFields(t *testing.T) {
	obdHistCount := make([]byte, 56) // sizeof(OBDHistCountType)
	obdWorstCase := make([]byte, 106)
	for i := range obdHistCount {
		obdHistCount[i] = byte(i + 1)
	}
	for i := range obdWorstCase {
		obdWorstCase[i] = byte(0xA0 + i)
	}

	t7 := &T7File{data: make([]byte, T7Length)}
	applyRealBinPiArea(t7)
	t7.otherFields = []*T7HeaderField{
		{ID: 0xF4, Length: 56, Data: obdHistCount},
		{ID: 0xF3, Length: 106, Data: obdWorstCase},
	}

	t7.clearPiArea()
	t7.createPiArea()

	if got := fieldData(t7.data, 0xF4); !bytes.Equal(got, obdHistCount) {
		t.Errorf("0xF4 OBDHistCount not preserved:\n got % X\nwant % X", got, obdHistCount)
	}
	if got := fieldData(t7.data, 0xF3); !bytes.Equal(got, obdWorstCase) {
		t.Errorf("0xF3 OBDWorstCase not preserved:\n got % X\nwant % X", got, obdWorstCase)
	}

	// they must also survive a parse -> rebuild cycle, which is what
	// UpdateChecksum does on every Save
	fresh := &T7File{data: t7.data}
	fresh.Collection = NewCollection()
	fresh.loadHeaders()
	if len(fresh.otherFields) != 2 {
		t.Fatalf("expected 2 preserved fields after reparse, got %d", len(fresh.otherFields))
	}
	fresh.clearPiArea()
	fresh.createPiArea()
	if got := fieldData(fresh.data, 0xF4); !bytes.Equal(got, obdHistCount) {
		t.Errorf("0xF4 lost on the second rebuild: got % X", got)
	}
	if got := fieldData(fresh.data, 0xF3); !bytes.Equal(got, obdWorstCase) {
		t.Errorf("0xF3 lost on the second rebuild: got % X", got)
	}
}

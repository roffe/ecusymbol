package symbol

import "testing"

// Q_AirInletTab2 is marked SIGNED in the T7 symbol table but the ECU reads it as
// u16, so entries above 32767 must not come back negative.
func TestFixT7SymbolTypeUnsignedAirInlet(t *testing.T) {
	s := &Symbol{
		Name:   "VIOSMAFCal.Q_AirInletTab2",
		Type:   0x23, // SIGNED|KONST|16bit as found in EU0D bins
		Length: 4,
		data:   []byte{0x84, 0xD0, 0xB4, 0xDC}, // 34000, 46300
	}
	fixT7SymbolType(s)
	got := s.Ints()
	want := []int{34000, 46300}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Ints() = %v, want %v", got, want)
		}
	}
	if got := s.BytesToInts(s.data); got[1] != 46300 {
		t.Fatalf("BytesToInts() = %v, want 46300 at index 1", got)
	}
}

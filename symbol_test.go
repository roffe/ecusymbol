package symbol

import (
	"bytes"
	"testing"
)

// Values outside what a symbol's type holds are clamped, not wrapped.
func TestEncodeClamps(t *testing.T) {
	for _, c := range []struct {
		typ  uint8
		in   float64
		want []byte
	}{
		{CHAR, 200, []byte{200}},
		{CHAR, 256, []byte{0xFF}},
		{CHAR, -1, []byte{0x00}},
		{CHAR | SIGNED, 200, []byte{0x7F}},
		{CHAR | SIGNED, -200, []byte{0x80}},
		{0, 70000, []byte{0xFF, 0xFF}},
		{0, -1, []byte{0x00, 0x00}},
		{SIGNED, -40000, []byte{0x80, 0x00}},
		{LONG, -1, []byte{0, 0, 0, 0}},
		{LONG | SIGNED, 1 << 40, []byte{0x7F, 0xFF, 0xFF, 0xFF}},
	} {
		s := &Symbol{Name: "x", Type: c.typ, Correctionfactor: 1}
		if got := s.EncodeFloat64(c.in); !bytes.Equal(got, c.want) {
			t.Errorf("type %02X: %g encodes to % X, want % X", c.typ, c.in, got, c.want)
		}
	}
	if got := (&Symbol{Type: CHAR}).EncodeInts([]int{300, -5}); !bytes.Equal(got, []byte{0xFF, 0x00}) {
		t.Errorf("EncodeInts = % X, want FF 00", got)
	}
}

// A one-byte symbol reads as a byte whatever its type byte says; tuned
// binaries carry 1-byte symbols typed 16-bit.
func TestOneByteSymbolIsByte(t *testing.T) {
	s := &Symbol{Name: "x", Length: 1, Type: SIGNED, Correctionfactor: 1, data: []byte{0xFF}}
	NewCollection(s)
	if got := s.Float64s(); len(got) != 1 || got[0] != -1 {
		t.Errorf("Float64s = %v, want [-1]", got)
	}
}

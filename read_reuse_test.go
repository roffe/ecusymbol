package symbol

import (
	"bytes"
	"testing"
)

// Read must reuse its buffer across frames yet always reflect the latest bytes.
func TestReadReusesBuffer(t *testing.T) {
	s := &Symbol{Name: "x", Length: 2}
	if err := s.Read(bytes.NewReader([]byte{0x12, 0x34})); err != nil {
		t.Fatal(err)
	}
	first := s.Bytes()
	if !bytes.Equal(first, []byte{0x12, 0x34}) {
		t.Fatalf("got %X", first)
	}
	if err := s.Read(bytes.NewReader([]byte{0xAB, 0xCD})); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(s.Bytes(), []byte{0xAB, 0xCD}) {
		t.Fatalf("stale data %X", s.Bytes())
	}
	// short read must error, not silently keep old data
	if err := s.Read(bytes.NewReader([]byte{0x01})); err == nil {
		t.Fatal("expected short-read error")
	}
}

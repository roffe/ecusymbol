package symbol

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"
)

var quietT5 = WithT5PrintFunc(func(string) {})

// t5Bins returns every T5 binary in the local TuningSuites reference set,
// or skips the test when the set is not present on this machine.
func t5Bins(t *testing.T) []string {
	t.Helper()
	files, _ := filepath.Glob("testdata/T5Binaries/*.BIN")
	if len(files) == 0 {
		t.Skip("no T5 reference binaries available")
	}
	return files
}

// TestT5RoundTrip loads every reference bin, checks that an unmodified
// writeback is byte-identical, then edits a map, saves, reloads and checks
// the edit survived with a valid checksum.
func TestT5RoundTrip(t *testing.T) {
	for _, path := range t5Bins(t) {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			orig := readBin(t, path)
			f, err := NewT5File(bytes.Clone(orig), quietT5)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if f.Version() == "" {
				t.Error("no software version found")
			}
			if f.Count() == 0 {
				t.Fatal("no symbols")
			}
			for _, s := range f.Symbols() {
				if strings.HasPrefix(s.Name, "Knock_count_cyl") && s.Length != 2 {
					t.Errorf("%s: length %d, want 2", s.Name, s.Length)
				}
				if strings.HasPrefix(s.Name, "Reg_kon_mat") && (s.Type&CHAR == CHAR) != (s.Length == 0x80) {
					t.Errorf("%s: length %d type %02X", s.Name, s.Length, s.Type)
				}
			}

			// Unmodified writeback must be identical to the input.
			out, err := f.Byte()
			if err != nil {
				t.Fatalf("byte: %v", err)
			}
			if !bytes.Equal(out, orig) {
				t.Fatal("unmodified writeback differs from input")
			}

			// Edit the first map that lives in flash. On the T5.2 bins that is
			// Adapt_ref, which shares its flash bytes with Adapt_korr and
			// Insp_mat!, so this also covers aliased symbols.
			var sym *Symbol
			for _, s := range f.Symbols() {
				if s.Address != 0 && s.Length >= 2 {
					sym = s
					break
				}
			}
			if sym == nil {
				t.Fatal("no flash-backed symbol to edit")
			}
			edited := bytes.Clone(sym.Bytes())
			edited[0] ^= 0xFF
			if err := sym.SetData(edited); err != nil {
				t.Fatal(err)
			}
			saved := filepath.Join(t.TempDir(), name)
			if err := f.Save(saved); err != nil {
				t.Fatalf("save: %v", err)
			}
			data := readBin(t, saved)
			if len(data) != len(orig) {
				t.Fatalf("saved size %d, want %d", len(data), len(orig))
			}
			// Every cache, aliases included, must now agree with the file.
			for _, s := range f.Symbols() {
				if s.Address != 0 && !bytes.Equal(s.Bytes(), data[s.Address:int(s.Address)+len(s.Bytes())]) {
					t.Errorf("%s: cached data differs from the saved file", s.Name)
				}
			}

			// Reload: checksum must verify and the edit must be in place.
			g, err := NewT5File(data, quietT5)
			if err != nil {
				t.Fatalf("reload: %v", err)
			}
			got := g.GetByName(sym.Name)
			if got == nil {
				t.Fatalf("%s missing after reload", sym.Name)
			}
			if !bytes.Equal(got.Bytes(), edited) {
				t.Errorf("%s: edit lost after reload", sym.Name)
			}
			// Only the edited byte and the checksum may differ.
			diff := 0
			for i := range orig[:len(orig)-4] {
				if orig[i] != data[i] {
					diff++
				}
			}
			if diff != 1 {
				t.Errorf("%d bytes differ outside the checksum, want 1", diff)
			}
		})
	}
}

// TestT5BadChecksum: a strict load refuses a wrong checksum, Load repairs it
// like T5Suite does.
func TestT5BadChecksum(t *testing.T) {
	path := t5Bins(t)[0]
	data := readBin(t, path)
	data[len(data)-1] ^= 0xFF
	if _, err := NewT5File(bytes.Clone(data), quietT5); !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("strict load: got %v, want ErrChecksumMismatch", err)
	}
	_, fw, err := Load(path, data, func(string) {})
	if err != nil {
		t.Fatalf("Load should repair the checksum: %v", err)
	}
	if err := fw.(*T5File).VerifyChecksum(); err != nil {
		t.Error(err)
	}
}

// TestT5CorruptInput: damaged images must fail or degrade cleanly, never panic.
func TestT5CorruptInput(t *testing.T) {
	bins := t5Bins(t)
	t.Run("end marker past EOF", func(t *testing.T) {
		data := readBin(t, bins[0])
		i := len(data) - 0x100 + bytes.IndexByte(data[len(data)-0x100:], 0xFE)
		copy(data[i-6:i], "FFFFF0") // read backwards as hex: 0FFFFF
		if _, err := NewT5File(data, quietT5); err == nil {
			t.Fatal("loaded without error")
		}
	})

	// The lookup table follows a fixed code sequence; its entries start 48 79 00.
	sig := []byte{0x4E, 0x75, 0x48, 0xE7, 0x01, 0x30, 0x26, 0x6F, 0x00, 0x16, 0x3E, 0x2F, 0x00, 0x14, 0x24, 0x6F, 0x00, 0x10, 0x60, 0x00, 0x00, 0x0A}
	pea := []byte{0x48, 0x79, 0x00}
	seen := map[int]bool{}
	for _, path := range bins {
		data := readBin(t, path)
		if seen[len(data)] { // one T5.5 and one T5.2 image is enough
			continue
		}
		seen[len(data)] = true
		t.Run("garbage lookup entries "+filepath.Base(path), func(t *testing.T) {
			pristine, err := NewT5File(bytes.Clone(data), quietT5)
			if err != nil {
				t.Fatal(err)
			}
			rec := bytes.Index(data, sig)
			for _, flash := range [][]byte{{0x00, 0x08, 0x00, 0x00}, {0x00, 0x03, 0xFF, 0xFF}} {
				rec += len(pea) + bytes.Index(data[rec+len(pea):], pea)
				copy(data[rec+2:], flash)
			}
			f, err := NewT5File(data, quietT5, WithT5AutoCorrectChecksum())
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if f.Count() != pristine.Count() {
				t.Errorf("%d symbols, want %d", f.Count(), pristine.Count())
			}
		})
	}

	t.Run("random images", func(t *testing.T) {
		r := rand.New(rand.NewPCG(1, 2))
		for _, size := range []int{LengthT52, LengthT55} {
			data := make([]byte, size)
			for i := 0; i < 4; i++ {
				for j := range data {
					data[j] = byte(r.Uint32())
				}
				if _, err := NewT5File(data, quietT5); err == nil {
					t.Errorf("random %d-byte image loaded without error", size)
				}
			}
		}
	})
}

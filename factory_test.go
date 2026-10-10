package symbol

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

// The factory binaries live under testdata/<ECU>Binaries (gitignored, local
// only). Every test here runs over all of them and skips when none are present.

var testdataDirs = []struct {
	typ ECUType
	dir string
}{
	{ECU_T5, "testdata/T5Binaries"},
	{ECU_T7, "testdata/T7Binaries"},
	{ECU_T8, "testdata/T8Binaries"},
}

// knownBroken are files in testdata that are not valid images; the reason is
// the skip message.
var knownBroken = map[string]string{
	// L1/L2 checksums verify, but 476 bytes of the compressed symbol-name
	// table differ from the otherwise identical 55352571_FA4H_C_FME9_28_SAN_PF_81b.BIN
	// and the names decode to garbage from there: a scrambled symbol table.
	"55352688_FA4H_C_FME9_28_SAN_PF_81b.BIN": "scrambled symbol table",
}

type factoryBin struct {
	typ  ECUType
	path string
}

func (b factoryBin) String() string {
	return fmt.Sprintf("%s/%s", b.typ, filepath.Base(b.path))
}

// factoryBins lists the binaries under testdata, or under $ECUSYMBOL_TESTDATA
// to run the suite against another copy of the set.
func factoryBins(t *testing.T) []factoryBin {
	t.Helper()
	root := os.Getenv("ECUSYMBOL_TESTDATA")
	var bins []factoryBin
	for _, d := range testdataDirs {
		dir := d.dir
		if root != "" {
			dir = filepath.Join(root, filepath.Base(d.dir))
		}
		files, _ := filepath.Glob(filepath.Join(dir, "*.[bB][iI][nN]"))
		for _, f := range files {
			if why := knownBroken[filepath.Base(f)]; why != "" {
				t.Logf("skipping %s: %s", filepath.Base(f), why)
				continue
			}
			if nordicTagged(f) {
				t.Logf("skipping %s: Nordic tune, symbol table broken on purpose", filepath.Base(f))
				continue
			}
			bins = append(bins, factoryBin{d.typ, f})
		}
	}
	if len(bins) == 0 {
		t.Skip("no factory binaries under testdata/")
	}
	return bins
}

func quiet(string) {}

// strictLoad parses without any checksum auto-correction, so a wrong checksum
// is reported instead of silently repaired.
func strictLoad(typ ECUType, data []byte) (FirmwareFile, error) {
	switch typ {
	case ECU_T5:
		return NewT5File(data, WithT5PrintFunc(quiet))
	case ECU_T7:
		return NewT7File(data, WithT7PrintFunc(quiet))
	case ECU_T8:
		return NewT8File(data, WithT8PrintFunc(quiet))
	}
	return nil, fmt.Errorf("no strict loader for %s", typ)
}

// flashRange is the file range a symbol's data is read from and written to,
// mirroring each Save implementation. ok is false for RAM-only symbols.
func flashRange(typ ECUType, s *Symbol, size int) (start, end int, ok bool) {
	addr := int64(s.Address)
	switch typ {
	case ECU_T5:
		if s.Address == 0 {
			return 0, 0, false
		}
	case ECU_T7:
		if s.Number == 0 { // the symbol table itself
			return 0, 0, false
		}
		if s.Address > 0x7FFFFF { // RAM: flash image only inside a calibration window
			if s.SramOffset == 0 {
				return 0, 0, false
			}
			addr -= int64(s.SramOffset)
		}
	case ECU_T8:
		if s.Address == 0 || s.Address > 0x100000 {
			return 0, 0, false
		}
	}
	if addr < 0 || addr+int64(s.Length) > int64(size) {
		return 0, 0, false
	}
	return int(addr), int(addr) + int(s.Length), s.Length > 0
}

// checksumSlot is where the stored checksum lives; a symbol covering it never
// reads back what was written because the writeback recomputes it.
func checksumSlot(typ ECUType, fw FirmwareFile, size int) (start, end int) {
	switch f := fw.(type) {
	case *T5File:
		return size - 4, size
	case *T7File:
		c, err := f.getFWChecksum()
		if err != nil {
			return 0, 0
		}
		return c.Address, c.Address + 4
	case *T8File:
		off, err := f.GetChecksumAreaOffset(f.data)
		if err != nil {
			return 0, 0
		}
		return off + 2, off + 18
	}
	return 0, 0
}

// checksumCovers reports whether the ECU's checksum includes the byte at pos.
func checksumCovers(fw FirmwareFile, pos int) bool {
	switch f := fw.(type) {
	case *T5File:
		end, err := f.getIndexOfFirstMarking()
		return err == nil && pos < end+4
	case *T7File:
		for _, a := range f.csumArea {
			if pos >= a.Address && pos < a.Address+a.Length {
				return true
			}
		}
		return false
	case *T8File:
		off, err := f.GetChecksumAreaOffset(f.data)
		return err == nil && pos >= 0x20000 && pos < off
	}
	return false
}

// pattern is a byte derived from its absolute file offset, so two symbols that
// share a range agree on what they write.
func pattern(pos int) byte {
	return byte(pos*31+0x5A) ^ byte(pos>>8)
}

// checksumBudget is the most bytes outside every symbol a writeback may touch
// (checksums, PI area / footer fields).
const checksumBudget = 256

type writer interface {
	Byte() ([]byte, error)
}

// nordicTagged reports whether a binary carries Nordic's "Tag:Nordic" stamp
// at the end of flash. Their tunes hide maps by breaking the symbol table.
func nordicTagged(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	tail := make([]byte, 0x100)
	fi, err := f.Stat()
	if err != nil || fi.Size() < int64(len(tail)) {
		return false
	}
	if _, err := f.ReadAt(tail, fi.Size()-int64(len(tail))); err != nil {
		return false
	}
	return bytes.Contains(tail, []byte("Tag:Nordic"))
}

func readBin(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestFactoryRead: every binary loads with a valid checksum, has a software
// version and symbols, and every flash-backed symbol's bytes are exactly what
// the file holds at its address.
func TestFactoryRead(t *testing.T) {
	for _, bin := range factoryBins(t) {
		t.Run(bin.String(), func(t *testing.T) {
			t.Parallel()
			orig := readBin(t, bin.path)
			typ, fw, err := Load(bin.path, bytes.Clone(orig), quiet)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if typ != bin.typ {
				t.Fatalf("detected %s, want %s", typ, bin.typ)
			}
			if _, err := strictLoad(bin.typ, bytes.Clone(orig)); err != nil {
				t.Errorf("strict load: %v", err)
			}
			if fw.Version() == "" {
				t.Error("no software version")
			}
			if fw.Count() == 0 {
				t.Fatal("no symbols")
			}
			names := map[string]int{}
			for _, s := range fw.Symbols() {
				if s.Name == "" {
					t.Errorf("symbol #%d has an empty name", s.Number)
				}
				names[s.Name]++
				start, end, ok := flashRange(bin.typ, s, len(orig))
				if !ok {
					if bin.typ == ECU_T5 && s.Address != 0 || bin.typ == ECU_T8 && s.Address != 0 && s.Address < 0x100000 {
						t.Errorf("%s: flash range %X+%d outside the file", s.Name, s.Address, s.Length)
					}
					continue
				}
				if len(s.Bytes()) != int(s.Length) {
					t.Errorf("%s: %d bytes of data, length %d", s.Name, len(s.Bytes()), s.Length)
					continue
				}
				if !bytes.Equal(s.Bytes(), orig[start:end]) {
					t.Errorf("%s: data differs from file bytes at %X", s.Name, start)
				}
			}
			if fw.GetByName(fw.Symbols()[0].Name) == nil {
				t.Error("GetByName does not find the first symbol")
			}
		})
	}
}

// TestFactoryWrite: an unmodified writeback is byte-identical; writing a
// pattern into every flash-backed symbol at once lands exactly at each
// symbol's address and changes nothing else but checksum bytes; the result
// reloads strictly and reads the pattern back; restoring the original data
// gives back the original file.
func TestFactoryWrite(t *testing.T) {
	for _, bin := range factoryBins(t) {
		t.Run(bin.String(), func(t *testing.T) {
			t.Parallel()
			orig := readBin(t, bin.path)
			_, fw, err := Load(bin.path, bytes.Clone(orig), quiet)
			if err != nil {
				t.Fatal(err)
			}
			w := fw.(writer)

			out, err := w.Byte()
			if err != nil {
				t.Fatalf("byte: %v", err)
			}
			if !bytes.Equal(out, orig) {
				t.Fatalf("unmodified writeback differs from the file at %d positions", countDiff(out, orig))
			}

			// Flash-backed symbols, skipping partial overlaps (they cannot all
			// hold their own pattern); identical ranges share one pattern.
			type rng struct {
				start, end int
				sym        *Symbol
			}
			var ranges []rng
			for _, s := range fw.Symbols() {
				if start, end, ok := flashRange(bin.typ, s, len(orig)); ok {
					ranges = append(ranges, rng{start, end, s})
				}
			}
			covered := make([]bool, len(orig))
			slotStart, slotEnd := checksumSlot(bin.typ, fw, len(orig))
			var written []rng
			for _, r := range ranges {
				partial := false
				for _, o := range ranges {
					if o.start < r.end && r.start < o.end && (o.start != r.start || o.end != r.end) {
						partial = true
						break
					}
				}
				for i := r.start; i < r.end; i++ {
					covered[i] = true
				}
				if partial || (r.start < slotEnd && slotStart < r.end) {
					continue
				}
				data := make([]byte, r.end-r.start)
				for i := range data {
					data[i] = pattern(r.start + i)
				}
				if err := r.sym.SetData(data); err != nil {
					t.Fatalf("%s: %v", r.sym.Name, err)
				}
				written = append(written, r)
			}
			if len(written) == 0 {
				t.Fatal("no flash-backed symbol to write")
			}

			out, err = w.Byte()
			if err != nil {
				t.Fatalf("byte after edit: %v", err)
			}
			if len(out) != len(orig) {
				t.Fatalf("size changed to %d", len(out))
			}
			for _, r := range written {
				for i := r.start; i < r.end; i++ {
					if out[i] != pattern(i) {
						t.Errorf("%s: byte %X is %02X, want %02X", r.sym.Name, i, out[i], pattern(i))
						break
					}
				}
			}
			var outside []int
			for i := range out {
				if !covered[i] && out[i] != orig[i] {
					outside = append(outside, i)
				}
			}
			if len(outside) > checksumBudget {
				t.Errorf("%d bytes outside every symbol changed (first at %X), want at most %d", len(outside), outside[0], checksumBudget)
			}

			saved := filepath.Join(t.TempDir(), filepath.Base(bin.path))
			if err := fw.Save(saved); err != nil {
				t.Fatalf("save: %v", err)
			}
			data := readBin(t, saved)
			if !bytes.Equal(data, out) {
				t.Error("saved file differs from Byte()")
			}
			re, err := strictLoad(bin.typ, data)
			if err != nil {
				t.Fatalf("strict reload: %v", err)
			}
			// Writing map data must not disturb the symbol table itself.
			if a, b := fw.Symbols(), re.Symbols(); len(a) != len(b) {
				t.Errorf("reload found %d symbols, want %d", len(b), len(a))
			} else {
				for i := range a {
					if a[i].Name != b[i].Name || a[i].Address != b[i].Address || a[i].Length != b[i].Length {
						t.Errorf("symbol #%d changed by the writeback: %s -> %s", i, a[i], b[i])
						break
					}
				}
			}
			index := map[*Symbol]int{}
			for i, s := range fw.Symbols() {
				index[s] = i
			}
			for _, r := range written {
				if index[r.sym] >= len(re.Symbols()) {
					continue // reported above
				}
				s := re.Symbols()[index[r.sym]]
				if !bytes.Equal(s.Bytes(), out[r.start:r.end]) {
					t.Errorf("%s: reloaded data differs from the written pattern", r.sym.Name)
				}
			}

			// Restore and expect the original file back, checksums included.
			for _, r := range written {
				if err := r.sym.SetData(bytes.Clone(orig[r.start:r.end])); err != nil {
					t.Fatal(err)
				}
			}
			out, err = w.Byte()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out, orig) {
				t.Errorf("restored writeback differs from the file at %d positions, first at %X", countDiff(out, orig), firstDiff(out, orig))
			}
		})
	}
}

// TestFactoryValues: every symbol's values encode back to its own bytes, and
// a value moved by one step reads back as written, the way the map editors
// save.
func TestFactoryValues(t *testing.T) {
	for _, bin := range factoryBins(t) {
		t.Run(bin.String(), func(t *testing.T) {
			t.Parallel()
			_, fw, err := Load(bin.path, readBin(t, bin.path), quiet)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range fw.Symbols() {
				orig := s.Bytes()
				if s.Length == 0 || len(orig) != int(s.Length) {
					continue
				}
				vals := s.Float64s()
				if got := s.EncodeFloat64s(vals); !bytes.Equal(got, orig) {
					t.Errorf("%s: values encode to % X, want % X", s.Name, got, orig)
					continue
				}
				// one step up, or down when the value is at the top of its range
				i := len(vals) / 2
				edited := slices.Clone(vals)
				edited[i] += s.Correctionfactor
				if _, hi := s.valueRange(); s.Ints()[i] == hi {
					edited[i] = vals[i] - s.Correctionfactor
				}
				if err := s.SetData(s.EncodeFloat64s(edited)); err != nil {
					t.Fatalf("%s: %v", s.Name, err)
				}
				if got := s.Float64s()[i]; math.Abs(got-edited[i]) > s.Correctionfactor/2 {
					t.Errorf("%s[%d]: wrote %g, reads %g", s.Name, i, edited[i], got)
				}
				_ = s.SetData(orig)
			}
		})
	}
}

// TestT8AliasedEdit: a T8 edit through one of two symbols sharing the same
// bytes survives the save even when the other one comes later in the table.
func TestT8AliasedEdit(t *testing.T) {
	for _, bin := range factoryBins(t) {
		if bin.typ != ECU_T8 {
			continue
		}
		_, fw, err := Load(bin.path, readBin(t, bin.path), quiet)
		if err != nil {
			t.Fatal(err)
		}
		f := fw.(*T8File)
		i := slices.IndexFunc(f.Symbols(), func(s *Symbol) bool {
			return strings.Contains(s.Name, "Cal.") && s.Length >= 2 && len(s.data) == int(s.Length)
		})
		if i < 0 {
			t.Fatal("no calibration symbol")
		}
		s := f.Symbols()[i]
		twin := &Symbol{Name: "twin", Number: -1, Address: s.Address, Length: s.Length, Type: s.Type, data: bytes.Clone(s.data)}
		f.Add(twin)
		edited := bytes.Clone(s.data)
		edited[0] ^= 0xFF
		if err := s.SetData(edited); err != nil {
			t.Fatal(err)
		}
		out, err := f.Byte()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out[s.Address:int(s.Address)+len(edited)], edited) {
			t.Errorf("%s: edit lost on save", s.Name)
		}
		if !bytes.Equal(twin.Bytes(), edited) {
			t.Errorf("%s: the symbol sharing its bytes doesn't show the edit", s.Name)
		}
		return // one binary is enough
	}
	t.Skip("no T8 factory binary")
}

// TestFactoryAliases: when several symbols share one flash range, an edit
// through any one of them must survive the writeback and be visible through
// the others.
func TestFactoryAliases(t *testing.T) {
	for _, bin := range factoryBins(t) {
		t.Run(bin.String(), func(t *testing.T) {
			t.Parallel()
			orig := readBin(t, bin.path)
			_, fw, err := Load(bin.path, bytes.Clone(orig), quiet)
			if err != nil {
				t.Fatal(err)
			}
			groups := map[[2]int][]*Symbol{}
			for _, s := range fw.Symbols() {
				if start, end, ok := flashRange(bin.typ, s, len(orig)); ok {
					groups[[2]int{start, end}] = append(groups[[2]int{start, end}], s)
				}
			}
			var keys [][2]int
			for k, g := range groups {
				if len(g) > 1 {
					keys = append(keys, k)
				}
			}
			if len(keys) == 0 {
				t.Skip("no aliased symbols")
			}
			sort.Slice(keys, func(i, j int) bool { return keys[i][0] < keys[j][0] })
			for _, k := range keys {
				g := groups[k]
				edited := bytes.Clone(g[0].Bytes())
				edited[0] ^= 0xFF
				if err := g[0].SetData(edited); err != nil {
					t.Fatal(err)
				}
				out, err := fw.(writer).Byte()
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(out[k[0]:k[1]], edited) {
					t.Errorf("%s @%X: edit lost, overwritten by %s", g[0].Name, k[0], names(g[1:]))
				}
				for _, s := range g[1:] {
					if !bytes.Equal(s.Bytes(), edited) {
						t.Errorf("%s: still holds the old bytes after %s was edited", s.Name, g[0].Name)
					}
				}
			}
		})
	}
}

// TestFactoryChecksum: a binary with a wrong checksum is refused by the strict
// loaders and either refused or repaired by Load; UpdateChecksum on the
// damaged image reproduces a verifiable checksum.
func TestFactoryChecksum(t *testing.T) {
	for _, bin := range factoryBins(t) {
		t.Run(bin.String(), func(t *testing.T) {
			t.Parallel()
			orig := readBin(t, bin.path)
			_, fw, err := Load(bin.path, bytes.Clone(orig), quiet)
			if err != nil {
				t.Fatal(err)
			}
			// Flip a byte inside a checksummed map so the stored checksum no longer matches.
			var target *Symbol
			for _, s := range fw.Symbols() {
				if start, _, ok := flashRange(bin.typ, s, len(orig)); ok && s.Length >= 16 && checksumCovers(fw, start) {
					target = s
					break
				}
			}
			if target == nil {
				t.Skip("no checksummed flash map")
			}
			start, _, _ := flashRange(bin.typ, target, len(orig))
			data := bytes.Clone(orig)
			data[start] ^= 0xFF
			if _, err := strictLoad(bin.typ, bytes.Clone(data)); err == nil {
				t.Errorf("strict load accepted a wrong checksum after editing %s", target.Name)
			}
			_, repaired, err := Load(bin.path, bytes.Clone(data), quiet)
			if err != nil {
				t.Logf("Load refuses the damaged image: %v", err)
				return
			}
			out, err := repaired.(writer).Byte()
			if err != nil {
				t.Fatalf("byte: %v", err)
			}
			if out[start] != data[start] {
				t.Errorf("repair reverted the edited byte")
			}
			if _, err := strictLoad(bin.typ, bytes.Clone(out)); err != nil {
				t.Errorf("repaired image does not verify: %v", err)
			}
		})
	}
}

// TestFactoryCorrupt: damaged images fail cleanly, never panic. One bin per
// ECU type is enough.
func TestFactoryCorrupt(t *testing.T) {
	seen := map[ECUType]bool{}
	for _, bin := range factoryBins(t) {
		if seen[bin.typ] {
			continue
		}
		seen[bin.typ] = true
		t.Run(bin.String(), func(t *testing.T) {
			orig := readBin(t, bin.path)
			mustFail := func(name string, data []byte) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("%s: panic: %v", name, r)
					}
				}()
				if _, _, err := Load(bin.path, data, quiet); err == nil {
					t.Errorf("%s: loaded without error", name)
				}
			}
			mustFail("truncated", bytes.Clone(orig[:len(orig)-1]))
			mustFail("zeroed", make([]byte, len(orig)))
			r := rand.New(rand.NewPCG(uint64(len(orig)), 2))
			for i := 0; i < 3; i++ {
				data := make([]byte, len(orig))
				for j := range data {
					data[j] = byte(r.Uint32())
				}
				mustFail(fmt.Sprintf("random %d", i), bytes.Clone(data))
				copy(data, orig[:0x100]) // keep the magic bytes so parsing goes deeper
				mustFail(fmt.Sprintf("random %d with header", i), data)
			}
			// Original with the second half scrambled: header, magic and
			// early tables intact, symbol/checksum areas garbage.
			data := bytes.Clone(orig)
			for j := len(data) / 2; j < len(data); j++ {
				data[j] = byte(r.Uint32())
			}
			mustFail("scrambled tail", data)
		})
	}
}

func firstDiff(a, b []byte) int {
	for i := range a {
		if a[i] != b[i] {
			return i
		}
	}
	return -1
}

func countDiff(a, b []byte) int {
	n := 0
	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}
	return n
}

func names(syms []*Symbol) string {
	var out []string
	for _, s := range syms {
		out = append(out, s.Name)
	}
	return fmt.Sprint(out)
}

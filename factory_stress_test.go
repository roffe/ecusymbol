package symbol

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// TestFactorySaveSequence edits, saves and reloads every binary over several
// rounds, the way a user saves map after map, and checks each saved file
// against a byte model: edited symbols hold what was written, every other
// symbol byte is unchanged, and only bytes outside every symbol (checksums,
// footer) may move. Rounds alternate between going on with the loaded file
// and with the reloaded saved one. A save without edits must not change the
// file, and putting the original data back must give the original file.
func TestFactorySaveSequence(t *testing.T) {
	for _, bin := range factoryBins(t) {
		t.Run(bin.String(), func(t *testing.T) {
			t.Parallel()
			orig := readBin(t, bin.path)
			_, fw, err := Load(bin.path, bytes.Clone(orig), quiet)
			if err != nil {
				t.Fatal(err)
			}

			// Edit only symbols that own their bytes: no other symbol overlaps
			// them and they don't cover the stored checksum. Aliases have
			// their own test.
			n := len(orig)
			cover := make([]int, n)
			rng := map[int][2]int{}
			for i, s := range fw.Symbols() {
				if start, end, ok := flashRange(bin.typ, s, n); ok {
					rng[i] = [2]int{start, end}
					for j := start; j < end; j++ {
						cover[j]++
					}
				}
			}
			slotStart, slotEnd := checksumSlot(bin.typ, fw, n)
			var own []int
			for i, r := range rng {
				shared := r[0] < slotEnd && slotStart < r[1]
				for j := r[0]; j < r[1] && !shared; j++ {
					shared = cover[j] > 1
				}
				if !shared {
					own = append(own, i)
				}
			}
			if len(own) == 0 {
				t.Skip("no symbol owns its bytes")
			}
			slices.Sort(own)

			r := rand.New(rand.NewPCG(uint64(crc32.ChecksumIEEE([]byte(filepath.Base(bin.path)))), 1))
			model := bytes.Clone(orig)
			edited := map[int]bool{}
			path := filepath.Join(t.TempDir(), filepath.Base(bin.path))
			for round := range 6 {
				for range 8 {
					i := own[r.IntN(len(own))]
					data := make([]byte, rng[i][1]-rng[i][0])
					for j := range data {
						data[j] = byte(r.Uint32())
					}
					if err := fw.Symbols()[i].SetData(data); err != nil {
						t.Fatal(err)
					}
					copy(model[rng[i][0]:], data)
					edited[i] = true
				}
				if err := fw.Save(path); err != nil {
					t.Fatalf("round %d: save: %v", round, err)
				}
				out := readBin(t, path)
				if len(out) != n {
					t.Fatalf("round %d: saved %d bytes, want %d", round, len(out), n)
				}
				moved := 0
				for j := range out {
					if out[j] == model[j] {
						continue
					}
					if cover[j] > 0 && (j < slotStart || j >= slotEnd) {
						var in []string
						for i, g := range rng {
							if j >= g[0] && j < g[1] {
								in = append(in, fmt.Sprintf("%s@%X+%d", fw.Symbols()[i].Name, g[0], g[1]-g[0]))
							}
						}
						t.Fatalf("round %d: byte %X in %v is %02X, want %02X", round, j, in, out[j], model[j])
					}
					moved++
				}
				if moved > checksumBudget {
					t.Errorf("round %d: %d bytes outside every symbol changed, want at most %d", round, moved, checksumBudget)
				}
				copy(model, out)

				re, err := strictLoad(bin.typ, bytes.Clone(out))
				if err != nil {
					t.Fatalf("round %d: strict reload: %v", round, err)
				}
				a, b := fw.Symbols(), re.Symbols()
				if len(a) != len(b) {
					t.Fatalf("round %d: reload found %d symbols, want %d", round, len(b), len(a))
				}
				for i := range a {
					if a[i].Name != b[i].Name || a[i].Address != b[i].Address || a[i].Length != b[i].Length {
						t.Fatalf("round %d: symbol #%d changed by the save: %s -> %s", round, i, a[i], b[i])
					}
					if g, ok := rng[i]; ok && !bytes.Equal(b[i].Bytes(), out[g[0]:g[1]]) {
						t.Fatalf("round %d: %s reads other bytes than the file holds", round, b[i].Name)
					}
				}
				if round%2 == 1 { // go on with the saved file, opened like txlogger opens it
					if _, fw, err = Load(path, bytes.Clone(out), quiet); err != nil {
						t.Fatalf("round %d: reload: %v", round, err)
					}
				}
			}

			if err := fw.Save(path); err != nil {
				t.Fatal(err)
			}
			if out := readBin(t, path); !bytes.Equal(out, model) {
				t.Errorf("a save without edits changed %d bytes, first at %X", countDiff(out, model), firstDiff(out, model))
			}

			for i := range edited {
				if err := fw.Symbols()[i].SetData(bytes.Clone(orig[rng[i][0]:rng[i][1]])); err != nil {
					t.Fatal(err)
				}
			}
			out, err := fw.(writer).Byte()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out, orig) {
				t.Errorf("the original data back gives %d bytes off the original file, first at %X", countDiff(out, orig), firstDiff(out, orig))
			}
		})
	}
}

// TestFactoryDamage damages the structures each loader parses (header,
// footer, symbol and name tables and the pointers to them) in every binary.
// Loading the result, reading every symbol and saving must fail cleanly or
// work: never panic, never hang. Run it under a memory cap like the rest of
// the factory suite; a loader that sizes an allocation from a damaged field
// shows up as the cap killing the run.
func TestFactoryDamage(t *testing.T) {
	for _, bin := range factoryBins(t) {
		t.Run(bin.String(), func(t *testing.T) {
			t.Parallel()
			orig := readBin(t, bin.path)
			_, fw, err := Load(bin.path, bytes.Clone(orig), quiet)
			if err != nil {
				t.Fatal(err)
			}
			regions, pointers := structures(fw, orig)
			r := rand.New(rand.NewPCG(uint64(crc32.ChecksumIEEE([]byte(filepath.Base(bin.path)))), 2))
			try := func(what string, damage func([]byte)) {
				data := bytes.Clone(orig)
				damage(data)
				if err := exercise(bin.path, data); err != nil {
					t.Errorf("%s: %v", what, err)
				}
			}
			for _, reg := range regions {
				lo, hi := max(reg.lo, 0), min(reg.hi, len(orig))
				if lo >= hi {
					continue
				}
				for k := range 6 {
					p, v := lo+r.IntN(hi-lo), byte(r.Uint32())
					try(fmt.Sprintf("%s byte %X = %02X (%d)", reg.name, p, v, k), func(d []byte) { d[p] = v })
				}
				try(reg.name+" burst", func(d []byte) {
					for range 8 {
						d[lo+r.IntN(hi-lo)] = byte(r.Uint32())
					}
				})
				try(reg.name+" zeroed", func(d []byte) { clear(d[lo:hi]) })
				try(reg.name+" erased", func(d []byte) {
					for j := lo; j < hi; j++ {
						d[j] = 0xFF
					}
				})
			}
			for _, p := range pointers {
				if p.at < 0 || p.at+4 > len(orig) {
					continue
				}
				for _, v := range []uint32{0, 1, 0x7FFFFFFF, 0xFFFFFFFF, uint32(len(orig)), uint32(len(orig) - 1)} {
					try(fmt.Sprintf("%s = %X", p.name, v), func(d []byte) {
						d[p.at], d[p.at+1], d[p.at+2], d[p.at+3] = byte(v>>24), byte(v>>16), byte(v>>8), byte(v)
					})
				}
			}
		})
	}
}

type region struct {
	name   string
	lo, hi int
}

type pointer struct {
	name string
	at   int
}

// structures lists what a loader parses in this binary: regions to damage
// byte by byte and 4-byte pointer/length fields to set to extreme values.
func structures(fw FirmwareFile, data []byte) ([]region, []pointer) {
	n := len(data)
	regions := []region{{"header", 0, 0x100}, {"tail", n - 0x200, n}}
	var pointers []pointer
	switch f := fw.(type) {
	case *T5File:
		s := f.m_symboltablestartaddress
		regions = append(regions, region{"symbol table start", s, s + 0x400})
	case *T7File:
		if packed, at, names, length, err := GetT7Offsets(data, quiet); err == nil && packed {
			regions = append(regions,
				region{"address table", at, at + 0x200},
				region{"name table start", names, names + 0x100},
				region{"name table end", names + length - 0x100, names + length})
			pointers = append(pointers, pointer{"name table offset", at}, pointer{"name table length", at + 4})
		} else if off, err := getSymbolListOffSet(data); err == nil {
			regions = append(regions, region{"symbol list", off, off + 0x400})
		}
	case *T8File:
		if end, err := GetEndOfSymbolTable(data); err == nil {
			if nq, err := GetFirstNqStringFromOffset(data, end); err == nil {
				names, _ := GetAddressFromOffset(data, nq)
				length, _ := GetLengthFromOffset(data, nq+4)
				regions = append(regions,
					region{"address table", nq + 28, nq + 28 + 0x200},
					region{"name table start", names, names + 0x100},
					region{"name table end", names + length - 0x100, names + length})
				pointers = append(pointers, pointer{"name table offset", nq}, pointer{"name table length", nq + 4})
			}
		}
		if off, err := f.GetChecksumAreaOffset(data); err == nil {
			regions = append(regions, region{"checksum area", off, off + 0x40})
		}
		pointers = append(pointers, pointer{"checksum area pointer", 0x20140})
	}
	return regions, pointers
}

// exercise does what txlogger does with a file: load it, read every symbol's
// values and save it. It reports a panic, or a load that hangs.
func exercise(name string, data []byte) error {
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic: %v", r)
			}
		}()
		_, fw, err := Load(name, data, quiet)
		if err == nil {
			for _, s := range fw.Symbols() {
				if len(s.data) == int(s.Length) {
					s.Float64s()
				}
			}
			if w, ok := fw.(writer); ok {
				_, _ = w.Byte()
			}
		}
		done <- nil
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(20 * time.Second):
		return fmt.Errorf("still busy after 20s")
	}
}

// FuzzLoad feeds mutated binaries to the loaders, seeded with one factory
// binary per ECU. A plain test run only replays the seeds; fuzz it under a
// memory cap with
//
//	go test -run '^$' -fuzz FuzzLoad -fuzztime 10m
func FuzzLoad(f *testing.F) {
	for _, d := range testdataDirs {
		if files, _ := filepath.Glob(filepath.Join(d.dir, "*.[bB][iI][nN]")); len(files) > 0 {
			if data, err := os.ReadFile(files[0]); err == nil {
				f.Add(data)
			}
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if err := exercise("fuzz.bin", data); err != nil {
			t.Fatal(err)
		}
	})
}

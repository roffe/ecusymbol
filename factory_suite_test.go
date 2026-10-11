package symbol

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// suiteSymbol is one symbol as T7Suite or T8Suite reads it.
type suiteSymbol struct {
	name        string
	flash, sram int64
	length      int
}

// readSuiteDump parses a cmd/suiteoracle dump: file name -> symbols.
func readSuiteDump(t *testing.T, path string) map[string][]suiteSymbol {
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("no suite dump: %v", err)
	}
	defer f.Close()
	out := map[string][]suiteSymbol{}
	var cur string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		p := strings.Split(sc.Text(), "\t")
		switch {
		case p[0] == "FILE" && len(p) == 2:
			cur = p[1]
			out[cur] = []suiteSymbol{}
		case p[0] == "SYM" && len(p) == 8:
			flash, _ := strconv.ParseInt(p[4], 16, 64)
			sram, _ := strconv.ParseInt(p[5], 16, 64)
			length, _ := strconv.Atoi(p[6])
			out[cur] = append(out[cur], suiteSymbol{p[2], flash, sram, length})
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestFactorySuiteSymbols compares every T5, T7 and T8 symbol table with what
// T5Suite, T7Suite and T8Suite read from the same binary (testdata/suite, made with
// cmd/suiteoracle): same symbols in the same order, same names, addresses
// and lengths, and the same flash location for everything stored in flash.
// Two differences are deliberate: ecusymbol names T7 symbols T7Suite leaves
// as "Symbolnumber N" (its built-in name lists), and a T7 RAM symbol gets a
// flash image only inside a start-up calconst->caldata copy window, where
// T7Suite maps every RAM symbol to flash or to none. T7Suite has no flash
// image for the symbols in those windows, so their flash location is checked
// only by the start-up code t7CalWindows reads it from.
func TestFactorySuiteSymbols(t *testing.T) {
	dumps := map[ECUType]map[string][]suiteSymbol{
		ECU_T5: readSuiteDump(t, "testdata/suite/t5.tsv"),
		ECU_T7: readSuiteDump(t, "testdata/suite/t7.tsv"),
		ECU_T8: readSuiteDump(t, "testdata/suite/t8.tsv"),
	}
	for _, bin := range factoryBins(t) {
		want, ok := dumps[bin.typ][filepath.Base(bin.path)]
		if !ok {
			continue
		}
		t.Run(bin.String(), func(t *testing.T) {
			t.Parallel()
			_, fw, err := Load(bin.path, readBin(t, bin.path), quiet)
			if err != nil {
				t.Fatal(err)
			}
			got := fw.Symbols()
			if len(got) != len(want) {
				t.Fatalf("%d symbols, the suite reads %d", len(got), len(want))
			}
			bad := 0
			for i, g := range got {
				w := want[i]
				var why []string
				named := g.Name == w.name || bin.typ == ECU_T7 && strings.HasPrefix(w.name, "Symbolnumber ")
				if !named {
					why = append(why, fmt.Sprintf("name %q", g.Name))
				}
				if int(g.Length) != w.length {
					why = append(why, fmt.Sprintf("length %d, suite %d", g.Length, w.length))
				}
				switch bin.typ {
				case ECU_T5:
					if int64(g.SramOffset) != w.sram {
						why = append(why, fmt.Sprintf("SRAM %X, suite %X", g.SramOffset, w.sram))
					}
					// T5Suite gives the ECU address; the file starts at 0x80000 minus its size
					if flash := w.flash - int64(0x80000-len(fw.(*T5File).data)); w.flash != 0 && int64(g.Address) != flash {
						why = append(why, fmt.Sprintf("flash %X, suite %X", g.Address, flash))
					} else if w.flash == 0 && g.Address != 0 {
						why = append(why, fmt.Sprintf("flash %X, suite none", g.Address))
					}
				case ECU_T7:
					if int64(g.Address) != w.sram {
						why = append(why, fmt.Sprintf("address %X, suite %X", g.Address, w.sram))
					}
					switch {
					case g.Number == 0: // the symbol table itself
					case g.Address < T7SRAMAddress:
						if w.flash != int64(g.Address) {
							why = append(why, fmt.Sprintf("flash %X, suite %X", g.Address, w.flash))
						}
					case g.SramOffset != 0: // in a copy window: the suite has no flash image or the same one
						if f := int64(g.Address - g.SramOffset); w.flash != f && w.flash != w.sram {
							why = append(why, fmt.Sprintf("flash %X, suite %X", f, w.flash))
						}
					}
				case ECU_T8:
					addr := w.sram
					if addr == 0 {
						addr = w.flash
					}
					if int64(g.Address) != addr {
						why = append(why, fmt.Sprintf("address %X, suite %X", g.Address, addr))
					}
				}
				if len(why) > 0 {
					if bad < 10 {
						t.Errorf("#%d %s: %s", i, w.name, strings.Join(why, ", "))
					}
					bad++
				}
			}
			if bad > 10 {
				t.Errorf("%d symbols differ in all", bad)
			}
		})
	}
}

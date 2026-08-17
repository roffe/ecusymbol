package symbol

import (
	"os"
	"strings"
	"testing"
)

// The AW55-50 definitions are mined from the firmware rather than read out of
// it, so this loads real dumps and checks the maps still line up with the data:
// every symbol must have its bytes, and a 2-D map's axes must match its shape.
func TestAW55(t *testing.T) {
	for _, f := range []struct{ path, family string }{
		{"/home/roffe/OneDrive/T8 Source/TCM/AW55/NG9-3_TCM_20260807_155756_2006-05-12_B207R.bin", "Y867"},
		{"/home/roffe/devel/GTISSPS/t2w_spssbdb/TCM/ecu1539_9938301295_20040406/_image.bin", "Y802"},
	} {
		data, err := os.ReadFile(f.path)
		if err != nil {
			t.Skipf("%s: %v", f.family, err)
		}
		ecu, fw, err := Load(f.path, data, func(string) {})
		if err != nil {
			t.Fatalf("%s: %v", f.family, err)
		}
		if ecu != ECU_AW55 {
			t.Fatalf("%s: detected %s, want AW55", f.family, ecu)
		}
		if got := AW55Family(data); got != f.family {
			t.Fatalf("family %q, want %q", got, f.family)
		}
		maps2d := 0
		for _, s := range fw.Symbols() {
			if len(s.Bytes()) != int(s.Length) {
				t.Fatalf("%s: %s has %d bytes, want %d", f.family, s.Name, len(s.Bytes()), s.Length)
			}
			ax := GetInfo(ECU_AW55, s.Name)
			if ax.Y == "" {
				continue
			}
			maps2d++
			x, y, z, _, _, _, err := fw.GetXYZ(ax.X, ax.Y, ax.Z)
			if err != nil {
				t.Fatalf("%s: %s: %v", f.family, s.Name, err)
			}
			if len(x)*len(y) != len(z) {
				t.Fatalf("%s: %s is %dx%d but holds %d values", f.family, s.Name, len(y), len(x), len(z))
			}
		}
		t.Logf("%s: %d symbols, %d two-dimensional maps", f.family, fw.Count(), maps2d)

		// The shift schedule is de-interleaved out of a record format, so check
		// it survived: every curve needs as many thresholds as breakpoints, and
		// every upshift must still sit above its matching downshift.
		shifts := 0
		for _, s := range fw.Symbols() {
			if !strings.HasPrefix(s.Name, "ShiftUp-") {
				continue
			}
			shifts++
			x := fw.GetByName(GetInfo(ECU_AW55, s.Name).X)
			if x == nil || len(x.Ints()) != len(s.Ints()) {
				t.Fatalf("%s: %s has no matching load axis", f.family, s.Name)
			}
			dn := fw.GetByName(strings.Replace(s.Name, "ShiftUp-", "ShiftDn-", 1))
			if dn == nil {
				t.Fatalf("%s: %s has no downshift partner", f.family, s.Name)
			}
			for i, up := range s.Ints() {
				if up < dn.Ints()[i] {
					t.Fatalf("%s: %s point %d: up %d below down %d", f.family, s.Name, i, up, dn.Ints()[i])
				}
			}
		}
		if shifts == 0 {
			t.Fatalf("%s: no shift curves found", f.family)
		}
		t.Logf("%s: %d upshift curves", f.family, shifts)
	}
}

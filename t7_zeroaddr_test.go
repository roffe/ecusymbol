package symbol

import (
	"os"
	"testing"
)

// MapTun-style binaries zero the address of protected maps in the address
// table. binaryPacked must reconstruct them from prev+prev.Length, otherwise
// the map reads from offset 0 (the vector table) and renders as garbage.
func TestZeroedAddressRepair(t *testing.T) {
	data, err := os.ReadFile(".tmp/Ali_Maptun.bin")
	if err != nil {
		t.Skip("test binary not present:", err)
	}
	f, err := NewT7File(data, WithT7PrintFunc(func(string) {}))
	if err != nil {
		t.Fatal(err)
	}
	// addresses cross-checked against T7Suite's EU09F01C.xml FLASHADDRESS
	for name, want := range map[string]uint32{
		"BFuelCal.Map":          0x743A,
		"BFuelCal.E85Map":       0x755A,
		"IgnNormCal.Map":        0x9BAC, // len 0x240, exercises the odd-address bump
		"TorqueCal.M_IgnLimMap": 0x501E,
		"BstKnkCal.MaxAirmass":  0x3D16,
	} {
		sym := f.GetByName(name)
		if sym == nil {
			t.Errorf("%s missing", name)
			continue
		}
		if sym.Address != want {
			t.Errorf("%s address = %08X, want %08X", name, sym.Address, want)
		}
	}
}

// First members of a struct follow the zero-length XxxCalType entry, so
// prev+prev.Length can't place them; they're derived from the next member.
// Shapes taken from EC0YS3SC.50A MapTun (nordenfors), checked against stock 5387998.
func TestRepairZeroedAddresses(t *testing.T) {
	syms := []*Symbol{
		{Name: "SymbolNames", Address: 0x54220, Length: 8},
		{Name: "BFuelCal.rampFac", Address: 0x6988, Length: 1},
		{Name: "InjCorrCalType"},
		{Name: "InjCorrCal.InjectorConst", Length: 2}, // zeroed first member
		{Name: "InjCorrCal.BattCorrTab", Length: 22},  // zeroed, prev zeroed too
		{Name: "InjCorrCal.BattCorrSP", Address: 0x69A2, Length: 22},
		{Name: "KnkFuelProt.WeightFac2", Address: 0xF07178, Length: 2}, // RAM prev
		{Name: "KnkFuelCalType"},
		{Name: "KnkFuelCal.EnrichmentMap", Length: 0x240},
		{Name: "KnkFuelCal.fi_MapMaxOffset", Address: 0x7A94, Length: 0x240},
		{Name: "OddCalType"},
		{Name: "OddCal.Flag", Length: 1}, // first member, one pad byte before next
		{Name: "OddCal.Word", Address: 0x5000, Length: 2},
		{Name: "IgnNormCal.m_AirXSP", Address: 0x9B8B, Length: 0x21},
		{Name: "IgnNormCal.Map", Length: 0x240}, // forward repair + odd bump
		{Name: "LoneCalType"},
		{Name: "LoneCal.Only", Length: 4}, // no neighbour to derive from
		{Name: "NextType"},
	}
	repairZeroedAddresses(syms)
	for name, want := range map[string]uint32{
		"InjCorrCal.InjectorConst": 0x698A,
		"InjCorrCal.BattCorrTab":   0x698C,
		"KnkFuelCal.EnrichmentMap": 0x7854,
		"OddCal.Flag":              0x4FFE,
		"IgnNormCal.Map":           0x9BAC,
		"LoneCal.Only":             0,
	} {
		for _, s := range syms {
			if s.Name == name && s.Address != want {
				t.Errorf("%s address = %X, want %X", name, s.Address, want)
			}
		}
	}
}

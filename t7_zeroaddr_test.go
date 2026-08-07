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

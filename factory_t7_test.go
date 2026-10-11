package symbol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

// t7Option is one Firmware information switch: whether the binary has it,
// its state and how to set it. set gets the binary's original info so a
// switch can keep what it shares with another one. Switches that fold
// several values into one (fast throttle's three flags, second lambda's
// I_LowLim, a sensor flag stored as other than 0/1) may come back with
// different values in their own symbols after a round trip.
type t7Option struct {
	name    string
	syms    []string // the symbols it writes, if any
	present func(T7FirmwareInfo) bool
	get     func(T7FirmwareInfo) bool
	set     func(t7 *T7File, on bool, orig T7FirmwareInfo) error
}

var t7Options = []t7Option{
	{"SID info open", nil,
		func(i T7FirmwareInfo) bool { return i.SIDInfoPresent },
		func(i T7FirmwareInfo) bool { return i.SIDInfoOpen },
		func(t7 *T7File, on bool, _ T7FirmwareInfo) error { return t7.SetSIDInfoOpen(on) }},
	{"torque limiters", []string{"TorqueCal.ST_Loop"},
		func(i T7FirmwareInfo) bool { return i.TorqueLimitersPresent },
		func(i T7FirmwareInfo) bool { return i.TorqueLimitersEnabled },
		func(t7 *T7File, on bool, _ T7FirmwareInfo) error { return t7.SetTorqueLimiters(on) }},
	{"OBDII", []string{"OBDCal.OBD2Enabled", "OBDCal.EOBDEnabled"},
		func(i T7FirmwareInfo) bool { return i.OBDIIPresent },
		func(i T7FirmwareInfo) bool { return i.OBDIIEnabled },
		func(t7 *T7File, on bool, _ T7FirmwareInfo) error { return t7.SetOBDII(on) }},
	{"second lambda", []string{"LambdaCal.ST_AdapEnable", "O2HeatPostCal.I_LowLim"},
		func(i T7FirmwareInfo) bool { return i.SecondLambdaPresent },
		func(i T7FirmwareInfo) bool { return i.SecondLambdaEnabled },
		func(t7 *T7File, on bool, _ T7FirmwareInfo) error { return t7.SetSecondLambda(on) }},
	{"fast throttle", []string{"EngTipCal.ST_EnableTipin", "EngTipCal.ST_EnableTipou", "EngTipCal.ST_EnableActG2"},
		func(i T7FirmwareInfo) bool { return i.FastThrottlePresent },
		func(i T7FirmwareInfo) bool { return i.FastThrottle },
		func(t7 *T7File, on bool, o T7FirmwareInfo) error { return t7.SetFastThrottle(on, o.ExtraFastThrottle) }},
	{"catalyst light-off", []string{"IdleCal.ST_EnableLOffRpm", "IgnLOffCal.ST_Enable"},
		func(i T7FirmwareInfo) bool { return i.CatalystLightOffPresent },
		func(i T7FirmwareInfo) bool { return i.CatalystLightOff },
		func(t7 *T7File, on bool, _ T7FirmwareInfo) error { return t7.SetCatalystLightOff(on) }},
	{"BioPower", []string{"E85Cal.ST_Enable"},
		func(i T7FirmwareInfo) bool { return i.BioPowerSoftware },
		func(i T7FirmwareInfo) bool { return i.BioPowerEnabled },
		func(t7 *T7File, on bool, _ T7FirmwareInfo) error { return t7.SetBioPower(on) }},
	{"ethanol sensor", []string{"E85Cal.ST_EthanolSensor"},
		func(i T7FirmwareInfo) bool { return i.BioPowerSoftware },
		func(i T7FirmwareInfo) bool { return i.EthanolSensor },
		func(t7 *T7File, on bool, _ T7FirmwareInfo) error { return t7.SetEthanolSensor(on) }},
	{"SID start screen off", nil,
		func(i T7FirmwareInfo) bool { return i.SIDOptionsPresent },
		func(i T7FirmwareInfo) bool { return i.SIDDisableStartScreen },
		func(t7 *T7File, on bool, o T7FirmwareInfo) error {
			return t7.SetSIDOptions(on, o.SIDDisableAdaptionMessages)
		}},
	{"SID adaption messages off", nil,
		func(i T7FirmwareInfo) bool { return i.SIDOptionsPresent },
		func(i T7FirmwareInfo) bool { return i.SIDDisableAdaptionMessages },
		func(t7 *T7File, on bool, o T7FirmwareInfo) error {
			return t7.SetSIDOptions(o.SIDDisableStartScreen, on)
		}},
	{"emission limiting off", nil,
		func(i T7FirmwareInfo) bool { return i.EmissionLimitingPatchPresent },
		func(i T7FirmwareInfo) bool { return i.EmissionLimitingDisabled },
		func(t7 *T7File, on bool, _ T7FirmwareInfo) error { return t7.SetEmissionLimitingDisabled(on) }},
}

func t7Bins(t *testing.T) []factoryBin {
	var out []factoryBin
	for _, b := range factoryBins(t) {
		if b.typ == ECU_T7 {
			out = append(out, b)
		}
	}
	if len(out) == 0 {
		t.Skip("no T7 factory binaries")
	}
	return out
}

func loadT7(t *testing.T, path string, data []byte) *T7File {
	t.Helper()
	_, fw, err := Load(path, bytes.Clone(data), quiet)
	if err != nil {
		t.Fatal(err)
	}
	return fw.(*T7File)
}

// TestFactoryT7FirmwareInfo flips every Firmware information switch a T7
// binary has, saves and reloads it strictly: the switch reads back flipped,
// no other switch moves, only a few bytes besides checksums and footer
// change, and flipping it back gives the original file.
func TestFactoryT7FirmwareInfo(t *testing.T) {
	for _, bin := range t7Bins(t) {
		t.Run(bin.String(), func(t *testing.T) {
			t.Parallel()
			orig := readBin(t, bin.path)
			info := loadT7(t, bin.path, orig).GetInfo()
			for _, o := range t7Options {
				if !o.present(info) {
					continue
				}
				t7 := loadT7(t, bin.path, orig)
				was := o.get(info)
				if err := o.set(t7, !was, info); errors.Is(err, ErrOptionNotAvailable) {
					continue
				} else if err != nil {
					t.Errorf("%s: %v", o.name, err)
					continue
				}
				out, err := t7.Byte()
				if err != nil {
					t.Fatalf("%s: %v", o.name, err)
				}
				re, err := strictLoad(ECU_T7, bytes.Clone(out))
				if err != nil {
					t.Fatalf("%s: strict reload: %v", o.name, err)
				}
				ri := re.(*T7File).GetInfo()
				if o.get(ri) == was {
					t.Errorf("%s: set to %v, reads back %v", o.name, !was, was)
				}
				for _, p := range t7Options {
					if p.name != o.name && p.present(info) && p.get(ri) != p.get(info) {
						t.Errorf("%s: also changed %s", o.name, p.name)
					}
				}
				if n := countDiff(out, orig); n > checksumBudget {
					t.Errorf("%s: %d bytes changed", o.name, n)
				}
				if err := o.set(t7, was, info); err != nil {
					t.Fatalf("%s back: %v", o.name, err)
				}
				back, err := t7.Byte()
				if err != nil {
					t.Fatal(err)
				}
				// besides its own symbols, only checksums may differ: the stored
				// checksum slot and bytes outside every symbol (footer)
				cs, ce := checksumSlot(ECU_T7, t7, len(orig))
				for j := range back {
					if back[j] != orig[j] && !inSymbols(t7, o.syms, j) && (j < cs || j >= ce) && inAnySymbol(t7, j) {
						t.Errorf("%s: set back, byte %X is %02X, was %02X", o.name, j, back[j], orig[j])
						break
					}
				}
			}
		})
	}
}

// inSymbols reports whether file offset pos lies in one of the named symbols.
func inSymbols(fw FirmwareFile, names []string, pos int) bool {
	for _, n := range names {
		if s := fw.GetByName(n); s != nil {
			if start, end, ok := flashRange(ECU_T7, s, 0x80000); ok && pos >= start && pos < end {
				return true
			}
		}
	}
	return false
}

// inAnySymbol reports whether file offset pos lies in any symbol's flash range.
func inAnySymbol(fw FirmwareFile, pos int) bool {
	for _, s := range fw.Symbols() {
		if start, end, ok := flashRange(ECU_T7, s, 0x80000); ok && pos >= start && pos < end {
			return true
		}
	}
	return false
}

// TestFactoryT7DateAndESP sets the programming date and the ESP calibration,
// saves, reloads strictly and reads them back.
func TestFactoryT7DateAndESP(t *testing.T) {
	for _, bin := range t7Bins(t) {
		t.Run(bin.String(), func(t *testing.T) {
			t.Parallel()
			orig := readBin(t, bin.path)
			t7 := loadT7(t, bin.path, orig)
			ts := time.Date(2024, 5, 17, 13, 37, 42, 0, time.Local) // stored without a zone
			t7.SetProgrammingDateTime(ts)
			esp := t7.findESPCalibrationPos() >= 0
			was := t7.GetESPCalibration()
			if esp {
				t7.SetESPCalibration(was ^ 0x01)
			}
			out, err := t7.Byte()
			if err != nil {
				t.Fatal(err)
			}
			re, err := strictLoad(ECU_T7, bytes.Clone(out))
			if err != nil {
				t.Fatalf("strict reload: %v", err)
			}
			r7 := re.(*T7File)
			if got := r7.GetInfo().ProgrammingDateTime; !got.Equal(ts) {
				t.Errorf("programming date reads back %v, want %v", got, ts)
			}
			if esp && r7.GetESPCalibration() != was^0x01 {
				t.Errorf("ESP calibration reads back %02X, want %02X", r7.GetESPCalibration(), was^0x01)
			}
		})
	}
}

// TestFactoryT7TransferSymbolTable hides maps the way tuners do, by pointing
// address table records at another symbol, with a valid checksum; importing
// the table from the original must give back the original file exactly.
func TestFactoryT7TransferSymbolTable(t *testing.T) {
	for _, bin := range t7Bins(t) {
		t.Run(bin.String(), func(t *testing.T) {
			t.Parallel()
			orig := readBin(t, bin.path)
			_, start, count, err := t7SymbolTableSpan(orig)
			if err != nil {
				t.Skip(err)
			}
			t7 := loadT7(t, bin.path, orig)
			for i := 5; i < count; i += 97 {
				copy(t7.data[start+i*10:start+i*10+4], t7.data[start+10:start+14])
			}
			if err := t7.UpdateChecksum(); err != nil {
				t.Fatal(err)
			}
			poisoned := bytes.Clone(t7.data)
			if bytes.Equal(poisoned, orig) {
				t.Fatal("poisoning changed nothing")
			}
			if binary.BigEndian.Uint32(poisoned[start+50:]) == binary.BigEndian.Uint32(orig[start+50:]) {
				t.Fatal("record 5 is not poisoned")
			}
			p7 := loadT7(t, bin.path, poisoned)
			if err := p7.TransferSymbolTable(orig); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(p7.Bytes(), orig) {
				t.Errorf("after the import %d bytes are off the original, first at %X", countDiff(p7.Bytes(), orig), firstDiff(p7.Bytes(), orig))
			}
		})
	}
}

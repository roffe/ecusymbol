package symbol

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"time"
)

// T7FirmwareInfo mirrors T7Suite's "Firmware information" dialog: PI-area
// identity plus the calibration switches a tuner flips. Every *Present flag
// says whether the binary carries the symbol/patch site at all; the matching
// value flag is only meaningful when Present is true.
type T7FirmwareInfo struct {
	EngineType         string
	SoftwareVersion    string
	Partnumber         string
	ImmobilizerCode    string
	ChassisID          string
	OriginalCarType    string
	OriginalEngineType string
	ProgrammingDate    string
	SIDDate            string

	// 0xF5-0xF8. These are the EOL security access seed/key words written by
	// XEolprg.c from u16 security[4] -- not feature flags.
	SecuritySeedL1 int
	SecurityKeyL1  int
	SecuritySeedL3 int
	SecurityKeyL3  int

	// ProgrammingDateTime is the T7Suite timestamp at 0x7FD00 (zero when unset).
	ProgrammingDateTime time.Time

	SIDInfoPresent        bool
	SIDInfoOpen           bool // "Open SID info": engine parameters viewable in the SID display. ASCII "No\0" = open, "Yes" = locked
	CompressedSymboltable bool
	MissingSymbolTable    bool
	ChecksumEnabled       bool

	BioPowerSoftware bool // TorqueCal.M_EngMaxE85Tab present
	BioPowerEnabled  bool // E85Cal.ST_Enable
	EthanolSensor    bool // E85Cal.ST_EthanolSensor

	TorqueLimitersPresent   bool
	TorqueLimitersEnabled   bool // TorqueCal.ST_Loop
	OBDIIPresent            bool
	OBDIIEnabled            bool // OBDCal.OBD2Enabled, else OBDCal.EOBDEnabled
	SecondLambdaPresent     bool
	SecondLambdaEnabled     bool // LambdaCal.ST_AdapEnable && O2HeatPostCal.I_LowLim != 0
	FastThrottlePresent     bool // EngTipCal.* present
	FastThrottle            bool // ST_EnableTipin == 0 && ST_EnableTipou == 0
	ExtraFastThrottle       bool // ... && ST_EnableActG2 == 0
	CatalystLightOffPresent bool
	CatalystLightOff        bool // IdleCal.ST_EnableLOffRpm || IgnLOffCal.ST_Enable

	// EU0AF01C / ET03F01C.46S / ET02U01C only: fixed code patch sites.
	SIDOptionsPresent            bool
	SIDDisableStartScreen        bool
	SIDDisableAdaptionMessages   bool
	EmissionLimitingPatchPresent bool
	EmissionLimitingDisabled     bool // 0x13837 == 0x03 (Pavel Angelov's EU0AF01C patch)
}

var t7SIDInfoIndicator = regexp.MustCompile(`Yes|No[.\x00]`)

// t7SIDPatch describes the SID start-screen / adaption-message patch sites per
// software family (T7Suite frmMain, trionictuning.com/forum t=109 p=8537).
type t7SIDPatch struct {
	startScreen, adaption []int
	width                 int // bytes per site: value is 00 00 (disabled) or 00 80 (enabled), or 1-byte 00/80
}

func (t7 *T7File) sidPatch() *t7SIDPatch {
	v := strings.TrimSpace(t7.softwareVersion)
	switch {
	case strings.HasPrefix(v, "EU0AF01C"), v == "ET03F01C.46S":
		return &t7SIDPatch{startScreen: []int{0x4968E, 0x496B4}, adaption: []int{0x49760}, width: 2}
	case strings.HasPrefix(v, "ET02U01C"):
		return &t7SIDPatch{startScreen: []int{0x46F4D}, adaption: []int{0x4701F}, width: 1}
	}
	return nil
}

// sidSite reports (present, disabled) for one patch site.
func (t7 *T7File) sidSite(off, width int) (bool, bool) {
	if off+width > len(t7.data) {
		return false, false
	}
	b := t7.data[off : off+width]
	on := []byte{0x00, 0x80}
	if width == 1 {
		on = []byte{0x80}
	}
	if bytes.Equal(b, on) {
		return true, false
	}
	if bytes.Equal(b, make([]byte, width)) {
		return true, true
	}
	return false, false
}

func (t7 *T7File) symByte(name string) (byte, bool) {
	s := t7.GetByName(name)
	if s == nil || len(s.data) < 1 {
		return 0, false
	}
	return s.data[0], true
}

func (t7 *T7File) symZero(name string) bool { // T7Suite CheckValueIsZero: missing symbol counts as zero
	b, ok := t7.symByte(name)
	return !ok || b == 0
}

func (t7 *T7File) hasPrefix(prefix string) bool {
	for _, s := range t7.Symbols() {
		if strings.HasPrefix(s.Name, prefix) {
			return true
		}
	}
	return false
}

func (t7 *T7File) GetInfo() T7FirmwareInfo {
	fi := T7FirmwareInfo{
		SoftwareVersion: t7.softwareVersion,
		ChassisID:       t7.chassisID,
		EngineType:      t7.engineType, // 0x97
		Partnumber:      t7.ecuSoftwNr,
		ImmobilizerCode: t7.immobilizerID,

		OriginalCarType:    t7.engineType,
		OriginalEngineType: t7.engineType,
		ProgrammingDate:    t7.softwareDate,
		SIDDate:            t7.softwareDate,

		SecuritySeedL1: t7.securitySeedL1,
		SecurityKeyL1:  t7.securityKeyL1,
		SecuritySeedL3: t7.securitySeedL3,
		SecurityKeyL3:  t7.securityKeyL3,
	}

	if m := t7SIDInfoIndicator.FindIndex(t7.data); m != nil {
		fi.SIDInfoPresent = true
		fi.SIDInfoOpen = t7.data[m[0]] == 'N'
	}
	if d := t7.data[0x7FD00:0x7FD06]; d[0] != 0xFF {
		if ts := time.Date(2000+int(d[2]), time.Month(d[1]), int(d[0]), int(d[3]), int(d[4]), int(d[5]), 0, time.Local); ts.Day() == int(d[0]) {
			fi.ProgrammingDateTime = ts
		}
	}
	fi.CompressedSymboltable = isBinaryPackedVersion(t7.data, 0x9B)
	missing := 0
	for _, s := range t7.Symbols() {
		if strings.HasPrefix(s.Name, "Symbol") {
			missing++
		}
	}
	fi.MissingSymbolTable = missing > 10

	// Checksum: any of the ST_Enable switches zero, or the ROM checksum
	// bounds symbols equal (T7Suite: BottomOffFlash == TopOffFlash), means off.
	fi.ChecksumEnabled = true
	for _, n := range []string{"MapChkCal.ST_Enable", "ROM339ChksmCal.ST_Enable", "MapChk.ST_Enable"} {
		if b, ok := t7.symByte(n); ok && b == 0 {
			fi.ChecksumEnabled = false
		}
	}
	if lo, hi := t7.GetByName("ROMChecksum.BottomOffFlash"), t7.GetByName("ROMChecksum.TopOffFlash"); lo != nil && hi != nil && bytes.Equal(lo.data, hi.data) {
		fi.ChecksumEnabled = false
	}

	fi.BioPowerSoftware = t7.GetByName("TorqueCal.M_EngMaxE85Tab") != nil || fi.MissingSymbolTable
	fi.BioPowerEnabled = !t7.symZero("E85Cal.ST_Enable")
	fi.EthanolSensor = !t7.symZero("E85Cal.ST_EthanolSensor")

	fi.TorqueLimitersPresent = t7.GetByName("TorqueCal.ST_Loop") != nil
	fi.TorqueLimitersEnabled = !t7.symZero("TorqueCal.ST_Loop")

	fi.OBDIIPresent = t7.GetByName("OBDCal.OBD2Enabled") != nil || t7.GetByName("OBDCal.EOBDEnabled") != nil
	if t7.GetByName("OBDCal.OBD2Enabled") != nil {
		fi.OBDIIEnabled = !t7.symZero("OBDCal.OBD2Enabled")
	} else {
		fi.OBDIIEnabled = !t7.symZero("OBDCal.EOBDEnabled")
	}

	fi.SecondLambdaPresent = t7.GetByName("LambdaCal.ST_AdapEnable") != nil
	fi.SecondLambdaEnabled = fi.SecondLambdaPresent && !t7.symZero("LambdaCal.ST_AdapEnable")
	if s := t7.GetByName("O2HeatPostCal.I_LowLim"); s != nil && len(s.data) == 2 && s.data[0] == 0 && s.data[1] == 0 {
		fi.SecondLambdaEnabled = false
	}

	fi.FastThrottlePresent = t7.hasPrefix("EngTipCal.")
	if fi.FastThrottlePresent {
		fi.FastThrottle = t7.symZero("EngTipCal.ST_EnableTipin") && t7.symZero("EngTipCal.ST_EnableTipou")
		fi.ExtraFastThrottle = fi.FastThrottle && t7.symZero("EngTipCal.ST_EnableActG2")
	}

	fi.CatalystLightOffPresent = t7.hasPrefix("IgnLOffCal.")
	fi.CatalystLightOff = !(t7.symZero("IdleCal.ST_EnableLOffRpm") && t7.symZero("IgnLOffCal.ST_Enable"))

	if p := t7.sidPatch(); p != nil {
		fi.SIDOptionsPresent, fi.SIDDisableStartScreen = true, true
		for _, off := range p.startScreen {
			ok, dis := t7.sidSite(off, p.width)
			fi.SIDOptionsPresent = fi.SIDOptionsPresent && ok
			fi.SIDDisableStartScreen = fi.SIDDisableStartScreen && dis
		}
		ok, dis := t7.sidSite(p.adaption[0], p.width)
		fi.SIDOptionsPresent = fi.SIDOptionsPresent && ok
		fi.SIDDisableAdaptionMessages = dis
	}
	if strings.HasPrefix(strings.TrimSpace(t7.softwareVersion), "EU0AF01C") {
		switch t7.data[0x13837] {
		case 0x03:
			fi.EmissionLimitingPatchPresent, fi.EmissionLimitingDisabled = true, true
		case 0x02:
			fi.EmissionLimitingPatchPresent, fi.EmissionLimitingDisabled = true, false
		}
	}
	return fi
}

// ---- setters: each changes the in-memory binary only; Save/Byte recompute the checksum.

var ErrOptionNotAvailable = errors.New("option not available in this binary")

func (t7 *T7File) setSymByte(name string, v byte) error {
	s := t7.GetByName(name)
	if s == nil || len(s.data) < 1 {
		return ErrOptionNotAvailable
	}
	s.data[0] = v
	return nil
}

// SetSIDInfoOpen writes the "Open SID info" indicator: open = "No\0", locked = "Yes".
func (t7 *T7File) SetSIDInfoOpen(open bool) error {
	m := t7SIDInfoIndicator.FindIndex(t7.data)
	if m == nil {
		return ErrOptionNotAvailable
	}
	if open {
		copy(t7.data[m[0]:m[0]+3], "No\x00")
	} else {
		copy(t7.data[m[0]:m[0]+3], "Yes")
	}
	return nil
}

func (t7 *T7File) SetTorqueLimiters(on bool) error {
	if on {
		return t7.setSymByte("TorqueCal.ST_Loop", 0x02)
	}
	return t7.setSymByte("TorqueCal.ST_Loop", 0x00)
}

// SetOBDII flips OBDCal.OBD2Enabled when present, otherwise OBDCal.EOBDEnabled
// (T7Suite asks EOBD vs "rest of world" there; both end up in the same byte).
func (t7 *T7File) SetOBDII(on bool) error {
	name := "OBDCal.OBD2Enabled"
	if t7.GetByName(name) == nil {
		name = "OBDCal.EOBDEnabled"
	}
	return t7.setSymByte(name, b2i(on))
}

func (t7 *T7File) SetSecondLambda(on bool) error {
	if err := t7.setSymByte("LambdaCal.ST_AdapEnable", b2i(on)); err != nil {
		return err
	}
	if s := t7.GetByName("O2HeatPostCal.I_LowLim"); s != nil && len(s.data) == 2 {
		s.data[0] = 0
		s.data[1] = 0
		if on {
			s.data[1] = 230
		}
	}
	return nil
}

// SetFastThrottle: fast = Tipin/Tipou off; extra additionally turns ActG2 off.
func (t7 *T7File) SetFastThrottle(fast, extra bool) error {
	if !t7.hasPrefix("EngTipCal.") {
		return ErrOptionNotAvailable
	}
	_ = t7.setSymByte("EngTipCal.ST_EnableTipin", b2i(!fast))
	_ = t7.setSymByte("EngTipCal.ST_EnableTipou", b2i(!fast))
	_ = t7.setSymByte("EngTipCal.ST_EnableActG2", b2i(!(fast && extra)))
	return nil
}

func (t7 *T7File) SetCatalystLightOff(on bool) error {
	e1 := t7.setSymByte("IdleCal.ST_EnableLOffRpm", b2i(on))
	e2 := t7.setSymByte("IgnLOffCal.ST_Enable", b2i(on))
	if e1 != nil && e2 != nil {
		return ErrOptionNotAvailable
	}
	return nil
}

func (t7 *T7File) SetBioPower(on bool) error { return t7.setSymByte("E85Cal.ST_Enable", b2i(on)) }
func (t7 *T7File) SetEthanolSensor(on bool) error {
	return t7.setSymByte("E85Cal.ST_EthanolSensor", b2i(on))
}

func (t7 *T7File) SetSIDOptions(disableStartScreen, disableAdaption bool) error {
	p := t7.sidPatch()
	if p == nil {
		return ErrOptionNotAvailable
	}
	write := func(off int, disabled bool) {
		if p.width == 2 {
			t7.data[off] = 0
			off++
		}
		if disabled {
			t7.data[off] = 0x00
		} else {
			t7.data[off] = 0x80
		}
	}
	for _, off := range p.startScreen {
		write(off, disableStartScreen)
	}
	write(p.adaption[0], disableAdaption)
	return nil
}

func (t7 *T7File) SetEmissionLimitingDisabled(disabled bool) error {
	if !strings.HasPrefix(strings.TrimSpace(t7.softwareVersion), "EU0AF01C") || (t7.data[0x13837] != 0x02 && t7.data[0x13837] != 0x03) {
		return ErrOptionNotAvailable
	}
	t7.data[0x13837] = 0x02
	if disabled {
		t7.data[0x13837] = 0x03
	}
	return nil
}

// SetProgrammingDateTime writes T7Suite's 6-byte timestamp at 0x7FD00.
func (t7 *T7File) SetProgrammingDateTime(ts time.Time) {
	copy(t7.data[0x7FD00:], []byte{byte(ts.Day()), byte(ts.Month()), byte(ts.Year() - 2000), byte(ts.Hour()), byte(ts.Minute()), byte(ts.Second())})
}

func b2i(b bool) byte {
	if b {
		return 1
	}
	return 0
}

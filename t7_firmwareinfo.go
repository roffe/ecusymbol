package symbol

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
	// XEolprg.c from u16 security[4] -- not feature flags. The previous
	// CompressedSymboltable/OpenSIDInfo/SecondLambdaSonde/... booleans tested
	// these for values 1..10; across 386 production bins no such value ever
	// occurs, so every one of them was permanently false.
	SecuritySeedL1 int
	SecurityKeyL1  int
	SecuritySeedL3 int
	SecurityKeyL3  int

	BioPowerEnabled bool
}

func (t7 *T7File) GetInfo() T7FirmwareInfo {

	isBioPower := func() bool {
		sym := t7.GetByName("E85Cal.ST_Enable")
		if sym != nil && sym.Bool() {
			return true
		}
		return false
	}

	return T7FirmwareInfo{
		SoftwareVersion: t7.softwareVersion,
		ChassisID:       t7.chassisID,
		EngineType:      t7.engineType, // 0x97
		Partnumber:      t7.ecuSoftwNr,
		ImmobilizerCode: t7.immobilizerID,

		OriginalCarType:    t7.engineType,
		OriginalEngineType: t7.engineType, // was 0x98, which is the tester serial number
		ProgrammingDate:    t7.softwareDate,
		SIDDate:            t7.softwareDate,

		SecuritySeedL1: t7.securitySeedL1,
		SecurityKeyL1:  t7.securityKeyL1,
		SecuritySeedL3: t7.securitySeedL3,
		SecurityKeyL3:  t7.securityKeyL3,

		BioPowerEnabled: isBioPower(),
	}
}

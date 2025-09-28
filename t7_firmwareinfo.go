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

	ChecksumEnabled       bool
	CompressedSymboltable bool
	NoSymboltablePresent  bool

	OpenSIDInfo               bool
	SecondLambdaSonde         bool
	FastThrottleResponse      bool
	TorqueLimiters            bool
	OBDIIFunctions            bool
	ExtraFastThrottleResponse bool
	CatalystLightOff          bool
	BioPowerEnabled           bool
	DisableEmissionLimiting   bool

	DisableStartscreen       bool
	DisableAdaptationMessage bool
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
		EngineType:      t7.carDescription,
		Partnumber:      t7.partNumber,
		ImmobilizerCode: t7.immobilizerID,

		OriginalCarType:           t7.carDescription,
		OriginalEngineType:        t7.engineType,
		ProgrammingDate:           t7.dateModified,
		SIDDate:                   t7.dateModified,
		ChecksumEnabled:           t7.romChecksumType == 2,
		CompressedSymboltable:     t7.valueF5 == 1,
		NoSymboltablePresent:      t7.valueF6 == 1,
		OpenSIDInfo:               t7.valueF7 == 1,
		SecondLambdaSonde:         t7.valueF8 == 1,
		FastThrottleResponse:      t7.valueF8 == 2,
		TorqueLimiters:            t7.valueF8 == 3,
		OBDIIFunctions:            t7.valueF8 == 4,
		ExtraFastThrottleResponse: t7.valueF8 == 5,
		CatalystLightOff:          t7.valueF8 == 6,

		BioPowerEnabled: isBioPower(),

		DisableEmissionLimiting:  t7.valueF8 == 8,
		DisableStartscreen:       t7.valueF8 == 9,
		DisableAdaptationMessage: t7.valueF8 == 10,
	}
}

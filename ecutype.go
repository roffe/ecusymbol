package symbol

type ECUType int

const (
	ECU_UNKNOWN ECUType = iota - 1
	ECU_T5      ECUType = iota // T5
	ECU_T7                     // T7
	ECU_T8                     // T8
	ECU_ME96                   // ME96 (not supported yet
)

func (e ECUType) String() string {
	switch e {
	case ECU_T5:
		return "T5"
	case ECU_T7:
		return "T7"
	case ECU_T8:
		return "T8"
	case ECU_ME96:
		return "ME9.6"
	case ECU_UNKNOWN:
		fallthrough
	default:
		return "Unknown"
	}
}

func ECUTypeFromString(s string) ECUType {
	switch s {
	case "T5":
		return ECU_T5
	case "T7":
		return ECU_T7
	case "T8":
		return ECU_T8
	case "ME9.6":
		return ECU_ME96
	default:
		return ECU_UNKNOWN
	}
}

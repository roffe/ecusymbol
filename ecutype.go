package symbol

type ECUType int

const (
	ECU_UNKNOWN ECUType = iota - 1
	ECU_T7      ECUType = iota // T7
	ECU_T8                     // T8
)

func (e ECUType) String() string {
	switch e {
	case ECU_T7:
		return "T7"
	case ECU_T8:
		return "T8"
	case ECU_UNKNOWN:
		fallthrough
	default:
		return "Unknown"
	}
}

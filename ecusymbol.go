package symbol

type FirmwareFile interface {
	GetByName(name string) *Symbol
	GetByNumber(number int) *Symbol
	GetXYZ(xAxis, yAxis, zAxis string) ([]int, []int, []int, float64, float64, float64, error)
	Symbols() []*Symbol
	Dump() string
	Count() int
	Add(symbols ...*Symbol)
	Save(filename string) error
	Version() string
}

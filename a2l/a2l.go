// Package a2l parses ASAM MCD-2 MC (ASAP2 / A2L) files, the symbol
// description format used by Bosch ME9.x and friends.
//
// A2ML and IF_DATA blocks are skipped, /include is not followed.
package a2l

import "math"

type File struct {
	Version string // ASAP2_VERSION, e.g. "1.61"
	Project Project
}

type Project struct {
	Name    string
	LongID  string
	Modules []*Module
}

// Module returns the named module, or the first one if name is empty.
func (p *Project) Module(name string) *Module {
	for _, m := range p.Modules {
		if name == "" || m.Name == name {
			return m
		}
	}
	return nil
}

type Module struct {
	Name   string
	LongID string

	ModPar    *ModPar
	ModCommon *ModCommon

	Characteristics []*Characteristic
	Measurements    []*Measurement
	AxisPoints      []*AxisPts
	CompuMethods    []*CompuMethod
	CompuTabs       []*CompuTab
	CompuVTabs      []*CompuVTab
	CompuVTabRanges []*CompuVTabRange
	RecordLayouts   []*RecordLayout
	Functions       []*Function
	Groups          []*Group

	characteristics map[string]*Characteristic
	measurements    map[string]*Measurement
	axisPoints      map[string]*AxisPts
	compuMethods    map[string]*CompuMethod
	recordLayouts   map[string]*RecordLayout
}

func (m *Module) Characteristic(name string) *Characteristic { return m.characteristics[name] }
func (m *Module) Measurement(name string) *Measurement       { return m.measurements[name] }
func (m *Module) AxisPts(name string) *AxisPts               { return m.axisPoints[name] }
func (m *Module) CompuMethod(name string) *CompuMethod       { return m.compuMethods[name] }
func (m *Module) RecordLayout(name string) *RecordLayout     { return m.recordLayouts[name] }

type ModPar struct {
	Comment         string
	Version         string
	EPK             string
	ECU             string
	CPUType         string
	SystemConstants map[string]string
	MemorySegments  []*MemorySegment
}

type MemorySegment struct {
	Name       string
	LongID     string
	PrgType    string // CODE, DATA, ...
	MemoryType string // FLASH, RAM, ...
	Attribute  string // INTERN, EXTERN
	Address    uint32
	Size       uint32
}

type ModCommon struct {
	Comment    string
	ByteOrder  string         // MSB_FIRST or MSB_LAST
	Alignments map[string]int // ALIGNMENT_* keyword -> value
}

type Characteristic struct {
	Name       string
	LongID     string
	Type       string // VALUE, CURVE, MAP, VAL_BLK, ASCII, CUBOID
	Address    uint32
	Deposit    string // RECORD_LAYOUT name
	MaxDiff    float64
	Conversion string // COMPU_METHOD name or NO_COMPU_METHOD
	LowerLimit float64
	UpperLimit float64

	Format         string
	PhysUnit       string
	ExtendedLimits []float64 // [lower, upper] if present
	BitMask        uint64
	ByteOrder      string // overrides MOD_COMMON if set
	Number         int    // ASCII length / VAL_BLK count
	MatrixDim      []int
	ReadOnly       bool
	Discrete       bool
	Axes           []*AxisDescr

	Layout *RecordLayout // resolved Deposit, nil if missing
	Compu  *CompuMethod  // resolved Conversion, nil for NO_COMPU_METHOD
}

type AxisDescr struct {
	Attribute     string // STD_AXIS, FIX_AXIS, COM_AXIS, RES_AXIS, CURVE_AXIS
	InputQuantity string
	Conversion    string
	MaxAxisPoints int
	LowerLimit    float64
	UpperLimit    float64

	AxisPtsRef    string
	Format        string
	PhysUnit      string
	ByteOrder     string
	FixAxisPoints []float64 // raw values computed from FIX_AXIS_PAR*, if FIX_AXIS

	AxisPts *AxisPts     // resolved AxisPtsRef
	Compu   *CompuMethod // resolved Conversion
}

type AxisPts struct {
	Name          string
	LongID        string
	Address       uint32
	InputQuantity string
	Deposit       string // RECORD_LAYOUT name
	MaxDiff       float64
	Conversion    string
	MaxAxisPoints int
	LowerLimit    float64
	UpperLimit    float64

	Format         string
	PhysUnit       string
	ByteOrder      string
	ExtendedLimits []float64
	ReadOnly       bool

	Layout *RecordLayout
	Compu  *CompuMethod
}

type Measurement struct {
	Name       string
	LongID     string
	DataType   DataType
	Conversion string
	Resolution int
	Accuracy   float64
	LowerLimit float64
	UpperLimit float64

	ECUAddress uint32
	ArraySize  int
	BitMask    uint64
	Format     string
	PhysUnit   string
	ByteOrder  string
	Discrete   bool
	ReadWrite  bool
	MatrixDim  []int

	Compu *CompuMethod
}

type RecordLayout struct {
	Name    string
	Static  bool // STATIC_RECORD_LAYOUT
	Entries []RecordLayoutEntry
}

// Entry returns the entry for the given keyword (e.g. "FNC_VALUES",
// "AXIS_PTS_X", "NO_AXIS_PTS_X"), or nil.
func (r *RecordLayout) Entry(keyword string) *RecordLayoutEntry {
	for i := range r.Entries {
		if r.Entries[i].Keyword == keyword {
			return &r.Entries[i]
		}
	}
	return nil
}

type RecordLayoutEntry struct {
	Keyword  string
	Position int
	DataType DataType // for RESERVED this is BYTE/WORD/LONG
	Rest     []string // index mode / addressing tokens, e.g. COLUMN_DIR DIRECT
}

type CompuMethod struct {
	Name   string
	LongID string
	Type   string // IDENTICAL, RAT_FUNC, LINEAR, TAB_INTP, TAB_NOINTP, TAB_VERB, FORM
	Format string
	Unit   string

	Coeffs       []float64 // RAT_FUNC: a b c d e f — int = (a·p²+b·p+c)/(d·p²+e·p+f)
	CoeffsLinear []float64 // LINEAR: a b — phys = a·raw + b
	TabRef       string
	Formula      string
	FormulaInv   string

	Tab       *CompuTab       // resolved TabRef for TAB_INTP / TAB_NOINTP
	VTab      *CompuVTab      // resolved TabRef for TAB_VERB
	VTabRange *CompuVTabRange // resolved TabRef for TAB_VERB ranges
}

// ToPhys converts a raw ECU value to its physical value. FORM and
// TAB_VERB methods return raw unchanged (use Text for verbal tables).
func (c *CompuMethod) ToPhys(x float64) float64 {
	switch c.Type {
	case "LINEAR":
		if len(c.CoeffsLinear) == 2 {
			return c.CoeffsLinear[0]*x + c.CoeffsLinear[1]
		}
	case "RAT_FUNC":
		// invertible when a == d == 0: phys = (c - f·x) / (e·x - b)
		if len(c.Coeffs) == 6 && c.Coeffs[0] == 0 && c.Coeffs[3] == 0 {
			if den := c.Coeffs[4]*x - c.Coeffs[1]; den != 0 {
				return (c.Coeffs[2] - c.Coeffs[5]*x) / den
			}
		}
	case "TAB_INTP":
		if c.Tab != nil {
			return c.Tab.Interp(x)
		}
	case "TAB_NOINTP":
		if c.Tab != nil {
			return c.Tab.Step(x)
		}
	}
	return x
}

// ToRaw converts a physical value back to its raw ECU value.
// Table and FORM methods return phys unchanged.
func (c *CompuMethod) ToRaw(p float64) float64 {
	switch c.Type {
	case "LINEAR":
		if len(c.CoeffsLinear) == 2 && c.CoeffsLinear[0] != 0 {
			return (p - c.CoeffsLinear[1]) / c.CoeffsLinear[0]
		}
	case "RAT_FUNC":
		if len(c.Coeffs) == 6 && c.Coeffs[0] == 0 && c.Coeffs[3] == 0 {
			if den := c.Coeffs[4]*p + c.Coeffs[5]; den != 0 {
				return (c.Coeffs[1]*p + c.Coeffs[2]) / den
			}
		}
	}
	return p
}

// LinearFactors reports the conversion as phys = factor·raw + offset,
// with ok false when the method is not a linear one.
func (c *CompuMethod) LinearFactors() (factor, offset float64, ok bool) {
	switch c.Type {
	case "IDENTICAL":
		return 1, 0, true
	case "LINEAR":
		if len(c.CoeffsLinear) == 2 {
			return c.CoeffsLinear[0], c.CoeffsLinear[1], true
		}
	case "RAT_FUNC":
		if len(c.Coeffs) == 6 &&
			c.Coeffs[0] == 0 && c.Coeffs[3] == 0 && c.Coeffs[4] == 0 && c.Coeffs[1] != 0 {
			return c.Coeffs[5] / c.Coeffs[1], -c.Coeffs[2] / c.Coeffs[1], true
		}
	}
	return 0, 0, false
}

// Text resolves a raw value through a verbal table (TAB_VERB).
func (c *CompuMethod) Text(x float64) (string, bool) {
	if c.VTab != nil {
		return c.VTab.Text(x)
	}
	if c.VTabRange != nil {
		return c.VTabRange.Text(x)
	}
	return "", false
}

type CompuTab struct {
	Name   string
	LongID string
	Type   string // TAB_INTP or TAB_NOINTP
	Keys   []float64
	Values []float64
}

// Interp linearly interpolates, clamping outside the table range.
func (t *CompuTab) Interp(x float64) float64 {
	n := len(t.Keys)
	if n == 0 {
		return x
	}
	if x <= t.Keys[0] {
		return t.Values[0]
	}
	if x >= t.Keys[n-1] {
		return t.Values[n-1]
	}
	for i := 1; i < n; i++ {
		if x <= t.Keys[i] {
			f := (x - t.Keys[i-1]) / (t.Keys[i] - t.Keys[i-1])
			return t.Values[i-1] + f*(t.Values[i]-t.Values[i-1])
		}
	}
	return t.Values[n-1]
}

// Step returns the value of the greatest key <= x (no interpolation).
func (t *CompuTab) Step(x float64) float64 {
	if len(t.Keys) == 0 {
		return x
	}
	v := t.Values[0]
	for i, k := range t.Keys {
		if k > x {
			break
		}
		v = t.Values[i]
	}
	return v
}

type CompuVTab struct {
	Name   string
	LongID string
	Keys   []float64
	Texts  []string
}

func (t *CompuVTab) Text(x float64) (string, bool) {
	for i, k := range t.Keys {
		if k == x {
			return t.Texts[i], true
		}
	}
	return "", false
}

type CompuVTabRange struct {
	Name    string
	LongID  string
	Lower   []float64
	Upper   []float64
	Texts   []string
	Default string
}

func (t *CompuVTabRange) Text(x float64) (string, bool) {
	for i := range t.Lower {
		if x >= t.Lower[i] && x <= t.Upper[i] {
			return t.Texts[i], true
		}
	}
	return t.Default, t.Default != ""
}

type Function struct {
	Name               string
	LongID             string
	DefCharacteristics []string
	RefCharacteristics []string
	InMeasurements     []string
	OutMeasurements    []string
	LocMeasurements    []string
	SubFunctions       []string
}

type Group struct {
	Name               string
	LongID             string
	Root               bool
	RefCharacteristics []string
	RefMeasurements    []string
	SubGroups          []string
}

type DataType string

func (d DataType) Size() int {
	switch d {
	case "UBYTE", "SBYTE", "BYTE":
		return 1
	case "UWORD", "SWORD", "WORD", "FLOAT16_IEEE":
		return 2
	case "ULONG", "SLONG", "LONG", "FLOAT32_IEEE":
		return 4
	case "A_UINT64", "A_INT64", "FLOAT64_IEEE":
		return 8
	}
	return 0
}

func (d DataType) Signed() bool {
	switch d {
	case "SBYTE", "SWORD", "SLONG", "A_INT64":
		return true
	}
	return d.Float()
}

func (d DataType) Float() bool {
	switch d {
	case "FLOAT16_IEEE", "FLOAT32_IEEE", "FLOAT64_IEEE":
		return true
	}
	return false
}

// fixAxisPoints expands FIX_AXIS_PAR (shift variant) or
// FIX_AXIS_PAR_DIST into raw axis values.
func fixAxisPoints(offset, step float64, n int, shift bool) []float64 {
	if shift {
		step = math.Pow(2, step)
	}
	pts := make([]float64, n)
	for i := range pts {
		pts[i] = offset + float64(i)*step
	}
	return pts
}

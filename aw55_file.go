package symbol

// Aisin AW55-50 transmission control module — Saab 9-3/9-5, Renesas SH7058,
// SH-2 big endian, 512 KB flash (1 MB part) with the calibration at 0x70000.
//
// Unlike Trionic there is no symbol table in the firmware: no names, no
// addresses, no map directory. What there is instead is an interpolation
// library, and every call to it carries a whole definition —
//
//	lookup  (x, short *axis, short *data, byte n)              // 1-D curve
//	lookup2d(x, xaxis, ncols, y, yaxis, nrows, data)           // 2-D map
//
// so the maps were mined by resolving those call sites statically (see
// GTISSPS/_re/tcm_y867 for the tooling) and are embedded here per calibration
// family. Names are not recoverable, so the maps come out as Symbol-0, 1, 2 …
// to be identified and renamed as they are decoded.

import (
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type AW55File struct {
	data []byte
	*Collection
}

func (f *AW55File) Byte() ([]byte, error) {
	return f.data, nil
}

func (f *AW55File) Save(filename string) error {
	return nil
}

const (
	aw55CalBase = 0x70000 // calibration start in flash
	aw55CalEnd  = 0x80000
	aw55Magic   = 0x12345678 // first longword of the calibration
	aw55Length  = 0x80000    // the checksummed image; 1 MB dumps are accepted too
)

//go:embed aw55_Y867.json
var aw55Y867 []byte

//go:embed aw55_Y802.json
var aw55Y802 []byte

var aw55Families = map[string][]byte{
	"Y867": aw55Y867,
	"Y802": aw55Y802,
}

// aw55Definition is one mined calibration object. A curve has rows == 1.
type aw55Definition struct {
	N    int    `json:"n"`
	Kind string `json:"kind"`
	Data uint32 `json:"data"`
	Rows int    `json:"rows"`
	Cols int    `json:"cols"`
	X    uint32 `json:"x"`
	Y    uint32 `json:"y,omitempty"`
}

// aw55Axis is what static analysis could establish about an axis. The AF40/AF55
// connector (Aisin AW DSE-00755) has exactly one temperature input — OT, oil
// temperature — so a temperature axis can only be that. Speeds share a unit and
// cannot be told apart from the values alone, so they are labelled "Speed".
type aw55Axis struct {
	Label  string `json:"label"`
	Unit   string `json:"unit"`
	Src    string `json:"src"` // the GBR-relative RAM variable that drives it
	Points int    `json:"points"`
}

type aw55Defs struct {
	Family string              `json:"family"`
	Maps   []aw55Definition    `json:"maps"`
	Axes   map[string]aw55Axis `json:"axes"`
	// CurveDirs are the pointer directories in the application image that
	// address the second calibration format (see aw55Curve). The first one is
	// the shift schedule.
	CurveDirs []uint32 `json:"curveDirs"`
}

// familyRe matches the source-control keywords the calibration carries, e.g.
// "$Workfile: LmY867.c $", which is what names the calibration family.
var familyRe = regexp.MustCompile(`\$Workfile: [LS]m(Y\d+)\.c \$`)

func IsAW55File(data []byte) error {
	if len(data) != aw55Length && len(data) != 2*aw55Length {
		return ErrInvalidLength
	}
	if binary.BigEndian.Uint32(data[aw55CalBase:]) != aw55Magic {
		return ErrMagicBytesNotFound
	}
	return nil
}

// AW55Family is the calibration family a dump belongs to ("Y867", "Y802"),
// taken from the $Workfile records at the end of the calibration.
func AW55Family(data []byte) string {
	if len(data) < aw55CalEnd {
		return ""
	}
	if m := familyRe.FindSubmatch(data[aw55CalBase:aw55CalEnd]); m != nil {
		return string(m[1])
	}
	return ""
}

func NewAW55File(data []byte, printFunc func(string)) (FirmwareFile, error) {
	if err := IsAW55File(data); err != nil {
		return nil, err
	}
	family := AW55Family(data)
	raw, ok := aw55Families[family]
	if !ok {
		return nil, fmt.Errorf("no AW55 definition for calibration family %q", family)
	}
	var defs aw55Defs
	if err := json.Unmarshal(raw, &defs); err != nil {
		return nil, err
	}
	if printFunc != nil {
		printFunc(fmt.Sprintf("AW55-50 TCM, calibration family %s, %d maps", family, len(defs.Maps)))
	}

	// One symbol per map, plus one per axis. The axes are shared between maps,
	// so they are named by address and only created once.
	var symbols []*Symbol
	axes := make(map[uint32]string)
	axisInfo := make(AxisInformation, len(defs.Maps))

	axisSymbol := func(addr uint32, points int) string {
		if name, ok := axes[addr]; ok {
			return name
		}
		info := defs.Axes[strconv.FormatUint(uint64(addr), 10)]
		// Name it after what drives it where that is known: two axes with the
		// same unit but different inputs are different signals, and seeing the
		// variable makes that obvious in the editor.
		name := fmt.Sprintf("Axis-%X", addr)
		switch {
		case info.Label != "" && info.Src != "":
			name = fmt.Sprintf("Axis-%s@%s-%X", info.Label, strings.TrimPrefix(info.Src, "0x"), addr)
		case info.Label != "":
			name = fmt.Sprintf("Axis-%s-%X", info.Label, addr)
		case info.Src != "":
			name = fmt.Sprintf("Axis@%s-%X", strings.TrimPrefix(info.Src, "0x"), addr)
		}
		axes[addr] = name
		sym := aw55Symbol(data, name, len(symbols), addr, points)
		sym.Unit = info.Unit
		symbols = append(symbols, sym)
		return name
	}

	for _, m := range defs.Maps {
		name := fmt.Sprintf("Symbol-%d", m.N)
		axis := Axis{Z: name}
		if m.Cols > 0 {
			axis.X = axisSymbol(m.X, m.Cols)
		}
		if m.Rows > 1 && m.Y != 0 {
			axis.Y = axisSymbol(m.Y, m.Rows)
		}
		axisInfo[name] = axis
		symbols = append(symbols, aw55Symbol(data, name, m.N, m.Data, m.Rows*m.Cols))
	}

	for d, base := range defs.CurveDirs {
		curves := make([]aw55CurveRec, 0, 256)
		for _, addr := range aw55Directory(data, base) {
			curves = append(curves, aw55Curve(data, addr))
		}
		for i := 0; i < len(curves); i += 10 {
			row := curves[i:min(i+10, len(curves))]
			shift := d == 0 && aw55IsShiftRow(row)
			for k, c := range row {
				name := fmt.Sprintf("Curve-%X", c.addr)
				if shift {
					// A[0:5] are the upshift points, B[5:10] the matching
					// downshifts. The index walks successive shifts (1-2, 2-3,
					// …) but the enum behind the row and the index is not
					// pinned down yet, so they are numbered, not named.
					name = fmt.Sprintf("Shift%s-%d-%d", map[bool]string{true: "Up", false: "Dn"}[k < 5], i/10, k%5)
				}
				xName := "Load-" + name
				xs := aw55Raw(data, name, len(symbols), c.addr, c.x)
				xs.Correctionfactor = 100.0 / 255.0 // the load index is 0-255
				xs.Unit = "%"
				xs.Name = xName
				ys := aw55Raw(data, name, len(symbols)+1, c.addr, c.y)
				if shift {
					ys.Unit = "rpm" // the threshold is output-shaft speed
				}
				symbols = append(symbols, xs, ys)
				axisInfo[name] = Axis{X: xName, Z: name}
			}
		}
	}

	axisTranslator[ECU_AW55] = axisInfo

	aw55 := &AW55File{
		data:       data,
		Collection: NewCollection(symbols...),
	}

	return aw55, nil
}

// The shift schedule and several other curve families are not reachable the way
// the maps above are: the code loads them through pointer directories in the
// application image rather than as PC-relative literals, so a scan of the
// calibration references never sees them. Each pointer names a record of 4-byte
// entries walked as a piecewise-linear curve by FUN_00049772 (Y867):
//
//	[x:u8][pad][y:s16be] ...   terminated by x == 0xFF
//
// x is the load index 0-255, built from engine torque; a repeated x is a
// vertical step, which is how the kickdown detent is expressed. For the shift
// schedule y is the output-shaft speed threshold in rpm, compared straight
// against the speed sensor.
const (
	aw55CurveEnd    = 0xFF // terminator, in the x byte
	aw55CurvePoints = 40   // sanity bound; the longest real record is 18
)

// aw55CurveRec is one record with the x and y streams de-interleaved into
// contiguous big-endian 16-bit values, which is what a Symbol can hold.
type aw55CurveRec struct {
	addr uint32
	x, y []byte
}

func aw55Curve(data []byte, addr uint32) aw55CurveRec {
	c := aw55CurveRec{addr: addr}
	for a := int(addr); a+4 <= len(data) && data[a] != aw55CurveEnd; a += 4 {
		c.x = append(c.x, 0, data[a])
		c.y = append(c.y, data[a+2], data[a+3])
		if len(c.x) >= 2*aw55CurvePoints {
			break
		}
	}
	return c
}

// aw55Directory reads a pointer array until a pointer stops pointing into the
// calibration, which is how every one of them is terminated.
func aw55Directory(data []byte, base uint32) []uint32 {
	var ptr []uint32
	for a := int(base); a+4 <= len(data); a += 4 {
		v := binary.BigEndian.Uint32(data[a:])
		if v < aw55CalBase || v >= aw55CalEnd {
			break
		}
		ptr = append(ptr, v)
	}
	return ptr
}

// aw55IsShiftRow reports whether ten consecutive curves are an
// upshift/downshift block: all the same length, and A[k] >= B[k] at every
// breakpoint. That hysteresis invariant is the evidence the identification
// rests on, so a row failing it keeps a neutral name instead of claiming to be
// a shift point.
func aw55IsShiftRow(row []aw55CurveRec) bool {
	if len(row) != 10 || len(row[0].y) < 12 {
		return false
	}
	for _, c := range row {
		if len(c.y) != len(row[0].y) {
			return false
		}
	}
	for k := 0; k < 5; k++ {
		up, dn := row[k].y, row[k+5].y
		for j := 0; j+1 < len(up); j += 2 {
			if int16(binary.BigEndian.Uint16(up[j:])) < int16(binary.BigEndian.Uint16(dn[j:])) {
				return false
			}
		}
	}
	return true
}

// aw55Raw builds a symbol over data the caller already de-interleaved, so the
// address is where the record lives rather than where the bytes are contiguous.
func aw55Raw(data []byte, name string, number int, addr uint32, buf []byte) *Symbol {
	s := &Symbol{
		Name:             name,
		Number:           number,
		Address:          addr,
		Length:           uint16(len(buf)),
		Type:             SIGNED,
		Correctionfactor: 1,
	}
	s.SetData(buf)
	return s
}

// aw55Symbol builds one symbol over `count` big-endian signed 16-bit values.
func aw55Symbol(data []byte, name string, number int, addr uint32, count int) *Symbol {
	s := &Symbol{
		Name:             name,
		Number:           number,
		Address:          addr,
		Length:           uint16(count * 2),
		Type:             SIGNED,
		Correctionfactor: 1,
	}
	if int(addr)+count*2 <= len(data) {
		buf := make([]byte, count*2)
		copy(buf, data[addr:int(addr)+count*2])
		s.SetData(buf)
	}
	return s
}

// Package as2 parses Trionic 7 ".as2" symbol-information files and exposes the
// per-symbol correction factor (resolution) and display precision.
//
// It is a drop-in replacement for the hand maintained lookup tables that used
// to live in package symbol: instead of guessing the correction factor from a
// hardcoded map, load the .as2 that belongs to the binary and ask it.
//
//	f, err := as2.Load("T7_E85V.as2")
//	cf := f.GetCorrectionfactor("BFuelCal.Map") // 0.01
//	p  := f.GetPrecision(cf)                     // 2
//
// # File layout
//
// Each symbol is a small record:
//
//	*BFuelCal.Map                 <- name, '*' prefix
//	MAP                           <- type (SCALAR, TABLE, TABLENOSP, MAP); may be absent
//	1 100 0.000 150 50 1 2 0      <- info line, optionally followed by a unit
//	BFuelCal.AirXSP               <- axis / input references (TABLE: 2, MAP: 4)
//	MAF.m_AirInletFuel
//	BFuelCal.RpmYSP
//	In.n_Engine
//	Description:[" ... "]         <- optional, may span several lines
//
// Info line fields: [0]/[1] is the correction factor (e.g. 1/100 = 0.01),
// [2] the offset, [3] the raw max, [4] the raw min, [6] the number of decimals
// to display. Anything after the eight numbers is the unit.
//
// .as2 files are ISO-8859-1 (Latin-1) with CRLF line endings; both are handled.
package as2

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
)

// Axis describes one dimension of a TABLE or MAP: which symbol holds the axis
// support points (the breakpoint values, stored in the binary) and which runtime
// signal is indexed against them.
type Axis struct {
	SupportPoints string // symbol holding the axis support points, e.g. BFuelCal.AirXSP
	Signal        string // runtime input signal indexed on this axis, e.g. MAF.m_AirInletFuel
}

// Symbol is the metadata for a single AS2 entry.
type Symbol struct {
	Name             string
	Type             string  // SCALAR, TABLE, TABLENOSP, MAP, or "" when absent
	Correctionfactor float64 // fields[0]/fields[1]; engineering value = raw*factor + offset
	Offset           float64 // fields[2]
	Min              float64 // fields[4], raw
	Max              float64 // fields[3], raw
	Decimals         int     // fields[6], intended number of decimals when displayed
	Unit             string
	XAxis            *Axis    // X axis support points / signal, nil for scalars
	YAxis            *Axis    // Y axis support points / signal, nil unless a MAP
	AxisRefs         []string // raw axis / input symbol references (TABLE: 2, MAP: 4)
	Description      string
	Fields           []float64 // raw numeric fields of the info line
}

// Axes returns the symbol's populated axes in order: X first, then Y. The result
// is empty for scalars.
func (s *Symbol) Axes() []*Axis {
	var out []*Axis
	if s.XAxis != nil {
		out = append(out, s.XAxis)
	}
	if s.YAxis != nil {
		out = append(out, s.YAxis)
	}
	return out
}

// File is a parsed AS2 symbol-information file.
type File struct {
	Symbols []*Symbol // in file order
	index   map[string]*Symbol
}

// Load reads and parses the .as2 file at path.
func Load(path string) (*File, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	return Parse(fh)
}

// Parse reads an .as2 file from r.
func Parse(r io.Reader) (*File, error) {
	b, err := io.ReadAll(bufio.NewReader(r))
	if err != nil {
		return nil, err
	}
	return ParseBytes(b), nil
}

// ParseBytes parses the raw bytes of an .as2 file.
func ParseBytes(b []byte) *File {
	lines := splitLines(latin1ToUTF8(b))
	f := &File{index: make(map[string]*Symbol)}

	for i := 0; i < len(lines); {
		line := lines[i]
		if !strings.HasPrefix(line, "*") {
			i++
			continue
		}

		sym := &Symbol{
			Name:             strings.TrimSpace(line[1:]),
			Correctionfactor: 1,
		}
		i++

		// Type line is optional: a couple of symbols jump straight to the
		// info line, which always starts with a number.
		if i < len(lines) && !isInfoLine(lines[i]) && !isDescStart(lines[i]) {
			sym.Type = strings.TrimSpace(lines[i])
			i++
		}

		// Info line.
		if i < len(lines) && isInfoLine(lines[i]) {
			parseInfoLine(lines[i], sym)
			i++
		}

		// Axis / input references, up to the description or the next record.
		for i < len(lines) {
			l := lines[i]
			if l == "" || strings.HasPrefix(l, "*") || isDescStart(l) {
				break
			}
			sym.AxisRefs = append(sym.AxisRefs, strings.TrimSpace(l))
			i++
		}
		sym.parseAxes()

		// Optional, possibly multi-line, description.
		if i < len(lines) && isDescStart(lines[i]) {
			var sb strings.Builder
			for i < len(lines) {
				l := lines[i]
				sb.WriteString(l)
				i++
				if isDescEnd(l) {
					break
				}
				sb.WriteByte('\n')
			}
			sym.Description = cleanDescription(sb.String())
		}

		f.Symbols = append(f.Symbols, sym)
		f.index[sym.Name] = sym
	}

	return f
}

// Symbol returns the named symbol and whether it was found.
func (f *File) Symbol(name string) (*Symbol, bool) {
	s, ok := f.index[name]
	return s, ok
}

// XAxis returns the X axis (support points + signal) of the named symbol.
func (f *File) XAxis(name string) (*Axis, bool) {
	if s, ok := f.index[name]; ok && s.XAxis != nil {
		return s.XAxis, true
	}
	return nil, false
}

// YAxis returns the Y axis (support points + signal) of the named symbol; only
// MAP symbols have one.
func (f *File) YAxis(name string) (*Axis, bool) {
	if s, ok := f.index[name]; ok && s.YAxis != nil {
		return s.YAxis, true
	}
	return nil, false
}

// Axes returns the populated axes (X, then Y) of the named symbol, empty if the
// symbol is unknown or a scalar.
func (f *File) Axes(name string) []*Axis {
	if s, ok := f.index[name]; ok {
		return s.Axes()
	}
	return nil
}

// parseAxes interprets the raw axis references of a TABLE/TABLENOSP/MAP as
// [X support, X signal, Y support, Y signal].
func (s *Symbol) parseAxes() {
	switch s.Type {
	case "TABLE", "TABLENOSP", "MAP":
	default:
		return
	}
	refs := s.AxisRefs
	if len(refs) >= 1 {
		s.XAxis = &Axis{SupportPoints: refs[0]}
		if len(refs) >= 2 {
			s.XAxis.Signal = refs[1]
		}
	}
	if len(refs) >= 3 {
		s.YAxis = &Axis{SupportPoints: refs[2]}
		if len(refs) >= 4 {
			s.YAxis.Signal = refs[3]
		}
	}
}

// GetCorrectionfactor returns the correction factor (resolution) for name, or 1
// when the symbol is not present in the file.
func (f *File) GetCorrectionfactor(name string) float64 {
	if s, ok := f.index[name]; ok {
		return s.Correctionfactor
	}
	return 1
}

// Precision returns the number of decimals the AS2 file specifies for name. When
// the symbol is unknown it falls back to GetPrecision's heuristic on a factor 1.
func (f *File) Precision(name string) int {
	if s, ok := f.index[name]; ok {
		return s.Decimals
	}
	return GetPrecision(1)
}

// GetPrecision returns the number of decimals to display for a given correction
// factor. It mirrors the historic heuristic so existing call sites keep working;
// prefer File.Precision, which uses the value declared in the file.
func (f *File) GetPrecision(corrFac float64) int { return GetPrecision(corrFac) }

// GetPrecision maps a correction factor to a display precision, matching the
// behaviour of the lookup tables this package replaces.
func GetPrecision(corrFac float64) int {
	switch corrFac {
	case 0.1:
		return 1
	case 0.01, 0.00390625, 0.004:
		return 2
	case 0.001:
		return 3
	case 0.0009765625: // 1/1024
		return 4
	case 0.0078125: // 1/128
		return 2
	}
	return 0
}

// parseInfoLine fills the numeric fields and unit of sym from an info line such
// as "1 100 0.000 150 50 1 2 0 kPa".
func parseInfoLine(line string, sym *Symbol) {
	toks := strings.Fields(line)
	nums := make([]float64, 0, 8)
	unitStart := len(toks)
	for idx, t := range toks {
		if len(nums) == 8 {
			unitStart = idx
			break
		}
		v, err := strconv.ParseFloat(t, 64)
		if err != nil {
			unitStart = idx
			break
		}
		nums = append(nums, v)
	}

	sym.Fields = nums
	sym.Unit = strings.TrimSpace(strings.Join(toks[unitStart:], " "))

	if len(nums) > 1 && nums[1] != 0 {
		sym.Correctionfactor = nums[0] / nums[1]
	}
	if len(nums) > 2 {
		sym.Offset = nums[2]
	}
	if len(nums) > 3 {
		sym.Max = nums[3]
	}
	if len(nums) > 4 {
		sym.Min = nums[4]
	}
	if len(nums) > 6 {
		sym.Decimals = int(nums[6])
	}
}

// isInfoLine reports whether line is a numeric info line (first token is a number).
func isInfoLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	first := line
	if sp := strings.IndexAny(line, " \t"); sp >= 0 {
		first = line[:sp]
	}
	_, err := strconv.ParseFloat(first, 64)
	return err == nil
}

// isDescStart matches the description opener case-insensitively: most entries
// use "Description:[", but some use "DESCRIPTION:[".
func isDescStart(line string) bool {
	line = strings.TrimSpace(line)
	return len(line) >= 12 && strings.EqualFold(line[:12], "Description:")
}

// isDescEnd reports whether a description line closes the block. The closer is a
// double quote followed by an optional space and a bracket: `"]` or `" ]`.
func isDescEnd(line string) bool {
	s := strings.TrimRight(line, " \t\r")
	if !strings.HasSuffix(s, "]") {
		return false
	}
	s = strings.TrimRight(s[:len(s)-1], " \t")
	return strings.HasSuffix(s, "\"")
}

// cleanDescription strips the Description:[" ... "] wrapper and trims whitespace.
func cleanDescription(s string) string {
	if idx := strings.Index(s, "[\""); idx >= 0 {
		s = s[idx+2:]
	}
	s = strings.TrimRight(s, " \t\r\n")
	s = strings.TrimSuffix(s, "]")
	s = strings.TrimRight(s, " \t\r\n")
	s = strings.TrimSuffix(s, "\"")
	return strings.TrimSpace(s)
}

// latin1ToUTF8 decodes ISO-8859-1 bytes to a UTF-8 string.
func latin1ToUTF8(b []byte) string {
	runes := make([]rune, len(b))
	for i, c := range b {
		runes[i] = rune(c)
	}
	return string(runes)
}

// splitLines splits on \n and trims a trailing \r from each line.
func splitLines(s string) []string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	return lines
}

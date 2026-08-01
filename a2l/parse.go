package a2l

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf16"
)

// ParseFile reads and parses an A2L file. UTF-16 files (as emitted by
// some Vector/ETAS tools) are converted transparently.
func ParseFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse parses A2L content.
func Parse(data []byte) (*File, error) {
	data = decodeBOM(data)
	toks := lex(data)
	root, err := buildTree(toks)
	if err != nil {
		return nil, err
	}
	return decodeFile(root)
}

func decodeBOM(data []byte) []byte {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return data[3:]
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}), bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		be := data[0] == 0xFE
		data = data[2:]
		u := make([]uint16, 0, len(data)/2)
		for i := 0; i+1 < len(data); i += 2 {
			if be {
				u = append(u, uint16(data[i])<<8|uint16(data[i+1]))
			} else {
				u = append(u, uint16(data[i+1])<<8|uint16(data[i]))
			}
		}
		return []byte(string(utf16.Decode(u)))
	}
	return data
}

// ---- lexer ----

type token struct {
	v    string
	str  bool // was a quoted string
	line int
}

func lex(data []byte) []token {
	var toks []token
	line := 1
	for i := 0; i < len(data); {
		c := data[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == 0:
			i++
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			i += 2
			for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
				if data[i] == '\n' {
					line++
				}
				i++
			}
			i += 2
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
		case c == '"':
			var sb strings.Builder
			start := line
			i++
			for i < len(data) {
				if data[i] == '\\' && i+1 < len(data) {
					sb.WriteByte(data[i+1])
					i += 2
					continue
				}
				if data[i] == '"' {
					if i+1 < len(data) && data[i+1] == '"' { // "" escape
						sb.WriteByte('"')
						i += 2
						continue
					}
					i++
					break
				}
				if data[i] == '\n' {
					line++
				}
				sb.WriteByte(data[i])
				i++
			}
			toks = append(toks, token{v: sb.String(), str: true, line: start})
		default:
			j := i
			for j < len(data) && data[j] != ' ' && data[j] != '\t' &&
				data[j] != '\r' && data[j] != '\n' && data[j] != '\f' && data[j] != 0 {
				j++
			}
			toks = append(toks, token{v: string(data[i:j]), line: line})
			i = j
		}
	}
	return toks
}

// ---- generic block tree ----

type block struct {
	kind string
	line int
	toks []token
	kids []*block
}

func (b *block) kid(kind string) *block {
	for _, k := range b.kids {
		if k.kind == kind {
			return k
		}
	}
	return nil
}

func (b *block) idents() []string {
	out := make([]string, 0, len(b.toks))
	for _, t := range b.toks {
		out = append(out, t.v)
	}
	return out
}

func buildTree(toks []token) (*block, error) {
	root := &block{kind: "ROOT"}
	stack := []*block{root}
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		cur := stack[len(stack)-1]
		if t.str || (t.v != "/begin" && t.v != "/end") {
			cur.toks = append(cur.toks, t)
			continue
		}
		if i+1 >= len(toks) {
			return nil, fmt.Errorf("a2l: line %d: dangling %s", t.line, t.v)
		}
		i++
		kind := toks[i].v
		if t.v == "/end" {
			if cur.kind != kind {
				return nil, fmt.Errorf("a2l: line %d: /end %s inside %s", t.line, kind, cur.kind)
			}
			stack = stack[:len(stack)-1]
			continue
		}
		if kind == "A2ML" || kind == "IF_DATA" { // foreign grammar, skip to matching /end
			depth := 1
			for i++; i < len(toks); i++ {
				if toks[i].str || i+1 >= len(toks) || toks[i+1].str {
					continue
				}
				if toks[i].v == "/begin" && toks[i+1].v == kind {
					depth++
					i++
				} else if toks[i].v == "/end" && toks[i+1].v == kind {
					depth--
					i++
					if depth == 0 {
						break
					}
				}
			}
			if depth != 0 {
				return nil, fmt.Errorf("a2l: line %d: unterminated /begin %s", t.line, kind)
			}
			continue
		}
		nb := &block{kind: kind, line: t.line}
		cur.kids = append(cur.kids, nb)
		stack = append(stack, nb)
	}
	if len(stack) != 1 {
		b := stack[len(stack)-1]
		return nil, fmt.Errorf("a2l: line %d: unterminated /begin %s", b.line, b.kind)
	}
	return root, nil
}

// ---- token scanner over one block ----

type scan struct {
	b   *block
	i   int
	err error
}

func (s *scan) fail(format string, a ...any) {
	if s.err == nil {
		s.err = fmt.Errorf("a2l: line %d: %s: %s", s.b.line, s.b.kind, fmt.Sprintf(format, a...))
	}
}

func (s *scan) more() bool { return s.err == nil && s.i < len(s.b.toks) }

func (s *scan) peek() string {
	if s.i < len(s.b.toks) {
		return s.b.toks[s.i].v
	}
	return ""
}

// optStr consumes the next token only if it is a quoted string.
// Old (pre-1.4) files omit LongID/Comment strings.
func (s *scan) optStr() string {
	if s.err == nil && s.i < len(s.b.toks) && s.b.toks[s.i].str {
		v := s.b.toks[s.i].v
		s.i++
		return v
	}
	return ""
}

func (s *scan) str() string {
	if s.i >= len(s.b.toks) {
		s.fail("unexpected end of block")
		return ""
	}
	v := s.b.toks[s.i].v
	s.i++
	return v
}

func (s *scan) f64() float64 {
	v := s.str()
	if s.err != nil {
		return 0
	}
	f, err := parseNum(v)
	if err != nil {
		s.fail("bad number %q", v)
	}
	return f
}

func (s *scan) int() int { return int(s.f64()) }

func (s *scan) u64() uint64 {
	v := s.str()
	if s.err != nil {
		return 0
	}
	u, err := strconv.ParseUint(v, 0, 64)
	if err != nil {
		f, ferr := parseNum(v)
		if ferr != nil || f < 0 {
			s.fail("bad unsigned number %q", v)
			return 0
		}
		return uint64(f)
	}
	return u
}

func (s *scan) u32() uint32 { return uint32(s.u64()) }

func parseNum(v string) (float64, error) {
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f, nil
	}
	i, err := strconv.ParseInt(v, 0, 64)
	if err == nil {
		return float64(i), nil
	}
	u, uerr := strconv.ParseUint(v, 0, 64)
	if uerr == nil {
		return float64(u), nil
	}
	return 0, err
}

func isNum(v string) bool {
	if v == "" {
		return false
	}
	c := v[0]
	return c >= '0' && c <= '9' || c == '-' || c == '+' || c == '.'
}

// ---- decoding ----

func decodeFile(root *block) (*File, error) {
	f := &File{}
	for i := 0; i+2 < len(root.toks); i++ {
		if !root.toks[i].str && root.toks[i].v == "ASAP2_VERSION" {
			f.Version = root.toks[i+1].v + "." + root.toks[i+2].v
			break
		}
	}
	pb := root.kid("PROJECT")
	if pb == nil {
		return nil, fmt.Errorf("a2l: no PROJECT block found")
	}
	s := &scan{b: pb}
	f.Project.Name = s.str()
	f.Project.LongID = s.optStr()
	if s.err != nil {
		return nil, s.err
	}
	for _, kb := range pb.kids {
		if kb.kind != "MODULE" {
			continue
		}
		m, err := decodeModule(kb)
		if err != nil {
			return nil, err
		}
		f.Project.Modules = append(f.Project.Modules, m)
	}
	if len(f.Project.Modules) == 0 {
		return nil, fmt.Errorf("a2l: no MODULE block found")
	}
	return f, nil
}

func decodeModule(b *block) (*Module, error) {
	s := &scan{b: b}
	m := &Module{Name: s.str(), LongID: s.optStr()}
	if s.err != nil {
		return nil, s.err
	}
	for _, kb := range b.kids {
		var err error
		switch kb.kind {
		case "MOD_PAR":
			m.ModPar, err = decodeModPar(kb)
		case "MOD_COMMON":
			m.ModCommon, err = decodeModCommon(kb)
		case "CHARACTERISTIC":
			var c *Characteristic
			if c, err = decodeCharacteristic(kb); err == nil {
				m.Characteristics = append(m.Characteristics, c)
			}
		case "MEASUREMENT":
			var mm *Measurement
			if mm, err = decodeMeasurement(kb); err == nil {
				m.Measurements = append(m.Measurements, mm)
			}
		case "AXIS_PTS":
			var a *AxisPts
			if a, err = decodeAxisPts(kb); err == nil {
				m.AxisPoints = append(m.AxisPoints, a)
			}
		case "COMPU_METHOD":
			var c *CompuMethod
			if c, err = decodeCompuMethod(kb); err == nil {
				m.CompuMethods = append(m.CompuMethods, c)
			}
		case "COMPU_TAB":
			var t *CompuTab
			if t, err = decodeCompuTab(kb); err == nil {
				m.CompuTabs = append(m.CompuTabs, t)
			}
		case "COMPU_VTAB":
			var t *CompuVTab
			if t, err = decodeCompuVTab(kb); err == nil {
				m.CompuVTabs = append(m.CompuVTabs, t)
			}
		case "COMPU_VTAB_RANGE":
			var t *CompuVTabRange
			if t, err = decodeCompuVTabRange(kb); err == nil {
				m.CompuVTabRanges = append(m.CompuVTabRanges, t)
			}
		case "RECORD_LAYOUT":
			var r *RecordLayout
			if r, err = decodeRecordLayout(kb); err == nil {
				m.RecordLayouts = append(m.RecordLayouts, r)
			}
		case "FUNCTION":
			m.Functions = append(m.Functions, decodeFunction(kb))
		case "GROUP":
			m.Groups = append(m.Groups, decodeGroup(kb))
		}
		if err != nil {
			return nil, err
		}
	}
	m.resolve()
	return m, nil
}

func (m *Module) resolve() {
	m.characteristics = make(map[string]*Characteristic, len(m.Characteristics))
	for _, c := range m.Characteristics {
		m.characteristics[c.Name] = c
	}
	m.measurements = make(map[string]*Measurement, len(m.Measurements))
	for _, mm := range m.Measurements {
		m.measurements[mm.Name] = mm
	}
	m.axisPoints = make(map[string]*AxisPts, len(m.AxisPoints))
	for _, a := range m.AxisPoints {
		m.axisPoints[a.Name] = a
	}
	m.compuMethods = make(map[string]*CompuMethod, len(m.CompuMethods))
	for _, c := range m.CompuMethods {
		m.compuMethods[c.Name] = c
	}
	m.recordLayouts = make(map[string]*RecordLayout, len(m.RecordLayouts))
	for _, r := range m.RecordLayouts {
		m.recordLayouts[r.Name] = r
	}

	tabs := make(map[string]*CompuTab, len(m.CompuTabs))
	for _, t := range m.CompuTabs {
		tabs[t.Name] = t
	}
	vtabs := make(map[string]*CompuVTab, len(m.CompuVTabs))
	for _, t := range m.CompuVTabs {
		vtabs[t.Name] = t
	}
	vranges := make(map[string]*CompuVTabRange, len(m.CompuVTabRanges))
	for _, t := range m.CompuVTabRanges {
		vranges[t.Name] = t
	}
	for _, c := range m.CompuMethods {
		if c.TabRef == "" {
			continue
		}
		c.Tab = tabs[c.TabRef]
		c.VTab = vtabs[c.TabRef]
		c.VTabRange = vranges[c.TabRef]
	}
	for _, c := range m.Characteristics {
		c.Layout = m.recordLayouts[c.Deposit]
		c.Compu = m.compuMethods[c.Conversion]
		for _, a := range c.Axes {
			a.Compu = m.compuMethods[a.Conversion]
			if a.AxisPtsRef != "" {
				a.AxisPts = m.axisPoints[a.AxisPtsRef]
			}
		}
	}
	for _, a := range m.AxisPoints {
		a.Layout = m.recordLayouts[a.Deposit]
		a.Compu = m.compuMethods[a.Conversion]
	}
	for _, mm := range m.Measurements {
		mm.Compu = m.compuMethods[mm.Conversion]
	}
}

func decodeModPar(b *block) (*ModPar, error) {
	s := &scan{b: b}
	p := &ModPar{Comment: s.optStr(), SystemConstants: map[string]string{}}
	for s.more() {
		switch s.str() {
		case "VERSION":
			p.Version = s.str()
		case "EPK":
			p.EPK = s.str()
		case "ECU":
			p.ECU = s.str()
		case "CPU_TYPE":
			p.CPUType = s.str()
		case "SYSTEM_CONSTANT":
			k := s.str()
			p.SystemConstants[k] = s.str()
		}
	}
	for _, kb := range b.kids {
		if kb.kind != "MEMORY_SEGMENT" {
			continue
		}
		ks := &scan{b: kb}
		seg := &MemorySegment{
			Name: ks.str(), LongID: ks.str(),
			PrgType: ks.str(), MemoryType: ks.str(), Attribute: ks.str(),
			Address: ks.u32(), Size: ks.u32(),
		}
		if ks.err != nil {
			return nil, ks.err
		}
		p.MemorySegments = append(p.MemorySegments, seg)
	}
	return p, s.err
}

func decodeModCommon(b *block) (*ModCommon, error) {
	s := &scan{b: b}
	c := &ModCommon{Comment: s.optStr(), Alignments: map[string]int{}}
	for s.more() {
		kw := s.str()
		switch {
		case kw == "BYTE_ORDER":
			c.ByteOrder = s.str()
		case strings.HasPrefix(kw, "ALIGNMENT_"):
			c.Alignments[kw] = s.int()
		}
	}
	return c, s.err
}

func decodeCharacteristic(b *block) (*Characteristic, error) {
	s := &scan{b: b}
	c := &Characteristic{
		Name: s.str(), LongID: s.str(), Type: s.str(),
		Address: s.u32(), Deposit: s.str(), MaxDiff: s.f64(),
		Conversion: s.str(), LowerLimit: s.f64(), UpperLimit: s.f64(),
	}
	for s.more() {
		switch s.str() {
		case "FORMAT":
			c.Format = s.str()
		case "PHYS_UNIT":
			c.PhysUnit = s.str()
		case "EXTENDED_LIMITS":
			c.ExtendedLimits = []float64{s.f64(), s.f64()}
		case "BIT_MASK":
			c.BitMask = s.u64()
		case "BYTE_ORDER":
			c.ByteOrder = s.str()
		case "NUMBER", "ARRAY_SIZE":
			c.Number = s.int()
		case "MATRIX_DIM":
			for s.more() && isNum(s.peek()) {
				c.MatrixDim = append(c.MatrixDim, s.int())
			}
		case "READ_ONLY":
			c.ReadOnly = true
		case "DISCRETE":
			c.Discrete = true
		}
	}
	for _, kb := range b.kids {
		if kb.kind != "AXIS_DESCR" {
			continue
		}
		a, err := decodeAxisDescr(kb)
		if err != nil {
			return nil, err
		}
		c.Axes = append(c.Axes, a)
	}
	return c, s.err
}

func decodeAxisDescr(b *block) (*AxisDescr, error) {
	s := &scan{b: b}
	a := &AxisDescr{
		Attribute: s.str(), InputQuantity: s.str(), Conversion: s.str(),
		MaxAxisPoints: s.int(), LowerLimit: s.f64(), UpperLimit: s.f64(),
	}
	for s.more() {
		switch s.str() {
		case "AXIS_PTS_REF":
			a.AxisPtsRef = s.str()
		case "FORMAT":
			a.Format = s.str()
		case "PHYS_UNIT":
			a.PhysUnit = s.str()
		case "BYTE_ORDER":
			a.ByteOrder = s.str()
		case "FIX_AXIS_PAR":
			off, sh, n := s.f64(), s.f64(), s.int()
			a.FixAxisPoints = fixAxisPoints(off, sh, n, true)
		case "FIX_AXIS_PAR_DIST":
			off, d, n := s.f64(), s.f64(), s.int()
			a.FixAxisPoints = fixAxisPoints(off, d, n, false)
		}
	}
	if kb := b.kid("FIX_AXIS_PAR_LIST"); kb != nil {
		ks := &scan{b: kb}
		for ks.more() {
			a.FixAxisPoints = append(a.FixAxisPoints, ks.f64())
		}
		if ks.err != nil {
			return nil, ks.err
		}
	}
	return a, s.err
}

func decodeAxisPts(b *block) (*AxisPts, error) {
	s := &scan{b: b}
	a := &AxisPts{
		Name: s.str(), LongID: s.str(), Address: s.u32(),
		InputQuantity: s.str(), Deposit: s.str(), MaxDiff: s.f64(),
		Conversion: s.str(), MaxAxisPoints: s.int(),
		LowerLimit: s.f64(), UpperLimit: s.f64(),
	}
	for s.more() {
		switch s.str() {
		case "FORMAT":
			a.Format = s.str()
		case "PHYS_UNIT":
			a.PhysUnit = s.str()
		case "BYTE_ORDER":
			a.ByteOrder = s.str()
		case "EXTENDED_LIMITS":
			a.ExtendedLimits = []float64{s.f64(), s.f64()}
		case "READ_ONLY":
			a.ReadOnly = true
		}
	}
	return a, s.err
}

func decodeMeasurement(b *block) (*Measurement, error) {
	s := &scan{b: b}
	m := &Measurement{
		Name: s.str(), LongID: s.str(), DataType: DataType(s.str()),
		Conversion: s.str(), Resolution: s.int(), Accuracy: s.f64(),
		LowerLimit: s.f64(), UpperLimit: s.f64(),
	}
	for s.more() {
		switch s.str() {
		case "ECU_ADDRESS":
			m.ECUAddress = s.u32()
		case "ARRAY_SIZE":
			m.ArraySize = s.int()
		case "BIT_MASK":
			m.BitMask = s.u64()
		case "FORMAT":
			m.Format = s.str()
		case "PHYS_UNIT":
			m.PhysUnit = s.str()
		case "BYTE_ORDER":
			m.ByteOrder = s.str()
		case "DISCRETE":
			m.Discrete = true
		case "READ_WRITE":
			m.ReadWrite = true
		case "MATRIX_DIM":
			for s.more() && isNum(s.peek()) {
				m.MatrixDim = append(m.MatrixDim, s.int())
			}
		}
	}
	return m, s.err
}

func decodeCompuMethod(b *block) (*CompuMethod, error) {
	s := &scan{b: b}
	c := &CompuMethod{
		Name: s.str(), LongID: s.str(), Type: s.str(),
		Format: s.str(), Unit: s.str(),
	}
	for s.more() {
		switch s.str() {
		case "COEFFS":
			c.Coeffs = []float64{s.f64(), s.f64(), s.f64(), s.f64(), s.f64(), s.f64()}
		case "COEFFS_LINEAR":
			c.CoeffsLinear = []float64{s.f64(), s.f64()}
		case "COMPU_TAB_REF":
			c.TabRef = s.str()
		}
	}
	if kb := b.kid("FORMULA"); kb != nil {
		ks := &scan{b: kb}
		c.Formula = ks.str()
		for ks.more() {
			if ks.str() == "FORMULA_INV" {
				c.FormulaInv = ks.str()
			}
		}
	}
	return c, s.err
}

func decodeCompuTab(b *block) (*CompuTab, error) {
	s := &scan{b: b}
	t := &CompuTab{Name: s.str(), LongID: s.str(), Type: s.str()}
	n := s.int()
	for range n {
		t.Keys = append(t.Keys, s.f64())
		t.Values = append(t.Values, s.f64())
	}
	return t, s.err
}

func decodeCompuVTab(b *block) (*CompuVTab, error) {
	s := &scan{b: b}
	t := &CompuVTab{Name: s.str(), LongID: s.str()}
	s.str() // conversion type, always TAB_VERB
	n := s.int()
	for range n {
		t.Keys = append(t.Keys, s.f64())
		t.Texts = append(t.Texts, s.str())
	}
	return t, s.err
}

func decodeCompuVTabRange(b *block) (*CompuVTabRange, error) {
	s := &scan{b: b}
	t := &CompuVTabRange{Name: s.str(), LongID: s.str()}
	n := s.int()
	for range n {
		t.Lower = append(t.Lower, s.f64())
		t.Upper = append(t.Upper, s.f64())
		t.Texts = append(t.Texts, s.str())
	}
	for s.more() {
		if s.str() == "DEFAULT_VALUE" {
			t.Default = s.str()
		}
	}
	return t, s.err
}

// record layout entries taking "position datatype" plus n extra tokens
var layoutEntries = map[string]int{
	"FNC_VALUES":     2, // index mode, addressing
	"AXIS_PTS_X":     2,
	"AXIS_PTS_Y":     2,
	"AXIS_PTS_Z":     2,
	"AXIS_RESCALE_X": 3,
	"NO_AXIS_PTS_X":  0,
	"NO_AXIS_PTS_Y":  0,
	"NO_AXIS_PTS_Z":  0,
	"NO_RESCALE_X":   0,
	"IDENTIFICATION": 0,
	"RESERVED":       0,
	"SRC_ADDR_X":     0,
	"SRC_ADDR_Y":     0,
	"SRC_ADDR_Z":     0,
	"SHIFT_OP_X":     0,
	"OFFSET_X":       0,
	"DIST_OP_X":      0,
	"RIP_ADDR_X":     0,
	"RIP_ADDR_Y":     0,
	"RIP_ADDR_W":     0,
}

func decodeRecordLayout(b *block) (*RecordLayout, error) {
	s := &scan{b: b}
	r := &RecordLayout{Name: s.str()}
	for s.more() {
		kw := s.str()
		if kw == "STATIC_RECORD_LAYOUT" {
			r.Static = true
			continue
		}
		extra, ok := layoutEntries[kw]
		if !ok {
			continue // unknown keyword; numeric args are skipped harmlessly
		}
		e := RecordLayoutEntry{Keyword: kw, Position: s.int(), DataType: DataType(s.str())}
		for range extra {
			e.Rest = append(e.Rest, s.str())
		}
		r.Entries = append(r.Entries, e)
	}
	return r, s.err
}

func decodeFunction(b *block) *Function {
	s := &scan{b: b}
	f := &Function{Name: s.str(), LongID: s.str()}
	for _, kb := range b.kids {
		switch kb.kind {
		case "DEF_CHARACTERISTIC":
			f.DefCharacteristics = kb.idents()
		case "REF_CHARACTERISTIC":
			f.RefCharacteristics = kb.idents()
		case "IN_MEASUREMENT":
			f.InMeasurements = kb.idents()
		case "OUT_MEASUREMENT":
			f.OutMeasurements = kb.idents()
		case "LOC_MEASUREMENT":
			f.LocMeasurements = kb.idents()
		case "SUB_FUNCTION":
			f.SubFunctions = kb.idents()
		}
	}
	return f
}

func decodeGroup(b *block) *Group {
	s := &scan{b: b}
	g := &Group{Name: s.str(), LongID: s.str()}
	for s.more() {
		if s.str() == "ROOT" {
			g.Root = true
		}
	}
	for _, kb := range b.kids {
		switch kb.kind {
		case "REF_CHARACTERISTIC":
			g.RefCharacteristics = kb.idents()
		case "REF_MEASUREMENT":
			g.RefMeasurements = kb.idents()
		case "SUB_GROUP":
			g.SubGroups = kb.idents()
		}
	}
	return g
}

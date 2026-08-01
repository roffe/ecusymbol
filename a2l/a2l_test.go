package a2l

import (
	"math"
	"slices"
	"testing"
)

func load(t *testing.T) *Module {
	t.Helper()
	f, err := ParseFile("testdata/feature_matrix.a2l")
	if err != nil {
		t.Fatal(err)
	}
	if f.Version != "1.61" {
		t.Errorf("version = %q, want 1.61", f.Version)
	}
	if f.Project.Name != "GOLDEN" {
		t.Errorf("project = %q, want GOLDEN", f.Project.Name)
	}
	m := f.Project.Module("GM")
	if m == nil {
		t.Fatal("module GM not found")
	}
	return m
}

func TestModPar(t *testing.T) {
	m := load(t)
	p := m.ModPar
	if p.EPK != "EPK_GOLD" || p.ECU != "GOLDENECU" || p.CPUType != "TestCPU" || p.Version != "GoldenV1" {
		t.Errorf("mod par = %+v", p)
	}
	if p.SystemConstants["SC_ONE"] != "42" {
		t.Errorf("SC_ONE = %q", p.SystemConstants["SC_ONE"])
	}
	if len(p.MemorySegments) != 1 {
		t.Fatalf("segments = %d, want 1", len(p.MemorySegments))
	}
	seg := p.MemorySegments[0]
	if seg.Name != "Dst1000" || seg.Address != 0x1000 || seg.Size != 0x200 || seg.MemoryType != "FLASH" {
		t.Errorf("segment = %+v", seg)
	}
	if m.ModCommon.ByteOrder != "MSB_FIRST" || m.ModCommon.Alignments["ALIGNMENT_WORD"] != 2 {
		t.Errorf("mod common = %+v", m.ModCommon)
	}
}

func TestCompuMethods(t *testing.T) {
	m := load(t)
	if len(m.CompuMethods) != 8 {
		t.Fatalf("compu methods = %d, want 8", len(m.CompuMethods))
	}

	fac, off, ok := m.CompuMethod("cm_lin").LinearFactors()
	if !ok || fac != 8 || off != 0 {
		t.Errorf("cm_lin factors = %v %v %v, want 8 0 true", fac, off, ok)
	}
	if got := m.CompuMethod("cm_lin").ToPhys(10); got != 80 {
		t.Errorf("cm_lin ToPhys(10) = %v, want 80", got)
	}
	if got := m.CompuMethod("cm_lin").ToRaw(80); got != 10 {
		t.Errorf("cm_lin ToRaw(80) = %v, want 10", got)
	}

	fac, off, ok = m.CompuMethod("cm_clin").LinearFactors()
	if !ok || fac != 0.5 || off != -40 {
		t.Errorf("cm_clin factors = %v %v %v, want 0.5 -40 true", fac, off, ok)
	}

	// hyperbolic: int = 200/phys -> phys = 200/int
	if got := m.CompuMethod("cm_hyp").ToPhys(50); got != 4 {
		t.Errorf("cm_hyp ToPhys(50) = %v, want 4", got)
	}
	if _, _, ok := m.CompuMethod("cm_hyp").LinearFactors(); ok {
		t.Error("cm_hyp should not report linear factors")
	}

	if got := m.CompuMethod("cm_tab").ToPhys(100); got != 25 {
		t.Errorf("cm_tab ToPhys(100) = %v, want 25", got)
	}
	if got := m.CompuMethod("cm_step").ToPhys(64); got != 1 {
		t.Errorf("cm_step ToPhys(64) = %v, want 1", got)
	}
	if got := m.CompuMethod("cm_step").ToPhys(128); got != 2 {
		t.Errorf("cm_step ToPhys(128) = %v, want 2", got)
	}

	if s, ok := m.CompuMethod("cm_verb").Text(1); !ok || s != "ACTIVE" {
		t.Errorf("cm_verb Text(1) = %q %v", s, ok)
	}
	if s, ok := m.CompuMethod("cm_range").Text(7); !ok || s != "HIGH" {
		t.Errorf("cm_range Text(7) = %q %v", s, ok)
	}
	if s, ok := m.CompuMethod("cm_range").Text(99); !ok || s != "OUT_OF_RANGE" {
		t.Errorf("cm_range Text(99) = %q %v", s, ok)
	}

	if got := m.CompuMethod("cm_form").Formula; got != "X1/8.0" {
		t.Errorf("cm_form formula = %q", got)
	}
}

func TestRecordLayouts(t *testing.T) {
	m := load(t)
	if len(m.RecordLayouts) != 8 {
		t.Fatalf("record layouts = %d, want 8", len(m.RecordLayouts))
	}
	mi := m.RecordLayout("MapInline")
	if e := mi.Entry("NO_AXIS_PTS_X"); e == nil || e.Position != 1 || e.DataType != "UBYTE" {
		t.Errorf("MapInline NO_AXIS_PTS_X = %+v", e)
	}
	if e := mi.Entry("FNC_VALUES"); e == nil || e.Position != 5 || e.DataType != "SWORD" ||
		!slices.Equal(e.Rest, []string{"COLUMN_DIR", "DIRECT"}) {
		t.Errorf("MapInline FNC_VALUES = %+v", e)
	}
	if !m.RecordLayout("CurveStatic").Static {
		t.Error("CurveStatic should be static")
	}
	if e := m.RecordLayout("IdRes").Entry("RESERVED"); e == nil || e.DataType.Size() != 2 {
		t.Errorf("IdRes RESERVED = %+v", e)
	}
}

func TestCharacteristics(t *testing.T) {
	m := load(t)
	if len(m.Characteristics) != 17 {
		t.Fatalf("characteristics = %d, want 17", len(m.Characteristics))
	}

	c := m.Characteristic("KW_SCALAR")
	if c.Type != "VALUE" || c.Address != 0x1000 || c.Deposit != "Val8" ||
		c.Conversion != "cm_lin" || c.LowerLimit != 0 || c.UpperLimit != 400 {
		t.Errorf("KW_SCALAR = %+v", c)
	}
	if !slices.Equal(c.ExtendedLimits, []float64{0, 2040}) || c.Format != "%6.2" {
		t.Errorf("KW_SCALAR extras = %+v", c)
	}
	if c.Layout == nil || c.Layout.Name != "Val8" || c.Compu == nil || c.Compu.Name != "cm_lin" {
		t.Error("KW_SCALAR links not resolved")
	}

	if c := m.Characteristic("KW_BIT"); c.BitMask != 0x0C || !c.ReadOnly {
		t.Errorf("KW_BIT = %+v", c)
	}
	if c := m.Characteristic("KW_FORM"); c.ByteOrder != "MSB_LAST" {
		t.Errorf("KW_FORM byte order = %q", c.ByteOrder)
	}
	if c := m.Characteristic("TXT_ID"); c.Type != "ASCII" || c.Number != 8 {
		t.Errorf("TXT_ID = %+v", c)
	}
	if c := m.Characteristic("BLK_2D"); !slices.Equal(c.MatrixDim, []int{4, 2, 1}) {
		t.Errorf("BLK_2D matrix dim = %v", c.MatrixDim)
	}
	if c := m.Characteristic("KW_FLOAT"); c.Compu != nil {
		t.Error("NO_COMPU_METHOD should resolve to nil")
	}
	if c := m.Characteristic("KW_STATE"); !c.Discrete {
		t.Error("KW_STATE should be discrete")
	}
}

func TestAxes(t *testing.T) {
	m := load(t)

	a := m.AxisPts("AX_SHARED")
	if a == nil || a.Address != 0x1060 || a.Deposit != "AxisU16" || a.MaxAxisPoints != 4 ||
		a.PhysUnit != "1/min" || !slices.Equal(a.ExtendedLimits, []float64{0, 52000}) {
		t.Fatalf("AX_SHARED = %+v", a)
	}
	if a.Layout == nil || a.Compu == nil {
		t.Error("AX_SHARED links not resolved")
	}

	if ax := m.Characteristic("KL_COM").Axes[0]; ax.Attribute != "COM_AXIS" ||
		ax.AxisPtsRef != "AX_SHARED" || ax.AxisPts != a {
		t.Errorf("KL_COM axis = %+v", ax)
	}

	if ax := m.Characteristic("KL_FIX").Axes[0]; !slices.Equal(ax.FixAxisPoints, []float64{0, 2, 4, 6}) {
		t.Errorf("KL_FIX points = %v", ax.FixAxisPoints)
	}
	if ax := m.Characteristic("KL_FIXDIST").Axes[0]; !slices.Equal(ax.FixAxisPoints, []float64{100, 150, 200}) {
		t.Errorf("KL_FIXDIST points = %v", ax.FixAxisPoints)
	}
	if ax := m.Characteristic("KL_FIXLIST").Axes[0]; !slices.Equal(ax.FixAxisPoints, []float64{5, 17.5, 900}) {
		t.Errorf("KL_FIXLIST points = %v", ax.FixAxisPoints)
	}

	kf := m.Characteristic("KF_MAP")
	if len(kf.Axes) != 2 || kf.Axes[0].InputQuantity != "m_nmot" || kf.Axes[1].Conversion != "cm_clin" {
		t.Errorf("KF_MAP axes = %+v", kf.Axes)
	}
}

func TestMeasurements(t *testing.T) {
	m := load(t)
	if len(m.Measurements) != 2 {
		t.Fatalf("measurements = %d, want 2", len(m.Measurements))
	}
	nmot := m.Measurement("m_nmot")
	if nmot.DataType != "UWORD" || nmot.ECUAddress != 0x40008000 || nmot.Format != "%5.0" {
		t.Errorf("m_nmot = %+v", nmot)
	}
	if nmot.DataType.Size() != 2 || nmot.DataType.Signed() {
		t.Error("UWORD should be size 2 unsigned")
	}
	load2 := m.Measurement("m_load")
	if load2.ArraySize != 4 || load2.BitMask != 0x7F || !load2.Discrete || !load2.ReadWrite {
		t.Errorf("m_load = %+v", load2)
	}
}

func TestFunctionsGroups(t *testing.T) {
	m := load(t)
	var root *Function
	for _, f := range m.Functions {
		if f.Name == "FCT_ROOT" {
			root = f
		}
	}
	if root == nil || !slices.Equal(root.DefCharacteristics, []string{"KW_SCALAR", "KL_DYN"}) ||
		!slices.Equal(root.SubFunctions, []string{"FCT_CHILD"}) {
		t.Errorf("FCT_ROOT = %+v", root)
	}
	var grp *Group
	for _, g := range m.Groups {
		if g.Name == "GRP_ROOT" {
			grp = g
		}
	}
	if grp == nil || !grp.Root || !slices.Equal(grp.RefCharacteristics, []string{"KW_SCALAR"}) ||
		!slices.Equal(grp.SubGroups, []string{"GRP_SUB"}) {
		t.Errorf("GRP_ROOT = %+v", grp)
	}
}

func TestMalformed(t *testing.T) {
	for _, in := range []string{
		"",
		`/begin PROJECT P "x"`,
		`/begin PROJECT P "x" /end MODULE`,
		`/begin`,
		`/begin PROJECT P "x" /begin A2ML junk`,
	} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("Parse(%q) should fail", in)
		}
	}
}

func TestStringWithBeginInside(t *testing.T) {
	src := `ASAP2_VERSION 1 61
/begin PROJECT P "contains /begin TRAP and /end TRAP"
  /begin MODULE M "also /end MODULE inside a string"
  /end MODULE
/end PROJECT`
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if f.Project.LongID != "contains /begin TRAP and /end TRAP" {
		t.Errorf("long id = %q", f.Project.LongID)
	}
}

func TestUTF16(t *testing.T) {
	src := "ASAP2_VERSION 1 61\n/begin PROJECT P \"p\"\n/begin MODULE M \"m\"\n/end MODULE\n/end PROJECT"
	le := []byte{0xFF, 0xFE}
	for _, r := range src {
		le = append(le, byte(r), byte(r>>8))
	}
	f, err := Parse(le)
	if err != nil {
		t.Fatal(err)
	}
	if f.Project.Name != "P" {
		t.Errorf("project = %q", f.Project.Name)
	}
}

func TestInterpClamp(t *testing.T) {
	tab := &CompuTab{Keys: []float64{0, 200}, Values: []float64{0, 50}}
	for _, tc := range [][2]float64{{-10, 0}, {0, 0}, {100, 25}, {200, 50}, {999, 50}} {
		if got := tab.Interp(tc[0]); math.Abs(got-tc[1]) > 1e-9 {
			t.Errorf("Interp(%v) = %v, want %v", tc[0], got, tc[1])
		}
	}
}

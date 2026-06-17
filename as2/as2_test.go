package as2

import (
	"os"
	"testing"
)

// fixture exercises the awkward parts of the format: a MAP with four axis
// references, a TABLENOSP with a Latin-1 unit (0xB0 = degree sign), a SCALAR,
// a single-line description, an entry with no type line, and a description that
// closes with `" ]`.
var fixture = []byte("" +
	"*BFuelCal.Map\r\n" +
	"MAP\r\n" +
	"1 100 0.000 150 50 1 2 0\r\n" +
	"BFuelCal.AirXSP\r\n" +
	"MAF.m_AirInletFuel\r\n" +
	"BFuelCal.RpmYSP\r\n" +
	"In.n_Engine\r\n" +
	"Description:[\" DESCRIPTION: base fuel map\r\n" +
	"RANGE      : 50 - 150 \"]\r\n" +
	"\r\n" +
	"*Out.fi_Ignition\r\n" +
	"SCALAR\r\n" +
	"1 10 0.000 450 -100 1 1 0 \xb0CA\r\n" +
	"Description:[\" ignition angle \"]\r\n" +
	"\r\n" +
	"*E85.AFR\r\n" +
	"1.000000 100 0.000000 1000 0 1 2 0\r\n" +
	"Description:[\" DESCRIPTION: Air to fuel ratio with E15 blend.\r\n" +
	" RANGE      : 0 - 1.00 \" ]\r\n" +
	"\r\n" +
	"*Some.NoDesc\r\n" +
	"SCALAR\r\n" +
	"1 1 0.000 255 0 1 0 0\r\n")

func TestParseFixture(t *testing.T) {
	f := ParseBytes(fixture)

	if got, want := len(f.Symbols), 4; got != want {
		t.Fatalf("symbol count = %d, want %d", got, want)
	}

	tests := []struct {
		name    string
		typ     string
		factor  float64
		prec    int
		unit    string
		decimal int
		axes    int
	}{
		{"BFuelCal.Map", "MAP", 0.01, 2, "", 2, 4},
		{"Out.fi_Ignition", "SCALAR", 0.1, 1, "°CA", 1, 0},
		{"E85.AFR", "", 0.01, 2, "", 2, 0},
		{"Some.NoDesc", "SCALAR", 1, 0, "", 0, 0},
	}

	for _, tt := range tests {
		s, ok := f.Symbol(tt.name)
		if !ok {
			t.Errorf("%s: not found", tt.name)
			continue
		}
		if s.Type != tt.typ {
			t.Errorf("%s: type = %q, want %q", tt.name, s.Type, tt.typ)
		}
		if cf := f.GetCorrectionfactor(tt.name); cf != tt.factor {
			t.Errorf("%s: correctionfactor = %v, want %v", tt.name, cf, tt.factor)
		}
		if p := f.GetPrecision(tt.factor); p != tt.prec {
			t.Errorf("%s: precision(%v) = %d, want %d", tt.name, tt.factor, p, tt.prec)
		}
		if s.Unit != tt.unit {
			t.Errorf("%s: unit = %q, want %q", tt.name, s.Unit, tt.unit)
		}
		if s.Decimals != tt.decimal {
			t.Errorf("%s: decimals = %d, want %d", tt.name, s.Decimals, tt.decimal)
		}
		if len(s.AxisRefs) != tt.axes {
			t.Errorf("%s: axis refs = %d, want %d", tt.name, len(s.AxisRefs), tt.axes)
		}
	}

	// MAP axis support points and signals.
	m, _ := f.Symbol("BFuelCal.Map")
	if m.XAxis == nil || m.YAxis == nil {
		t.Fatalf("BFuelCal.Map: expected X and Y axes, got %+v / %+v", m.XAxis, m.YAxis)
	}
	if m.XAxis.SupportPoints != "BFuelCal.AirXSP" || m.XAxis.Signal != "MAF.m_AirInletFuel" {
		t.Errorf("BFuelCal.Map X axis = %+v", *m.XAxis)
	}
	if m.YAxis.SupportPoints != "BFuelCal.RpmYSP" || m.YAxis.Signal != "In.n_Engine" {
		t.Errorf("BFuelCal.Map Y axis = %+v", *m.YAxis)
	}
	if got := len(m.Axes()); got != 2 {
		t.Errorf("BFuelCal.Map Axes() = %d, want 2", got)
	}

	// SCALARs have no axes.
	if x, ok := f.XAxis("Out.fi_Ignition"); ok {
		t.Errorf("Out.fi_Ignition: unexpected X axis %+v", x)
	}

	if cf := f.GetCorrectionfactor("Does.NotExist"); cf != 1 {
		t.Errorf("missing symbol: correctionfactor = %v, want 1", cf)
	}

	if d := f.Symbol2Desc("Out.fi_Ignition"); d != "ignition angle" {
		t.Errorf("description = %q, want %q", d, "ignition angle")
	}
}

// Symbol2Desc is a tiny helper used only by the test.
func (f *File) Symbol2Desc(name string) string {
	if s, ok := f.Symbol(name); ok {
		return s.Description
	}
	return ""
}

// TestRealFile runs against a real .as2 if AS2_TESTFILE points at one (or the
// known sample exists), and spot-checks a few symbols. It is skipped otherwise.
func TestRealFile(t *testing.T) {
	path := os.Getenv("AS2_TESTFILE")
	if path == "" {
		path = "/home/roffe/OneDrive/T7 Source/eu03_erik_org/Build/T7_E85V.as2"
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("sample file not available: %v", err)
	}

	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Symbols) < 5000 {
		t.Fatalf("only %d symbols parsed, expected >5000", len(f.Symbols))
	}

	checks := map[string]float64{
		"BFuelCal.Map":     0.01,
		"InjAnglCal.Map":   0.1,
		"Out.fi_Ignition":  0.1,
		"Lambda.LambdaInt": 0.01,
	}
	for name, want := range checks {
		if cf := f.GetCorrectionfactor(name); cf != want {
			t.Errorf("%s: correctionfactor = %v, want %v", name, cf, want)
		}
	}

	// A known MAP must expose both axes with the right support points.
	x, ok := f.XAxis("BFuelCal.Map")
	if !ok || x.SupportPoints != "BFuelCal.AirXSP" {
		t.Errorf("BFuelCal.Map X support points = %+v, ok=%v", x, ok)
	}
	if y, ok := f.YAxis("BFuelCal.Map"); !ok || y.SupportPoints != "BFuelCal.RpmYSP" {
		t.Errorf("BFuelCal.Map Y support points = %+v, ok=%v", y, ok)
	}

	// No SCALAR should ever carry an axis (guards the description-detection bug
	// that used to slurp DESCRIPTION:[ lines as axis references).
	for _, s := range f.Symbols {
		if s.Type == "SCALAR" && (s.XAxis != nil || s.YAxis != nil) {
			t.Errorf("%s: SCALAR unexpectedly has an axis", s.Name)
		}
	}
}

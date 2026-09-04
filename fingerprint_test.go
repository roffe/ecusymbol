package symbol

import (
	"bytes"
	"os"
	"testing"
)

// Hex-editing the software ID must not change the code fingerprint, and the
// original ID must still be found via Identify.
func TestFingerprintSurvivesVersionEdit(t *testing.T) {
	const dir = "/run/media/roffe/87219D58D1FD6B88/TrionicDevel/TuningSuites/"
	for _, tc := range []struct{ file, version string }{
		{"T5Binaries/4781035.BIN", "A554X24L.18C"},
		{"T7Binaries/5383930.bin", "EC0XY3RC.48H"},
		{"T8Binaries/55567225_FD0M_C_FMEP_14_FIEF_80l.BIN", "FD0M_C_FMEP_14_FIE_80l.s19"},
	} {
		data, err := os.ReadFile(dir + tc.file)
		if err != nil {
			t.Skip(err)
		}
		_, ff, err := Load(tc.file, data, func(string) {})
		if err != nil {
			t.Fatal(err)
		}
		if ff.Version() != tc.version {
			t.Fatalf("%s: version %q", tc.file, ff.Version())
		}
		want := ff.(Fingerprinter).CodeFingerprint()
		if ids, score := Identify(ff); score != 1 || !contains(ids, tc.version) {
			t.Fatalf("%s: Identify = %v %v", tc.file, ids, score)
		}

		// Edit in place (the file shares data's backing array) and refresh the
		// checksums so the edited binary still loads.
		edited := "HELLO" + tc.version[5:]
		copy(data, bytes.Replace(data, []byte(tc.version), []byte(edited), -1))
		if t7, ok := ff.(*T7File); ok { // UpdateChecksum rebuilds the PI area from fields
			t7.softwareVersion = edited
		}
		if c, ok := ff.(interface{ UpdateChecksum() error }); ok {
			if err := c.UpdateChecksum(); err != nil {
				t.Fatal(err)
			}
		}
		_, ef, err := Load(tc.file, data, func(string) {})
		if err != nil {
			t.Fatal(err)
		}
		if ef.Version() == tc.version {
			t.Fatalf("%s: edit did not take", tc.file)
		}
		if got := ef.(Fingerprinter).CodeFingerprint(); got != want {
			t.Fatalf("%s: fingerprint changed %s -> %s", tc.file, want, got)
		}
		if ids, score := Identify(ef); score != 1 || !contains(ids, tc.version) {
			t.Fatalf("%s: edited file not identified: %v %v", tc.file, ids, score)
		}

		// Patch a byte inside code: still identified, but no longer exact.
		codeStart := 0x100
		for _, r := range ef.(Fingerprinter).DataRanges() {
			if r[0] <= codeStart && codeStart < r[1] {
				codeStart = r[1] + 0x100
			}
		}
		data[codeStart] ^= 0xFF
		if c, ok := ef.(interface{ UpdateChecksum() error }); ok {
			if err := c.UpdateChecksum(); err != nil {
				t.Fatal(err)
			}
		}
		_, pf, err := Load(tc.file, data, func(string) {})
		if err != nil {
			t.Fatal(err)
		}
		if ids, score := Identify(pf); score >= 1 || score < 0.95 || !contains(ids, tc.version) {
			t.Fatalf("%s: patched file: %v %v", tc.file, ids, score)
		}
	}
}

func contains(s []Software, id string) bool {
	for _, x := range s {
		if x.ID == id && len(x.PartNumbers) > 0 {
			return true
		}
	}
	return false
}

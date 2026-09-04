package symbol

import (
	"fmt"
	"hash/crc32"
	"slices"
	"sort"
	"strings"
)

// fingerprintBlock is the granularity of the code fingerprint. Tuner patches
// to constants inside code only invalidate the blocks they touch, so a
// patched binary still scores close to 1 against its base software.
const fingerprintBlock = 4096

type fingerprintEntry struct {
	blocks   string // one CRC32 as 8 hex chars per file block, or maskedBlock
	software []Software
}

// Software is a known software ID and the Saab part numbers it shipped under.
type Software struct {
	ID          string
	PartNumbers []string
}

// maskedBlock marks a block that holds no code at all.
const maskedBlock = "........"

// Fingerprinter is implemented by firmware files whose code section can be
// hashed independently of calibration data.
type Fingerprinter interface {
	// CodeFingerprint hashes the code part of the binary block by block,
	// skipping calibration, checksums, adaption areas and EOL/PI data, so a
	// hex-edited software ID or a retuned map still yields the same value
	// and a patched constant in code only changes one block.
	CodeFingerprint() string
	// DataRanges returns the [start,end) byte ranges CodeFingerprint skips.
	DataRanges() [][2]int
}

// Identify returns the known softwares whose code best matches the file and
// the fraction of code blocks that agree (1 = identical code). It returns
// nil, 0 when nothing in the database shares any code with the file.
func Identify(f FirmwareFile) ([]Software, float64) {
	fp, ok := f.(Fingerprinter)
	if !ok {
		return nil, 0
	}
	got := fp.CodeFingerprint()
	var best *fingerprintEntry
	bestHits, bestN := 0, 1
	for i := range knownFingerprints {
		e := &knownFingerprints[i]
		if len(e.blocks) != len(got) {
			continue
		}
		hits, n := 0, 0
		for j := 0; j < len(got); j += 8 {
			a, b := got[j:j+8], e.blocks[j:j+8]
			if a == maskedBlock && b == maskedBlock {
				continue
			}
			n++
			if a == b {
				hits++
			}
		}
		if hits*bestN > bestHits*n {
			best, bestHits, bestN = e, hits, n
		}
	}
	if best == nil {
		return nil, 0
	}
	return best.software, float64(bestHits) / float64(bestN)
}

func (t7 *T7File) DataRanges() [][2]int {
	// The calibration block is masked as one span (unnamed calibration bytes
	// sit between named symbols), but a few data symbols such as LocalID and
	// the symbol table markers live inside code, so runs of symbols are
	// masked separately. The FW checksum word lives in code; above
	// topOfProgram is flash-stored adaption and the PI area. The ESP
	// calibration byte is parked in code right after an rts.
	r := symbolRuns(t7.Symbols(), len(t7.data))
	if cs, err := t7.getFWChecksum(); err == nil && cs.Address >= 0 {
		r = append(r, [2]int{cs.Address, cs.Address + 4})
	}
	if i := t7.findESPByte(); i >= 0 {
		r = append(r, [2]int{i, i + 1})
	}
	top := t7.topOfProgram
	if top <= 0 || top > len(t7.data) {
		top = 0x70000
	}
	return append(r, [2]int{top, len(t7.data)})
}

// findESPByte returns the offset of the ESP calibration byte, which sits in
// "4E 75 xx FF 20 7C 00", or -1.
func (t7 *T7File) findESPByte() int {
	d := t7.data
	for i := 0; i+7 <= len(d); i++ {
		if d[i] == 0x4E && d[i+1] == 0x75 && d[i+3] == 0xFF && d[i+4] == 0x20 && d[i+5] == 0x7C && d[i+6] == 0 {
			return i + 2
		}
	}
	return -1
}

func (t8 *T8File) DataRanges() [][2]int {
	// 0x00000-0x20000 is the bootloader plus the NVDM partition (car
	// specific data, DTCs) and boot-written words. Small calibration groups
	// can sit inside code next to their functions, so runs of symbols are
	// masked separately; after the last one only unnamed initialised data,
	// the coded EOL/checksum block and filler remain.
	r := append([][2]int{{0, 0x20000}}, symbolRuns(t8.Symbols(), len(t8.data))...)
	return append(r, [2]int{symbolSpan(t8.Symbols(), len(t8.data))[1], len(t8.data)})
}

func (t5 *T5File) DataRanges() [][2]int {
	// Calibration sits at the start of flash and ends with the software ID
	// "XXXXXXXX.NNA$" plus a lock byte; code follows. The footer (part
	// numbers, chassis, checksum) begins where the checksummed area ends.
	r := [][2]int{{0, symbolSpan(t5.Symbols(), len(t5.data))[1]}}
	if i := t5.findSoftwareVersionEnd(); i > 0 {
		r[0][1] = max(r[0][1], i+2)
	}
	if i, err := t5.getIndexOfFirstMarking(); err == nil {
		r = append(r, [2]int{i + 4, len(t5.data)})
	}
	return r
}

func (t7 *T7File) CodeFingerprint() string { return fingerprint(t7.data, t7.DataRanges()) }
func (t8 *T8File) CodeFingerprint() string { return fingerprint(t8.data, t8.DataRanges()) }
func (t5 *T5File) CodeFingerprint() string { return fingerprint(t5.data, t5.DataRanges()) }

// symbolRuns returns one [start,end) span per run of flash symbols, a run
// being broken by a gap of at least 4 KB without symbols.
func symbolRuns(syms []*Symbol, limit int) [][2]int {
	const gap = 0x1000
	var r [][2]int
	for _, s := range syms {
		if s.Length > 0 && int(s.Address)+int(s.Length) <= limit {
			r = append(r, [2]int{int(s.Address), int(s.Address) + int(s.Length)})
		}
	}
	sort.Slice(r, func(i, j int) bool { return r[i][0] < r[j][0] })
	runs := r[:0]
	for _, x := range r {
		if n := len(runs); n > 0 && x[0]-runs[n-1][1] < gap {
			runs[n-1][1] = max(runs[n-1][1], x[1])
		} else {
			runs = append(runs, x)
		}
	}
	return runs
}

// symbolSpan returns the [lo,hi) range covering every symbol stored in flash.
func symbolSpan(syms []*Symbol, limit int) [2]int {
	lo, hi := limit, 0
	for _, s := range syms {
		if s.Length == 0 || int(s.Address)+int(s.Length) > limit {
			continue
		}
		lo = min(lo, int(s.Address))
		hi = max(hi, int(s.Address)+int(s.Length))
	}
	return [2]int{lo, hi}
}

// fingerprint hashes data in fixed-offset blocks with the skipped ranges
// zeroed, so blocks stay aligned between binaries whatever their
// calibration layout.
func fingerprint(data []byte, skip [][2]int) string {
	code := slices.Clone(data)
	masked := make([]int, len(data)/fingerprintBlock+1) // masked bytes per block
	for _, r := range skip {
		for i := max(r[0], 0); i < min(r[1], len(code)); i++ {
			code[i] = 0
			masked[i/fingerprintBlock]++
		}
	}
	var out strings.Builder
	for i := 0; i < len(code); i += fingerprintBlock {
		end := min(i+fingerprintBlock, len(code))
		if masked[i/fingerprintBlock] == end-i {
			out.WriteString(maskedBlock)
			continue
		}
		fmt.Fprintf(&out, "%08x", crc32.ChecksumIEEE(code[i:end]))
	}
	return out.String()
}

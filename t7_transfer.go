package symbol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/roffe/ecusymbol/kmp"
)

var t7SymbolTableMarker = []byte("sYMBOLtABLE\x00")

// t7SymbolTableSpan locates the symbol address table: start offset (first
// 10-byte record) and record count. Layout per symhead2.h: marker, DC.L delta,
// optional u16 NUMBEROFSYMBOLS, records until the "SC" terminator.
func t7SymbolTableSpan(data []byte) (marker, start, count int, err error) {
	marker = bytes.Index(data, t7SymbolTableMarker)
	if marker < 0 {
		return 0, 0, 0, errors.New("t7: symbol table marker not found")
	}
	pat := kmp.BytePatternSearch(data, searchPattern, 0x30000)
	if pat < 0 {
		return 0, 0, 0, ErrAddressTableOffsetNotFound
	}
	start = pat - 6
	for p := start; p+10 <= len(data); p += 10 {
		if data[p] == 'S' && data[p+1] == 'C' {
			break
		}
		count++
	}
	return marker, start, count, nil
}

// TransferSymbolTable copies the symbol address table (and, when the target's
// is missing, the compressed name table) from donor into t7. Tuners poison
// table entries to hide maps; code and data are untouched, so a clean table
// from any bin of the same software trace restores them. Both files must share
// the trace layout: same marker offset, same RAM/ROM delta, same record count.
// The caller must save the result to a new file; the checksum is updated.
func (t7 *T7File) TransferSymbolTable(donor []byte) error {
	if len(donor) != len(t7.data) {
		return ErrInvalidLength
	}
	dm, ds, dc, err := t7SymbolTableSpan(donor)
	if err != nil {
		return fmt.Errorf("donor: %w", err)
	}
	tm := bytes.Index(t7.data, t7SymbolTableMarker) // a poisoned table may defeat the pattern search, so only need the marker
	if tm < 0 {
		return errors.New("target: symbol table marker not found")
	}
	// ponytail: same trace == same marker offset + delta; the address table start
	// is then also identical, so ignore the (possibly poisoned) target table.
	if dm != tm || !bytes.Equal(donor[dm+12:dm+18], t7.data[tm+12:tm+18]) {
		return errors.New("t7: donor is not the same software trace (symbol table marker/delta/count differ)")
	}
	copy(t7.data[ds:ds+dc*10+2], donor[ds:ds+dc*10+2]) // records + "SC"

	// record 0 points at the compressed name table; restore it if the target's is blank
	naddr := int(binary.BigEndian.Uint32(donor[ds:]))
	nlen := int(binary.BigEndian.Uint16(donor[ds+4:]))
	if naddr > 0 && naddr+nlen <= len(donor) && bytes.HasPrefix(t7.data[naddr:], []byte{0xFF, 0xFF, 0xFF, 0xFF}) {
		copy(t7.data[naddr:naddr+nlen], donor[naddr:naddr+nlen])
	}

	symbols, err := loadT7Symbols(t7.data, t7.printFunc)
	if err != nil {
		return err
	}
	t7.Collection = symbols
	return t7.UpdateChecksum()
}

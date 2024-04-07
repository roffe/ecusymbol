package symbol

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/roffe/ecusymbol/blowfish"
	"github.com/roffe/ecusymbol/lzhuf"
)

const (
	SIGNED   = 0x01 /* signed flag in type */
	KONST    = 0x02 /* konstant flag in type */
	CHAR     = 0x04 /* character flag in type */
	LONG     = 0x08 /* long flag in type */
	BITFIELD = 0x10 /* bitfield flag in type */
	STRUCT   = 0x20 /* struct flag in type */
)

func ExpandCompressedSymbolNames(in []byte) ([]string, error) {
	if len(in) < 0x1000 {
		return nil, errors.New("invalid symbol table size")
	}

	if bytes.HasPrefix(in, []byte{0xF1, 0x1A, 0x06, 0x5B, 0xA2, 0x6B, 0xCC, 0x6F}) {
		return blowfish.DecryptSymbolNames(in)
	}

	expandedFileSize := int(in[0]) | (int(in[1]) << 8) | (int(in[2]) << 16) | (int(in[3]) << 24)

	if expandedFileSize == -1 {
		return nil, errors.New("invalid expanded file size")
	}

	out := make([]byte, expandedFileSize)
	returnedSize := lzhuf.Decode(in, out)

	if returnedSize != expandedFileSize {
		return nil, fmt.Errorf("decoded data size missmatch: %d != %d", returnedSize, expandedFileSize)
	}

	return strings.Split(string(out), "\r\n"), nil
}

func CString(data []byte) string {
	n := -1
	for i, v := range data {
		if v == 0 {
			n = i
			break
		}
	}
	// If there was no null byte, convert the whole slice.
	if n == -1 {
		n = len(data)
	}
	return string(data[:n])
}

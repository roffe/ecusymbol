package symbol

import (
	"bytes"
	"fmt"
	"os"
)

const (
	MaxFileLength    = 0x200000
	MagicBytesToRead = 10
)

var ecuMAP = map[ECUType]func(file *os.File, size int64) error{
	ECU_T7: IsTrionic7File,
	ECU_T8: IsTrionic8File,
}

func DetectType(filename string) (ECUType, error) {
	// Check file exists
	fi, err := os.Stat(filename)
	if err != nil {
		return ECU_UNKNOWN, err
	}

	// Check file size
	if fi.Size() > MaxFileLength {
		return ECU_UNKNOWN, ErrToLarge
	}

	// Open file
	f, err := os.Open(filename)
	if err != nil {
		return ECU_UNKNOWN, err
	}
	defer f.Close()

	// Check file type
	for typ, isType := range ecuMAP {
		if err := isType(f, fi.Size()); err == nil {
			return typ, nil
		}
	}

	// Unknown file type
	return ECU_UNKNOWN, fmt.Errorf("unknown file type")
}

func IsTrionic8File(file *os.File, size int64) error {
	if size != T8Length {
		return ErrInvalidLength
	}
	return fileHasPrefix(file, []byte{0x00, 0x10, 0x0C, 0x00})
}

func IsTrionic7File(file *os.File, size int64) error {
	if size != T7Length {
		return ErrInvalidLength
	}
	return fileHasPrefix(file, []byte{0xFF, 0xFF, 0xEF, 0xFC, 0x00})
}

func fileHasPrefix(file *os.File, prefix []byte) error {
	data := make([]byte, len(prefix))
	if _, err := file.ReadAt(data, 0); err != nil {
		return err
	}
	if !bytes.HasPrefix(data, prefix) {
		return ErrMagicBytesNotFound
	}
	return nil
}

package symbol

import (
	"bytes"
	"fmt"
)

const (
	MaxFileLength    = 0x200000
	MagicBytesToRead = 10
)

var ecuMAP = map[ECUType]func(data []byte) error{
	ECU_T5: IsTrionic5File,
	ECU_T7: IsTrionic7File,
	ECU_T8: IsTrionic8File,
}

func DetectType(data []byte) (ECUType, error) {
	// Check file size
	if len(data) > MaxFileLength {
		return ECU_UNKNOWN, ErrToLarge
	}

	// Check file type
	for typ, isType := range ecuMAP {
		if err := isType(data); err == nil {
			return typ, nil
		}
	}

	// Unknown file type
	return ECU_UNKNOWN, fmt.Errorf("unknown file type")
}

func IsTrionic5File(data []byte) error {
	if len(data) != LengthT55 {
		return ErrInvalidLength
	}
	//return fileHasPrefix(file, T5MagicBytes)
	return nil
}

func IsTrionic7File(data []byte) error {
	if len(data) != T7Length {
		return ErrInvalidLength
	}
	return dataHasPrefix(data, T7MagicBytes)
}

func IsTrionic8File(data []byte) error {
	if len(data) != T8Length {
		return ErrInvalidLength
	}
	return dataHasPrefix(data, T8MagicBytes)
}

func dataHasPrefix(data []byte, prefix []byte) error {
	if !bytes.HasPrefix(data, prefix) {
		return ErrMagicBytesNotFound
	}
	return nil
}

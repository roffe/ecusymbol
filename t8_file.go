package symbol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log"
)

type T8File struct {
	data []byte // the raw data
	*Collection

	autoCorrect bool
	printFunc   func(string, ...any)
}

type T8FileOpt func(*T8File) error

func WithAutoCorrect() T8FileOpt {
	return func(t8 *T8File) error {
		t8.autoCorrect = true
		return nil
	}
}

func WithT8PrintFunc(f func(string, ...any)) T8FileOpt {
	return func(t8 *T8File) error {
		t8.printFunc = f
		return nil
	}
}

var _ SymbolCollection = &T8File{}

func NewT8File(data []byte, opts ...T8FileOpt) (*T8File, error) {
	t8 := &T8File{
		data: data,
		printFunc: func(format string, v ...any) {
			log.Printf(format, v...)
		},
	}
	for _, opt := range opts {
		if err := opt(t8); err != nil {
			return nil, err
		}
	}
	return t8.init()
}

func (t8 *T8File) init() (*T8File, error) {

	col, err := LoadT8Symbols(t8.data, func(s string) {
		t8.printFunc(s)
	})
	if err != nil {
		return nil, err
	}
	t8.Collection = col

	if err := t8.VerifyChecksum(); err != nil {
		return nil, err
	}

	return t8, nil
}

func (t8 *T8File) VerifyChecksum() error {
	offset, err := t8.GetChecksumAreaOffset(t8.data)
	if err != nil {
		return err
	}

	t8.printFunc("checksum offset: %04X", offset)

	if len(t8.data) != 0x100000 {
		return errors.New("wrong file size")
	}

	// Calculate checksum
	var checksum int32
	for _, b := range t8.data[:offset] {
		checksum += int32(b)
	}

	// Read checksum from file
	var fileChecksum int32
	err = binary.Read(bytes.NewReader(t8.data[offset:offset+4]), binary.BigEndian, &fileChecksum)
	if err != nil {
		return err
	}

	if checksum != fileChecksum {
		log.Printf("checksum mismatch: %x != %x", checksum, fileChecksum)
	}

	return nil
}

func (t8 *T8File) GetChecksumAreaOffset(data []byte) (int, error) {
	const offset = 0x20140
	if len(data) < offset+4 {
		return 0, errors.New("data is too short")
	}

	// Read bytes and convert to int
	var retval int32
	err := binary.Read(bytes.NewReader(data[offset:offset+4]), binary.BigEndian, &retval)
	if err != nil {
		return 0, err
	}

	return int(retval), nil
}

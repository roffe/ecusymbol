package symbol

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
)

const (
	T8Length = 0x100000
)

var T8MagicBytes = []byte{0x00, 0x10, 0x0C, 0x00}

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

func NewT8File(data []byte, opts ...T8FileOpt) (*T8File, error) {
	if len(data) != T8Length {
		return nil, ErrInvalidLength
	}
	if !bytes.HasPrefix(data, T8MagicBytes) {
		return nil, ErrMagicBytesNotFound
	}

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
	if err := t8.VerifyChecksum(); err != nil {
		return nil, err
	}

	col, err := loadT8Symbols(t8.data, func(s string) {
		t8.printFunc(s)
	})
	if err != nil {
		return nil, err
	}
	t8.Collection = col

	return t8, nil
}

func (t8 *T8File) Bytes() []byte {
	return t8.data
}

func (t8 *T8File) Save(filename string) error {
	for _, sym := range t8.Symbols() {
		if sym.Address == 0 {
			continue
		}
		if sym.Address > 0x100000 { //+32768 {
			if sym.Address > uint32(len(t8.data)) {
				//log.Printf("%s: addr %X sram offset %X", sym.Name, sym.Address, sym.SramOffset)
				//return ErrAddressOutOfRange
				continue
			}
		}
		copy(t8.data[sym.Address:sym.Address+uint32(len(sym.data))], sym.data)
	}

	if err := t8.VerifyChecksum(); err != nil {
		return err
	}

	err := os.WriteFile(filename, t8.data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (t8 *T8File) Version() string {
	sym := t8.GetByName("ECUIDCal.ApplicationFileName")
	if sym == nil {
		return "unknown"
	}
	return strings.TrimSpace(sym.CString())
}

func (t8 *T8File) VerifyChecksum() error {
	if len(t8.data) != 0x100000 {
		return ErrInvalidLength
	}

	offset, err := t8.GetChecksumAreaOffset(t8.data)
	if err != nil {
		return err
	}

	t8.printFunc("Checksum area offset: %08X", offset)

	crc, err := t8.GetChecksumInFile(offset)
	if err != nil {
		return err
	}

	calculatedCrc, err := t8.CalculateLayer1ChecksumMD5(offset)
	if err != nil {
		return err
	}

	t8.printFunc("L1 checksum: %X", crc)
	t8.printFunc("L1 calculated checksum: %X", calculatedCrc)

	if !bytes.Equal(crc, calculatedCrc) {
		t8.printFunc("L1 checksum was invalid, should be updated!")
		if t8.autoCorrect {
			if err := t8.setL1Checksum(offset, calculatedCrc); err != nil {
				return err
			}
			log.Println("L1 checksum updated successfully")
		} else {
			return fmt.Errorf("L1 Checksum mismatch: %X != %X", crc, calculatedCrc)
		}
	} else {
		t8.printFunc("L1 checksum is valid")
	}

	return t8.CalculateLayer2Checksum(offset)
}

func (t8 *T8File) setL1Checksum(offset int, hash []byte) error {
	if len(hash) != 16 {
		return errors.New("invalid hash length")
	}
	copy(t8.data[offset+2:offset+2+16], hash)
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

func (t8 *T8File) GetChecksumInFile(offset int) ([]byte, error) {
	if len(t8.data) < offset+18 {
		return nil, errors.New("data is too short")
	}
	return t8.data[offset+2 : offset+2+16], nil
}

// Calculate checksum
func (t8 *T8File) CalculateLayer1ChecksumMD5(offset int) ([]byte, error) {
	areaEnd := 0x20000 + offset - 0x20000

	//t8.printFunc("L1 calculating from 0x20000 to %08X", areaEnd)

	checksum := md5.New()
	checksum.Write(t8.data[0x20000:areaEnd])
	finalHash := checksum.Sum(nil)

	for i := range finalHash {
		finalHash[i] ^= 0x21
		finalHash[i] -= 0xD6
	}

	return finalHash, nil
}

func (t8 *T8File) CalculateLayer2Checksum(offset int) error {
	const MatrixDimensionLimit = 0x020000
	var checksum0, checksum1, sum0, matrixDimension, partialAddress, x uint32
	var index int
	var chkFound bool

	// Prepare coded_buffer
	codedBuffer := make([]byte, 0x100)
	copy(codedBuffer, t8.data[offset:offset+0x100])
	for i := 0; i < len(codedBuffer); i++ {
		codedBuffer[i] = (codedBuffer[i] + 0xD6) ^ 0x21
	}

	// Search for checksum information in coded_buffer
	for index = 0; index < 0x100; index++ {
		if codedBuffer[index] == 0xFB && codedBuffer[index+6] == 0xFC && codedBuffer[index+12] == 0xFD {
			sum0 = binary.BigEndian.Uint32(codedBuffer[index+1 : index+5])
			matrixDimension = binary.BigEndian.Uint32(codedBuffer[index+7 : index+11])
			partialAddress = binary.BigEndian.Uint32(codedBuffer[index+13 : index+17])

			if matrixDimension >= MatrixDimensionLimit {
				checksum0 = 0
				x = partialAddress

				for x < (matrixDimension - 4) {
					checksum0 += uint32(t8.data[x])
					x++
				}
				checksum0 += uint32(t8.data[matrixDimension-1])

				checksum1 = 0
				x = partialAddress
				for x < (matrixDimension - 4) {
					checksum1 += binary.BigEndian.Uint32(t8.data[x : x+4])
					x += 4
				}

				if (checksum0 & 0xFFF00000) != (sum0 & 0xFFF00000) {
					checksum0 = checksum1
				}

				if checksum0 != sum0 {
					t8.printFunc("L2 checksum was invalid, should be updated!")
					if t8.autoCorrect {
						err := t8.UpdateLayer2(offset, checksum0, index)
						if err != nil {
							return errors.New("L2 checksum was invalid, autocorrection failed: " + err.Error())
						}
						t8.printFunc("Layer 2 checksum updated successfully")
						chkFound = true
					} else {
						return errors.New("L2 checksum was invalid, update required")
					}
				} else {
					chkFound = true
					break
				}
			}
		}
	}

	if !chkFound {
		return errors.New("L2 checksum could not be calculated [file incompatible]")
	}
	t8.printFunc("L2 checksum: %08X", sum0)
	t8.printFunc("L2 calculated checksum: %08X", checksum0)
	t8.printFunc("L2 checksum is valid")
	return nil
}

func (t8 *T8File) UpdateLayer2(offsetLayer2 int, checksum0 uint32, index int) error {
	checksumToFile := make([]byte, 4)
	binary.BigEndian.PutUint32(checksumToFile, checksum0)

	for i := range checksumToFile {
		checksumToFile[i] = ((checksumToFile[i] ^ 0x21) - 0xD6) & 0xFF
	}

	copyPosition := index + offsetLayer2 + 1
	if copyPosition+4 <= len(t8.data) {
		copy(t8.data[copyPosition:copyPosition+4], checksumToFile)
		return nil // Successfully updated
	}
	return errors.New("update failed: index out of range or insufficient data length")
}

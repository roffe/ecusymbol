package symbol

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/roffe/ecusymbol/kmp"
)

const (
	T7Length      = 0x80000
	T7SRAMAddress = 0x0F00000
)

var T7MagicBytes = []byte{0xFF, 0xFF, 0xEF, 0xFC, 0x00}

type T7File struct {
	autoFixFooter bool
	data          []byte // the raw data

	chassisIDDetected           bool
	immocodeDetected            bool
	symbolTableMarkerDetected   bool
	symbolTableChecksumDetected bool
	f2ChecksumDetected          bool
	chassisIDCounter            int

	symbolTableAddress int
	sramOffset         int
	checksumF2         int
	checksumFB         int

	bottomOfFlash   int
	romChecksumType int
	fwLength        int

	valueF5          int
	valueF6          int
	valueF7          int
	valueF8          int
	romChecksumError byte

	chassisID       string
	immobilizerID   string
	softwareVersion string
	carDescription  string
	partNumber      string
	engineType      string
	vehicleIDNr     string
	dateModified    string
	ecuHardwareNr   string
	testserialnr    string
	lastModifiedBy  []byte

	csumArea [16]T7ChecksumArea

	printFunc func(string)

	*Collection // the symbol collection
}

type T7FileOpt func(*T7File) error

func WithT7AutoFixFooter() T7FileOpt {
	return func(t7 *T7File) error {
		t7.autoFixFooter = true
		return nil
	}
}

func WithT7PrintFunc(f func(string)) T7FileOpt {
	return func(t7 *T7File) error {
		t7.printFunc = f
		return nil
	}
}

func NewT7File(data []byte, opts ...T7FileOpt) (*T7File, error) {
	if len(data) != T7Length {
		return nil, ErrInvalidLength
	}
	if !bytes.HasPrefix(data, T7MagicBytes) {
		return nil, ErrMagicBytesNotFound
	}

	t7 := &T7File{
		data:            data,
		chassisID:       "00000000000000000",
		immobilizerID:   "000000000000000",
		engineType:      "0000000000000",
		vehicleIDNr:     "000000000",
		partNumber:      "0000000",
		softwareVersion: "000000000000",
		carDescription:  "00000000000000000000",
		dateModified:    "0000",
		ecuHardwareNr:   "0000000",
		lastModifiedBy:  []byte{0x42, 0xFB, 0xFA, 0xFF, 0xFF},
		testserialnr:    "050225",
		printFunc: func(str string) {
			log.Println(str)
		},
	}

	for _, opt := range opts {
		if err := opt(t7); err != nil {
			return nil, err
		}
	}

	return t7.parse()
}

func (t7 *T7File) parse() (*T7File, error) {
	symbols, err := loadT7Symbols(t7.data, func(s string) {
		t7.printFunc(s)
	})
	if err != nil {
		return nil, err
	}
	t7.Collection = symbols
	t7.loadHeaders()
	return t7, t7.VerifyChecksum()
}

func (t7 *T7File) findESPCalibrationPos() int {
	return kmp.BytePatternSearch(t7.data, []byte{0xF0, 0x03, 0x34, 0x4e, 0x75}, 0)
}

func (t7 *T7File) GetESPCalibration() byte {
	pos := t7.findESPCalibrationPos()
	if pos == -1 {
		return 0
	}
	return t7.data[pos+5]
}

func (t7 *T7File) SetESPCalibration(val byte) {
	pos := t7.findESPCalibrationPos()
	if pos == -1 {
		return
	}
	t7.data[pos+5] = val
}

func (t7 *T7File) UpdateSymbol(sym *Symbol) error {
	addr := sym.Address
	if sym.Address > 0x7FFFFF {
		if sym.Address-sym.SramOffset > uint32(len(t7.data)) {
			return ErrAddressOutOfRange
		}
		addr = sym.Address - sym.SramOffset
	}

	for i, b := range sym.data {
		(t7.data)[addr+uint32(i)] = b
	}

	return nil
}

func (t7 *T7File) Bytes() []byte {
	return t7.data
}

func (t7 *T7File) Save(filename string) error {
	for _, sym := range t7.Symbols() {
		addr := sym.Address
		if sym.Address > 0x7FFFFF {
			if sym.Address-sym.SramOffset > uint32(len(t7.data)) {
				return ErrAddressOutOfRange
			}
			addr = sym.Address - sym.SramOffset
		}
		for idx, b := range sym.data {
			(t7.data)[addr+uint32(idx)] = b
		}
		//copy(t7.data[addr:addr+uint32(len(sym.data))], sym.data)

	}

	if err := t7.UpdateChecksum(); err != nil {
		return err
	}

	if err := t7.VerifyChecksum(); err != nil {
		return err
	}

	err := os.WriteFile(filename, t7.data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write %s : %w", filename, err)
	}
	return nil
}

func (t7 *T7File) Version() string {
	return t7.softwareVersion
}

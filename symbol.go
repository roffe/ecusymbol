package symbol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
)

type Symbol struct {
	data []byte

	Name             string
	Number           int
	SramOffset       uint32
	Address          uint32
	Length           uint16
	Mask             uint16
	Type             uint8
	ExtendedType     uint8
	Correctionfactor float64
	Unit             string `json:",omitempty"`
	Skip             bool   `json:"-"`
}

func Load(filename string, printFunc func(string)) (ECUType, SymbolCollection, error) {
	ecuType, err := DetectType(filename)
	if err != nil {
		return ECU_UNKNOWN, nil, err
	}

	printFunc(fmt.Sprintf("Loading %s", filepath.Base(filename)))

	data, err := os.ReadFile(filename)
	if err != nil {
		return -1, nil, err
	}

	switch ecuType {
	case ECU_T5:
		sym, err := NewT5File(
			data,
			WithT5PrintFunc(printFunc),
		)
		return ECU_T5, sym, err
	case ECU_T7:
		sym, err := NewT7File(data,
			WithT7AutoFixFooter(),
			WithT7PrintFunc(printFunc),
		)
		return ECU_T7, sym, err
	case ECU_T8:
		sym, err := NewT8File(data,
			WithT8AutoCorrectChecksum(),
			WithT8PrintFunc(printFunc),
		)
		return ECU_T8, sym, err
	default:
		return -1, nil, fmt.Errorf("unknown file format: %s", filename)
	}
}

func (s *Symbol) SetData(data []byte) error {
	if len(data) != int(s.Length) {
		return fmt.Errorf("Symbol %s expected %d bytes, got %d", s.Name, s.Length, len(data))
	}
	s.data = data
	return nil
}

func (s *Symbol) Read(r io.Reader) error {
	s.data = make([]byte, s.Length)
	n, err := r.Read(s.data)
	if err != nil {
		return err
	}
	if n != int(s.Length) {
		return fmt.Errorf("symbol expected %d bytes, got %d", s.Length, n)
	}
	return nil
}

/*
	func (s *Symbol) Decode() interface{} {
		switch {
		case s.Length == 1:
			if len(s.data) != 1 {
				return -1
			}
			if s.Type&SIGNED == SIGNED {
				return s.Int8()
			}
			return s.Uint8()
		case s.Length == 2:
			if len(s.data) != 2 {
				return -1
			}
			if s.Type&SIGNED == SIGNED {
				return s.Int16()
			}
			return s.Uint16()
		case s.Length == 4:
			if len(s.data) != 4 {
				return -1
			}
			if s.Type&SIGNED == SIGNED {
				return s.Int32()
			}
			return s.Uint32()
		default:
			return -1
		}
	}
*/

func (s *Symbol) Bytes() []byte {
	return s.data
}

func (s *Symbol) String() string {
	return fmt.Sprintf("%s #%d @%08X $%06X type: %02X len: %d", s.Name, s.Number, s.Address, s.SramOffset, s.Type, s.Length)
}

func (s *Symbol) CString() string {
	n := -1
	for i, v := range s.data {
		if v == 0 {
			n = i
			break
		}
	}
	// If there was no null byte, convert the whole slice.
	if n == -1 {
		n = len(s.data)
	}
	return string(s.data[:n])
}

func (s *Symbol) StringValue() string {
	var precission int
	switch s.Correctionfactor {
	case 0.1:
		precission = 1
	case 0.01, 0.0078125, 0.0009765625, 0.00390625, 0.004:
		precission = 2
	case 0.001:
		precission = 3
	default:
		precission = 0
	}
	return strconv.FormatFloat(s.Float64(), 'f', precission, 64)
}

func (s *Symbol) Bool() bool {
	return (s.data)[0] == 1
}

func (s *Symbol) Uint8() uint8 {
	return uint8(s.data[0])
}

func (s *Symbol) Int8() int8 {
	return int8(s.data[0])
}

func (s *Symbol) Uint16() uint16 {
	return binary.BigEndian.Uint16(s.data)
}

func (s *Symbol) Int16() int16 {
	return int16(binary.BigEndian.Uint16(s.data))
}

func (s *Symbol) Uint32() uint32 {
	return binary.BigEndian.Uint32(s.data)
}

func (s *Symbol) Int32() int32 {
	return int32(binary.BigEndian.Uint32(s.data))
}

func (s *Symbol) Uint64() uint64 {
	return binary.BigEndian.Uint64(s.data)
}

func (s *Symbol) Int64() int64 {
	return int64(binary.BigEndian.Uint64(s.data))
}

func (s *Symbol) Float64s() []float64 {
	var floats []float64
	for _, v := range s.Ints() {
		//log.Printf("%f", T5Offsets[s.Name])
		floats = append(floats, (float64(v)*s.Correctionfactor)+T5Offsets[s.Name])
	}
	return floats
}

func (s *Symbol) Float64() float64 {
	if len(s.data) != int(s.Length) {
		return -1
	}

	var val int64
	switch s.Length {
	case 1:
		if s.Type&SIGNED != 0 {
			val = int64(int8(s.data[0]))
		} else {
			val = int64(s.data[0])
		}
	case 2:
		if s.Type&SIGNED != 0 {
			val = int64(int16(binary.BigEndian.Uint16(s.data)))
		} else {
			val = int64(binary.BigEndian.Uint16(s.data))
		}
	case 4:
		if s.Type&SIGNED != 0 {
			val = int64(int32(binary.BigEndian.Uint32(s.data)))
		} else {
			val = int64(binary.BigEndian.Uint32(s.data))
		}
	case 8:
		if s.Type&SIGNED != 0 {
			val = int64(binary.BigEndian.Uint64(s.data))
		} else {
			val = int64(binary.BigEndian.Uint64(s.data))
		}
	default:
		return 0.0
	}
	return float64(val) * s.Correctionfactor
}

func (s *Symbol) Float642() float64 {
	switch {
	case s.Length == 1:
		if len(s.data) != 1 {
			return -1
		}
		if s.Type&SIGNED != 0 {
			return float64(s.Int8()) * s.Correctionfactor
		}
		return float64(s.Uint8()) * s.Correctionfactor
	case s.Length == 2:
		if len(s.data) != 2 {
			return -1
		}
		if s.Type&SIGNED != 0 {
			return float64(s.Int16()) * s.Correctionfactor
		}
		return float64(s.Uint16()) * s.Correctionfactor
	case s.Length == 4:
		if len(s.data) != 4 {
			return -1
		}
		if s.Type&SIGNED != 0 {
			return float64(s.Int32()) * s.Correctionfactor
		}
		return float64(s.Uint32()) * s.Correctionfactor
	case s.Length == 8:
		if len(s.data) != 8 {
			return -1
		}
		if s.Type&SIGNED != 0 {
			return float64(s.Int64()) * s.Correctionfactor
		}
		return float64(s.Uint64()) * s.Correctionfactor
	default:
		return 0.0
	}
}

func (s *Symbol) Int() int {
	switch {
	case s.Length == 1:
		if len(s.data) != 1 {
			return -1
		}
		if s.Type&SIGNED != 0 {
			return int(s.Int8())
		}
		return int(s.Uint8())
	case s.Length == 2:
		if len(s.data) != 2 {
			return -1
		}
		if s.Type&SIGNED != 0 {
			return int(s.Int16())
		}
		return int(s.Uint16())
	case s.Length == 4:
		if len(s.data) != 4 {
			return -1
		}
		if s.Type&SIGNED != 0 {
			return int(s.Int32())
		}
		return int(s.Uint32())
	case s.Length == 8:
		if len(s.data) != 8 {
			return -1
		}
		if s.Type&SIGNED != 0 {
			return int(s.Int64())
		}
		return int(s.Uint64())
	default:
		return 0.0
	}
}

func (s *Symbol) BytesToFloat64s(data []byte) []float64 {
	var floats []float64
	for _, v := range s.BytesToInts(data) {
		floats = append(floats, (float64(v)*s.Correctionfactor)+T5Offsets[s.Name])
	}
	return floats
}

func (s *Symbol) BytesToInts(data []byte) []int {
	signed := s.Type&SIGNED == SIGNED
	char := s.Type&CHAR == CHAR
	long := s.Type&LONG == LONG
	var ints []int
	r := bytes.NewReader(data)
	switch {
	case signed && char:
		// log.Println("int8")
		x := make([]int8, s.Length)
		if err := binary.Read(r, binary.BigEndian, &x); err != nil {
			log.Println(err)
		}
		for _, v := range x {
			ints = append(ints, int(v))
		}
	case !signed && char:
		// log.Println("uint8")
		x := make([]uint8, s.Length)
		if err := binary.Read(r, binary.BigEndian, &x); err != nil {
			log.Println(err)
		}
		for _, v := range x {
			ints = append(ints, int(v))
		}
	case signed && !char && !long:
		// log.Println("int16")
		x := make([]int16, s.Length/2)
		if err := binary.Read(r, binary.BigEndian, &x); err != nil {
			log.Println(err)
		}
		for _, v := range x {
			ints = append(ints, int(v))
		}
	case !signed && !char && !long:
		// log.Println("uint16")
		x := make([]uint16, s.Length/2)
		if err := binary.Read(r, binary.BigEndian, &x); err != nil {
			log.Println(err)
		}
		for _, v := range x {
			ints = append(ints, int(v))
		}
	case signed && !char && long:
		// log.Println("int32")
		x := make([]uint32, s.Length/4)
		if err := binary.Read(r, binary.BigEndian, &x); err != nil {
			log.Println(err)
		}
		for _, v := range x {
			ints = append(ints, int(v))
		}
	case !signed && !char && long:
		// log.Println("uint32")
		x := make([]uint32, s.Length/4)
		if err := binary.Read(r, binary.BigEndian, &x); err != nil {
			log.Println(err)
		}
		for _, v := range x {
			ints = append(ints, int(v))
		}
	default:
		// log.Println("uint16")
		x := make([]uint16, s.Length/2)
		if err := binary.Read(r, binary.BigEndian, &x); err != nil {
			log.Println(err)
		}
		for _, v := range x {
			ints = append(ints, int(v))
		}
	}
	return ints
}

func (s *Symbol) EncodeInt(input int) []byte {
	//signed := s.Type&SIGNED == SIGNED
	//konst := s.Type&KONST == KONST
	char := s.Type&CHAR == CHAR
	long := s.Type&LONG == LONG
	switch {
	case char && !long:
		return []byte{byte(input)}
	case !char && !long:
		return []byte{byte(input >> 8), byte(input)}
	case long && !char:
		return []byte{byte(input >> 24), byte(input >> 16), byte(input >> 8), byte(input)}
	default:
		return []byte{byte(input >> 8), byte(input)}
	}
}

func (s *Symbol) EncodeFloat64(v float64) []byte {
	newValue := int(math.Round((v - T5Offsets[s.Name]) / s.Correctionfactor))
	//log.Printf("(%f - %f) / %f = %d", v, T5Offsets[s.Name], s.Correctionfactor, newValue)
	return s.EncodeInt(newValue)
}

func (s *Symbol) EncodeInts(input []int) []byte {
	//signed := s.Type&SIGNED == SIGNED
	//konst := s.Type&KONST == KONST
	char := s.Type&CHAR == CHAR
	long := s.Type&LONG == LONG
	buf := bytes.NewBuffer(nil)
	for _, v := range input {
		switch {
		case char && !long:
			buf.Write([]byte{byte(v)})
		case !char && !long:
			buf.Write([]byte{byte(v >> 8), byte(v)})
		case long && !char:
			buf.Write([]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
		default:
			buf.Write([]byte{byte(v >> 8), byte(v)})
		}
	}
	return buf.Bytes()
}

func (s *Symbol) EncodeFloat64s(input []float64) []byte {
	buf := bytes.NewBuffer(nil)
	for _, value := range input {
		buf.Write(s.EncodeFloat64(value))
	}
	return buf.Bytes()
}

func (s *Symbol) Ints() []int {
	signed := s.Type&SIGNED == SIGNED
	//konst := s.Type&KONST == KONST
	char := s.Type&CHAR == CHAR
	long := s.Type&LONG == LONG
	//log.Printf("Ints From Data %s signed: %t konst: %t char: %t long: %t len: %d: type %X", s.Name, signed, konst, char, long, s.Length, s.Type)

	switch {
	case char && !signed:
		//log.Println("uint8")
		return s.Uint8s()
	case char && signed:
		//log.Println("int8")
		return s.Int8s()
	case !char && !long && !signed:
		//log.Println("uint16")
		return s.Uint16s()
	case !char && !long && signed:
		//log.Println("int16")
		return s.Int16s()
	case !char && long && !signed:
		//log.Println("uint32")
		return s.Uint32s()
	case !char && long && signed:
		//log.Println("int32")
		return s.Int32s()
	}
	//log.Println("xint16")
	return s.Int16s()
}

func (s *Symbol) Int8s() []int {
	values := make([]int, 0, len(s.data))
	for _, b := range s.data {
		values = append(values, int(int8(b)))
	}
	return values
}

func (s *Symbol) Uint8s() []int {
	values := make([]int, 0, len(s.data))
	for _, b := range s.data {
		values = append(values, int(uint8(b)))
	}
	return values
}

func (s *Symbol) Uint16s() []int {
	if len(s.data)%2 != 0 {
		log.Panicf("data length is not even: %d", len(s.data))
	}
	values := make([]int, 0, len(s.data)/2)
	for i := 0; i < len(s.data); i += 2 {
		value := binary.BigEndian.Uint16((s.data)[i : i+2])
		values = append(values, int(value))
	}
	return values
}

func (s *Symbol) Int16s() []int {
	if len(s.data)%2 != 0 {
		log.Panicf("data length is not even: %d", len(s.data))
	}
	values := make([]int, 0, len(s.data)/2)
	for i := 0; i < len(s.data); i += 2 {
		value := int16(binary.BigEndian.Uint16((s.data)[i : i+2]))
		values = append(values, int(value))
	}
	return values
}

func (s *Symbol) Uint32s() []int {
	if len(s.data)%4 != 0 {
		log.Panicf("data length is not even: %d", len(s.data))
	}
	values := make([]int, 0, len(s.data)/4)
	for i := 0; i < len(s.data); i += 4 {
		value := binary.BigEndian.Uint32(s.data[i : i+4])
		values = append(values, int(value))
	}
	return values
}

func (s *Symbol) Int32s() []int {
	if len(s.data)%4 != 0 {
		log.Panicf("data length is not even: %d", len(s.data))
	}
	values := make([]int, 0, len(s.data)/4)
	for i := 0; i < len(s.data); i += 4 {
		value := int32(binary.BigEndian.Uint32((s.data)[i : i+4]))
		values = append(values, int(value))
	}
	return values
}

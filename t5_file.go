package symbol

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
)

const (
	LengthT52 = 0x20000
	LengthT55 = 0x40000
)

var T5MagicBytes = []byte{0xFF, 0xFF, 0xF7, 0xFC, 0x00}

type T5File struct {
	data                      []byte
	numberOfSymbols           int
	m_symboltablestartaddress int
	*Collection
}

type T5FileOpt func(*T5File) error

func NewT5File(data []byte, opts ...T5FileOpt) (*T5File, error) {
	if len(data) != LengthT55 {
		return nil, ErrInvalidLength
	}

	if !bytes.HasPrefix(data, T5MagicBytes) {
		return nil, ErrMagicBytesNotFound
	}

	t5 := &T5File{
		data:       data,
		Collection: NewCollection(),
	}

	for _, opt := range opts {
		if err := opt(t5); err != nil {
			return nil, err
		}
	}

	return t5.init()
}

func (t5 *T5File) init() (*T5File, error) {
	if err := t5.parseData(); err != nil {
		return nil, err
	}
	return t5, nil
}

func (t5 *T5File) parseData() error {
	var state int = -10
	var charcount int
	buff := bytes.NewBuffer(nil)
outer:
	for t, b := range t5.data {
		switch state {
		case -10:
			if b == 0 {
				state++
			}
		case -9:
			if b == 0x0a {
				state++
			} else {
				state = -10
			}
		case -8:
			if b == 0x28 {
				state++
			} else {
				state = -10
			}
		case -7:
			if b == 0x79 {
				state++
			} else {
				state = -10
			}
		case -6:
			if b == 0x00 {
				state++
			} else {
				state = -10
			}
		case -5:
			if b == 0x4E {
				state++
			}
		case -4:
			if b == 0x75 {
				state = 2
			} else {
				state = -5
			}
		case 0:
			if b == 0x0d {
				state++
			}
		case 1:
			if b == 0x0a {
				state++
				charcount = 0
			} else {
				state = 0
			}
		case 2:
			if charcount < 32 {
				if b == 0x0d && t5.data[t+1] == 0x0a { // start of next symbol
					state = 1
					address := t - charcount
					if err := t5.addtosymbolcollection(buff.Bytes()); err != nil {
						return err
					}
					if t5.m_symboltablestartaddress == 0 && t > 0xA000 {
						t5.m_symboltablestartaddress = address
					}
					buff.Reset()
				} else {
					if err := buff.WriteByte(b); err != nil {
						return err
					}
					charcount++
				}
			} else {
				address := t - charcount
				if t5.m_symboltablestartaddress == 0 && t > 0xA000 {
					t5.m_symboltablestartaddress = address
				}
				if bytes.HasPrefix(buff.Bytes(), []byte("END$")) {
					log.Println("END$ found")
					break outer
				} else {
					if err := t5.addtosymbolcollection(buff.Bytes()); err != nil {
						return err
					}
					buff.Reset()
					state = 0
				}
			}
		}
	}
	buff.Reset()

	if err := t5.readAddressLookupTable(); err != nil {
		return err
	}

	for _, sym := range t5.Symbols() {
		if sym.Address == 0 {
			continue
		}
		


		sym.data = make([]byte, sym.Length)
		log.Println(sym.String())
		binary.Read(bytes.NewReader(t5.data[sym.Address:sym.Address+uint32(sym.Length)]), binary.BigEndian, sym.data)
	}

	return nil
}

func (t5 *T5File) readAddressLookupTable() error {
	var readstate int = -30
	var lookuptablestartaddress int
	for t, b := range t5.data {
		switch readstate {
		case -30:
			if b == 0x4E {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -29:
			if b == 0x75 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -28:
			if b == 0x48 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -27:
			if b == 0xE7 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -26:
			if b == 0x01 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -25:
			if b == 0x30 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -24:
			if b == 0x26 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -23:
			if b == 0x6F {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -22:
			if b == 0x00 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -21:
			if b == 0x16 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -20:
			if b == 0x3E {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -19:
			if b == 0x2F {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -18:
			if b == 0x00 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -17:
			if b == 0x14 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -16:
			if b == 0x24 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -15:
			if b == 0x6F {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -14:
			if b == 0x00 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -13:
			if b == 0x10 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -12:
			if b == 0x60 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -11:
			if b == 0x00 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -10:
			if b == 0x00 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}
		case -9:
			if b == 0x0A {
				readstate = 0
			} else {
				lookuptablestartaddress = 0x00
				readstate = -30
			}

		case 0:
			// waiting for first recognition char 48
			if b == 0x48 {
				lookuptablestartaddress = t
				readstate++
			}
		case 1:
			// waiting for second char 79
			if b == 0x79 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = 0
			}
		case 2:
			// waiting for third char 00
			if b == 0x00 {
				readstate++
			} else {
				lookuptablestartaddress = 0x00
				readstate = 0
			}
		case 3:
			// waiting for last char 04
			if len(t5.data) == 0x20000 {
				if b == 0x06 {
					readstate++
				} else {
					lookuptablestartaddress = 0x00
					readstate = 0
				}
			} else {
				if b == 0x04 {
					readstate++
				} else {
					lookuptablestartaddress = 0x00
					readstate = 0
				}
			}
		}
	}

	binPos := lookuptablestartaddress + 2
	br := bytes.NewReader(t5.data)
	if _, err := br.Seek(int64(binPos), io.SeekStart); err != nil {
		return err
	}
	for sc := 0; sc < t5.numberOfSymbols; sc++ {
		if binPos >= t5.m_symboltablestartaddress {
			log.Println("pos is greater than symboltablestartaddress")
			break
		}

		var flashAddress uint32
		if err := binary.Read(br, binary.BigEndian, &flashAddress); err != nil {
			return err
		}
		binPos += 4
		//log.Printf("flashAddress: 0x%08X\n", flashAddress)

		// 8x dummy bytes
		dummy := make([]byte, 8)
		if err := binary.Read(br, binary.BigEndian, dummy); err != nil {
			return err
		}
		//		log.Printf("dummy: %X len: %X other: %X", dummy[:2], dummy[2:4], dummy[4:])
		binPos += 8

		var sramAddress uint16
		if err := binary.Read(br, binary.BigEndian, &sramAddress); err != nil {
			return err
		}
		binPos += 2
		//log.Printf("sramAddress: %06X\n", sramAddress)

		tel := 0
		found := false
		tstate := 0
	outer:
		for tel < 16 && !found {
			tb, err := br.ReadByte()
			if err != nil {
				return err
			}
			binPos++

			//fmt.Printf("%02X ", tb)
			switch tstate {
			case 0:
				if tb == 0x48 {
					tstate++
				}
			case 1:
				if tb == 0x79 {
					found = true
					//		fmt.Println()
					break outer
				} else {
					tstate = 0
				}
			}
			tel++
		}

		ttt := t5.data[binPos-tel : binPos]
		dd := binary.BigEndian.Uint32(t5.data[binPos-tel : binPos+4])
		log.Printf("ttt: %X : %d | 0x%08X", ttt, len(ttt), dd)

		if !found {
			break
		}
		for _, sym := range t5.Symbols() {
			if sym.SramOffset == uint32(sramAddress) {
				sym.Address = flashAddress - uint32(len(t5.data))
				break
			}
		}
	}
	return nil
}

func (t5 *T5File) addtosymbolcollection(data []byte) error {
	var invalidCharCount int
	for _, b := range data[4:] {
		if b < 10 {
			invalidCharCount++
		}
	}
	if invalidCharCount > 2 {
		//		log.Println("Too many invalid chars")
		return nil
	}
	sym := &Symbol{
		Number:     t5.numberOfSymbols,
		SramOffset: uint32(binary.BigEndian.Uint16(data[0:2])),
		Length:     binary.BigEndian.Uint16(data[2:4]),
		Name:       CString(data[4:]),
	}
	//	log.Println(sym.String())
	t5.Add(sym)
	t5.numberOfSymbols++
	return nil
}

package lzhuf

import "encoding/binary"

const nul = bufSize // NIL, leaf of tree

type encoder struct {
	huffman
	out    []byte
	putbuf byte
	putlen int

	textBuf            [bufSize + lookahead - 1]byte
	matchPos, matchLen int
	lson, dad          [bufSize + 1]int
	rson               [bufSize + 257]int
}

func (e *encoder) initTree() {
	for i := bufSize + 1; i <= bufSize+256; i++ {
		e.rson[i] = nul // root
	}
	for i := range bufSize {
		e.dad[i] = nul // node
	}
}

func (e *encoder) insertNode(r int) {
	cmp := 1
	key := e.textBuf[r:]
	p := bufSize + 1 + int(key[0])
	e.rson[r], e.lson[r] = nul, nul
	e.matchLen = 0
	for {
		if cmp >= 0 {
			if e.rson[p] == nul {
				e.rson[p] = r
				e.dad[r] = p
				return
			}
			p = e.rson[p]
		} else {
			if e.lson[p] == nul {
				e.lson[p] = r
				e.dad[r] = p
				return
			}
			p = e.lson[p]
		}
		i := 1
		for ; i < lookahead; i++ {
			if cmp = int(key[i]) - int(e.textBuf[p+i]); cmp != 0 {
				break
			}
		}
		if i > threshold {
			pos := (r-p)&(bufSize-1) - 1
			if i > e.matchLen {
				e.matchPos, e.matchLen = pos, i
				if i >= lookahead {
					break
				}
			} else if i == e.matchLen && pos < e.matchPos {
				e.matchPos = pos
			}
		}
	}
	e.dad[r] = e.dad[p]
	e.lson[r] = e.lson[p]
	e.rson[r] = e.rson[p]
	e.dad[e.lson[p]] = r
	e.dad[e.rson[p]] = r
	if e.rson[e.dad[p]] == p {
		e.rson[e.dad[p]] = r
	} else {
		e.lson[e.dad[p]] = r
	}
	e.dad[p] = nul // remove p
}

func (e *encoder) deleteNode(p int) {
	if e.dad[p] == nul {
		return // not registered
	}
	var q int
	switch {
	case e.rson[p] == nul:
		q = e.lson[p]
	case e.lson[p] == nul:
		q = e.rson[p]
	default:
		q = e.lson[p]
		if e.rson[q] != nul {
			for e.rson[q] != nul {
				q = e.rson[q]
			}
			e.rson[e.dad[q]] = e.lson[q]
			e.dad[e.lson[q]] = e.dad[q]
			e.lson[q] = e.lson[p]
			e.dad[e.lson[p]] = q
		}
		e.rson[q] = e.rson[p]
		e.dad[e.rson[p]] = q
	}
	e.dad[q] = e.dad[p]
	if e.rson[e.dad[p]] == p {
		e.rson[e.dad[p]] = q
	} else {
		e.lson[e.dad[p]] = q
	}
	e.dad[p] = nul
}

// putCode outputs the top l bits of c.
// ponytail: one bit at a time, batch into a wider putbuf if encoding speed ever matters
func (e *encoder) putCode(l int, c uint32) {
	for ; l > 0; l-- {
		e.putbuf = e.putbuf<<1 | byte(c>>31)
		c <<= 1
		if e.putlen++; e.putlen == 8 {
			e.out = append(e.out, e.putbuf)
			e.putlen = 0
		}
	}
}

func (e *encoder) encodeChar(c int) {
	// travel from leaf to root. The original collects the code in 16 bits and
	// corrupts longer codes; 32 bits covers the deepest possible tree.
	var code uint32
	n := 0
	for k := int(e.prnt[c+tableSize]); k != root; k = int(e.prnt[k]) {
		code >>= 1
		// if node's address is odd-numbered, choose bigger brother node
		if k&1 != 0 {
			code |= 1 << 31
		}
		n++
	}
	e.putCode(n, code)
	e.update(c)
}

func (e *encoder) encodePosition(c int) {
	// output upper 6 bits by table lookup
	i := c >> 6
	e.putCode(int(pLen[i]), uint32(pCode[i])<<24)

	// output lower 6 bits verbatim
	e.putCode(6, uint32(c&0x3f)<<26)
}

// Encode compresses in to a 4-byte little-endian decoded size followed by the
// LZHUF stream, byte-identical to the output of the original LZHUF.C.
func Encode(in []byte) []byte {
	e := &encoder{out: binary.LittleEndian.AppendUint32(make([]byte, 0, len(in)/2+8), uint32(len(in)))}
	if len(in) == 0 {
		return e.out
	}
	e.startHuff()
	e.initTree()
	s, r := 0, bufSize-lookahead
	for i := range r {
		e.textBuf[i] = ' '
	}
	length := copy(e.textBuf[r:bufSize], in)
	in = in[length:]
	for i := 1; i <= lookahead; i++ {
		e.insertNode(r - i)
	}
	e.insertNode(r)
	for length > 0 {
		if e.matchLen > length {
			e.matchLen = length
		}
		if e.matchLen <= threshold {
			e.matchLen = 1
			e.encodeChar(int(e.textBuf[r]))
		} else {
			e.encodeChar(255 - threshold + e.matchLen)
			e.encodePosition(e.matchPos)
		}
		last := e.matchLen
		i := 0
		for ; i < last && len(in) > 0; i++ {
			c := in[0]
			in = in[1:]
			e.deleteNode(s)
			e.textBuf[s] = c
			if s < lookahead-1 {
				e.textBuf[s+bufSize] = c
			}
			s = (s + 1) & (bufSize - 1)
			r = (r + 1) & (bufSize - 1)
			e.insertNode(r)
		}
		for ; i < last; i++ {
			e.deleteNode(s)
			s = (s + 1) & (bufSize - 1)
			r = (r + 1) & (bufSize - 1)
			if length--; length > 0 {
				e.insertNode(r)
			}
		}
	}
	if e.putlen > 0 {
		e.out = append(e.out, e.putbuf<<(8-e.putlen))
	}
	return e.out
}

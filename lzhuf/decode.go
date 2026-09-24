// Package lzhuf decodes LZHUF (LZSS + adaptive Huffman) streams. It is a pure
// Go port of the decoder in Yoshizaki/Okumura's LZHUF.C (1989), which SAAB
// used unmodified to compress the Trionic symbol name table.
package lzhuf

const (
	bufSize   = 4096 // N, ring buffer size
	lookahead = 60   // F, lookahead buffer size
	threshold = 2
	nChar     = 256 - threshold + lookahead // kinds of characters
	tableSize = nChar*2 - 1                 // T
	root      = tableSize - 1               // R
	maxFreq   = 0x8000                      // rebuild tree when root frequency reaches this
)

// tables for encoding (pCode, pLen) and decoding (dCode, dLen) the upper 6
// bits of position
var pCode, pLen, dCode, dLen = func() (pCode, pLen [64]byte, dCode, dLen [256]byte) {
	i := 0
	for _, run := range []struct{ codes, width, bits int }{
		{1, 32, 3}, {3, 16, 4}, {8, 8, 5}, {12, 4, 6}, {24, 2, 7}, {16, 1, 8},
	} {
		for range run.codes * run.width {
			dLen[i] = byte(run.bits)
			i++
		}
	}
	c := 0
	for i = 0; i < 256; c++ {
		pCode[c], pLen[c] = byte(i), dLen[i]
		for range 1 << (8 - dLen[i]) {
			dCode[i] = byte(c)
			i++
		}
	}
	return
}()

type decoder struct {
	huffman
	in     []byte
	pos    int
	getbuf uint16
	getlen uint8
}

// huffman is the adaptive Huffman tree shared by the encoder and decoder
type huffman struct {
	freq [tableSize + 1]uint16
	// pointers to parent nodes, except for the elements
	// [tableSize..tableSize+nChar-1] which are used to get the positions of
	// leaves corresponding to the codes.
	prnt [tableSize + nChar]uint16
	son  [tableSize]uint16 // pointers to child nodes (son[], son[]+1)
}

func (d *decoder) fill() {
	for d.getlen <= 8 {
		var b uint16 // reads past the end yield zeros, like getc() == EOF in the original
		if d.pos < len(d.in) {
			b = uint16(d.in[d.pos])
		}
		d.pos++
		d.getbuf |= b << (8 - d.getlen)
		d.getlen += 8
	}
}

func (d *decoder) getBit() int {
	d.fill()
	i := d.getbuf
	d.getbuf <<= 1
	d.getlen--
	return int(i >> 15)
}

func (d *decoder) getByte() int {
	d.fill()
	i := d.getbuf
	d.getbuf <<= 8
	d.getlen -= 8
	return int(i >> 8)
}

func (d *huffman) startHuff() {
	for i := range nChar {
		d.freq[i] = 1
		d.son[i] = uint16(i + tableSize)
		d.prnt[i+tableSize] = uint16(i)
	}
	for i, j := 0, nChar; j <= root; i, j = i+2, j+1 {
		d.freq[j] = d.freq[i] + d.freq[i+1]
		d.son[j] = uint16(i)
		d.prnt[i] = uint16(j)
		d.prnt[i+1] = uint16(j)
	}
	d.freq[tableSize] = 0xffff
	d.prnt[root] = 0
}

func (d *huffman) reconst() {
	// collect leaf nodes in the first half of the table
	// and replace the freq by (freq + 1) / 2.
	j := 0
	for i := range tableSize {
		if d.son[i] >= tableSize {
			d.freq[j] = (d.freq[i] + 1) / 2
			d.son[j] = d.son[i]
			j++
		}
	}
	// begin constructing tree by connecting sons
	for i, j := 0, nChar; j < tableSize; i, j = i+2, j+1 {
		f := d.freq[i] + d.freq[i+1]
		k := j
		for f < d.freq[k-1] {
			k--
		}
		copy(d.freq[k+1:j+1], d.freq[k:j])
		d.freq[k] = f
		copy(d.son[k+1:j+1], d.son[k:j])
		d.son[k] = uint16(i)
	}
	// connect prnt
	for i := range tableSize {
		k := d.son[i]
		d.prnt[k] = uint16(i)
		if k < tableSize {
			d.prnt[k+1] = uint16(i)
		}
	}
}

// update increments the frequency of the given code by one, and updates the tree
func (d *huffman) update(c int) {
	if d.freq[root] == maxFreq {
		d.reconst()
	}
	c = int(d.prnt[c+tableSize])
	for {
		d.freq[c]++
		k := d.freq[c]

		// if the order is disturbed, exchange nodes
		if l := c + 1; k > d.freq[l] {
			for l++; k > d.freq[l]; l++ {
			}
			l--
			d.freq[c] = d.freq[l]
			d.freq[l] = k

			i := d.son[c]
			d.prnt[i] = uint16(l)
			if i < tableSize {
				d.prnt[i+1] = uint16(l)
			}

			j := d.son[l]
			d.son[l] = i

			d.prnt[j] = uint16(c)
			if j < tableSize {
				d.prnt[j+1] = uint16(c)
			}
			d.son[c] = j

			c = l
		}
		if c = int(d.prnt[c]); c == 0 { // repeat up to root
			return
		}
	}
}

func (d *decoder) decodeChar() int {
	// travel from root to leaf, choosing the smaller child node (son[]) if
	// the read bit is 0, the bigger (son[]+1) if 1
	c := int(d.son[root])
	for c < tableSize {
		c = int(d.son[c+d.getBit()])
	}
	c -= tableSize
	d.update(c)
	return c
}

func (d *decoder) decodePosition() int {
	// recover upper 6 bits from table
	i := d.getByte()
	c := int(dCode[i]) << 6

	// read lower 6 bits verbatim
	for j := dLen[i] - 2; j > 0; j-- {
		i = i<<1 + d.getBit()
	}
	return c | i&0x3f
}

// Decode decompresses in (4-byte little-endian decoded size followed by the
// LZHUF stream) into out and returns the number of bytes written. It never
// writes past len(out); a short count means in was truncated or out too small.
func Decode(in []byte, out []byte) int {
	if len(in) < 4 {
		return 0
	}
	textsize := int(in[0]) | int(in[1])<<8 | int(in[2])<<16 | int(in[3])<<24
	if textsize < 0 || textsize > len(out) {
		textsize = len(out)
	}
	out = out[:textsize]

	d := &decoder{in: in[4:]}
	d.startHuff()

	var textBuf [bufSize]byte
	for i := range bufSize - lookahead {
		textBuf[i] = ' '
	}
	r := bufSize - lookahead
	count := 0
	// the bit reader prefetches up to 2 bytes, anything beyond that is a truncated stream
	for count < len(out) && d.pos <= len(d.in)+2 {
		c := d.decodeChar()
		if c < 256 {
			out[count] = byte(c)
			textBuf[r] = byte(c)
			r = (r + 1) & (bufSize - 1)
			count++
			continue
		}
		i := r - d.decodePosition() - 1
		for k := range c - 255 + threshold {
			if count == len(out) {
				break
			}
			b := textBuf[(i+k)&(bufSize-1)]
			out[count] = b
			textBuf[r] = b
			r = (r + 1) & (bufSize - 1)
			count++
		}
	}
	return count
}

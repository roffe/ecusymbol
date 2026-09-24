package lzhuf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

// testdata/reconst.lzh was produced by the original C encoder from 50000
// bytes of mostly-random data, enough symbols to trigger one reconst().
func TestCodec(t *testing.T) {
	const size, sum = 50000, "e06d4a105e470ca375ce9739111453f4fb0f0c09e76a5e1ae4647e261a9e7aeb"
	in, err := os.ReadFile("testdata/reconst.lzh")
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, size)
	if n := Decode(in, out); n != size {
		t.Fatalf("decoded %d bytes, want %d", n, size)
	}
	if got := sha256.Sum256(out); hex.EncodeToString(got[:]) != sum {
		t.Fatalf("sha256 mismatch: %x", got)
	}

	// the encoder must reproduce the C encoder's output bit for bit
	if !bytes.Equal(Encode(out), in) {
		t.Fatal("Encode output differs from the original C encoder")
	}
	if got := Encode(nil); !bytes.Equal(got, []byte{0, 0, 0, 0}) {
		t.Fatalf("Encode(nil) = %x", got)
	}

	// hostile input must neither overrun out nor spin forever
	if n := Decode(in, out[:100]); n != 100 {
		t.Fatalf("short out: decoded %d bytes, want 100", n)
	}
	if n := Decode(in[:1000], out); n >= size {
		t.Fatalf("truncated in: decoded %d bytes", n)
	}
}

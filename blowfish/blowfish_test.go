package blowfish

import "testing"

// Encrypted data that isn't whole 8 byte blocks is an error, not a panic.
func TestDecryptPartialBlock(t *testing.T) {
	if _, err := decrypt(make([]byte, 13), []byte("key")); err == nil {
		t.Error("no error for 13 bytes")
	}
}

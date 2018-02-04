package crypto

import (
	"bytes"
	"testing"
)

var encKey = "b567ef1d391e8a10d94100faa34b7d28fdab13e3f51f94b8"

func TestEncrypt(t *testing.T) {
	var data [80000]byte
	for i := 0; i < 80000; i++ {
		data[i] = 100
	}

	r, _ := NewService(encKey, 0).Seal(data[:])
	if bytes.Equal(r, data[:]) {
		t.Error("Encrypt output should not be equal the input.")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	var data [80000]byte
	for i := 0; i < 80000; i++ {
		data[i] = 100
	}
	s := NewService(encKey, 0)
	enc, _ := s.Seal(data[:])
	dec, _ := s.Open(enc)
	if !bytes.Equal(dec, data[:]) {
		t.Error("Encrypt - Decrypt error")
	}
}

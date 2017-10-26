package crypto

import (
	"bytes"
	"testing"
)

var encKey = "b567ef1d391e8a10d94100faa34b7d28fdab13e3f51f94b8"

func TestEncrypt(t *testing.T) {
	var b bytes.Buffer
	var data [80000]byte
	for i := 0; i < 80000; i++ {
		data[i] = 100
	}

	Encrypt(&b, encKey, bytes.NewReader(data[:]))
	if bytes.Equal(b.Bytes(), data[:]) {
		t.Error("Encrypt output should not be equal the input.")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	var b bytes.Buffer
	var data [80000]byte
	for i := 0; i < 80000; i++ {
		data[i] = 100
	}

	Encrypt(&b, encKey, bytes.NewReader(data[:]))
	var b2 bytes.Buffer
	Decrypt(&b2, encKey, &b)
	if !bytes.Equal(b2.Bytes(), data[:]) {
		t.Error("Encrypt - Decrypt error")
	}
}

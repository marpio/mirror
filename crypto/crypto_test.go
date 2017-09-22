package crypto

import (
	"bytes"
	"testing"
)

func TestEncrypt(t *testing.T) {
	var b bytes.Buffer
	encKey := "b567ef1d391e8a10d94100faa34b7d28fdab13e3f51f94b8"
	data := []byte("test string")

	Encrypt(&b, encKey, bytes.NewReader(data))

	if string(b.Bytes()) == string(data) {
		t.Error("Encrypt output should not be equal the input.")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	var b bytes.Buffer
	encKey := "b567ef1d391e8a10d94100faa34b7d28fdab13e3f51f94b8"
	data := []byte("test string")

	Encrypt(&b, encKey, bytes.NewReader(data))
	var b2 bytes.Buffer
	Decrypt(&b2, encKey, &b)

	if string(b2.Bytes()) != string(data) {
		t.Error("Encrypt - Decrypt error")
	}
}

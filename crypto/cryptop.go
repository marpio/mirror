package crypto

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"

	"golang.org/x/crypto/nacl/secretbox"
)

func CalculateHash(r io.Reader) (string, error) {
	h := sha1.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	hash := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return hash, nil
}

func Encrypt(encryptionKey string, data []byte) ([]byte, error) {
	secretKeyBytes, err := hex.DecodeString(encryptionKey)
	if err != nil {
		return nil, err
	}

	var secretKey [32]byte
	copy(secretKey[:], secretKeyBytes)

	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, err
	}

	encrypted := secretbox.Seal(nonce[:], data, &nonce, &secretKey)
	return encrypted, nil
}

func Decrypt(encryptionKey string, encryptedData []byte) ([]byte, error) {
	secretKeyBytes, err := hex.DecodeString(encryptionKey)
	if err != nil {
		return nil, err
	}

	var secretKey [32]byte
	copy(secretKey[:], secretKeyBytes)
	var decryptNonce [24]byte
	copy(decryptNonce[:], encryptedData[:24])
	decrypted, ok := secretbox.Open(nil, encryptedData[24:], &decryptNonce, &secretKey)
	if !ok {
		return nil, errors.New("Could not decrypt data")
	}

	return decrypted, nil
}

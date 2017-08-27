package crypto

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"

	"golang.org/x/crypto/nacl/secretbox"
)

const dataChunkSize = 64 * 1024

func CalculateHash(r io.Reader) (string, error) {
	h := sha1.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	hash := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return hash, nil
}

func Encrypt(encryptionKey string, data []byte) ([]byte, error) {
	var encrypted []byte
	secretKeyBytes, err := hex.DecodeString(encryptionKey)
	if err != nil {
		return nil, err
	}

	var secretKey [32]byte
	copy(secretKey[:], secretKeyBytes)

	chunkSize := dataChunkSize
	length := len(data)
	for i := 0; i < len(data); i = i + chunkSize {
		end := i + chunkSize
		if end > length {
			end = length
		}
		nonce, err := genNonce()
		if err != nil {
			return nil, err
		}
		encryptedChunk := secretbox.Seal(nonce[:], data[i:end], &nonce, &secretKey)
		encrypted = append(encrypted, encryptedChunk...)

	}
	return encrypted, nil
}

func Test(encryptionKey string) {
	w := []byte{2, 2, 2, 2}
	x, _ := Encrypt(encryptionKey, w)
	y, _ := Decrypt(encryptionKey, x)
	log.Print((y[:]))
}
func Decrypt(encryptionKey string, encryptedData []byte) ([]byte, error) {
	var decrypted []byte
	secretKeyBytes, err := hex.DecodeString(encryptionKey)
	if err != nil {
		return nil, err
	}

	var secretKey [32]byte
	copy(secretKey[:], secretKeyBytes)

	chunkSize := 24 + dataChunkSize + 16
	length := len(encryptedData)
	for i := 0; i < len(encryptedData); i = i + chunkSize {
		end := i + chunkSize
		if end > length {
			end = length
		}
		encryptedDataChunk := encryptedData[i:end]
		var decryptNonce [24]byte
		copy(decryptNonce[:], encryptedDataChunk[:24])
		decryptedChunk, ok := secretbox.Open(nil, encryptedDataChunk[24:], &decryptNonce, &secretKey)
		if !ok {
			return nil, fmt.Errorf("Could not decrypt data chunk nr: %v", i)
		}
		decrypted = append(decrypted, decryptedChunk...)

	}
	return decrypted, nil
}

func genNonce() ([24]byte, error) {
	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return [24]byte{}, err
	}
	return nonce, nil
}

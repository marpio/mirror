package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"

	"golang.org/x/crypto/nacl/secretbox"
)

const keyLen = 32
const nonceLen = 24

type Service interface {
	Seal(plaintxt []byte) ([]byte, error)
	Open(encrypted []byte) ([]byte, error)
	NonceSize() int
	BlockSize() int
	Overhead() int
}

type option func(*srv)

func WithBlockSize(blockSize int) option {
	return func(s *srv) {
		s.blockSize = blockSize
	}
}

func NewService(encryptionKey string, options ...option) Service {
	cs := &srv{encryptionKey: encryptionKey, blockSize: 64 * 1024}
	for _, opt := range options {
		opt(cs)
	}
	return cs
}

type srv struct {
	encryptionKey string
	blockSize     int
}

func (s *srv) BlockSize() int {
	return s.blockSize
}

func (s *srv) Overhead() int {
	return secretbox.Overhead
}

func (s *srv) NonceSize() int {
	return nonceLen
}

func (s *srv) Seal(plaintxt []byte) ([]byte, error) {
	secretKeyBytes, err := hex.DecodeString(s.encryptionKey)
	if err != nil {
		return nil, err
	}
	var secretKey [keyLen]byte
	copy(secretKey[:], secretKeyBytes)
	nonce, err := genNonce()
	if err != nil {
		return nil, err
	}
	encrypted := secretbox.Seal(nonce[:], plaintxt[:], &nonce, &secretKey)
	return encrypted, nil
}

func (s *srv) Open(encrypted []byte) ([]byte, error) {
	secretKeyBytes, err := hex.DecodeString(s.encryptionKey)
	if err != nil {
		return nil, err
	}

	var secretKey [keyLen]byte
	copy(secretKey[:], secretKeyBytes)

	var decryptNonce [nonceLen]byte
	copy(decryptNonce[:], encrypted[:nonceLen])
	decrypted, ok := secretbox.Open(nil, encrypted[nonceLen:], &decryptNonce, &secretKey)
	if !ok {
		log.Printf("Encrypted len: %v", len(encrypted))
		return nil, fmt.Errorf("Could not decrypt data")
	}
	return decrypted, nil
}

func genNonce() ([nonceLen]byte, error) {
	var nonce [nonceLen]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return [nonceLen]byte{}, err
	}
	return nonce, nil
}

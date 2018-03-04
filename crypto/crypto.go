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
	cs := &srv{encryptionKey: encryptionKey, blockSize: 16 * 1024}
	secretKeyBytes, err := hex.DecodeString(encryptionKey)
	if err != nil {
		return nil
	}
	var secretKey [keyLen]byte
	copy(secretKey[:], secretKeyBytes)
	cs.secretKey = secretKey
	for _, opt := range options {
		opt(cs)
	}
	return cs
}

type srv struct {
	encryptionKey string
	blockSize     int
	secretKey     [keyLen]byte
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
	if len(plaintxt) == 0 {
		return plaintxt, nil
	}
	rounds := len(plaintxt) / s.blockSize
	rem := len(plaintxt) % s.blockSize
	if rounds != 0 && rem != 0 {
		return nil, fmt.Errorf("")
	}
	res := make([]byte, 0)
	for i := 0; i < rounds; i++ {
		start := i * s.blockSize
		end := (i + 1) * s.blockSize
		data, err := s.encrypt(plaintxt[start:end])
		if err != nil {
			return nil, err
		}
		res = append(res, data...)
	}
	if rem > 0 {
		data, err := s.encrypt(plaintxt[rounds*s.blockSize:])
		if err != nil {
			return nil, err
		}
		res = append(res, data...)
	}
	return res, nil
}

func (s *srv) encrypt(plaintxt []byte) ([]byte, error) {
	nonce, err := genNonce()
	if err != nil {
		return nil, err
	}
	encrypted := secretbox.Seal(nonce[:], plaintxt[:], &nonce, &s.secretKey)
	return encrypted, nil
}

func (s *srv) Open(encrypted []byte) ([]byte, error) {
	var decryptNonce [nonceLen]byte
	copy(decryptNonce[:], encrypted[:nonceLen])
	decrypted, ok := secretbox.Open(nil, encrypted[nonceLen:], &decryptNonce, &s.secretKey)
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

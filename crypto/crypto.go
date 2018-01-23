package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"

	"golang.org/x/crypto/nacl/secretbox"
)

const dataChunkSize = 64 * 1024

type Service interface {
	Encrypt(dst io.Writer, plainDataReader io.Reader) error
	Decrypt(dst io.Writer, encryptedDataReader io.Reader) error
}

func NewService(encryptionKey string) Service {
	return &srv{encryptionKey: encryptionKey}
}

type srv struct {
	encryptionKey string
}

func (s *srv) Encrypt(dst io.Writer, plainDataReader io.Reader) error {
	secretKeyBytes, err := hex.DecodeString(s.encryptionKey)
	if err != nil {
		return err
	}
	var secretKey [32]byte
	copy(secretKey[:], secretKeyBytes)
	chunkSize := dataChunkSize
	buf := make([]byte, 0, chunkSize)
	for {
		n, err := io.ReadAtLeast(plainDataReader, buf[0:cap(buf)], chunkSize)
		//n, err := plainDataReader.Read(buf[0:cap(buf)])
		buf = buf[:n]
		if n == 0 {
			if err == nil {
				continue
			}
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		if err != nil && err != io.ErrUnexpectedEOF {
			return err
		}
		nonce, err := genNonce()
		if err != nil {
			return err
		}
		encryptedChunk := secretbox.Seal(nonce[:], buf[:], &nonce, &secretKey)
		dst.Write(encryptedChunk)
	}
	return nil
}

func (s *srv) Decrypt(dst io.Writer, encryptedDataReader io.Reader) error {
	secretKeyBytes, err := hex.DecodeString(s.encryptionKey)
	if err != nil {
		return err
	}

	var secretKey [32]byte
	copy(secretKey[:], secretKeyBytes)

	chunkSize := 24 + dataChunkSize + 16
	buf := make([]byte, 0, chunkSize)
	for {
		n, err := encryptedDataReader.Read(buf[0:cap(buf)])
		buf = buf[:n]
		if n == 0 {
			if err == nil {
				continue
			}
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		if err != nil && err != io.EOF {
			return err
		}

		var decryptNonce [24]byte
		copy(decryptNonce[:], buf[:24])
		decryptedChunk, ok := secretbox.Open(nil, buf[24:], &decryptNonce, &secretKey)
		if !ok {
			return fmt.Errorf("Could not decrypt data")
		}
		dst.Write(decryptedChunk)
	}
	return nil
}

func genNonce() ([24]byte, error) {
	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return [24]byte{}, err
	}
	return nonce, nil
}

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

func Encrypt(dst io.Writer, encryptionKey string, dataReader io.Reader) error {
	secretKeyBytes, err := hex.DecodeString(encryptionKey)
	if err != nil {
		return err
	}
	var secretKey [32]byte
	copy(secretKey[:], secretKeyBytes)
	chunkSize := dataChunkSize
	buf := make([]byte, 0, chunkSize)
	for {
		n, err := dataReader.Read(buf[:cap(buf)])
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
		nonce, err := genNonce()
		if err != nil {
			return err
		}
		encryptedChunk := secretbox.Seal(nonce[:], buf[:], &nonce, &secretKey)
		dst.Write(encryptedChunk)
	}
	return nil
}

func Decrypt(dst io.Writer, encryptionKey string, encryptedDataReader io.Reader) error {
	secretKeyBytes, err := hex.DecodeString(encryptionKey)
	if err != nil {
		return err
	}

	var secretKey [32]byte
	copy(secretKey[:], secretKeyBytes)

	chunkSize := 24 + dataChunkSize + 16
	buf := make([]byte, 0, chunkSize)
	for {
		n, err := encryptedDataReader.Read(buf[:cap(buf)])
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

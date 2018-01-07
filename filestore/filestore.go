package filestore

import (
	"io"
	"log"

	"github.com/marpio/img-store/crypto"
)

type Service interface {
	ReaderService
	WriterService
}

type ReaderService interface {
	DownloadDecrypted(dst io.Writer, fileName string)
}

type WriterService interface {
	UploadEncrypted(fileName string, reader io.Reader) error
	Delete(fileName string) error
}

type Backend struct {
	ReadFn        func(string) io.ReadCloser
	WriteFn       func(string) io.WriteCloser
	DeleteFn      func(string) error
	EncryptionKey string
}

func (b *Backend) DownloadDecrypted(dst io.Writer, fileName string) {
	r := b.ReadFn(fileName)
	//r.ConcurrentDownloads = downloads
	defer r.Close()

	err := crypto.Decrypt(dst, b.EncryptionKey, r)
	if err != nil {
		log.Print(err)
		panic(err)
	}
}

func (b *Backend) UploadEncrypted(fileName string, reader io.Reader) error {
	w := b.WriteFn(fileName)
	crypto.Encrypt(w, b.EncryptionKey, reader)
	if err := w.Close(); err != nil {
		return err
	}
	return nil
}

func (b *Backend) Delete(fileName string) error {
	if err := b.DeleteFn(fileName); err != nil {
		return err
	}
	return nil
}

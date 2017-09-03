package filestore

import (
	"io"
	"log"

	"github.com/marpio/img-store/crypto"
)

type FileStore interface {
	FileStoreReader
	FileStoreWriter
}

type FileStoreReader interface {
	DownloadDecrypted(dst io.Writer, encryptionKey, fileName string)
}

type FileStoreWriter interface {
	UploadEncrypted(fileName string, reader io.Reader, encryptionKey string) error
	Delete(fileName string) error
}

type BackendStore struct {
	readerProvider func(string) io.ReadCloser
	writerProvider func(string) io.WriteCloser
}

func NewBackendStore(rp func(string) io.ReadCloser, wp func(string) io.WriteCloser) *BackendStore {
	b := BackendStore{readerProvider: rp, writerProvider: wp}
	return &b
}

func (b *BackendStore) DownloadDecrypted(dst io.Writer, encryptionKey, fileName string) {
	r := b.readerProvider(fileName)
	//r.ConcurrentDownloads = downloads
	defer r.Close()

	err := crypto.Decrypt(dst, encryptionKey, r)
	if err != nil {
		log.Print(err)
	}
}

func (b *BackendStore) UploadEncrypted(fileName string, reader io.Reader, encryptionKey string) error {
	w := b.writerProvider(fileName)
	crypto.Encrypt(w, encryptionKey, reader)
	if err := w.Close(); err != nil {
		return err
	}
	return nil
}

func (b *BackendStore) Delete(fileName string) error {
	return nil
}

package filestore

import (
	"io"
	"log"
)

type FileStore interface {
	FileStoreReader
	FileStoreWriter
}

type FileStoreReader interface {
	DownloadDecrypted(dst io.Writer, fileName string)
}

type FileStoreWriter interface {
	UploadEncrypted(fileName string, reader io.Reader) error
	Delete(fileName string) error
}

type BackendStore struct {
	readFn        func(string) io.ReadCloser
	writeFn       func(string) io.WriteCloser
	deleteFn      func(string) error
	encryptionKey string
}

func NewFileStore(rp func(string) io.ReadCloser, wp func(string) io.WriteCloser, del func(string) error, encryptKey string) *BackendStore {
	b := BackendStore{readFn: rp, writeFn: wp, deleteFn: del, encryptionKey: encryptKey}
	return &b
}

func (b *BackendStore) DownloadDecrypted(dst io.Writer, fileName string) {
	//r := b.readFn(fileName)
	////r.ConcurrentDownloads = downloads
	//defer r.Close()
	//
	//err := crypto.Decrypt(dst, b.encryptionKey, r)
	//if err != nil {
	//	log.Print(err)
	//}
}

func (b *BackendStore) UploadEncrypted(fileName string, reader io.Reader) error {
	log.Printf("Uploading file: %v", fileName)
	//w := b.writeFn(fileName)
	//crypto.Encrypt(w, b.encryptionKey, reader)
	//if err := w.Close(); err != nil {
	//	return err
	//}
	return nil
}

func (b *BackendStore) Delete(fileName string) error {
	//if err := b.deleteFn(fileName); err != nil {
	//	return err
	//}
	return nil
}

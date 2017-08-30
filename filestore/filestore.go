package filestore

import (
	"io"
)

type FileStore interface {
	FileStoreReader
	FileStoreWriter
}

type FileStoreReader interface {
	Download(dst io.Writer, encryptionKey, src string)
}

type FileStoreWriter interface {
	Upload(imgFileName string, reader io.Reader) error
	Delete(fileName string) error
}

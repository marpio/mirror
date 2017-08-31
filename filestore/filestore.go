package filestore

import (
	"io"
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

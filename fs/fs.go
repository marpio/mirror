package fs

import (
	"io"
	"time"
)

type File interface {
	io.ReadCloser
	io.Seeker
}

type FileInfo struct {
	Path    string
	ModTime time.Time
}

type FileReaderFn func(path string) (File, error)

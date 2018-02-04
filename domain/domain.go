package domain

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"strconv"
	"time"
)

type Storage interface {
	StorageReader
	StorageWriter
	Exists(fileName string) bool
}

type StorageReader interface {
	NewReader(path string) (io.ReadCloser, error)
}

type StorageReadSeeker interface {
	NewReadSeeker(path string) (ReadCloseSeeker, error)
}

type StorageWriter interface {
	NewWriter(string) io.WriteCloser
	Delete(fileName string) error
}

type LocalStorage interface {
	StorageReader
	StorageReadSeeker
	SearchFiles(rootPath string, filter func(*FileInfo) bool, fileExt ...string) []*FileInfo
}

type ReadCloseSeeker interface {
	io.ReadCloser
	io.Seeker
}

type Extractor interface {
	Extract(ctx context.Context, files []*FileInfo) <-chan *Photo
}

type Item interface {
	ID() string
	ThumbID() string
	Dir() string
	ModTimeHash() string
}

type FileInfo struct {
	FilePath    string
	FileModTime time.Time
}

type Metadata struct {
	CreatedAt time.Time
	Thumbnail []byte
}

type Photo struct {
	*FileInfo
	*Metadata
}

func (p *FileInfo) ID() string {
	return genHash(p.FilePath)
}

func (p *FileInfo) ModTimeHash() string {
	return genHash(strconv.FormatInt(p.FileModTime.UnixNano(), 10))
}

func (p *Photo) ThumbID() string {
	return "thumb_" + p.ID()
}

func (p *Photo) Dir() string {
	return fmt.Sprintf("%d-%02d", p.CreatedAt.Year(), p.CreatedAt.Month())
}

func genHash(s string) string {
	h := sha256.Sum256([]byte(s))
	hex := fmt.Sprintf("%x", h)
	return hex
}

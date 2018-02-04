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
	Exists(ctx context.Context, path string) bool
}

type StorageReader interface {
	NewReader(ctx context.Context, path string) (io.ReadCloser, error)
}

type StorageReadSeeker interface {
	NewReadSeeker(ctx context.Context, path string) (ReadCloseSeeker, error)
}

type StorageWriter interface {
	NewWriter(ctx context.Context, path string) io.WriteCloser
	Delete(ctx context.Context, path string) error
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

type MetadataRepo interface {
	MetadataRepoReader
	MetadataRepoWriter
}

type MetadataRepoWriter interface {
	Add(item Item) error
	Delete(id string) error
	Persist(ctx context.Context) error
}

type MetadataRepoReader interface {
	Exists(id string) (bool, error)
	GetModTime(id string) (string, error)
	GetByDir(name string) ([]Item, error)
	GetByDirAndId(dir, id string) (Item, error)
	GetDirs() ([]string, error)
	Reload(ctx context.Context) error
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

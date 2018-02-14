package domain

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/apex/log"
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
	Extract(ctx context.Context, logctx log.Interface, files []*FileInfo) <-chan Photo
}

type Item interface {
	ID() string
	ThumbID() string
	Dir() string
	ModTimeHash() string
}

type FileInfo struct {
	FilePath    string
	FileExt     string
	FileModTime time.Time
}

type Metadata struct {
	CreatedAt time.Time
	Thumbnail []byte
}

type Photo interface {
	ID() string
	ThumbID() string
	FilePath() string
	FileModTime() time.Time
	CreatedAt() time.Time
	SetCreatedAt(t time.Time)
	Thumbnail() []byte
	NewJpgReader() (io.ReadCloser, error)
	Dir() string
	ModTimeHash() string
}

type photo struct {
	*FileInfo
	*Metadata
	jpegReaderProvider func() (io.ReadCloser, error)
}

func (ph *photo) FilePath() string {
	return ph.FileInfo.FilePath
}

func (ph *photo) FileModTime() time.Time {
	return ph.FileInfo.FileModTime
}

func (ph *photo) CreatedAt() time.Time {
	return ph.Metadata.CreatedAt
}

func (ph *photo) SetCreatedAt(t time.Time) {
	ph.Metadata.CreatedAt = t
}

func (ph *photo) Thumbnail() []byte {
	return ph.Metadata.Thumbnail
}

func (ph *photo) NewJpgReader() (io.ReadCloser, error) {
	return ph.jpegReaderProvider()
}

func NewPhoto(fi *FileInfo, meta *Metadata, jpegReaderProvider func() (io.ReadCloser, error)) Photo {
	return &photo{
		FileInfo:           fi,
		Metadata:           meta,
		jpegReaderProvider: jpegReaderProvider,
	}
}

func (p *FileInfo) ID() string {
	return genHash(p.FilePath)
}

func (p *FileInfo) ModTimeHash() string {
	return genHash(strconv.FormatInt(p.FileModTime.UnixNano(), 10))
}

func (p *photo) ThumbID() string {
	return "thumb_" + p.ID()
}

func (p *photo) Dir() string {
	return fmt.Sprintf("%d-%02d", p.CreatedAt().Year(), p.CreatedAt().Month())
}

func genHash(s string) string {
	h := sha256.Sum256([]byte(s))
	hex := fmt.Sprintf("%x", h)
	return hex
}

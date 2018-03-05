package mirror

import (
	"context"
	"io"
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

type ReadOnlyStorage interface {
	StorageReader
	StorageReadSeeker
	SearchFiles(rootPath string, fileExt ...string) []FileInfo
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
	Add(item RepoEntry) error
	Delete(id string) error
	Persist(ctx context.Context) error
}

type MetadataRepoReader interface {
	GetAll() []RepoEntry
	Exists(id string) (bool, error)
	GetByDir(name string) ([]RepoEntry, error)
	GetByDirAndId(dir, id string) (RepoEntry, error)
	GetDirs() ([]string, error)
	Reload(ctx context.Context) error
}

type Extractor interface {
	Extract(ctx context.Context, logctx log.Interface, photos []FileInfo) []Photo
}

type RepoEntry interface {
	ID() string
	ThumbID() string
	Dir() string
}

type FileInfo interface {
	ID() string
	FilePath() string
	FileExt() string
}

type Photo interface {
	RepoEntry
	FilePath() string
	CreatedAt() time.Time
	SetCreatedAt(t time.Time)
	Thumbnail() []byte
	NewJpgReader() (io.ReadCloser, error)
}

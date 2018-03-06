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
	NewReader(ctx context.Context, path string) (ReadCloseSeeker, error)
}

type StorageWriter interface {
	NewWriter(ctx context.Context, path string) io.WriteCloser
	Delete(ctx context.Context, path string) error
}

type ReadOnlyStorage interface {
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
	Add(item RemotePhoto) error
	Delete(id string) error
	Persist(ctx context.Context) error
}

type MetadataRepoReader interface {
	GetAll() []RemotePhoto
	Exists(id string) (bool, error)
	GetByDir(name string) ([]RemotePhoto, error)
	GetByDirAndId(dir, id string) (RemotePhoto, error)
	GetDirs() ([]string, error)
	Reload(ctx context.Context) error
}

type Extractor interface {
	Extract(ctx context.Context, logctx log.Interface, photos []FileInfo) []LocalPhoto
}

type FileInfo interface {
	ID() string
	FilePath() string
}

type RemotePhoto interface {
	ID() string
	ThumbID() string
	Dir() string
}

type LocalPhoto interface {
	RemotePhoto
	FilePath() string
	CreatedAt() time.Time
	SetCreatedAt(t time.Time)
	Thumbnail() []byte
	NewJpgReader() (io.ReadCloser, error)
}

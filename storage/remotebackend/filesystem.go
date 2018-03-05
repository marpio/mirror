package remotebackend

import (
	"context"
	"io"

	"github.com/spf13/afero"
)

type FileSystem struct {
	fs afero.Fs
}

func NewFileSystem(fs afero.Fs) *FileSystem {
	return &FileSystem{fs: fs}
}

func (b *FileSystem) NewReader(ctx context.Context, fileName string) (io.ReadCloser, error) {
	f, err := b.fs.Open(fileName)
	return f, err
}

func (b *FileSystem) NewWriter(ctx context.Context, fileName string) io.WriteCloser {
	f, _ := b.fs.Create(fileName)
	return f
}

func (b *FileSystem) Delete(ctx context.Context, fileName string) error {
	if err := b.fs.Remove(fileName); err != nil {
		return err
	}
	return nil
}

func (b *FileSystem) Exists(ctx context.Context, fileName string) bool {
	e, _ := afero.Exists(b.fs, fileName)
	return e
}

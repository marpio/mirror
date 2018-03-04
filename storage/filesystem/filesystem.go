package filesystem

import (
	"context"
	"io"

	"github.com/marpio/mirror"
	"github.com/spf13/afero"
)

type fsBackend struct {
	fs afero.Fs
}

func New(fs afero.Fs) mirror.Storage {
	return &fsBackend{fs: fs}
}

func (b *fsBackend) NewReader(ctx context.Context, fileName string) (io.ReadCloser, error) {
	f, err := b.fs.Open(fileName)
	return f, err
}

func (b *fsBackend) NewWriter(ctx context.Context, fileName string) io.WriteCloser {
	f, _ := b.fs.Create(fileName)
	return f
}

func (b *fsBackend) Delete(ctx context.Context, fileName string) error {
	if err := b.fs.Remove(fileName); err != nil {
		return err
	}
	return nil
}

func (b *fsBackend) Exists(ctx context.Context, fileName string) bool {
	e, _ := afero.Exists(b.fs, fileName)
	return e
}

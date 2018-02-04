package filesystem

import (
	"io"

	"github.com/marpio/img-store/domain"
	"github.com/spf13/afero"
)

type fsBackend struct {
	fs afero.Fs
}

func New(fs afero.Fs) domain.Storage {
	return &fsBackend{fs: fs}
}

func (b *fsBackend) NewReader(fileName string) (io.ReadCloser, error) {
	f, err := b.fs.Open(fileName)
	return f, err
}

func (b *fsBackend) NewWriter(fileName string) io.WriteCloser {
	f, _ := b.fs.Create(fileName)
	return f
}

func (b *fsBackend) Delete(fileName string) error {
	if err := b.fs.Remove(fileName); err != nil {
		return err
	}
	return nil
}

func (b *fsBackend) Exists(fileName string) bool {
	e, _ := afero.Exists(b.fs, fileName)
	return e
}

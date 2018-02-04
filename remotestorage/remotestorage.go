package remotestorage

import (
	"context"
	"io"

	"github.com/marpio/img-store/crypto"
	"github.com/marpio/img-store/domain"
)

func New(b domain.Storage, c crypto.Service) domain.Storage {
	return &rs{backend: b, crpt: c}
}

type rs struct {
	backend domain.Storage
	crpt    crypto.Service
}

type reader struct {
	rd      io.ReadCloser
	buf     []byte
	r       int
	w       int
	err     error
	bufSize int
	crpt    crypto.Service
}

func (b *rs) NewReader(ctx context.Context, path string) (io.ReadCloser, error) {
	bufSize := b.crpt.NonceSize() + b.crpt.BlockSize() + b.crpt.Overhead()
	rd, err := b.backend.NewReader(ctx, path)
	if err != nil {
		return nil, err
	}
	return &reader{rd: rd, buf: make([]byte, bufSize), bufSize: bufSize, crpt: b.crpt}, nil
}
func (b *reader) readErr() error {
	err := b.err
	b.err = nil
	return err
}

func (b *reader) Read(p []byte) (n int, err error) {
	n = len(p)
	if n == 0 {
		return 0, b.readErr()
	}
	if b.r == b.w {
		if b.err != nil {
			return 0, b.readErr()
		}
		b.buf = make([]byte, b.bufSize)
		b.r = 0
		b.w = 0

		n, b.err = b.rd.Read(b.buf)
		if n < 0 {
			panic("errNegativeRead")
		}
		if n == 0 {
			return 0, b.readErr()
		}
		d, err := b.crpt.Open(b.buf[0:n])
		if err != nil {
			return 0, err
		}
		n = len(d)
		b.buf = d
		b.w += n
	}
	// copy as much as we can
	n = copy(p, b.buf[b.r:b.w])
	b.r += n
	return n, nil
}

func (b *reader) Close() error {
	return b.rd.Close()
}

type writer struct {
	err  error
	buf  []byte
	n    int
	wr   io.WriteCloser
	crpt crypto.Service
}

func (b *rs) NewWriter(ctx context.Context, path string) io.WriteCloser {
	return &writer{wr: b.backend.NewWriter(ctx, path), buf: make([]byte, b.crpt.BlockSize()), crpt: b.crpt}
}

// Available returns how many bytes are unused in the buffer.
func (b *writer) available() int { return len(b.buf) - b.n }

func (b *writer) Write(p []byte) (nn int, err error) {
	for len(p) > b.available() && b.err == nil {
		var n int
		n = copy(b.buf[b.n:], p)
		b.n += n
		b.flush()
		nn += n
		p = p[n:]
	}
	if b.err != nil {
		return nn, b.err
	}
	n := copy(b.buf[b.n:], p)
	b.n += n
	nn += n

	return nn, nil
}

// Flush writes any buffered data to the underlying io.Writer.
func (b *writer) flush() error {
	if b.err != nil {
		return b.err
	}
	if b.n == 0 {
		return nil
	}

	encrypted, err := b.crpt.Seal(b.buf[0:b.n])
	if err != nil {
		return err
	}
	n, err := b.wr.Write(encrypted)
	n = n - b.crpt.NonceSize() - b.crpt.Overhead()
	if n < b.n && err == nil {
		err = io.ErrShortWrite
	}
	if err != nil {
		if n > 0 && n < b.n {
			copy(b.buf[0:b.n-n], b.buf[n:b.n])
		}
		b.n -= n
		b.err = err
		return err
	}
	b.n = 0
	return nil
}

func (b *writer) Close() error {
	defer b.wr.Close()
	return b.flush()
}

func (b *rs) Exists(ctx context.Context, fileName string) bool {
	return b.backend.Exists(ctx, fileName)
}

func (b *rs) Delete(ctx context.Context, fileName string) error {
	if err := b.backend.Delete(ctx, fileName); err != nil {
		return err
	}
	return nil
}

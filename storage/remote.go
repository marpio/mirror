package storage

import (
	"context"
	"io"

	"github.com/marpio/mirror"
	"github.com/marpio/mirror/crypto"
)

func NewRemote(b mirror.Storage, c crypto.Service) mirror.Storage {
	return &rs{backend: b, crpt: c}
}

type rs struct {
	backend mirror.Storage
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
	return &reader{
		rd:      rd,
		buf:     make([]byte, bufSize),
		bufSize: bufSize,
		crpt:    b.crpt,
	}, nil
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
	wr   io.WriteCloser
	crpt crypto.Service
}

func (b *rs) NewWriter(ctx context.Context, path string) io.WriteCloser {
	return &writer{wr: b.backend.NewWriter(ctx, path), buf: make([]byte, 0), crpt: b.crpt}
}

func (b *writer) Write(p []byte) (int, error) {
	l := len(b.buf) + len(p)
	d := make([]byte, l, l)
	copy(d[:len(b.buf)], b.buf[:])
	copy(d[len(b.buf):], p[:])
	end := (len(d) / b.crpt.BlockSize()) * b.crpt.BlockSize()
	encrypted, err := b.crpt.Seal(d[:end])
	if err != nil {
		return 0, err
	}
	_, err = b.wr.Write(encrypted)
	if err != nil {
		return 0, err
	}
	b.buf = d[end:]
	return len(p), nil
}

// Flush writes any buffered data to the underlying io.Writer.
func (b *writer) flush() error {
	encrypted, err := b.crpt.Seal(b.buf)
	if err != nil {
		return err
	}
	_, err = b.wr.Write(encrypted)
	if err != nil {
		return err
	}
	// TODO: handle the case when less then len(encrypted) has been written
	return nil
}

func (b *writer) Close() error {
	err := b.flush()
	if err != nil {
		return err
	}
	err = b.wr.Close()
	if err != nil {
		return err
	}
	return nil
}

func (b *rs) Exists(ctx context.Context, fileName string) bool {
	return b.backend.Exists(ctx, fileName)
}

func (b *rs) Delete(ctx context.Context, fileName string) error {
	return b.backend.Delete(ctx, fileName)
}

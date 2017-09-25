package filestore

import (
	"bytes"
	"io"
	"testing"
)

const encKey = "b567ef1d391e8a10d94100faa34b7d28fdab13e3f51f94b8"

type readCloser struct {
	io.Reader
}

func (readCloser) Close() error { return nil }

type writeCloser struct {
	io.Writer
}

func (writeCloser) Close() error { return nil }

func TestUpload(t *testing.T) {
	var b bytes.Buffer
	s := NewFileStore(func(p string) io.ReadCloser { return readCloser{bytes.NewReader(b.Bytes())} }, func(p string) io.WriteCloser { return writeCloser{&b} }, nil, encKey)

	pic := []byte("test string")
	s.UploadEncrypted("c.jpg", bytes.NewReader(pic))
	if len(b.Bytes()) == 0 {
		t.Error(b.Bytes())
	}
	var dst bytes.Buffer
	s.DownloadDecrypted(&dst, "c.jpg")
	if string(dst.Bytes()) != string(pic) {
		t.Error("err")
	}
}

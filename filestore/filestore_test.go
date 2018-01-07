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

func TestUploadDownload(t *testing.T) {
	var b bytes.Buffer
	s := Backend{ReadFn: func(p string) io.ReadCloser { return readCloser{bytes.NewReader(b.Bytes())} }, WriteFn: func(p string) io.WriteCloser { return writeCloser{&b} }, DeleteFn: nil, EncryptionKey: encKey}
	pic := []byte("picture bytes")

	s.UploadEncrypted("pic.jpg", bytes.NewReader(pic))
	expectedLen := 24 + 16 + len(pic)
	actualLen := len(b.Bytes())
	if actualLen != expectedLen {
		t.Errorf("Expected len of the uploaded file: %v, actual: %v. File not written or encryption broken.", expectedLen, actualLen)
	}

	var downloadDst bytes.Buffer
	s.DownloadDecrypted(&downloadDst, "pic.jpg")
	if !bytes.Equal(pic, downloadDst.Bytes()) {
		t.Error("Downloaded file does not match the uploaded one.")
	}
}

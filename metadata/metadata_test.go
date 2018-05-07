package metadata

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/apex/log"
	"github.com/marpio/mirror"
	"github.com/marpio/mirror/storage"

	"github.com/spf13/afero"
)

var ctx context.Context = context.Background()

func TestThumbnail(t *testing.T) {
	p := "../test/sample.jpg"
	r, _ := os.Open(p)
	defer r.Close()
	thumb, _ := extractThumb(r)
	buff := make([]byte, 512)
	thumbReader := bytes.NewReader(thumb)
	thumbReader.Read(buff)
	mimeType := http.DetectContentType(buff)
	if mimeType != "image/jpeg" {
		t.Error("Thumbnail is not a jpeg file.")
	}
}

func TestThumbnail_NEF(t *testing.T) {
	p := "../test/sample3.NEF"

	thumb, err := extractThumbNEF(p)
	if err != nil {
		t.Error(err)
	}
	mimeType := http.DetectContentType(thumb)
	if mimeType != "image/jpeg" {
		t.Errorf("Thumbnail is not a jpeg file. It is: %v", mimeType)
	}
}

func TestJpeg_NEF(t *testing.T) {
	p := "../test/sample3.NEF"

	r, err := extractJpgNEF(p)
	if err != nil {
		t.Error(err)
	}
	img, _ := ioutil.ReadAll(r)
	mimeType := http.DetectContentType(img)
	if mimeType != "image/jpeg" {
		t.Errorf("Thumbnail is not a jpeg file. It is: %v", mimeType)
	}
}

func TestThumbnail_Photo_without_metadata(t *testing.T) {
	p := "../test/sample2.jpg"
	r, _ := os.Open(p)
	defer r.Close()
	thumb, _ := extractThumb(r)
	buff := make([]byte, 512)
	thumbReader := bytes.NewReader(thumb)
	thumbReader.Read(buff)
	mimeType := http.DetectContentType(buff)
	if mimeType != "image/jpeg" {
		t.Error("Thumbnail is not a jpeg file.")
	}
}

func TestCreatedAt(t *testing.T) {
	p := "../test/sample.jpg"
	r, _ := os.Open(p)
	defer r.Close()
	c, _ := extractCreatedAt(r)

	if !(c.Year() == 2017 && c.Month() == 8 && c.Day() == 25 && c.Hour() == 17 && c.Minute() == 3 && c.Second() == 30) {
		t.Error("Extracting CreatedAt failed.")
	}
}

func TestCreatedAt_NEF(t *testing.T) {
	p := "../test/sample3.NEF"
	r, _ := os.Open(p)
	defer r.Close()
	c, _ := extractCreatedAt(r)

	if !(c.Year() == 2018 && c.Month() == 1 && c.Day() == 1 && c.Hour() == 14 && c.Minute() == 56 && c.Second() == 48) {
		t.Error("Extracting CreatedAt failed.")
	}
}

func TestCreatedAt_Photo_without_metadata(t *testing.T) {
	afs := afero.NewOsFs()
	rs := NewStorageReadSeeker(afs)
	ex := NewExtractor(rs)
	path1 := "../test/sample.jpg"
	path2 := "../test/sample2.jpg"
	fi1 := storage.NewFileInfo(path1,
		func(string) (io.ReadCloser, error) { return os.Open(path1) },
		func(io.Reader) (string, error) { return "abc111", nil })
	fi2 := storage.NewFileInfo(path2,
		func(string) (io.ReadCloser, error) { return os.Open(path2) },
		func(io.Reader) (string, error) { return "abc222", nil })
	files := []mirror.FileInfo{fi1, fi2}
	ch := ex.Extract(context.Background(), log.Log, files)
	for _, p := range ch {
		c := p.CreatedAt()
		if !(c.Year() == 2017 && c.Month() == 8 && c.Day() == 25 && c.Hour() == 17 && c.Minute() == 3 && c.Second() == 30) {
			t.Error("Extracting CreatedAt failed.")
		}
	}
}

type storageReadSeekerMock struct {
	fs afero.Fs
}

func NewStorageReadSeeker(fs afero.Fs) *storageReadSeekerMock {
	return &storageReadSeekerMock{fs: fs}
}
func (m *storageReadSeekerMock) NewReader(ctx context.Context, path string) (io.ReadCloser, error) {
	return m.fs.Open(path)
}

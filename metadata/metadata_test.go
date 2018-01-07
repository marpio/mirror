package metadata

import (
	"bytes"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/spf13/afero"
)

func TestThumbnail(t *testing.T) {
	p := "../test/sample.jpg"
	r, _ := os.Open(p)
	defer r.Close()
	thumb, _ := thumbnailExtractor(r)
	buff := make([]byte, 512)
	thumbReader := bytes.NewReader(thumb)
	thumbReader.Read(buff)
	mimeType := http.DetectContentType(buff)
	if mimeType != "image/jpeg" {
		t.Error("Thumbnail is not a jpeg file.")
	}
}

func TestThumbnail_Photo_without_metadata(t *testing.T) {
	p := "../test/sample2.jpg"
	r, _ := os.Open(p)
	defer r.Close()
	thumb, _ := thumbnailExtractor(r)
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
	c, _ := createdAtExtractor(afero.NewOsFs())("../test/", p, r, time.Time{})

	if !(c.Year() == 2017 && c.Month() == 8 && c.Day() == 25 && c.Hour() == 17 && c.Minute() == 3 && c.Second() == 30) {
		t.Error("Extracting CreatedAt failed.")
	}
}

func TestCreatedAt_Photo_without_metadata(t *testing.T) {
	p := "../test/sample2.jpg"
	r, _ := os.Open(p)
	defer r.Close()
	c, _ := createdAtExtractor(afero.NewOsFs())("../test/", p, r, time.Time{})

	if !(c.Year() == 2017 && c.Month() == 8 && c.Day() == 25 && c.Hour() == 17 && c.Minute() == 3 && c.Second() == 30) {
		t.Error("Extracting CreatedAt failed.")
	}
}

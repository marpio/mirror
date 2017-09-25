package metadata

import (
	"bufio"
	"bytes"
	"image/jpeg"
	"io"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/marpio/img-store/file"
	"github.com/nfnt/resize"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/spf13/afero"
)

func CreatedAtExtractor(fs afero.Fs) func(dir string, path string, r file.File, dirCreatedAt time.Time) (time.Time, error) {
	return func(dir string, path string, r file.File, dirCreatedAt time.Time) (time.Time, error) {
		return extractCreatedAt(fs, dir, path, r, dirCreatedAt)
	}
}

func ExtractThumbnail(r io.ReadSeeker) ([]byte, error) {
	x, err := exif.Decode(r)
	if err != nil {
		return nil, err
	}

	thumbnail, err := x.JpegThumbnail()
	if err != nil {
		thumbnail, err = resizeImg(r)
		if err != nil {
			return nil, err
		}
	}
	return thumbnail, nil
}

func extractCreatedAt(fs afero.Fs, dir string, path string, r file.File, dirCreatedAt time.Time) (time.Time, error) {
	x, err := exif.Decode(r)
	if err != nil {
		return time.Time{}, err
	}
	imgCreatedAt, err := x.DateTime()
	if err != nil {
		if (dirCreatedAt != time.Time{}) {
			return dirCreatedAt, nil
		}

		imgCreatedAt, found := findNeighborImgCreatedAt(fs, dir, path)
		if !found {
			return time.Time{}, nil
		}
		return imgCreatedAt, nil
	}
	return imgCreatedAt, nil
}

func resizeImg(r io.ReadSeeker) ([]byte, error) {
	r.Seek(0, 0)
	img, err := jpeg.Decode(r)
	if err != nil {
		return nil, err
	}
	m := resize.Thumbnail(200, 200, img, resize.NearestNeighbor)
	var b bytes.Buffer
	writer := bufio.NewWriter(&b)
	if err := jpeg.Encode(writer, m, &jpeg.Options{Quality: 50}); err != nil {
		return nil, err
	}
	if err := writer.Flush(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func findNeighborImgCreatedAt(fs afero.Fs, dir string, path string) (time.Time, bool) {
	var imgCreatedAt = time.Time{}
	matches, _ := afero.Glob(fs, filepath.Join(dir, "*.jpg"))
	for _, imgfile := range matches {
		if imgfile == path {
			continue
		}
		imgCreatedAt = func(f string) time.Time {
			reader, err := fs.Open(f)
			if err != nil {
				return time.Time{}
			}
			defer reader.Close()
			other, err := exif.Decode(reader)
			if err != nil {
				return time.Time{}
			}
			imgCreatedAt, err = other.DateTime()
			if err != nil {
				return time.Time{}
			}
			return imgCreatedAt.Add(time.Millisecond * time.Duration(1))
		}(imgfile)
		foundCreatedAt := imgCreatedAt != time.Time{}
		if foundCreatedAt {
			return imgCreatedAt, true
		}
	}
	return time.Time{}.Add(time.Millisecond * time.Duration(rand.Intn(1000000))), false
}

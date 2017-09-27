package metadata

import (
	"bufio"
	"bytes"
	"image/jpeg"
	"io"
	"fmt"
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

		imgCreatedAt, err := findNeighborImgCreatedAt(fs, dir, path)
		if err != nil {
			return time.Time{}, err
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

func findNeighborImgCreatedAt(fs afero.Fs, dir string, path string) (time.Time, error) {
	var imgCreatedAt = time.Time{}
	matches, _ := afero.Glob(fs, filepath.Join(dir, "*.jpg"))
	for _, imgfile := range matches {
		if imgfile == path {
			continue
		}
		imgCreatedAt, err := func(f string) (time.Time, error) {
			reader, err := fs.Open(f)
			if err != nil {
				return time.Time{}, err
			}
			defer reader.Close()
			other, err := exif.Decode(reader)
			if err != nil {
				return time.Time{}, err
			}
			imgCreatedAt, err = other.DateTime()
			if err != nil {
				return time.Time{}, err
			}
			return imgCreatedAt.Add(time.Millisecond * time.Duration(1)), nil
		}(imgfile)
		if err == nil {
			return imgCreatedAt, nil
		}
	}
	return time.Time{}, fmt.Errorf("Could not find CreatedAt for %v", path)
}

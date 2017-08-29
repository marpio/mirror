package metadata

import (
	"bufio"
	"bytes"
	"fmt"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/nfnt/resize"
	"github.com/rwcarlsen/goexif/exif"
)

type Image struct {
	ImgID         string    `db:"img_id"`
	CreatedAt     time.Time `db:"created_at"`
	ImgHash       string    `db:"img_hash"`
	ImgName       string    `db:"img_name"`
	ThumbnailName string    `db:"thumbnail_name"`
}

func ExtractCreatedAt(imgPath string, r *os.File) (time.Time, error) {
	x, err := exif.Decode(r)
	if err != nil {
		return time.Time{}, err
	}
	defer r.Close()
	imgCreatedAt, err := x.DateTime()
	if err != nil {
		imgCreatedAt, err = findNeighborImgCreatedAt(imgPath)
		if err != nil {
			return time.Time{}, err
		}
	}
	return imgCreatedAt, nil
}

func ExtractThumbnail(imgPath string, r io.ReadSeeker) ([]byte, error) {
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

func findNeighborImgCreatedAt(imgPath string) (time.Time, error) {
	var imgCreatedAt time.Time
	containingdDir := filepath.Dir(imgPath)
	matches, _ := filepath.Glob(filepath.Join(containingdDir, "*.jpg"))
	for _, imgfile := range matches {
		imgCreatedAt = func(f string) time.Time {
			if f == imgPath {
				return time.Time{}
			}
			reader, err := os.Open(f)
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
			return imgCreatedAt, nil
		}
	}
	return time.Time{}, fmt.Errorf("Coldn't extract CreatedAt for file %v", imgPath)
}

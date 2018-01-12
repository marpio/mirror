package metadata

import (
	"bufio"
	"bytes"
	"fmt"
	"image/jpeg"
	"io"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/marpio/img-store/entity"
	"github.com/marpio/img-store/fsutils"
	"github.com/nfnt/resize"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/spf13/afero"
)

type MetadataExtractor interface {
	Extract(pathsGroupedByDir map[string][]*fsutils.FileInfo, fileReader fsutils.FileReaderFn) <-chan *entity.PhotoWithThumb
}

type metadataExtractor struct {
	extractCreatedAt func(imgPath string, path string, r fsutils.File, dirCreatedAt time.Time) (time.Time, error)
	extractThumbnail func(r io.ReadSeeker) ([]byte, error)
}

func NewExtractor(fs afero.Fs) MetadataExtractor {
	return &metadataExtractor{extractCreatedAt: createdAtExtractor(fs), extractThumbnail: thumbnailExtractor}
}

func (s metadataExtractor) Extract(pathsGroupedByDir map[string][]*fsutils.FileInfo, fileReader fsutils.FileReaderFn) <-chan *entity.PhotoWithThumb {
	metadataStream := make(chan *entity.PhotoWithThumb, 100)
	go func() {
		defer close(metadataStream)
		var wg sync.WaitGroup
		wg.Add(len(pathsGroupedByDir))
		for dir, paths := range pathsGroupedByDir {
			go func(directory string, ps []*fsutils.FileInfo) {
				defer wg.Done()
				s.extractMetadataDir(directory, ps, metadataStream, fileReader)
			}(dir, paths)
		}
		wg.Wait()
	}()
	return metadataStream
}

func (s metadataExtractor) extractMetadataDir(dir string, photos []*fsutils.FileInfo, metadataStream chan<- *entity.PhotoWithThumb, fileReader fsutils.FileReaderFn) {
	dirCreatedAt := time.Time{}
	for _, ph := range photos {
		err := func(p *fsutils.FileInfo) error {
			f, err := fileReader(p.Path)
			if err != nil {
				return fmt.Errorf("error opening file %v - err msg: %v", p.Path, err)
			}
			defer f.Close()

			createdAt, err := s.extractCreatedAt(dir, p.Path, f, dirCreatedAt)
			if err != nil {
				return fmt.Errorf("can't extract created_at for path: %v; err: %v", p.Path, err)
			}
			createdAtMonth := time.Date(createdAt.Year(), createdAt.Month(), 1, 0, 0, 0, 0, time.UTC)
			f.Seek(0, 0)
			thumb, err := s.extractThumbnail(f)
			if err != nil {
				return fmt.Errorf("can't extract thumbnail for path: %v; err: %v", p.Path, err)
			}
			thumbnailName := fsutils.GenerateUniqueFileName("thumb", p.Path, createdAt)
			imgName := fsutils.GenerateUniqueFileName("orig", p.Path, createdAt)
			res := &entity.PhotoWithThumb{Photo: &entity.Photo{FileInfo: p, Metadata: &entity.Metadata{Name: imgName, ThumbnailName: thumbnailName, CreatedAt: createdAt, CreatedAtMonth: createdAtMonth}}, Thumbnail: thumb}
			dirCreatedAt = createdAt
			metadataStream <- res
			return nil
		}(ph)
		if err != nil {
			log.Printf("error extracting metadata: %v", err)
		}
	}
}

func createdAtExtractor(fs afero.Fs) func(dir string, path string, r fsutils.File, dirCreatedAt time.Time) (time.Time, error) {
	return func(dir string, path string, r fsutils.File, dirCreatedAt time.Time) (time.Time, error) {
		return extractCreatedAt(fs, dir, path, r, dirCreatedAt)
	}
}

func thumbnailExtractor(r io.ReadSeeker) ([]byte, error) {
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

func extractCreatedAt(fs afero.Fs, dir string, path string, r fsutils.File, dirCreatedAt time.Time) (time.Time, error) {
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

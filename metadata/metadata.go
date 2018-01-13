package metadata

import (
	"bufio"
	"bytes"
	"image/jpeg"
	"io"
	"log"
	"sync"
	"time"

	"github.com/marpio/img-store/entity"
	"github.com/marpio/img-store/fsutils"
	"github.com/nfnt/resize"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/spf13/afero"
)

type Extractor interface {
	Extract(pathsGroupedByDir map[string][]*fsutils.FileInfo, fileReader fsutils.FileReaderFn) <-chan *entity.PhotoWithThumb
}

type extractor struct {
	extractCreatedAt func(r fsutils.File) (time.Time, error)
	extractThumbnail func(r io.ReadSeeker) ([]byte, error)
}

func NewExtractor(fs afero.Fs) Extractor {
	return &extractor{extractCreatedAt: extractCreatedAt, extractThumbnail: extractThumb}
}

func (s extractor) Extract(pathsGroupedByDir map[string][]*fsutils.FileInfo, fileReader fsutils.FileReaderFn) <-chan *entity.PhotoWithThumb {
	metadataStream := make(chan *entity.PhotoWithThumb, 100)
	go func() {
		defer close(metadataStream)
		var wg sync.WaitGroup
		wg.Add(len(pathsGroupedByDir))
		for _, paths := range pathsGroupedByDir {
			go func(ps []*fsutils.FileInfo) {
				defer wg.Done()
				s.extractMetadataDir(ps, metadataStream, fileReader)
			}(paths)
		}
		wg.Wait()
	}()
	return metadataStream
}

func (s extractor) extractMetadataDir(photos []*fsutils.FileInfo, metadataStream chan<- *entity.PhotoWithThumb, fileReader fsutils.FileReaderFn) {
	dirCreatedAt := time.Time{}
	md := make([]*entity.PhotoWithThumb, 0, len(photos))
	for _, ph := range photos {
		f, err := fileReader(ph.Path)
		if err != nil {
			log.Printf("error opening file %v: %v", ph.Path, err)
		}
		defer f.Close()

		createdAt, err := s.extractCreatedAt(f)
		if err != nil {
			log.Printf("can't extract created_at for path: %v: %v", ph.Path, err)
		} else {
			dirCreatedAt = createdAt
		}

		createdAtMonth := time.Date(createdAt.Year(), createdAt.Month(), 1, 0, 0, 0, 0, time.UTC)
		f.Seek(0, 0)
		thumb, err := s.extractThumbnail(f)
		if err != nil {
			log.Printf("can't extract thumbnail for path: %v: %v", ph.Path, err)
		}
		thumbnailName := fsutils.GenerateUniqueFileName("thumb", ph.Path, createdAt)
		imgName := fsutils.GenerateUniqueFileName("orig", ph.Path, createdAt)
		p := &entity.PhotoWithThumb{Photo: &entity.Photo{FileInfo: ph, Metadata: &entity.Metadata{Name: imgName, ThumbnailName: thumbnailName, CreatedAt: createdAt, CreatedAtMonth: createdAtMonth}}, Thumbnail: thumb}
		md = append(md, p)
	}
	for _, meta := range md {
		if (meta.CreatedAt == time.Time{}) {
			meta.CreatedAt = dirCreatedAt
		}
		metadataStream <- meta
	}
}

func extractCreatedAt(r fsutils.File) (time.Time, error) {
	x, err := exif.Decode(r)
	if err != nil {
		return time.Time{}, err
	}
	imgCreatedAt, err := x.DateTime()
	if err != nil {
		return time.Time{}, err
	}
	return imgCreatedAt, nil
}

func extractThumb(r io.ReadSeeker) ([]byte, error) {
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

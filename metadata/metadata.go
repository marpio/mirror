package metadata

import (
	"bufio"
	"bytes"
	"context"
	"image/jpeg"
	"io"
	"log"
	"sync"
	"time"

	"github.com/marpio/img-store/domain"
	"github.com/marpio/img-store/localstorage"
	"github.com/nfnt/resize"
	"github.com/rwcarlsen/goexif/exif"
)

type extractor struct {
	rd domain.StorageReadSeeker
}

func NewExtractor(rd domain.StorageReadSeeker) domain.Extractor {
	return &extractor{rd: rd}
}

func (s extractor) Extract(ctx context.Context, files []*domain.FileInfo) <-chan *domain.Photo {
	pathsGroupedByDir := localstorage.GroupByDir(files)
	metadataStream := make(chan *domain.Photo, 100)
	go func() {
		defer close(metadataStream)
		var wg sync.WaitGroup
		wg.Add(len(pathsGroupedByDir))
		for _, paths := range pathsGroupedByDir {
			select {
			case <-ctx.Done():
				break
			default:
				go func(ps []*domain.FileInfo) {
					defer wg.Done()
					s.extractMetadataDir(ctx, metadataStream, ps)
				}(paths)
			}
		}
		wg.Wait()
	}()
	return metadataStream
}

func (s extractor) extractMetadataDir(ctx context.Context, metadataStream chan<- *domain.Photo, photos []*domain.FileInfo) {
	dirCreatedAt := time.Time{}
	md := make([]*domain.Photo, 0, len(photos))
	for _, ph := range photos {
		select {
		case <-ctx.Done():
			break
		default:
			f, err := s.rd.NewReadSeeker(ph.FilePath)
			if err != nil {
				log.Printf("error opening file %v: %v", ph.FilePath, err)
			}
			defer f.Close()

			createdAt, err := extractCreatedAt(f)
			if err != nil {
				log.Printf("can't extract created_at for path: %v: %v", ph.FilePath, err)
			} else {
				dirCreatedAt = createdAt
			}
			f.Seek(0, 0)
			thumb, err := extractThumb(f)
			if err != nil {
				log.Printf("can't extract thumbnail for path: %v: %v", ph.FilePath, err)
			}

			p := &domain.Photo{FileInfo: ph, Metadata: &domain.Metadata{CreatedAt: createdAt, Thumbnail: thumb}}
			md = append(md, p)
		}
	}
	for _, meta := range md {
		if (meta.CreatedAt == time.Time{}) {
			meta.CreatedAt = dirCreatedAt
		}
		metadataStream <- meta
	}
}

func extractCreatedAt(r domain.ReadCloseSeeker) (time.Time, error) {
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

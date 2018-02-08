package metadata

import (
	"bufio"
	"bytes"
	"context"
	"image/jpeg"
	"io"
	"sync"
	"time"

	"github.com/apex/log"
	"github.com/marpio/mirror/domain"
	"github.com/marpio/mirror/localstorage"
	"github.com/nfnt/resize"
	"github.com/rwcarlsen/goexif/exif"
)

type extractor struct {
	rd domain.StorageReadSeeker
}

func NewExtractor(rd domain.StorageReadSeeker) domain.Extractor {
	return &extractor{rd: rd}
}

func (s extractor) Extract(ctx context.Context, logctx log.Interface, files []*domain.FileInfo) <-chan *domain.Photo {
	pathsGroupedByDir := localstorage.GroupByDir(files)
	metadataStream := make(chan *domain.Photo, 100)

	go func() {
		defer close(metadataStream)
		var wg sync.WaitGroup
		wg.Add(len(pathsGroupedByDir))
		for dir, paths := range pathsGroupedByDir {
			select {
			case <-ctx.Done():
				return
			default:
				go func(dir string, ps []*domain.FileInfo) {
					loger := logctx.WithFields(log.Fields{
						"extracting_metadata_dir": dir,
					})
					loger.Info("starting to extract metadata.")

					defer wg.Done()
					s.extractMetadataDir(ctx, loger, metadataStream, ps)
					loger.Info("done extracting metadata.")
				}(dir, paths)
			}
		}
		wg.Wait()
	}()
	return metadataStream
}

func (s extractor) extractMetadataDir(ctx context.Context, logctx log.Interface, metadataStream chan<- *domain.Photo, photos []*domain.FileInfo) {
	dirCreatedAt := time.Time{}
	md := make([]*domain.Photo, 0, len(photos))
loop:
	for _, ph := range photos {
		logctx = log.WithFields(log.Fields{
			"photo_path": ph.FilePath,
		})
		f, err := s.rd.NewReadSeeker(ctx, ph.FilePath)
		if err != nil {
			logctx.WithError(err)
			continue loop
		}
		defer f.Close()

		createdAt, err := extractCreatedAt(f)
		if err != nil {
			logctx.WithError(err)
			continue loop
		} else {
			dirCreatedAt = createdAt
		}
		f.Seek(0, 0)
		thumb, err := extractThumb(f)
		if err != nil {
			logctx.WithError(err)
			continue loop
		}

		p := &domain.Photo{FileInfo: ph, Metadata: &domain.Metadata{CreatedAt: createdAt, Thumbnail: thumb}}
		md = append(md, p)
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

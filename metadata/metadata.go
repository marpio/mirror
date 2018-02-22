package metadata

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/apex/log"
	"github.com/marpio/mirror/domain"
	"github.com/nfnt/resize"
	"github.com/rwcarlsen/goexif/exif"
)

type extractor struct {
	rd domain.StorageReadSeeker
}

func NewExtractor(rd domain.StorageReadSeeker) domain.Extractor {
	return &extractor{rd: rd}
}

func (s extractor) Extract(ctx context.Context, logctx log.Interface, filesByDirStream <-chan []*domain.FileInfo) <-chan domain.Photo {
	metadataStream := make(chan domain.Photo, 100)

	go func() {
		defer close(metadataStream)
		var wg sync.WaitGroup
		for paths := range filesByDirStream {
			select {
			case <-ctx.Done():
				return
			default:
				wg.Add(1)
				go func(filesByDir []*domain.FileInfo) {
					logctx.Infof("%v", filesByDir)
					defer wg.Done()
					s.extractMetadataDir(ctx, logctx, metadataStream, filesByDir)
				}(paths)
			}
		}
		wg.Wait()
	}()
	return metadataStream
}

func (s extractor) extractMetadataDir(ctx context.Context, logctx log.Interface, metadataStream chan<- domain.Photo, photos []*domain.FileInfo) {
	dirCreatedAt := time.Time{}
	md := make([]domain.Photo, 0, len(photos))
loop:
	for _, ph := range photos {
		logctx = log.WithFields(log.Fields{
			"photo_path": ph.FilePath,
		})
		var p domain.Photo
		var err error
		switch ext := strings.ToLower(ph.FileExt); ext {
		case ".nef":
			p, err = extractMetadataNEF(ctx, ph, s.rd)
		case ".jpg":
		case ".jpeg":
			p, err = extractMetadataJpg(ctx, logctx, ph, s.rd)
		default:
			err = fmt.Errorf("not supported format %s", ext)
		}
		if err != nil {
			logctx.Errorf("woow %v", err)
			continue loop
		}

		dirCreatedAt = p.CreatedAt()
		md = append(md, p)
	}
	for _, meta := range md {
		if (meta.CreatedAt() == time.Time{}) {
			meta.SetCreatedAt(dirCreatedAt)
		}
		metadataStream <- meta
	}
}

func extractMetadataNEF(ctx context.Context, fi *domain.FileInfo, rs domain.StorageReadSeeker) (domain.Photo, error) {
	f, err := rs.NewReadSeeker(ctx, fi.FilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	createdAt, err := extractCreatedAt(f)
	if err != nil {
		return nil, err
	}
	thumb, err := extractThumbNEF(fi.FilePath)
	if err != nil {
		return nil, err
	}
	readerFn := func() (io.ReadCloser, error) { return extractJpgNEF(fi.FilePath) }
	p := domain.NewPhoto(fi, &domain.Metadata{CreatedAt: createdAt, Thumbnail: thumb}, readerFn)
	return p, nil
}

func extractMetadataJpg(ctx context.Context, logctx log.Interface, fi *domain.FileInfo, rs domain.StorageReadSeeker) (domain.Photo, error) {
	f, err := rs.NewReadSeeker(ctx, fi.FilePath)
	if err != nil {
		logctx.Infof("error %v", fi.FilePath)
		return nil, err
	}
	defer f.Close()

	createdAt, err := extractCreatedAt(f)
	if err != nil {
		logctx.Infof("error %v", fi.FilePath)
		return nil, err
	}

	f.Seek(0, 0)
	thumb, err := extractThumb(f)
	if err != nil {
		logctx.Infof("error %v", fi.FilePath)
		return nil, err
	}
	readerFn := func() (io.ReadCloser, error) { return rs.NewReadSeeker(ctx, fi.FilePath) }
	p := domain.NewPhoto(fi, &domain.Metadata{CreatedAt: createdAt, Thumbnail: thumb}, readerFn)
	if p == nil {
		logctx.Infof("nil pointer %v", fi.FilePath)
	}
	return p, nil
}

func extractThumbNEF(path string) ([]byte, error) {
	cmd := exec.Command("exiftool", "-b", "-PreviewImage", path)
	r, err := cmd.StdoutPipe()

	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func extractJpgNEF(path string) (io.ReadCloser, error) {
	cmd := exec.Command("exiftool", "-b", "-JpgFromRaw", path)
	r, err := cmd.StdoutPipe()

	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return r, nil
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

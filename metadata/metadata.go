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
	"path"
	"strings"
	"sync"
	"time"

	"github.com/apex/log"
	"github.com/marpio/mirror"
	"github.com/nfnt/resize"
	"github.com/rwcarlsen/goexif/exif"
)

type Metadata struct {
	CreatedAt time.Time
	Thumbnail []byte
}

type Photo struct {
	mirror.FileInfo
	*Metadata
	jpegReaderProvider func() (io.ReadCloser, error)
}

func (ph *Photo) FilePath() string {
	return ph.FileInfo.FilePath()
}

func (ph *Photo) CreatedAt() time.Time {
	return ph.Metadata.CreatedAt
}

func (ph *Photo) SetCreatedAt(t time.Time) {
	ph.Metadata.CreatedAt = t
}

func (ph *Photo) Thumbnail() []byte {
	return ph.Metadata.Thumbnail
}

func (ph *Photo) NewJpgReader() (io.ReadCloser, error) {
	return ph.jpegReaderProvider()
}

func NewPhoto(fi mirror.FileInfo, meta *Metadata, jpegReaderProvider func() (io.ReadCloser, error)) mirror.LocalPhoto {
	return &Photo{
		FileInfo:           fi,
		Metadata:           meta,
		jpegReaderProvider: jpegReaderProvider,
	}
}

func (p *Photo) ThumbID() string {
	return "thumb_" + p.ID()
}

func (p *Photo) Dir() string {
	return fmt.Sprintf("%d-%02d", p.CreatedAt().Year(), p.CreatedAt().Month())
}

type Extractor struct {
	rd mirror.StorageReader
}

func NewExtractor(rd mirror.StorageReader) *Extractor {
	return &Extractor{rd: rd}
}

func (s Extractor) Extract(ctx context.Context, logctx log.Interface, photos []mirror.FileInfo) []mirror.LocalPhoto {
	md := make([]mirror.LocalPhoto, len(photos), len(photos))
	var wg sync.WaitGroup
	wg.Add(len(photos))
	for i, ph := range photos {
		go func(i int, ph mirror.FileInfo) {
			defer wg.Done()
			logger := log.WithFields(log.Fields{
				"photo_path": ph.FilePath(),
			})
			var p mirror.LocalPhoto
			var err error
			ext := strings.ToLower(path.Ext(ph.FilePath()))
			switch ext {
			case ".nef":
				p, err = extractMetadataNEF(ctx, ph, s.rd)
			case ".jpg", ".jpeg":
				p, err = extractMetadataJpg(ctx, logger, ph, s.rd)
			default:
				err = fmt.Errorf("not supported format %s", ext)
			}
			if err != nil {
				logger.Errorf("error extracting metadata %v", err)
				return
			}
			md[i] = p
		}(i, ph)
	}
	wg.Wait()
	dirCreatedAt := time.Time{}
	res := make([]mirror.LocalPhoto, 0)
	for _, p := range md {
		if p != nil {
			res = append(res, p)
			if (p.CreatedAt() != time.Time{}) {
				dirCreatedAt = p.CreatedAt()
			}
		}
	}

	for _, p := range res {
		if (p.CreatedAt() == time.Time{}) {
			p.SetCreatedAt(dirCreatedAt)
		}
	}
	return res
}

func extractMetadataNEF(ctx context.Context, fi mirror.FileInfo, rs mirror.StorageReader) (mirror.LocalPhoto, error) {
	f, err := rs.NewReader(ctx, fi.FilePath())
	if err != nil {
		return nil, err
	}
	defer f.Close()

	createdAt, err := extractCreatedAt(f)
	if err != nil {
		return nil, err
	}

	thumb, err := extractThumbNEF(fi.FilePath())
	if err != nil {
		return nil, err
	}
	readerFn := func() (io.ReadCloser, error) { return extractJpgNEF(fi.FilePath()) }
	p := NewPhoto(fi, &Metadata{CreatedAt: createdAt, Thumbnail: thumb}, readerFn)
	return p, nil
}

func extractMetadataJpg(ctx context.Context, logctx log.Interface, fi mirror.FileInfo, rs mirror.StorageReader) (mirror.LocalPhoto, error) {
	f, err := rs.NewReader(ctx, fi.FilePath())
	if err != nil {
		logctx.Errorf("error %v", fi.FilePath())
		return nil, err
	}
	defer f.Close()

	createdAt, err := extractCreatedAt(f)
	if err != nil {
		logctx.Errorf("error %v", fi.FilePath())
		return nil, err
	}
	s, ok := f.(io.Seeker)
	if ok {
		s.Seek(0, 0)
	} else {
		f.Close()
		f, err = rs.NewReader(ctx, fi.FilePath())
	}
	thumb, err := extractThumb(f)
	if err != nil {
		logctx.Errorf("error %v", fi.FilePath())
		return nil, err
	}
	readerFn := func() (io.ReadCloser, error) { return rs.NewReader(ctx, fi.FilePath()) }
	p := NewPhoto(fi, &Metadata{CreatedAt: createdAt, Thumbnail: thumb}, readerFn)

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

func extractCreatedAt(r io.Reader) (time.Time, error) {
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

func extractThumb(r io.Reader) ([]byte, error) {
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

func resizeImg(r io.Reader) ([]byte, error) {
	s, ok := r.(io.Seeker)
	if ok {
		s.Seek(0, 0)
	} else {
		return nil, fmt.Errorf("r must implement io.Seeker for resize to work")
	}
	s.Seek(0, 0)
	img, err := jpeg.Decode(r)
	if err != nil {
		return nil, err
	}
	m := resize.Thumbnail(160, 120, img, resize.NearestNeighbor)
	var b bytes.Buffer
	writer := bufio.NewWriter(&b)
	if err := jpeg.Encode(writer, m, &jpeg.Options{Quality: 40}); err != nil {
		return nil, err
	}
	if err := writer.Flush(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func isSeeker(r io.Reader) bool {
	_, ok := r.(io.Seeker)
	return ok
}

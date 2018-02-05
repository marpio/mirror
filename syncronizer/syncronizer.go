package syncronizer

import (
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	"github.com/apex/log"
	"github.com/marpio/img-store/domain"
)

type Service struct {
	remotestrg           domain.Storage
	metadataStore        domain.MetadataRepo
	localstrg            domain.LocalStorage
	metadataextr         domain.Extractor
	maxConcurrentUploads int
	timeout              time.Duration
}

type option func(*Service)

func WithMaxConcurrentUploads(maxConcurrentUploads int) option {
	return func(s *Service) {
		s.maxConcurrentUploads = maxConcurrentUploads
	}
}
func WithTimeout(t time.Duration) option {
	return func(s *Service) {
		s.timeout = t
	}
}

func New(remotestorage domain.Storage,
	metadataStore domain.MetadataRepo, localFilesRepo domain.LocalStorage,
	metadataextr domain.Extractor,
	options ...option) *Service {

	s := &Service{remotestrg: remotestorage, metadataStore: metadataStore, localstrg: localFilesRepo, metadataextr: metadataextr, maxConcurrentUploads: 10, timeout: time.Minute}
	for _, opt := range options {
		opt(s)
	}
	return s
}

func (s *Service) Execute(ctx context.Context, rootPath string) {
	filterFn := func(it *domain.FileInfo) bool {
		exists, _ := s.metadataStore.Exists(it.ID())
		if !exists {
			return true
		}
		modTime, _ := s.metadataStore.GetModTime(it.ID())
		isModified := modTime != it.ModTimeHash()
		if isModified {
			s.metadataStore.Delete(it.ID())
		}
		return isModified
	}

	newAndModifiedFiles := s.localstrg.SearchFiles(rootPath, filterFn, ".jpg", ".jpeg")

	photosStream := s.metadataextr.Extract(ctx, newAndModifiedFiles)
	syncedPhotosStream := s.syncWithRemoteStorage(ctx, photosStream)
	s.saveToDb(ctx, syncedPhotosStream)
}

func (s *Service) saveToDb(ctx context.Context, uploadedPhotosStream <-chan *domain.Photo) {
	for {
		select {
		case p, more := <-uploadedPhotosStream:
			if more {
				s.metadataStore.Add(p)
			} else {
				c, cancel := context.WithTimeout(ctx, time.Minute)
				defer cancel()
				if err := s.metadataStore.Persist(c); err != nil {
					log.Fatalf("error commiting to DB %v", err)
				}
				return
			}
		case <-ctx.Done():
			c, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			if err := s.metadataStore.Persist(c); err != nil {
				log.Fatalf("error commiting to DB %v", err)
			}
			return
		}
	}
}

func (s *Service) syncWithRemoteStorage(ctx context.Context, metadataStream <-chan *domain.Photo) <-chan *domain.Photo {

	uploadedPhotosStream := make(chan *domain.Photo)

	go func() {
		limiter := make(chan struct{}, s.maxConcurrentUploads)
		var wg sync.WaitGroup
		defer close(uploadedPhotosStream)
	loop:
		for {
			select {
			case metaData, ok := <-metadataStream:
				if !ok {
					break loop
				}
				limiter <- struct{}{}
				wg.Add(1)
				go func(m *domain.Photo) {
					defer wg.Done()
					defer func() { <-limiter }()
					s.uploadPhoto(ctx, m)
					s.uploadThumb(ctx, m)
					uploadedPhotosStream <- m
				}(metaData)
			case <-ctx.Done():
				break loop
			}
		}
		wg.Wait()
	}()
	return uploadedPhotosStream
}

func (s *Service) uploadPhoto(ctx context.Context, img *domain.Photo) {
	logctx := log.WithFields(log.Fields{
		"photoFilePath": img.FilePath,
	})
	defer logctx.Trace("uploading")
	c, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	f, err := s.localstrg.NewReader(c, img.FilePath)
	if err != nil {
		logctx.WithError(err)
		return
	}
	defer f.Close()

	w := s.remotestrg.NewWriter(c, img.ID())
	_, err = io.Copy(w, f)
	if err != nil {
		logctx.WithError(err)
		return
	}
	if err := w.Close(); err != nil {
		logctx.WithError(err)
		return
	}
}

func (s *Service) uploadThumb(ctx context.Context, img *domain.Photo) {
	logctx := log.WithFields(log.Fields{
		"thumbFilePath": img.FilePath,
	})
	defer logctx.Trace("uploading")
	c, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	w := s.remotestrg.NewWriter(c, img.ThumbID())
	_, err := io.Copy(w, bytes.NewReader(img.Thumbnail))
	if err != nil {
		logctx.WithError(err)
		return
	}
	if err := w.Close(); err != nil {
		logctx.WithError(err)
		return
	}
}

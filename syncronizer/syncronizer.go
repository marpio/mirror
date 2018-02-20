package syncronizer

import (
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	"github.com/apex/log"
	"github.com/marpio/mirror/domain"
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
	metadataStore domain.MetadataRepo,
	localFilesRepo domain.LocalStorage,
	metadataextr domain.Extractor,
	options ...option) *Service {

	s := &Service{
		remotestrg:           remotestorage,
		metadataStore:        metadataStore,
		localstrg:            localFilesRepo,
		metadataextr:         metadataextr,
		maxConcurrentUploads: 10,
		timeout:              1 * time.Minute,
	}
	for _, opt := range options {
		opt(s)
	}
	return s
}

func (s *Service) Execute(ctx context.Context, logctx log.Interface, rootPath string) {
	isNewfilterFn := func(fi *domain.FileInfo) bool {
		exists, err := s.metadataStore.Exists(fi.ID())
		if err != nil {
			return false
		}
		return !exists
	}

	newFiles, oldFiles := s.localstrg.SearchFiles(rootPath, isNewfilterFn, ".jpg", ".jpeg", ".nef")
	logctx.Infof("%d element(s) to sync.", len(newFiles))
	photosStream := s.metadataextr.Extract(ctx, logctx, newFiles)
	syncedPhotosStream := s.syncWithRemoteStorage(ctx, logctx, photosStream)
	s.saveToDb(ctx, syncedPhotosStream)
	allFiles := make(map[string]struct{})
	for _, fi := range newFiles {
		if _, ok := allFiles[fi.ID()]; !ok {
			allFiles[fi.ID()] = struct{}{}
		}
	}
	for _, fi := range oldFiles {
		if _, ok := allFiles[fi.ID()]; !ok {
			allFiles[fi.ID()] = struct{}{}
		}
	}
	for _, f := range s.metadataStore.GetAll() {
		if _, ok := allFiles[f.ID()]; !ok {
			s.metadataStore.Delete(f.ID())
			s.remotestrg.Delete(ctx, f.ID())
		}
	}
}

func (s *Service) saveToDb(ctx context.Context, uploadedPhotosStream <-chan domain.Photo) {
	for {
		select {
		case p, more := <-uploadedPhotosStream:
			if more {
				s.metadataStore.Add(p)
			} else {
				if err := s.metadataStore.Persist(ctx); err != nil {
					log.Fatalf("error commiting to DB %v", err)
				}
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) syncWithRemoteStorage(ctx context.Context, logctx log.Interface, metadataStream <-chan domain.Photo) <-chan domain.Photo {
	uploadedPhotosStream := make(chan domain.Photo)
	logctx = logctx.WithFields(log.Fields{
		"action": "sync_with_remote_storage",
	})
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
				go func(m domain.Photo) {
					defer wg.Done()
					defer func() { <-limiter }()
					logctx = logctx.WithFields(log.Fields{
						"photo_path": m.FilePath(),
					})
					c, cancel := context.WithTimeout(ctx, s.timeout)
					defer cancel()
					if err := s.uploadPhoto(c, logctx, m); err == nil {
						s.uploadThumb(c, logctx, m)
						uploadedPhotosStream <- m
					}
				}(metaData)
			case <-ctx.Done():
				return
			}
		}
		wg.Wait()
	}()
	return uploadedPhotosStream
}

func (s *Service) uploadPhoto(ctx context.Context, logctx log.Interface, img domain.Photo) error {
	logctx.Info("uploading photo.")
	f, err := img.NewJpgReader()
	if err != nil {
		logctx.WithError(err)
		return err
	}
	defer f.Close()

	w := s.remotestrg.NewWriter(ctx, img.ID())
	_, err = io.Copy(w, f)
	if err != nil {
		logctx.WithError(err)
		return err
	}
	if err := w.Close(); err != nil {
		logctx.WithError(err)
		return err
	}
	logctx.Info("photo upload complete.")
	return nil
}

func (s *Service) uploadThumb(ctx context.Context, logctx log.Interface, img domain.Photo) {
	logctx.Info("uploading thumb.")
	w := s.remotestrg.NewWriter(ctx, img.ThumbID())
	_, err := io.Copy(w, bytes.NewReader(img.Thumbnail()))
	if err != nil {
		logctx.WithError(err)
		return
	}
	if err := w.Close(); err != nil {
		logctx.WithError(err)
		return
	}
	logctx.Info("thumb upload complete.")
}

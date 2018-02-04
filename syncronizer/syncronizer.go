package syncronizer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/marpio/img-store/domain"
	"github.com/marpio/img-store/repository"
)

type Service struct {
	remotestrg           domain.Storage
	metadataStore        repository.Service
	localstrg            domain.LocalStorage
	metadataextr         domain.Extractor
	maxConcurrentUploads int
}

type option func(*Service)

func WithMaxConcurrentUploads(maxConcurrentUploads int) option {
	return func(s *Service) {
		s.maxConcurrentUploads = maxConcurrentUploads
	}
}

func New(remotestorage domain.Storage,
	metadataStore repository.Service,
	localFilesRepo domain.LocalStorage,
	metadataextr domain.Extractor,
	options ...option) *Service {

	s := &Service{remotestrg: remotestorage, metadataStore: metadataStore, localstrg: localFilesRepo, metadataextr: metadataextr, maxConcurrentUploads: 10}
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
				if err := s.metadataStore.Persist(); err != nil {
					log.Fatalf("error commiting to DB %v", err)
				}
				return
			}
		case <-ctx.Done():
			if err := s.metadataStore.Persist(); err != nil {
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
					err := s.uploadPhoto(m)
					if err != nil {
						log.Print(err)
						return
					}
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

func (s *Service) uploadPhoto(img *domain.Photo) error {
	f, err := s.localstrg.NewReader(img.FilePath)
	if err != nil {
		return fmt.Errorf("error opening file %v: %v", img.FilePath, err)
	}
	defer f.Close()

	w := s.remotestrg.NewWriter(img.ThumbID())
	_, err = io.Copy(w, bytes.NewReader(img.Thumbnail))
	if err != nil {
		return fmt.Errorf("error writing thumbnail. path: %v, err: %v", img.FilePath, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("error closing remote storage writer %v", err)
	}

	w = s.remotestrg.NewWriter(img.ID())
	_, err = io.Copy(w, f)
	if err != nil {
		return fmt.Errorf("error writing photo. path: %v, err: %v", img.FilePath, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("error closing remote storage writer %v", err)
	}

	return nil
}

package syncronizer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/apex/log"

	"github.com/marpio/mirror"
	"github.com/marpio/mirror/storage"
)

type Service struct {
	remotestrg           mirror.Storage
	metadataStore        mirror.MetadataRepo
	localstrg            mirror.ReadOnlyStorage
	metadataextr         mirror.Extractor
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

func New(remotestorage mirror.Storage,
	metadataStore mirror.MetadataRepo,
	localFilesRepo mirror.ReadOnlyStorage,
	metadataextr mirror.Extractor,
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
	logctx.Info("starting")
	files := s.localstrg.SearchFiles(rootPath, ".jpg", ".jpeg", ".nef")
	logctx.Infof("no. of files: %d", len(files))
	pathsGroupedByDir := storage.GroupByDir(files)
	unsyncedFilesByDir := s.getUnsyncedFiles(ctx, logctx, pathsGroupedByDir)
	photosStream := s.metadataextr.Extract(ctx, logctx, unsyncedFilesByDir)
	syncedPhotosStream := s.syncWithRemoteStorage(ctx, logctx, photosStream)
	s.saveToDb(ctx, syncedPhotosStream)
	allFiles := make(map[string]struct{})
	for _, fi := range files {
		if _, ok := allFiles[fi.ID()]; !ok {
			allFiles[fi.ID()] = struct{}{}
		}
	}
}

func (s *Service) getUnsyncedFiles(ctx context.Context, logctx log.Interface, pathsGroupedByDir map[string][]*mirror.FileInfo) <-chan []*mirror.FileInfo {
	fileInfoStream := make(chan []*mirror.FileInfo)
	go func() {
		defer close(fileInfoStream)
		for _, dirFiles := range pathsGroupedByDir {
			dirFileInfoStream := make([]*mirror.FileInfo, len(dirFiles), len(dirFiles))
			var wg sync.WaitGroup
			wg.Add(len(dirFiles))
			for i, fi := range dirFiles {
				select {
				case <-ctx.Done():
					return
				default:
					go func(i int, fi *mirror.FileInfo) {
						defer wg.Done()
						exists, _ := s.metadataStore.Exists(fi.ID())
						if !exists {
							dirFileInfoStream[i] = fi
						}
					}(i, fi)
				}
			}
			wg.Wait()
			newFiles := make([]*mirror.FileInfo, 0)
			send := false
			for _, elem := range dirFileInfoStream {
				if elem != nil {
					newFiles = append(newFiles, elem)
					send = true
				}
			}
			if send {
				fileInfoStream <- newFiles
			}
		}
	}()
	return fileInfoStream
}

func (s *Service) saveToDb(ctx context.Context, uploadedPhotosStream <-chan mirror.Photo) {
	for {
		select {
		case <-ctx.Done():
			return
		case p, more := <-uploadedPhotosStream:
			if more {
				s.metadataStore.Add(p)
			} else {
				if err := s.metadataStore.Persist(ctx); err != nil {
					log.Fatalf("error commiting to DB %v", err)
				}
				return
			}
		}
	}
}

func (s *Service) syncWithRemoteStorage(ctx context.Context, logctx log.Interface, metadataStream <-chan mirror.Photo) <-chan mirror.Photo {
	uploadedPhotosStream := make(chan mirror.Photo)
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
			case <-ctx.Done():
				return
			case metaData, ok := <-metadataStream:
				if !ok {
					break loop
				}
				limiter <- struct{}{}
				wg.Add(1)
				go func(m mirror.Photo) {
					defer wg.Done()
					defer func() { <-limiter }()
					logctx = logctx.WithFields(log.Fields{
						"photo_path": m.FilePath(),
					})
					if err := s.uploadPhoto(ctx, logctx, m); err == nil {
						s.uploadThumb(ctx, logctx, m)
						uploadedPhotosStream <- m
					} else {
						logctx.Errorf("error uploading file: %v", err)
					}
				}(metaData)
			}
		}
		wg.Wait()
	}()
	return uploadedPhotosStream
}

func (s *Service) uploadPhoto(ctx context.Context, logctx log.Interface, img mirror.Photo) error {
	f, err := img.NewJpgReader()
	if err != nil {
		logctx.WithError(err).Errorf("error uploading file %s", img.FilePath)
		return err
	}
	defer f.Close()

	w := s.remotestrg.NewWriter(ctx, img.ID())
	_, err = io.Copy(w, f)
	if err != nil {
		logctx.WithError(err).Errorf("error uploading file %s", img.FilePath)
		fmt.Printf("err %v", err)
		return err
	}
	if err := w.Close(); err != nil {
		logctx.WithError(err).Error("error closing writer")
		return err
	}

	return nil
}

func (s *Service) uploadThumb(ctx context.Context, logctx log.Interface, img mirror.Photo) {
	w := s.remotestrg.NewWriter(ctx, img.ThumbID())
	_, err := io.Copy(w, bytes.NewReader(img.Thumbnail()))
	if err != nil {
		logctx.WithError(err).Errorf("error uploading thumb file %s", img.FilePath)
		return
	}
	if err := w.Close(); err != nil {
		logctx.WithError(err).Error("error closing writer")
		return
	}
}

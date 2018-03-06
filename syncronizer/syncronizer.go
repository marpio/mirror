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
	fileExts             []string
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

func WithFileExts(exts ...string) option {
	return func(s *Service) {
		s.fileExts = exts
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
		fileExts:             []string{".jpg", ".jpeg", ".nef"},
	}
	for _, opt := range options {
		opt(s)
	}
	return s
}

func (s *Service) Execute(ctx context.Context, logctx log.Interface, rootPath string) {
	files := s.localstrg.SearchFiles(rootPath, s.fileExts...)
	logctx.Infof("found %d files to sync", len(files))

	unsyncedFilesByDir := s.getUnsyncedFiles(ctx, logctx, storage.GroupByDir(files))
	photosStream := s.extractMetadata(ctx, logctx, unsyncedFilesByDir)
	syncedPhotosStream := s.syncRemoteStorage(ctx, logctx, photosStream)

	s.syncMetadataRepo(ctx, files, syncedPhotosStream)
}

func (s *Service) getUnsyncedFiles(ctx context.Context, logctx log.Interface, pathsGroupedByDir map[string][]mirror.FileInfo) <-chan []mirror.FileInfo {
	fileInfoStream := make(chan []mirror.FileInfo)
	go func() {
		defer close(fileInfoStream)
		for _, dirFiles := range pathsGroupedByDir {
			dirFileInfoStream := make([]mirror.FileInfo, len(dirFiles), len(dirFiles))
			var wg sync.WaitGroup
			wg.Add(len(dirFiles))
			for i, fi := range dirFiles {
				select {
				case <-ctx.Done():
					return
				default:
					go func(i int, fi mirror.FileInfo) {
						defer wg.Done()
						exists, _ := s.metadataStore.Exists(fi.ID())
						if !exists {
							dirFileInfoStream[i] = fi
						}
					}(i, fi)
				}
			}
			wg.Wait()
			newFiles := make([]mirror.FileInfo, 0)
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

func (s *Service) extractMetadata(ctx context.Context, logctx log.Interface, filesByDirStream <-chan []mirror.FileInfo) <-chan mirror.LocalPhoto {
	metadataStream := make(chan mirror.LocalPhoto, 2*s.maxConcurrentUploads)

	go func() {
		defer close(metadataStream)
		var wg sync.WaitGroup
		for paths := range filesByDirStream {
			select {
			case <-ctx.Done():
				return
			default:
				md := s.metadataextr.Extract(ctx, logctx, paths)
				for _, p := range md {
					metadataStream <- p
				}
			}
		}
		wg.Wait()
	}()
	return metadataStream
}

func (s *Service) syncRemoteStorage(ctx context.Context, logctx log.Interface, metadataStream <-chan mirror.LocalPhoto) <-chan mirror.LocalPhoto {
	uploadedPhotosStream := make(chan mirror.LocalPhoto)
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
				go func(m mirror.LocalPhoto) {
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

func (s *Service) uploadPhoto(ctx context.Context, logctx log.Interface, img mirror.LocalPhoto) error {
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

func (s *Service) uploadThumb(ctx context.Context, logctx log.Interface, img mirror.LocalPhoto) {
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

func (s *Service) syncMetadataRepo(ctx context.Context, files []mirror.FileInfo, uploadedPhotosStream <-chan mirror.LocalPhoto) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		s.addNewFiles(ctx, uploadedPhotosStream)
	}()
	go func() {
		defer wg.Done()
		s.removeDeletedFiles(ctx, files)
	}()
	wg.Wait()
	s.metadataStore.Persist(ctx)
}

func (s *Service) addNewFiles(ctx context.Context, uploadedPhotosStream <-chan mirror.LocalPhoto) {
	for {
		select {
		case <-ctx.Done():
			return
		case p, more := <-uploadedPhotosStream:
			if more {
				s.metadataStore.Add(p)
			} else {
				return
			}
		}
	}
}

func (s *Service) removeDeletedFiles(ctx context.Context, localFiles []mirror.FileInfo) {
	localFileIDs := make(map[string]struct{})
	for _, fi := range localFiles {
		if _, ok := localFileIDs[fi.ID()]; !ok {
			localFileIDs[fi.ID()] = struct{}{}
		}
	}
	var wg sync.WaitGroup
	for _, item := range s.metadataStore.GetAll() {
		if _, ok := localFileIDs[item.ID()]; !ok {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				if err := s.remotestrg.Delete(ctx, id); err == nil {
					s.metadataStore.Delete(id)
				}
			}(item.ID())
		}
	}
	wg.Wait()
}

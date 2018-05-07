package syncronizer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/apex/log"

	"github.com/marpio/mirror"
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
	files := s.localstrg.FindFiles(rootPath, s.fileExts...)
	logctx.Infof("found %d files to sync", len(files))
	printMemUsage("after search files")
	unsyncedFilesByDir := s.getUnsyncedFiles(ctx, logctx, GroupByDir(files))
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
				printMemUsage("getUnsyncedFiles")
				fileInfoStream <- newFiles
			}
		}
	}()
	return fileInfoStream
}

func (s *Service) extractMetadata(ctx context.Context, logctx log.Interface, filesByDirStream <-chan []mirror.FileInfo) <-chan mirror.LocalPhoto {
	metadataStream := make(chan mirror.LocalPhoto)

	go func() {
		defer close(metadataStream)
		var wg sync.WaitGroup
		for paths := range filesByDirStream {
			select {
			case <-ctx.Done():
				return
			default:
				c, cancel := context.WithCancel(ctx)
				md := s.metadataextr.Extract(c, logctx, paths)
				for _, p := range md {
					metadataStream <- p
				}
				printMemUsage("extractMetadata")
				cancel()
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
					c, cancel := context.WithCancel(ctx)
					if err := s.uploadPhoto(c, logctx, m); err == nil {
						c, cancel := context.WithCancel(ctx)
						s.uploadThumb(c, logctx, m)
						cancel()
						uploadedPhotosStream <- m
					} else {
						logctx.Errorf("error uploading file: %v", err)
					}
					cancel()
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
	s.addNewFiles(ctx, uploadedPhotosStream)
	s.metadataStore.Persist(ctx)
}

func (s *Service) addNewFiles(ctx context.Context, uploadedPhotosStream <-chan mirror.LocalPhoto) {
	for {
		select {
		case <-ctx.Done():
			return
		case p, ok := <-uploadedPhotosStream:
			if !ok {
				return
			}
			s.metadataStore.Add(p)
		}
	}
}

func GroupByDir(files []mirror.FileInfo) map[string][]mirror.FileInfo {
	filesGroupedByDir := make(map[string][]mirror.FileInfo)
	for _, p := range files {
		dir := filepath.Dir(p.FilePath())
		if v, ok := filesGroupedByDir[dir]; ok {
			v = append(v, p)
			filesGroupedByDir[dir] = v
		} else {
			ps := make([]mirror.FileInfo, 0)
			ps = append(ps, p)
			filesGroupedByDir[dir] = ps
		}
	}
	return filesGroupedByDir
}

func printMemUsage(where string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Println(where)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

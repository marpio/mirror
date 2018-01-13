package syncronizer

import (
	"log"
	"time"

	"github.com/marpio/img-store/filestore"
	"github.com/marpio/img-store/fsutils"
	"github.com/marpio/img-store/metadata"
	"github.com/marpio/img-store/metadatastore"
)

type Service struct {
	fileStore      filestore.Service
	metadataStore  metadatastore.Service
	localFilesRepo fsutils.LocalFilesRepo
	metadataextr   metadata.Extractor
}

func New(fileStore filestore.Service,
	metadataStore metadatastore.Service,
	localFilesRepo fsutils.LocalFilesRepo,
	metadataextr metadata.Extractor) *Service {

	return &Service{fileStore: fileStore, metadataStore: metadataStore, localFilesRepo: localFilesRepo, metadataextr: metadataextr}
}

func (s *Service) Execute(rootPath string, done <-chan interface{}) {
	isChangedOrNew := func(id string, modTime time.Time) bool {
		existing, _ := s.metadataStore.GetByPath(id)
		return (len(existing) == 0 || existing[0].ModTime != modTime)
	}

	newAndChangedPhotos := s.localFilesRepo.SearchFiles(rootPath, isChangedOrNew, ".jpg", ".jpeg")
	for _, p := range newAndChangedPhotos {
		s.metadataStore.Delete(p.Path)
	}

	metadataStream := s.metadataextr.Extract(fsutils.GroupByDir(newAndChangedPhotos), s.localFilesRepo.ReadFile)
	photosStream := filestore.UploadPhotos(metadataStream, s.localFilesRepo.ReadFile, s.fileStore)

	for {
		select {
		case p, more := <-photosStream:
			if more {
				s.metadataStore.Add(p.Photo)
			} else {
				if err := s.metadataStore.Persist(); err != nil {
					log.Fatalf("error commiting to DB %v", err)
				}
				return
			}
		case <-done:
			if err := s.metadataStore.Persist(); err != nil {
				log.Fatalf("error commiting to DB %v", err)
			}
			return
		}
	}
}

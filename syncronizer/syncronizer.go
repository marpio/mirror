package syncronizer

import (
	"log"
	"time"

	"github.com/marpio/img-store/localstorage"
	"github.com/marpio/img-store/metadata"
	"github.com/marpio/img-store/metadatastore"
	"github.com/marpio/img-store/remotestorage"
)

type Service struct {
	remotestrg    remotestorage.Service
	metadataStore metadatastore.Service
	localstrg     localstorage.Service
	metadataextr  metadata.Extractor
}

func New(remotestorage remotestorage.Service,
	metadataStore metadatastore.Service,
	localFilesRepo localstorage.Service,
	metadataextr metadata.Extractor) *Service {

	return &Service{remotestrg: remotestorage, metadataStore: metadataStore, localstrg: localFilesRepo, metadataextr: metadataextr}
}

func (s *Service) Execute(rootPath string, done <-chan interface{}) {
	isChangedOrNew := func(id string, modTime time.Time) bool {
		existing, _ := s.metadataStore.GetByPath(id)
		return (len(existing) == 0 || existing[0].ModTime != modTime)
	}

	newAndChangedPhotos := s.localstrg.SearchFiles(rootPath, isChangedOrNew, ".jpg", ".jpeg")
	for _, p := range newAndChangedPhotos {
		s.metadataStore.Delete(p.Path)
	}

	metadataStream := s.metadataextr.Extract(localstorage.GroupByDir(newAndChangedPhotos), s.localstrg.ReadFile)
	photosStream := remotestorage.UploadPhotos(metadataStream, s.localstrg.ReadFile, s.remotestrg)

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

package syncronizer

import (
	"log"
	"time"

	"github.com/marpio/img-store/filestore"
	"github.com/marpio/img-store/fsutils"
	"github.com/marpio/img-store/metadata"
	"github.com/marpio/img-store/metadatastore"
)

type Syncronizer struct {
	fileStore     filestore.Service
	metadataStore metadatastore.Service
	fileReader    fsutils.FileReaderFn
	photosFinder  func(string, func(id string, modTime time.Time) bool) []*fsutils.FileInfo
	metadataextr  metadata.MetadataExtractor
}

func New(fileStore filestore.Service,
	metadataStore metadatastore.Service,
	fr fsutils.FileReaderFn,
	photosFinder func(string, func(id string, modTime time.Time) bool) []*fsutils.FileInfo,
	metadataextr metadata.MetadataExtractor) *Syncronizer {

	return &Syncronizer{fileStore: fileStore, metadataStore: metadataStore, fileReader: fr, photosFinder: photosFinder, metadataextr: metadataextr}
}

func (s *Syncronizer) Execute(rootPath string, done <-chan interface{}) {
	isChangedOrNew := func(id string, modTime time.Time) bool {
		existing, _ := s.metadataStore.GetByPath(id)
		return (len(existing) == 0 || existing[0].ModTime != modTime)
	}

	newAndChangedPhotos := s.photosFinder(rootPath, isChangedOrNew)
	for _, p := range newAndChangedPhotos {
		s.metadataStore.Delete(p.Path)
	}

	metadataStream := s.metadataextr.Extract(fsutils.GroupByDir(newAndChangedPhotos), s.fileReader)
	photosStream := filestore.UploadPhotos(metadataStream, s.fileReader, s.fileStore)

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

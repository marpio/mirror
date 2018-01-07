package syncronizer

import (
	"log"
	"time"

	"github.com/marpio/img-store/file"
	"github.com/marpio/img-store/filestore"
	"github.com/marpio/img-store/fileupload"
	"github.com/marpio/img-store/metadata"
	"github.com/marpio/img-store/metadatastore"
)

type Syncronizer struct {
	fileStore     filestore.Service
	metadataStore metadatastore.Service
	fileReader    func(string) (file.File, error)
	photosFinder  func(string, func(id string, modTime time.Time) bool) []*file.FileInfo
	metadataextr  metadata.MetadataExtractor
}

func New(fileStore filestore.Service,
	metadataStore metadatastore.Service,
	fr func(string) (file.File, error),
	photosFinder func(string, func(id string, modTime time.Time) bool) []*file.FileInfo,
	metadataextr metadata.MetadataExtractor) *Syncronizer {

	return &Syncronizer{fileStore: fileStore, metadataStore: metadataStore, fileReader: fr, photosFinder: photosFinder, metadataextr: metadataextr}
}

func (s *Syncronizer) Execute(rootPath string, done <-chan interface{}) {
	isUnchanged := func(id string, modTime time.Time) bool {
		existing, _ := s.metadataStore.GetByPath(id)
		return (len(existing) == 1 && existing[0].ModTime == modTime)
	}

	newOrChanged := s.photosFinder(rootPath, isUnchanged)
	for _, p := range newOrChanged {
		s.metadataStore.Delete(p.Path)
	}

	metadataStream := s.metadataextr.Extract(file.GroupByDir(newOrChanged), s.fileReader)
	photosStream := fileupload.UploadFiles(metadataStream, s.fileReader, s.fileStore)

	for {
		select {
		case p, more := <-photosStream:
			if more {
				s.metadataStore.Add(p)
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

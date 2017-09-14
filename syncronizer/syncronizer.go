package syncronizer

import (
	"bytes"
	"io"
	"log"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/marpio/img-store/file"
	"github.com/marpio/img-store/filestore"
	"github.com/marpio/img-store/metadatastore"
	"github.com/marpio/img-store/photo"
)

const concurrentFiles = 500

type Syncronizer struct {
	fileStore        filestore.FileStore
	metadataStore    metadatastore.DataStore
	fileReader       func(string) (file.File, error)
	photosFinder     func(string, func(id string, modTime time.Time) bool) ([]*file.FileInfo, map[string]*file.FileInfo)
	extractCreatedAt func(imgPath string, path string, r file.File, dirCreatedAt time.Time) time.Time
	extractThumbnail func(r io.ReadSeeker) ([]byte, error)
}

func NewSyncronizer(fileStore filestore.FileStore,
	metadataStore metadatastore.DataStore,
	fr func(string) (file.File, error),
	photosFinder func(string, func(id string, modTime time.Time) bool) ([]*file.FileInfo, map[string]*file.FileInfo),
	extractCreatedAt func(dir string, path string, r file.File, dirCreatedAt time.Time) time.Time,
	extractThumbnail func(r io.ReadSeeker) ([]byte, error)) *Syncronizer {
	return &Syncronizer{fileStore: fileStore, metadataStore: metadataStore, fileReader: fr, photosFinder: photosFinder, extractCreatedAt: extractCreatedAt, extractThumbnail: extractThumbnail}
}

func (s *Syncronizer) Sync(rootPath string) error {
	log.Print("Syncing...")
	newOrChanged, unchanged := s.photosFinder(rootPath, isPhotoUnchangedFn(s.metadataStore))

	s.syncDatastore(unchanged)

	metadataStream := s.extractMetadata(groupByDir(newOrChanged))
	uploaded := s.uploadPhotos(metadataStream)
	for u := range uploaded {
		s.metadataStore.Add(u)
	}
	if err := s.metadataStore.Commit(); err != nil {
		log.Printf("Error commiting DB %v", err)
	}
	log.Println("Sync compleated.")
	return nil
}

func (s *Syncronizer) extractMetadata(pathsGroupedByDir map[string][]*file.FileInfo) <-chan *photo.FileWithMetadata {
	metadataStream := make(chan *photo.FileWithMetadata)
	go func() {
		defer close(metadataStream)
		var wg sync.WaitGroup
		wg.Add(len(pathsGroupedByDir))
		for dir, paths := range pathsGroupedByDir {
			go func(directory string, ps []*file.FileInfo) {
				defer wg.Done()
				s.extractMetadataForDir(directory, ps, metadataStream)
			}(dir, paths)
		}
		wg.Wait()
	}()
	return metadataStream
}

func (s *Syncronizer) extractMetadataForDir(dir string, photos []*file.FileInfo, metadataStream chan<- *photo.FileWithMetadata) {
	dirCreatedAt := time.Time{}
	for _, p := range photos {
		f, err := s.fileReader(p.Path)
		if err != nil {
			log.Printf("Error opening file %v - err msg: %v", p.Path, err)
			continue
		}
		defer f.Close()

		createdAt := s.extractCreatedAt(dir, p.Path, f, dirCreatedAt)
		createdAtMonth := time.Date(createdAt.Year(), createdAt.Month(), 1, 0, 0, 0, 0, time.UTC)
		f.Seek(0, 0)
		thumb, err := s.extractThumbnail(f)
		if err != nil {
			log.Printf("Can't extract thumbnail: %v", err)
			continue
		}
		thumbnailName := generateUniqueFileName("thumb", p.Path, createdAt)
		imgName := generateUniqueFileName("orig", p.Path, createdAt)
		res := &photo.FileWithMetadata{FileInfo: p, Thumbnail: thumb, Metadata: &photo.Metadata{Name: imgName, ThumbnailName: thumbnailName, CreatedAt: createdAt, CreatedAtMonth: createdAtMonth}}
		dirCreatedAt = createdAt
		metadataStream <- res
	}
}

func (s *Syncronizer) syncDatastore(unchanged map[string]*file.FileInfo) error {
	syncedPhotos, err := s.metadataStore.GetAll()
	if err != nil {
		return err
	}
	for _, elem := range syncedPhotos {
		if _, exists := unchanged[elem.PathHash]; !exists {
			s.metadataStore.Delete(elem.PathHash)
		}
	}
	return nil
}

func (s *Syncronizer) UploadMetadataStore(imgDBpath string) error {
	dbFileReader, err := s.fileReader(imgDBpath)
	if err != nil {
		return err
	}
	if err := s.fileStore.UploadEncrypted(imgDBpath, dbFileReader); err != nil {
		return err
	}
	return nil
}

func isPhotoUnchangedFn(store metadatastore.DataStoreReader) func(id string, modTime time.Time) bool {
	return func(id string, modTime time.Time) bool {
		existing, _ := store.GetByID(id)
		return (len(existing) == 1 && existing[0].ModTime == modTime)
	}
}

func groupByDir(photos []*file.FileInfo) map[string][]*file.FileInfo {
	photosGroupedByDir := make(map[string][]*file.FileInfo)
	for _, p := range photos {
		dir := filepath.Dir(p.Path)
		if v, ok := photosGroupedByDir[dir]; ok {
			v = append(v, p)
			photosGroupedByDir[dir] = v
		} else {
			ps := make([]*file.FileInfo, 0)
			ps = append(ps, p)
			photosGroupedByDir[dir] = ps
		}
	}
	return photosGroupedByDir
}

func (s *Syncronizer) uploadPhotos(metadataStream <-chan *photo.FileWithMetadata) <-chan *photo.Photo {
	uploadedPhotosStream := make(chan *photo.Photo)
	go func() {
		var wg sync.WaitGroup
		defer close(uploadedPhotosStream)
		for metaData := range metadataStream {
			wg.Add(1)
			go func(m *photo.FileWithMetadata) {
				defer wg.Done()
				p, err := s.uploadPhoto(m)
				if err != nil {
					log.Printf("Error uploading file: %v - err: %v", m.Path, err)
					return
				}
				uploadedPhotosStream <- p
			}(metaData)
		}
		wg.Wait()
	}()
	return uploadedPhotosStream
}

func (s *Syncronizer) uploadPhoto(img *photo.FileWithMetadata) (*photo.Photo, error) {
	f, err := s.fileReader(img.Path)
	if err != nil {
		log.Printf("Error opening file %v - err msg: %v", img.Path, err)
		return nil, err
	}
	defer f.Close()

	if err := s.fileStore.UploadEncrypted(img.ThumbnailName, bytes.NewReader(img.Thumbnail)); err != nil {
		log.Printf("Error uploading : %v - img: %v", err, img.Path)
		return nil, err
	}

	if err := s.fileStore.UploadEncrypted(img.Name, f); err != nil {
		log.Printf("Error uploading: %v - img: %v", err, img.Path)
		return nil, err
	}

	p := &photo.Photo{FileInfo: img.FileInfo, Metadata: img.Metadata}

	return p, nil
}

//func (s *Syncronizer) deleteOutOfSyncMetadata(existingFileIDs map[string]bool) {
//	metadata, err := s.metadataStore.GetAll()
//	if err != nil {
//		log.Printf("Error geting metadata: %v", err.Error())
//	}
//	for _, img := range metadata {
//		if _, exists := existingFileIDs[img.PathHash]; !exists {
//			s.metadataStore.Delete(img.ImgID)
//			s.fileStore.Delete(img.ImgName)
//			s.fileStore.Delete(img.ThumbnailName)
//		}
//	}
//}

func generateUniqueFileName(prefix string, imgPath string, imgCreatedAt time.Time) string {
	nano := strconv.FormatInt(imgCreatedAt.UnixNano(), 10)
	imgFileName := prefix + "_" + nano + "_" + path.Base(imgPath)
	return imgFileName
}

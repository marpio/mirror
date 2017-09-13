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
	extractCreatedAt func(imgPath string, r file.File, dirCreatedAt time.Time) time.Time
	extractThumbnail func(r io.ReadSeeker) ([]byte, error)
}

func NewSyncronizer(fileStore filestore.FileStore,
	metadataStore metadatastore.DataStore,
	fr func(string) (file.File, error),
	photosFinder func(string, func(id string, modTime time.Time) bool) ([]*file.FileInfo, map[string]*file.FileInfo),
	extractCreatedAt func(dir string, r file.File, dirCreatedAt time.Time) time.Time,
	extractThumbnail func(r io.ReadSeeker) ([]byte, error)) *Syncronizer {
	return &Syncronizer{fileStore: fileStore, metadataStore: metadataStore, fileReader: fr, photosFinder: photosFinder, extractCreatedAt: extractCreatedAt, extractThumbnail: extractThumbnail}
}

func (s *Syncronizer) Sync(rootPath string) error {
	log.Print("Syncing...")
	newOrChanged, unchanged := s.photosFinder(rootPath, isPhotoUnchangedFn(s.metadataStore))

	syncedPhotos, err := s.metadataStore.GetAll()
	if err != nil {
		return err
	}
	for _, elem := range syncedPhotos {
		if _, exists := unchanged[elem.PathHash]; !exists {
			s.metadataStore.Delete(elem.PathHash)
		}
	}

	metadataStream := make(chan []*photo.FileWithMetadata)
	s.extractMetadata(groupByDir(newOrChanged), metadataStream)

	newOrChangedWithMetadata := make([]*photo.FileWithMetadata, len(newOrChanged))
	for dirFilesMetadata := range metadataStream {
		newOrChangedWithMetadata = append(newOrChangedWithMetadata, dirFilesMetadata...)
	}
	photos := s.syncPhotos(newOrChangedWithMetadata)

	s.metadataStore.Save(photos)
	log.Println("Sync compleated.")
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

func (s *Syncronizer) extractMetadata(pathsGroupedByDir map[string][]*file.FileInfo, metadataStream chan []*photo.FileWithMetadata) {
	var wg sync.WaitGroup
	wg.Add(len(pathsGroupedByDir))
	for dir, paths := range pathsGroupedByDir {
		go func(directory string, ps []*file.FileInfo) {
			defer wg.Done()
			metadata := s.extractMetadataForDir(directory, ps)
			metadataStream <- metadata
		}(dir, paths)
	}
	wg.Wait()
	close(metadataStream)
}

func (s *Syncronizer) extractMetadataForDir(dir string, photos []*file.FileInfo) []*photo.FileWithMetadata {
	dirCreatedAt := time.Time{}
	metadata := make([]*photo.FileWithMetadata, 0)
	for _, p := range photos {
		f, err := s.fileReader(p.Path)
		if err != nil {
			log.Printf("Error opening file %v - err msg: %v", p.Path, err)
			continue
		}
		defer f.Close()

		createdAt := s.extractCreatedAt(dir, f, dirCreatedAt)
		thumb, err := s.extractThumbnail(f)
		if err != nil {
			continue
		}
		res := &photo.FileWithMetadata{FileInfo: p, Metadata: &photo.Metadata{CreatedAt: createdAt, Thumbnail: thumb}}
		metadata = append(metadata, res)
		dirCreatedAt = createdAt
	}
	return metadata
}

func (s *Syncronizer) syncPhotos(photos []*photo.FileWithMetadata) []*photo.Photo {
	length := len(photos)

	result := make([]*photo.Photo, length)

	for i := 0; i < length; i = i + concurrentFiles {
		end := i + concurrentFiles
		if end > length {
			end = length
		}

		var wg sync.WaitGroup
		wg.Add(len(photos[i:end]))
		for n, img := range photos[i:end] {
			index := i + n
			go func(p *photo.FileWithMetadata, ix int) {
				defer wg.Done()
				err := s.syncPhoto(p, ix, result)
				if err != nil {
					log.Printf("Error syncing photo: %v", p.Path)
				}
			}(img, index)
		}
		wg.Wait()
	}
	return result
}

func (s *Syncronizer) syncPhoto(img *photo.FileWithMetadata, currentIndex int, result []*photo.Photo) error {
	f, err := s.fileReader(img.Path)
	if err != nil {
		log.Printf("Error opening file %v - err msg: %v", img.Path, err)
		return err
	}
	defer f.Close()
	thumbnailName := generateUniqueFileName("thumb", img.Path, img.CreatedAt)
	imgName := generateUniqueFileName("orig", img.Path, img.CreatedAt)

	if err := s.fileStore.UploadEncrypted(thumbnailName, bytes.NewReader(img.Thumbnail)); err != nil {
		log.Printf("Error uploading : %v - img: %v", err, img.Path)
		return err
	}

	if err := s.fileStore.UploadEncrypted(imgName, f); err != nil {
		log.Printf("Error uploading: %v - img: %v", err, img.Path)
		return err
	}

	createdAtMonth := time.Date(img.CreatedAt.Year(), img.CreatedAt.Month(), 1, 0, 0, 0, 0, time.UTC)
	p := &photo.Photo{FileWithMetadata: img, ThumbnailName: thumbnailName, Name: imgName, CreatedAtMonth: createdAtMonth}
	result[currentIndex] = p

	return nil
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

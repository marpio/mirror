package syncronizer

import (
	"bytes"
	"io"
	"log"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/marpio/img-store/crypto"
	"github.com/marpio/img-store/file"
	"github.com/marpio/img-store/filestore"
	"github.com/marpio/img-store/metadatastore"
)

const concurrentFiles = 20

type imgFileDto struct {
	ImgID     string
	Path      string
	CreatedAt time.Time
}

type Syncronizer struct {
	fileStore        filestore.FileStore
	metadataStore    metadatastore.DataStore
	fileReader       func(string) (file.File, error)
	imgsFinder       func(string) []string
	extractCreatedAt func(imgPath string, r file.File) (time.Time, error)
	extractThumbnail func(imgPath string, r io.ReadSeeker) ([]byte, error)
}

func NewSyncronizer(fileStore filestore.FileStore,
	metadataStore metadatastore.DataStore,
	fr func(string) (file.File, error),
	imgsFinder func(string) []string,
	extractCreatedAt func(imgPath string, r file.File) (time.Time, error),
	extractThumbnail func(imgPath string, r io.ReadSeeker) ([]byte, error)) *Syncronizer {
	return &Syncronizer{fileStore: fileStore, metadataStore: metadataStore, fileReader: fr, imgsFinder: imgsFinder, extractCreatedAt: extractCreatedAt, extractThumbnail: extractThumbnail}
}

func (s *Syncronizer) Sync(rootPath string) {
	imgsPaths := s.imgsFinder(rootPath)
	imgFiles := s.getImagesMetadata(imgsPaths)

	log.Printf("Found %v images.", len(imgFiles))
	s.syncImages(imgFiles)
	log.Println("Sync compleated.")
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

func (s *Syncronizer) getImagesMetadata(imgsPaths []string) []*imgFileDto {
	var isImgToOldPredicate = func(createdAt time.Time) bool {
		return createdAt.Year() < time.Now().Add(-1*time.Hour*24*365*10).Year()
	}
	imgFiles := make([]*imgFileDto, 0)
	for _, path := range imgsPaths {
		f, err := s.fileReader(path)
		if err != nil {
			log.Printf("Error opening file %v - err msg: %v", path, err)
			continue
		}
		defer f.Close()
		imgCreatedAt, err := s.extractCreatedAt(path, f)
		if err != nil {
			log.Printf("Error obtaining exif-metadata from file: %v - err msg: %v", path, err)
			continue
		}
		if !isImgToOldPredicate(imgCreatedAt) {
			imgID, err := crypto.CalculateHash(bytes.NewReader([]byte(path)))
			if err != nil {
				log.Printf("Error calculating path's hash - path: %v - err msg: %v", path, err)
				continue
			}
			imgFiles = append(imgFiles, &imgFileDto{ImgID: imgID, Path: path, CreatedAt: imgCreatedAt})
		}
	}
	return imgFiles
}

func (s *Syncronizer) syncImages(images []*imgFileDto) {
	existingFileIDs := make(map[string]bool)
	length := len(images)
	for i := 0; i < length; i = i + concurrentFiles {
		end := i + concurrentFiles
		if end > length {
			end = length
		}

		var wg sync.WaitGroup
		for _, img := range images[i:end] {
			existingFileIDs[img.ImgID] = false
			wg.Add(1)
			go func(image *imgFileDto) {
				defer wg.Done()
				err := s.syncImage(image)
				if err != nil {
					log.Printf("Error syncing image: %v", image.Path)
				}
			}(img)
		}
		wg.Wait()
	}
	s.deleteOutOfSyncMetadata(existingFileIDs)
}

func (s *Syncronizer) syncImage(img *imgFileDto) error {
	imgID := img.ImgID
	f, err := s.fileReader(img.Path)
	if err != nil {
		log.Printf("Error opening file %v - err msg: %v", img.Path, err)
		return err
	}
	defer f.Close()
	imgHash, err := crypto.CalculateHash(f)
	if err != nil {
		log.Printf("Error calculating img hash: %v", err)
		return err
	}
	f.Seek(0, 0)
	imgMetadata, err := s.metadataStore.GetByID(imgID)
	if err != nil {
		return err
	}
	imgMetadataExist := len(imgMetadata) > 0
	if imgMetadataExist && imgMetadata[0].ImgHash == imgHash {
		return nil
	}
	if err != nil {
		log.Printf("Error reading file %v - err msg: %v", img.Path, err)
		return err
	}
	thumbnail, err := s.extractThumbnail(img.Path, f)
	if err != nil {
		log.Printf("Error obtaining exif-metadata from file: %v - err msg: %v", img.Path, err)
		return err
	}
	f.Seek(0, 0)
	b2thumbnailName := generateUniqueFileName("thumb", img.Path, img.CreatedAt)
	b2imgName := generateUniqueFileName("orig", img.Path, img.CreatedAt)

	if err := s.fileStore.UploadEncrypted(b2thumbnailName, bytes.NewReader(thumbnail)); err != nil {
		log.Printf("Error uploading to b2: %v - img: %v", err, img.Path)
		return err
	}

	if err := s.fileStore.UploadEncrypted(b2imgName, f); err != nil {
		log.Printf("Error uploading to b2: %v - img: %v", err, img.Path)
		return err
	}
	if imgMetadataExist && imgMetadata[0].ImgHash != imgHash {
		if err := s.metadataStore.Delete(imgID); err != nil {
			return err
		}
	}
	createdAtMonth := time.Date(img.CreatedAt.Year(), img.CreatedAt.Month(), 1, 0, 0, 0, 0, time.UTC)
	imgEntity := &metadatastore.Image{ImgID: imgID, CreatedAt: img.CreatedAt, CreatedAtMonth: createdAtMonth, ImgHash: imgHash, ImgName: b2imgName, ThumbnailName: b2thumbnailName}
	err = s.metadataStore.Insert(imgEntity)
	if err != nil {
		return err
	}

	return nil
}

func (s *Syncronizer) deleteOutOfSyncMetadata(existingFileIDs map[string]bool) {
	metadata, err := s.metadataStore.GetAll()
	if err != nil {
		log.Printf("Error geting metadata: %v", err.Error())
	}
	for _, img := range metadata {
		if _, exists := existingFileIDs[img.ImgID]; !exists {
			s.metadataStore.Delete(img.ImgID)
			s.fileStore.Delete(img.ImgName)
			s.fileStore.Delete(img.ThumbnailName)
		}
	}
}

func generateUniqueFileName(prefix string, imgPath string, imgCreatedAt time.Time) string {
	nano := strconv.FormatInt(imgCreatedAt.UnixNano(), 10)
	imgFileName := prefix + "_" + nano + "_" + path.Base(imgPath)
	return imgFileName
}

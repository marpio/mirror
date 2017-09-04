package syncronizer

import (
	"bytes"
	"context"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/marpio/img-store/crypto"
	"github.com/marpio/img-store/file"
	"github.com/marpio/img-store/filestore"
	"github.com/marpio/img-store/metadata"
	"github.com/marpio/img-store/metadatastore"
)

const concurrentFiles = 20

type imgFileDto struct {
	ImgID     string
	Path      string
	CreatedAt time.Time
}

type Syncronizer struct {
	ctx           context.Context
	fileStore     filestore.FileStore
	metadataStore metadatastore.DataStore
	fileReader    func(string) (file.File, error)
}

func NewSyncronizer(ctx context.Context, fileStore filestore.FileStore, metadataStore metadatastore.DataStore, fr func(string) (file.File, error)) *Syncronizer {
	return &Syncronizer{ctx: ctx, fileStore: fileStore, metadataStore: metadataStore, fileReader: fr}
}

func (s *Syncronizer) Sync(rootPath string) {
	imgs := s.getImages(rootPath)
	log.Printf("Found %v images.", len(imgs))
	s.syncImages(imgs)
	log.Println("Sync compleated.")
}

func (s *Syncronizer) getImages(rootPath string) []*imgFileDto {
	imgFiles := make([]*imgFileDto, 0)

	var isJpegPredicate = func(path string, f os.FileInfo) bool {
		return !f.IsDir() && (strings.HasSuffix(strings.ToLower(f.Name()), ".jpg") || strings.HasSuffix(strings.ToLower(f.Name()), ".jpeg"))
	}
	var isImgToOldPredicate = func(createdAt time.Time) bool {
		return createdAt.Year() < time.Now().Add(-1*time.Hour*24*365*10).Year()
	}
	err := filepath.Walk(rootPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error while walking the directory structure: %v", err.Error())
		}
		if isJpegPredicate(path, fi) {
			f, err := s.fileReader(path)
			if err != nil {
				log.Printf("Error opening file %v - err msg: %v", path, err)
				return err
			}
			defer f.Close()
			imgCreatedAt, err := metadata.ExtractCreatedAt(path, f)
			if err != nil {
				log.Printf("Error obtaining exif-metadata from file: %v - err msg: %v", path, err)
				return err
			}
			if !isImgToOldPredicate(imgCreatedAt) {
				imgID, err := crypto.CalculateHash(bytes.NewReader([]byte(path)))
				if err != nil {
					log.Printf("Error calculating path's hash - path: %v - err msg: %v", path, err)
					return err
				}
				imgFiles = append(imgFiles, &imgFileDto{ImgID: imgID, Path: path, CreatedAt: imgCreatedAt})
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf(err.Error())
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
				err := s.syncImageStreamed(image)
				if err != nil {
					log.Printf("Error syncing image: %v", image.Path)
				}
			}(img)
		}
		wg.Wait()
	}
	s.deleteOutOfSyncMetadata(existingFileIDs)
}

func (s *Syncronizer) syncImageStreamed(img *imgFileDto) error {
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
	thumbnail, err := metadata.ExtractThumbnail(img.Path, f)
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

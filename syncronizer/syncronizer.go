package syncronizer

import (
	"bytes"
	"context"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/marpio/img-store/crypto"
	"github.com/marpio/img-store/metadata"
)

type Datastore interface {
	GetAll() ([]*metadata.Image, error)
	GetByID(imgID string) ([]*metadata.Image, error)
	Insert(imgEntity *metadata.Image) error
	DeleteIfImgChanged(imgID string, imgContentHash string) error
	Delete(imgID string) error
	Exists(imgID string) (*metadata.Image, error)
}

type FileStore interface {
	Upload(imgFileName string, payload []byte) error
	//download(encryptionKey, src, dst string)
}

type imgFileDto struct {
	ImgID     string
	Path      string
	CreatedAt time.Time
}

type Syncronizer struct {
	ctx           context.Context
	fileStore     FileStore
	metadataStore Datastore
}

func NewSyncronizer(ctx context.Context, fileStore FileStore, metadataStore Datastore) *Syncronizer {
	return &Syncronizer{ctx: ctx, fileStore: fileStore, metadataStore: metadataStore}
}

func (s *Syncronizer) Sync(rootPath string, encryptionKey string) {
	imgs := getImages(rootPath)
	log.Printf("Found %v images.", len(imgs))
	syncImages(s.ctx, imgs, s.fileStore, s.metadataStore, encryptionKey)
	log.Println("Sync compleated.")
}

func getImages(rootPath string) []*imgFileDto {
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
			f, err := os.Open(path)
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

func syncImages(ctx context.Context, images []*imgFileDto, fileStore FileStore, metadataStore Datastore, encryptionKey string) {
	existingFileIDs := make(map[string]bool)
	chunkSize := 10
	length := len(images)
	for i := 0; i < length; i = i + chunkSize {
		end := i + chunkSize
		if end > length {
			end = length
		}

		var wg sync.WaitGroup
		for _, img := range images[i:end] {
			existingFileIDs[img.ImgID] = false
			wg.Add(1)
			go func(image *imgFileDto) {
				defer wg.Done()
				err := syncImage(ctx, image, fileStore, metadataStore, encryptionKey)
				if err != nil {
					log.Printf("Error syncing image: %v", image.Path)
				}
			}(img)
		}
		wg.Wait()
	}
	deleteOutOfSyncMetadata(metadataStore, existingFileIDs)
}

func syncImage(ctx context.Context, img *imgFileDto, fileStore FileStore, metadataStore Datastore, encryptionKey string) error {
	imgID := img.ImgID
	f, err := os.Open(img.Path)
	if err != nil {
		log.Printf("Error opening file %v - err msg: %v", img.Path, err)
		return err
	}
	defer f.Close()
	imgContent, err := ioutil.ReadAll(f)
	if err != nil {
		log.Printf("Error reading file %v - err msg: %v", img.Path, err)
		return err
	}
	imgContentReader := bytes.NewReader(imgContent)
	thumbnail, err := metadata.ExtractThumbnail(img.Path, imgContentReader)
	if err != nil {
		log.Printf("Error obtaining exif-metadata from file: %v - err msg: %v", img.Path, err)
		return err
	}

	imgHash, err := crypto.CalculateHash(imgContentReader)
	if err != nil {
		log.Printf("Error calculating img hash: %v", err)
		return err
	}
	encryptedThumbnail, err := crypto.Encrypt(encryptionKey, thumbnail)
	if err != nil {
		log.Printf("Error encrypting thumbnail: %v - err msg: %v", img.Path, err)
		return err
	}
	encryptedImg, err := crypto.Encrypt(encryptionKey, imgContent)
	if err != nil {
		log.Printf("Error encrypting image: %v - err msg: %v", img.Path, err)
		return err
	}

	if err := metadataStore.DeleteIfImgChanged(imgID, imgHash); err != nil {
		return err
	}

	b2thumbnailName := generateUniqueFileName("thumb", img.Path, img.CreatedAt)
	b2imgName := generateUniqueFileName("orig", img.Path, img.CreatedAt)

	if err := fileStore.Upload(b2thumbnailName, encryptedThumbnail); err != nil {
		log.Printf("Error uploading to b2: %v - img: %v", err, img.Path)
		return err
	}
	if err := fileStore.Upload(b2imgName, encryptedImg); err != nil {
		log.Printf("Error uploading to b2: %v - img: %v", err, img.Path)
		return err
	}
	imgEntity := &metadata.Image{ImgID: imgID, CreatedAt: img.CreatedAt, ImgHash: imgHash, B2ImgName: b2imgName, B2ThumbnailName: b2thumbnailName}
	err = metadataStore.Insert(imgEntity)
	if err != nil {
		return err
	}

	return nil
}

func deleteOutOfSyncMetadata(metadataStore Datastore, existingFileIDs map[string]bool) {
	metadata, err := metadataStore.GetAll()
	if err != nil {
		log.Printf("Error geting metadata: %v", err.Error())
	}
	for _, img := range metadata {
		if _, exists := existingFileIDs[img.ImgID]; !exists {
			metadataStore.Delete(img.ImgID)
		}
	}
}

func generateUniqueFileName(prefix string, imgPath string, imgCreatedAt time.Time) string {
	nano := strconv.FormatInt(imgCreatedAt.UnixNano(), 10)
	imgFileName := prefix + "_" + nano + "_" + path.Base(imgPath)
	return imgFileName
}

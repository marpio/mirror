package syncronizer

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/marpio/img-store/crypto"
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
	encryptionKey string
}

func NewSyncronizer(ctx context.Context, fileStore filestore.FileStore, metadataStore metadatastore.DataStore, encryptKey string) *Syncronizer {
	return &Syncronizer{ctx: ctx, fileStore: fileStore, metadataStore: metadataStore, encryptionKey: encryptKey}
}

func (s *Syncronizer) Sync(rootPath string) {
	imgs := getImages(rootPath)
	log.Printf("Found %v images.", len(imgs))
	syncImages(s.ctx, imgs, s.fileStore, s.metadataStore, s.encryptionKey)
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

func syncImages(ctx context.Context, images []*imgFileDto, fileStore filestore.FileStore, metadataStore metadatastore.DataStore, encryptionKey string) {
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
				err := syncImageStreamed(ctx, image, fileStore, metadataStore, encryptionKey)
				if err != nil {
					log.Printf("Error syncing image: %v", image.Path)
				}
			}(img)
		}
		wg.Wait()
	}
	deleteOutOfSyncMetadata(metadataStore, fileStore, existingFileIDs)
}

func syncImageStreamed(ctx context.Context, img *imgFileDto, fileStore filestore.FileStore, metadataStore metadatastore.DataStore, encryptionKey string) error {
	imgID := img.ImgID
	f, err := os.Open(img.Path)
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
	imgMetadata, err := metadataStore.GetByID(imgID)
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

	encryptedThumbnailReader, thumbPipeWriter := io.Pipe()
	go func() {
		// close the writer, so the reader knows there's no more data
		defer thumbPipeWriter.Close()
		err = crypto.Encrypt(thumbPipeWriter, encryptionKey, bytes.NewReader(thumbnail))
		if err != nil {
			log.Printf("Error encrypting thumbnail: %v - err msg: %v", img.Path, err)
		}
	}()
	if err := fileStore.Upload(b2thumbnailName, encryptedThumbnailReader); err != nil {
		log.Printf("Error uploading to b2: %v - img: %v", err, img.Path)
		return err
	}
	encryptedImgReader, imgPipeWriter := io.Pipe()
	go func() {
		// close the writer, so the reader knows there's no more data
		defer imgPipeWriter.Close()
		err = crypto.Encrypt(imgPipeWriter, encryptionKey, f)
		if err != nil {
			log.Printf("Error encrypting image: %v - err msg: %v", img.Path, err)
		}
	}()
	if err := fileStore.Upload(b2imgName, encryptedImgReader); err != nil {
		log.Printf("Error uploading to b2: %v - img: %v", err, img.Path)
		return err
	}
	if imgMetadataExist && imgMetadata[0].ImgHash != imgHash {
		if err := metadataStore.Delete(imgID); err != nil {
			return err
		}
	}
	createdAtMonth := time.Date(img.CreatedAt.Year(), img.CreatedAt.Month(), 1, 0, 0, 0, 0, img.CreatedAt.Location())
	imgEntity := &metadatastore.Image{ImgID: imgID, CreatedAt: img.CreatedAt, CreatedAtMonth: createdAtMonth, ImgHash: imgHash, ImgName: b2imgName, ThumbnailName: b2thumbnailName}
	err = metadataStore.Insert(imgEntity)
	if err != nil {
		return err
	}

	return nil
}

func deleteOutOfSyncMetadata(metadataStore metadatastore.DataStore, fileStore filestore.FileStore, existingFileIDs map[string]bool) {
	metadata, err := metadataStore.GetAll()
	if err != nil {
		log.Printf("Error geting metadata: %v", err.Error())
	}
	for _, img := range metadata {
		if _, exists := existingFileIDs[img.ImgID]; !exists {
			metadataStore.Delete(img.ImgID)
			fileStore.Delete(img.ImgName)
			fileStore.Delete(img.ThumbnailName)
		}
	}
}

func generateUniqueFileName(prefix string, imgPath string, imgCreatedAt time.Time) string {
	nano := strconv.FormatInt(imgCreatedAt.UnixNano(), 10)
	imgFileName := prefix + "_" + nano + "_" + path.Base(imgPath)
	return imgFileName
}

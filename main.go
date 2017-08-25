package main

import (
	"bytes"
	"context"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/marpio/img-store/crypto"
	"github.com/marpio/img-store/filestore"
	"github.com/marpio/img-store/metadata"
	"github.com/marpio/img-store/metadatastore"
	_ "github.com/mattn/go-sqlite3"
)

type imgFileDto struct {
	Path      string
	CreatedAt time.Time
}

func main() {
	f := initLog()
	defer f.Close()

	if err := godotenv.Load("settings.env"); err != nil {
		log.Fatal("Error loading .env file")
	}
	encryptionKey := os.Getenv("ENCR_KEY")
	b2id := os.Getenv("B2_ACCOUNT_ID")
	b2key := os.Getenv("B2_ACCOUNT_KEY")
	bucketName := os.Getenv("B2_BUCKET_NAME")

	ctx := context.Background()

	imgStore := filestore.NewFileStore(ctx, b2id, b2key, bucketName)
	metadataStore := metadatastore.NewMetadataStore("img.db", `
	CREATE TABLE IF NOT EXISTS img (
		img_id text PRIMARY KEY,
		created_at DATETIME,
		img_hash text NOT NULL,
		b2_img_name text NOT NULL,
		b2_thumbnail_name text NOT NULL);`)

	imgRootDir := flag.String("syncdir", "", "Abs path to the directory containing pictures")
	flag.Parse()

	if *imgRootDir != "" {
		imgs := getImages(*imgRootDir)
		log.Printf("Found %v images.", len(imgs))
		localState := syncLocalImagesWithBackblaze(ctx, imgs, imgStore, metadataStore, encryptionKey)
		log.Println("Sync compleated.")
		for _, imgID := range localState {
			log.Printf("ImgID: %v", imgID)
		}
	}
}

func initLog() io.Closer {
	f, err := os.OpenFile("output.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	log.SetOutput(f)
	return f
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
				imgFiles = append(imgFiles, &imgFileDto{Path: path, CreatedAt: imgCreatedAt})
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf(err.Error())
	}
	return imgFiles
}

func syncLocalImagesWithBackblaze(ctx context.Context, images []*imgFileDto, imgStore filestore.FileStore, metadataStore metadatastore.Datastore, encryptionKey string) []string {
	imgIds := make([]string, 0)

	for _, img := range images {
		imgID, err := syncImage(ctx, img, imgStore, metadataStore, encryptionKey)
		if err != nil {
			continue
		}
		imgIds = append(imgIds, imgID)
	}
	return imgIds
}

func syncImage(ctx context.Context, img *imgFileDto, imgStore filestore.FileStore, metadataStore metadatastore.Datastore, encryptionKey string) (string, error) {
	imgID, err := crypto.CalculateHash(bytes.NewReader([]byte(img.Path)))
	if err != nil {
		log.Printf("Error calculating path's hash - path: %v - err msg: %v", img.Path, err)
		return "", err
	}
	f, err := os.Open(img.Path)
	if err != nil {
		log.Printf("Error opening file %v - err msg: %v", img.Path, err)
		return "", err
	}
	defer f.Close()
	imgContent, err := ioutil.ReadAll(f)
	if err != nil {
		log.Printf("Error reading file %v - err msg: %v", img.Path, err)
		return "", err
	}
	imgContentReader := bytes.NewReader(imgContent)
	thumbnail, err := metadata.ExtractThumbnail(img.Path, imgContentReader)
	if err != nil {
		log.Printf("Error obtaining exif-metadata from file: %v - err msg: %v", img.Path, err)
		return "", err
	}

	imgHash, err := crypto.CalculateHash(imgContentReader)
	if err != nil {
		log.Printf("Error calculating img hash: %v", err)
		return "", err
	}
	encryptedThumbnail, err := crypto.Encrypt(encryptionKey, thumbnail)
	if err != nil {
		log.Printf("Error encrypting thumbnail: %v - err msg: %v", img.Path, err)
		return "", err
	}
	log.Printf("Thumbnail size: %v", len(encryptedThumbnail))
	encryptedImg, err := crypto.Encrypt(encryptionKey, imgContent)
	if err != nil {
		log.Printf("Error encrypting image: %v - err msg: %v", img.Path, err)
		return "", err
	}

	if err := metadataStore.Update(imgID, imgHash); err != nil {
		return "", err
	}

	b2thumbnailName := generateUniqueFileName("thumb", img.Path, img.CreatedAt)
	b2imgName := generateUniqueFileName("orig", img.Path, img.CreatedAt)
	imgEntity := &metadatastore.Image{ImgID: imgID, CreatedAt: img.CreatedAt, ImgHash: imgHash, B2ImgName: b2imgName, B2ThumbnailName: b2thumbnailName}
	err = metadataStore.Insert(imgEntity, func() error {
		log.Printf("Starting upload to b2 - img %v", img.Path)

		if err := imgStore.Upload(b2thumbnailName, encryptedThumbnail); err != nil {
			log.Printf("Error uploading to b2: %v - img: %v", err, img.Path)
			return err
		}
		if err := imgStore.Upload(b2imgName, encryptedImg); err != nil {
			log.Printf("Error uploading to b2: %v - img: %v", err, img.Path)
			return err
		}
		log.Printf("Upload compleated for img %v", img.Path)
		return nil
	})
	if err != nil {
		return "", err
	}

	return imgID, nil
}

func generateUniqueFileName(prefix string, imgPath string, imgCreatedAt time.Time) string {
	nano := strconv.FormatInt(imgCreatedAt.UnixNano(), 10)
	imgFileName := prefix + "_" + nano + "_" + path.Base(imgPath)
	return imgFileName
}

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"image/jpeg"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/kurin/blazer/b2"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nfnt/resize"
	"github.com/rwcarlsen/goexif/exif"
	"golang.org/x/crypto/nacl/secretbox"
)

var createSchema = `
CREATE TABLE IF NOT EXISTS img (
	img_id text PRIMARY KEY,
	created_at DATETIME,
    img_hash text NOT NULL,
	b2_img_name text NOT NULL,
	b2_thumbnail_name text NOT NULL);`

type image struct {
	ImgID           string    `db:"img_id"`
	CreatedAt       time.Time `db:"created_at"`
	ImgHash         string    `db:"img_hash"`
	B2ImgName       string    `db:"b2_img_name"`
	B2ThumbnailName string    `db:"b2_thumbnail_name"`
}

type fileStore interface {
	uploadToB2(imgFileName string, payload []byte) error
	downloadFile(encryptionKey, src, dst string)
}

type b2Store struct {
	bucket *b2.Bucket
	ctx    context.Context
}

func newb2Store(ctx context.Context, b2id string, b2key string, bucketName string) *b2Store {
	b2Client, err := b2.NewClient(ctx, b2id, b2key)
	if err != nil {
		log.Fatal(err)
	}

	bucket, err := b2Client.Bucket(ctx, bucketName)
	if err != nil {
		log.Fatal(err)
	}
	return &b2Store{bucket: bucket, ctx: ctx}
}

func (b2 *b2Store) downloadFile(encryptionKey, src, dst string) {
	r := b2.bucket.Object(src).NewReader(ctx)
	defer r.Close()

	var b bytes.Buffer
	writer := bufio.NewWriter(&b)
	if _, err := io.Copy(writer, r); err != nil {
		log.Fatal("Booom!!!")
	}
	writer.Flush()
	encryptedData := b.Bytes()
	decryptedData, _ := decrypt(encryptionKey, encryptedData)
	f, err := os.Create(dst)
	if err != nil {
		log.Fatal("Booom!!!")
	}
	//r.ConcurrentDownloads = downloads
	if _, err := io.Copy(f, bytes.NewReader(decryptedData)); err != nil {
		log.Fatal("Booom!!!")
	}
}

func (b2 *b2Store) uploadToB2(imgFileName string, payload []byte) error {
	imgObj := b2.bucket.Object(imgFileName)
	b2Writer := imgObj.NewWriter(b2.ctx)
	reader := bytes.NewReader(payload)
	if _, err := io.Copy(b2Writer, reader); err != nil {
		return err
	}
	if err := b2Writer.Close(); err != nil {
		return err
	}
	return nil
}

type imgFileDto struct {
	Path      string
	CreatedAt time.Time
}
type action func(tx *sqlx.Tx) error
type datastore interface {
	executeInsideTran(actionFn action) error
	getImgByID(imgID string) ([]image, error)
	insertImg(tx *sqlx.Tx, imgEntity *image) error
	updateExisting(imgID string, imgContentHash string) error
}

type db struct {
	db *sqlx.DB
}

func newDB(dbName string) *db {
	dbInstance := sqlx.MustConnect("sqlite3", dbName)
	dbInstance.MustExec(createSchema)
	return &db{db: dbInstance}
}

func (datastore *db) executeInsideTran(actionFn action) error {
	tx := datastore.db.MustBegin()
	if err := actionFn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (datastore *db) getImgByID(imgID string) ([]image, error) {
	var existingImgs = []image{}
	if err := datastore.db.Select(&existingImgs, "SELECT img_id, created_at, img_hash, b2_img_name, b2_thumbnail_name FROM img WHERE img_id=$1 LIMIT 1;", imgID); err != nil {
		log.Printf("Error quering existing image %v - err: %v", imgID, err)
		return nil, err
	}
	return existingImgs, nil
}

func (datastore *db) insertImg(tx *sqlx.Tx, imgEntity *image) error {
	if _, err := tx.NamedExec("INSERT INTO img (img_id, created_at, img_hash, b2_img_name, b2_thumbnail_name) VALUES (:img_id, :created_at, :img_hash, :b2_img_name, :b2_thumbnail_name)", imgEntity); err != nil {
		log.Printf("Error inserting into DB: %v", err)
		return err
	}
	return nil
}

func (datastore *db) updateExisting(imgID string, imgContentHash string) error {
	existingImgs, err := datastore.getImgByID(imgID)
	if err != nil {
		log.Printf("Error quering existing image %v - err: %v", imgID, err)
		return err
	}
	imgExists := len(existingImgs) > 0
	existingImgChanged := imgExists && existingImgs[0].ImgHash != imgContentHash
	if imgExists && !existingImgChanged {
		log.Print("Existing image unchanged - do nothing and process next img")
		return nil
	} else if existingImgChanged {
		log.Print("Existing image changed.")
		_, err := datastore.db.Exec("DELETE FROM img WHERE img_id=$1", imgID)
		if err != nil {
			log.Printf("Image with the ID = %v could not be deleted - err: %v", imgID, err)
			return err
		}
	}
	return nil
}

func main() {
	f, err := os.OpenFile("output.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	if err := godotenv.Load("settings.env"); err != nil {
		log.Fatal("Error loading .env file")
	}

	encryptionKey := os.Getenv("ENCR_KEY")
	b2id := os.Getenv("B2_ACCOUNT_ID")
	b2key := os.Getenv("B2_ACCOUNT_KEY")
	bucketName := os.Getenv("B2_BUCKET_NAME")

	ctx := context.Background()

	imgStore := newb2Store(ctx, b2id, b2key, bucketName)
	db := newDB("img.db")

	imgRootDir := flag.String("syncdir", "", "Abs path to the directory containing pictures")
	imgName := flag.String("download", "", "File to download and decrypt")
	flag.Parse()

	if *imgRootDir != "" {
		imgs := getImages(*imgRootDir)
		log.Printf("Found %v images.", len(imgs))
		localState := syncLocalImagesWithBackblaze(ctx, imgs, imgStore, db, encryptionKey)
		log.Println("Sync compleated.")
		for _, imgID := range localState {
			log.Printf("ImgID: %v", imgID)
		}
	} else if *imgName != "" {
		imgStore.downloadFile(encryptionKey, *imgName, "/home/piotr/Documents/imgtest/decrypted.jpg")
	}
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
			imgCreatedAt, err := extractCreatedAt(path, f)
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

func syncLocalImagesWithBackblaze(ctx context.Context, images []*imgFileDto, imgStore fileStore, db datastore, encryptionKey string) []string {
	imgIds := make([]string, 0)

	for _, img := range images {
		imgID, err := syncImage(ctx, img, imgStore, db, encryptionKey)
		if err != nil {
			continue
		}
		imgIds = append(imgIds, imgID)
	}
	return imgIds
}

func syncImage(ctx context.Context, img *imgFileDto, imgStore fileStore, db datastore, encryptionKey string) (string, error) {
	imgID, err := calculateHash(bytes.NewReader([]byte(img.Path)))
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
	thumbnail, err := extractThumbnail(img.Path, imgContentReader)
	if err != nil {
		log.Printf("Error obtaining exif-metadata from file: %v - err msg: %v", img.Path, err)
		return "", err
	}

	imgHash, err := calculateHash(imgContentReader)
	if err != nil {
		log.Printf("Error calculating img hash: %v", err)
		return "", err
	}
	encryptedThumbnail, err := encrypt(encryptionKey, thumbnail)
	if err != nil {
		log.Printf("Error encrypting thumbnail: %v - err msg: %v", img.Path, err)
		return "", err
	}
	log.Printf("Thumbnail size: %v", len(encryptedThumbnail))
	encryptedImg, err := encrypt(encryptionKey, imgContent)
	if err != nil {
		log.Printf("Error encrypting image: %v - err msg: %v", img.Path, err)
		return "", err
	}

	if err := db.updateExisting(imgID, imgHash); err != nil {
		return "", err
	}

	b2thumbnailName := generateUniqueFileName("thumb", img.Path, img.CreatedAt)
	b2imgName := generateUniqueFileName("orig", img.Path, img.CreatedAt)
	err = db.executeInsideTran(func(tx *sqlx.Tx) error {
		imgEntity := &image{imgID, img.CreatedAt, imgHash, b2imgName, b2thumbnailName}
		if err := db.insertImg(tx, imgEntity); err != nil {
			log.Printf("Error inserting into DB: %v", err)
			return err
		}
		log.Printf("Starting upload to b2 - img %v", img.Path)

		if err := imgStore.uploadToB2(b2thumbnailName, encryptedThumbnail); err != nil {
			log.Printf("Error uploading to b2: %v - img: %v", err, img.Path)
			return err
		}
		if err := imgStore.uploadToB2(b2imgName, encryptedImg); err != nil {
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

func extractCreatedAt(imgPath string, r *os.File) (time.Time, error) {
	x, err := exif.Decode(r)
	if err != nil {
		return time.Time{}, err
	}
	defer r.Close()
	imgCreatedAt, err := x.DateTime()
	if err != nil {
		imgCreatedAt, err = findNeighborImgCreatedAt(imgPath)
		if err != nil {
			return time.Time{}, err
		}
	}
	return imgCreatedAt, nil
}

func extractThumbnail(imgPath string, r io.ReadSeeker) ([]byte, error) {
	x, err := exif.Decode(r)
	if err != nil {
		return nil, err
	}

	thumbnail, err := x.JpegThumbnail()
	if err != nil {
		thumbnail, err = resizeImg(r)
		if err != nil {
			return nil, err
		}
	}
	return thumbnail, nil
}

func resizeImg(r io.ReadSeeker) ([]byte, error) {
	r.Seek(0, 0)
	img, err := jpeg.Decode(r)
	if err != nil {
		return nil, err
	}
	m := resize.Thumbnail(200, 200, img, resize.NearestNeighbor)
	var b bytes.Buffer
	writer := bufio.NewWriter(&b)
	if err := jpeg.Encode(writer, m, nil); err != nil {
		return nil, err
	}
	if err := writer.Flush(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func findNeighborImgCreatedAt(imgPath string) (time.Time, error) {
	var imgCreatedAt time.Time
	containingdDir := filepath.Dir(imgPath)
	matches, _ := filepath.Glob(filepath.Join(containingdDir, "*.jpg"))
	for _, imgfile := range matches {
		imgCreatedAt = func(f string) time.Time {
			if f == imgPath {
				return time.Time{}
			}
			reader, err := os.Open(f)
			if err != nil {
				return time.Time{}
			}
			defer reader.Close()
			other, err := exif.Decode(reader)
			if err != nil {
				return time.Time{}
			}
			imgCreatedAt, err = other.DateTime()
			if err != nil {
				return time.Time{}
			}
			return imgCreatedAt.Add(time.Millisecond * time.Duration(1))
		}(imgfile)
		foundCreatedAt := imgCreatedAt != time.Time{}
		if foundCreatedAt {
			return imgCreatedAt, nil
		}
	}
	return time.Time{}, fmt.Errorf("Coldn't extract CreatedAt for file %v", imgPath)
}

func calculateHash(r io.Reader) (string, error) {
	h := sha1.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	hash := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return hash, nil
}

func encrypt(encryptionKey string, data []byte) ([]byte, error) {
	secretKeyBytes, err := hex.DecodeString(encryptionKey)
	if err != nil {
		return nil, err
	}

	var secretKey [32]byte
	copy(secretKey[:], secretKeyBytes)

	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, err
	}

	encrypted := secretbox.Seal(nonce[:], data, &nonce, &secretKey)
	return encrypted, nil
}

func decrypt(encryptionKey string, encryptedData []byte) ([]byte, error) {
	secretKeyBytes, err := hex.DecodeString(encryptionKey)
	if err != nil {
		return nil, err
	}

	var secretKey [32]byte
	copy(secretKey[:], secretKeyBytes)
	var decryptNonce [24]byte
	copy(decryptNonce[:], encryptedData[:24])
	decrypted, ok := secretbox.Open(nil, encryptedData[24:], &decryptNonce, &secretKey)
	if !ok {
		return nil, errors.New("Could not decrypt data")
	}

	return decrypted, nil
}

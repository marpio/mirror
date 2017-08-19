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

func main() {
	//f, err := os.OpenFile("output.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	//if err != nil {
	//	log.Fatalf("error opening file: %v", err)
	//}
	//defer f.Close()
	//log.SetOutput(f)
	err := godotenv.Load("settings.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	encryptionKey := os.Getenv("ENCR_KEY")
	b2id := os.Getenv("B2_ACCOUNT_ID")
	b2key := os.Getenv("B2_ACCOUNT_KEY")
	bucketName := os.Getenv("B2_BUCKET_NAME")

	db := sqlx.MustConnect("sqlite3", "img.db")
	ctx := context.Background()
	b2Client, err := b2.NewClient(ctx, b2id, b2key)
	if err != nil {
		log.Fatal(err)
	}

	bucket, err := b2Client.Bucket(ctx, bucketName)
	if err != nil {
		log.Fatal(err)
	}
	db.MustExec(createSchema)

	imgRootDir := os.Args[1]

	paths := getAllImagesPaths(imgRootDir)
	log.Printf("Found %v images.", len(paths))
	localState := syncLocalImagesWithBackblaze(ctx, paths, bucket, db, encryptionKey)
	log.Println("Uploading new and updating existing images compleated.")
	for _, imgID := range localState {
		log.Printf("ImgID: %v", imgID)
	}
}

func getAllImagesPaths(rootPath string) []string {
	paths := make([]string, 0)
	var isJpegPredicate = func(path string, f os.FileInfo) bool {
		return !f.IsDir() && (strings.HasSuffix(strings.ToLower(f.Name()), ".jpg") || strings.HasSuffix(strings.ToLower(f.Name()), ".jpeg"))
	}
	err := filepath.Walk(rootPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error while walking the directory structure: %v", err.Error())
		}
		if isJpegPredicate(path, fi) {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf(err.Error())
	}
	return paths
}

func syncLocalImagesWithBackblaze(ctx context.Context, paths []string, bucket *b2.Bucket, db *sqlx.DB, encryptionKey string) []string {
	imgIds := make([]string, 0)

	for _, p := range paths {
		imgID, err := syncImage(ctx, p, bucket, db, encryptionKey)
		if err != nil {
			continue
		}
		imgIds = append(imgIds, imgID)
	}
	return imgIds
}

func syncImage(ctx context.Context, imgPath string, bucket *b2.Bucket, db *sqlx.DB, encryptionKey string) (string, error) {
	imgID, err := calculateHash(bytes.NewReader([]byte(imgPath)))
	if err != nil {
		log.Printf("Error calculating path's hash - path: %v - err msg: %v", imgPath, err)
		return "", err
	}
	f, err := os.Open(imgPath)
	if err != nil {
		log.Printf("Error opening file %v - err msg: %v", imgPath, err)
		return "", err
	}
	defer f.Close()
	imgContent, err := ioutil.ReadAll(f)
	if err != nil {
		log.Printf("Error reading file %v - err msg: %v", imgPath, err)
		return "", err
	}
	imgContentReader := bytes.NewReader(imgContent)
	imgCreatedAt, thumbnail, err := readExifMetadata(imgContentReader)
	if err != nil {
		log.Printf("Error obtaining exif-metadata from file: %v - err msg: %v", imgPath, err)
		return "", err
	}
	if isImgToOld(imgCreatedAt) {
		return "", err
	}
	imgHash, err := calculateHash(imgContentReader)
	if err != nil {
		log.Printf("Error calculating img hash: %v", err)
		return "", err
	}
	log.Printf("Calculated img hash: %v", imgHash)
	encryptedThumbnail, err := encrypt(encryptionKey, thumbnail)
	if err != nil {
		log.Printf("Error encrypting thumbnail: %v - err msg: %v", imgPath, err)
		return "", err
	}
	log.Printf("Thumbnail size: %v", len(encryptedThumbnail))
	encryptedImg, err := encrypt(encryptionKey, imgContent)
	if err != nil {
		log.Printf("Error encrypting image: %v - err msg: %v", imgPath, err)
		return "", err
	}
	log.Printf("Image size: %v", len(encryptedImg))

	var existingImgs = []image{}
	if err := db.Select(&existingImgs, "SELECT img_id, created_at, img_hash, b2_img_name, b2_thumbnail_name FROM img WHERE img_id=$1 LIMIT 1;", imgID); err != nil {
		log.Printf("Error quering existing image %v - err: %v", imgID, err)
		return "", err
	}
	if len(existingImgs) > 0 && existingImgs[0].ImgHash != imgHash {
		log.Print("Existing image changed.")
		_, err := db.Exec("DELETE FROM img WHERE img_id=$1", imgID)
		if err != nil {
			log.Printf("Image with the ID = %v could not be deleted - err: %v", imgID, err)
			return "", err
		}
	} else if len(existingImgs) == 0 {
		b2thumbnailName := generateUniqueFileName("thumb", imgPath, imgCreatedAt)
		b2imgName := generateUniqueFileName("", imgPath, imgCreatedAt)

		thumbnailb2Writer := createB2Writer(ctx, bucket, b2thumbnailName)
		thumbnailReader := bytes.NewReader(encryptedThumbnail)
		imgb2Writer := createB2Writer(ctx, bucket, b2imgName)
		imgReader := bytes.NewReader(encryptedImg)

		tx := db.MustBegin()

		img := &image{imgID, imgCreatedAt, imgHash, b2imgName, b2thumbnailName}
		if _, err := tx.NamedExec("INSERT INTO img (img_id, created_at, img_hash, b2_img_name, b2_thumbnail_name) VALUES (:img_id, :created_at, :img_hash, :b2_img_name, :b2_thumbnail_name)", img); err != nil {
			log.Printf("Error inserting into DB: %v", err)
			return "", err
		}
		if _, err := io.Copy(thumbnailb2Writer, thumbnailReader); err != nil {
			log.Printf("Error copying to b2: %v", err)
			return "", err
		}
		if err := thumbnailb2Writer.Close(); err != nil {
			log.Printf("Error closing thumbnailb2Writer: %v", err)
			return "", err
		}
		if _, err := io.Copy(imgb2Writer, imgReader); err != nil {
			log.Printf("Error copying to b2: %v", err)
			return "", err
		}
		if err := imgb2Writer.Close(); err != nil {
			log.Printf("Error closing imgb2Writer: %v", err)
			return "", err
		}
		err = tx.Commit()
		if err != nil {
			log.Printf("Error commiting to the db: %v", err)
			return "", err
		}
	}
	return imgID, nil
}

func createB2Writer(ctx context.Context, bucket *b2.Bucket, imgFileName string) *b2.Writer {
	imgObj := bucket.Object(imgFileName)
	b2Writer := imgObj.NewWriter(ctx)
	return b2Writer
}

func generateUniqueFileName(prefix string, imgPath string, imgCreatedAt time.Time) string {
	nano := strconv.FormatInt(imgCreatedAt.UnixNano(), 10)
	imgFileName := prefix + "_" + nano + "_" + path.Base(imgPath)
	return imgFileName
}

func isImgToOld(createdAt time.Time) bool {
	return createdAt.Year() < time.Now().Add(-1*time.Hour*24*365*10).Year()
}

func readExifMetadata(r io.ReadSeeker) (time.Time, []byte, error) {
	x, err := exif.Decode(r)
	if err != nil {
		return time.Time{}, nil, err
	}
	imgCreatedAt, err := x.DateTime()
	if err != nil {
		return time.Time{}, nil, err
	}
	thumbnail, err := x.JpegThumbnail()
	if err != nil {
		r.Seek(0, 0)
		img, err := jpeg.Decode(r)
		m := resize.Thumbnail(200, 200, img, resize.NearestNeighbor)
		var b bytes.Buffer
		writer := bufio.NewWriter(&b)
		jpeg.Encode(writer, m, nil)
		writer.Flush()
		return imgCreatedAt, b.Bytes(), err
	}
	return imgCreatedAt, thumbnail, nil
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
		return nil, errors.New("could not decrypt data")
	}

	return decrypted, nil
}

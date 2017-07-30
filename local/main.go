package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"image/jpeg"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nfnt/resize"
	"github.com/rwcarlsen/goexif/exif"
)

type image struct {
	Path      string
	Hash      string
	Thumbnail []byte
	CreatedAt time.Time
}

func main() {
	f, err := os.OpenFile("output.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)
	imgRootDir := os.Args[1]

	imgs := getLocalImages(imgRootDir)

	//for path, img := range imgs {
	//	log.Println(path)
	//	log.Println(img)
	//	log.Println("-----------------------------------------")
	//}
	log.Println(len(imgs))
	log.Println("--------------------------------------")

}

func getLocalImages(rootPath string) map[string]image {
	imgs := make(map[string]image)

	var isJpegPredicate = func(path string, f os.FileInfo) bool {
		return !f.IsDir() && (strings.HasSuffix(strings.ToLower(f.Name()), ".jpg") || strings.HasSuffix(strings.ToLower(f.Name()), ".jpeg"))
	}
	err := filepath.Walk(rootPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error while walking the directory structure: %v", err.Error())
		}
		if isJpegPredicate(path, fi) {
			f, err := os.Open(path)
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()

			x, err := exif.Decode(f)
			if err != nil {
				log.Fatal(err)
			}
			imgCreatedAt, _ := x.DateTime()
			if imgCreatedAt.Year() > time.Now().Add(-1*time.Hour*24*365*10).Year() {
				h := md5.New()
				if _, err := io.Copy(h, f); err != nil {
					log.Fatal(err)
				}
				decodedImg, err := jpeg.Decode(f)
				if err != nil {
					log.Fatal(err)
				}
				m := resize.Thumbnail(100, 100, decodedImg, resize.NearestNeighbor)
				var b bytes.Buffer
				writer := bufio.NewWriter(&b)
				jpeg.Encode(writer, m, &jpeg.Options{Quality: 50})

				img := image{
					Path:      path,
					Hash:      base64.StdEncoding.EncodeToString(h.Sum(nil)),
					Thumbnail: b.Bytes(),
					CreatedAt: imgCreatedAt,
				}
				imgs[path] = img
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf(err.Error())
	}
	return imgs
}

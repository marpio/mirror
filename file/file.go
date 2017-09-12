package file

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/marpio/img-store/crypto"
)

type File interface {
	io.ReadWriteCloser
	io.Seeker
}

type FileInfo struct {
	PathHash string
	Path     string
	ModTime  time.Time
}

func FindPhotos(rootPath string) []*FileInfo {
	var isJpegPredicate = func(path string, f os.FileInfo) bool {
		return !f.IsDir() && (strings.HasSuffix(strings.ToLower(f.Name()), ".jpg") || strings.HasSuffix(strings.ToLower(f.Name()), ".jpeg"))
	}
	photos := make([]*FileInfo, 0)
	err := filepath.Walk(rootPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error while walking the directory structure: %v", err.Error())
		}
		if isJpegPredicate(path, fi) {
			id, _ := crypto.CalculateHash(bytes.NewReader([]byte(path)))

			finf := &FileInfo{PathHash: id, Path: path, ModTime: fi.ModTime()}
			photos = append(photos, finf)
		}
		return nil
	})
	if err != nil {
		log.Fatalf(err.Error())
	}
	return photos
}

func ReadFile(filename string) (File, error) {
	return os.Open(filename)
}

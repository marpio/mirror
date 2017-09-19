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

func FindPhotos(rootPath string, isUnchangedFn func(string, time.Time) bool) (newOrChanged []*FileInfo) {
	newOrChanged = make([]*FileInfo, 0)
	var isJpeg = func(path string, f os.FileInfo) bool {
		return !f.IsDir() && (strings.HasSuffix(strings.ToLower(f.Name()), ".jpg") || strings.HasSuffix(strings.ToLower(f.Name()), ".jpeg"))
	}
	err := filepath.Walk(rootPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error while walking the directory structure: %v", err.Error())
		}
		if isJpeg(path, fi) {
			id, _ := crypto.CalculateHash(bytes.NewReader([]byte(path)))
			modTime := fi.ModTime()
			finf := &FileInfo{PathHash: id, Path: path, ModTime: fi.ModTime()}
			if !isUnchangedFn(id, modTime) {
				newOrChanged = append(newOrChanged, finf)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf(err.Error())
	}
	return newOrChanged
}

func ReadFile(filename string) (File, error) {
	return os.Open(filename)
}

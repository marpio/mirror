package file

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type File interface {
	io.ReadWriteCloser
	io.Seeker
}

type FileInfo struct {
	Path    string
	ModTime time.Time
}

func FindPhotos(rootPath string, isUnchangedFn func(string, time.Time) bool) (newOrChanged []*FileInfo) {
	newOrChanged = make([]*FileInfo, 0)

	err := filepath.Walk(rootPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error while walking the directory structure: %v", err.Error())
		}
		isJpeg := !fi.IsDir() && (strings.HasSuffix(strings.ToLower(fi.Name()), ".jpg") || strings.HasSuffix(strings.ToLower(fi.Name()), ".jpeg"))
		if isJpeg {
			modTime := fi.ModTime()
			finf := &FileInfo{Path: path, ModTime: fi.ModTime()}
			if !isUnchangedFn(path, modTime) {
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

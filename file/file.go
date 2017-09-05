package file

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type File interface {
	io.ReadWriteCloser
	io.Seeker
}

func GetImages(rootPath string) []string {
	imgFiles := make([]string, 0)

	var isJpegPredicate = func(path string, f os.FileInfo) bool {
		return !f.IsDir() && (strings.HasSuffix(strings.ToLower(f.Name()), ".jpg") || strings.HasSuffix(strings.ToLower(f.Name()), ".jpeg"))
	}

	err := filepath.Walk(rootPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error while walking the directory structure: %v", err.Error())
		}
		if isJpegPredicate(path, fi) {
			imgFiles = append(imgFiles, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf(err.Error())
	}
	return imgFiles
}

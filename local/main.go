package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	imgDir := os.Args[1]
	imgs := []string{}
	var isJpegPredicate = func(path string, f os.FileInfo) bool {
		return !f.IsDir() && (strings.HasSuffix(strings.ToLower(f.Name()), ".jpg") || strings.HasSuffix(strings.ToLower(f.Name()), ".jpeg"))
	}
	err := filepath.Walk(imgDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error while walking the directory structure: %v", err.Error())
		}
		if isJpegPredicate(path, fi) {
			imgs = append(imgs, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf(err.Error())
	}
	for _, imgPath := range imgs {
		log.Println(imgPath)
	}
}

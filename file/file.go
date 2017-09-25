package file

import (
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/afero"
)

type File interface {
	io.ReadCloser
	io.Seeker
}

type FileInfo struct {
	Path    string
	ModTime time.Time
}

func PhotosFinder(fs afero.Fs) func(rootPath string, isUnchangedFn func(string, time.Time) bool) (newOrChanged []*FileInfo) {
	return func(rootPath string, isUnchangedFn func(string, time.Time) bool) (newOrChanged []*FileInfo) {
		return findPhotos(fs, rootPath, isUnchangedFn)
	}
}

func FileReader(fs afero.Fs) func(path string) (File, error) {
	return func(path string) (File, error) {
		return readFile(fs, path)
	}
}

func GenerateUniqueFileName(prefix string, fpath string, createdAt time.Time) string {
	nano := strconv.FormatInt(createdAt.UnixNano(), 10)
	imgFileName := prefix + "_" + nano + "_" + path.Base(fpath)
	return imgFileName
}

func GroupByDir(photos []*FileInfo) map[string][]*FileInfo {
	photosGroupedByDir := make(map[string][]*FileInfo)
	for _, p := range photos {
		dir := filepath.Dir(p.Path)
		if v, ok := photosGroupedByDir[dir]; ok {
			v = append(v, p)
			photosGroupedByDir[dir] = v
		} else {
			ps := make([]*FileInfo, 0)
			ps = append(ps, p)
			photosGroupedByDir[dir] = ps
		}
	}
	return photosGroupedByDir
}

func findPhotos(fs afero.Fs, rootPath string, isUnchangedFn func(string, time.Time) bool) (newOrChanged []*FileInfo) {
	newOrChanged = make([]*FileInfo, 0)

	err := afero.Walk(fs, rootPath, func(path string, fi os.FileInfo, err error) error {

		if err != nil {
			log.Printf("Error while walking the directory structure: %v", err.Error())
		}
		isDir, err := afero.IsDir(fs, path)

		if err != nil {
			return err
		}
		isJpeg := !isDir && (strings.HasSuffix(strings.ToLower(fi.Name()), ".jpg") || strings.HasSuffix(strings.ToLower(fi.Name()), ".jpeg"))
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

func readFile(fs afero.Fs, filePath string) (File, error) {
	return fs.Open(filePath)
}

package fsutils

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

type FileReaderFn func(path string) (File, error)

func PhotosFinder(fs afero.Fs) func(rootPath string, predicate func(string, time.Time) bool) (newAndChangedPhotos []*FileInfo) {
	return func(rootPath string, predicate func(string, time.Time) bool) (newAndChangedPhotos []*FileInfo) {
		return findFiles(fs, rootPath, predicate, ".jpg", ".jpeg")
	}
}

func FileReader(fs afero.Fs) FileReaderFn {
	return func(path string) (File, error) {
		return fs.Open(path)
	}
}

func GenerateUniqueFileName(prefix string, fpath string, createdAt time.Time) string {
	nano := strconv.FormatInt(createdAt.UnixNano(), 10)
	imgFileName := prefix + "_" + nano + "_" + path.Base(fpath)
	return imgFileName
}

func GroupByDir(files []*FileInfo) map[string][]*FileInfo {
	photosGroupedByDir := make(map[string][]*FileInfo)
	for _, p := range files {
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

func findFiles(fs afero.Fs, rootPath string, predicate func(string, time.Time) bool, fileExt ...string) (files []*FileInfo) {
	files = make([]*FileInfo, 0)

	err := afero.Walk(fs, rootPath, func(path string, fi os.FileInfo, err error) error {

		if err != nil {
			log.Printf("Error while walking the directory structure: %v", err.Error())
		}
		isDir, err := afero.IsDir(fs, path)

		if err != nil {
			return err
		}
		if isDir {
			return nil
		}
		hasExt := false
		for _, ext := range fileExt {
			hasExt = strings.HasSuffix(strings.ToLower(fi.Name()), ext)
			if hasExt {
				break
			}
		}
		if hasExt {
			modTime := fi.ModTime()
			finf := &FileInfo{Path: path, ModTime: fi.ModTime()}

			if predicate(path, modTime) {
				files = append(files, finf)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf(err.Error())
	}
	return files
}

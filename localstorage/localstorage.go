package localstorage

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/marpio/img-store/fs"
	"github.com/spf13/afero"
)

type Service interface {
	SearchFiles(rootPath string, filter func(string, time.Time) bool, fileExt ...string) []*fs.FileInfo
	ReadFile(path string) (fs.File, error)
}

func NewService(fs afero.Fs) Service {
	return &srv{fs: fs}
}

type srv struct {
	fs afero.Fs
}

func (repo *srv) SearchFiles(rootPath string, filter func(string, time.Time) bool, fileExt ...string) []*fs.FileInfo {
	return findFiles(repo.fs, rootPath, filter, fileExt...)
}

func (repo *srv) ReadFile(path string) (fs.File, error) {
	return repo.fs.Open(path)
}

func findFiles(afs afero.Fs, rootPath string, predicate func(string, time.Time) bool, fileExt ...string) []*fs.FileInfo {
	files := make([]*fs.FileInfo, 0)

	err := afero.Walk(afs, rootPath, func(path string, fi os.FileInfo, err error) error {

		if err != nil {
			log.Printf("Error while walking the directory structure: %v", err)
		}
		isDir, err := afero.IsDir(afs, path)

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
			finf := &fs.FileInfo{Path: path, ModTime: fi.ModTime()}

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

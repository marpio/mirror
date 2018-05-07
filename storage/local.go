package storage

import (
	"context"
	"io"
	"log"
	"os"
	"strings"

	"github.com/marpio/mirror"
	"github.com/spf13/afero"
)

type FileInfo struct {
	id               string
	readFile         func(string) (io.ReadCloser, error)
	generateFileHash func(r io.Reader) (string, error)
	filePath         string
	fileExt          string
}

func (fi FileInfo) FilePath() string {
	return fi.filePath
}

func (fi FileInfo) ID() string {
	if fi.id != "" {
		return fi.id
	}
	f, err := fi.readFile(fi.filePath)
	if err != nil {
		return ""
	}
	defer f.Close()
	id, err := fi.generateFileHash(f)
	if err != nil {
		return ""
	}
	fi.id = id
	return fi.id
}

func NewFileInfo(filePath string, readFile func(string) (io.ReadCloser, error), generateFileHash func(io.Reader) (string, error)) FileInfo {
	return FileInfo{
		readFile:         readFile,
		generateFileHash: generateFileHash,
		filePath:         filePath,
	}
}

type ReadOnlyLocalStorage struct {
	fs               afero.Fs
	generateFileHash func(io.Reader) (string, error)
}

func NewLocal(fs afero.Fs, generateFileHash func(io.Reader) (string, error)) *ReadOnlyLocalStorage {
	return &ReadOnlyLocalStorage{fs: fs, generateFileHash: generateFileHash}
}

func (repo *ReadOnlyLocalStorage) NewReader(ctx context.Context, path string) (io.ReadCloser, error) {
	return repo.fs.Open(path)
}

func (repo *ReadOnlyLocalStorage) FindFiles(dir string, fileExt ...string) []mirror.FileInfo {
	files := make([]mirror.FileInfo, 0)
	open := func(p string) (io.ReadCloser, error) {
		return os.Open(p)
	}
	err := afero.Walk(repo.fs, dir, func(pth string, fi os.FileInfo, err error) error {
		isDir, err := afero.IsDir(repo.fs, pth)

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
				finf := NewFileInfo(pth, open, repo.generateFileHash)
				files = append(files, finf)
				break
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf(err.Error())
	}
	return files
}

func GenerateUniqueFileName(prefix string, id string) string {
	imgFileName := prefix + "_" + id
	return imgFileName
}

package storage

import (
	"context"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/marpio/mirror"
	"github.com/spf13/afero"
)

type fileInfo struct {
	id               string
	readFile         func(string) ([]byte, error)
	generateFileHash func([]byte) string
	filePath         string
	fileExt          string
}

func (fi *fileInfo) FilePath() string {
	return fi.filePath
}

func (fi *fileInfo) FileExt() string {
	return fi.fileExt
}

func (fi *fileInfo) ID() string {
	if fi.id != "" {
		return fi.id
	}
	b, err := fi.readFile(fi.filePath)
	if err != nil {
		return ""
	}
	fi.id = fi.generateFileHash(b)
	return fi.id
}

func newFileInfo(filePath string, fileExt string, readFile func(string) ([]byte, error), generateFileHash func([]byte) string) mirror.FileInfo {
	return &fileInfo{
		readFile:         readFile,
		generateFileHash: generateFileHash,
		filePath:         filePath,
		fileExt:          fileExt,
	}
}

type ReadOnlyLocalStorage struct {
	fs               afero.Fs
	generateFileHash func([]byte) string
}

func NewLocal(fs afero.Fs, generateFileHash func([]byte) string) *ReadOnlyLocalStorage {
	return &ReadOnlyLocalStorage{fs: fs, generateFileHash: generateFileHash}
}

func (repo *ReadOnlyLocalStorage) NewReader(ctx context.Context, path string) (io.ReadCloser, error) {
	return repo.fs.Open(path)
}

func (repo *ReadOnlyLocalStorage) NewReadSeeker(ctx context.Context, path string) (mirror.ReadCloseSeeker, error) {
	return repo.fs.Open(path)
}

func (repo *ReadOnlyLocalStorage) SearchFiles(rootPath string, fileExt ...string) []mirror.FileInfo {
	files := make([]mirror.FileInfo, 0)
	err := afero.Walk(repo.fs, rootPath, func(pth string, fi os.FileInfo, err error) error {

		if err != nil {
			log.Printf("Error while walking the directory structure: %v", err)
		}
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
				finf := newFileInfo(pth, path.Ext(pth), ioutil.ReadFile, repo.generateFileHash)
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

func GroupByDir(files []mirror.FileInfo) map[string][]mirror.FileInfo {
	filesGroupedByDir := make(map[string][]mirror.FileInfo)
	for _, p := range files {
		dir := filepath.Dir(p.FilePath())
		if v, ok := filesGroupedByDir[dir]; ok {
			v = append(v, p)
			filesGroupedByDir[dir] = v
		} else {
			ps := make([]mirror.FileInfo, 0)
			ps = append(ps, p)
			filesGroupedByDir[dir] = ps
		}
	}
	return filesGroupedByDir
}

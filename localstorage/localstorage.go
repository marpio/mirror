package localstorage

import (
	"context"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/marpio/mirror/domain"
	"github.com/spf13/afero"
)

type srv struct {
	fs afero.Fs
}

func NewService(fs afero.Fs) domain.LocalStorage {
	return &srv{fs: fs}
}

func (repo *srv) NewReader(ctx context.Context, path string) (io.ReadCloser, error) {
	return repo.fs.Open(path)
}

func (repo *srv) NewReadSeeker(ctx context.Context, path string) (domain.ReadCloseSeeker, error) {
	return repo.fs.Open(path)
}

func (repo *srv) SearchFiles(rootPath string, isNewFilter func(*domain.FileInfo) bool, fileExt ...string) ([]*domain.FileInfo, []*domain.FileInfo) {
	newFiles := make([]*domain.FileInfo, 0)
	oldFiles := make([]*domain.FileInfo, 0)
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
				break
			}
		}
		if hasExt {
			finf := &domain.FileInfo{FilePath: pth, FileExt: path.Ext(pth)}

			if isNewFilter(finf) {
				newFiles = append(newFiles, finf)
			} else {
				oldFiles = append(oldFiles, finf)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf(err.Error())
	}
	return newFiles, oldFiles
}

func GenerateUniqueFileName(prefix string, id string) string {
	imgFileName := prefix + "_" + id
	return imgFileName
}

func GroupByDir(files []*domain.FileInfo) map[string][]*domain.FileInfo {
	filesGroupedByDir := make(map[string][]*domain.FileInfo)
	for _, p := range files {
		dir := filepath.Dir(p.FilePath)
		if v, ok := filesGroupedByDir[dir]; ok {
			v = append(v, p)
			filesGroupedByDir[dir] = v
		} else {
			ps := make([]*domain.FileInfo, 0)
			ps = append(ps, p)
			filesGroupedByDir[dir] = ps
		}
	}
	return filesGroupedByDir
}

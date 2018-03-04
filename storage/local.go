package storage

import (
	"context"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/marpio/mirror"
	"github.com/spf13/afero"
)

type local struct {
	fs afero.Fs
}

func NewLocal(fs afero.Fs) mirror.ReadOnlyStorage {
	return &local{fs: fs}
}

func (repo *local) NewReader(ctx context.Context, path string) (io.ReadCloser, error) {
	return repo.fs.Open(path)
}

func (repo *local) NewReadSeeker(ctx context.Context, path string) (mirror.ReadCloseSeeker, error) {
	return repo.fs.Open(path)
}

func (repo *local) SearchFiles(rootPath string, fileExt ...string) []*mirror.FileInfo {
	files := make([]*mirror.FileInfo, 0)
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
				finf := &mirror.FileInfo{FilePath: pth, FileExt: path.Ext(pth)}
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

func GroupByDir(files []*mirror.FileInfo) map[string][]*mirror.FileInfo {
	filesGroupedByDir := make(map[string][]*mirror.FileInfo)
	for _, p := range files {
		dir := filepath.Dir(p.FilePath)
		if v, ok := filesGroupedByDir[dir]; ok {
			v = append(v, p)
			filesGroupedByDir[dir] = v
		} else {
			ps := make([]*mirror.FileInfo, 0)
			ps = append(ps, p)
			filesGroupedByDir[dir] = ps
		}
	}
	return filesGroupedByDir
}

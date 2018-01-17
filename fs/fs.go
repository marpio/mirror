package fs

import (
	"io"
	"path"
	"path/filepath"
	"strconv"
	"time"
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

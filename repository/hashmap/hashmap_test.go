package hashmap

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/marpio/mirror"
	"github.com/marpio/mirror/crypto"
	"github.com/marpio/mirror/storage"
	"github.com/marpio/mirror/storage/filesystem"

	"github.com/spf13/afero"
)

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

var ctx context.Context = context.Background()

const dbPath string = "mirror.db"
const key string = "b567ef1d391e8a10d94100faa34b7d28fdab13e3f51f94b8"

func setup() (mirror.MetadataRepo, afero.Fs) {
	afs := afero.NewMemMapFs()
	s, afs := initRepo(afs)
	return s, afs
}

func initRepo(afs afero.Fs) (mirror.MetadataRepo, afero.Fs) {
	b := storage.NewRemote(filesystem.New(afs), crypto.NewService(key))
	s, _ := New(ctx, b, dbPath)
	return s, afs
}
func TestExists(t *testing.T) {
	s, _ := setup()
	p := "/path/to/file"
	p2 := "/path/to/file2"
	ph1 := mirror.NewPhoto(
		&mirror.FileInfo{FilePath: p},
		&mirror.Metadata{CreatedAt: time.Date(2017, 5, 1, 0, 0, 0, 0, time.UTC)},
		func() (io.ReadCloser, error) { return nopCloser{bytes.NewReader(make([]byte, 0))}, nil })
	ph2 := mirror.NewPhoto(
		&mirror.FileInfo{FilePath: p2},
		&mirror.Metadata{CreatedAt: time.Date(2017, 5, 1, 0, 0, 0, 0, time.UTC)},
		func() (io.ReadCloser, error) { return nopCloser{bytes.NewReader(make([]byte, 0))}, nil })
	s.Add(ph1)
	s.Add(ph2)
	exists, _ := s.Exists(ph1.ID())
	if !exists {
		t.Errorf("expected to find element with id %v", ph1.ID())
	}
}

func TestGetByDir(t *testing.T) {
	s, _ := setup()
	p := "/path/to/file"
	m := time.Date(2017, 5, 1, 0, 0, 0, 0, time.UTC)
	ph := mirror.NewPhoto(
		&mirror.FileInfo{FilePath: p},
		&mirror.Metadata{CreatedAt: m},
		func() (io.ReadCloser, error) { return nopCloser{bytes.NewReader(make([]byte, 0))}, nil })
	s.Add(ph)
	r, _ := s.GetByDir("2017-05")
	if len(r) != 1 || r[0].Dir() != "2017-05" {
		t.Errorf("Expected one result, got: %v", len(r))
	}
}

func TestGetDirs(t *testing.T) {
	s, _ := setup()
	p := "/path/to/file"
	p2 := "/path/to/file2"
	m := time.Date(2017, 5, 1, 0, 0, 0, 0, time.UTC)
	m2 := time.Date(2017, 6, 1, 0, 0, 0, 0, time.UTC)
	ph1 := mirror.NewPhoto(
		&mirror.FileInfo{FilePath: p},
		&mirror.Metadata{CreatedAt: m},
		func() (io.ReadCloser, error) { return nopCloser{bytes.NewReader(make([]byte, 0))}, nil })
	ph2 := mirror.NewPhoto(
		&mirror.FileInfo{FilePath: p2},
		&mirror.Metadata{CreatedAt: m2},
		func() (io.ReadCloser, error) { return nopCloser{bytes.NewReader(make([]byte, 0))}, nil })
	s.Add(ph1)
	s.Add(ph2)
	r, _ := s.GetDirs()
	if len(r) != 2 {
		t.Errorf("Expected 2 results, got: %v", len(r))
	}
	if !(r[0] == "2017-06" && r[1] == "2017-05") {
		t.Errorf("Months not sorted.")
	}
}

func TestDelete(t *testing.T) {
	s, _ := setup()
	p := "/path/to/file"
	m := time.Date(2017, 5, 1, 0, 0, 0, 0, time.UTC)
	p1 := mirror.NewPhoto(
		&mirror.FileInfo{FilePath: p},
		&mirror.Metadata{CreatedAt: m},
		func() (io.ReadCloser, error) { return nopCloser{bytes.NewReader(make([]byte, 0))}, nil })
	s.Add(p1)
	s.Delete(p1.ID())
	exst, _ := s.Exists(p1.ID())
	if exst {
		t.Error("expected not to find anything")
	}
}

func TestPersist(t *testing.T) {
	s, afs := setup()
	p := "/path/to/file"
	m := time.Date(2017, 5, 1, 0, 0, 0, 0, time.UTC)
	p1 := mirror.NewPhoto(
		&mirror.FileInfo{FilePath: p},
		&mirror.Metadata{CreatedAt: m},
		func() (io.ReadCloser, error) { return nopCloser{bytes.NewReader(make([]byte, 0))}, nil })
	s.Add(p1)

	s.Persist(ctx)
	s2, _ := initRepo(afs)
	exists, _ := s2.Exists(p1.ID())
	if !exists {
		t.Error("expected to find one item")
	}
}

func TestReload(t *testing.T) {
	dbPath := "mirror.db"
	s, afs := setup()
	p := "/path/to/file"
	m := time.Date(2017, 5, 1, 0, 0, 0, 0, time.UTC)
	p1 := mirror.NewPhoto(
		&mirror.FileInfo{FilePath: p},
		&mirror.Metadata{CreatedAt: m},
		func() (io.ReadCloser, error) { return nopCloser{bytes.NewReader(make([]byte, 0))}, nil })
	s.Add(p1)
	s.Persist(ctx)

	afs.Rename(dbPath, "photo2.db")

	s2, _ := initRepo(afs)
	exists, _ := s2.Exists(p1.ID())
	if exists {
		t.Error("expected not to find anything")
	}

	afs.Rename("photo2.db", dbPath)
	s2.Reload(ctx)
	exists, _ = s2.Exists(p1.ID())
	if !exists {
		t.Error("expected to find one item")
	}
}

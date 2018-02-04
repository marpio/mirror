package hashmap

import (
	"context"
	"testing"
	"time"

	"github.com/marpio/img-store/crypto"
	"github.com/marpio/img-store/remotestorage"
	"github.com/marpio/img-store/remotestorage/filesystem"

	"github.com/marpio/img-store/domain"
	"github.com/spf13/afero"
)

var ctx context.Context = context.Background()

func TestExists(t *testing.T) {
	s, _ := setup()
	p := "/path/to/file"
	p2 := "/path/to/file2"
	ph1 := &domain.Photo{FileInfo: &domain.FileInfo{FilePath: p}, Metadata: &domain.Metadata{CreatedAt: time.Date(2017, 5, 1, 0, 0, 0, 0, time.UTC)}}
	ph2 := &domain.Photo{FileInfo: &domain.FileInfo{FilePath: p2}, Metadata: &domain.Metadata{CreatedAt: time.Date(2017, 5, 1, 0, 0, 0, 0, time.UTC)}}
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
	s.Add(&domain.Photo{FileInfo: &domain.FileInfo{FilePath: p}, Metadata: &domain.Metadata{CreatedAt: m}})
	r, _ := s.GetByDir("2017-05")
	if len(r) != 1 || r[0].Dir() != "2017-05" {
		t.Errorf("Expected one result, got: %v", len(r))
	}
}

func TestGetMonths(t *testing.T) {
	s, _ := setup()
	p := "/path/to/file"
	p2 := "/path/to/file2"
	m := time.Date(2017, 5, 1, 0, 0, 0, 0, time.UTC)
	m2 := time.Date(2017, 6, 1, 0, 0, 0, 0, time.UTC)
	s.Add(&domain.Photo{FileInfo: &domain.FileInfo{FilePath: p}, Metadata: &domain.Metadata{CreatedAt: m}})
	s.Add(&domain.Photo{FileInfo: &domain.FileInfo{FilePath: p2}, Metadata: &domain.Metadata{CreatedAt: m2}})
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
	p1 := &domain.Photo{FileInfo: &domain.FileInfo{FilePath: p}, Metadata: &domain.Metadata{CreatedAt: m}}
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
	p1 := &domain.Photo{FileInfo: &domain.FileInfo{FilePath: p}, Metadata: &domain.Metadata{CreatedAt: m}}
	s.Add(p1)

	s.Persist(ctx)
	dbPath := "domain.db"
	s2 := setup2(afs, dbPath)
	exists, _ := s2.Exists(p1.ID())
	if !exists {
		t.Error("expected to find one item")
	}
}

func TestReload(t *testing.T) {
	dbPath := "domain.db"
	s, afs := setup()
	p := "/path/to/file"
	m := time.Date(2017, 5, 1, 0, 0, 0, 0, time.UTC)
	p1 := &domain.Photo{FileInfo: &domain.FileInfo{FilePath: p}, Metadata: &domain.Metadata{CreatedAt: m}}
	s.Add(p1)
	s.Persist(ctx)

	afs.Rename(dbPath, "photo2.db")

	s2 := setup2(afs, dbPath)
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

func setup() (domain.MetadataRepo, afero.Fs) {
	afs := afero.NewMemMapFs()
	b := remotestorage.New(filesystem.New(afs), crypto.NewService("b567ef1d391e8a10d94100faa34b7d28fdab13e3f51f94b8"))
	dbPath := "domain.db"

	s, _ := New(ctx, b, dbPath)
	return s, afs
}

func setup2(afs afero.Fs, dbPath string) domain.MetadataRepo {
	b := remotestorage.New(filesystem.New(afs), crypto.NewService("b567ef1d391e8a10d94100faa34b7d28fdab13e3f51f94b8"))
	s, _ := New(ctx, b, dbPath)
	return s
}

package hashmap

import (
	"testing"
	"time"

	"github.com/marpio/img-store/crypto"
	"github.com/marpio/img-store/remotestorage"
	"github.com/marpio/img-store/remotestorage/filesystem"

	"github.com/marpio/img-store/domain"
	"github.com/marpio/img-store/metadatastore"
	"github.com/spf13/afero"
)

func TestGetAll(t *testing.T) {
	s, _ := setup()
	p := "/path/to/file"
	s.Add(&domain.Photo{FileInfo: &domain.FileInfo{Path: p}})
	r, _ := s.GetAll()
	if len(r) != 1 {
		t.Errorf("Expected one result, got: %v", len(r))
	}
}

func TestExists(t *testing.T) {
	s, _ := setup()
	p := "/path/to/file"
	p2 := "/path/to/file2"
	ph1 := &domain.Photo{FileInfo: &domain.FileInfo{Path: p}}
	ph2 := &domain.Photo{FileInfo: &domain.FileInfo{Path: p2}}
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
	s.Add(&domain.Photo{FileInfo: &domain.FileInfo{Path: p}, Metadata: &domain.Metadata{CreatedAt: m}})
	r, _ := s.GetByDir(m)
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
	s.Add(&domain.Photo{FileInfo: &domain.FileInfo{Path: p}, Metadata: &domain.Metadata{CreatedAtMonth: m}})
	s.Add(&domain.Photo{FileInfo: &domain.FileInfo{Path: p2}, Metadata: &domain.Metadata{CreatedAtMonth: m2}})
	r, _ := s.GetMonths()
	if len(r) != 2 {
		t.Errorf("Expected 2 results, got: %v", len(r))
	}
	if !(r[0] == m2 && r[1] == m) {
		t.Errorf("Months not sorted.")
	}
}

func TestDelete(t *testing.T) {
	s, _ := setup()
	p := "/path/to/file"
	s.Add(&domain.Photo{FileInfo: &domain.FileInfo{Path: p}})
	s.Delete(p)
	r, _ := s.GetByPath(p)
	if len(r) > 0 {
		t.Errorf("Expected zero results, got: %v", len(r))
	}
}

func TestPersist(t *testing.T) {
	s, afs := setup()
	p := "/path/to/file"
	s.Add(&domain.Photo{FileInfo: &domain.FileInfo{Path: p}})

	s.Persist()
	dbPath := "domain.db"
	s2 := setup2(afs, dbPath)
	r, _ := s2.GetAll()
	if len(r) != 1 {
		t.Errorf("Expected one result, got: %v", len(r))
	}
}

func TestReload(t *testing.T) {
	dbPath := "domain.db"
	s, afs := setup()
	p := "/path/to/file"
	s.Add(&domain.Photo{FileInfo: &domain.FileInfo{Path: p}})
	s.Persist()

	afs.Rename(dbPath, "photo2.db")

	s2 := setup2(afs, dbPath)
	r, _ := s2.GetAll()
	if len(r) != 0 {
		t.Errorf("Expected 0 results, got: %v", len(r))
	}

	afs.Rename("photo2.db", dbPath)
	s2.Reload()
	r, _ = s2.GetAll()
	if len(r) != 1 {
		t.Errorf("Expected 1 result, got: %v", len(r))
	}
}

func setup() (metadatastore.Service, afero.Fs) {
	afs := afero.NewMemMapFs()
	b := remotestorage.New(filesystem.New(afs), crypto.NewService("b567ef1d391e8a10d94100faa34b7d28fdab13e3f51f94b8"))
	dbPath := "domain.db"

	s := New(b, dbPath)
	return s, afs
}

func setup2(afs afero.Fs, dbPath string) metadatastore.Service {
	b := remotestorage.New(filesystem.New(afs), crypto.NewService("b567ef1d391e8a10d94100faa34b7d28fdab13e3f51f94b8"))
	s := New(b, dbPath)
	return s
}

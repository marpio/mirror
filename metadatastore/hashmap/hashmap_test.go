package hashmap

import (
	"testing"
	"time"

	"github.com/marpio/img-store/file"
	"github.com/marpio/img-store/photo"
	"github.com/spf13/afero"
)

func TestGetAll(t *testing.T) {
	s, _ := setup()
	p := "/path/to/file"
	s.Add(&photo.Photo{FileInfo: &file.FileInfo{Path: p}})
	r, _ := s.GetAll()
	if len(r) != 1 {
		t.Errorf("Expected one result, got: %v", len(r))
	}
}

func TestGetByPath(t *testing.T) {
	s, _ := setup()
	p := "/path/to/file"
	p2 := "/path/to/file2"
	s.Add(&photo.Photo{FileInfo: &file.FileInfo{Path: p}})
	s.Add(&photo.Photo{FileInfo: &file.FileInfo{Path: p2}})
	r, _ := s.GetByPath(p)
	if len(r) != 1 {
		t.Errorf("Expected one result, got %v", len(r))
	}
	if r[0].Path != p {
		t.Errorf("Expected one result with path %v, got %v", p, r[0].Path)
	}
}

func TestGetByMonth(t *testing.T) {
	s, _ := setup()
	p := "/path/to/file"
	m := time.Date(2017, 5, 1, 0, 0, 0, 0, time.UTC)
	s.Add(&photo.Photo{FileInfo: &file.FileInfo{Path: p}, Metadata: &photo.Metadata{CreatedAtMonth: m}})
	r, _ := s.GetByMonth(m)
	if len(r) != 1 || r[0].CreatedAtMonth != m {
		t.Errorf("Expected one result, got: %v", len(r))
	}
}

func TestGetMonths(t *testing.T) {
	s, _ := setup()
	p := "/path/to/file"
	p2 := "/path/to/file2"
	m := time.Date(2017, 5, 1, 0, 0, 0, 0, time.UTC)
	m2 := time.Date(2017, 6, 1, 0, 0, 0, 0, time.UTC)
	s.Add(&photo.Photo{FileInfo: &file.FileInfo{Path: p}, Metadata: &photo.Metadata{CreatedAtMonth: m}})
	s.Add(&photo.Photo{FileInfo: &file.FileInfo{Path: p2}, Metadata: &photo.Metadata{CreatedAtMonth: m2}})
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
	s.Add(&photo.Photo{FileInfo: &file.FileInfo{Path: p}})
	s.Delete(p)
	r, _ := s.GetByPath(p)
	if len(r) > 0 {
		t.Errorf("Expected zero results, got: %v", len(r))
	}
}

func TestPersist(t *testing.T) {
	s, fs := setup()
	p := "/path/to/file"
	s.Add(&photo.Photo{FileInfo: &file.FileInfo{Path: p}})

	s.Persist()
	dbPath := "photo.db"
	s2 := New(fs, dbPath)
	r, _ := s2.GetAll()
	if len(r) != 1 {
		t.Errorf("Expected one result, got: %v", len(r))
	}
}

func TestReload(t *testing.T) {
	dbPath := "photo.db"
	s, fs := setup()
	p := "/path/to/file"
	s.Add(&photo.Photo{FileInfo: &file.FileInfo{Path: p}})
	s.Persist()

	fs.Rename(dbPath, "photo2.db")

	s2 := New(fs, dbPath)
	r, _ := s2.GetAll()
	if len(r) != 0 {
		t.Errorf("Expected 0 results, got: %v", len(r))
	}

	fs.Rename("photo2.db", dbPath)
	s2.Reload()
	r, _ = s2.GetAll()
	if len(r) != 1 {
		t.Errorf("Expected 1 result, got: %v", len(r))
	}
}

func setup() (*HashmapMetadataStore, afero.Fs) {
	dbPath := "photo.db"
	fs := afero.NewMemMapFs()
	s := New(fs, dbPath)
	return s, fs
}

package hashmap

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/marpio/img-store/domain"
	"github.com/marpio/img-store/repository"
)

type item struct {
	FileID      string `json:"id"`
	FileModTime string `json:"modTime"`
	Directory   string `json:"directory"`
}

func (it item) ID() string {
	return it.FileID
}

func (it item) ThumbID() string {
	return "thumb_" + it.ID()
}

func (it item) ModTimeHash() string {
	return it.FileModTime
}

func (it item) Dir() string {
	return it.Directory
}

type M map[string]map[string]*item

type hashmapStore struct {
	rs       domain.Storage
	data     M
	filename string
}

func New(rs domain.Storage, filename string) (repository.Service, error) {
	var decodedMetadata M
	exists := rs.Exists(filename)
	if !exists {
		decodedMetadata = make(M)
	} else {
		r, err := rs.NewReader(filename)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		dec := json.NewDecoder(r)
		if err := dec.Decode(&decodedMetadata); err != nil {
			decodedMetadata = make(M)
		}
	}
	return &hashmapStore{rs: rs, data: decodedMetadata, filename: filename}, nil
}

func (s *hashmapStore) Reload() error {
	var decodedMetadata M
	r, err := s.rs.NewReader(s.filename)
	if err != nil {
		return err
	}
	defer r.Close()
	dec := json.NewDecoder(r)
	if err := dec.Decode(&decodedMetadata); err != nil {
		s.data = make(M)
		return err
	}
	s.data = decodedMetadata
	return nil
}

func (s *hashmapStore) GetByDir(dir string) ([]domain.Item, error) {
	var res = make([]domain.Item, 0)
	if p, ok := s.data[dir]; ok {
		for _, x := range p {
			res = append(res, x)
		}
	}
	return res, nil
}

func (s *hashmapStore) GetByDirAndId(dir, id string) (domain.Item, error) {
	if p, ok := s.data[dir]; ok {
		if f, ok := p[id]; ok {
			return f, nil
		}
	}
	return nil, nil
}

func (s *hashmapStore) Exists(id string) (bool, error) {
	var found *item
	for _, d := range s.data {
		if p, ok := d[id]; ok {
			found = p
		}
	}
	return (found != nil), nil
}

func (s *hashmapStore) GetModTime(id string) (string, error) {
	found := s.getByID(id)
	if found != nil {
		return found.FileModTime, nil
	}
	return "", fmt.Errorf("element with id: %v not found", id)
}

func (s *hashmapStore) getByID(id string) *item {
	for _, d := range s.data {
		if p, ok := d[id]; ok {
			return p
		}
	}
	return nil
}

func (s *hashmapStore) Add(it domain.Item) error {
	x := &item{FileID: it.ID(), Directory: it.Dir(), FileModTime: it.ModTimeHash()}
	if _, ok := s.data[x.Directory]; !ok {
		s.data[x.Directory] = make(map[string]*item)
	}
	if _, ok := s.data[x.Directory][x.FileID]; !ok {
		s.data[x.Directory][x.FileID] = x
	}
	return nil
}

func (s *hashmapStore) Persist() error {
	w := s.rs.NewWriter(s.filename)
	defer w.Close()
	en := json.NewEncoder(w)
	en.SetIndent("", "    ")
	en.Encode(s.data)
	return nil
}

func (s *hashmapStore) Delete(id string) error {
	for _, d := range s.data {
		if _, ok := d[id]; ok {
			delete(d, id)
			return nil
		}
	}
	return fmt.Errorf("could not find %v", id)
}

func (s *hashmapStore) GetDirs() ([]string, error) {
	ds := make(sort.StringSlice, 0)
	for k := range s.data {
		ds = append(ds, k)
	}
	sort.Sort(sort.Reverse(ds[:]))

	return ds[:], nil
}

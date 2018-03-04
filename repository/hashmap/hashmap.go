package hashmap

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/marpio/mirror"
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

func (it item) Dir() string {
	return it.Directory
}

type m map[string]map[string]*item

type hashmapStore struct {
	rs       mirror.Storage
	data     m
	filename string
	mutex    sync.RWMutex
}

func New(ctx context.Context, rs mirror.Storage, filename string) (mirror.MetadataRepo, error) {
	var decodedMetadata m
	exists := rs.Exists(ctx, filename)
	if !exists {
		decodedMetadata = make(m)
	} else {
		r, err := rs.NewReader(ctx, filename)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		dec := json.NewDecoder(r)
		if err := dec.Decode(&decodedMetadata); err != nil {
			decodedMetadata = make(m)
		}
	}
	return &hashmapStore{rs: rs, data: decodedMetadata, filename: filename}, nil
}

func (s *hashmapStore) Reload(ctx context.Context) error {
	var decodedMetadata m
	r, err := s.rs.NewReader(ctx, s.filename)
	if err != nil {
		return err
	}
	defer r.Close()
	dec := json.NewDecoder(r)
	if err := dec.Decode(&decodedMetadata); err != nil {
		s.data = make(m)
		return err
	}
	s.data = decodedMetadata
	return nil
}

func (s *hashmapStore) GetByDir(dir string) ([]mirror.Item, error) {
	var res = make([]mirror.Item, 0)
	if p, ok := s.data[dir]; ok {
		for _, x := range p {
			res = append(res, x)
		}
	}
	return res, nil
}

func (s *hashmapStore) GetByDirAndId(dir, id string) (mirror.Item, error) {
	if p, ok := s.data[dir]; ok {
		if f, ok := p[id]; ok {
			return f, nil
		}
	}
	return nil, nil
}

func (s *hashmapStore) Exists(id string) (bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	for _, d := range s.data {
		if _, ok := d[id]; ok {
			return true, nil
		}
	}
	return false, nil
}

func (s *hashmapStore) getByID(id string) *item {
	for _, d := range s.data {
		if p, ok := d[id]; ok {
			return p
		}
	}
	return nil
}

func (s *hashmapStore) Add(it mirror.Item) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	x := &item{FileID: it.ID(), Directory: it.Dir()}
	if _, ok := s.data[x.Directory]; !ok {
		s.data[x.Directory] = make(map[string]*item)
	}
	if _, ok := s.data[x.Directory][x.FileID]; !ok {
		s.data[x.Directory][x.FileID] = x
	}
	return nil
}

func (s *hashmapStore) Persist(ctx context.Context) error {
	w := s.rs.NewWriter(ctx, s.filename)
	defer w.Close()
	en := json.NewEncoder(w)
	en.SetIndent("", "    ")
	en.Encode(s.data)
	return nil
}

func (s *hashmapStore) GetAll() []mirror.Item {
	var res = make([]mirror.Item, 0)
	for _, d := range s.data {
		for _, p := range d {
			res = append(res, p)
		}
	}
	return res
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

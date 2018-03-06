package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/marpio/mirror"
)

type entry struct {
	FileID      string `json:"id"`
	FileModTime string `json:"modTime"`
	Directory   string `json:"directory"`
}

func (it entry) ID() string {
	return it.FileID
}

func (it entry) ThumbID() string {
	return "thumb_" + it.ID()
}

func (it entry) Dir() string {
	return it.Directory
}

type m map[string]map[string]*entry

type HashmapStore struct {
	rs       mirror.Storage
	data     m
	filename string
	mutex    sync.RWMutex
}

func NewHashmap(ctx context.Context, rs mirror.Storage, filename string) (*HashmapStore, error) {
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
	return &HashmapStore{rs: rs, data: decodedMetadata, filename: filename}, nil
}

func (s *HashmapStore) Reload(ctx context.Context) error {
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

func (s *HashmapStore) GetByDir(dir string) ([]mirror.RemotePhoto, error) {
	var res = make([]mirror.RemotePhoto, 0)
	if p, ok := s.data[dir]; ok {
		for _, x := range p {
			res = append(res, x)
		}
	}
	return res, nil
}

func (s *HashmapStore) GetByDirAndId(dir, id string) (mirror.RemotePhoto, error) {
	if p, ok := s.data[dir]; ok {
		if f, ok := p[id]; ok {
			return f, nil
		}
	}
	return nil, nil
}

func (s *HashmapStore) Exists(id string) (bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	for _, d := range s.data {
		if _, ok := d[id]; ok {
			return true, nil
		}
	}
	return false, nil
}

func (s *HashmapStore) getByID(id string) *entry {
	for _, d := range s.data {
		if p, ok := d[id]; ok {
			return p
		}
	}
	return nil
}

func (s *HashmapStore) Add(it mirror.RemotePhoto) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	x := &entry{FileID: it.ID(), Directory: it.Dir()}
	if _, ok := s.data[x.Directory]; !ok {
		s.data[x.Directory] = make(map[string]*entry)
	}
	if _, ok := s.data[x.Directory][x.FileID]; !ok {
		s.data[x.Directory][x.FileID] = x
	}
	return nil
}

func (s *HashmapStore) Persist(ctx context.Context) error {
	w := s.rs.NewWriter(ctx, s.filename)
	defer w.Close()
	en := json.NewEncoder(w)
	en.SetIndent("", "    ")
	en.Encode(s.data)
	return nil
}

func (s *HashmapStore) GetAll() []mirror.RemotePhoto {
	var res = make([]mirror.RemotePhoto, 0)
	for _, d := range s.data {
		for _, p := range d {
			res = append(res, p)
		}
	}
	return res
}

func (s *HashmapStore) Delete(id string) error {
	for _, d := range s.data {
		if _, ok := d[id]; ok {
			delete(d, id)
			return nil
		}
	}
	return fmt.Errorf("could not find %v", id)
}

func (s *HashmapStore) GetDirs() ([]string, error) {
	ds := make(sort.StringSlice, 0)
	for k := range s.data {
		ds = append(ds, k)
	}
	sort.Sort(sort.Reverse(ds[:]))

	return ds[:], nil
}

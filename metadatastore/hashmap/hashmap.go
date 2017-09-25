package hashmap

import (
	"encoding/gob"
	"sort"
	"time"

	"github.com/marpio/img-store/photo"
	"github.com/spf13/afero"
)

type HashmapMetadataStore struct {
	fs         afero.Fs
	data       map[string]*photo.Photo
	dbFilePath string
}

type timeSlice []time.Time

func NewHashmapMetadataStore(fs afero.Fs, dbFilePath string) *HashmapMetadataStore {
	var decodedMetadata map[string]*photo.Photo
	exists, err := afero.Exists(fs, dbFilePath)
	if err != nil || !exists {
		decodedMetadata = make(map[string]*photo.Photo)
	} else {
		f, err := fs.Open(dbFilePath)
		if err != nil {
			return nil
		}
		defer f.Close()
		dec := gob.NewDecoder(f)
		if err := dec.Decode(&decodedMetadata); err != nil {
			decodedMetadata = make(map[string]*photo.Photo)
		}
	}
	return &HashmapMetadataStore{fs: fs, data: decodedMetadata, dbFilePath: dbFilePath}
}

func (datastore *HashmapMetadataStore) Reload() error {
	var decodedMetadata map[string]*photo.Photo
	f, err := datastore.fs.Open(datastore.dbFilePath)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&decodedMetadata); err != nil {
		datastore.data = make(map[string]*photo.Photo)
		return err
	} else {
		datastore.data = decodedMetadata
		return nil
	}
}

func (datastore *HashmapMetadataStore) GetAll() (all []*photo.Photo, err error) {
	for _, p := range datastore.data {
		all = append(all, p)
	}
	return all, nil
}

func (datastore *HashmapMetadataStore) GetByMonth(month time.Time) ([]*photo.Photo, error) {
	var res = []*photo.Photo{}

	for _, p := range datastore.data {
		if p.CreatedAtMonth == month {
			res = append(res, p)
		}
	}
	return res, nil
}

func (datastore *HashmapMetadataStore) GetByPath(path string) ([]*photo.Photo, error) {
	res := make([]*photo.Photo, 0)
	if p, ok := datastore.data[path]; ok {
		res = append(res, p)
	}
	return res, nil
}

func (datastore *HashmapMetadataStore) Add(photo *photo.Photo) error {
	datastore.data[photo.Path] = photo
	return nil
}

func (datastore *HashmapMetadataStore) Commit() error {
	f, err := datastore.fs.Create(datastore.dbFilePath)
	if err != nil {
		return err
	}
	defer f.Close()
	e := gob.NewEncoder(f)
	if err := e.Encode(datastore.data); err != nil {
		return err
	}
	return nil
}

func (datastore *HashmapMetadataStore) Delete(imgID string) error {
	if _, ok := datastore.data[imgID]; ok {
		delete(datastore.data, imgID)
	}
	return nil
}

func (datastore *HashmapMetadataStore) GetMonths() ([]time.Time, error) {
	months := make([]time.Time, 0)
	var m = make(map[time.Time]struct{})
	for _, p := range datastore.data {
		if _, ok := m[p.CreatedAtMonth]; !ok {
			m[p.CreatedAtMonth] = struct{}{}
			months = append(months, p.CreatedAtMonth)
		}
	}
	sort.Sort(sort.Reverse(timeSlice(months)))
	return months, nil
}

// Forward request for length
func (p timeSlice) Len() int {
	return len(p)
}

// Define compare
func (p timeSlice) Less(i, j int) bool {
	return p[i].Before(p[j])
}

// Define swap over an array
func (p timeSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

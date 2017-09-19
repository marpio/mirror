package hashmap

import (
	"encoding/gob"
	"os"
	"sort"
	"time"

	"github.com/marpio/img-store/photo"
)

type timeSlice []time.Time

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

type HashmapMetadataStore struct {
	data map[string]*photo.Photo
}

var dbName string

func NewHashmapMetadataStore(dbFileName string) *HashmapMetadataStore {
	dbName = dbFileName
	var decodedMetadata map[string]*photo.Photo
	if _, err := os.Stat(dbName); os.IsNotExist(err) {
		decodedMetadata = make(map[string]*photo.Photo)
	} else {
		f, err := os.Open(dbName)
		if err != nil {
			return nil
		}
		defer f.Close()
		dec := gob.NewDecoder(f)
		if err := dec.Decode(&decodedMetadata); err != nil {
			decodedMetadata = make(map[string]*photo.Photo)
		}
	}
	return &HashmapMetadataStore{data: decodedMetadata}
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

func (datastore *HashmapMetadataStore) GetByID(id string) ([]*photo.Photo, error) {
	res := make([]*photo.Photo, 0)
	if p, ok := datastore.data[id]; ok {
		res = append(res, p)
	}
	return res, nil
}

func (datastore *HashmapMetadataStore) Add(photo *photo.Photo) error {
	datastore.data[photo.PathHash] = photo
	return nil
}

func (datastore *HashmapMetadataStore) Commit() error {
	f, err := os.Create(dbName)
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
	sort.Sort(timeSlice(months))
	return months, nil
}

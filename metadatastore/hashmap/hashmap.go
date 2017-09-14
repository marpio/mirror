package hashmap

import (
	"encoding/gob"
	"os"
	"time"

	"github.com/marpio/img-store/photo"
)

type HashmapMetadataStore struct {
	data map[string]*photo.Photo
}

const dbName string = "photo.db"

func NewHashmapMetadataStore() *HashmapMetadataStore {
	var decodedMetadata map[string]*photo.Photo
	if _, err := os.Stat("photo.db"); os.IsNotExist(err) {
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
		if p.CreatedAt == month {
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
	delete(datastore.data, imgID)
	return nil
}

func (datastore *HashmapMetadataStore) GetMonths() ([]*time.Time, error) {
	var res = make(map[time.Time]interface{})
	for _, p := range datastore.data {
		if _, ok := res[p.CreatedAtMonth]; !ok {
			res[p.CreatedAtMonth] = nil
		}
	}
	list := make([]*time.Time, len(res))
	for t, _ := range res {
		list = append(list, &t)
	}
	return list, nil
}

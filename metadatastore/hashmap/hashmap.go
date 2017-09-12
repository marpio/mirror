package hashmap

import (
	"encoding/gob"
	"os"
	"sync"
	"time"

	"github.com/marpio/img-store/metadatastore"
)

type HashmapMetadataStore struct {
	data *sync.Map
}

const dbName string = "photo.db"

func NewHashmapMetadataStore() *HashmapMetadataStore {
	var decodedMetadata []*metadatastore.Image
	if _, err := os.Stat("photo.db"); os.IsNotExist(err) {
		decodedMetadata = make([]*metadatastore.Image, 0)
	} else {
		f, err := os.Open(dbName)
		if err != nil {
			return nil
		}
		defer f.Close()
		dec := gob.NewDecoder(f)
		if err := dec.Decode(&decodedMetadata); err != nil {
			decodedMetadata = make([]*metadatastore.Image, 0)
		}
	}
	d := sync.Map{}
	for _, m := range decodedMetadata {
		d.Store(m.ImgID, m)
	}
	return &HashmapMetadataStore{data: &d}
}

func (datastore *HashmapMetadataStore) GetAll() ([]*metadatastore.Image, error) {
	var existingImgs = []*metadatastore.Image{}
	datastore.data.Range(func(k, v interface{}) bool {
		img, _ := v.(*metadatastore.Image)
		existingImgs = append(existingImgs, img)
		return true
	})
	return existingImgs, nil
}

func (datastore *HashmapMetadataStore) GetByMonth(month time.Time) ([]*metadatastore.Image, error) {
	var existingImgs = []*metadatastore.Image{}
	datastore.data.Range(func(k, v interface{}) bool {
		img, _ := v.(*metadatastore.Image)
		if img.CreatedAtMonth == month {
			existingImgs = append(existingImgs, img)
		}
		return true
	})
	return existingImgs, nil
}

func (datastore *HashmapMetadataStore) GetByID(imgID string) ([]*metadatastore.Image, error) {
	r := make([]*metadatastore.Image, 0)
	v, ok := datastore.data.Load(imgID)
	if !ok {
		return r, nil
	}
	res, _ := v.(*metadatastore.Image)
	r = append(r, res)
	return r, nil
}

func (datastore *HashmapMetadataStore) Save(metadataEntities []*metadatastore.Image) (ok bool) {
	f, err := os.Create(dbName)
	if err != nil {
		return false
	}
	defer f.Close()
	e := gob.NewEncoder(f)
	if err := e.Encode(metadataEntities); err != nil {
		return false
	}
	return true
}

func (datastore *HashmapMetadataStore) Delete(imgID string) error {
	datastore.data.Delete(imgID)
	return nil
}

func (datastore *HashmapMetadataStore) GetMonths() ([]*time.Time, error) {
	var res = []*time.Time{}

	return res, nil
}

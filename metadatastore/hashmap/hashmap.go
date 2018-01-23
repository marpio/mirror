package hashmap

import (
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"sort"
	"time"

	"github.com/marpio/img-store/metadatastore"
	"github.com/marpio/img-store/remotestorage"

	"github.com/marpio/img-store/entity"
)

type hashmapStore struct {
	rs         remotestorage.Service
	data       map[string]*entity.Photo
	dbFileName string
}

type timeSlice []time.Time

func New(rs remotestorage.Service, dbFileName string) metadatastore.Service {
	var decodedMetadata map[string]*entity.Photo
	exists := rs.Exists(dbFileName)
	log.Printf("db: %v", dbFileName)
	if !exists {
		decodedMetadata = make(map[string]*entity.Photo)
	} else {
		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			rs.DownloadDecrypted(pw, dbFileName)
		}()
		dec := gob.NewDecoder(pr)
		if err := dec.Decode(&decodedMetadata); err != nil {
			decodedMetadata = make(map[string]*entity.Photo)
		}
	}
	return &hashmapStore{rs: rs, data: decodedMetadata, dbFileName: dbFileName}
}

func (datastore *hashmapStore) Reload() error {
	var decodedMetadata map[string]*entity.Photo
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		datastore.rs.DownloadDecrypted(pw, datastore.dbFileName)
	}()
	dec := gob.NewDecoder(pr)
	if err := dec.Decode(&decodedMetadata); err != nil {
		datastore.data = make(map[string]*entity.Photo)
		return err
	}
	datastore.data = decodedMetadata
	return nil
}

func (datastore *hashmapStore) GetAll() (all []*entity.Photo, err error) {
	for _, p := range datastore.data {
		all = append(all, p)
	}
	return all, nil
}

func (datastore *hashmapStore) GetByMonth(month time.Time) ([]*entity.Photo, error) {
	var res = []*entity.Photo{}

	for _, p := range datastore.data {
		if p.CreatedAtMonth == month {
			res = append(res, p)
		}
	}
	return res, nil
}

func (datastore *hashmapStore) GetByPath(path string) ([]*entity.Photo, error) {
	res := make([]*entity.Photo, 0)
	if p, ok := datastore.data[path]; ok {
		res = append(res, p)
	}
	return res, nil
}

func (datastore *hashmapStore) Add(photo *entity.Photo) error {
	datastore.data[photo.Path] = photo
	return nil
}

func (datastore *hashmapStore) Persist() error {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		gob.NewEncoder(pw).Encode(datastore.data)
	}()
	if err := datastore.rs.UploadEncrypted(datastore.dbFileName, pr); err != nil {
		return fmt.Errorf("error persisting db: %v", err)
	}
	if err := pr.Close(); err != nil {
		return fmt.Errorf("error closing reader: %v", err)
	}
	return nil
}

func (datastore *hashmapStore) Delete(path string) error {
	if _, ok := datastore.data[path]; ok {
		delete(datastore.data, path)
	}
	return nil
}

func (datastore *hashmapStore) GetMonths() ([]time.Time, error) {
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

package metadatastore

import (
	"time"

	"github.com/marpio/img-store/photo"
)

type DataStore interface {
	DataStoreReader
	DataStoreWriter
}

type DataStoreWriter interface {
	Add(photo *photo.Photo) error
	Delete(path string) error
	Persist() error
}

type DataStoreReader interface {
	GetAll() ([]*photo.Photo, error)
	GetByPath(path string) ([]*photo.Photo, error)
	GetByMonth(month time.Time) ([]*photo.Photo, error)
	GetMonths() ([]time.Time, error)
	Reload() error
}

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
	Delete(imgID string) error
	Commit() error
}

type DataStoreReader interface {
	GetAll() ([]*photo.Photo, error)
	GetByID(imgID string) ([]*photo.Photo, error)
	GetByMonth(month time.Time) ([]*photo.Photo, error)
	GetMonths() ([]time.Time, error)
}

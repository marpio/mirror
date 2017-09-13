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
	Save(metadataEntities []*photo.Photo) (ok bool)
	Delete(imgID string) error
}

type DataStoreReader interface {
	GetAll() ([]*photo.Photo, error)
	GetByID(imgID string) ([]*photo.Photo, error)
	GetByMonth(month time.Time) ([]*photo.Photo, error)
	GetMonths() ([]*time.Time, error)
}

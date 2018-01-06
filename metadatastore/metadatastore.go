package metadatastore

import (
	"time"

	"github.com/marpio/img-store/photo"
)

type Service interface {
	ReaderService
	WriterService
}

type WriterService interface {
	Add(photo *photo.Photo) error
	Delete(path string) error
	Persist() error
}

type ReaderService interface {
	GetAll() ([]*photo.Photo, error)
	GetByPath(path string) ([]*photo.Photo, error)
	GetByMonth(month time.Time) ([]*photo.Photo, error)
	GetMonths() ([]time.Time, error)
	Reload() error
}

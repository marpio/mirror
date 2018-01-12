package metadatastore

import (
	"time"

	"github.com/marpio/img-store/entity"
)

type Service interface {
	ReaderService
	WriterService
}

type WriterService interface {
	Add(photo *entity.Photo) error
	Delete(path string) error
	Persist() error
}

type ReaderService interface {
	GetAll() ([]*entity.Photo, error)
	GetByPath(path string) ([]*entity.Photo, error)
	GetByMonth(month time.Time) ([]*entity.Photo, error)
	GetMonths() ([]time.Time, error)
	Reload() error
}

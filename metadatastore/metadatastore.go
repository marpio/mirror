package metadatastore

import (
	"time"

	"github.com/marpio/img-store/photo"
)

//type Image struct {
//	ID          string    `db:"img_id"`
//	Path           string    `db:"path"`
//	CreatedAt      time.Time `db:"created_at"`
//	CreatedAtMonth time.Time `db:"created_at_month"`
//	ModTime        time.Time `db:"mod_time"`
//	ImgName        string    `db:"name"`
//	ThumbnailName  string    `db:"thumbnail_name"`
//}

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

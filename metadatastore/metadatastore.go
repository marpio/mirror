package metadatastore

import "time"

type Image struct {
	ImgID          string    `db:"img_id"`
	CreatedAt      time.Time `db:"created_at"`
	CreatedAtMonth time.Time `db:"created_at_month"`
	ImgHash        string    `db:"img_hash"`
	ImgName        string    `db:"img_name"`
	ThumbnailName  string    `db:"thumbnail_name"`
}

type DataStore interface {
	DataStoreReader
	DataStoreWriter
}

type DataStoreWriter interface {
	Insert(imgEntity *Image) error
	Delete(imgID string) error
}

type DataStoreReader interface {
	GetAll() ([]*Image, error)
	GetByID(imgID string) ([]*Image, error)
}

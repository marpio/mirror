package entity

import (
	"time"

	"github.com/marpio/img-store/fs"
)

type Metadata struct {
	Name           string
	ThumbnailName  string
	CreatedAt      time.Time
	CreatedAtMonth time.Time
}

type PhotoWithThumb struct {
	*Photo
	Thumbnail []byte
}

type Photo struct {
	*fs.FileInfo
	*Metadata
}

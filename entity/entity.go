package entity

import (
	"time"

	"github.com/marpio/img-store/fsutils"
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
	*fsutils.FileInfo
	*Metadata
}

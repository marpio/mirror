package photo

import (
	"time"

	"github.com/marpio/img-store/file"
)

type Metadata struct {
	Name           string
	ThumbnailName  string
	CreatedAt      time.Time
	CreatedAtMonth time.Time
}

type FileWithMetadata struct {
	*file.FileInfo
	*Metadata
	Thumbnail []byte
}

type Photo struct {
	*file.FileInfo
	*Metadata
}

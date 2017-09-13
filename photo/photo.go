package photo

import (
	"time"

	"github.com/marpio/img-store/file"
)

type Metadata struct {
	CreatedAt time.Time
	Thumbnail []byte
}

type FileWithMetadata struct {
	*file.FileInfo
	*Metadata
}

type Photo struct {
	*FileWithMetadata
	CreatedAtMonth time.Time
	Name           string
	ThumbnailName  string
}

package photo

import (
	"io"
	"time"

	"github.com/marpio/img-store/file"
)

type File interface {
	io.ReadWriteCloser
	io.Seeker
}

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
	CreatedAtMonth time.Time `db:"created_at_month"`
	Name           string    `db:"name"`
	ThumbnailName  string    `db:"thumbnail_name"`
}

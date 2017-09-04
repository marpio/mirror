package file

import "io"

type File interface {
	io.ReadWriteCloser
	io.Seeker
}

package repository

import "github.com/marpio/img-store/domain"

type Service interface {
	ReaderService
	WriterService
}

type WriterService interface {
	Add(item domain.Item) error
	Delete(id string) error
	Persist() error
}

type ReaderService interface {
	Exists(id string) (bool, error)
	GetModTime(id string) (string, error)
	GetByDir(name string) ([]domain.Item, error)
	GetByDirAndId(dir, id string) (domain.Item, error)
	GetDirs() ([]string, error)
	Reload() error
}

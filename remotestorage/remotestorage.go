package remotestorage

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/marpio/img-store/crypto"
	"github.com/marpio/img-store/entity"
	"github.com/marpio/img-store/fs"
)

type Service interface {
	ReaderService
	WriterService
}

type ReaderService interface {
	DownloadDecrypted(dst io.Writer, fileName string)
}

type WriterService interface {
	UploadEncrypted(fileName string, reader io.Reader) error
	Delete(fileName string) error
}

type Backend struct {
	ReadFn        func(string) io.ReadCloser
	WriteFn       func(string) io.WriteCloser
	DeleteFn      func(string) error
	EncryptionKey string
	CryptoSrv     crypto.Service
}

func (b *Backend) DownloadDecrypted(dst io.Writer, fileName string) {
	r := b.ReadFn(fileName)
	//r.ConcurrentDownloads = downloads
	defer r.Close()

	err := b.CryptoSrv.Decrypt(dst, b.EncryptionKey, r)
	if err != nil {
		log.Print(err)
		panic(err)
	}
}

func (b *Backend) UploadEncrypted(fileName string, reader io.Reader) error {
	w := b.WriteFn(fileName)
	b.CryptoSrv.Encrypt(w, b.EncryptionKey, reader)
	if err := w.Close(); err != nil {
		return err
	}
	return nil
}

func (b *Backend) Delete(fileName string) error {
	if err := b.DeleteFn(fileName); err != nil {
		return err
	}
	return nil
}

const maxConcurrentUploads = 10

func UploadPhotos(metadataStream <-chan *entity.PhotoWithThumb, fileReader fs.FileReaderFn, remotestorage Service) <-chan *entity.PhotoWithThumb {
	uploadedPhotosStream := make(chan *entity.PhotoWithThumb)

	go func() {
		limiter := make(chan struct{}, maxConcurrentUploads)
		var wg sync.WaitGroup
		defer close(uploadedPhotosStream)
		for metaData := range metadataStream {
			limiter <- struct{}{}
			wg.Add(1)
			go func(m *entity.PhotoWithThumb) {
				defer wg.Done()
				defer func() { <-limiter }()
				err := uploadPhoto(m, fileReader, remotestorage)
				if err != nil {
					log.Printf("error uploading photo or thumbnail: %v", err)
					return
				}
				uploadedPhotosStream <- m
			}(metaData)
		}
		wg.Wait()
	}()
	return uploadedPhotosStream
}

func uploadPhoto(img *entity.PhotoWithThumb, fileReader fs.FileReaderFn, remotestorage Service) error {
	f, err := fileReader(img.Path)
	if err != nil {
		return fmt.Errorf("error opening file %v: %v", img.Path, err)
	}
	defer f.Close()

	if err := remotestorage.UploadEncrypted(img.ThumbnailName, bytes.NewReader(img.Thumbnail)); err != nil {
		return fmt.Errorf("error uploading thumbnail: %v - path: %v : %v", img.ThumbnailName, img.Path, err)
	}

	if err := remotestorage.UploadEncrypted(img.Name, f); err != nil {
		return fmt.Errorf("error uploading photo: %v - img: %v: %v", img.Name, img.Path, err)
	}

	return nil
}

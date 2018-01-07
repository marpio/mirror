package fileupload

import (
	"bytes"
	"fmt"
	"log"
	"sync"

	"github.com/marpio/img-store/file"
	"github.com/marpio/img-store/filestore"
	"github.com/marpio/img-store/photo"
)

const maxConcurrentUploads = 10

func UploadFiles(metadataStream <-chan *photo.FileWithMetadata, fileReader func(string) (file.File, error), fileStore filestore.Service) <-chan *photo.Photo {
	uploadedPhotosStream := make(chan *photo.Photo)

	go func() {
		limiter := make(chan struct{}, maxConcurrentUploads)
		var wg sync.WaitGroup
		defer close(uploadedPhotosStream)
		for metaData := range metadataStream {
			limiter <- struct{}{}
			wg.Add(1)
			go func(m *photo.FileWithMetadata) {
				defer wg.Done()
				defer func() { <-limiter }()
				p, err := uploadFile(m, fileReader, fileStore)
				if err != nil {
					log.Printf("error uploading photo or thumbnail: %v", err)
					return
				}
				uploadedPhotosStream <- p
			}(metaData)
		}
		wg.Wait()
	}()
	return uploadedPhotosStream
}

func uploadFile(img *photo.FileWithMetadata, fileReader func(string) (file.File, error), fileStore filestore.Service) (*photo.Photo, error) {
	f, err := fileReader(img.Path)
	if err != nil {
		return nil, fmt.Errorf("error opening file %v - err msg: %v", img.Path, err)
	}
	defer f.Close()

	if err := fileStore.UploadEncrypted(img.ThumbnailName, bytes.NewReader(img.Thumbnail)); err != nil {
		return nil, fmt.Errorf("error uploading thumbnail: %v - path: %v : %v", img.ThumbnailName, img.Path, err)
	}

	if err := fileStore.UploadEncrypted(img.Name, f); err != nil {
		return nil, fmt.Errorf("error uploading photo: %v - img: %v: %v", img.Name, img.Path, err)
	}

	p := &photo.Photo{FileInfo: img.FileInfo, Metadata: img.Metadata}

	return p, nil
}

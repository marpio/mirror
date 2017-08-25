package filestore

import (
	"bytes"
	"context"
	"io"
	"log"

	"github.com/kurin/blazer/b2"
)

type FileStore interface {
	Upload(imgFileName string, payload []byte) error
	//download(encryptionKey, src, dst string)
}

type b2Store struct {
	bucket *b2.Bucket
	ctx    context.Context
}

// NewFilesStore creates new FilesStore instance
func NewFileStore(ctx context.Context, b2id string, b2key string, bucketName string) FileStore {
	b2Client, err := b2.NewClient(ctx, b2id, b2key)
	if err != nil {
		log.Fatal(err)
	}

	bucket, err := b2Client.Bucket(ctx, bucketName)
	if err != nil {
		log.Fatal(err)
	}
	return &b2Store{bucket: bucket, ctx: ctx}
}

//func (b2 *b2Store) download(encryptionKey, src, dst string) {
//	r := b2.bucket.Object(src).NewReader(b2.ctx)
//	defer r.Close()
//
//	var b bytes.Buffer
//	writer := bufio.NewWriter(&b)
//	if _, err := io.Copy(writer, r); err != nil {
//		log.Fatal("Booom!!!")
//	}
//	writer.Flush()
//	encryptedData := b.Bytes()
//	decryptedData, _ := decrypt(encryptionKey, encryptedData)
//	f, err := os.Create(dst)
//	if err != nil {
//		log.Fatal("Booom!!!")
//	}
//	//r.ConcurrentDownloads = downloads
//	if _, err := io.Copy(f, bytes.NewReader(decryptedData)); err != nil {
//		log.Fatal("Booom!!!")
//	}
//}

func (b2 *b2Store) Upload(imgFileName string, payload []byte) error {
	imgObj := b2.bucket.Object(imgFileName)
	b2Writer := imgObj.NewWriter(b2.ctx)
	reader := bytes.NewReader(payload)
	if _, err := io.Copy(b2Writer, reader); err != nil {
		return err
	}
	if err := b2Writer.Close(); err != nil {
		return err
	}
	return nil
}

package b2

import (
	"context"
	"io"
	"log"

	"github.com/kurin/blazer/b2"
	"github.com/marpio/img-store/crypto"
)

type B2Store struct {
	bucket *b2.Bucket
	ctx    context.Context
}

// NewB2Store creates new NewB2Store instance
func NewB2Store(ctx context.Context, b2id string, b2key string, bucketName string) *B2Store {
	b2Client, err := b2.NewClient(ctx, b2id, b2key)
	if err != nil {
		log.Fatal(err)
	}

	bucket, err := b2Client.Bucket(ctx, bucketName)
	if err != nil {
		log.Fatal(err)
	}
	return &B2Store{bucket: bucket, ctx: ctx}
}

func (b2 *B2Store) DownloadDecrypted(dst io.Writer, encryptionKey, fileName string) {
	r := b2.bucket.Object(fileName).NewReader(b2.ctx)
	//r.ConcurrentDownloads = downloads
	defer r.Close()

	err := crypto.Decrypt(dst, encryptionKey, r)
	if err != nil {
		log.Print(err)
	}
}

func (b2 *B2Store) UploadEncrypted(fileName string, reader io.Reader, encryptionKey string) error {
	imgObj := b2.bucket.Object(fileName)
	b2Writer := imgObj.NewWriter(b2.ctx)
	crypto.Encrypt(b2Writer, encryptionKey, reader)
	if err := b2Writer.Close(); err != nil {
		return err
	}
	return nil
}

func (b2 *B2Store) Delete(fileName string) error {
	return nil
}

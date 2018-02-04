package b2

import (
	"context"
	"io"
	"log"

	"github.com/kurin/blazer/b2"
	"github.com/marpio/img-store/domain"
)

type b2Backend struct {
	ctx    context.Context
	bucket *b2.Bucket
}

func New(ctx context.Context, b2id, b2key, bucketName string) domain.Storage {
	bucket := newB2Bucket(ctx, b2id, b2key, bucketName)
	return &b2Backend{ctx: ctx, bucket: bucket}
}

func (b *b2Backend) NewReader(fileName string) (io.ReadCloser, error) {
	rd := b.bucket.Object(fileName).NewReader(b.ctx)
	return rd, nil
}

func (b *b2Backend) NewWriter(fileName string) io.WriteCloser {
	wr := b.bucket.Object(fileName).NewWriter(b.ctx)
	return wr
}

func (b *b2Backend) Delete(fileName string) error {
	if err := b.bucket.Object(fileName).Delete(b.ctx); err != nil {
		return err
	}
	return nil
}

func (b *b2Backend) Exists(fileName string) bool {
	_, err := b.bucket.Object(fileName).Attrs(b.ctx)
	if err != nil {
		return b2.IsNotExist(err)
	}
	return true
}

func newB2Bucket(ctx context.Context, b2id string, b2key string, bucketName string) *b2.Bucket {
	b2Client, err := b2.NewClient(ctx, b2id, b2key)
	if err != nil {
		log.Fatal(err)
	}
	bucket, err := b2Client.Bucket(ctx, bucketName)
	if err != nil {
		log.Fatal(err)
	}
	return bucket
}

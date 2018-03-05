package remotebackend

import (
	"context"
	"io"
	"log"

	"github.com/kurin/blazer/b2"
)

type B2 struct {
	ctx    context.Context
	bucket *b2.Bucket
}

func NewB2(ctx context.Context, b2id, b2key, bucketName string) *B2 {
	bucket := newB2Bucket(ctx, b2id, b2key, bucketName)
	return &B2{ctx: ctx, bucket: bucket}
}

func (b *B2) NewReader(ctx context.Context, fileName string) (io.ReadCloser, error) {
	rd := b.bucket.Object(fileName).NewReader(ctx)
	return rd, nil
}

func (b *B2) NewWriter(ctx context.Context, fileName string) io.WriteCloser {
	wr := b.bucket.Object(fileName).NewWriter(ctx)
	return wr
}

func (b *B2) Delete(ctx context.Context, fileName string) error {
	return b.bucket.Object(fileName).Delete(ctx)
}

func (b *B2) Exists(ctx context.Context, fileName string) bool {
	_, err := b.bucket.Object(fileName).Attrs(ctx)
	if err != nil {
		return !b2.IsNotExist(err)
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

package b2

import (
	"context"
	"io"
	"log"

	"github.com/kurin/blazer/b2"
)

func NewB2Bucket(ctx context.Context, b2id string, b2key string, bucketName string) *b2.Bucket {
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

func ReaderProviderFactory(ctx context.Context, bucket *b2.Bucket) func(string) io.ReadCloser {
	return func(fileName string) io.ReadCloser {
		r := bucket.Object(fileName).NewReader(ctx)
		return r
	}
}

func WriterProviderFactory(ctx context.Context, bucket *b2.Bucket) func(string) io.WriteCloser {
	return func(fileName string) io.WriteCloser {
		obj := bucket.Object(fileName)
		w := obj.NewWriter(ctx)
		return w
	}
}

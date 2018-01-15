package b2

import (
	"context"
	"io"
	"log"

	"github.com/kurin/blazer/b2"
	"github.com/marpio/img-store/crypto"
	"github.com/marpio/img-store/remotestorage"
)

func New(ctx context.Context, b2id, b2key, bucketName, encryptKey string, cryptoSrv crypto.Service) remotestorage.Service {
	bucket := newB2Bucket(ctx, b2id, b2key, bucketName)

	r := func(fileName string) io.ReadCloser {
		rd := bucket.Object(fileName).NewReader(ctx)
		return rd
	}
	w := func(fileName string) io.WriteCloser {
		obj := bucket.Object(fileName)
		wr := obj.NewWriter(ctx)
		return wr
	}
	d := func(fileName string) error {
		if err := bucket.Object(fileName).Delete(ctx); err != nil {
			return err
		}
		return nil
	}
	b := remotestorage.Backend{ReadFn: r, WriteFn: w, DeleteFn: d, EncryptionKey: encryptKey, CryptoSrv: cryptoSrv}
	return &b
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

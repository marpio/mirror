package filestore

import (
	"bufio"
	"bytes"
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

func (b2 *B2Store) Download(dst io.Writer, encryptionKey, src string) {
	r := b2.bucket.Object(src).NewReader(b2.ctx)
	defer r.Close()

	var b bytes.Buffer
	writer := bufio.NewWriter(&b)
	if _, err := io.Copy(writer, r); err != nil {
		log.Fatal(err)
	}
	writer.Flush()
	encryptedData := b.Bytes()
	log.Printf("Encrypted: %v", len(encryptedData))
	decryptedData, err := crypto.Decrypt(encryptionKey, encryptedData)
	if err != nil {
		log.Print(err)
	}
	log.Printf("Decrypted: %v", len(decryptedData))
	//r.ConcurrentDownloads = downloads
	if _, err := io.Copy(dst, bytes.NewReader(decryptedData)); err != nil {
		log.Fatal(err)
	}
}

func (b2 *B2Store) Upload(imgFileName string, payload []byte) error {
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

func (b2 *B2Store) UploadStream(imgFileName string, reader io.Reader) error {
	imgObj := b2.bucket.Object(imgFileName)
	b2Writer := imgObj.NewWriter(b2.ctx)
	if _, err := io.Copy(b2Writer, reader); err != nil {
		return err
	}
	if err := b2Writer.Close(); err != nil {
		return err
	}
	return nil
}

func (b2 *B2Store) Delete(fileName string) error {
	return nil
}

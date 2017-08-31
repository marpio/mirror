package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/marpio/img-store/filestore/b2"
	"github.com/marpio/img-store/metadatastore/sqlite"
	"github.com/marpio/img-store/syncronizer"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	f := initLog()
	defer f.Close()

	if err := godotenv.Load("../../settings.env"); err != nil {
		log.Fatal("Error loading .env file")
	}
	encryptionKey := os.Getenv("ENCR_KEY")
	b2id := os.Getenv("B2_ACCOUNT_ID")
	b2key := os.Getenv("B2_ACCOUNT_KEY")
	bucketName := os.Getenv("B2_BUCKET_NAME")
	imgDBPath := os.Getenv("IMG_DB")

	ctx := context.Background()

	fileStore := b2.NewB2Store(ctx, b2id, b2key, bucketName)
	metadataStore := sqlite.NewSqliteMetadataStore(imgDBPath)

	dir := flag.String("syncdir", "", "Abs path to the directory containing pictures")
	downloadsrc := flag.String("src", "", "File to ....")
	downloaddest := flag.String("dest", "", "File to ....")
	flag.Parse()

	if *dir != "" {
		syncronizer := syncronizer.NewSyncronizer(ctx, fileStore, metadataStore, encryptionKey)
		syncronizer.Sync(*dir)

		if err := uploadMetadataStore(filepath.Base(imgDBPath), fileStore, encryptionKey); err != nil {
			log.Fatalf("Error uploading File-Store %v", err)
		}
	}
	if *downloadsrc != "" && *downloaddest != "" {
		f, err := os.Create(*downloaddest)
		defer f.Close()
		if err != nil {
			log.Fatal(err)
		}
		fileStore.DownloadDecrypted(f, encryptionKey, *downloadsrc)
	}

}

func uploadMetadataStore(imgDBpath string, fileStore *b2.B2Store, encryptionKey string) error {
	dbFileReader, err := os.OpenFile(imgDBpath, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	if err := fileStore.UploadEncrypted(imgDBpath, dbFileReader, encryptionKey); err != nil {
		return err
	}
	return nil
}

func initLog() io.Closer {
	f, err := os.OpenFile("output.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	log.SetOutput(f)
	return f
}

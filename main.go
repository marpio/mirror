package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/marpio/img-store/filestore"
	"github.com/marpio/img-store/metadatastore"
	"github.com/marpio/img-store/syncronizer"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	f := initLog()
	defer f.Close()

	if err := godotenv.Load("settings.env"); err != nil {
		log.Fatal("Error loading .env file")
	}
	encryptionKey := os.Getenv("ENCR_KEY")
	b2id := os.Getenv("B2_ACCOUNT_ID")
	b2key := os.Getenv("B2_ACCOUNT_KEY")
	bucketName := os.Getenv("B2_BUCKET_NAME")

	ctx := context.Background()

	fileStore := filestore.NewB2Store(ctx, b2id, b2key, bucketName)
	metadataStore := metadatastore.NewSqliteMetadataStore("img.db")

	dir := flag.String("syncdir", "", "Abs path to the directory containing pictures")
	flag.Parse()

	if *dir != "" {
		syncronizer := syncronizer.NewSyncronizer(ctx, fileStore, metadataStore)
		syncronizer.Sync(*dir, encryptionKey)
	}
}

func initLog() io.Closer {
	f, err := os.OpenFile("output.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	log.SetOutput(f)
	return f
}

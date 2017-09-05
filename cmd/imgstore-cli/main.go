package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"

	"github.com/marpio/img-store/metadata"

	"github.com/marpio/img-store/file"
	"github.com/marpio/img-store/filestore"

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
	r, w, d := b2.NewB2(ctx, b2id, b2key, bucketName)
	fileStore := filestore.NewFileStore(r, w, d, encryptionKey)
	metadataStore := sqlite.NewSqliteMetadataStore(imgDBPath)

	dir := flag.String("syncdir", "", "Abs path to the directory containing pictures")
	downloadsrc := flag.String("src", "", "File to ....")
	downloaddest := flag.String("dest", "", "File to ....")
	flag.Parse()

	if *dir != "" {
		syncronizer := syncronizer.NewSyncronizer(fileStore,
			metadataStore,
			file.ReadFile,
			file.GetImages,
			metadata.ExtractCreatedAt,
			metadata.ExtractThumbnail)
		syncronizer.Sync(*dir)
		if err := syncronizer.UploadMetadataStore(imgDBPath); err != nil {
			log.Print("Error uploading DB")
		}
	}
	if *downloadsrc != "" && *downloaddest != "" {
		f, err := os.Create(*downloaddest)
		defer f.Close()
		if err != nil {
			log.Fatal(err)
		}
		fileStore.DownloadDecrypted(f, *downloadsrc)
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

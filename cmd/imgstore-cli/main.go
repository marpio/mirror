package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/afero"

	"github.com/marpio/img-store/metadata"
	"github.com/marpio/img-store/metadatastore/hashmap"

	"github.com/marpio/img-store/file"
	"github.com/marpio/img-store/filestore"

	"github.com/marpio/img-store/filestore/b2"
	"github.com/marpio/img-store/syncronizer"
)

func main() {
	f := initLog()
	defer f.Close()

	encryptionKey := os.Getenv("ENCR_KEY")
	b2id := os.Getenv("B2_ACCOUNT_ID")
	b2key := os.Getenv("B2_ACCOUNT_KEY")
	bucketName := os.Getenv("B2_BUCKET_NAME")
	dbPath := os.Getenv("IMG_DB")
	ctx := context.Background()

	r, w, d := b2.NewB2(ctx, b2id, b2key, bucketName)

	fileStore := filestore.NewFileStore(r, w, d, encryptionKey)

	appFs := afero.NewOsFs()
	metadataStore := hashmap.NewMetadataStore(appFs, dbPath)

	dir := flag.String("sync", "", "Abs path to the directory containing pictures")
	downloadsrc := flag.String("src", "", "File to ....")
	downloaddest := flag.String("dst", "", "File to ....")
	flag.Parse()

	if *dir != "" {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		readFileFn := file.FileReader(appFs)
		syncronizer := syncronizer.NewSyncronizer(fileStore,
			metadataStore,
			readFileFn,
			file.PhotosFinder(appFs),
			metadata.CreatedAtExtractor(appFs),
			metadata.ExtractThumbnail)
		syncronizer.Sync(*dir, sigs)

		dbFileReader, err := readFileFn(dbPath)
		if err != nil {
			log.Print("Error uploading DB")
		}
		if err := fileStore.UploadEncrypted(dbPath, dbFileReader); err != nil {
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
	f, err := os.Create("output.log")
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	log.SetOutput(f)
	return f
}

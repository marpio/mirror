package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aymerick/raymond"
	"github.com/marpio/img-store/filestore"
	"github.com/marpio/img-store/metadatastore"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/marpio/img-store/filestore/b2"
	"github.com/marpio/img-store/metadatastore/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

func mainPageHandler(metadataStore metadatastore.DataStoreReader) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		months, err := metadataStore.GetMonths()
		var folders []interface{}
		for _, m := range months {
			data := struct {
				Year  int
				Month int
			}{
				int(m.Year()),
				int(m.Month()),
			}
			folders = append(folders, data)
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		ctx := map[string]interface{}{
			"folders": folders,
		}
		tmpl, err := raymond.ParseFile("templates/index.hbs")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		result, err := tmpl.Exec(ctx)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		fmt.Fprint(w, result)
	}
}

func fileHandler(fileStore filestore.FileStoreReader, encryptionKey string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		fileName := vars["name"]

		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			fileStore.DownloadDecrypted(pw, encryptionKey, fileName)
		}()
		b, err := ioutil.ReadAll(pr)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		http.ServeContent(w, r, fileName, time.Now(), bytes.NewReader(b))
	}
}

func main() {
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

	f, err := os.Create(filepath.Base(imgDBPath))

	if err != nil {
		log.Fatal(err)
	}
	fileStore.DownloadDecrypted(f, encryptionKey, filepath.Base(imgDBPath))
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}

	metadataStore := sqlite.NewSqliteMetadataStore(imgDBPath)

	r := mux.NewRouter()
	r.HandleFunc("/", mainPageHandler(metadataStore))
	r.HandleFunc("/year/{y}/month/{m}", mainPageHandler(metadataStore))
	r.HandleFunc("/files/{name}", fileHandler(fileStore, encryptionKey))
	r.PathPrefix("/public/").Handler(http.StripPrefix("/public/", http.FileServer(http.Dir("public/"))))

	http.ListenAndServe(":5000", r)
}

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
	"strconv"
	"time"

	"github.com/aymerick/raymond"
	"github.com/goji/httpauth"
	"github.com/gorilla/mux"
	"github.com/marpio/img-store/entity"
	"github.com/marpio/img-store/filestore"
	"github.com/marpio/img-store/filestore/b2"
	"github.com/marpio/img-store/metadatastore"
	"github.com/marpio/img-store/metadatastore/hashmap"
	"github.com/spf13/afero"
)

func main() {
	encryptionKey := os.Getenv("ENCR_KEY")
	b2id := os.Getenv("B2_ACCOUNT_ID")
	b2key := os.Getenv("B2_ACCOUNT_KEY")
	bucketName := os.Getenv("B2_BUCKET_NAME")
	dbPath := os.Getenv("IMG_DB")
	username := os.Getenv("PICS_USERNAME")
	password := os.Getenv("PICS_PASSWORD")
	ctx := context.Background()
	fileStore := b2.New(ctx, b2id, b2key, bucketName, encryptionKey)
	var appFs afero.Fs = afero.NewOsFs()
	metadataStore, err := createMetadataStore(appFs, dbPath, fileStore)
	if err != nil {
		log.Fatal(err)
	}

	router := configureRouter(metadataStore, fileStore, encryptionKey, dbPath)
	http.Handle("/", httpauth.SimpleBasicAuth(username, password)(router))

	http.ListenAndServe(":5000", nil)
}

func configureRouter(metadataStore metadatastore.ReaderService, fileStore filestore.ReaderService, encryptionKey string, imgDBPath string) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/", mainPageHandler(metadataStore))
	r.HandleFunc("/images/year/{year}/month/{month}", monthImgsHandler(metadataStore))
	r.HandleFunc("/files/{name}", fileHandler(fileStore))
	r.HandleFunc("/reloaddb", func(w http.ResponseWriter, r *http.Request) {
		err := metadataStore.Reload()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		fmt.Fprint(w, "ok")
	})
	r.PathPrefix("/public/").Handler(http.StripPrefix("/public/", http.FileServer(http.Dir("public/"))))
	return r
}

func createMetadataStore(fs afero.Fs, imgDBPath string, fileStore filestore.ReaderService) (metadatastore.Service, error) {
	f, err := fs.Create(filepath.Base(imgDBPath))

	if err != nil {
		log.Print(err)
		return nil, err
	}
	fileStore.DownloadDecrypted(f, filepath.Base(imgDBPath))
	if err := f.Close(); err != nil {
		log.Print(err)
		return nil, err
	}

	metadataStore := hashmap.New(fs, imgDBPath)
	return metadataStore, nil
}

func mainPageHandler(metadataStore metadatastore.ReaderService) func(w http.ResponseWriter, r *http.Request) {
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

func monthImgsHandler(metadataStore metadatastore.ReaderService) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		year, err := strconv.Atoi(vars["year"])
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		month, err := strconv.Atoi(vars["month"])
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		m := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		imgs, err := metadataStore.GetByMonth(m)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		ctx := map[string][]*entity.Photo{
			"imgs": imgs,
		}
		tmpl, err := raymond.ParseFile("templates/month.hbs")
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

func fileHandler(fileStore filestore.ReaderService) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		fileName := vars["name"]

		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			fileStore.DownloadDecrypted(pw, fileName)
		}()
		b, err := ioutil.ReadAll(pr)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		http.ServeContent(w, r, fileName, time.Now(), bytes.NewReader(b))
	}
}

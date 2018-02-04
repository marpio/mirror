package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/aymerick/raymond"
	"github.com/goji/httpauth"
	"github.com/gorilla/mux"
	"github.com/marpio/img-store/crypto"
	"github.com/marpio/img-store/domain"
	"github.com/marpio/img-store/remotestorage"
	"github.com/marpio/img-store/remotestorage/b2"
	"github.com/marpio/img-store/repository"
	"github.com/marpio/img-store/repository/hashmap"
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
	rsBackend := b2.New(ctx, b2id, b2key, bucketName)
	rs := remotestorage.New(rsBackend, crypto.NewService(encryptionKey))
	appFs := afero.NewOsFs()
	metadataStore := createMetadataStore(appFs, dbPath, rs)

	router := configureRouter(metadataStore, rs, dbPath)
	http.Handle("/", httpauth.SimpleBasicAuth(username, password)(router))

	http.ListenAndServe(":5000", nil)
}

func configureRouter(metadataStore repository.ReaderService, remotestorage domain.StorageReader, imgDBPath string) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/", mainPageHandler(metadataStore))
	r.HandleFunc("/dirs/{dir}", dirHandler(metadataStore))
	r.HandleFunc("/files/{id}", fileHandler(remotestorage))
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

func createMetadataStore(fs afero.Fs, imgDBPath string, remotestorage domain.Storage) repository.ReaderService {
	repo, err := hashmap.New(remotestorage, imgDBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating metadata repository: %v", err)
		os.Exit(-1)
	}
	return repo
}

func mainPageHandler(metadataStore repository.ReaderService) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		dirs, err := metadataStore.GetDirs()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		ctx := map[string][]string{
			"folders": dirs,
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

func dirHandler(metadataStore repository.ReaderService) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		dir := vars["dir"]

		imgs, err := metadataStore.GetByDir(dir)

		ctx := map[string][]domain.Item{
			"imgs": imgs,
		}
		tmpl, err := raymond.ParseFile("templates/dir.hbs")
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

func fileHandler(remotestorage domain.StorageReader) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]

		rd, err := remotestorage.NewReader(id)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		b, err := ioutil.ReadAll(rd)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		http.ServeContent(w, r, id, time.Now(), bytes.NewReader(b))
	}
}

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
	metadataStore := createMetadataStore(ctx, appFs, dbPath, rs)

	router := configureRouter(ctx, metadataStore, rs, dbPath)
	http.Handle("/", httpauth.SimpleBasicAuth(username, password)(router))

	http.ListenAndServe(":5000", nil)
}

func configureRouter(ctx context.Context, metadataStore domain.MetadataRepoReader, remotestorage domain.StorageReader, imgDBPath string) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/", mainPageHandler(metadataStore))
	r.HandleFunc("/dirs/{dir}", dirHandler(metadataStore))
	r.HandleFunc("/files/{id}", fileHandler(ctx, remotestorage))
	r.HandleFunc("/reloaddb", func(w http.ResponseWriter, r *http.Request) {
		err := metadataStore.Reload(ctx)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		fmt.Fprint(w, "ok")
	})
	r.PathPrefix("/public/").Handler(http.StripPrefix("/public/", http.FileServer(http.Dir("public/"))))
	return r
}

func createMetadataStore(ctx context.Context, fs afero.Fs, imgDBPath string, remotestorage domain.Storage) domain.MetadataRepoReader {
	repo, err := hashmap.New(ctx, remotestorage, imgDBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating metadata repository: %v", err)
		os.Exit(-1)
	}
	return repo
}

func mainPageHandler(metadataStore domain.MetadataRepoReader) func(w http.ResponseWriter, r *http.Request) {
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

func dirHandler(metadataStore domain.MetadataRepoReader) func(w http.ResponseWriter, r *http.Request) {
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

func fileHandler(ctx context.Context, remotestorage domain.StorageReader) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		rd, err := remotestorage.NewReader(ctx, id)
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

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
	"github.com/marpio/mirror/crypto"

	"github.com/marpio/mirror/remotestorage"
	"github.com/marpio/mirror/remotestorage/b2"
	"github.com/marpio/mirror/repository/hashmap"
	"github.com/spf13/afero"
)

func getenv(n string) string {
	v := os.Getenv(n)
	if v == "" {
		panic("could not find env var " + n)
	}
	return v
}

func main() {
	encryptionKey := getenv("ENCR_KEY")
	b2id := getenv("B2_ACCOUNT_ID")
	b2key := getenv("B2_ACCOUNT_KEY")
	bucketName := getenv("B2_BUCKET_NAME")
	repoFileName := getenv("REPO")
	username := getenv("MIRROR_USERNAME")
	password := getenv("MIRROR_PASSWORD")
	ctx := context.Background()
	rsBackend := b2.New(ctx, b2id, b2key, bucketName)
	rs := remotestorage.New(rsBackend, crypto.NewService(encryptionKey))
	appFs := afero.NewOsFs()
	metadataStore := createMetadataStore(ctx, appFs, repoFileName, rs)

	router := configureRouter(ctx, metadataStore, rs, repoFileName)
	http.Handle("/", httpauth.SimpleBasicAuth(username, password)(router))

	http.ListenAndServe(":5000", nil)
}

func configureRouter(ctx context.Context, metadataStore mirror.MetadataRepoReader, remotestorage mirror.StorageReader, imgDBPath string) *mux.Router {
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

func createMetadataStore(ctx context.Context, fs afero.Fs, imgDBPath string, remotestorage mirror.Storage) mirror.MetadataRepoReader {
	repo, err := hashmap.New(ctx, remotestorage, imgDBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating metadata repository: %v", err)
		os.Exit(-1)
	}
	return repo
}

func mainPageHandler(metadataStore mirror.MetadataRepoReader) func(w http.ResponseWriter, r *http.Request) {
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

func dirHandler(metadataStore mirror.MetadataRepoReader) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		dir := vars["dir"]

		items, err := metadataStore.GetByDir(dir)
		photos := make([]interface{}, 0)
		for _, it := range items {
			p := struct {
				ID      string
				ThumbID string
			}{
				it.ID(),
				it.ThumbID(),
			}
			photos = append(photos, p)
		}

		ctx := map[string][]interface{}{
			"imgs": photos,
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

func fileHandler(ctx context.Context, remotestorage mirror.StorageReader) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]
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

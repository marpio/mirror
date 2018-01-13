// Copyright © 2017 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/marpio/img-store/filestore/b2"
	"github.com/marpio/img-store/fsutils"
	"github.com/marpio/img-store/metadata"
	"github.com/marpio/img-store/metadatastore/hashmap"
	"github.com/marpio/img-store/syncronizer"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync local directory with a remote.",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		runSync(args[0])
	},
}

func runSync(dir string) {
	f := initLog()
	defer f.Close()

	encryptionKey := os.Getenv("ENCR_KEY")
	b2id := os.Getenv("B2_ACCOUNT_ID")
	b2key := os.Getenv("B2_ACCOUNT_KEY")
	bucketName := os.Getenv("B2_BUCKET_NAME")
	dbPath := os.Getenv("IMG_DB")
	ctx := context.Background()

	fileStore := b2.New(ctx, b2id, b2key, bucketName, encryptionKey)

	appFs := afero.NewOsFs()
	metadataStore := hashmap.New(appFs, dbPath)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := wrapChan(sigs)
	defer close(sigs)
	defer close(done)

	localFilesRepo := fsutils.NewLocalFilesRepo(appFs)
	syncronizer := syncronizer.New(fileStore,
		metadataStore,
		localFilesRepo,
		metadata.NewExtractor(appFs))
	syncronizer.Execute(dir, done)

	dbFileReader, err := localFilesRepo.ReadFile(dbPath)
	if err != nil {
		log.Print("Error uploading DB")
	}
	if err := fileStore.UploadEncrypted(dbPath, dbFileReader); err != nil {
		log.Print("Error uploading DB")
	}
}

func wrapChan(sigs <-chan os.Signal) chan interface{} {
	c := make(chan interface{}, 1)
	go func() {
		for {
			_, ok := <-sigs
			if ok {
				c <- struct{}{}
			}
		}
	}()
	return c
}

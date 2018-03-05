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
	"os"
	"os/signal"
	"syscall"

	"github.com/apex/log"
	"github.com/apex/log/handlers/json"
	"github.com/apex/log/handlers/multi"
	"github.com/apex/log/handlers/text"
	"github.com/marpio/mirror/crypto"
	"github.com/marpio/mirror/metadata"
	"github.com/marpio/mirror/repository/hashmap"
	"github.com/marpio/mirror/storage"
	"github.com/marpio/mirror/storage/remotebackend"
	"github.com/marpio/mirror/syncronizer"
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

func getenv(n string) string {
	v := os.Getenv(n)
	if v == "" {
		panic("could not find env var " + n)
	}
	return v
}

func runSync(dir string) {
	logFile, err := os.Create("log.json")
	if err != nil {
		log.Fatal("error creating log file")
	}
	defer logFile.Close()
	log.SetHandler(multi.New(
		text.New(os.Stderr),
		json.New(logFile),
	))

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer close(sigs)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	logctx := log.WithFields(log.Fields{
		"cmd":         "mirror-cli",
		"syncing_dir": dir,
	})

	encryptionKey := getenv("ENCR_KEY")
	b2id := getenv("B2_ACCOUNT_ID")
	b2key := getenv("B2_ACCOUNT_KEY")
	bucketName := getenv("B2_BUCKET_NAME")
	rsBackend := remotebackend.NewB2(ctx, b2id, b2key, bucketName)
	rs := storage.NewRemote(rsBackend, crypto.NewService(encryptionKey))

	dbPath := getenv("REPO")
	repo, err := hashmap.New(ctx, rs, dbPath)
	if err != nil {
		log.Fatalf("error creating metadata repository: %v", err)
	}
	go func() {
		for {
			select {
			case <-sigs:
				logctx.Warn("SIGINT or SIGTERM - saving and terminating...")
				repo.Persist(ctx)
				cancel()
				return
			}
		}
	}()
	appFs := afero.NewOsFs()
	localFilesRepo := storage.NewLocal(appFs, crypto.GenerateSha256)
	syncronizer := syncronizer.New(rs,
		repo,
		localFilesRepo,
		metadata.NewExtractor(localFilesRepo))
	syncronizer.Execute(ctx, logctx, dir)
	logctx.Info("done syncing.")
}

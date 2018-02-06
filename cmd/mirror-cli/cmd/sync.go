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
	"time"

	"github.com/apex/log"
	"github.com/apex/log/handlers/json"
	"github.com/apex/log/handlers/multi"
	"github.com/apex/log/handlers/text"
	"github.com/marpio/mirror/crypto"
	"github.com/marpio/mirror/localstorage"
	"github.com/marpio/mirror/metadata"
	"github.com/marpio/mirror/remotestorage"
	"github.com/marpio/mirror/remotestorage/b2"
	"github.com/marpio/mirror/repository/hashmap"
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

	encryptionKey := getenv("ENCR_KEY")
	b2id := getenv("B2_ACCOUNT_ID")
	b2key := getenv("B2_ACCOUNT_KEY")
	bucketName := getenv("B2_BUCKET_NAME")
	dbPath := getenv("REPO")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer close(sigs)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	logctx := log.WithFields(log.Fields{
		"cmd": "cli",
		"dir": dir,
	})
	go func() {
		for {
			select {
			case <-sigs:
				logctx.Warn("process terminated. syscall.SIGINT or syscall.SIGTERM.")
				cancel()
			}
		}
	}()

	rsBackend := b2.New(ctx, b2id, b2key, bucketName)
	rs := remotestorage.New(rsBackend, crypto.NewService(encryptionKey))

	c, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	repo, err := hashmap.New(c, rs, dbPath)
	if err != nil {
		log.Fatalf("error creating metadata repository: %v", err)
	}
	appFs := afero.NewOsFs()
	localFilesRepo := localstorage.NewService(appFs)
	syncronizer := syncronizer.New(rs,
		repo,
		localFilesRepo,
		metadata.NewExtractor(localFilesRepo))
	syncronizer.Execute(ctx, logctx, dir)
}

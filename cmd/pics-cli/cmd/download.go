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
	"fmt"
	"io"
	"log"
	"os"

	"github.com/marpio/img-store/crypto"
	"github.com/marpio/img-store/remotestorage"
	"github.com/marpio/img-store/remotestorage/b2"
	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download file from remote.",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		runDownload(args[0], args[1])
	},
}

func runDownload(localFilePath, remoteFilePath string) {
	l := initLog()
	defer l.Close()

	encryptionKey := os.Getenv("ENCR_KEY")
	b2id := os.Getenv("B2_ACCOUNT_ID")
	b2key := os.Getenv("B2_ACCOUNT_KEY")
	bucketName := os.Getenv("B2_BUCKET_NAME")
	ctx := context.Background()

	rsBackend := b2.New(ctx, b2id, b2key, bucketName)
	rs := remotestorage.New(rsBackend, crypto.NewService(encryptionKey))

	f, err := os.Create(localFilePath)

	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	r, err := rs.NewReader(remoteFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening file on remote storage: %v", err)
		os.Exit(-1)
	}
	defer r.Close()
	io.Copy(f, r)
}

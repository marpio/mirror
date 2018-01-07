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

	"github.com/marpio/img-store/filestore/b2"
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

func runDownload(dstFilePath, remoteFileName string) {
	l := initLog()
	defer l.Close()

	encryptionKey := os.Getenv("ENCR_KEY")
	b2id := os.Getenv("B2_ACCOUNT_ID")
	b2key := os.Getenv("B2_ACCOUNT_KEY")
	bucketName := os.Getenv("B2_BUCKET_NAME")
	ctx := context.Background()

	fileStore := b2.New(ctx, b2id, b2key, bucketName, encryptionKey)

	f, err := os.Create(dstFilePath)

	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	fileStore.DownloadDecrypted(f, remoteFileName)
}

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
	"os"
	"path/filepath"

	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
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
	log.SetHandler(text.New(os.Stdout))
	m, _ := filepath.Glob("/home/piotr/Pictures/Fotos/**/*.{jpeg,jpg,NEF}")
	for _, x := range m {
		log.Debug(x)
	}
	//defer log.WithFields(log.Fields{
	//	"localFilePath":  localFilePath,
	//	"remoteFilePath": remoteFilePath,
	//}).Trace("starting download.")
	//ctx := context.Background()
	//
	//encryptionKey := os.Getenv("ENCR_KEY")
	//b2id := os.Getenv("B2_ACCOUNT_ID")
	//b2key := os.Getenv("B2_ACCOUNT_KEY")
	//bucketName := os.Getenv("B2_BUCKET_NAME")
	//
	//rsBackend := b2.New(ctx, b2id, b2key, bucketName)
	//rs := remotestorage.New(rsBackend, crypto.NewService(encryptionKey))
	//
	//f, err := os.Create(localFilePath)
	//
	//if err != nil {
	//	log.Fatal("could not create the destination file.")
	//}
	//defer f.Close()
	//ctx, cancel := context.WithTimeout(ctx, time.Minute)
	//defer cancel()
	//r, err := rs.NewReader(ctx, remoteFilePath)
	//if err != nil {
	//	log.Fatal(err.Error())
	//}
	//defer r.Close()
	//io.Copy(f, r)
}

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
	"os/signal"
	"syscall"

	"github.com/marpio/img-store/file"
	"github.com/marpio/img-store/filestore"
	"github.com/marpio/img-store/filestore/b2"
	"github.com/marpio/img-store/metadata"
	"github.com/marpio/img-store/metadatastore/hashmap"
	"github.com/marpio/img-store/syncronizer"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "imgstore-cli",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync local directory with a remote.",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		runSync(args[0])
	},
}

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download file from remote.",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		runDownload(args[0], args[1])
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.AddCommand(syncCmd)
	RootCmd.AddCommand(downloadCmd)

	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.myapp.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".myapp" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".myapp")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
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

	r, w, d := b2.NewB2(ctx, b2id, b2key, bucketName)

	fileStore := filestore.NewFileStore(r, w, d, encryptionKey)

	appFs := afero.NewOsFs()
	metadataStore := hashmap.NewMetadataStore(appFs, dbPath)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	done := wrapChan(sigs)
	syncronizer := syncronizer.NewSyncronizer(fileStore,
		metadataStore,
		file.FileReader(appFs),
		file.PhotosFinder(appFs),
		metadata.CreatedAtExtractor(appFs),
		metadata.ExtractThumbnail)
	syncronizer.Sync(dir, done)

	close(sigs)
	close(done)

	dbFileReader, err := file.FileReader(appFs)(dbPath)
	if err != nil {
		log.Print("Error uploading DB")
	}
	if err := fileStore.UploadEncrypted(dbPath, dbFileReader); err != nil {
		log.Print("Error uploading DB")
	}
}

func runDownload(dstFilePath, remoteFileName string) {
	l := initLog()
	defer l.Close()

	encryptionKey := os.Getenv("ENCR_KEY")
	b2id := os.Getenv("B2_ACCOUNT_ID")
	b2key := os.Getenv("B2_ACCOUNT_KEY")
	bucketName := os.Getenv("B2_BUCKET_NAME")
	ctx := context.Background()

	r, w, d := b2.NewB2(ctx, b2id, b2key, bucketName)

	fileStore := filestore.NewFileStore(r, w, d, encryptionKey)
	f, err := os.Create(dstFilePath)

	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	fileStore.DownloadDecrypted(f, remoteFileName)
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

func initLog() io.Closer {
	f, err := os.Create("output.log")
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	log.SetOutput(f)
	return f
}

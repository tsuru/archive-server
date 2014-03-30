// Copyright 2014 Globo.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/globocom/tsuru/db/storage"
)

var (
	databaseAddr string
	databaseName string
	baseDir      string
	readHttp     string
	writeHttp    string
)

func init() {
	flag.StringVar(&databaseAddr, "mongodb", "127.0.0.1:27017", "Address of the database server")
	flag.StringVar(&databaseName, "dbname", "archives", "Name of the database to store information about archives")
	flag.StringVar(&baseDir, "dir", "/var/lib/archives/", "Base directory, where the server will create and serve the archives")
	flag.StringVar(&readHttp, "read-http", "", "Address to bind the API that serves archives. Omit to not start this API.")
	flag.StringVar(&writeHttp, "write-http", "", "Address to bind the API that creates archives. Omit to not start this API.")
}

func conn() (*storage.Storage, error) {
	return storage.Open(databaseAddr, databaseName)
}

func main() {
	flag.Parse()
	if readHttp == "" && writeHttp == "" {
		fmt.Println("You need to specify at-least one of -read-http and -write-http")
		os.Exit(1)
	}
}

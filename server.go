// Copyright 2015 Globo.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/tsuru/tsuru/db/storage"
	"gopkg.in/tylerb/graceful.v1"
)

const version = "0.1.1"

var (
	databaseAddr string
	databaseName string
	baseDir      string
	readHttp     string
	writeHttp    string
	checkVersion bool
)

func init() {
	flag.StringVar(&databaseAddr, "mongodb", "127.0.0.1:27017", "Address of the database server")
	flag.StringVar(&databaseName, "dbname", "archives", "Name of the database to store information about archives")
	flag.StringVar(&baseDir, "dir", "/var/lib/archives/", "Base directory, where the server will create and serve the archives")
	flag.StringVar(&readHttp, "read-http", "", "Address to bind the API that serves archives. Omit to not start this API.")
	flag.StringVar(&writeHttp, "write-http", "", "Address to bind the API that creates archives. Omit to not start this API.")
	flag.BoolVar(&checkVersion, "version", false, "Print version and exit")
}

func conn() (*storage.Storage, error) {
	return storage.Open(databaseAddr, databaseName)
}

func createArchiveHandler(w http.ResponseWriter, r *http.Request) {
	path := r.FormValue("path")
	refid := r.FormValue("refid")
	prefix := r.FormValue("prefix")
	if path == "" || refid == "" {
		http.Error(w, "path and refid are required", http.StatusBadRequest)
		return
	}
	archive, err := NewArchive(path, refid, baseDir, prefix)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := map[string]string{"id": archive.ID}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(response)
}

func readArchiveHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	keep := r.URL.Query().Get("keep") == "1"
	if id == "" {
		http.Error(w, "missing archive id", http.StatusBadRequest)
		return
	}
	archive, err := GetArchive(id)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrArchiveNotFound {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	switch archive.Status {
	case StatusReady:
		serve(w, archive, keep)
	case StatusDestroyed:
		http.Error(w, ErrArchiveNotFound.Error(), http.StatusNotFound)
	case StatusBuilding:
		w.Header().Add("Content-Type", "text")
		fmt.Fprintln(w, "BUILDING")
	case StatusError:
		http.Error(w, archive.Log, http.StatusInternalServerError)
	default:
		http.Error(w, "unknown error", http.StatusInternalServerError)
	}
}

func serve(w http.ResponseWriter, archive *Archive, keep bool) {
	if !keep {
		defer DestroyArchive(archive.ID)
	}
	file, err := os.Open(archive.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()
	w.Header().Add("Content-Type", "application/x-gzip")
	io.Copy(w, file)
}

func main() {
	flag.Parse()
	if checkVersion {
		fmt.Printf("archive-server version %s\n", version)
		os.Exit(0)
	}
	if readHttp == "" && writeHttp == "" {
		fmt.Println("You need to specify at-least one of -read-http and -write-http")
		os.Exit(1)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	if writeHttp != "" {
		go func() {
			srv := graceful.Server{
				Timeout: 10 * time.Minute,
				Server: &http.Server{
					Addr:    writeHttp,
					Handler: http.HandlerFunc(createArchiveHandler),
				},
			}
			srv.ListenAndServe()
			wg.Done()
		}()
	}
	if readHttp != "" {
		go func() {
			srv := graceful.Server{
				Timeout: 10 * time.Minute,
				Server: &http.Server{
					Addr:    readHttp,
					Handler: http.HandlerFunc(readArchiveHandler),
				},
			}
			srv.ListenAndServe()
			wg.Done()
		}()
	}
	wg.Wait()
}

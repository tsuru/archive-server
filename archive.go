// Copyright 2014 Globo.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha512"
	"errors"
	"fmt"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const collectionName = "archives"

const (
	// StatusBuilding indicates that the server is building the archive.
	StatusBuilding Status = iota

	// StatusReady indicates that the archive is ready.
	StatusReady

	// StatusError indicates that the archive failed to build.
	StatusError

	// StatusDestroyed indicates that the archive has been destroyed.
	StatusDestroyed
)

// Error returned when an archive does not exist.
var ErrArchiveNotFound = errors.New("archive not found")

// Status represents the current status of the archive.
type Status byte

// String returns the string representation of the status.
func (s Status) String() string {
	switch s {
	case StatusBuilding:
		return "building"
	case StatusReady:
		return "ready"
	case StatusError:
		return "error"
	case StatusDestroyed:
		return "destroyed"
	default:
		return "unknown"
	}
}

// Archive represents a git archive.
type Archive struct {
	ID     string `bson:"_id"`
	Path   string
	Status Status
	Log    string
}

// NewArchive inserts a new archive in the database and starts the generation
// of the actual archive in background.
func NewArchive(path, refid, baseDir, prefix string) (*Archive, error) {
	archive := Archive{
		ID:     newID(path),
		Status: StatusBuilding,
	}
	archive.Path = filepath.Join(baseDir, archive.ID+".tar.gz")
	db, err := conn()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	err = db.Collection(collectionName).Insert(archive)
	if err != nil {
		return nil, err
	}
	go archive.generate(path, refid, archive.Path, prefix)
	return &archive, nil
}

func (archive Archive) generate(repositoryPath, refid, archivePath, prefix string) {
	db, err := conn()
	if err != nil {
		return
	}
	defer db.Close()
	status := StatusReady
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	var buf bytes.Buffer
	command := exec.Command(
		"git", "archive", "--format=tar.gz",
		"--output="+archivePath, "--prefix="+prefix, refid,
	)
	command.Dir = repositoryPath
	command.Stdout = &buf
	command.Stderr = &buf
	if err := command.Run(); err != nil {
		status = StatusError
	}
	archive.Log = buf.String()
	update := bson.M{"$set": bson.M{"status": status, "log": archive.Log}}
	db.Collection(collectionName).UpdateId(archive.ID, update)
}

func newID(path string) string {
	var buf [32]byte
	_, err := rand.Read(buf[:])
	if err != nil {
		return ""
	}
	nanoUnix := time.Now().UnixNano()
	hash := sha512.New()
	hash.Write(buf[:])
	hash.Write([]byte(path))
	hash.Write([]byte(fmt.Sprintf("%d", nanoUnix)))
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// GetArchive returns an archive by its ID.
func GetArchive(id string) (*Archive, error) {
	db, err := conn()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var archive Archive
	err = db.Collection(collectionName).FindId(id).One(&archive)
	if err == mgo.ErrNotFound {
		return nil, ErrArchiveNotFound
	}
	return &archive, nil
}

// DestroyArchive removes an archive by its ID.
func DestroyArchive(id string) error {
	archive, err := GetArchive(id)
	if err != nil {
		return err
	}
	db, err := conn()
	if err != nil {
		return err
	}
	defer db.Close()
	update := bson.M{"$set": bson.M{"status": StatusDestroyed}}
	err = db.Collection(collectionName).UpdateId(id, update)
	if err != nil {
		return err
	}
	os.Remove(archive.Path)
	return nil
}

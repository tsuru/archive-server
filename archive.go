// Copyright 2014 Globo.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha512"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"labix.org/v2/mgo/bson"
)

const (
	StatusBuilding Status = iota
	StatusReady
	StatusError
)

type Status byte

func (s Status) String() string {
	switch s {
	case StatusBuilding:
		return "building"
	case StatusReady:
		return "ready"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

type Archive struct {
	ID     string `bson:"_id"`
	Path   string
	Status Status
	Log    string
}

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
	err = db.Collection("archives").Insert(archive)
	if err != nil {
		return nil, err
	}
	go generate(archive, path, refid, archive.Path, prefix)
	return &archive, nil
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

func generate(archive Archive, repositoryPath, refid, archivePath, prefix string) {
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
	db.Collection("archives").UpdateId(archive.ID, update)
}

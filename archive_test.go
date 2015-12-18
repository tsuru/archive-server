// Copyright 2015 Globo.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/check.v1"
	"gopkg.in/mgo.v2/bson"
)

func (Suite) TestStatus(c *check.C) {
	var tests = []struct {
		input    Status
		expected string
	}{
		{StatusBuilding, "building"},
		{StatusReady, "ready"},
		{StatusError, "error"},
		{StatusDestroyed, "destroyed"},
		{Status(6), "unknown"},
	}
	for _, t := range tests {
		got := t.input.String()
		c.Check(got, check.Equals, t.expected)
	}
}

func (Suite) TestNewArchive(c *check.C) {
	archive, err := NewArchive(ioutil.NopCloser(bytes.NewBuffer([]byte("my file"))), "app_commit_uuid.tar.gz", "/tmp/")
	c.Assert(err, check.IsNil)
	sess, err := conn()
	c.Assert(err, check.IsNil)
	defer sess.Close()
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	c.Assert(archive.Status, check.Equals, StatusBuilding)
	c.Assert(archive.Path, check.Equals, "/tmp/"+archive.ID+".tar.gz")
	wait(c, 3e9, func() bool {
		count, err := sess.Collection(collectionName).Find(bson.M{"_id": archive.ID, "status": StatusReady}).Count()
		return err == nil && count == 1
	})
	err = sess.Collection(collectionName).FindId(archive.ID).One(&archive)
	c.Assert(err, check.IsNil)
	c.Assert(archive.Status, check.Equals, StatusReady)
}

func (Suite) TestNewArchiveFailure(c *check.C) {
	archive, err := NewArchive(ioutil.NopCloser(bytes.NewBuffer([]byte("my file"))), "app_commit_uuid.tar.gz", "/tmp/archive-server")
	c.Assert(err, check.IsNil)
	sess, err := conn()
	c.Assert(err, check.IsNil)
	defer sess.Close()
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	wait(c, 3e9, func() bool {
		count, err := sess.Collection(collectionName).Find(bson.M{"_id": archive.ID, "status": StatusError}).Count()
		return err == nil && count == 1
	})
	err = sess.Collection(collectionName).FindId(archive.ID).One(&archive)
	c.Assert(err, check.IsNil)
	c.Assert(archive.Status, check.Equals, StatusError)
}

func (Suite) TestGetArchive(c *check.C) {
	id := "some interesting id"
	archive := Archive{ID: id, Path: "/tmp/archive.tar.gz", Status: StatusBuilding}
	sess, err := conn()
	c.Assert(err, check.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	gotArchive, err := GetArchive(archive.ID)
	c.Assert(err, check.IsNil)
	c.Assert(*gotArchive, check.DeepEquals, archive)
}

func (Suite) TestGetArchiveNotFound(c *check.C) {
	archive, err := GetArchive("wat")
	c.Assert(archive, check.IsNil)
	c.Assert(err, check.Equals, ErrArchiveNotFound)
}

func (Suite) TestDestroyArchive(c *check.C) {
	tmpdir := os.TempDir()
	path := filepath.Join(tmpdir, "some-temp-file.txt")
	file, err := os.Create(path)
	c.Assert(err, check.IsNil)
	file.Close()
	defer os.Remove(path)
	id := "some interesting id"
	t := time.Now()
	archive := Archive{
		ID:        id,
		Path:      path,
		Status:    StatusReady,
		CreatedAt: t,
		UpdatedAt: t,
	}
	sess, err := conn()
	c.Assert(err, check.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	err = DestroyArchive(archive.ID)
	c.Assert(err, check.IsNil)
	_, err = os.Stat(path)
	c.Assert(os.IsNotExist(err), check.Equals, true)
	err = sess.Collection(collectionName).FindId(archive.ID).One(&archive)
	c.Assert(err, check.IsNil)
	c.Assert(archive.Status, check.Equals, StatusDestroyed)
	c.Assert(archive.UpdatedAt, check.Not(check.DeepEquals), t)
}

func (Suite) TestDestroyArchiveNotFound(c *check.C) {
	err := DestroyArchive("waaat")
	c.Assert(err, check.Equals, ErrArchiveNotFound)
}

func (Suite) TestDestroyArchiveDBError(c *check.C) {
	archive := Archive{ID: "hello hello"}
	sess, err := conn()
	c.Assert(err, check.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	oldDbAddr := databaseAddr
	databaseAddr = "256.256.256.256:27017"
	defer func() { databaseAddr = oldDbAddr }()
	err = DestroyArchive(archive.ID)
	c.Assert(err, check.NotNil)
}

func wait(c *check.C, timeout time.Duration, fn func() bool) {
	done := make(chan int)
	quit := make(chan int)
	go func() {
		run := true
		for run {
			select {
			case <-quit:
				run = false
			default:
				run = !fn()
			}
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		close(quit)
		c.Fatalf("Timed out after %s", timeout)
	}
}

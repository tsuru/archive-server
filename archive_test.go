// Copyright 2014 Globo.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/tsuru/commandmocker"
	"labix.org/v2/mgo/bson"
	"launchpad.net/gocheck"
)

func (Suite) TestStatus(c *gocheck.C) {
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
		c.Check(got, gocheck.Equals, t.expected)
	}
}

func (Suite) TestNewArchive(c *gocheck.C) {
	tmpdir, err := commandmocker.Add("git", "success")
	c.Assert(err, gocheck.IsNil)
	defer commandmocker.Remove(tmpdir)
	path, _ := filepath.Abs("testdata/test.git")
	archive, err := NewArchive(path, "e101294022323", "/tmp/archive-server", "sproject")
	c.Assert(err, gocheck.IsNil)
	sess, err := conn()
	c.Assert(err, gocheck.IsNil)
	defer sess.Close()
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	c.Assert(archive.Status, gocheck.Equals, StatusBuilding)
	c.Assert(archive.Path, gocheck.Equals, "/tmp/archive-server/"+archive.ID+".tar.gz")
	wait(c, 3e9, func() bool {
		count, err := sess.Collection(collectionName).Find(bson.M{"_id": archive.ID, "status": StatusReady}).Count()
		return err == nil && count == 1
	})
	c.Assert(commandmocker.Ran(tmpdir), gocheck.Equals, true)
	expected := []string{
		"archive", "--format=tar.gz",
		"--output=/tmp/archive-server/" + archive.ID + ".tar.gz",
		"--prefix=sproject/", "e101294022323",
	}
	c.Assert(commandmocker.Parameters(tmpdir), gocheck.DeepEquals, expected)
	err = sess.Collection(collectionName).FindId(archive.ID).One(&archive)
	c.Assert(err, gocheck.IsNil)
	c.Assert(archive.Status, gocheck.Equals, StatusReady)
	c.Assert(archive.Log, gocheck.Equals, "success")
}

func (Suite) TestNewArchiveFailure(c *gocheck.C) {
	tmpdir, err := commandmocker.Error("git", "failed to generate file", 1)
	c.Assert(err, gocheck.IsNil)
	defer commandmocker.Remove(tmpdir)
	path, _ := filepath.Abs("testdata/test.git")
	archive, err := NewArchive(path, "e101294022323", "/tmp/archive-server", "sproject")
	c.Assert(err, gocheck.IsNil)
	sess, err := conn()
	c.Assert(err, gocheck.IsNil)
	defer sess.Close()
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	wait(c, 3e9, func() bool {
		count, err := sess.Collection(collectionName).Find(bson.M{"_id": archive.ID, "status": StatusError}).Count()
		return err == nil && count == 1
	})
	c.Assert(commandmocker.Ran(tmpdir), gocheck.Equals, true)
	err = sess.Collection(collectionName).FindId(archive.ID).One(&archive)
	c.Assert(err, gocheck.IsNil)
	c.Assert(archive.Status, gocheck.Equals, StatusError)
	c.Assert(archive.Log, gocheck.Equals, "failed to generate file")
}

func (Suite) TestGetArchive(c *gocheck.C) {
	id := "some interesting id"
	archive := Archive{ID: id, Path: "/tmp/archive.tar.gz", Status: StatusBuilding}
	sess, err := conn()
	c.Assert(err, gocheck.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	gotArchive, err := GetArchive(archive.ID)
	c.Assert(err, gocheck.IsNil)
	c.Assert(*gotArchive, gocheck.DeepEquals, archive)
}

func (Suite) TestGetArchiveNotFound(c *gocheck.C) {
	archive, err := GetArchive("wat")
	c.Assert(archive, gocheck.IsNil)
	c.Assert(err, gocheck.Equals, ErrArchiveNotFound)
}

func (Suite) TestDestroyArchive(c *gocheck.C) {
	tmpdir := os.TempDir()
	path := filepath.Join(tmpdir, "some-temp-file.txt")
	file, err := os.Create(path)
	c.Assert(err, gocheck.IsNil)
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
	c.Assert(err, gocheck.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	err = DestroyArchive(archive.ID)
	c.Assert(err, gocheck.IsNil)
	_, err = os.Stat(path)
	c.Assert(os.IsNotExist(err), gocheck.Equals, true)
	err = sess.Collection(collectionName).FindId(archive.ID).One(&archive)
	c.Assert(err, gocheck.IsNil)
	c.Assert(archive.Status, gocheck.Equals, StatusDestroyed)
	c.Assert(archive.UpdatedAt, gocheck.Not(gocheck.DeepEquals), t)
}

func (Suite) TestDestroyArchiveNotFound(c *gocheck.C) {
	err := DestroyArchive("waaat")
	c.Assert(err, gocheck.Equals, ErrArchiveNotFound)
}

func wait(c *gocheck.C, timeout time.Duration, fn func() bool) {
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

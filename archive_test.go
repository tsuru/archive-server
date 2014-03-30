// Copyright 2014 Globo.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
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
	defer sess.Collection("archives").RemoveId(archive.ID)
	c.Assert(archive.Status, gocheck.Equals, StatusBuilding)
	c.Assert(archive.Path, gocheck.Equals, "/tmp/archive-server/"+archive.ID+".tar.gz")
	wait(c, 3e9, func() bool {
		count, err := sess.Collection("archives").Find(bson.M{"_id": archive.ID, "status": StatusReady}).Count()
		return err == nil && count == 1
	})
	c.Assert(commandmocker.Ran(tmpdir), gocheck.Equals, true)
	expected := []string{
		"archive", "--format=tar.gz",
		"--output=/tmp/archive-server/" + archive.ID + ".tar.gz",
		"--prefix=sproject/", "e101294022323",
	}
	c.Assert(commandmocker.Parameters(tmpdir), gocheck.DeepEquals, expected)
	err = sess.Collection("archives").FindId(archive.ID).One(&archive)
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
	defer sess.Collection("archives").RemoveId(archive.ID)
	wait(c, 3e9, func() bool {
		count, err := sess.Collection("archives").Find(bson.M{"_id": archive.ID, "status": StatusError}).Count()
		return err == nil && count == 1
	})
	c.Assert(commandmocker.Ran(tmpdir), gocheck.Equals, true)
	err = sess.Collection("archives").FindId(archive.ID).One(&archive)
	c.Assert(err, gocheck.IsNil)
	c.Assert(archive.Status, gocheck.Equals, StatusError)
	c.Assert(archive.Log, gocheck.Equals, "failed to generate file")
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

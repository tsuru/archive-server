// Copyright 2014 Globo.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"launchpad.net/gocheck"
	"testing"
)

type Suite struct{}

var _ = gocheck.Suite(Suite{})

func Test(t *testing.T) {
	gocheck.TestingT(t)
}

func (Suite) SetUpSuite(c *gocheck.C) {
	databaseAddr = "127.0.0.1:27017"
	databaseName = "archive_server_test"
}

func (Suite) TearDownSuite(c *gocheck.C) {
	sess, err := conn()
	c.Assert(err, gocheck.IsNil)
	sess.Collection("something").Database.DropDatabase()
}

func (Suite) TestConn(c *gocheck.C) {
	sess, err := conn()
	c.Assert(err, gocheck.IsNil)
	defer sess.Close()
	err = sess.Collection("something").Database.Session.Ping()
	c.Assert(err, gocheck.IsNil)
}

// Copyright 2014 Globo.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"launchpad.net/gocheck"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	baseDir = "/tmp/archive-server-tests"
	os.MkdirAll(baseDir, 0755)
}

func (Suite) TearDownSuite(c *gocheck.C) {
	sess, err := conn()
	c.Assert(err, gocheck.IsNil)
	sess.Collection("something").Database.DropDatabase()
	os.RemoveAll(baseDir)
}

func (Suite) TestConn(c *gocheck.C) {
	sess, err := conn()
	c.Assert(err, gocheck.IsNil)
	defer sess.Close()
	err = sess.Collection("something").Database.Session.Ping()
	c.Assert(err, gocheck.IsNil)
}

func (Suite) TestCreateArchiveHandler(c *gocheck.C) {
	path, _ := filepath.Abs("testdata/test.git")
	body := fmt.Sprintf("path=%s&refid=e101294022323&prefix=sproject", path)
	request, err := http.NewRequest("POST", "/", strings.NewReader(body))
	c.Assert(err, gocheck.IsNil)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	createArchiveHandler(recorder, request)
	var m map[string]string
	err = json.NewDecoder(recorder.Body).Decode(&m)
	c.Assert(err, gocheck.IsNil)
	_, err = GetArchive(m["id"])
	c.Assert(err, gocheck.IsNil)
}

func (Suite) TestCreateArchiveHandlerMissingParams(c *gocheck.C) {
	path, _ := filepath.Abs("testdata/test.git")
	body := fmt.Sprintf("path=%s&prefix=sproject", path)
	request, err := http.NewRequest("POST", "/", strings.NewReader(body))
	c.Assert(err, gocheck.IsNil)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	createArchiveHandler(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusBadRequest)
	c.Assert(recorder.Body.String(), gocheck.Equals, "path and refid are required\n")
}

func (Suite) TestCreateArchiveHandlerArchiveFailure(c *gocheck.C) {
	oldDbAddr := databaseAddr
	databaseAddr = "256.256.256.256:27017"
	defer func() { databaseAddr = oldDbAddr }()
	path, _ := filepath.Abs("testdata/test.git")
	body := fmt.Sprintf("path=%s&refid=e101294022323&prefix=sproject", path)
	request, err := http.NewRequest("POST", "/", strings.NewReader(body))
	c.Assert(err, gocheck.IsNil)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	createArchiveHandler(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusInternalServerError)
}

func (Suite) TestReadArchiveHandlerStatusReady(c *gocheck.C) {
	var buf bytes.Buffer
	testFilePath := "/tmp/archive.tar.gz"
	defer os.Remove(testFilePath)
	dst, err := os.Create(testFilePath)
	c.Assert(err, gocheck.IsNil)
	src, err := os.Open("testdata/test.tar.gz")
	c.Assert(err, gocheck.IsNil)
	io.Copy(dst, src)
	dst.Close()
	src.Seek(0, 0)
	io.Copy(&buf, src)
	src.Close()
	id := "some interesting id"
	archive := Archive{ID: id, Path: testFilePath, Status: StatusReady}
	sess, err := conn()
	c.Assert(err, gocheck.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	request, err := http.NewRequest("GET", "/?id="+id, nil)
	c.Assert(err, gocheck.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusOK)
	c.Assert(recorder.Body.Bytes(), gocheck.DeepEquals, buf.Bytes())
	err = sess.Collection(collectionName).FindId(id).One(&archive)
	c.Assert(err, gocheck.IsNil)
	c.Assert(archive.Status, gocheck.Equals, StatusDestroyed)
	_, err = os.Stat(testFilePath)
	c.Assert(os.IsNotExist(err), gocheck.Equals, true)
}

func (Suite) TestReadArchiveHandlerStatusReadyFileNotfound(c *gocheck.C) {
	id := "some interesting id"
	archive := Archive{
		ID:     id,
		Path:   "/tmp/file-that-doesnt-exist-29192.tar.gz",
		Status: StatusReady,
	}
	sess, err := conn()
	c.Assert(err, gocheck.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	request, err := http.NewRequest("GET", "/?id="+id, nil)
	c.Assert(err, gocheck.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusInternalServerError)
	expectedErr := "open /tmp/file-that-doesnt-exist-29192.tar.gz: no such file or directory\n"
	c.Assert(recorder.Body.String(), gocheck.Equals, expectedErr)
}

func (Suite) TestReadArchiveHandlerStatusReadyKeep(c *gocheck.C) {
	var buf bytes.Buffer
	testFilePath := "/tmp/archive.tar.gz"
	defer os.Remove(testFilePath)
	dst, err := os.Create(testFilePath)
	c.Assert(err, gocheck.IsNil)
	src, err := os.Open("testdata/test.tar.gz")
	c.Assert(err, gocheck.IsNil)
	io.Copy(dst, src)
	dst.Close()
	src.Seek(0, 0)
	io.Copy(&buf, src)
	src.Close()
	id := "some interesting id"
	archive := Archive{ID: id, Path: testFilePath, Status: StatusReady}
	sess, err := conn()
	c.Assert(err, gocheck.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	request, err := http.NewRequest("GET", "/?keep=1&id="+id, nil)
	c.Assert(err, gocheck.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusOK)
	c.Assert(recorder.Body.Bytes(), gocheck.DeepEquals, buf.Bytes())
	err = sess.Collection(collectionName).FindId(id).One(&archive)
	c.Assert(err, gocheck.IsNil)
	c.Assert(archive.Status, gocheck.Equals, StatusReady)
	_, err = os.Stat(testFilePath)
	c.Assert(err, gocheck.IsNil)
}

func (Suite) TestReadArchiveHandlerStatusDestroyed(c *gocheck.C) {
	id := "some interesting id"
	archive := Archive{ID: id, Path: "/tmp/file.tar.gz", Status: StatusDestroyed}
	sess, err := conn()
	c.Assert(err, gocheck.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	request, err := http.NewRequest("GET", "/?id="+id, nil)
	c.Assert(err, gocheck.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusNotFound)
	c.Assert(recorder.Body.String(), gocheck.Equals, ErrArchiveNotFound.Error()+"\n")
}

func (Suite) TestReadArchiveHandlerStatusBuilding(c *gocheck.C) {
	id := "some interesting id"
	archive := Archive{ID: id, Path: "/tmp/file.tar.gz", Status: StatusBuilding}
	sess, err := conn()
	c.Assert(err, gocheck.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	request, err := http.NewRequest("GET", "/?id="+id, nil)
	c.Assert(err, gocheck.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusOK)
	c.Assert(recorder.Body.String(), gocheck.Equals, "BUILDING\n")
}

func (Suite) TestReadArchiveHandlerStatusError(c *gocheck.C) {
	id := "some interesting id"
	archive := Archive{
		ID:     id,
		Path:   "/tmp/file.tar.gz",
		Status: StatusError,
		Log:    "something went wrong",
	}
	sess, err := conn()
	c.Assert(err, gocheck.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	request, err := http.NewRequest("GET", "/?id="+id, nil)
	c.Assert(err, gocheck.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusInternalServerError)
	c.Assert(recorder.Body.String(), gocheck.Equals, "something went wrong\n")
}

func (Suite) TestReadArchiveHandlerUnknownStatus(c *gocheck.C) {
	id := "some interesting id"
	archive := Archive{ID: id, Path: "/tmp/file.tar.gz", Status: 7}
	sess, err := conn()
	c.Assert(err, gocheck.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	request, err := http.NewRequest("GET", "/?id="+id, nil)
	c.Assert(err, gocheck.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusInternalServerError)
	c.Assert(recorder.Body.String(), gocheck.Equals, "unknown error\n")
}

func (Suite) TestReadArchiveHandlerMissingID(c *gocheck.C) {
	request, err := http.NewRequest("GET", "/", nil)
	c.Assert(err, gocheck.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusBadRequest)
	c.Assert(recorder.Body.String(), gocheck.Equals, "missing archive id\n")
}

func (Suite) TestReadArchiveHandlerNotFound(c *gocheck.C) {
	request, err := http.NewRequest("GET", "/?id=somethingnotfound", nil)
	c.Assert(err, gocheck.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusNotFound)
	c.Assert(recorder.Body.String(), gocheck.Equals, ErrArchiveNotFound.Error()+"\n")
}

func (Suite) TestReadArchiveHandlerDBFailure(c *gocheck.C) {
	oldDbAddr := databaseAddr
	databaseAddr = "256.256.256.256:27017"
	defer func() { databaseAddr = oldDbAddr }()
	request, err := http.NewRequest("GET", "/?id=somethingnotfound", nil)
	c.Assert(err, gocheck.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusInternalServerError)
}

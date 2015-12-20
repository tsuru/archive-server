// Copyright 2015 Globo.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/check.v1"
)

type Suite struct{}

var _ = check.Suite(Suite{})

func Test(t *testing.T) {
	check.TestingT(t)
}

func (Suite) SetUpSuite(c *check.C) {
	databaseAddr = "127.0.0.1:27017"
	databaseName = "archive_server_test"
	baseDir = "/tmp/archive-server-tests"
	os.MkdirAll(baseDir, 0755)
	log.SetOutput(ioutil.Discard)
}

func (Suite) TearDownSuite(c *check.C) {
	sess, err := conn()
	c.Assert(err, check.IsNil)
	sess.Collection("something").Database.DropDatabase()
	os.RemoveAll(baseDir)
}

func (Suite) TestConn(c *check.C) {
	sess, err := conn()
	c.Assert(err, check.IsNil)
	defer sess.Close()
	err = sess.Collection("something").Database.Session.Ping()
	c.Assert(err, check.IsNil)
}

func (Suite) TestCreateArchiveHandler(c *check.C) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("archive", "app_commit_uuid.tar.gz")
	c.Assert(err, check.IsNil)
	file.Write([]byte("hello world!"))
	writer.Close()
	request, err := http.NewRequest("POST", "/", strings.NewReader(string(body.Bytes())))
	c.Assert(err, check.IsNil)
	request.Header.Set("Content-Type", "multipart/form-data; boundary="+writer.Boundary())
	recorder := httptest.NewRecorder()
	createArchiveHandler(recorder, request)
	var m map[string]string
	err = json.NewDecoder(recorder.Body).Decode(&m)
	c.Assert(err, check.IsNil)
	_, err = GetArchive(m["id"])
	c.Assert(err, check.IsNil)
}

func (Suite) TestCreateArchiveHandlerMissingParams(c *check.C) {
	request, err := http.NewRequest("POST", "/", strings.NewReader(""))
	c.Assert(err, check.IsNil)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.Close()
	request.Header.Set("Content-Type", "multipart/form-data; boundary="+writer.Boundary())
	recorder := httptest.NewRecorder()
	createArchiveHandler(recorder, request)
	c.Assert(recorder.Code, check.Equals, http.StatusBadRequest)
	c.Assert(recorder.Body.String(), check.Equals, "missing archive file\n")
}

func (Suite) TestCreateArchiveHandlerArchiveFailure(c *check.C) {
	oldDbAddr := databaseAddr
	databaseAddr = "256.256.256.256:27017"
	defer func() { databaseAddr = oldDbAddr }()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("archive", "app_commit_uuid.tar.gz")
	c.Assert(err, check.IsNil)
	file.Write([]byte("hello world!"))
	writer.Close()
	request, err := http.NewRequest("POST", "/", strings.NewReader(string(body.Bytes())))
	c.Assert(err, check.IsNil)
	request.Header.Set("Content-Type", "multipart/form-data; boundary="+writer.Boundary())
	recorder := httptest.NewRecorder()
	createArchiveHandler(recorder, request)
	c.Assert(recorder.Code, check.Equals, http.StatusInternalServerError)
}

func (Suite) TestCreateArchiveHandlerLegacy(c *check.C) {
	path, _ := filepath.Abs("testdata/test.git")
	body := fmt.Sprintf("path=%s&refid=e101294022323&prefix=sproject", path)
	request, err := http.NewRequest("POST", "/", strings.NewReader(body))
	c.Assert(err, check.IsNil)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	createArchiveHandler(recorder, request)
	var m map[string]string
	err = json.NewDecoder(recorder.Body).Decode(&m)
	c.Assert(err, check.IsNil)
	_, err = GetArchive(m["id"])
	c.Assert(err, check.IsNil)
}

func (Suite) TestReadArchiveHandlerStatusReady(c *check.C) {
	var buf bytes.Buffer
	testFilePath := "/tmp/archive.tar.gz"
	defer os.Remove(testFilePath)
	dst, err := os.Create(testFilePath)
	c.Assert(err, check.IsNil)
	src, err := os.Open("testdata/test.tar.gz")
	c.Assert(err, check.IsNil)
	io.Copy(dst, src)
	dst.Close()
	src.Seek(0, 0)
	io.Copy(&buf, src)
	src.Close()
	id := "some interesting id"
	archive := Archive{ID: id, Path: testFilePath, Status: StatusReady}
	sess, err := conn()
	c.Assert(err, check.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	request, err := http.NewRequest("GET", "/?id="+id, nil)
	c.Assert(err, check.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, check.Equals, http.StatusOK)
	c.Assert(recorder.Body.Bytes(), check.DeepEquals, buf.Bytes())
	err = sess.Collection(collectionName).FindId(id).One(&archive)
	c.Assert(err, check.IsNil)
	c.Assert(archive.Status, check.Equals, StatusDestroyed)
	_, err = os.Stat(testFilePath)
	c.Assert(os.IsNotExist(err), check.Equals, true)
}

func (Suite) TestReadArchiveHandlerStatusReadyFileNotfound(c *check.C) {
	id := "some interesting id"
	archive := Archive{
		ID:     id,
		Path:   "/tmp/file-that-doesnt-exist-29192.tar.gz",
		Status: StatusReady,
	}
	sess, err := conn()
	c.Assert(err, check.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	request, err := http.NewRequest("GET", "/?id="+id, nil)
	c.Assert(err, check.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, check.Equals, http.StatusInternalServerError)
	expectedErr := "open /tmp/file-that-doesnt-exist-29192.tar.gz: no such file or directory\n"
	c.Assert(recorder.Body.String(), check.Equals, expectedErr)
}

func (Suite) TestReadArchiveHandlerStatusReadyKeep(c *check.C) {
	var buf bytes.Buffer
	testFilePath := "/tmp/archive.tar.gz"
	defer os.Remove(testFilePath)
	dst, err := os.Create(testFilePath)
	c.Assert(err, check.IsNil)
	src, err := os.Open("testdata/test.tar.gz")
	c.Assert(err, check.IsNil)
	io.Copy(dst, src)
	dst.Close()
	src.Seek(0, 0)
	io.Copy(&buf, src)
	src.Close()
	id := "some interesting id"
	archive := Archive{ID: id, Path: testFilePath, Status: StatusReady}
	sess, err := conn()
	c.Assert(err, check.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	request, err := http.NewRequest("GET", "/?keep=1&id="+id, nil)
	c.Assert(err, check.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, check.Equals, http.StatusOK)
	c.Assert(recorder.Body.Bytes(), check.DeepEquals, buf.Bytes())
	err = sess.Collection(collectionName).FindId(id).One(&archive)
	c.Assert(err, check.IsNil)
	c.Assert(archive.Status, check.Equals, StatusReady)
	_, err = os.Stat(testFilePath)
	c.Assert(err, check.IsNil)
}

func (Suite) TestReadArchiveHandlerStatusDestroyed(c *check.C) {
	id := "some interesting id"
	archive := Archive{ID: id, Path: "/tmp/file.tar.gz", Status: StatusDestroyed}
	sess, err := conn()
	c.Assert(err, check.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	request, err := http.NewRequest("GET", "/?id="+id, nil)
	c.Assert(err, check.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, check.Equals, http.StatusNotFound)
	c.Assert(recorder.Body.String(), check.Equals, ErrArchiveNotFound.Error()+"\n")
}

func (Suite) TestReadArchiveHandlerStatusBuilding(c *check.C) {
	id := "some interesting id"
	archive := Archive{ID: id, Path: "/tmp/file.tar.gz", Status: StatusBuilding}
	sess, err := conn()
	c.Assert(err, check.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	request, err := http.NewRequest("GET", "/?id="+id, nil)
	c.Assert(err, check.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, check.Equals, http.StatusOK)
	c.Assert(recorder.Body.String(), check.Equals, "BUILDING\n")
}

func (Suite) TestReadArchiveHandlerStatusError(c *check.C) {
	id := "some interesting id"
	archive := Archive{
		ID:     id,
		Path:   "/tmp/file.tar.gz",
		Status: StatusError,
		Log:    "something went wrong",
	}
	sess, err := conn()
	c.Assert(err, check.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	request, err := http.NewRequest("GET", "/?id="+id, nil)
	c.Assert(err, check.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, check.Equals, http.StatusInternalServerError)
	c.Assert(recorder.Body.String(), check.Equals, "something went wrong\n")
}

func (Suite) TestReadArchiveHandlerUnknownStatus(c *check.C) {
	id := "some interesting id"
	archive := Archive{ID: id, Path: "/tmp/file.tar.gz", Status: 7}
	sess, err := conn()
	c.Assert(err, check.IsNil)
	defer sess.Close()
	sess.Collection(collectionName).Insert(archive)
	defer sess.Collection(collectionName).RemoveId(archive.ID)
	request, err := http.NewRequest("GET", "/?id="+id, nil)
	c.Assert(err, check.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, check.Equals, http.StatusInternalServerError)
	c.Assert(recorder.Body.String(), check.Equals, "unknown error\n")
}

func (Suite) TestReadArchiveHandlerMissingID(c *check.C) {
	request, err := http.NewRequest("GET", "/", nil)
	c.Assert(err, check.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, check.Equals, http.StatusBadRequest)
	c.Assert(recorder.Body.String(), check.Equals, "missing archive id\n")
}

func (Suite) TestReadArchiveHandlerNotFound(c *check.C) {
	request, err := http.NewRequest("GET", "/?id=somethingnotfound", nil)
	c.Assert(err, check.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, check.Equals, http.StatusNotFound)
	c.Assert(recorder.Body.String(), check.Equals, ErrArchiveNotFound.Error()+"\n")
}

func (Suite) TestReadArchiveHandlerDBFailure(c *check.C) {
	oldDbAddr := databaseAddr
	databaseAddr = "256.256.256.256:27017"
	defer func() { databaseAddr = oldDbAddr }()
	request, err := http.NewRequest("GET", "/?id=somethingnotfound", nil)
	c.Assert(err, check.IsNil)
	recorder := httptest.NewRecorder()
	readArchiveHandler(recorder, request)
	c.Assert(recorder.Code, check.Equals, http.StatusInternalServerError)
}

// Copyright 2014 Globo.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/globocom/tsuru/db/storage"
)

var databaseAddr, databaseName string

func conn() (*storage.Storage, error) {
	return storage.Open(databaseAddr, databaseName)
}

func main() {
}

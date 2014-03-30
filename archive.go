// Copyright 2014 Globo.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

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
}

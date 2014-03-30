// Copyright 2014 Globo.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
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

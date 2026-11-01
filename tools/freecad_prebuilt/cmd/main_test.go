// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/filmil/bazel_rules_cam/tools/freecad_prebuilt"
)

func TestWorkRequired(t *testing.T) {
	if _, err := plan(nil, "/wd", io.Discard); err == nil || !strings.Contains(err.Error(), "--work is required") {
		t.Errorf("got %v", err)
	}
}

func TestRelativeWorkIsTheCallers(t *testing.T) {
	c, err := plan([]string{"--work=fc"}, "/home/op", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if c.env[0] != "FREECAD_WORK=/home/op/fc" {
		t.Errorf("env %v", c.env)
	}
	if _, err := plan([]string{"--work=fc"}, "", io.Discard); err == nil {
		t.Error("a relative --work without BUILD_WORKING_DIRECTORY: want an error")
	}
}

func TestEnvAndUpload(t *testing.T) {
	c, err := plan([]string{"--work=/w"}, "/wd", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"FREECAD_WORK=/w", "EXPECTED_SHA256=" + prebuilt.ExpectedSHA256}
	if !reflect.DeepEqual(c.env, want) || len(c.args) != 2 {
		t.Errorf("got %v %v", c.args, c.env)
	}
	c, err = plan([]string{"--work=/w", "--upload"}, "/wd", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if c.args[len(c.args)-1] != "--upload" {
		t.Errorf("--upload not passed: %v", c.args)
	}
}

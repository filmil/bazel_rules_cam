// SPDX-License-Identifier: Apache-2.0

// dressup_test reads the tagged plate job's G-code (the test's argument):
// the outline is posted once, by its holding-tag dressup, and not again by
// the Profile the dressup wraps (issue #36), which would cut the tags away.
package testing

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var beginOp = regexp.MustCompile(`(?im)^\(begin operation:\s*(.*?)\s*\)`)

func TestDressupPostedOnce(t *testing.T) {
	if flag.NArg() != 1 {
		t.Fatal("give the tagged job's G-code as the argument")
	}
	data, err := os.ReadFile(filepath.Join(os.Getenv("TEST_SRCDIR"), os.Getenv("TEST_WORKSPACE"), flag.Arg(0)))
	if err != nil {
		t.Fatal(err)
	}
	var ops []string
	for _, m := range beginOp.FindAllStringSubmatch(string(data), -1) {
		ops = append(ops, m[1])
	}
	got := strings.Join(ops, ", ")
	if !strings.Contains(got, "DressupTag") {
		t.Errorf("no DressupTag operation: %s", got)
	}
	for _, op := range ops {
		if op == "Profile" {
			t.Errorf("the Profile the tags wrap is posted too: %s", got)
		}
	}
}

// SPDX-License-Identifier: Apache-2.0

// dressup_test reads dressed jobs' G-code: each posts its outline once, by
// its outer dressup, and not again by the operations the dressup wraps
// (issue #36), which would cut the tags away. Each argument is a job's
// G-code, "=", and the comma-separated operations it must post, in order.
package testing

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var beginOp = regexp.MustCompile(`(?im)^\(begin operation:\s*(.*?)\s*\)`)

func TestDressupPostedOnce(t *testing.T) {
	if flag.NArg() == 0 {
		t.Fatal("give <G-code>=<operations> arguments")
	}
	for _, arg := range flag.Args() {
		path, want, ok := strings.Cut(arg, "=")
		if !ok {
			t.Fatalf("argument %q: want <G-code>=<operations>", arg)
		}
		data, err := os.ReadFile(filepath.Join(os.Getenv("TEST_SRCDIR"), os.Getenv("TEST_WORKSPACE"), path))
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, m := range beginOp.FindAllStringSubmatch(string(data), -1) {
			got = append(got, m[1])
		}
		if w := strings.Split(want, ","); !reflect.DeepEqual(got, w) {
			t.Errorf("%s posts operations %q, want %q", path, got, w)
		}
	}
}

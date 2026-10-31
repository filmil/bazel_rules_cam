// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFilesAndDirs(t *testing.T) {
	dir := t.TempDir()
	tree := filepath.Join(dir, "gerbers")
	if err := os.Mkdir(tree, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "sub", "out.pdf")
	var stdout bytes.Buffer
	if err := Write(file+"\n"+tree+"\n", "", []string{"sch", "export", "pdf"}, &stdout); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{file, filepath.Join(tree, "stub_record")} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(b), Marker+"\n") || !strings.Contains(string(b), `["sch","export","pdf"]`) {
			t.Errorf("%s: got %q", p, b)
		}
	}
}

// The output that is the tool's stdout gets the record through stdout, and
// no file is written for it.
func TestWriteStdoutOutput(t *testing.T) {
	dir := t.TempDir()
	file, out := filepath.Join(dir, "a.txt"), filepath.Join(dir, "version.txt")
	var stdout bytes.Buffer
	if err := Write(file+"\n"+out, out, []string{"version"}, &stdout); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), Record([]string{"version"})) {
		t.Errorf("stdout got %q", stdout.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("%s was written as a file too", out)
	}
	if _, err := os.Stat(file); err != nil {
		t.Errorf("the other output is missing: %v", err)
	}
}

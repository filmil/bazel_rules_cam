// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "tools.json"), filepath.Join(dir, "out.json")
	write := func(s string) {
		if err := os.WriteFile(in, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write(`{"tools": [{"number": 1, "name": "em", "shape": "endmill", "diameter": 3.0, "length": 38, "cutting_edge_height": 12}]}`)
	if err := run(in, out); err != nil {
		t.Fatalf("good library: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(got), `"diameter": 3,`) {
		t.Errorf("output %q, %v; want the canonical library", got, err)
	}

	// A bad library: an error naming the count, and no output written.
	os.Remove(out)
	write(`{"tools": [{"number": 1, "name": "em", "shape": "endmill", "length": 38, "cutting_edge_height": 12, "flutes": 0}]}`)
	err = run(in, out)
	if err == nil || !strings.Contains(err.Error(), "2 problem(s)") {
		t.Errorf("bad library: %v; want 2 problem(s)", err)
	}
	if _, serr := os.Stat(out); !os.IsNotExist(serr) {
		t.Errorf("a bad library wrote %s", out)
	}

	if err := run(filepath.Join(dir, "missing.json"), out); err == nil {
		t.Errorf("a missing input gave no error")
	}
}

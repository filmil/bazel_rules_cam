// SPDX-License-Identifier: Apache-2.0

// Command stub stands in for any tool in the swap test.
//
// It creates every output the launcher lists in RUN_TOOL_OUTPUTS and writes
// into each a record: the Marker line, then its own arguments as JSON. An
// output that Bazel made as a directory (a tree artifact) gets the record in
// a file named stub_record inside it. The output named in RUN_TOOL_STDOUT,
// which the launcher already opened as the tool's standard output, gets the
// record through standard output instead, so two file handles never write
// the same file. The swap test then checks that every
// output of the targets under test holds a record, which shows that every
// action ran a tool from a toolchain.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Marker starts every record.
const Marker = "RUN_TOOL_STUB"

// Record returns what the stub writes for args.
func Record(args []string) []byte {
	j, _ := json.Marshal(args)
	return []byte(Marker + "\n" + string(j) + "\n")
}

// Write creates each output in outputs (one path per line) with the record.
// The output stdoutPath gets it through stdout instead.
func Write(outputs, stdoutPath string, args []string, stdout io.Writer) error {
	rec := Record(args)
	for _, p := range strings.Split(outputs, "\n") {
		if p == "" {
			continue
		}
		if stdoutPath != "" && p == stdoutPath {
			if _, err := stdout.Write(rec); err != nil {
				return err
			}
			continue
		}
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			p = filepath.Join(p, "stub_record")
		} else if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, rec, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := Write(os.Getenv("RUN_TOOL_OUTPUTS"), os.Getenv("RUN_TOOL_STDOUT"), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "stub:", err)
		os.Exit(1)
	}
}

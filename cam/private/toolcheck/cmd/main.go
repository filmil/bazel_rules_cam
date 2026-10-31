// SPDX-License-Identifier: Apache-2.0

// Command toolcheck validates a tool library and writes its canonical form.
//
//	toolcheck --in tools.json --out canonical.json
//
// It prints every problem it finds, then exits 1, writing nothing.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/filmil/bazel_rules_cam/cam/private/toolcheck"
)

func run(in, out string) error {
	data, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	l, err := toolcheck.Parse(data)
	if err != nil {
		return fmt.Errorf("%s: %v", in, err)
	}
	if errs := toolcheck.Check(l); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "%s: %s\n", in, e)
		}
		return fmt.Errorf("%s: %d problem(s)", in, len(errs))
	}
	c, err := toolcheck.Canonical(l)
	if err != nil {
		return err
	}
	return os.WriteFile(out, c, 0o644)
}

func main() {
	in := flag.String("in", "", "the tool library (JSON)")
	out := flag.String("out", "", "where to write its canonical form")
	flag.Parse()
	if *in == "" || *out == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: toolcheck --in tools.json --out canonical.json")
		os.Exit(2)
	}
	if err := run(*in, *out); err != nil {
		fmt.Fprintln(os.Stderr, "toolcheck:", err)
		os.Exit(1)
	}
}

// SPDX-License-Identifier: Apache-2.0

// Command freecad_prebuilt makes the FreeCAD prebuilt tarball that the
// FreeCAD toolchain pins, from the official AppImage, and checks that it is
// the same bytes. The operator runs it by hand, on Linux x86_64, with curl,
// unsquashfs, readelf, python3, tar and xz on the host:
//
//	bazel run //tools:freecad_prebuilt -- --work=/data/tmp/freecad-prebuilt
//
// With --upload it also uploads the tarball as the release asset (gh, logged
// in). It is not part of any build.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/filmil/bazel_rules_cam/tools/freecad_prebuilt"
)

func main() {
	work := flag.String("work", "", "work directory: the AppImage, the tree and the tarball go here (required; about 8 GB)")
	upload := flag.Bool("upload", false, "upload the tarball to the release prebuilt-freecad-1.1.4")
	flag.Parse()
	if *work == "" {
		fmt.Fprintln(os.Stderr, "freecad_prebuilt: --work is required")
		os.Exit(2)
	}
	abs, err := filepath.Abs(*work)
	if err != nil {
		fmt.Fprintln(os.Stderr, "freecad_prebuilt:", err)
		os.Exit(2)
	}
	// The script feeds inline programs to python3 on standard input, so it
	// runs from a file, not from bash's standard input.
	f, err := os.CreateTemp("", "freecad-prebuilt-*.sh")
	if err != nil {
		fmt.Fprintln(os.Stderr, "freecad_prebuilt:", err)
		os.Exit(2)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(prebuilt.Script); err != nil {
		fmt.Fprintln(os.Stderr, "freecad_prebuilt:", err)
		os.Exit(2)
	}
	f.Close()
	args := []string{f.Name()}
	if *upload {
		args = append(args, "--upload")
	}
	c := exec.Command("bash", args...)
	c.Env = append(os.Environ(), "FREECAD_WORK="+abs, "EXPECTED_SHA256="+prebuilt.ExpectedSHA256)
	c.Stdin, c.Stdout, c.Stderr = nil, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "freecad_prebuilt:", err)
		os.Exit(1)
	}
}

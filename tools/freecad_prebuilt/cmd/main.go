// SPDX-License-Identifier: Apache-2.0

// Command freecad_prebuilt makes the FreeCAD prebuilt tarball that the
// FreeCAD toolchain pins, from the official AppImage, and checks that it is
// the same bytes. The operator runs it by hand, on Linux x86_64, with curl,
// unsquashfs, readelf, python3, tar and xz on the host:
//
//	bazel run //tools:freecad_prebuilt -- --work=/data/tmp/freecad-prebuilt
//
// A relative --work is taken from the directory bazel run was started in.
// With --upload it also uploads the tarball as the release asset (gh,
// logged in). It is not part of any build.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/filmil/bazel_rules_cam/tools/freecad_prebuilt"
)

// command is what run executes: bash, the script file, and its arguments,
// with the environment to add.
type command struct {
	args []string
	env  []string
}

// plan checks the flags and returns the command, the script to write to
// args[1], or an error. wd is the directory bazel run was started in.
func plan(argv []string, wd string, stderr io.Writer) (*command, error) {
	fs := flag.NewFlagSet("freecad_prebuilt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	work := fs.String("work", "", "work directory: the AppImage, the tree and the tarball go here (required; about 8 GB)")
	upload := fs.Bool("upload", false, "upload the tarball to the release prebuilt-freecad-1.1.4")
	if err := fs.Parse(argv); err != nil {
		return nil, err
	}
	if *work == "" {
		return nil, fmt.Errorf("--work is required")
	}
	dir := *work
	// bazel run starts the tool in its runfiles; a relative path means the
	// caller's directory, not that one.
	if !filepath.IsAbs(dir) {
		if wd == "" {
			return nil, fmt.Errorf("--work %q is relative and BUILD_WORKING_DIRECTORY is not set; give an absolute path", dir)
		}
		dir = filepath.Join(wd, dir)
	}
	c := &command{
		args: []string{"bash", "SCRIPT"},
		env:  []string{"FREECAD_WORK=" + filepath.Clean(dir), "EXPECTED_SHA256=" + prebuilt.ExpectedSHA256},
	}
	if *upload {
		c.args = append(c.args, "--upload")
	}
	return c, nil
}

func run() error {
	c, err := plan(os.Args[1:], os.Getenv("BUILD_WORKING_DIRECTORY"), os.Stderr)
	if err != nil {
		return err
	}
	// The script feeds inline programs to python3 on standard input, so it
	// runs from a file, not from bash's standard input.
	f, err := os.CreateTemp("", "freecad-prebuilt-*.sh")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(prebuilt.Script); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	c.args[1] = f.Name()
	x := exec.Command(c.args[0], c.args[1:]...)
	x.Env = append(os.Environ(), c.env...)
	x.Stdout, x.Stderr = os.Stdout, os.Stderr
	return x.Run()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "freecad_prebuilt:", err)
		os.Exit(1)
	}
}

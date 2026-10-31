// SPDX-License-Identifier: Apache-2.0

// Command exitcheck runs freecadcmd on scripts through freecad_script_args'
// bootstrap, and checks each exit status (the test exit_status_test).
//
// EXITCHECK_TOOL is freecadcmd's runfiles path. EXITCHECK_CASES is a JSON
// list of cases: {"script", "args", "want"}. freecadcmd runs with HOME and
// TMPDIR in the test's own temporary directory, and a fixed small
// environment.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Case is one run: the script, freecadcmd's arguments, and the exit
// status the run must give.
type Case struct {
	Script string   `json:"script"`
	Args   []string `json:"args"`
	Want   int      `json:"want"`
}

// ParseCases reads EXITCHECK_CASES.
func ParseCases(s string) ([]Case, error) {
	var cs []Case
	if err := json.Unmarshal([]byte(s), &cs); err != nil {
		return nil, fmt.Errorf("cases: %v", err)
	}
	if len(cs) == 0 {
		return nil, errors.New("no cases")
	}
	for _, c := range cs {
		if c.Script == "" || len(c.Args) == 0 {
			return nil, fmt.Errorf("case %+v: needs a script and arguments", c)
		}
	}
	return cs, nil
}

func run(tool string, c Case, tmp string) (int, string, error) {
	home, err := os.MkdirTemp(tmp, "home")
	if err != nil {
		return 0, "", err
	}
	cmd := exec.Command(tool, c.Args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + home, "TMPDIR=" + home, "LC_ALL=C.UTF-8", "TZ=UTC0"}
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), string(out), nil
	}
	return 0, string(out), err
}

func main() {
	cs, err := ParseCases(os.Getenv("EXITCHECK_CASES"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "exitcheck:", err)
		os.Exit(2)
	}
	root := os.Getenv("RUNFILES_DIR")
	if root == "" {
		root = os.Getenv("TEST_SRCDIR")
	}
	tool, err := filepath.Abs(filepath.Join(root, os.Getenv("EXITCHECK_TOOL")))
	if err != nil {
		fmt.Fprintln(os.Stderr, "exitcheck:", err)
		os.Exit(2)
	}
	bad := 0
	for _, c := range cs {
		got, out, err := run(tool, c, os.Getenv("TEST_TMPDIR"))
		script := c.Script
		switch {
		case err != nil:
			fmt.Printf("FAIL %s: %v\n", script, err)
			bad++
		case got != c.Want:
			fmt.Printf("FAIL %s: exit status %d, want %d\n%s\n", script, got, c.Want, out)
			bad++
		default:
			fmt.Printf("ok   %s: exit status %d\n", script, got)
		}
	}
	if bad > 0 {
		os.Exit(1)
	}
}

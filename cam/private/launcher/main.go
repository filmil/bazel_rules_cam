// SPDX-License-Identifier: Apache-2.0

// Command launcher runs one tool for a Bazel action with a private home.
//
// Usage:
//
//	launcher [--home-env NAME]... [--env NAME=VALUE]... [--env-abs NAME=PATH]...
//	         [--output PATH]... [--stdout PATH] -- TOOL ARGS...
//
// It makes a fresh directory inside the action's working directory, points
// HOME, TMPDIR and the XDG base directories at it (and every NAME given with
// --home-env at a directory of its own inside it), runs TOOL with ARGS,
// removes the directory, and exits with the tool's status. So a tool that
// writes settings or caches writes them where the action can see and
// nothing else can (spec requirement H3).
//
// --env-abs sets NAME to PATH made absolute against the action's working
// directory, for tools that read a path from the environment and resolve it
// against somewhere else (fontconfig reads FONTCONFIG_FILE that way).
// --stdout sends the tool's standard output to PATH instead.
//
// --output names a declared output of the action. The launcher passes the
// list to the tool in RUN_TOOL_OUTPUTS, one path per line; real tools
// ignore it, and the stub tool of the swap test creates those outputs.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Request is a parsed command line.
type Request struct {
	HomeEnv []string
	Env     []string
	EnvAbs  []string
	Outputs []string
	Stdout  string
	Tool    string
	Args    []string
}

// Parse reads the launcher's command line, without the program name.
func Parse(args []string) (Request, error) {
	var r Request
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			if i+1 >= len(args) {
				return r, errors.New("no tool after --")
			}
			r.Tool, r.Args = args[i+1], args[i+2:]
			return r, nil
		}
		if i+1 >= len(args) {
			return r, fmt.Errorf("%s needs a value", a)
		}
		v := args[i+1]
		i++
		switch a {
		case "--home-env":
			r.HomeEnv = append(r.HomeEnv, v)
		case "--env":
			if !strings.Contains(v, "=") {
				return r, fmt.Errorf("--env %q: want NAME=VALUE", v)
			}
			r.Env = append(r.Env, v)
		case "--env-abs":
			k, p, ok := strings.Cut(v, "=")
			if !ok || k == "" || p == "" {
				return r, fmt.Errorf("--env-abs %q: want NAME=PATH", v)
			}
			r.EnvAbs = append(r.EnvAbs, v)
		case "--stdout":
			if v == "" {
				return r, errors.New("--stdout needs a path")
			}
			if r.Stdout != "" {
				return r, errors.New("--stdout given twice")
			}
			r.Stdout = v
		case "--output":
			r.Outputs = append(r.Outputs, v)
		default:
			return r, fmt.Errorf("unknown flag %q", a)
		}
	}
	return r, errors.New("missing -- before the tool")
}

// Dropped are variables that would connect a tool to the host's desktop or
// user session. Bazel's strict action environment does not pass them, but
// an action run some other way might.
var Dropped = []string{
	"DISPLAY", "WAYLAND_DISPLAY", "DBUS_SESSION_BUS_ADDRESS",
	"XDG_RUNTIME_DIR", "XDG_CONFIG_DIRS", "XDG_DATA_DIRS", "XAUTHORITY",
}

// DroppedPrefixes start the names of variables that would point a tool at
// host libraries, settings or fonts: FreeCAD's (FREECAD_USER_HOME and the
// like), Python's (PYTHONPATH, PYTHONHOME), Qt's, the dynamic loader's
// (LD_LIBRARY_PATH, LD_PRELOAD) and fontconfig's. A toolchain sets the ones
// it needs with --env.
var DroppedPrefixes = []string{"FREECAD", "PYTHON", "QT_", "LD_", "FONTCONFIG_", "FC_"}

func dropped(k string, drop map[string]bool) bool {
	if drop[k] {
		return true
	}
	for _, p := range DroppedPrefixes {
		if strings.HasPrefix(k, p) {
			return true
		}
	}
	return false
}

// Environ returns the tool's environment: base without the Dropped
// variables, with the home variables pointing inside home, then the fixed
// --env values, then the --env-abs values made absolute against cwd.
func Environ(base []string, home, cwd string, r Request) []string {
	set := map[string]string{}
	var order []string
	put := func(k, v string) {
		if _, ok := set[k]; !ok {
			order = append(order, k)
		}
		set[k] = v
	}
	drop := map[string]bool{}
	for _, k := range Dropped {
		drop[k] = true
	}
	for _, kv := range base {
		k, v, _ := strings.Cut(kv, "=")
		if !dropped(k, drop) {
			put(k, v)
		}
	}
	put("HOME", home)
	put("TMPDIR", filepath.Join(home, "tmp"))
	put("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	put("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	put("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	put("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	for _, n := range r.HomeEnv {
		put(n, filepath.Join(home, n))
	}
	put("RUN_TOOL_OUTPUTS", strings.Join(r.Outputs, "\n"))
	if r.Stdout != "" {
		put("RUN_TOOL_STDOUT", r.Stdout)
	}
	for _, kv := range r.Env {
		k, v, _ := strings.Cut(kv, "=")
		put(k, v)
	}
	for _, kv := range r.EnvAbs {
		k, p, _ := strings.Cut(kv, "=")
		if !filepath.IsAbs(p) {
			p = filepath.Join(cwd, p)
		}
		put(k, p)
	}
	out := make([]string, 0, len(order))
	for _, k := range order {
		out = append(out, k+"="+set[k])
	}
	return out
}

// Dirs returns the directories Environ points at, which must exist.
func Dirs(home string, r Request) []string {
	d := []string{
		filepath.Join(home, "tmp"),
		filepath.Join(home, ".config"),
		filepath.Join(home, ".cache"),
		filepath.Join(home, ".local", "share"),
		filepath.Join(home, ".local", "state"),
	}
	for _, n := range r.HomeEnv {
		d = append(d, filepath.Join(home, n))
	}
	return d
}

// Run runs the request with a fresh home under dir and returns the tool's
// exit status. Relative paths in r are relative to dir.
func Run(r Request, dir string) (code int, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return 0, err
	}
	home, err := os.MkdirTemp(abs, "_run_tool_home_")
	if err != nil {
		return 0, err
	}
	defer removeAll(home)
	for _, d := range Dirs(home, r) {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return 0, err
		}
	}
	cmd := exec.Command(r.Tool, r.Args...)
	cmd.Env = Environ(os.Environ(), home, abs, r)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if r.Stdout != "" {
		p := r.Stdout
		if !filepath.IsAbs(p) {
			p = filepath.Join(abs, p)
		}
		f, ferr := os.Create(p)
		if ferr != nil {
			return 0, ferr
		}
		defer func() {
			if cerr := f.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}()
		cmd.Stdout = f
	}
	err = cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), nil
	}
	if err != nil {
		return 0, err
	}
	return 0, nil
}

// removeAll removes dir even when the tool left read-only directories in
// it.
func removeAll(dir string) {
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			os.Chmod(p, 0o755)
		}
		return nil
	})
	os.RemoveAll(dir)
}

func main() {
	r, err := Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "launcher:", err)
		os.Exit(2)
	}
	code, err := Run(r, ".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "launcher:", err)
		os.Exit(2)
	}
	os.Exit(code)
}

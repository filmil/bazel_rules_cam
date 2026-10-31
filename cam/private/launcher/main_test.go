// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	r, err := Parse([]string{"--home-env", "FREECAD_USER_HOME", "--env", "A=b=c", "--env-abs", "F=x/f.conf", "--output", "o/x", "--stdout", "o/out", "--", "tool", "-v", "--", "z"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Tool != "tool" || strings.Join(r.Args, " ") != "-v -- z" || r.HomeEnv[0] != "FREECAD_USER_HOME" || r.Env[0] != "A=b=c" ||
		r.EnvAbs[0] != "F=x/f.conf" || r.Outputs[0] != "o/x" || r.Stdout != "o/out" {
		t.Errorf("got %+v", r)
	}
	for _, bad := range [][]string{{"tool"}, {"--"}, {"--env", "NOEQUALS", "--", "t"}, {"--env-abs", "F=", "--", "t"}, {"--env-abs", "=p", "--", "t"}, {"--stdout", "", "--", "t"}, {"--stdout", "a", "--stdout", "b", "--", "t"}, {"--bogus", "x", "--", "t"}, {"--output"}} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("%q: want an error", bad)
		}
	}
}

func TestEnvironPointsInsideHome(t *testing.T) {
	r := Request{HomeEnv: []string{"FREECAD_USER_HOME"}, Env: []string{"HOME=/never", "X=1", "PYTHONDONTWRITEBYTECODE=1"}, EnvAbs: []string{"REL=ext/f.conf", "ABS=/etc/f.conf"}, Outputs: []string{"a", "b"}, Stdout: "b"}
	env := map[string]string{}
	for _, kv := range Environ([]string{"HOME=/host/home", "PATH=/bin:/usr/bin", "XDG_CONFIG_HOME=/host/cfg", "DISPLAY=:0", "XDG_RUNTIME_DIR=/run/user/1", "PYTHONPATH=/host/py", "PYTHONHOME=/usr", "LD_LIBRARY_PATH=/host/lib", "QT_PLUGIN_PATH=/host/qt", "FREECAD_USER_DATA=/host/fc", "FONTCONFIG_PATH=/etc/fonts", "FC_DEBUG=1"}, "/act/h", "/act", r) {
		k, v, _ := strings.Cut(kv, "=")
		env[k] = v
	}
	for k, want := range map[string]string{
		"XDG_CONFIG_HOME":   "/act/h/.config",
		"XDG_CACHE_HOME":    "/act/h/.cache",
		"TMPDIR":            "/act/h/tmp",
		"FREECAD_USER_HOME": "/act/h/FREECAD_USER_HOME",
		"PATH":              "/bin:/usr/bin",
		"X":                 "1",
		"RUN_TOOL_OUTPUTS":  "a\nb",
		// --env-abs: relative paths resolve against the working directory.
		"REL": "/act/ext/f.conf",
		// A fixed --env value passes even with a dropped prefix.
		"PYTHONDONTWRITEBYTECODE": "1",
		"RUN_TOOL_STDOUT":     "b",
		"ABS": "/etc/f.conf",
		// A fixed --env value wins, even over HOME.
		"HOME": "/never",
	} {
		if env[k] != want {
			t.Errorf("%s = %q, want %q", k, env[k], want)
		}
	}
	for _, k := range []string{"DISPLAY", "XDG_RUNTIME_DIR", "PYTHONPATH", "PYTHONHOME", "LD_LIBRARY_PATH", "QT_PLUGIN_PATH", "FREECAD_USER_DATA", "FONTCONFIG_PATH", "FC_DEBUG"} {
		if _, ok := env[k]; ok {
			t.Errorf("%s reached the tool", k)
		}
	}
}

// TestMain lets the test binary stand in for a tool: with
// LAUNCHER_TEST_TOOL set, it records what a tool sees and exits 3. No host
// shell or other host tool is run.
func TestMain(m *testing.M) {
	if os.Getenv("LAUNCHER_TEST_TOOL") == "stdout" {
		os.Stdout.WriteString("to stdout\n")
		os.Exit(0)
	}
	if os.Getenv("LAUNCHER_TEST_TOOL") == "1" {
		home, cfg, own := os.Getenv("HOME"), os.Getenv("XDG_CONFIG_HOME"), os.Getenv("TOOL_CONFIG")
		for _, p := range []string{filepath.Join(home, "written"), filepath.Join(cfg, "x"), filepath.Join(own, "y")} {
			if err := os.WriteFile(p, nil, 0o644); err != nil {
				os.Exit(10)
			}
		}
		// A read-only directory must not stop the cleanup.
		ro := filepath.Join(home, "ro")
		if os.Mkdir(ro, 0o755) != nil || os.WriteFile(filepath.Join(ro, "f"), nil, 0o444) != nil || os.Chmod(ro, 0o500) != nil {
			os.Exit(12)
		}
		if err := os.WriteFile(os.Args[1], []byte(home+"|"+cfg+"|"+own), 0o644); err != nil {
			os.Exit(11)
		}
		os.Exit(3)
	}
	os.Exit(m.Run())
}

// The tool sees a home inside the action directory, can write there, and
// the directory is gone afterwards. The tool's exit status comes back.
func TestRunUsesPrivateHome(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "seen")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := Request{
		HomeEnv: []string{"TOOL_CONFIG"},
		Env:     []string{"LAUNCHER_TEST_TOOL=1"},
		Tool:    self,
		Args:    []string{out},
	}
	code, err := Run(r, dir)
	if err != nil {
		t.Fatal(err)
	}
	if code != 3 {
		t.Errorf("exit code %d, want the tool's 3", code)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(b), "|")
	if len(parts) != 3 || !strings.HasPrefix(parts[0], dir+string(filepath.Separator)+"_run_tool_home_") ||
		parts[1] != filepath.Join(parts[0], ".config") || parts[2] != filepath.Join(parts[0], "TOOL_CONFIG") {
		t.Errorf("tool saw %q", parts)
	}
	if _, err := os.Stat(parts[0]); !os.IsNotExist(err) {
		t.Errorf("home %s still exists after the run", parts[0])
	}
}

// --stdout sends the tool's standard output to a file.
func TestRunStdoutToFile(t *testing.T) {
	dir := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.txt")
	// A relative --stdout is relative to the action directory.
	code, err := Run(Request{Env: []string{"LAUNCHER_TEST_TOOL=stdout"}, Stdout: "out.txt", Tool: self}, dir)
	if err != nil || code != 0 {
		t.Fatalf("code %d, err %v", code, err)
	}
	if b, err := os.ReadFile(out); err != nil || string(b) != "to stdout\n" {
		t.Errorf("stdout file has %q (%v)", b, err)
	}
}

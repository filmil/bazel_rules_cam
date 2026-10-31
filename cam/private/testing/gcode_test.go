// SPDX-License-Identifier: Apache-2.0

// gcode_test reads the plate job's G-code, one file per post-processor
// (the test's arguments), and checks what the job asked for: tool 1 only,
// its spindle speed and feeds, and the cut down to the part's bottom (Z 0:
// the stock has no margin below the part, and FreeCAD's final depth is the
// part's bottom).
// gcode_check_test (issue #10) checks G-code against a machine; this only
// checks that cam_job passed the job's settings through.
package testing

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var word = regexp.MustCompile(`([A-Z])(-?[0-9]+(?:\.[0-9]+)?)`)

// A tool change: "M6 T1", or "T1 M6". GRBL cannot change tools, so its post
// writes the change as a comment, "( M6 T1 )"; this finds it either way.
var toolChange = regexp.MustCompile(`M0?6\s*T(\d+)|T(\d+)\s*M0?6`)

func TestPlateGcode(t *testing.T) {
	if flag.NArg() == 0 {
		t.Fatal("give the G-code files as arguments")
	}
	for _, path := range flag.Args() {
		// rules_go runs the test in its package's directory; the
		// arguments are paths from the runfiles root.
		data, err := os.ReadFile(filepath.Join(os.Getenv("TEST_SRCDIR"), os.Getenv("TEST_WORKSPACE"), path))
		if err != nil {
			t.Fatal(err)
		}
		tools, spindles, feeds := map[string]bool{}, map[string]bool{}, map[float64]bool{}
		for _, m := range toolChange.FindAllStringSubmatch(string(data), -1) {
			tools[m[1]+m[2]] = true
		}
		minZ := 1e9
		for _, line := range strings.Split(string(data), "\n") {
			if i := strings.IndexAny(line, "(;"); i >= 0 {
				line = line[:i]
			}
			cut := strings.HasPrefix(strings.TrimSpace(line), "G1")
			for _, m := range word.FindAllStringSubmatch(line, -1) {
				v, _ := strconv.ParseFloat(m[2], 64)
				switch m[1] {
				case "S":
					spindles[m[2]] = true
				case "F":
					feeds[v] = true
				case "Z":
					if cut && v < minZ {
						minZ = v
					}
				}
			}
		}
		if len(tools) != 1 || !tools["1"] {
			t.Errorf("%s: tools %v; want only T1", path, tools)
		}
		if len(spindles) != 1 || !(spindles["12000"] || spindles["12000.0"]) {
			t.Errorf("%s: spindle speeds %v; want 12000", path, spindles)
		}
		if len(feeds) != 2 || !feeds[600] || !feeds[200] {
			t.Errorf("%s: feeds %v; want 600 and 200 mm/min", path, feeds)
		}
		if minZ > 0.001 || minZ < -0.001 {
			t.Errorf("%s: lowest cut at Z %g; want 0, the part's bottom", path, minZ)
		}
		if regexp.MustCompile(`\d{4}-\d{2}-\d{2}`).Match(data) {
			t.Errorf("%s: holds a date", path)
		}
	}
}

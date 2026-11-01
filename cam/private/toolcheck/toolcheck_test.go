// SPDX-License-Identifier: Apache-2.0

package toolcheck

import (
	"strings"
	"testing"
)

const good = `{"tools": [
  {"number": 3, "name": "5 mm drill", "shape": "drill", "diameter": 5.00, "length": 50, "tip_angle": 118},
  {"number": 4, "name": "6 mm ball", "shape": "ballend", "diameter": 6, "length": 50, "cutting_edge_height": 20, "shank_diameter": 6},
  {"number": 5, "name": "90 degree chamfer", "shape": "chamfer", "diameter": 12, "length": 50, "cutting_edge_angle": 90, "cutting_edge_height": 6, "tip_diameter": 0},
  {"number": 999, "name": "6 mm bullnose", "shape": "bullnose", "diameter": 6, "length": 50, "cutting_edge_height": 12, "corner_radius": 1, "flutes": 16},
  {"number": 1, "name": "3 mm flat end mill", "shape": "endmill", "diameter": 3.0, "length": 38, "cutting_edge_height": 12, "flutes": 2, "max_stepdown": 1.50},
  {"number": 2, "name": "60 degree V-bit", "shape": "vbit", "diameter": 6, "length": 40, "cutting_edge_angle": 60, "cutting_edge_height": 5, "tip_diameter": 0.1}
]}`

func TestGoodLibrary(t *testing.T) {
	l, err := Parse([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if errs := Check(l); len(errs) != 0 {
		t.Fatalf("Check: %v", errs)
	}
	c, err := Canonical(l)
	if err != nil {
		t.Fatal(err)
	}
	s := string(c)
	// Sorted by number; 3.0 and 5.00 written the one way.
	if i, j := strings.Index(s, `"number": 1`), strings.Index(s, `"number": 3`); i < 0 || j < i {
		t.Errorf("not sorted by number:\n%s", s)
	}
	if !strings.Contains(s, `"diameter": 3,`) || !strings.Contains(s, `"diameter": 5,`) {
		t.Errorf("numbers not canonical:\n%s", s)
	}
	if !strings.HasSuffix(s, "}\n") {
		t.Errorf("no final newline")
	}
}

// The same library, reordered and spelled differently, gives the same bytes.
func TestCanonicalIsStable(t *testing.T) {
	other := `{"tools":[{"number":999,"name":"6 mm bullnose","shape":"bullnose","diameter":6,"length":50,"cutting_edge_height":12,"corner_radius":1.0,"flutes":16},
	{"number":5,"name":"90 degree chamfer","shape":"chamfer","diameter":12,"length":50,"cutting_edge_angle":90,"cutting_edge_height":6,"tip_diameter":-0.0},
	{"number":4,"name":"6 mm ball","shape":"ballend","diameter":6,"length":50,"cutting_edge_height":20,"shank_diameter":6.0},
	{"shape":"endmill","number":1,"cutting_edge_height":12.000,"flutes":2,"name":"3 mm flat end mill","length":38.0,"diameter":3,"max_stepdown":1.5},
	{"number":2,"name":"60 degree V-bit","shape":"vbit","diameter":6.0,"length":40,"cutting_edge_angle":60,"cutting_edge_height":5,"tip_diameter":0.10},
	{"number":3,"name":"5 mm drill","shape":"drill","diameter":5,"length":50,"tip_angle":118.0}]}`
	var outs []string
	for _, src := range []string{good, other} {
		l, err := Parse([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		c, err := Canonical(l)
		if err != nil {
			t.Fatal(err)
		}
		outs = append(outs, string(c))
	}
	if outs[0] != outs[1] {
		t.Errorf("canonical forms differ:\n%s\n---\n%s", outs[0], outs[1])
	}
}

func TestBadLibraries(t *testing.T) {
	em := func(extra string) string {
		return `{"tools": [{"number": 1, "name": "em", "shape": "endmill", "diameter": 3, "length": 38, "cutting_edge_height": 12` + extra + `}]}`
	}
	for _, tc := range []struct{ name, src, want string }{
		{"no tools", `{"tools": []}`, "no tools"},
		{"unknown key", em(`, "colour": "red"`), `unknown field "colour"`},
		{"two values", em("") + em(""), "something follows the JSON object"},
		{"stray brace", em("") + "}", "something follows the JSON object"},
		{"stray bracket", `{"tools": []}]`, "something follows the JSON object"},
		{"null tool", `{"tools": [null]}`, "tool 1 of the list is null"},
		{"number 1000", `{"tools": [{"number": 1000, "name": "em", "shape": "endmill", "diameter": 3, "length": 38, "cutting_edge_height": 12}]}`, "number 1000; want 1 to 999"},
		{"no length", `{"tools": [{"number": 1, "name": "x", "shape": "endmill", "diameter": 3, "cutting_edge_height": 12}]}`, "no length"},
		{"unknown shape still checked", `{"tools": [{"number": 1, "name": "x", "shape": "laser", "diameter": 3, "length": 38, "flutes": 99}]}`, "flutes is 99; want 1 to 16"},
		{"flutes 17", em(`, "flutes": 17`), "flutes is 17; want 1 to 16"},
		{"max_stepdown 0", em(`, "max_stepdown": 0`), "max_stepdown is 0; want a number above 0"},
		{"max_stepdown past the edge", em(`, "max_stepdown": 13`), "max_stepdown 13 is more than the cutting_edge_height 12"},
		{"shank", em(`, "shank_diameter": 0`), "shank_diameter is 0; want a number above 0"},
		{"negative tip", `{"tools": [{"number": 1, "name": "x", "shape": "vbit", "diameter": 6, "length": 40, "cutting_edge_angle": 60, "cutting_edge_height": 5, "tip_diameter": -1}]}`, "tip_diameter is -1; want 0 or more"},
		{"edge angle", `{"tools": [{"number": 1, "name": "x", "shape": "chamfer", "diameter": 6, "length": 40, "cutting_edge_angle": 200, "cutting_edge_height": 5, "tip_diameter": 0}]}`, "cutting_edge_angle is 200 degrees; want less than 180"},
		{"short ball", `{"tools": [{"number": 1, "name": "x", "shape": "ballend", "diameter": 6, "length": 40, "cutting_edge_height": 2}]}`, "cutting_edge_height 2; want at least the ball's radius 3"},
		{"short corner", `{"tools": [{"number": 1, "name": "x", "shape": "bullnose", "diameter": 6, "length": 40, "cutting_edge_height": 0.5, "corner_radius": 1}]}`, "cutting_edge_height 0.5; want at least the corner radius 1"},
		{"old flat_radius", `{"tools": [{"number": 1, "name": "x", "shape": "bullnose", "diameter": 6, "length": 40, "cutting_edge_height": 12, "flat_radius": 2}]}`, `unknown field "flat_radius"`},
		{"not an int", `{"tools": [{"number": 1.5}]}`, "not a tool library"},
		{"number 0", `{"tools": [{"number": 0, "name": "em", "shape": "endmill", "diameter": 3, "length": 38, "cutting_edge_height": 12}]}`, "number 0; want 1 to 999"},
		{"duplicate number", `{"tools": [
			{"number": 1, "name": "a", "shape": "endmill", "diameter": 3, "length": 38, "cutting_edge_height": 12},
			{"number": 1, "name": "b", "shape": "endmill", "diameter": 3, "length": 38, "cutting_edge_height": 12}]}`, "number 1 appears twice"},
		{"duplicate name", `{"tools": [
			{"number": 1, "name": "a", "shape": "endmill", "diameter": 3, "length": 38, "cutting_edge_height": 12},
			{"number": 2, "name": "a", "shape": "endmill", "diameter": 3, "length": 38, "cutting_edge_height": 12}]}`, "name appears twice"},
		{"blank name", `{"tools": [{"number": 1, "name": " ", "shape": "endmill", "diameter": 3, "length": 38, "cutting_edge_height": 12}]}`, "want a name without spaces"},
		{"unknown shape", `{"tools": [{"number": 1, "name": "x", "shape": "laser", "diameter": 3, "length": 38}]}`, `shape "laser"; want one of ballend, bullnose, chamfer, drill, endmill, vbit`},
		{"no diameter", `{"tools": [{"number": 1, "name": "x", "shape": "endmill", "length": 38, "cutting_edge_height": 12}]}`, "no diameter"},
		{"missing need", `{"tools": [{"number": 1, "name": "x", "shape": "drill", "diameter": 3, "length": 38}]}`, "shape drill needs tip_angle"},
		{"foreign dimension", em(`, "tip_angle": 118`), "shape endmill has no tip_angle"},
		{"negative", `{"tools": [{"number": 1, "name": "x", "shape": "endmill", "diameter": -3, "length": 38, "cutting_edge_height": 12}]}`, "diameter is -3; want a number above 0"},
		{"flutes", em(`, "flutes": 0`), "flutes is 0; want 1 to 16"},
		{"angle", `{"tools": [{"number": 1, "name": "x", "shape": "drill", "diameter": 3, "length": 38, "tip_angle": 180}]}`, "tip_angle is 180 degrees; want less than 180"},
		{"edge too long", `{"tools": [{"number": 1, "name": "x", "shape": "endmill", "diameter": 3, "length": 10, "cutting_edge_height": 12}]}`, "cutting_edge_height 12 is more than the length 10"},
		{"corner radius", `{"tools": [{"number": 1, "name": "x", "shape": "bullnose", "diameter": 6, "length": 40, "cutting_edge_height": 12, "corner_radius": 3}]}`, "corner_radius 3; want less than the radius 3"},
		{"tip too wide", `{"tools": [{"number": 1, "name": "x", "shape": "vbit", "diameter": 6, "length": 40, "cutting_edge_angle": 60, "cutting_edge_height": 5, "tip_diameter": 6}]}`, "tip_diameter 6; want less than the diameter 6"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, err := Parse([]byte(tc.src))
			var msgs []string
			if err != nil {
				msgs = []string{err.Error()}
			} else {
				msgs = Check(l)
			}
			if !strings.Contains(strings.Join(msgs, "\n"), tc.want) {
				t.Errorf("got %q, want a message with %q", msgs, tc.want)
			}
		})
	}
}

// A bad diameter gives one message, not more from the fit checks too.
func TestNoFitMessagesOnBadDiameter(t *testing.T) {
	l, err := Parse([]byte(`{"tools": [{"number": 1, "name": "x", "shape": "vbit", "diameter": -6, "length": 40, "cutting_edge_angle": 60, "cutting_edge_height": 5, "tip_diameter": 0}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if errs := Check(l); len(errs) != 1 || !strings.Contains(errs[0], "diameter is -6") {
		t.Errorf("got %q, want only the diameter's message", errs)
	}
}

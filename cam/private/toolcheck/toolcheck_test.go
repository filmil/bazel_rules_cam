// SPDX-License-Identifier: Apache-2.0

package toolcheck

import (
	"strings"
	"testing"
)

const good = `{"tools": [
  {"number": 3, "name": "5 mm drill", "shape": "drill", "diameter": 5.00, "length": 50, "tip_angle": 118},
  {"number": 1, "name": "3 mm flat end mill", "shape": "endmill", "diameter": 3.0, "length": 38, "cutting_edge_height": 12, "flutes": 2},
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
	other := `{"tools":[{"shape":"endmill","number":1,"cutting_edge_height":12.000,"flutes":2,"name":"3 mm flat end mill","length":38.0,"diameter":3},
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
		{"two values", em("") + em(""), "more than one JSON value"},
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
		{"flat radius", `{"tools": [{"number": 1, "name": "x", "shape": "bullnose", "diameter": 6, "length": 40, "cutting_edge_height": 12, "flat_radius": 3}]}`, "flat_radius 3; want less than the radius 3"},
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

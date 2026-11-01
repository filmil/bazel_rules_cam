// SPDX-License-Identifier: Apache-2.0

package gcodecheck

import (
	"fmt"
	"strings"
	"testing"
)

// The stock in these tests is the package's: X -2 to 42, Y -2 to 32, top at
// Z 11. T2 is a 6 mm end mill with no max_stepdown, so it may cut 6 mm.

// passes cuts a line across the stock with T2, one pass per depth.
func passes(depths ...float64) string {
	var b strings.Builder
	b.WriteString("G21 G90\nM6 T2\nM3 S10000\nG0 Z20\nG0 X5 Y15\n")
	for _, z := range depths {
		fmt.Fprintf(&b, "G1 Z%g F100\nG1 X35 F300\nG0 Z20\nG0 X5\n", z)
	}
	return b.String()
}

func depthViolations(t *testing.T, g string, tools map[int]Tool) []Violation {
	t.Helper()
	var out []Violation
	for _, v := range Check(g, machine, stock, tools).Violations {
		if v.Kind != "depth" {
			t.Errorf("unexpected violation %v", v)
			continue
		}
		out = append(out, v)
	}
	return out
}

func TestDepthOnePassTooDeep(t *testing.T) {
	// 11 to 4 is 7 mm, in one pass, with a 6 mm tool.
	vs := depthViolations(t, passes(4), tools)
	if len(vs) == 0 {
		t.Fatal("a 7 mm pass with a 6 mm tool passed")
	}
	if !strings.Contains(vs[0].Message, "7.000 mm deep") || !strings.Contains(vs[0].Message, "T2") {
		t.Errorf("message: %s", vs[0].Message)
	}
}

func TestDepthInSteps(t *testing.T) {
	// The same 7 mm, in passes of 3.5: each cuts only its own depth, as the
	// height map remembers the earlier ones.
	if vs := depthViolations(t, passes(7.5, 4), tools); len(vs) != 0 {
		t.Errorf("stepped passes: %v", vs)
	}
}

func TestDepthMaxStepdown(t *testing.T) {
	limited := map[int]Tool{2: {Shape: "endmill", Diameter: 6, MaxStepdown: 2}}
	if vs := depthViolations(t, passes(9, 7, 5, 4), limited); len(vs) != 0 {
		t.Errorf("2 mm passes with max_stepdown 2: %v", vs)
	}
	if vs := depthViolations(t, passes(8), limited); len(vs) == 0 {
		t.Error("a 3 mm pass with max_stepdown 2 passed")
	}
}

func TestDepthHelix(t *testing.T) {
	// A 10 mm hole with T2, by helix: 1 mm per turn, 5 turns.
	var b strings.Builder
	b.WriteString("G21 G90\nM6 T2\nM3 S10000\nG0 Z20\nG0 X22 Y15\nG0 Z11\n")
	for z := 10; z >= 6; z-- {
		fmt.Fprintf(&b, "G2 X22 Y15 Z%d I-2 J0 F300\n", z)
	}
	b.WriteString("G0 Z20\n")
	if vs := depthViolations(t, b.String(), tools); len(vs) != 0 {
		t.Errorf("helix: %v", vs)
	}
	// The same hole in one turn, 7 mm down: too deep.
	one := "G21 G90\nM6 T2\nM3 S10000\nG0 Z20\nG0 X22 Y15\nG0 Z11\nG2 X22 Y15 Z4 I-2 J0 F300\nG0 Z20\n"
	if vs := depthViolations(t, one, tools); len(vs) == 0 {
		t.Error("a 7 mm deep helix turn passed")
	}
}

func TestDepthDrillExempt(t *testing.T) {
	drill := map[int]Tool{2: {Shape: "drill", Diameter: 6}}
	if vs := depthViolations(t, passes(0), drill); len(vs) != 0 {
		t.Errorf("drill: %v", vs)
	}
}

func TestDepthUnknownToolUnchecked(t *testing.T) {
	// No depth check without the tool; the tool check reports it.
	r := Check(passes(0), machine, stock, map[int]Tool{})
	for _, v := range r.Violations {
		if v.Kind == "depth" {
			t.Errorf("depth violation for an unknown tool: %v", v)
		}
	}
}

func TestDepthPartialG3(t *testing.T) {
	// A half circle counterclockwise at a fresh level 3 mm down, then the
	// same at 8 mm down: within, then past, the 6 mm limit.
	g := func(z float64) string {
		return fmt.Sprintf("G21 G90\nM6 T2\nM3 S10000\nG0 Z20\nG0 X25 Y15\nG1 Z%g F100\nG3 X15 Y15 I-5 J0 F300\nG0 Z20\n", z)
	}
	if vs := depthViolations(t, g(8), tools); len(vs) != 0 {
		t.Errorf("3 mm: %v", vs)
	}
	vs := depthViolations(t, g(3), tools)
	if len(vs) == 0 {
		t.Fatal("8 mm passed")
	}
	// The plunge reports first; the arc, at the same depth, too.
	if len(vs) != 2 || vs[0].Line != 6 || vs[1].Line != 7 {
		t.Errorf("want the plunge (line 6) and the arc (line 7): %v", vs)
	}
}

func TestDepthRamp(t *testing.T) {
	// A ramp from the top to 7 mm down over 30 mm: it ends 7 mm deep.
	g := "G21 G90\nM6 T2\nM3 S10000\nG0 Z20\nG0 X5 Y15\nG0 Z11\nG1 X35 Z4 F300\nG0 Z20\n"
	if vs := depthViolations(t, g, tools); len(vs) == 0 {
		t.Error("a ramp to 7 mm passed")
	}
}

func TestDepthAfterDrillCycle(t *testing.T) {
	// A 6 mm hole drilled 8 mm deep with a cycle, then T1, a 3 mm end mill
	// that may cut 5 mm, plunges 8 mm down inside it: no stock in its way.
	ts := map[int]Tool{1: {Shape: "endmill", Diameter: 3, MaxStepdown: 5}, 2: {Shape: "drill", Diameter: 6}}
	g := "G21 G90\nM6 T2\nM3 S3000\nG0 Z20\nG0 X20 Y15\nG98 G81 X20 Y15 Z3 R12 F100\nG80\nG0 Z20\n" +
		"M6 T1\nM3 S10000\nG0 X20 Y15\nG0 Z12\nG1 Z3 F100\nG0 Z20\n"
	if vs := depthViolations(t, g, ts); len(vs) != 0 {
		t.Errorf("end mill in a drilled hole: %v", vs)
	}
}

func TestDepthTinyToolOnLargeStock(t *testing.T) {
	// 600 mm stock: 0.3 mm cells. A 0.2 mm tool covers no cell's center
	// but is still checked.
	big := Stock{XMin: 0, XMax: 600, YMin: 0, YMax: 600, ZMin: 0, ZMax: 10}
	ts := map[int]Tool{2: {Shape: "endmill", Diameter: 0.2, MaxStepdown: 0.1}}
	g := "G21 G90\nM6 T2\nM3 S10000\nG0 Z20\nG0 X100.15 Y100.15\nG1 Z9 F100\nG1 X110.15 F300\nG0 Z20\n"
	r := Check(g, Machine{Envelope: [3]float64{700, 700, 50}, MaxFeed: 1000, MaxSpindle: 20000}, big, ts)
	// Both the plunge (line 6) and the cut across (line 7): the cut is
	// checked against the stock before it, not as its own samples cut it.
	lines := map[int]bool{}
	for _, v := range r.Violations {
		if v.Kind == "depth" {
			lines[v.Line] = true
		}
	}
	if !lines[6] || !lines[7] {
		t.Errorf("want depth violations on lines 6 and 7: %v", r.Violations)
	}
}

func TestDepthEndMillInDrillCycle(t *testing.T) {
	// An end mill (3 mm, max_stepdown 2) run through a G81 cycle 8 mm deep
	// into fresh stock: checked, unlike a drill.
	ts := map[int]Tool{1: {Shape: "endmill", Diameter: 3, MaxStepdown: 2}}
	g := "G21 G90\nM6 T1\nM3 S10000\nG0 Z20\nG0 X20 Y15\nG98 G81 X20 Y15 Z3 R12 F100\nG80\nG0 Z20\n"
	if vs := depthViolations(t, g, ts); len(vs) == 0 {
		t.Error("an end mill's 8 mm cycle passed")
	}
}

func TestDepthPastDrilledHole(t *testing.T) {
	// A hole drilled to Z 3; an end mill (max_stepdown 2) goes on to Z 0,
	// 3 mm below the hole's bottom, and is caught.
	ts := map[int]Tool{1: {Shape: "endmill", Diameter: 3, MaxStepdown: 2}, 2: {Shape: "drill", Diameter: 6}}
	g := "G21 G90\nM6 T2\nM3 S3000\nG0 Z20\nG0 X20 Y15\nG98 G81 X20 Y15 Z3 R12 F100\nG80\nG0 Z20\n" +
		"M6 T1\nM3 S10000\nG0 X20 Y15\nG0 Z12\nG1 Z0 F100\nG0 Z20\n"
	if vs := depthViolations(t, g, ts); len(vs) == 0 {
		t.Error("3 mm past the drilled bottom passed")
	}
}

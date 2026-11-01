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

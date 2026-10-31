// SPDX-License-Identifier: Apache-2.0

package gcodecheck

import (
	"strings"
	"testing"
)

var (
	machine = Machine{Envelope: [3]float64{100, 100, 50}, MaxFeed: 1000, MaxSpindle: 20000}
	stock   = Stock{XMin: -2, XMax: 42, YMin: -2, YMax: 32, ZMin: 0, ZMax: 11}
	tools   = map[int]bool{1: true, 2: true}
)

// A job as FreeCAD's posts write it: to clearance, across, down to just
// above the stock, a plunge and a cut, back up.
const good = `(begin preamble)
G17 G54 G40 G49 G80 G90
G21
(T1: 3 mm end mill)
M6 T1
G43 H1
M3 S12000
G0 Z16.000
G0 X25.500 Y14.500
G0 X25.500 Y14.500 Z14.000
G1 X25.500 Y14.500 Z6.000 F200.000
G1 X14.500 Y14.500 Z6.000 F600.000
G2 X14.500 Y16.500 I0 J1 F600
G0 Z16.000
M5
`

func kinds(t *testing.T, g string) []string {
	t.Helper()
	return Kinds(Check(g, machine, stock, tools))
}

func TestGood(t *testing.T) {
	r := Check(good, machine, stock, tools)
	if len(r.Violations) != 0 {
		t.Fatalf("violations: %v", r.Violations)
	}
	if r.Moves != 7 || len(r.Tools) != 1 || r.Tools[0] != 1 {
		t.Errorf("moves %d, tools %v", r.Moves, r.Tools)
	}
	// The arc G2 from (14.5, 14.5) about (14.5, 15.5), clockwise to
	// (14.5, 16.5), passes X 13.5 on its way: the low X is the arc's.
	if r.Min[0] > 13.5+eps || r.Min[0] < 13.5-eps {
		t.Errorf("min X %g; want 13.5, the arc's left point", r.Min[0])
	}
	if !strings.Contains(Report(r, machine, stock), "no violations") {
		t.Errorf("report: %s", Report(r, machine, stock))
	}
}

func TestViolations(t *testing.T) {
	for _, tc := range []struct{ name, g, want, msg string }{
		{"feed", strings.Replace(good, "F600.000", "F1500.000", 1), "feed", "F1500 mm/min is above the machine's 1000"},
		{"no feed", "G21\nG1 X1 Y1 Z12\n", "feed", "a cutting move with no feed rate"},
		{"spindle", strings.Replace(good, "S12000", "S24000", 1), "spindle", "S24000 rpm is above the machine's 20000"},
		{"tool", strings.Replace(good, "M6 T1", "M6 T7", 1), "tool", "T7 is not in the tool library"},
		{"tool in a comment", strings.Replace(good, "M6 T1", "( M6 T9 )", 1), "tool", "T9 is not in the tool library"},
		{"rapid down into the stock", strings.Replace(good, "G0 X25.500 Y14.500 Z14.000", "G0 X25.500 Y14.500 Z8.000", 1), "rapid", "below its top at Z 11.000"},
		{"rapid across, low", strings.Replace(good, "G0 Z16.000\nM5", "G0 X1 Y1 Z6\nM5", 1), "rapid", "G0 from Z 6.000 to Z 6.000"},
		{"envelope", strings.Replace(good, "G0 Z16.000\nM5", "G0 Z70\nM5", 1), "envelope", "span 64.000 mm in Z"},
		{"inches", "G20\nG0 X0 Y0 Z1\nG0 X5 Y0 Z1\n", "envelope", "span 127.000 mm in X"},
		{"plane", "G18\nG0 X0 Y0 Z20\n", "unsupported", "arcs outside the XY plane"},
		{"move first", "X1 Y1 Z20\n", "unsupported", "a move before any G0 to G3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Check(tc.g, machine, stock, tools)
			k := Kinds(r)
			if len(k) != 1 || k[0] != tc.want {
				t.Fatalf("kinds %v; want [%s]: %v", k, tc.want, r.Violations)
			}
			if rep := Report(r, machine, stock); !strings.Contains(rep, tc.msg) {
				t.Errorf("report lacks %q:\n%s", tc.msg, rep)
			}
		})
	}
}

// A rapid straight up out of a cut is not a crash; one beside the stock,
// low, is not either.
func TestRapidsThatAreFine(t *testing.T) {
	for _, g := range []string{
		"G21\nG0 X10 Y10 Z16\nG1 Z6 F100\nG0 Z16\n",
		"G21\nG0 X60 Y60 Z16\nG0 X60 Y60 Z2\nG0 X70 Y60 Z2\n",
	} {
		if k := kinds(t, g); len(k) != 0 {
			t.Errorf("%q: %v", g, k)
		}
	}
}

// Incremental moves add up.
func TestIncremental(t *testing.T) {
	r := Check("G21 G90\nG0 X0 Y0 Z20\nG91\nG0 X10\nG0 X10\n", machine, stock, tools)
	if r.Max[0] != 20 {
		t.Errorf("max X %g; want 20", r.Max[0])
	}
}

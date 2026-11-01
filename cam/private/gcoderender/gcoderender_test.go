// SPDX-License-Identifier: Apache-2.0

package gcoderender

import (
	"bytes"
	"image/png"
	"math"
	"strings"
	"testing"

	"github.com/filmil/bazel_rules_cam/cam/private/gcodecheck"
)

var (
	machine = gcodecheck.Machine{Envelope: [3]float64{100, 100, 50}, MaxFeed: 1000, MaxSpindle: 20000}
	stock   = gcodecheck.Stock{XMin: -2, XMax: 42, YMin: -2, YMax: 32, ZMin: 0, ZMax: 11}
	tools   = map[int]gcodecheck.Tool{1: {Shape: "endmill", Diameter: 3, MaxStepdown: 5}}
)

const job = `(begin operation: Pocket)
G21 G90
M6 T1
M3 S12000
G0 Z16
G0 X10 Y10
G0 Z12
G1 Z9 F200
G1 X30 F600
G2 X30 Y20 I0 J5
G1 X10
G1 Z7 F200
G1 X30 F600
G0 Z16
(finish operation: Pocket)
G0 X0 Y0
(begin operation: Drill)
G98 G81 X20 Y25 Z6 R12 F100
G80
G0 Z16
`

func TestPath(t *testing.T) {
	r := gcodecheck.Check(job, machine, stock, tools)
	if len(r.Violations) != 0 {
		t.Fatalf("violations: %v", r.Violations)
	}
	ops := map[string]int{}
	arc := 0
	for _, g := range r.Path {
		ops[g.Op]++
		if g.Line == 10 {
			arc++
		}
	}
	if ops["Pocket"] == 0 || ops["Drill"] == 0 {
		t.Errorf("operations: %v", ops)
	}
	// A half circle of radius 5 is about 15.7 mm: some 60 pieces of 0.25.
	if arc < 50 {
		t.Errorf("the arc came as %d pieces", arc)
	}
}

func TestDrawIsDeterministicAndDrawn(t *testing.T) {
	r := gcodecheck.Check(job, machine, stock, tools)
	for name, proj := range map[string]projection{"top": Top, "iso": Iso} {
		a, b := Draw(r, stock, proj), Draw(r, stock, proj)
		if !bytes.Equal(a, b) {
			t.Errorf("%s: two draws differ", name)
		}
		img, err := png.Decode(bytes.NewReader(a))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if img.Bounds().Dx() != Size {
			t.Errorf("%s: width %d", name, img.Bounds().Dx())
		}
		// Some pixels are the deepest color, dark red-ish: cuts were drawn.
		red := 0
		for y := 0; y < Size; y++ {
			for x := 0; x < Size; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if r>>8 > 100 && g>>8 < 60 && b>>8 < 80 {
					red++
				}
			}
		}
		if red == 0 {
			t.Errorf("%s: no deep cut drawn", name)
		}
	}
}

func TestReport(t *testing.T) {
	r := gcodecheck.Check(job, machine, stock, tools)
	rep := Report("plate", r, stock, tools, machine.MaxFeed)
	for _, want := range []string{
		"# Toolpath of plate",
		"| Pocket | T1 (3 mm endmill) | 9, 7 | 7.000 |",
		"| Drill | T1 (3 mm endmill) | - | 6.000 |",
		"Estimated time:",
	} {
		if !strings.Contains(rep, want) {
			t.Errorf("report lacks %q:\n%s", want, rep)
		}
	}
}

func TestReportRows(t *testing.T) {
	// Two runs of one name stay two rows; a move between operations belongs
	// to none; a feed above the stock is no level; an unknown tool is named
	// bare; violations are flagged.
	g := `G21 G90
M6 T2
M3 S12000
(begin operation: Profile)
G0 Z16
G0 X10 Y10
G1 X20 Y10 Z12 F600
G1 Z10.0001 F200
G1 X30 F600
G0 Z16
(finish operation: Profile)
G0 X0 Y0
(begin operation: Profile)
G1 X10 Y10 Z9.5 F200
G1 X30 F600
G0 Z16
`
	r := gcodecheck.Check(g, machine, stock, map[int]gcodecheck.Tool{})
	rep := Report("x", r, stock, map[int]gcodecheck.Tool{}, machine.MaxFeed)
	rows := 0
	for _, l := range strings.Split(rep, "\n") {
		if strings.HasPrefix(l, "| Profile |") {
			rows++
			if !strings.Contains(l, "| T2 |") {
				t.Errorf("unknown tool not named bare: %s", l)
			}
		}
	}
	if rows != 2 {
		t.Errorf("want two Profile rows:\n%s", rep)
	}
	if !strings.Contains(rep, "| (none named) |") {
		t.Errorf("the move between operations has no row:\n%s", rep)
	}
	if strings.Contains(rep, "| 12 |") || strings.Contains(rep, "12, ") || !strings.Contains(rep, "| 9.5 |") {
		t.Errorf("levels above the stock listed, or the cut one missing:\n%s", rep)
	}
	if !strings.Contains(rep, "gcode_check finds") {
		t.Errorf("violations (an unknown tool) not flagged:\n%s", rep)
	}
}

func TestLevels(t *testing.T) {
	m := map[float64]bool{}
	for i := 0; i < 10; i++ {
		m[float64(10-i)] = true
	}
	if got := levels(m); got != "10, 9, 8 ... 2, 1 (10)" {
		t.Errorf("elided: %q", got)
	}
	if got := levels(map[float64]bool{math.Round(-0.0001*1000)/1000 + 0: true}); got != "0" {
		t.Errorf("-0: %q", got)
	}
}

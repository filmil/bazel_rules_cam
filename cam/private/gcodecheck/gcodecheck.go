// SPDX-License-Identifier: Apache-2.0

// Package gcodecheck checks a job's G-code against its machine, its stock
// and its tool library (gcode_check_test, issue #10).
//
// The checks, each a violation kind:
//
//   - envelope: on some axis, the moves span more than the machine's
//     travel. The G-code is in the part's coordinates, and where the part
//     sits on the machine is the operator's choice, so the check is that
//     the job fits: the span of every axis, arcs included, is at most its
//     travel.
//   - rapid: a rapid move (G0) goes down to, or across at, a height below
//     the stock's top while over the stock. A rapid straight up, out of a
//     cut, is fine.
//   - feed: a feed rate above the machine's max_feed, or a cutting move
//     with no feed rate.
//   - spindle: a spindle speed above the machine's max_spindle.
//   - tool: a tool number that is not in the tool library. A post that
//     cannot change tools (GRBL) writes the change as a comment, "( M6 T1
//     )"; that counts too.
//   - unsupported: G-code this checker does not follow (arcs outside the
//     XY plane, a coordinate system change), so it cannot vouch for it.
//
// It reads the words posts write: G0 to G3, G17, G20 and G21, G90 and G91,
// G54 and the like, F, S, T, M3 to M6, X, Y, Z, I, J, K and comments in
// parentheses or after a semicolon.
package gcodecheck

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Machine is the machine's limits, as machine() gives them.
type Machine struct {
	Envelope   [3]float64 `json:"envelope"` // X, Y, Z travel, mm
	MaxFeed    float64    `json:"max_feed"` // mm/min
	MaxSpindle float64    `json:"max_spindle"`
}

// Stock is the stock's box in the G-code's coordinates.
type Stock struct {
	XMin, XMax, YMin, YMax, ZMin, ZMax float64
}

// Violation is one problem, at a line of the G-code (0 for the whole file).
type Violation struct {
	Kind, Message string
	Line          int
}

func (v Violation) String() string {
	if v.Line == 0 {
		return fmt.Sprintf("%s: %s", v.Kind, v.Message)
	}
	return fmt.Sprintf("%s: line %d: %s", v.Kind, v.Line, v.Message)
}

var (
	word        = regexp.MustCompile(`([A-Za-z])\s*([-+]?(?:\d+\.?\d*|\.\d+))`)
	commentTool = regexp.MustCompile(`(?i)M0*6\s*T(\d+)|T(\d+)\s*M0*6`)
)

const eps = 1e-6

// Result is what Check found: the violations and the extent of the moves.
type Result struct {
	Violations []Violation
	Min, Max   [3]float64
	Moves      int
	Tools      []int
}

// Check reads the G-code and returns what it found.
func Check(gcode string, m Machine, stock Stock, tools map[int]bool) Result {
	var r Result
	for i := 0; i < 3; i++ {
		r.Min[i], r.Max[i] = math.Inf(1), math.Inf(-1)
	}
	add := func(kind string, line int, format string, a ...any) {
		r.Violations = append(r.Violations, Violation{kind, fmt.Sprintf(format, a...), line})
	}
	var pos [3]float64
	var known [3]bool
	motion, absolute, scale, feed := -1, true, 1.0, 0.0
	usedTools := map[int]bool{}
	useTool := func(n, line int) {
		usedTools[n] = true
		if !tools[n] {
			add("tool", line, "T%d is not in the tool library", n)
		}
	}
	extend := func(p [3]float64) {
		for i := 0; i < 3; i++ {
			if known[i] {
				r.Min[i] = math.Min(r.Min[i], p[i])
				r.Max[i] = math.Max(r.Max[i], p[i])
			}
		}
	}
	for ln, raw := range strings.Split(gcode, "\n") {
		line := ln + 1
		code, comments := splitComments(raw)
		for _, c := range comments {
			for _, mt := range commentTool.FindAllStringSubmatch(c, -1) {
				n, _ := strconv.Atoi(mt[1] + mt[2])
				useTool(n, line)
			}
		}
		words := word.FindAllStringSubmatch(code, -1)
		if len(words) == 0 {
			continue
		}
		var target [3]*float64
		var ij [2]float64
		hasIJ := false
		lineMotion := -1
		for _, w := range words {
			letter := strings.ToUpper(w[1])
			v, err := strconv.ParseFloat(w[2], 64)
			if err != nil {
				add("unsupported", line, "cannot read %q", w[0])
				continue
			}
			switch letter {
			case "G":
				switch g := v; {
				case g == 0, g == 1, g == 2, g == 3:
					lineMotion = int(g)
				case g == 17, g == 40, g == 49, g == 80, g == 94:
				case g == 18, g == 19:
					add("unsupported", line, "G%g: arcs outside the XY plane", g)
				case g == 20:
					scale = 25.4
				case g == 21:
					scale = 1
				case g == 90:
					absolute = true
				case g == 91:
					absolute = false
				case g >= 54 && g <= 59.3:
					// The work offset: positions stay in its coordinates.
				case g == 43, g == 4:
					// Tool length offset, dwell: no motion.
				case g == 81, g == 82, g == 83, g == 73, g == 98, g == 99:
					// Drilling cycles: their X, Y, Z (and R) are positions.
					lineMotion = 1
				default:
					add("unsupported", line, "G%g", g)
				}
			case "X", "Y", "Z":
				i := int(letter[0] - 'X')
				x := v * scale
				target[i] = &x
			case "R":
				// A cycle's retract height: a position the tool goes to.
			case "I", "J":
				ij[letter[0]-'I'] = v * scale
				hasIJ = true
			case "K":
			case "F":
				feed = v * scale
				if feed > m.MaxFeed+eps {
					add("feed", line, "F%g mm/min is above the machine's %g", feed, m.MaxFeed)
				}
			case "S":
				if v > m.MaxSpindle+eps {
					add("spindle", line, "S%g rpm is above the machine's %g", v, m.MaxSpindle)
				}
			case "T":
				useTool(int(v), line)
			case "M", "N", "P", "Q", "H", "D", "L":
			default:
				add("unsupported", line, "word %s", w[0])
			}
		}
		if lineMotion >= 0 {
			motion = lineMotion
		}
		if target[0] == nil && target[1] == nil && target[2] == nil {
			continue
		}
		if motion < 0 {
			add("unsupported", line, "a move before any G0 to G3")
			continue
		}
		from, fromKnown := pos, known
		for i := 0; i < 3; i++ {
			if target[i] != nil {
				if absolute || !known[i] {
					pos[i] = *target[i]
				} else {
					pos[i] += *target[i]
				}
				known[i] = true
			}
		}
		r.Moves++
		if motion != 0 && !(feed > 0) {
			add("feed", line, "a cutting move with no feed rate")
		}
		if motion == 0 && fromKnown[2] && known[2] && fromKnown[0] && fromKnown[1] {
			rapid(&r, line, from, pos, stock)
		}
		extend(pos)
		if (motion == 2 || motion == 3) && hasIJ && fromKnown[0] && fromKnown[1] {
			arcExtremes(from, pos, ij, motion == 2, extend)
		}
	}
	axes := "XYZ"
	for i := 0; i < 3; i++ {
		if r.Max[i] < r.Min[i] {
			continue
		}
		if span := r.Max[i] - r.Min[i]; span > m.Envelope[i]+eps {
			r.Violations = append(r.Violations, Violation{"envelope", fmt.Sprintf(
				"the moves span %.3f mm in %c (%.3f to %.3f); the machine travels %g", span, axes[i], r.Min[i], r.Max[i], m.Envelope[i]), 0})
		}
	}
	for n := range usedTools {
		r.Tools = append(r.Tools, n)
	}
	sort.Ints(r.Tools)
	return r
}

// rapid flags a G0 from a to b that goes down below the stock's top, or
// across below it, while over the stock.
func rapid(r *Result, line int, a, b [3]float64, s Stock) {
	top := s.ZMax
	down := b[2] < a[2]-eps
	across := math.Abs(b[0]-a[0]) > eps || math.Abs(b[1]-a[1]) > eps
	low := math.Min(a[2], b[2]) < top-eps
	if !low || !(down || across) {
		return
	}
	if !across && b[2] >= top-eps {
		return
	}
	// The segment's XY box against the stock's.
	if math.Max(a[0], b[0]) < s.XMin-eps || math.Min(a[0], b[0]) > s.XMax+eps ||
		math.Max(a[1], b[1]) < s.YMin-eps || math.Min(a[1], b[1]) > s.YMax+eps {
		return
	}
	r.Violations = append(r.Violations, Violation{"rapid", fmt.Sprintf(
		"G0 from Z %.3f to Z %.3f over the stock, below its top at Z %.3f", a[2], b[2], top), line})
}

// arcExtremes extends the bounds by the points of an XY arc at 0, 90, 180
// and 270 degrees that the arc passes.
func arcExtremes(from, to [3]float64, ij [2]float64, clockwise bool, extend func([3]float64)) {
	cx, cy := from[0]+ij[0], from[1]+ij[1]
	rad := math.Hypot(ij[0], ij[1])
	a0 := math.Atan2(from[1]-cy, from[0]-cx)
	a1 := math.Atan2(to[1]-cy, to[0]-cx)
	sweep := a1 - a0
	if clockwise {
		for sweep >= 0 {
			sweep -= 2 * math.Pi
		}
	} else {
		for sweep <= 0 {
			sweep += 2 * math.Pi
		}
	}
	for k := 0; k < 4; k++ {
		ang := float64(k) * math.Pi / 2
		d := ang - a0
		if clockwise {
			for d > 0 {
				d -= 2 * math.Pi
			}
			if d < sweep {
				continue
			}
		} else {
			for d < 0 {
				d += 2 * math.Pi
			}
			if d > sweep {
				continue
			}
		}
		p := to
		p[0], p[1] = cx+rad*math.Cos(ang), cy+rad*math.Sin(ang)
		extend(p)
	}
}

// splitComments returns a line's code and its comments.
func splitComments(line string) (string, []string) {
	var code strings.Builder
	var comments []string
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '(':
			j := strings.IndexByte(line[i:], ')')
			if j < 0 {
				j = len(line) - i
			}
			comments = append(comments, line[i:i+j])
			i += j
		case ';':
			comments = append(comments, line[i+1:])
			i = len(line)
		default:
			code.WriteByte(line[i])
		}
	}
	return code.String(), comments
}

// Report is the check's report, for a person: what was checked, and every
// violation.
func Report(r Result, m Machine, s Stock) string {
	var b strings.Builder
	fmt.Fprintf(&b, "G-code check: %d moves, tools %v\n", r.Moves, r.Tools)
	axes := "XYZ"
	for i := 0; i < 3; i++ {
		if r.Max[i] >= r.Min[i] {
			fmt.Fprintf(&b, "  %c: %.3f to %.3f (span %.3f of %g mm travel)\n", axes[i], r.Min[i], r.Max[i], r.Max[i]-r.Min[i], m.Envelope[i])
		}
	}
	fmt.Fprintf(&b, "  stock top at Z %.3f; max feed %g mm/min; max spindle %g rpm\n", s.ZMax, m.MaxFeed, m.MaxSpindle)
	if len(r.Violations) == 0 {
		b.WriteString("no violations\n")
	}
	for _, v := range r.Violations {
		fmt.Fprintf(&b, "VIOLATION %s\n", v)
	}
	return b.String()
}

// Kinds is the sorted set of the violations' kinds.
func Kinds(r Result) []string {
	set := map[string]bool{}
	for _, v := range r.Violations {
		set[v.Kind] = true
	}
	var k []string
	for x := range set {
		k = append(k, x)
	}
	sort.Strings(k)
	return k
}

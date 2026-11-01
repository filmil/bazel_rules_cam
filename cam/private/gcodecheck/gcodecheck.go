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
//     cut, is fine, and so is one whose whole path the job has already
//     cut down to the tool's bottom (within 0.001 mm) (the height map of the depth check): a
//     helix's move to its bore's center, say.
//   - feed: a feed rate above the machine's max_feed, or a cutting move
//     with no feed rate.
//   - spindle: a spindle speed above the machine's max_spindle.
//   - tool: a tool number that is not in the tool library. A post that
//     cannot change tools (GRBL) writes the change as a comment, "( M6 T1
//     )"; that counts too.
//   - depth: a cutting move goes deeper into the stock than its tool may
//     cut in one pass: its max_stepdown in the tool library, or else its
//     diameter. The checker keeps a height map of the stock, lowered by
//     every cutting move, so a pass in stock that earlier passes cut counts
//     only its own depth. Drills are not checked.
//   - unsupported: G-code this checker does not follow (arcs outside the
//     XY plane or in R form, expressions, unknown words), so it cannot
//     vouch for it.
//
// It reads the words posts write: G0 to G3, the drilling cycles G73 and G81
// to G83 with R, G98 and G99 and G80, G17, G20 and G21, G90 and G91, G54 and
// the like, F, S, T, M3 to M6, X, Y, Z, I, J, K and comments in parentheses
// or after a semicolon. Within a block, units and modes apply to all its
// words, whatever their order.
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
	// Path is the toolpath, every move between known points, in order:
	// what gcode_render draws. Arcs come as short straight pieces.
	Path []Segment
}

// Segment is one straight piece of the toolpath.
type Segment struct {
	From, To [3]float64
	Rapid    bool    // a G0; else the tool cuts at Feed
	Feed     float64 // mm/min; 0 for a rapid
	Tool     int     // the tool in the spindle, 0 none
	Op       string  // the operation, from FreeCAD's "Begin operation" comments
	Line     int
}

// opComment is the comment FreeCAD's posts write before each operation.
// A comment arrives with its opening parenthesis; posts differ in case
// ("Begin operation" in grbl, "begin operation" in linuxcnc).
var opComment = regexp.MustCompile(`(?i)^\(?\s*begin operation:\s*(.*?)\s*$`)

// opEnd is the comment after it; moves after it belong to no operation.
var opEnd = regexp.MustCompile(`(?i)^\(?\s*finish operation:`)

// arcPiece is the longest straight piece an arc is drawn with, mm.
const arcPiece = 0.25

// trace records a straight move from a to b, if both are known.
func (c *checker) trace(line int, a [3]float64, aKnown [3]bool, b [3]float64, bKnown [3]bool, rapid bool) {
	all := [3]bool{true, true, true}
	if aKnown != all || bKnown != all {
		return
	}
	s := Segment{From: a, To: b, Rapid: rapid, Tool: c.tool, Op: c.op, Line: line}
	if !rapid {
		s.Feed = c.feed
	}
	c.r.Path = append(c.r.Path, s)
}

// Check reads the G-code and returns what it found.
func Check(gcode string, m Machine, stock Stock, tools map[int]Tool) Result {
	c := &checker{m: m, stock: stock, tools: tools, motion: -1, absolute: true, scale: 1, used: map[int]bool{}, hm: newHeights(stock)}
	for i := 0; i < 3; i++ {
		c.r.Min[i], c.r.Max[i] = math.Inf(1), math.Inf(-1)
	}
	for ln, raw := range strings.Split(gcode, "\n") {
		c.block(ln+1, raw)
	}
	axes := "XYZ"
	for i := 0; i < 3; i++ {
		if c.r.Max[i] < c.r.Min[i] {
			continue
		}
		if span := c.r.Max[i] - c.r.Min[i]; span > m.Envelope[i]+eps {
			c.r.Violations = append(c.r.Violations, Violation{"envelope", fmt.Sprintf(
				"the moves span %.3f mm in %c (%.3f to %.3f); the machine travels %g", span, axes[i], c.r.Min[i], c.r.Max[i], m.Envelope[i]), 0})
		}
	}
	for n := range c.used {
		c.r.Tools = append(c.r.Tools, n)
	}
	sort.Ints(c.r.Tools)
	return c.r
}

type checker struct {
	r        Result
	m        Machine
	stock    Stock
	tools    map[int]Tool
	tool     int      // the tool in the spindle, 0 none
	hm       *heights // the stock as cut so far; nil without a stock
	op       string   // the current operation's name, if the post wrote one
	pos      [3]float64
	known    [3]bool
	motion   int     // 0 to 3, 81 to 83 or 73 (a drilling cycle), -1 none
	absolute bool    // G90
	scale    float64 // 1 for mm, 25.4 for inches
	feed     float64
	retract  float64 // a cycle's R plane
	oldZ     bool    // G98: a cycle returns to the Z it started at
	used     map[int]bool
}

func (c *checker) add(kind string, line int, format string, a ...any) {
	c.r.Violations = append(c.r.Violations, Violation{kind, fmt.Sprintf(format, a...), line})
}

func (c *checker) useTool(n, line int) {
	c.used[n] = true
	c.tool = n
	if _, ok := c.tools[n]; !ok {
		c.add("tool", line, "T%d is not in the tool library", n)
	}
}

func (c *checker) extend(p [3]float64) {
	for i := 0; i < 3; i++ {
		if c.known[i] {
			c.r.Min[i] = math.Min(c.r.Min[i], p[i])
			c.r.Max[i] = math.Max(c.r.Max[i], p[i])
		}
	}
}

// block reads one line. Like RS274/NGC, it takes a block's settings
// (units, distance mode, feed) before its move, whatever the words' order.
func (c *checker) block(line int, raw string) {
	code, comments := splitComments(raw)
	for _, cm := range comments {
		if m := opComment.FindStringSubmatch(cm); m != nil {
			c.op = m[1]
		} else if opEnd.MatchString(cm) {
			c.op = ""
		}
		for _, mt := range commentTool.FindAllStringSubmatch(cm, -1) {
			n, _ := strconv.Atoi(mt[1] + mt[2])
			c.useTool(n, line)
		}
	}
	words := word.FindAllStringSubmatchIndex(code, -1)
	// Text no word covers: an expression, a bare letter, "X1.2.3".
	rest := []byte(code)
	for _, w := range words {
		for k := w[0]; k < w[1]; k++ {
			rest[k] = ' '
		}
	}
	if left := strings.TrimSpace(string(rest)); left != "" {
		c.add("unsupported", line, "cannot read %q", left)
		return
	}
	type wv struct {
		letter string
		v      float64
	}
	var ws []wv
	for _, w := range words {
		v, err := strconv.ParseFloat(code[w[4]:w[5]], 64)
		if err != nil {
			c.add("unsupported", line, "cannot read %q", code[w[0]:w[1]])
			return
		}
		ws = append(ws, wv{strings.ToUpper(code[w[2]:w[3]]), v})
	}
	if len(ws) == 0 {
		return
	}
	// Settings first.
	lineMotion := -1
	for _, w := range ws {
		if w.letter != "G" {
			continue
		}
		switch g := w.v; {
		case g == 0, g == 1, g == 2, g == 3:
			lineMotion = int(g)
		case g == 73, g == 81, g == 82, g == 83:
			lineMotion = int(g)
		case g == 80:
			c.motion = -1
		case g == 17, g == 40, g == 49, g == 94, g == 43, g == 4:
		case g >= 54 && g <= 59.3:
			// The work offset: positions stay in its coordinates.
		case g == 18, g == 19:
			c.add("unsupported", line, "G%g: arcs outside the XY plane", g)
		case g == 20:
			c.scale = 25.4
		case g == 21:
			c.scale = 1
		case g == 90:
			c.absolute = true
		case g == 91:
			c.absolute = false
		case g == 98:
			c.oldZ = true
		case g == 99:
			c.oldZ = false
		default:
			c.add("unsupported", line, "G%g", g)
		}
	}
	if lineMotion >= 0 {
		c.motion = lineMotion
	}
	var target [3]*float64
	var ij [2]float64
	hasIJ, hasR := false, false
	for _, w := range ws {
		switch w.letter {
		case "G":
		case "X", "Y", "Z":
			x := w.v * c.scale
			target[w.letter[0]-'X'] = &x
		case "I", "J":
			ij[w.letter[0]-'I'] = w.v * c.scale
			hasIJ = true
		case "K":
		case "R":
			hasR = true
			c.retract = w.v * c.scale
		case "F":
			c.feed = w.v * c.scale
			if c.feed > c.m.MaxFeed+eps {
				c.add("feed", line, "F%g mm/min is above the machine's %g", c.feed, c.m.MaxFeed)
			}
		case "S":
			if w.v > c.m.MaxSpindle+eps {
				c.add("spindle", line, "S%g rpm is above the machine's %g", w.v, c.m.MaxSpindle)
			}
		case "T":
			c.useTool(int(w.v), line)
		case "M", "N", "P", "Q", "H", "D", "L":
		default:
			c.add("unsupported", line, "word %s", w.letter)
		}
	}
	if target[0] == nil && target[1] == nil && target[2] == nil {
		return
	}
	if c.motion < 0 {
		c.add("unsupported", line, "a move with no motion mode (G0 to G3, or a drilling cycle)")
		return
	}
	dest, destKnown := c.pos, c.known
	for i := 0; i < 3; i++ {
		if target[i] != nil {
			if c.absolute || !c.known[i] {
				dest[i] = *target[i]
			} else {
				dest[i] += *target[i]
			}
			destKnown[i] = true
		}
	}
	switch c.motion {
	case 0:
		c.rapid(line, c.pos, c.known, dest)
		c.trace(line, c.pos, c.known, dest, destKnown, true)
		c.moveTo(dest, destKnown)
	case 1:
		c.cut(line)
		if c.known == [3]bool{true, true, true} && destKnown == c.known {
			path, n := straight(c.pos, dest)
			c.sweep(line, path, n)
		}
		c.trace(line, c.pos, c.known, dest, destKnown, false)
		c.moveTo(dest, destKnown)
	case 2, 3:
		if !hasIJ || hasR {
			c.add("unsupported", line, "an arc without I and J (an R-form arc)")
			c.moveTo(dest, destKnown)
			return
		}
		c.cut(line)
		if c.known == [3]bool{true, true, true} && destKnown == c.known {
			path, n := arc(c.pos, dest, ij, c.motion == 2)
			c.sweep(line, path, n)
			pieces := max(int(math.Ceil(n/arcPiece)), 1)
			for k := 0; k < pieces; k++ {
				c.trace(line, path(float64(k)/float64(pieces)), c.known, path(float64(k+1)/float64(pieces)), destKnown, false)
			}
		}
		from, fromKnown := c.pos, c.known
		c.moveTo(dest, destKnown)
		if fromKnown[0] && fromKnown[1] {
			arcExtremes(from, dest, ij, c.motion == 2, c.extend)
		}
	default: // a drilling cycle
		c.drill(line, dest, destKnown)
	}
}

func (c *checker) cut(line int) {
	if !(c.feed > 0) {
		c.add("feed", line, "a cutting move with no feed rate")
	}
}

func (c *checker) moveTo(p [3]float64, known [3]bool) {
	c.pos, c.known = p, known
	c.r.Moves++
	c.extend(p)
}

// drill follows one hole of a cycle: a rapid across at the start height, a
// rapid down to R, a feed to the hole's bottom, and back up to R (G99) or
// the start height (G98).
func (c *checker) drill(line int, dest [3]float64, known [3]bool) {
	start := c.pos
	if !c.known[2] {
		c.add("unsupported", line, "a drilling cycle from an unknown height")
		return
	}
	bottom := dest[2]
	above := start
	above[0], above[1] = dest[0], dest[1]
	c.rapid(line, start, c.known, above)
	c.trace(line, start, c.known, above, known, true)
	atR := above
	atR[2] = c.retract
	c.rapid(line, above, known, atR)
	c.trace(line, above, known, atR, known, true)
	c.cut(line)
	hole := atR
	hole[2] = bottom
	k := known
	k[2] = true
	c.moveTo(atR, k)
	// The feed into the hole cuts the stock (and is checked, unless the
	// tool is a drill), so a later pass that enters the hole finds it cut.
	// (A cycle whose R plane is below its bottom is malformed; it cuts
	// nothing here.)
	if k == [3]bool{true, true, true} && atR[2] >= hole[2] {
		path, n := straight(atR, hole)
		c.sweep(line, path, n)
	}
	c.trace(line, atR, k, hole, k, false)
	c.moveTo(hole, k)
	back := hole
	back[2] = c.retract
	if c.oldZ {
		back[2] = math.Max(start[2], c.retract)
	}
	c.trace(line, hole, k, back, k, true)
	c.moveTo(back, k)
}

// rapid flags a G0 from a to b whose part below the stock's top passes
// over the stock, unless it goes straight up (out of a cut). From an
// unknown height, the tool is taken to come from above.
func (c *checker) rapid(line int, a [3]float64, aKnown [3]bool, b [3]float64) {
	s := c.stock
	top := s.ZMax
	if !aKnown[0] || !aKnown[1] {
		a[0], a[1] = b[0], b[1]
	}
	if !aKnown[2] {
		a[2] = math.Inf(1)
	}
	xyMoves := math.Abs(b[0]-a[0]) > eps || math.Abs(b[1]-a[1]) > eps
	if !xyMoves && b[2] >= a[2]-eps {
		return // straight up, or not at all
	}
	// The part of the move at or below the stock's top: t in [t0, t1].
	t0, t1 := 0.0, 1.0
	za, zb := a[2], b[2]
	switch {
	case za >= top-eps && zb >= top-eps:
		return
	case math.IsInf(za, 1):
		t0 = 1 // from above: only the end is low
	case za < top-eps && zb < top-eps:
	case za < top-eps:
		t1 = (top - za) / (zb - za)
	default:
		t0 = (za - top) / (za - zb)
	}
	p := func(t float64) (float64, float64) {
		if math.IsInf(za, 1) {
			return b[0], b[1]
		}
		return a[0] + t*(b[0]-a[0]), a[1] + t*(b[1]-a[1])
	}
	x0, y0 := p(t0)
	x1, y1 := p(t1)
	if !clipHits(x0, y0, x1, y1, s.XMin-eps, s.XMax+eps, s.YMin-eps, s.YMax+eps) {
		return
	}
	if aKnown == [3]bool{true, true, true} && c.cleared(a, b) {
		return // through stock the job has already cut away
	}
	from := "an unknown height"
	if aKnown[2] {
		from = fmt.Sprintf("Z %.3f", a[2])
	}
	c.add("rapid", line, "G0 from %s to Z %.3f over the stock, below its top at Z %.3f", from, b[2], top)
}

// clipHits says whether the segment (x0,y0)-(x1,y1) meets the rectangle
// (Liang-Barsky).
func clipHits(x0, y0, x1, y1, xmin, xmax, ymin, ymax float64) bool {
	t0, t1 := 0.0, 1.0
	dx, dy := x1-x0, y1-y0
	for _, e := range [][2]float64{{-dx, x0 - xmin}, {dx, xmax - x0}, {-dy, y0 - ymin}, {dy, ymax - y0}} {
		p, q := e[0], e[1]
		if p == 0 {
			if q < 0 {
				return false
			}
			continue
		}
		r := q / p
		if p < 0 {
			t0 = math.Max(t0, r)
		} else {
			t1 = math.Min(t1, r)
		}
		if t0 > t1 {
			return false
		}
	}
	return true
}

// arcExtremes extends the bounds by the points of an XY arc at 0, 90, 180
// and 270 degrees that the arc passes. A full circle (from = to) passes
// all four.
func arcExtremes(from, to [3]float64, ij [2]float64, clockwise bool, extend func([3]float64)) {
	cx, cy := from[0]+ij[0], from[1]+ij[1]
	rad := math.Hypot(ij[0], ij[1])
	a0 := math.Atan2(from[1]-cy, from[0]-cx)
	a1 := math.Atan2(to[1]-cy, to[0]-cx)
	tau := 2 * math.Pi
	mod := func(x float64) float64 { return math.Mod(math.Mod(x, tau)+tau, tau) }
	// The sweep, as a positive angle in the arc's own direction.
	sweep := mod(a1 - a0)
	if clockwise {
		sweep = mod(a0 - a1)
	}
	if sweep < 1e-9 {
		sweep = tau
	}
	for k := 0; k < 4; k++ {
		ang := float64(k) * math.Pi / 2
		d := mod(ang - a0)
		if clockwise {
			d = mod(a0 - ang)
		}
		if d > sweep+1e-12 {
			continue
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

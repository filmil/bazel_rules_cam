// SPDX-License-Identifier: Apache-2.0

// Package gcoderender draws a job's toolpath and writes a report of it
// (gcode_render, issue #35): a top view and an isometric view as PNGs,
// and a Markdown summary per operation. It reads the G-code through
// gcodecheck, so the pictures and the check follow the same moves.
//
// In the pictures, cutting moves are colored by depth, from the stock's
// top (light blue) to the deepest cut (dark red); rapids are dashed gray;
// the stock is a gray box. The PNGs hold no text and no metadata, so the
// same G-code gives the same bytes; the legend is in the report.
package gcoderender

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"sort"
	"strings"

	"github.com/filmil/bazel_rules_cam/cam/private/gcodecheck"
)

// Size is the pictures' width and height, in pixels.
const Size = 1200

const margin = 40

var (
	white    = color.RGBA{255, 255, 255, 255}
	stockCol = color.RGBA{150, 150, 150, 255}
	rapidCol = color.RGBA{120, 120, 120, 255}
	// The depth ramp: the stock's top to the deepest cut.
	ramp = []color.RGBA{{120, 190, 230, 255}, {40, 110, 200, 255}, {120, 60, 170, 255}, {200, 40, 60, 255}, {120, 10, 20, 255}}
)

// projection maps a point of the job to the picture's plane (u right, v up).
type projection func(p [3]float64) (u, v float64)

// Top looks down Z.
func Top(p [3]float64) (float64, float64) { return p[0], p[1] }

// Iso is the isometric view: X to the right and down, Y to the left and
// down, Z up.
func Iso(p [3]float64) (float64, float64) {
	c, s := math.Cos(math.Pi/6), math.Sin(math.Pi/6)
	return (p[0] - p[1]) * c, (p[0]+p[1])*s + p[2]
}

// canvas is an RGBA picture with a fixed world-to-pixel map.
type canvas struct {
	img          *image.RGBA
	proj         projection
	u0, v0, k    float64
	offU, offV   float64
}

func newCanvas(proj projection, pts [][3]float64) *canvas {
	img := image.NewRGBA(image.Rect(0, 0, Size, Size))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	umin, vmin, umax, vmax := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range pts {
		u, v := proj(p)
		umin, umax = math.Min(umin, u), math.Max(umax, u)
		vmin, vmax = math.Min(vmin, v), math.Max(vmax, v)
	}
	if !(umax > umin) {
		umin, umax = umin-1, umin+1
	}
	if !(vmax > vmin) {
		vmin, vmax = vmin-1, vmin+1
	}
	room := float64(Size - 2*margin)
	k := math.Min(room/(umax-umin), room/(vmax-vmin))
	c := &canvas{img: img, proj: proj, u0: umin, v0: vmin, k: k}
	// Center the drawing.
	c.offU = (room - (umax-umin)*k) / 2
	c.offV = (room - (vmax-vmin)*k) / 2
	return c
}

func (c *canvas) px(p [3]float64) (float64, float64) {
	u, v := c.proj(p)
	x := float64(margin) + c.offU + (u-c.u0)*c.k
	y := float64(Size-margin) - c.offV - (v-c.v0)*c.k
	return x, y
}

// blend paints col over the pixel at (x, y) with coverage a in [0, 1].
func (c *canvas) blend(x, y int, col color.RGBA, a float64) {
	if x < 0 || y < 0 || x >= Size || y >= Size || a <= 0 {
		return
	}
	i := c.img.PixOffset(x, y)
	for j, ch := range []uint8{col.R, col.G, col.B} {
		old := float64(c.img.Pix[i+j])
		c.img.Pix[i+j] = uint8(math.Round(old + (float64(ch)-old)*math.Min(a, 1)))
	}
}

// line draws an anti-aliased line of the given width in pixels, as
// samples of a soft disc along it. dash > 0 draws dashes of that length.
func (c *canvas) line(a, b [3]float64, col color.RGBA, width, dash float64) {
	x0, y0 := c.px(a)
	x1, y1 := c.px(b)
	l := math.Hypot(x1-x0, y1-y0)
	n := int(math.Ceil(l*2)) + 1
	r := width / 2
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		if dash > 0 && int(t*l/dash)%2 == 1 {
			continue
		}
		cx, cy := x0+(x1-x0)*t, y0+(y1-y0)*t
		for y := int(math.Floor(cy - r - 1)); y <= int(math.Ceil(cy+r+1)); y++ {
			for x := int(math.Floor(cx - r - 1)); x <= int(math.Ceil(cx+r+1)); x++ {
				d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
				// Full inside r, fading over the next pixel; repeated
				// samples only darken toward col, never past it.
				c.blend(x, y, col, (r+0.5-d)*0.6)
			}
		}
	}
}

// depthColor is the ramp's color for z between top and bottom.
func depthColor(z, top, bottom float64) color.RGBA {
	t := 0.0
	if top > bottom {
		t = math.Max(0, math.Min(1, (top-z)/(top-bottom)))
	}
	f := t * float64(len(ramp)-1)
	i := int(math.Floor(f))
	if i >= len(ramp)-1 {
		return ramp[len(ramp)-1]
	}
	a, b, w := ramp[i], ramp[i+1], f-float64(i)
	mix := func(p, q uint8) uint8 { return uint8(math.Round(float64(p) + (float64(q)-float64(p))*w)) }
	return color.RGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), 255}
}

// stockEdges are the stock box's twelve edges.
func stockEdges(s gcodecheck.Stock) [][2][3]float64 {
	x := [2]float64{s.XMin, s.XMax}
	y := [2]float64{s.YMin, s.YMax}
	z := [2]float64{s.ZMin, s.ZMax}
	var e [][2][3]float64
	for _, a := range [][3]int{{0, 0, 0}, {0, 0, 1}, {0, 1, 0}, {0, 1, 1}, {1, 0, 0}, {1, 0, 1}, {1, 1, 0}, {1, 1, 1}} {
		p := [3]float64{x[a[0]], y[a[1]], z[a[2]]}
		for k := 0; k < 3; k++ {
			if a[k] == 0 {
				q := p
				switch k {
				case 0:
					q[0] = x[1]
				case 1:
					q[1] = y[1]
				case 2:
					q[2] = z[1]
				}
				e = append(e, [2][3]float64{p, q})
			}
		}
	}
	return e
}

// Draw renders the path in one view.
func Draw(r gcodecheck.Result, s gcodecheck.Stock, proj projection) []byte {
	pts := [][3]float64{}
	for _, e := range stockEdges(s) {
		pts = append(pts, e[0], e[1])
	}
	for _, g := range r.Path {
		pts = append(pts, g.From, g.To)
	}
	c := newCanvas(proj, pts)
	for _, e := range stockEdges(s) {
		c.line(e[0], e[1], stockCol, 1.2, 0)
	}
	bottom := s.ZMax
	for _, g := range r.Path {
		if !g.Rapid {
			bottom = math.Min(bottom, math.Min(g.From[2], g.To[2]))
		}
	}
	for _, g := range r.Path {
		if g.Rapid {
			c.line(g.From, g.To, rapidCol, 1, 6)
		}
	}
	for _, g := range r.Path {
		if !g.Rapid {
			c.line(g.From, g.To, depthColor(math.Min(g.From[2], g.To[2]), s.ZMax, bottom), 2, 0)
		}
	}
	var b bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&b, c.img); err != nil {
		panic(err) // writing to memory does not fail
	}
	return b.Bytes()
}

// opStats is what the report says of one operation.
type opStats struct {
	name                 string
	tools                map[int]bool
	levels               map[float64]bool
	cut, rapid, minutes  float64
	zmin                 float64
	first                int
}

// Report is the Markdown summary of the path. rapidFeed is the speed of a
// rapid for the time estimate (the machine's max_feed), mm/min.
func Report(name string, r gcodecheck.Result, s gcodecheck.Stock, tools map[int]gcodecheck.Tool, rapidFeed float64) string {
	ops := map[string]*opStats{}
	var order []string
	for _, g := range r.Path {
		o := ops[g.Op]
		if o == nil {
			o = &opStats{name: g.Op, tools: map[int]bool{}, levels: map[float64]bool{}, zmin: math.Inf(1), first: g.Line}
			ops[g.Op] = o
			order = append(order, g.Op)
		}
		l := math.Sqrt(sq(g.To[0]-g.From[0]) + sq(g.To[1]-g.From[1]) + sq(g.To[2]-g.From[2]))
		if g.Rapid {
			o.rapid += l
			if rapidFeed > 0 {
				o.minutes += l / rapidFeed
			}
			continue
		}
		o.cut += l
		if g.Feed > 0 {
			o.minutes += l / g.Feed
		}
		o.tools[g.Tool] = true
		o.zmin = math.Min(o.zmin, math.Min(g.From[2], g.To[2]))
		// A level is a Z at which the tool moves across.
		if math.Abs(g.From[2]-g.To[2]) < 1e-6 && math.Hypot(g.To[0]-g.From[0], g.To[1]-g.From[1]) > 1e-6 {
			o.levels[math.Round(g.From[2]*1000)/1000] = true
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Toolpath of %s\n\n", name)
	fmt.Fprintf(&b, "Stock: X %.3f to %.3f, Y %.3f to %.3f, Z %.3f to %.3f mm (top at Z %.3f).\n\n",
		s.XMin, s.XMax, s.YMin, s.YMax, s.ZMin, s.ZMax, s.ZMax)
	fmt.Fprintf(&b, "| Operation | Tools | Levels (Z, mm) | Deepest Z | Cut (mm) | Rapid (mm) | Time (min) |\n")
	fmt.Fprintf(&b, "|---|---|---|---|---|---|---|\n")
	var total float64
	for _, k := range order {
		o := ops[k]
		total += o.minutes
		name := o.name
		if name == "" {
			name = "(none named)"
		}
		var ts []string
		for _, t := range sortedInts(o.tools) {
			d := ""
			if tl, ok := tools[t]; ok {
				d = fmt.Sprintf(" (%g mm %s)", tl.Diameter, tl.Shape)
			}
			ts = append(ts, fmt.Sprintf("T%d%s", t, d))
		}
		deep := "-"
		if !math.IsInf(o.zmin, 1) {
			deep = fmt.Sprintf("%.3f", o.zmin)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %.1f | %.1f | %.1f |\n",
			name, strings.Join(ts, ", "), levels(o.levels), deep, o.cut, o.rapid, o.minutes)
	}
	fmt.Fprintf(&b, "\nEstimated time: %.1f min (feeds as written; rapids at %g mm/min; no acceleration).\n", total, rapidFeed)
	if len(r.Violations) > 0 {
		fmt.Fprintf(&b, "\n**gcode_check finds %d violations**; see gcode_check_test.\n", len(r.Violations))
	}
	fmt.Fprintf(&b, "\nIn the pictures, cuts are colored by depth: light blue at the stock's top, through blue and purple, to dark red at the deepest cut. Rapids are dashed gray, and the stock is the gray box.\n")
	return b.String()
}

func sq(x float64) float64 { return x * x }

func sortedInts(m map[int]bool) []int {
	var s []int
	for k := range m {
		s = append(s, k)
	}
	sort.Ints(s)
	return s
}

// levels lists Z levels, top first; many of them are shortened.
func levels(m map[float64]bool) string {
	var s []float64
	for k := range m {
		s = append(s, k)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(s)))
	var out []string
	for _, z := range s {
		out = append(out, fmt.Sprintf("%g", z))
	}
	if len(out) > 8 {
		return fmt.Sprintf("%s ... %s (%d)", strings.Join(out[:3], ", "), strings.Join(out[len(out)-2:], ", "), len(out))
	}
	if len(out) == 0 {
		return "-"
	}
	return strings.Join(out, ", ")
}

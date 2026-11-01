// SPDX-License-Identifier: Apache-2.0

package gcodecheck

import "math"

// Tool is what the depth check needs of a cutter.
type Tool struct {
	Shape       string
	Diameter    float64
	MaxStepdown float64 // 0: the diameter
}

// limit is the deepest cut the tool may take.
func (t Tool) limit() float64 {
	if t.MaxStepdown > 0 {
		return t.MaxStepdown
	}
	return t.Diameter
}

// heights is the stock's top, as a grid of cells over its XY box: each
// cutting move lowers the cells under the tool to the tool's bottom. It is
// a coarse model of the stock (a flat-bottomed tool, no side load), enough
// to tell how deep a cut goes into material that is still there.
type heights struct {
	x0, y0, cell float64
	nx, ny       int
	h            []float32
}

// maxCells bounds the grid's side; a cell is at least 0.05 mm.
const maxCells = 2000

func newHeights(s Stock) *heights {
	dx, dy := s.XMax-s.XMin, s.YMax-s.YMin
	if !(dx > 0 && dy > 0) {
		return nil
	}
	cell := math.Max(0.05, math.Max(dx, dy)/maxCells)
	m := &heights{x0: s.XMin, y0: s.YMin, cell: cell, nx: int(math.Ceil(dx / cell)), ny: int(math.Ceil(dy / cell))}
	m.h = make([]float32, m.nx*m.ny)
	for i := range m.h {
		m.h[i] = float32(s.ZMax)
	}
	return m
}

// disc calls f for each cell whose center is within r of (x, y). A tool
// smaller than a cell can cover no cell's center; then it is the cell
// under (x, y), so that no tool goes unchecked or cuts nothing.
func (m *heights) disc(x, y, r float64, f func(i int)) {
	if r <= 0 {
		return
	}
	any := false
	defer func() {
		i, j := int(math.Floor((x-m.x0)/m.cell)), int(math.Floor((y-m.y0)/m.cell))
		if !any && i >= 0 && i < m.nx && j >= 0 && j < m.ny {
			f(j*m.nx + i)
		}
	}()
	i0 := int(math.Floor((x - r - m.x0) / m.cell))
	i1 := int(math.Floor((x + r - m.x0) / m.cell))
	j0 := int(math.Floor((y - r - m.y0) / m.cell))
	j1 := int(math.Floor((y + r - m.y0) / m.cell))
	i0, j0 = max(i0, 0), max(j0, 0)
	i1, j1 = min(i1, m.nx-1), min(j1, m.ny-1)
	for j := j0; j <= j1; j++ {
		cy := m.y0 + (float64(j)+0.5)*m.cell - y
		for i := i0; i <= i1; i++ {
			cx := m.x0 + (float64(i)+0.5)*m.cell - x
			if cx*cx+cy*cy <= r*r {
				any = true
				f(j*m.nx + i)
			}
		}
	}
}

// depth is how far below the stock still under it the tool's bottom is.
func (m *heights) depth(p [3]float64, r float64) float64 {
	d := 0.0
	m.disc(p[0], p[1], r, func(i int) {
		d = math.Max(d, float64(m.h[i])-p[2])
	})
	return d
}

// stamp cuts the stock under the tool. It cuts half a cell wider than the
// tool, so that a pass that repeats an earlier one's path, give or take a
// rounding, does not find the cells on its rim uncut; the next sample, a
// cell on, still meets the stock ahead of it. The price: a pass beside an
// earlier one may hide up to half a cell of its shoulder (0.025 mm on
// stock up to 100 mm across).
func (m *heights) stamp(p [3]float64, r float64) {
	z := float32(p[2])
	m.disc(p[0], p[1], r+m.cell/2, func(i int) {
		if z < m.h[i] {
			m.h[i] = z
		}
	})
}

// sweep follows a cutting move, given as a path over t in [0, 1] and its
// length, through the stock: a "depth" violation where the tool, anywhere
// along the move, is deeper than it may be below the stock as it was
// before the move; then the move cuts the stock. Measuring against the
// stock before the move, not as the move itself cuts it, counts a plunge,
// a ramp or a helix turn by the depth it reaches. A drill is meant to
// plunge, so it cuts without the check. Nothing is checked before the
// stock is known, or with a tool that is not in the library.
func (c *checker) sweep(line int, path func(t float64) [3]float64, length float64) {
	if c.hm == nil {
		return
	}
	t, ok := c.tools[c.tool]
	if !ok || !(t.Diameter > 0) {
		return
	}
	r := t.Diameter / 2
	n := int(math.Ceil(length/c.hm.cell)) + 1
	worst := 0.0
	if t.Shape != "drill" {
		for k := 0; k <= n; k++ {
			worst = math.Max(worst, c.hm.depth(path(float64(k)/float64(n)), r))
		}
	}
	for k := 0; k <= n; k++ {
		c.hm.stamp(path(float64(k)/float64(n)), r)
	}
	if worst > t.limit()+1e-3 {
		c.add("depth", line, "a cut %.3f mm deep into the stock with T%d; it takes at most %g mm (its max_stepdown, or else its diameter)", worst, c.tool, t.limit())
	}
}

// straight is a straight move's path.
func straight(a, b [3]float64) (func(float64) [3]float64, float64) {
	return func(t float64) [3]float64 {
		return [3]float64{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
	}, math.Sqrt((b[0]-a[0])*(b[0]-a[0]) + (b[1]-a[1])*(b[1]-a[1]) + (b[2]-a[2])*(b[2]-a[2]))
}

// arc is an XY arc's path (a helix, if Z changes): from a to b about
// a + ij, clockwise for G2. An arc that ends where it starts is a full
// circle.
func arc(a, b [3]float64, ij [2]float64, clockwise bool) (func(float64) [3]float64, float64) {
	cx, cy := a[0]+ij[0], a[1]+ij[1]
	r0 := math.Hypot(a[0]-cx, a[1]-cy)
	r1 := math.Hypot(b[0]-cx, b[1]-cy)
	a0 := math.Atan2(a[1]-cy, a[0]-cx)
	a1 := math.Atan2(b[1]-cy, b[0]-cx)
	sweep := a1 - a0
	if clockwise {
		sweep = a0 - a1
	}
	for sweep <= 1e-9 {
		sweep += 2 * math.Pi
	}
	dir := 1.0
	if clockwise {
		dir = -1
	}
	return func(t float64) [3]float64 {
		ang := a0 + dir*sweep*t
		rr := r0 + (r1-r0)*t
		return [3]float64{cx + rr*math.Cos(ang), cy + rr*math.Sin(ang), a[2] + (b[2]-a[2])*t}
	}, sweep*math.Max(r0, r1) + math.Abs(b[2]-a[2])
}

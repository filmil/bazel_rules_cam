// SPDX-License-Identifier: Apache-2.0

// Package toolcheck validates a tool library (tool_library, issue #8) and
// writes it in a canonical form, which cam_job reads.
//
// A library is JSON: {"tools": [TOOL, ...]}. Each tool has a number (the
// T word in the G-code), a name, a shape (one of FreeCAD's tool bit
// shapes) and the dimensions that shape needs, in mm and degrees:
//
//	{"number": 1, "name": "3 mm flat end mill", "shape": "endmill",
//	 "diameter": 3, "length": 38, "cutting_edge_height": 12}
//
// Unknown keys, a dimension the shape does not use, a missing one, a
// duplicate number or name, and dimensions that cannot fit together are
// errors. The canonical form sorts the tools by number and writes every
// number in one way, so two libraries that mean the same give the same
// bytes.
package toolcheck

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Tool is one cutter of the library. Optional dimensions are pointers, so
// a missing one is told apart from zero.
type Tool struct {
	Number            int      `json:"number"`
	Name              string   `json:"name"`
	Shape             string   `json:"shape"`
	Diameter          *float64 `json:"diameter,omitempty"`
	Length            *float64 `json:"length,omitempty"`
	CuttingEdgeHeight *float64 `json:"cutting_edge_height,omitempty"`
	ShankDiameter     *float64 `json:"shank_diameter,omitempty"`
	Flutes            *int     `json:"flutes,omitempty"`
	FlatRadius        *float64 `json:"flat_radius,omitempty"`
	TipAngle          *float64 `json:"tip_angle,omitempty"`
	CuttingEdgeAngle  *float64 `json:"cutting_edge_angle,omitempty"`
	TipDiameter       *float64 `json:"tip_diameter,omitempty"`
}

// Library is the whole file.
type Library struct {
	Tools []Tool `json:"tools"`
}

// Shapes lists, for each shape, the dimensions it needs beyond diameter
// and length, and the ones it may have. The names are FreeCAD's tool bit
// shapes (Mod/CAM/Tools/Shape/<name>.fcstd), with v-bit spelled vbit.
var Shapes = map[string]struct{ Need, May []string }{
	"endmill":  {Need: []string{"cutting_edge_height"}, May: []string{"shank_diameter", "flutes"}},
	"ballend":  {Need: []string{"cutting_edge_height"}, May: []string{"shank_diameter", "flutes"}},
	"bullnose": {Need: []string{"cutting_edge_height", "flat_radius"}, May: []string{"shank_diameter", "flutes"}},
	"drill":    {Need: []string{"tip_angle"}, May: []string{"shank_diameter", "flutes"}},
	"vbit":     {Need: []string{"cutting_edge_angle", "cutting_edge_height", "tip_diameter"}, May: []string{"shank_diameter", "flutes"}},
	"chamfer":  {Need: []string{"cutting_edge_angle", "cutting_edge_height", "tip_diameter"}, May: []string{"shank_diameter", "flutes"}},
}

// ShapeNames is the sorted list of shapes, for messages.
func ShapeNames() []string {
	var s []string
	for k := range Shapes {
		s = append(s, k)
	}
	sort.Strings(s)
	return s
}

func (t *Tool) present() map[string]bool {
	return map[string]bool{
		"cutting_edge_height": t.CuttingEdgeHeight != nil,
		"shank_diameter":      t.ShankDiameter != nil,
		"flutes":              t.Flutes != nil,
		"flat_radius":         t.FlatRadius != nil,
		"tip_angle":           t.TipAngle != nil,
		"cutting_edge_angle":  t.CuttingEdgeAngle != nil,
		"tip_diameter":        t.TipDiameter != nil,
	}
}

// Parse reads a library strictly: one JSON object, no unknown keys.
func Parse(data []byte) (*Library, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	var l Library
	if err := d.Decode(&l); err != nil {
		return nil, fmt.Errorf("not a tool library: %v", err)
	}
	if d.More() {
		return nil, errors.New("not a tool library: more than one JSON value")
	}
	return &l, nil
}

func positive(errs *[]string, who, name string, v *float64) {
	if v != nil && !(*v > 0 && !math.IsInf(*v, 0)) {
		*errs = append(*errs, fmt.Sprintf("%s: %s is %g; want a number above 0", who, name, *v))
	}
}

// Check returns every problem of the library, one message each, in tool
// order; none means it is valid.
func Check(l *Library) []string {
	var errs []string
	if len(l.Tools) == 0 {
		return []string{"the library has no tools"}
	}
	numbers, names := map[int]bool{}, map[string]bool{}
	for i, t := range l.Tools {
		who := fmt.Sprintf("tool %d (%q)", t.Number, t.Name)
		if t.Number < 1 || t.Number > 999 {
			errs = append(errs, fmt.Sprintf("tool %d of the list: number %d; want 1 to 999", i+1, t.Number))
		} else if numbers[t.Number] {
			errs = append(errs, fmt.Sprintf("%s: number %d appears twice", who, t.Number))
		}
		numbers[t.Number] = true
		if strings.TrimSpace(t.Name) == "" || t.Name != strings.TrimSpace(t.Name) {
			errs = append(errs, fmt.Sprintf("tool %d: name %q; want a name without spaces around it", t.Number, t.Name))
		} else if names[t.Name] {
			errs = append(errs, fmt.Sprintf("%s: name appears twice", who))
		}
		names[t.Name] = true
		shape, ok := Shapes[t.Shape]
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: shape %q; want one of %s", who, t.Shape, strings.Join(ShapeNames(), ", ")))
			continue
		}
		if t.Diameter == nil {
			errs = append(errs, fmt.Sprintf("%s: no diameter", who))
		}
		if t.Length == nil {
			errs = append(errs, fmt.Sprintf("%s: no length", who))
		}
		allowed := map[string]bool{}
		have := t.present()
		for _, n := range shape.Need {
			allowed[n] = true
			if !have[n] {
				errs = append(errs, fmt.Sprintf("%s: shape %s needs %s", who, t.Shape, n))
			}
		}
		for _, n := range shape.May {
			allowed[n] = true
		}
		var extra []string
		for n, h := range have {
			if h && !allowed[n] {
				extra = append(extra, n)
			}
		}
		sort.Strings(extra)
		for _, n := range extra {
			errs = append(errs, fmt.Sprintf("%s: shape %s has no %s", who, t.Shape, n))
		}
		for _, p := range []struct {
			n string
			v *float64
		}{
			{"diameter", t.Diameter}, {"length", t.Length}, {"cutting_edge_height", t.CuttingEdgeHeight},
			{"shank_diameter", t.ShankDiameter}, {"flat_radius", t.FlatRadius}, {"tip_angle", t.TipAngle},
			{"cutting_edge_angle", t.CuttingEdgeAngle},
		} {
			positive(&errs, who, p.n, p.v)
		}
		if t.TipDiameter != nil && !(*t.TipDiameter >= 0) {
			errs = append(errs, fmt.Sprintf("%s: tip_diameter is %g; want 0 or more", who, *t.TipDiameter))
		}
		if t.Flutes != nil && (*t.Flutes < 1 || *t.Flutes > 16) {
			errs = append(errs, fmt.Sprintf("%s: flutes is %d; want 1 to 16", who, *t.Flutes))
		}
		for _, a := range []struct {
			n string
			v *float64
		}{{"tip_angle", t.TipAngle}, {"cutting_edge_angle", t.CuttingEdgeAngle}} {
			if a.v != nil && *a.v >= 180 {
				errs = append(errs, fmt.Sprintf("%s: %s is %g degrees; want less than 180", who, a.n, *a.v))
			}
		}
		// Dimensions that cannot fit together.
		if t.Diameter != nil && t.Length != nil {
			if t.CuttingEdgeHeight != nil && *t.CuttingEdgeHeight > *t.Length {
				errs = append(errs, fmt.Sprintf("%s: cutting_edge_height %g is more than the length %g", who, *t.CuttingEdgeHeight, *t.Length))
			}
			if t.FlatRadius != nil && *t.FlatRadius >= *t.Diameter/2 {
				errs = append(errs, fmt.Sprintf("%s: flat_radius %g; want less than the radius %g", who, *t.FlatRadius, *t.Diameter/2))
			}
			if t.TipDiameter != nil && *t.TipDiameter >= *t.Diameter {
				errs = append(errs, fmt.Sprintf("%s: tip_diameter %g; want less than the diameter %g", who, *t.TipDiameter, *t.Diameter))
			}
		}
	}
	return errs
}

// Canonical writes a valid library sorted by number, with two-space
// indent and a final newline. encoding/json writes each float in its
// shortest exact form, so 3, 3.0 and 3.00 all give 3.
func Canonical(l *Library) ([]byte, error) {
	tools := append([]Tool(nil), l.Tools...)
	sort.Slice(tools, func(i, j int) bool { return tools[i].Number < tools[j].Number })
	out, err := json.MarshalIndent(Library{Tools: tools}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

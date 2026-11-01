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
	"io"
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
	CornerRadius      *float64 `json:"corner_radius,omitempty"`
	TipAngle          *float64 `json:"tip_angle,omitempty"`
	CuttingEdgeAngle  *float64 `json:"cutting_edge_angle,omitempty"`
	TipDiameter       *float64 `json:"tip_diameter,omitempty"`
	// MaxStepdown is the deepest cut the tool may take, for
	// gcode_check_test; FreeCAD does not read it.
	MaxStepdown *float64 `json:"max_stepdown,omitempty"`
}

// Library is the whole file.
type Library struct {
	Tools []Tool `json:"tools"`
}

// Shapes lists, for each shape, the dimensions it needs beyond diameter
// and length, and the ones it may have. The names are FreeCAD's tool bit
// shapes (Mod/CAM/Tools/Shape/<name>.fcstd), with v-bit spelled vbit.
var Shapes = map[string]struct{ Need, May []string }{
	"endmill":  {Need: []string{"cutting_edge_height"}, May: []string{"shank_diameter", "flutes", "max_stepdown"}},
	"ballend":  {Need: []string{"cutting_edge_height"}, May: []string{"shank_diameter", "flutes", "max_stepdown"}},
	"bullnose": {Need: []string{"cutting_edge_height", "corner_radius"}, May: []string{"shank_diameter", "flutes", "max_stepdown"}},
	"drill":    {Need: []string{"tip_angle"}, May: []string{"shank_diameter", "flutes", "max_stepdown"}},
	"vbit":     {Need: []string{"cutting_edge_angle", "cutting_edge_height", "tip_diameter"}, May: []string{"shank_diameter", "flutes", "max_stepdown"}},
	"chamfer":  {Need: []string{"cutting_edge_angle", "cutting_edge_height", "tip_diameter"}, May: []string{"shank_diameter", "flutes", "max_stepdown"}},
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
		"corner_radius":       t.CornerRadius != nil,
		"tip_angle":           t.TipAngle != nil,
		"cutting_edge_angle":  t.CuttingEdgeAngle != nil,
		"tip_diameter":        t.TipDiameter != nil,
		"max_stepdown":        t.MaxStepdown != nil,
	}
}

// Parse reads a library strictly: one JSON object, no unknown keys, no
// null tools, nothing after it.
func Parse(data []byte) (*Library, error) {
	// First the list as raw values, so a null tool is told apart from an
	// empty one (encoding/json gives a zero Tool for both).
	var raw struct {
		Tools []json.RawMessage `json:"tools"`
	}
	if err := decodeOne(data, &raw, false); err != nil {
		return nil, err
	}
	for i, t := range raw.Tools {
		if string(bytes.TrimSpace(t)) == "null" {
			return nil, fmt.Errorf("not a tool library: tool %d of the list is null", i+1)
		}
	}
	var l Library
	if err := decodeOne(data, &l, true); err != nil {
		return nil, err
	}
	return &l, nil
}

// decodeOne decodes exactly one JSON value from data into v.
func decodeOne(data []byte, v any, strict bool) error {
	d := json.NewDecoder(bytes.NewReader(data))
	if strict {
		d.DisallowUnknownFields()
	}
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("not a tool library: %v", err)
	}
	// d.More is false before a stray ] or }, so read one more token.
	if _, err := d.Token(); err != io.EOF {
		return errors.New("not a tool library: something follows the JSON object")
	}
	return nil
}

func positive(errs *[]string, who, name string, v *float64) {
	if v != nil && !(*v > 0 && !math.IsInf(*v, 0)) {
		*errs = append(*errs, fmt.Sprintf("%s: %s is %g; want a number above 0", who, name, *v))
	}
}

// Check returns every problem of the library, one message each, in tool
// order; none means it is valid. A tool with an unknown shape still gets
// every check that does not depend on its shape.
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
		if t.Diameter == nil {
			errs = append(errs, fmt.Sprintf("%s: no diameter", who))
		}
		if t.Length == nil {
			errs = append(errs, fmt.Sprintf("%s: no length", who))
		}
		// An unknown shape still gets every check that does not depend on
		// the shape.
		shape, known := Shapes[t.Shape]
		if !known {
			errs = append(errs, fmt.Sprintf("%s: shape %q; want one of %s", who, t.Shape, strings.Join(ShapeNames(), ", ")))
		} else {
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
		}
		for _, p := range []struct {
			n string
			v *float64
		}{
			{"diameter", t.Diameter}, {"length", t.Length}, {"cutting_edge_height", t.CuttingEdgeHeight},
			{"shank_diameter", t.ShankDiameter}, {"corner_radius", t.CornerRadius}, {"tip_angle", t.TipAngle},
			{"cutting_edge_angle", t.CuttingEdgeAngle}, {"max_stepdown", t.MaxStepdown},
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
		// Dimensions that cannot fit together, checked only once the
		// diameter and length are themselves good.
		if t.Diameter != nil && t.Length != nil && *t.Diameter > 0 && *t.Length > 0 {
			r := *t.Diameter / 2
			if t.CuttingEdgeHeight != nil && *t.CuttingEdgeHeight > *t.Length {
				errs = append(errs, fmt.Sprintf("%s: cutting_edge_height %g is more than the length %g", who, *t.CuttingEdgeHeight, *t.Length))
			}
			if t.MaxStepdown != nil && t.CuttingEdgeHeight != nil && *t.MaxStepdown > *t.CuttingEdgeHeight {
				errs = append(errs, fmt.Sprintf("%s: max_stepdown %g is more than the cutting_edge_height %g", who, *t.MaxStepdown, *t.CuttingEdgeHeight))
			}
			if t.CornerRadius != nil && *t.CornerRadius > 0 && *t.CornerRadius >= r {
				errs = append(errs, fmt.Sprintf("%s: corner_radius %g; want less than the radius %g (a ballend has that)", who, *t.CornerRadius, r))
			}
			if t.TipDiameter != nil && *t.TipDiameter >= 0 && *t.TipDiameter >= *t.Diameter {
				errs = append(errs, fmt.Sprintf("%s: tip_diameter %g; want less than the diameter %g", who, *t.TipDiameter, *t.Diameter))
			}
			// The cutting edge holds the tip's own rounding: the ball of a
			// ballend, the corner radius of a bullnose.
			if t.CuttingEdgeHeight != nil && *t.CuttingEdgeHeight > 0 {
				if t.Shape == "ballend" && *t.CuttingEdgeHeight < r {
					errs = append(errs, fmt.Sprintf("%s: cutting_edge_height %g; want at least the ball's radius %g", who, *t.CuttingEdgeHeight, r))
				}
				if t.Shape == "bullnose" && t.CornerRadius != nil && *t.CornerRadius > 0 && *t.CornerRadius < r && *t.CuttingEdgeHeight < *t.CornerRadius {
					errs = append(errs, fmt.Sprintf("%s: cutting_edge_height %g; want at least the corner radius %g", who, *t.CuttingEdgeHeight, *t.CornerRadius))
				}
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
	for i := range tools {
		t := &tools[i]
		// -0 and 0 are one value; JSON would write them apart.
		for _, p := range []**float64{&t.Diameter, &t.Length, &t.CuttingEdgeHeight, &t.ShankDiameter,
			&t.CornerRadius, &t.TipAngle, &t.CuttingEdgeAngle, &t.TipDiameter, &t.MaxStepdown} {
			if *p != nil && **p == 0 {
				z := 0.0
				*p = &z
			}
		}
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Number < tools[j].Number })
	out, err := json.MarshalIndent(Library{Tools: tools}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

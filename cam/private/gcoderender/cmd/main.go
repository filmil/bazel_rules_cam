// SPDX-License-Identifier: Apache-2.0

// Command gcoderender writes a job's toolpath pictures and report
// (gcode_render). Usage:
//
//	gcoderender --gcode=G --tools=T --info=I --machine=M --name=N \
//	  --top=top.png --iso=iso.png --report=report.md
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/filmil/bazel_rules_cam/cam/private/gcodecheck"
	"github.com/filmil/bazel_rules_cam/cam/private/gcoderender"
)

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %v", path, err)
	}
	return nil
}

func run() error {
	gcode := flag.String("gcode", "", "the G-code")
	toolsPath := flag.String("tools", "", "the canonical tool library")
	info := flag.String("info", "", "the job's facts (stock box), JSON")
	machinePath := flag.String("machine", "", "the machine's limits, JSON")
	name := flag.String("name", "", "the job's name, for the report")
	top := flag.String("top", "", "where to write the top view")
	iso := flag.String("iso", "", "where to write the isometric view")
	report := flag.String("report", "", "where to write the report")
	flag.Parse()
	g, err := os.ReadFile(*gcode)
	if err != nil {
		return err
	}
	var m gcodecheck.Machine
	if err := readJSON(*machinePath, &m); err != nil {
		return err
	}
	var lib struct {
		Tools []struct {
			Number      int     `json:"number"`
			Shape       string  `json:"shape"`
			Diameter    float64 `json:"diameter"`
			MaxStepdown float64 `json:"max_stepdown"`
		} `json:"tools"`
	}
	if err := readJSON(*toolsPath, &lib); err != nil {
		return err
	}
	tools := map[int]gcodecheck.Tool{}
	for _, t := range lib.Tools {
		tools[t.Number] = gcodecheck.Tool{Shape: t.Shape, Diameter: t.Diameter, MaxStepdown: t.MaxStepdown}
	}
	var in struct {
		Stock gcodecheck.Stock `json:"stock"`
	}
	if err := readJSON(*info, &in); err != nil {
		return err
	}
	r := gcodecheck.Check(string(g), m, in.Stock, tools)
	if len(r.Path) == 0 {
		return fmt.Errorf("%s: no moves to draw", *gcode)
	}
	for path, data := range map[string][]byte{
		*top:    gcoderender.Draw(r, in.Stock, gcoderender.Top),
		*iso:    gcoderender.Draw(r, in.Stock, gcoderender.Iso),
		*report: []byte(gcoderender.Report(*name, r, in.Stock, tools, m.MaxFeed)),
	} {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gcoderender:", err)
		os.Exit(1)
	}
}

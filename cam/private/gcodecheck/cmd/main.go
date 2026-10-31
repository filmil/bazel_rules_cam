// SPDX-License-Identifier: Apache-2.0

// Command gcodecheck is gcode_check_test's executable: it checks a job's
// G-code and writes a report.
//
//	gcodecheck --gcode job.ngc --machine machine.json --tools tools.json \
//	           --info job.ngc.json [--expect KIND] [--report report.txt]
//
// Without --expect it passes when there are no violations. With --expect
// it passes when there are violations, all of that kind: the known-bad
// tests. The report goes to --report and to standard output.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/filmil/bazel_rules_cam/cam/private/gcodecheck"
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

func run(gcodePath, machinePath, toolsPath, infoPath, expect, reportPath string) (bool, string, error) {
	g, err := os.ReadFile(gcodePath)
	if err != nil {
		return false, "", err
	}
	var m gcodecheck.Machine
	if err := readJSON(machinePath, &m); err != nil {
		return false, "", err
	}
	var lib struct {
		Tools []struct {
			Number int `json:"number"`
		} `json:"tools"`
	}
	if err := readJSON(toolsPath, &lib); err != nil {
		return false, "", err
	}
	tools := map[int]bool{}
	for _, t := range lib.Tools {
		tools[t.Number] = true
	}
	var info struct {
		Stock gcodecheck.Stock `json:"stock"`
	}
	if err := readJSON(infoPath, &info); err != nil {
		return false, "", err
	}
	r := gcodecheck.Check(string(g), m, info.Stock, tools)
	report := gcodecheck.Report(r, m, info.Stock)
	kinds := gcodecheck.Kinds(r)
	var ok bool
	if expect == "" {
		ok = len(kinds) == 0
	} else {
		ok = len(kinds) == 1 && kinds[0] == expect
		report += fmt.Sprintf("expected only %q violations; found %v\n", expect, kinds)
	}
	if reportPath != "" {
		if err := os.WriteFile(reportPath, []byte(report), 0o644); err != nil {
			return false, report, err
		}
	}
	return ok, report, nil
}

func main() {
	gcode := flag.String("gcode", "", "the G-code")
	machine := flag.String("machine", "", "the machine's limits, JSON")
	tools := flag.String("tools", "", "the canonical tool library")
	info := flag.String("info", "", "the job's facts (stock box), JSON")
	expect := flag.String("expect", "", "pass only if every violation is of this kind")
	report := flag.String("report", "", "where to write the report")
	// As a test, its arguments come from GCODE_CHECK_ARGS, one per line,
	// followed by any the test was given (--test_arg).
	var args []string
	if e := os.Getenv("GCODE_CHECK_ARGS"); e != "" {
		args = strings.Split(e, "\n")
	}
	args = append(args, os.Args[1:]...)
	if err := flag.CommandLine.Parse(args); err != nil {
		os.Exit(2)
	}
	if *report == "" {
		if d := os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR"); d != "" {
			*report = filepath.Join(d, "gcode_check_report.txt")
		}
	}
	ok, text, err := run(*gcode, *machine, *tools, *info, *expect, *report)
	fmt.Print(text)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gcodecheck:", err)
		os.Exit(2)
	}
	if !ok {
		fmt.Println(strings.Repeat("-", 8), "FAIL")
		os.Exit(1)
	}
}

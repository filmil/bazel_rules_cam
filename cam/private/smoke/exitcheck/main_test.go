// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestParseCases(t *testing.T) {
	cs, err := ParseCases(`[{"script": "a.py", "args": ["-c", "x"], "want": 1}, {"script": "b.py", "args": ["-c", "y"], "want": 3}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 || cs[0].Want != 1 || cs[1].Want != 3 || cs[1].Args[1] != "y" {
		t.Errorf("got %+v", cs)
	}
	for _, bad := range []string{"", "[]", `[{"want": 1}]`, "{"} {
		if _, err := ParseCases(bad); err == nil {
			t.Errorf("ParseCases(%q) gave no error", bad)
		}
	}
}

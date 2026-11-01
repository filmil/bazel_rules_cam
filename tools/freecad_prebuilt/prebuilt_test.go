// SPDX-License-Identifier: Apache-2.0

package prebuilt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The sha256 the tool checks is the one the FreeCAD extension pins.
func TestExpectedMatchesExtension(t *testing.T) {
	p := filepath.Join(os.Getenv("TEST_SRCDIR"), os.Getenv("TEST_WORKSPACE"), "cam/private/extensions.bzl")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `sha256 = "`+ExpectedSHA256+`"`) {
		t.Errorf("cam/private/extensions.bzl does not pin %s", ExpectedSHA256)
	}
}

// The script checks the AppImage and the tarball, and uploads only when asked.
func TestScript(t *testing.T) {
	for _, want := range []string{
		"APPIMAGE_SHA256=f6dc6ba676e5ac96a565ebc8d657232f94c6158e85b4352141bd1a46f6b43434",
		`[ "$SHA" = "$EXPECTED_SHA256" ] || die`,
		"DRY_RUN=1",
		`[ "${1:-}" = "--upload" ] && DRY_RUN=0`,
		"tar --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner",
		"xz -T1 -9",
		"umask 077",
	} {
		if !strings.Contains(Script, want) {
			t.Errorf("prebuilt.sh lacks %q", want)
		}
	}
}

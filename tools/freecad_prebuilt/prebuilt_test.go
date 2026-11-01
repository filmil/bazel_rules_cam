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
	ext := string(b)
	if !strings.Contains(ext, `sha256 = "`+ExpectedSHA256+`"`) {
		t.Errorf("cam/private/extensions.bzl does not pin %s", ExpectedSHA256)
	}
	// The same version and tarball name as the script makes.
	for _, want := range []string{`"1.1.4": struct(`, "prebuilt-freecad-1.1.4/freecad-1.1.4-linux-x86_64.tar.xz", `strip_prefix = "freecad-1.1.4-linux-x86_64"`} {
		if !strings.Contains(ext, want) {
			t.Errorf("cam/private/extensions.bzl lacks %q", want)
		}
	}
	for _, want := range []string{"FREECAD_VERSION=1.1.4", `NAME="freecad-${FREECAD_VERSION}-linux-x86_64"`, `TAG="prebuilt-freecad-${FREECAD_VERSION}"`} {
		if !strings.Contains(Script, want) {
			t.Errorf("prebuilt.sh lacks %q", want)
		}
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

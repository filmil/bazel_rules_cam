// SPDX-License-Identifier: Apache-2.0

// Package prebuilt holds the script that makes the FreeCAD prebuilt tarball
// (prebuilt.sh, embedded) and the sha256 the module pins for it.
package prebuilt

import (
	_ "embed"
)

// Script makes the tarball from the official AppImage (prebuilt.sh).
//
//go:embed prebuilt.sh
var Script string

// ExpectedSHA256 is the sha256 of freecad-1.1.4-linux-x86_64.tar.xz that
// cam/private/extensions.bzl pins. The script stops if the tarball it makes
// differs; prebuilt_test checks this constant against extensions.bzl.
const ExpectedSHA256 = "85d7d93c84f6d08e799e1a890860c86ae5b2f4d527afed509aca196a50d33ba3"

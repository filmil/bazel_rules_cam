# SPDX-License-Identifier: Apache-2.0
"""Public API of hdlf_rules_cam.

Load every rule from this file:

```starlark
load("@hdlf_rules_cam//cam:defs.bzl", "determinism_test", "freecad_toolchain", "swap_test")
```

The rules arrive in milestone M5 (see the issues of
https://github.com/filmil/bazel_rules_cam). This file is the only public
entry point; everything under `//cam/private` may change without notice.
"""

load("//cam/private:determinism.bzl", _determinism_test = "determinism_test")
load(
    "//cam/private:freecad_toolchain.bzl",
    _FreecadToolchainInfo = "FreecadToolchainInfo",
    _freecad_toolchain = "freecad_toolchain",
)
load("//cam/private:swap_test.bzl", _swap_test = "swap_test")
load("//cam/private:toolchains.bzl", _tool_toolchain = "tool_toolchain")

determinism_test = _determinism_test
freecad_toolchain = _freecad_toolchain
FreecadToolchainInfo = _FreecadToolchainInfo
swap_test = _swap_test
tool_toolchain = _tool_toolchain

# The release this file belongs to. It lets a consumer check, in a test,
# that the module it resolved is the one it expects.
RULES_CAM_API_VERSION = "0"

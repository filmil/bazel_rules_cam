# SPDX-License-Identifier: Apache-2.0
"""Module extensions of hdlf_rules_cam.

```starlark
freecad = use_extension("@hdlf_rules_cam//cam:extensions.bzl", "freecad")
freecad.toolchain(version = "1.1.4")
```
"""

load("//cam/private:extensions.bzl", _freecad = "freecad")

freecad = _freecad

# SPDX-License-Identifier: Apache-2.0
"""Public API of rules_cam.

Load every rule from this file. For now it holds only the API version:

```starlark
load("@rules_cam//cam:defs.bzl", "RULES_CAM_API_VERSION")
```

The rules arrive in milestone M5 (see the issues of
https://github.com/filmil/bazel_rules_cam). This file is the only public
entry point; everything under `//cam/private` may change without notice.
"""

# The release this file belongs to. It lets a consumer check, in a test,
# that the module it resolved is the one it expects.
RULES_CAM_API_VERSION = "0"

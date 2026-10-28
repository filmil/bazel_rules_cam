<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Public API of rules_cam.

Load every rule from this file. For now it holds only the API version:

```starlark
load("@rules_cam//cam:defs.bzl", "RULES_CAM_API_VERSION")
```

The rules arrive in milestone M5 (see the issues of
https://github.com/filmil/bazel_rules_cam). This file is the only public
entry point; everything under `//cam/private` may change without notice.


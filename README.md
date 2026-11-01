<!-- SPDX-License-Identifier: Apache-2.0 -->
[![Test](https://github.com/filmil/bazel_rules_cam/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/filmil/bazel_rules_cam/actions/workflows/test.yml)
[![Tag and Release](https://github.com/filmil/bazel_rules_cam/actions/workflows/tag-and-release.yml/badge.svg)](https://github.com/filmil/bazel_rules_cam/releases)
[![Publish to my Bazel registry](https://github.com/filmil/bazel_rules_cam/actions/workflows/publish.yml/badge.svg)](https://github.com/filmil/bazel_rules_cam/actions/workflows/publish.yml)
[![Publish on Bazel Central Registry](https://github.com/filmil/bazel_rules_cam/actions/workflows/publish-bcr.yml/badge.svg)](https://github.com/filmil/bazel_rules_cam/actions/workflows/publish-bcr.yml)

# hdlf_rules_cam

Hermetic Bazel rules for machining with FreeCAD CAM.
The build turns a part's STEP file, a stock, a tool library and an
operations script into G-code for a named post-processor, and checks the
G-code against the machine's envelope and limits. It does not send G-code
to a machine.
Every tool comes from a Bazel toolchain, so nothing needs to be installed
on the machine, and two builds give byte-identical outputs.

Only Linux x86_64 hosts are supported.

## Using it

`MODULE.bazel`:

```starlark
bazel_dep(name = "hdlf_rules_cam", version = "0.0.1")
```

`.bazelrc`, while the module is only in the filmil registry:

```
common --registry=https://raw.githubusercontent.com/filmil/bazel-registry/main
common --registry=https://bcr.bazel.build
```

The module registers its FreeCAD toolchain and its G-code checker, so the
`MODULE.bazel` line is all the setup a user needs.

A job takes a part as a STEP file with one solid: one in the repository,
or the output of another rule, such as `cad_part` of
[hdlf_rules_brep](https://github.com/filmil/bazel_rules_brep). `BUILD.bazel`:

```starlark
load(
    "@hdlf_rules_cam//cam:defs.bzl",
    "cam_job",
    "determinism_test",
    "gcode_check_test",
    "machine",
    "tool_library",
)

# The cutters, checked: tools.json is below.
tool_library(
    name = "tools",
    src = "tools.json",
)

# The machine: travel in mm, limits, and the post-processor for its
# controller.
machine(
    name = "router",
    envelope = ["400", "300", "75"],
    max_feed = "5000",      # mm/min
    max_spindle = "24000",  # rpm
    post = "grbl",
)

# G-code for one part: bracket.ngc.
cam_job(
    name = "bracket",
    machine = ":router",
    ops = "bracket_ops.py",
    part = "bracket.step",
    # Margins around the part, mm: x and y on each side, z_top above,
    # z_bottom below; a missing key is 0.
    stock = {"x": "3", "y": "3", "z_top": "1"},
    tools = ":tools",
)

# The G-code against the machine's travel and limits, the stock, and the
# tool library.
gcode_check_test(
    name = "bracket_gcode_test",
    job = ":bracket",
)

# Two builds give the same G-code, byte for byte.
determinism_test(
    name = "bracket_determinism_test",
    target = ":bracket",
)
```

`tools.json`: each tool's number (the T word), a name, one of FreeCAD's tool
bit shapes, and that shape's dimensions in mm and degrees:

```json
{"tools": [
  {"number": 1, "name": "3 mm flat end mill", "shape": "endmill",
   "diameter": 3, "length": 38, "cutting_edge_height": 12, "flutes": 2},
  {"number": 2, "name": "5 mm drill", "shape": "drill",
   "diameter": 5, "length": 50, "tip_angle": 118}
]}
```

`bracket_ops.py`, the operations script: it defines
`operations(job, model, tools)`, adds FreeCAD CAM operations to the job, and
sets the feeds and spindle speed of each tool it uses.

```python
import Path.Op.PocketShape as PocketShape
import Path.Op.Profile as Profile


def operations(job, model, tools):
    t1 = tools[1]
    t1.HorizFeed = "600 mm/min"  # always with units: a plain number is mm/s
    t1.VertFeed = "200 mm/min"
    t1.SpindleSpeed = 12000

    # The pocket's floor: the flat face at Z 6.
    floor = [
        "Face%d" % (i + 1)
        for i, f in enumerate(model.Shape.Faces)
        if f.BoundBox.ZLength < 1e-6 and abs(f.BoundBox.ZMin - 6) < 1e-6
    ]
    pocket = PocketShape.Create("Pocket", parentJob=job)
    pocket.ToolController = t1
    pocket.Base = [(model, floor)]

    profile = Profile.Create("Profile", parentJob=job)
    profile.ToolController = t1
```

`bazel build //:bracket` writes `bazel-bin/bracket.ngc`;
`bazel test //:bracket_gcode_test` prints the check's report into the test
log (`bazel-testlogs/bracket_gcode_test/test.log`), and keeps it as
`gcode_check_report.txt` in the test's outputs
(`bazel-testlogs/bracket_gcode_test/test.outputs/`).

The build stops, with a message that names the problem, when:

* the tool library has a mistake, such as a duplicate number or a cutting
  edge longer than the tool, or the machine has a bad value;
* the part has more than one solid;
* the script uses Adaptive clearing, or an operation fails or makes no
  toolpath;
* a tool's feed or spindle speed is missing, or above the machine's limit.

`cam_job` checks the feeds and spindle speeds it sets against the machine
before it posts. `gcode_check_test` then checks the G-code itself, as the
post wrote it, and can check it against another machine too: every move
within the machine's travel, no rapid move into the stock, feeds and
spindle speeds within limits, and every tool in the library.

Every rule is in [docs/defs.md](docs/defs.md), and the module extension in
[docs/extensions.md](docs/extensions.md). No machine runs the G-code until a
person has checked it.

### FreeCAD

The module registers a FreeCAD 1.1.4 toolchain, so a user installs
nothing. Its tree is the official AppImage, unpacked without running it,
then pruned to `freecadcmd` and the CAM workbench (169 MB download,
742 MB unpacked). It runs with no display and loads no host library but
glibc. Every action gives FreeCAD a private home, and drops the host's
`PYTHON*`, `QT_*`, `LD_*`, `FREECAD*`, `FONTCONFIG_*` and `FC_*`
variables. FreeCAD's Python
ignores `PYTHON*` variables anyway, so scripts run through a bootstrap that
turns off `.pyc` writing.

Two traps for operations scripts:

* **Adaptive clearing is not deterministic.** FreeCAD's adaptive op
  (`libarea/Adaptive.cpp`) stops its search on a CPU-time budget and draws
  angles from `rand()`, so its toolpath changes from run to run. The rules
  do not offer it.
* **Feeds need units.** A plain number given to `HorizFeed` or `VertFeed`
  is read as mm/s, so `600` becomes `F36000`. Write `"600 mm/min"`.

## Building

```sh
bazel build //...
bazel test //...
cd integration && bazel test //...
```

After changing a `.bzl` file, regenerate the documentation with
`bazel run //docs:update_docs`.
Format with `bazel run //tools:buildifier`.

## Releasing

The operator runs the `Tag and Release` workflow
(`.github/workflows/tag-and-release.yml`, `workflow_dispatch`).
It picks the next version from the conventional commits since the last
tag, makes the tag, the release and its source archive, and opens one
pull request on the filmil registry and one on the Bazel Central Registry.
It needs the repository secret `BCR_PUBLISH_TOKEN`.

None of the modules published here are published with any sort of guarantee
whatsoever. Use at your own risk.

See the overall description at the [project page][pp].

[pp]: https://hdlfactory.com/post/2025/09/29/getting-ready-for-the-brave-new-bazel-modules-world/

## Licenses

This repository is under the Apache License 2.0 (`LICENSE`).
The tools and libraries that the build downloads are listed here, with their
versions, sources and licenses. FreeCAD's tree includes OpenCASCADE
(LGPL-2.1 with an exception), opencamlib (LGPL-2.1-or-later), numpy
(BSD-3-Clause) and Qt 6 (LGPL-3.0). Its `packages.txt`, at the top of the
tree (`freecad/packages.txt` in the `freecad_toolchains` repository that the
module extension makes), lists every package in it.

| What | Version | Source | License |
|---|---|---|---|
| Go SDK (builds the helpers: the launcher, the tool library's checker, the G-code checker) | as rules_go 0.64.2 picks it | [go.dev/dl](https://go.dev/dl/), fetched by rules_go | BSD-3-Clause |
| FreeCAD, pruned | 1.1.4 | [FreeCAD_1.1.4-Linux-x86_64-py311.AppImage](https://github.com/FreeCAD/FreeCAD/releases/tag/1.1.4), sha256 `f6dc6ba6…43434`; mirrored as the release asset [prebuilt-freecad-1.1.4](https://github.com/filmil/bazel_rules_cam/releases/tag/prebuilt-freecad-1.1.4) | LGPL-2.0-or-later; its bundled libraries under their own licenses, listed in `packages.txt` (above) |

<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Public API of hdlf_rules_cam.

Load every rule from this file:

```starlark
load(
    "@hdlf_rules_cam//cam:defs.bzl",
    "determinism_test",
    "freecad_toolchain",
    "machine",
    "swap_test",
    "tool_library",
    "tool_toolchain",
)
```

The rules arrive in milestone M5 (see the issues of
https://github.com/filmil/bazel_rules_cam). This file is the only public
entry point; everything under `//cam/private` may change without notice.

<a id="freecad_toolchain"></a>

## freecad_toolchain

<pre>
load("@hdlf_rules_cam//cam:defs.bzl", "freecad_toolchain")

freecad_toolchain(<a href="#freecad_toolchain-name">name</a>, <a href="#freecad_toolchain-data">data</a>, <a href="#freecad_toolchain-freecadcmd">freecadcmd</a>, <a href="#freecad_toolchain-posts">posts</a>, <a href="#freecad_toolchain-version">version</a>)
</pre>

A FreeCAD toolchain, for the toolchain type `@hdlf_rules_cam//cam:toolchain_type`.

The `freecad` module extension makes one from the prebuilt FreeCAD. A user
who builds or fetches FreeCAD another way declares their own:

```starlark
load("@hdlf_rules_cam//cam:defs.bzl", "freecad_toolchain")

freecad_toolchain(
    name = "my_freecad_impl",
    data = ["@my_freecad//:all_files"],
    freecadcmd = "@my_freecad//:usr/bin/freecadcmd",
    posts = ["grbl", "linuxcnc"],
    version = "1.1.4",
)

toolchain(
    name = "my_freecad",
    exec_compatible_with = ["@platforms//os:linux", "@platforms//cpu:x86_64"],
    target_settings = ["@hdlf_rules_cam//cam/toolchains:stub_off"],
    toolchain = ":my_freecad_impl",
    toolchain_type = "@hdlf_rules_cam//cam:toolchain_type",
)
```

Every action runs freecadcmd through `run_tool`, which gives it a private
home: `HOME`, the XDG directories and `FREECAD_USER_HOME`,
`FREECAD_USER_DATA` and `FREECAD_USER_TEMP` all point inside the action
(spec H3). The launcher drops the host's `PYTHON*`, `QT_*`, `LD_*`,
`FREECAD*`, `FONTCONFIG_*` and `FC_*` variables, so FreeCAD's own Python,
libraries and fonts are the only ones it loads.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="freecad_toolchain-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="freecad_toolchain-data"></a>data |  The rest of the FreeCAD tree: libraries, Python, workbenches.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="freecad_toolchain-freecadcmd"></a>freecadcmd |  The freecadcmd executable.   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="freecad_toolchain-posts"></a>posts |  The post-processors the tree has, by name.   | List of strings | required |  |
| <a id="freecad_toolchain-version"></a>version |  The FreeCAD version freecadcmd reports.   | String | required |  |


<a id="machine"></a>

## machine

<pre>
load("@hdlf_rules_cam//cam:defs.bzl", "machine")

machine(<a href="#machine-name">name</a>, <a href="#machine-envelope">envelope</a>, <a href="#machine-max_feed">max_feed</a>, <a href="#machine-max_spindle">max_spindle</a>, <a href="#machine-post">post</a>)
</pre>

The machine that runs a job's G-code: its travel, its limits, its controller.

cam_job writes G-code for the machine's post-processor, and
gcode_check_test checks the G-code against its envelope and limits.
Numbers are strings, as Starlark has no float attributes; each is
checked when the build reads it.

```starlark
machine(
    name = "shapeoko",
    envelope = ["400", "400", "75"],  # X, Y, Z travel in mm
    max_feed = "5000",                # mm/min
    max_spindle = "24000",            # rpm
    post = "grbl",
)
```

`post` names one of the FreeCAD toolchain's post-processors
(`FreecadToolchainInfo.posts`; the prebuilt has `grbl`, `linuxcnc`,
`mach3_mach4` and 30 more).

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="machine-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="machine-envelope"></a>envelope |  The X, Y and Z travel, in mm.   | List of strings | required |  |
| <a id="machine-max_feed"></a>max_feed |  The highest feed rate, in mm/min.   | String | required |  |
| <a id="machine-max_spindle"></a>max_spindle |  The highest spindle speed, in rpm: a whole number, 1 to 1000000.   | String | required |  |
| <a id="machine-post"></a>post |  The FreeCAD post-processor for the machine's controller.   | String | required |  |


<a id="tool_library"></a>

## tool_library

<pre>
load("@hdlf_rules_cam//cam:defs.bzl", "tool_library")

tool_library(<a href="#tool_library-name">name</a>, <a href="#tool_library-src">src</a>)
</pre>

The cutters a CAM job may use, checked.

`src` is JSON: a list of tools, each with its number (the T word in the
G-code), a name, one of FreeCAD's tool bit shapes, and that shape's
dimensions, in mm and degrees:

```json
{"tools": [
  {"number": 1, "name": "3 mm flat end mill", "shape": "endmill",
   "diameter": 3, "length": 38, "cutting_edge_height": 12, "flutes": 2},
  {"number": 2, "name": "5 mm drill", "shape": "drill",
   "diameter": 5, "length": 50, "tip_angle": 118}
]}
```

| Shape | Needs, beyond `diameter` and `length` |
|---|---|
| `endmill`, `ballend` | `cutting_edge_height` |
| `bullnose` | `cutting_edge_height`, `corner_radius` |
| `drill` | `tip_angle` |
| `vbit`, `chamfer` | `cutting_edge_angle`, `cutting_edge_height`, `tip_diameter` |

Every shape may also have `shank_diameter` and `flutes`. The build fails,
naming each problem, on:

* a file that is not one JSON object, an unknown key, an empty list or a
  null tool;
* a tool number outside 1 to 999, or one used twice; a name used twice, or
  with spaces around it;
* a dimension the shape lacks, or one it does not use;
* a dimension that is not above 0 (`tip_diameter` may be 0), `flutes`
  outside 1 to 16, or an angle of 180 degrees or more;
* dimensions that cannot fit together: a cutting edge longer than the
  tool, a `corner_radius` of half the diameter or more, a `tip_diameter` of
  the diameter or more, or a cutting edge shorter than the ball of a
  `ballend` (half the diameter) or the corner of a `bullnose`
  (`corner_radius`).

`corner_radius` is the radius of a bullnose's rounded corner, as FreeCAD's
bullnose shape takes it (its `CornerRadius`).

The output is the library in a canonical form: sorted by number, each
number written one way (`3`, `3.0` and `3.00` all give `3`).

```starlark
tool_library(
    name = "tools",
    src = "tools.json",
)
```

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="tool_library-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="tool_library-src"></a>src |  The tool library, JSON.   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |


<a id="tool_toolchain"></a>

## tool_toolchain

<pre>
load("@hdlf_rules_cam//cam:defs.bzl", "tool_toolchain")

tool_toolchain(<a href="#tool_toolchain-name">name</a>, <a href="#tool_toolchain-tools">tools</a>)
</pre>

A toolchain that offers tools by name, for `run_tool`.

Example, a toolchain for a tool of your own:

```starlark
tool_toolchain(
    name = "my_tool_impl",
    tools = {"//third_party/my_tool": "my_tool"},
)

toolchain(
    name = "my_tool",
    toolchain = ":my_tool_impl",
    toolchain_type = "//my:toolchain_type",
)
```

For FreeCAD, use `freecad_toolchain`: the rules that run freecadcmd also
read its `FreecadToolchainInfo`.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="tool_toolchain-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="tool_toolchain-tools"></a>tools |  Executable targets, each mapped to the name rules use for it.   | <a href="https://bazel.build/rules/lib/core/dict">Dictionary: Label -> String</a> | required |  |


<a id="FreecadToolchainInfo"></a>

## FreecadToolchainInfo

<pre>
load("@hdlf_rules_cam//cam:defs.bzl", "FreecadToolchainInfo")

FreecadToolchainInfo(<a href="#FreecadToolchainInfo-version">version</a>, <a href="#FreecadToolchainInfo-posts">posts</a>)
</pre>

What a FreeCAD toolchain offers beyond the `freecadcmd` tool.

**FIELDS**

| Name  | Description |
| :------------- | :------------- |
| <a id="FreecadToolchainInfo-version"></a>version |  The FreeCAD version freecadcmd reports, such as "1.1.4".    |
| <a id="FreecadToolchainInfo-posts"></a>posts |  Sorted list of the post-processors in the tree, by the name FreeCAD's PostProcessorFactory takes ("linuxcnc" for `linuxcnc_post.py`).    |


<a id="MachineInfo"></a>

## MachineInfo

<pre>
load("@hdlf_rules_cam//cam:defs.bzl", "MachineInfo")

MachineInfo(<a href="#MachineInfo-envelope">envelope</a>, <a href="#MachineInfo-max_feed">max_feed</a>, <a href="#MachineInfo-max_spindle">max_spindle</a>, <a href="#MachineInfo-post">post</a>)
</pre>

A machine's working envelope, its limits and its post-processor.

**FIELDS**

| Name  | Description |
| :------------- | :------------- |
| <a id="MachineInfo-envelope"></a>envelope |  struct(x, y, z): the travel of each axis, in mm, as floats.    |
| <a id="MachineInfo-max_feed"></a>max_feed |  The highest feed rate, in mm/min, as a float.    |
| <a id="MachineInfo-max_spindle"></a>max_spindle |  The highest spindle speed, in rpm, as an int.    |
| <a id="MachineInfo-post"></a>post |  The FreeCAD post-processor for its controller, such as "grbl".    |


<a id="ToolLibraryInfo"></a>

## ToolLibraryInfo

<pre>
load("@hdlf_rules_cam//cam:defs.bzl", "ToolLibraryInfo")

ToolLibraryInfo(<a href="#ToolLibraryInfo-file">file</a>)
</pre>

A checked tool library.

**FIELDS**

| Name  | Description |
| :------------- | :------------- |
| <a id="ToolLibraryInfo-file"></a>file |  File: the library in canonical JSON, sorted by tool number. cam_job reads this one.    |


<a id="determinism_test"></a>

## determinism_test

<pre>
load("@hdlf_rules_cam//cam:defs.bzl", "determinism_test")

determinism_test(<a href="#determinism_test-name">name</a>, <a href="#determinism_test-target">target</a>, <a href="#determinism_test-kwargs">**kwargs</a>)
</pre>

Checks that two builds of `target` give byte-identical outputs.

The target is built twice in one build, under two configurations that
differ only in a setting no rule reads, so every action runs twice.
The test compares the SHA-256 of every output file, directories walked,
and fails on any difference (spec D1).

Two runs in the same second can hide a time stamp with a one-second
resolution, so pair this with a header scan where a format has one. CI
also builds every determinism_test target in two fresh output bases and
compares the results.

Example:

```starlark
determinism_test(
    name = "pocket_gcode_determinism_test",
    target = ":pocket_gcode",
)
```


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="determinism_test-name"></a>name |  the test's name.   |  none |
| <a id="determinism_test-target"></a>target |  the target whose outputs must not change between builds.   |  none |
| <a id="determinism_test-kwargs"></a>kwargs |  common test attributes, such as `size` and `tags`.   |  none |


<a id="swap_test"></a>

## swap_test

<pre>
load("@hdlf_rules_cam//cam:defs.bzl", "swap_test")

swap_test(<a href="#swap_test-name">name</a>, <a href="#swap_test-targets">targets</a>, <a href="#swap_test-kwargs">**kwargs</a>)
</pre>

Builds targets with every tool swapped for a stub, and checks them.

The targets are built with `--@hdlf_rules_cam//cam/toolchains:use_stub`,
so each tool toolchain that honors the flag resolves to the stub tool.
The stub writes a record into every output, files and directories
alike. The test fails if any output file of the targets lacks the
record: some action made it without a toolchain tool.

Example:

```starlark
swap_test(
    name = "swap_test",
    targets = [":gerbers", ":bom"],
)
```


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="swap_test-name"></a>name |  the test's name.   |  none |
| <a id="swap_test-targets"></a>targets |  targets whose outputs must all come from the stub.   |  none |
| <a id="swap_test-kwargs"></a>kwargs |  common test attributes, such as `size` and `tags`.   |  none |



# SPDX-License-Identifier: Apache-2.0
"""tool_library: the cutters a CAM job may use, checked (issue #8)."""

load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE", "RUN_TOOL_TOOLCHAINS", "run_tool")

ToolLibraryInfo = provider(
    doc = "A checked tool library.",
    fields = {
        "file": "File: the library in canonical JSON, sorted by tool number. cam_job reads this one.",
    },
)

def _tool_library_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".tools.json")
    args = ctx.actions.args()
    args.add("--in", ctx.file.src)
    args.add("--out", out)
    run_tool(
        ctx,
        HELPERS_TOOLCHAIN_TYPE,
        "toolcheck",
        arguments = [args],
        inputs = [ctx.file.src],
        outputs = [out],
        mnemonic = "CamToolLibrary",
        progress_message = "Checking the tool library %{label}",
    )
    return [
        DefaultInfo(files = depset([out])),
        ToolLibraryInfo(file = out),
    ]

tool_library = rule(
    implementation = _tool_library_impl,
    doc = """The cutters a CAM job may use, checked.

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

Every shape may also have `shank_diameter` and `flutes`, and every shape
but `drill` a `max_stepdown`: the deepest cut the tool may take in one
pass, for `gcode_check_test`'s `depth` check (without it, the tool's
diameter). It is not more than `cutting_edge_height`, and FreeCAD does not
read it. A drill plunges by design, and the check skips it. The
build fails,
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
""",
    attrs = {
        "src": attr.label(
            doc = "The tool library, JSON.",
            mandatory = True,
            allow_single_file = [".json"],
        ),
    },
    toolchains = RUN_TOOL_TOOLCHAINS,
)

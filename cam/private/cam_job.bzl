# SPDX-License-Identifier: Apache-2.0
"""cam_job: G-code for one part, from FreeCAD's CAM (issue #9)."""

load(":freecad_toolchain.bzl", "FREECAD_TOOLCHAIN_TYPE", "freecad_script_args")
load(":machine.bzl", "MachineInfo")
load(":numbers.bzl", "parse_number")
load(":run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")
load(":tool_library.bzl", "ToolLibraryInfo")

_STOCK_KEYS = ["x", "y", "z_top", "z_bottom"]

def _stock(label, stock):
    for k in stock:
        if k not in _STOCK_KEYS:
            fail("cam_job %s: stock key %r: want %s" % (label, k, ", ".join(_STOCK_KEYS)))
    out = {}
    for k in _STOCK_KEYS:
        text = stock.get(k, "0")
        v = parse_number(text)
        if v == None or v != v or v < 0:
            fail("cam_job %s: stock %s %r: want a number of mm, 0 or more" % (label, k, text))
        out[k] = v
    return out

CamJobInfo = provider(
    doc = "A cam_job's G-code and what it was made for.",
    fields = {
        "gcode": "File: the G-code.",
        "info": "File: JSON with the job's stock box, in the G-code's coordinates.",
        "machine": "The MachineInfo of the job's machine.",
        "tools": "File: the canonical tool library.",
    },
)

def cam_job_config(ctx, out, path = None, info = None):
    """The runner's configuration, as a dict.

    Args:
      ctx: a cam_job (or its failure test's) rule context.
      out: the G-code path to write, a string.
      path: a function from `File` to the path the runner sees; the
        execution root path when None.
      info: the path to write the job's facts to, a string; next to `out`
        when None.

    Returns:
      The dict to write as JSON.
    """
    p = path or (lambda f: f.path)
    m = ctx.attr.machine[MachineInfo]
    return {
        "info": info or (out + ".json"),
        "label": str(ctx.label),
        "machine": {
            "max_feed": m.max_feed,
            "max_spindle": m.max_spindle,
            "post": m.post,
        },
        "ops": p(ctx.file.ops),
        "out": out,
        "part": p(ctx.file.part),
        "stock": _stock(str(ctx.label), ctx.attr.stock),
        "tools": p(ctx.attr.tools[ToolLibraryInfo].file),
    }

def _cam_job_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".ngc")
    info = ctx.actions.declare_file(ctx.label.name + ".ngc.json")
    config = ctx.actions.declare_file(ctx.label.name + ".cam_job.json")
    ctx.actions.write(config, json.encode_indent(cam_job_config(ctx, out.path, info = info.path)) + "\n")
    lib = ctx.attr.tools[ToolLibraryInfo].file
    run_tool(
        ctx,
        FREECAD_TOOLCHAIN_TYPE,
        "freecadcmd",
        arguments = freecad_script_args(ctx.file._runner),
        inputs = [config, ctx.file.part, ctx.file.ops, lib, ctx.file._runner],
        outputs = [out, info],
        path_env = {"CAM_JOB_CONFIG": config},
        mnemonic = "CamJob",
        progress_message = "Making G-code for %{label}",
    )
    return [
        DefaultInfo(files = depset([out])),
        CamJobInfo(
            gcode = out,
            info = info,
            machine = ctx.attr.machine[MachineInfo],
            tools = lib,
        ),
    ]

CAM_JOB_ATTRS = {
    "part": attr.label(
        doc = "The part, a STEP file with one solid.",
        mandatory = True,
        allow_single_file = [".step", ".stp"],
    ),
    "stock": attr.string_dict(
        doc = "The stock, as margins around the part's bounding box, in mm: " +
              "`x` and `y` on each side, `z_top` above and `z_bottom` below. " +
              "A missing key is 0.",
    ),
    "tools": attr.label(
        doc = "The tool library (`tool_library`).",
        mandatory = True,
        providers = [ToolLibraryInfo],
    ),
    "ops": attr.label(
        doc = "The operations script: Python that defines `operations(job, model, tools)`.",
        mandatory = True,
        allow_single_file = [".py"],
    ),
    "machine": attr.label(
        doc = "The machine (`machine`): its post-processor and limits.",
        mandatory = True,
        providers = [MachineInfo],
    ),
    "_runner": attr.label(
        default = "//cam/private/runner:runner.py",
        allow_single_file = True,
    ),
}

cam_job = rule(
    implementation = _cam_job_impl,
    doc = """G-code for one part, from FreeCAD's CAM: `<name>.ngc`.

The job takes the part from a STEP file, puts the stock around it, gives
it the library's tools, and runs the operations script, which adds the
operations. Then it checks the result and posts it with the machine's
post-processor.

```starlark
cam_job(
    name = "bracket",
    machine = ":router",
    ops = "bracket_ops.py",
    part = ":bracket_step",
    stock = {"x": "3", "y": "3", "z_top": "1"},
    tools = ":tools",
)
```

The operations script defines `operations(job, model, tools)`. `model` is
the job's copy of the part; `tools` maps each tool number of the library
to its FreeCAD tool controller. The script adds FreeCAD CAM operations
(`Path.Op.*`) to the job, sets each one's `ToolController` (a new
operation gets the lowest-numbered tool), and sets the feeds and spindle
speed of each tool it uses. Give feeds with units: a plain number is mm/s.
A dressup that reads its operation's toolpath (holding tags, say) needs
`job.Document.recompute()` first.

Depth properties (`StepDown`, `StartDepth`, `FinalDepth`) follow
expressions: `StepDown` the tool's diameter, the depths the base
geometry's heights. A plain assignment loses to the expression on the next
recompute, silently. Clear it first:

```python
op.setExpression("StepDown", None)
op.StepDown = "0.5 mm"
```

A profile based on a face also cuts from that face down: one based on a
part's bottom face cuts it in a single full-depth pass. Base a profile
through the part on its top face. `gcode_check_test`
catches a pass deeper than the tool takes (its `depth` check).

```python
import Path.Op.PocketShape as PocketShape
import Path.Op.Profile as Profile

def operations(job, model, tools):
    t1 = tools[1]
    t1.HorizFeed = "600 mm/min"
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

The build fails, naming the problem, when:

* the part has more than one solid, or none;
* the script defines no `operations`, raises (the message names the
  exception, after its traceback), adds no operation, or turns every one
  off;
* an operation is Adaptive, even inside a dressup: its toolpath changes
  from run to run, because FreeCAD stops its search on a CPU-time budget;
* an active operation fails, makes no toolpath, or uses a tool that is not
  in the library (a dressup uses its operation's tool);
* a tool that an active operation uses has no feed or spindle speed, or
  one above the machine's `max_feed` or `max_spindle`.

The G-code has no header (it would hold the date and a path), and a line
that is only a comment and holds a date or a time (2026-10-31,
31/10/2026, Oct 31 2026, 12:34 or 12:34:56) is dropped, so two builds give
the same bytes. The job does not check the G-code against the machine's envelope:
`gcode_check_test` does.

A person checks G-code before a machine runs it (rule 7 of this module's
AGENTS.md).
""",
    attrs = CAM_JOB_ATTRS,
    toolchains = [FREECAD_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)

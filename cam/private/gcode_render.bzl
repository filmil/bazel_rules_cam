# SPDX-License-Identifier: Apache-2.0
"""gcode_render: a job's toolpath as pictures and a report (issue #35)."""

load(":cam_job.bzl", "CamJobInfo")
load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE", "RUN_TOOL_TOOLCHAINS", "run_tool")

def _gcode_render_impl(ctx):
    job = ctx.attr.job[CamJobInfo]
    m = job.machine
    machine = ctx.actions.declare_file(ctx.label.name + ".machine.json")
    ctx.actions.write(machine, json.encode_indent({
        "envelope": [m.envelope.x, m.envelope.y, m.envelope.z],
        "max_feed": m.max_feed,
        "max_spindle": m.max_spindle,
    }) + "\n")
    top = ctx.actions.declare_file(ctx.label.name + "_top.png")
    iso = ctx.actions.declare_file(ctx.label.name + "_iso.png")
    report = ctx.actions.declare_file(ctx.label.name + ".md")
    args = ctx.actions.args()
    args.add("--gcode", job.gcode)
    args.add("--tools", job.tools)
    args.add("--info", job.info)
    args.add("--machine", machine)
    label = ctx.attr.job.label
    args.add("--name", "//%s:%s" % (label.package, label.name))
    args.add("--top", top)
    args.add("--iso", iso)
    args.add("--report", report)
    run_tool(
        ctx,
        HELPERS_TOOLCHAIN_TYPE,
        "gcoderender",
        arguments = [args],
        inputs = [job.gcode, job.tools, job.info, machine],
        outputs = [top, iso, report],
        mnemonic = "CamGcodeRender",
        progress_message = "Drawing the toolpath of %{label}",
    )
    return [DefaultInfo(files = depset([top, iso, report]))]

gcode_render = rule(
    implementation = _gcode_render_impl,
    doc = """Draws a job's toolpath, and writes a report of it.

```starlark
gcode_render(
    name = "bracket_toolpath",
    job = ":bracket",
)
```

The outputs:

* `<name>_top.png`: the toolpath seen from above;
* `<name>_iso.png`: the same in an isometric view from above, past the
  origin's corner (X runs to the right and up the picture, Y to the left
  and up, Z up);
* `<name>.md`: per operation, its tools, the Z levels it cuts at, its
  deepest Z, its cut and rapid lengths, and an estimated time from the
  feeds (rapids at the machine's `max_feed`, no acceleration).

In the pictures, cutting moves are colored by depth: light blue at the
stock's top, through blue and purple, to dark red at the deepest cut.
Rapids are dashed gray, and the stock is a gray box. The pictures hold no
text and no metadata, so the same job gives the same bytes
(`determinism_test`). The legend is in the report.

The G-code is read the way `gcode_check_test` reads it, so the pictures
show the moves the check vouches for. The pictures and the report are what
a pull request that changes a job attaches (rule 5 of AGENTS.md).
""",
    attrs = {
        "job": attr.label(
            doc = "The job (`cam_job`).",
            mandatory = True,
            providers = [CamJobInfo],
        ),
    },
    toolchains = RUN_TOOL_TOOLCHAINS,
)

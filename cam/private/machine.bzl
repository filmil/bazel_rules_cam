# SPDX-License-Identifier: Apache-2.0
"""machine: the limits of the machine that runs a job's G-code (issue #8)."""

load(":freecad_toolchain.bzl", "FREECAD_TOOLCHAIN_TYPE")
load(":numbers.bzl", "parse_number")

MachineInfo = provider(
    doc = "A machine's working envelope, its limits and its post-processor.",
    fields = {
        "envelope": "struct(x, y, z): the travel of each axis, in mm, as floats.",
        "max_feed": "The highest feed rate, in mm/min, as a float.",
        "max_spindle": "The highest spindle speed, in rpm, as an int.",
        "post": "The FreeCAD post-processor for its controller, such as \"grbl\".",
    },
)

def _positive(name, attr, text):
    v = parse_number(text)
    if v == None or v != v or not v > 0:
        fail("machine %s: %s %r: want a number above 0" % (name, attr, text))
    return v

def _machine_impl(ctx):
    name = str(ctx.label)
    if len(ctx.attr.envelope) != 3:
        fail("machine %s: envelope has %d values; want 3, the X, Y and Z travel in mm" % (name, len(ctx.attr.envelope)))
    x, y, z = [_positive(name, "envelope", t) for t in ctx.attr.envelope]
    feed = _positive(name, "max_feed", ctx.attr.max_feed)
    spindle = parse_number(ctx.attr.max_spindle)
    if spindle == None or spindle != int(spindle) or spindle < 1 or spindle > 1000000:
        fail("machine %s: max_spindle %r: want a whole number of rpm, 1 to 1000000" % (name, ctx.attr.max_spindle))
    posts = ctx.toolchains[FREECAD_TOOLCHAIN_TYPE].freecad.posts
    if ctx.attr.post not in posts:
        fail("machine %s: post %r: the FreeCAD toolchain has %s" % (name, ctx.attr.post, ", ".join(posts)))
    return [MachineInfo(
        envelope = struct(x = x, y = y, z = z),
        max_feed = feed,
        max_spindle = int(spindle),
        post = ctx.attr.post,
    )]

machine = rule(
    implementation = _machine_impl,
    doc = """The machine that runs a job's G-code: its travel, its limits, its controller.

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
""",
    attrs = {
        "envelope": attr.string_list(
            doc = "The X, Y and Z travel, in mm.",
            mandatory = True,
        ),
        "max_feed": attr.string(
            doc = "The highest feed rate, in mm/min.",
            mandatory = True,
        ),
        "max_spindle": attr.string(
            doc = "The highest spindle speed, in rpm: a whole number, 1 to 1000000.",
            mandatory = True,
        ),
        "post": attr.string(
            doc = "The FreeCAD post-processor for the machine's controller.",
            mandatory = True,
        ),
    },
    toolchains = [FREECAD_TOOLCHAIN_TYPE],
)

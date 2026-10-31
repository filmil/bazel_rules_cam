# SPDX-License-Identifier: Apache-2.0
"""Writes what the registered FreeCAD toolchain says about itself."""

def _freecad_version_impl(ctx):
    info = ctx.toolchains["@hdlf_rules_cam//cam:toolchain_type"].freecad
    out = ctx.actions.declare_file(ctx.label.name + ".txt")
    has = [p for p in ["grbl", "linuxcnc"] if p in info.posts]
    ctx.actions.write(out, "FreeCAD %s, posts include %s\n" % (info.version, " ".join(has)))
    return [DefaultInfo(files = depset([out]))]

freecad_version = rule(
    implementation = _freecad_version_impl,
    toolchains = ["@hdlf_rules_cam//cam:toolchain_type"],
)

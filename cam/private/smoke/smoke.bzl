# SPDX-License-Identifier: Apache-2.0
"""freecad_smoke: runs FreeCAD's CAM once, to prove the FreeCAD toolchain works.

It lives under private/ because it calls run_tool. The real rules
(cam_job) arrive with their own issues.
"""

load("//cam/private:freecad_toolchain.bzl", "freecad_script_args")
load("//cam/private:run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_FREECAD = Label("//cam:toolchain_type")

def _freecad_smoke_impl(ctx):
    info = ctx.toolchains[_FREECAD].freecad
    if ctx.attr.post not in info.posts:
        fail("post %r: the FreeCAD %s toolchain has %s" % (ctx.attr.post, info.version, info.posts))
    out = ctx.actions.declare_file(ctx.label.name + ".ngc")
    run_tool(
        ctx,
        _FREECAD,
        "freecadcmd",
        arguments = freecad_script_args(ctx.file.script),
        inputs = [ctx.file.script],
        outputs = [out],
        env = {
            "SMOKE_POST": ctx.attr.post,
            "SMOKE_VERSION": info.version,
        },
        path_env = {"SMOKE_OUT": out},
        mnemonic = "FreecadSmoke",
        progress_message = "Running FreeCAD CAM for %{label}",
    )
    return [DefaultInfo(files = depset([out]))]

freecad_smoke = rule(
    implementation = _freecad_smoke_impl,
    doc = "G-code for a fixed test part, made by freecadcmd from the FreeCAD toolchain.",
    attrs = {
        "script": attr.label(mandatory = True, allow_single_file = [".py"]),
        "post": attr.string(default = "linuxcnc"),
    },
    toolchains = [_FREECAD] + RUN_TOOL_TOOLCHAINS,
)

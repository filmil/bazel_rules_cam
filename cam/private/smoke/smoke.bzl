# SPDX-License-Identifier: Apache-2.0
"""freecad_smoke: runs FreeCAD's CAM once, to prove the FreeCAD toolchain works.

It lives under private/ because it calls run_tool. The real rules
(cam_job) arrive with their own issues.
"""

load("//cam/private:freecad_toolchain.bzl", "FREECAD_TOOLCHAIN_TYPE", "freecad_script_args")
load("//cam/private:run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_FREECAD = FREECAD_TOOLCHAIN_TYPE

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

def _runfiles_path(ctx, f):
    if f.short_path.startswith("../"):
        return f.short_path[3:]
    return ctx.workspace_name + "/" + f.short_path

def _exit_status_test_impl(ctx):
    tc = ctx.toolchains[_FREECAD]
    cmd = tc.tools["freecadcmd"]
    cases = []
    for script, want in ctx.attr.cases.items():
        f = script.files.to_list()
        if len(f) != 1:
            fail("cases: %s is not one file" % script.label)
        cases.append({
            "args": freecad_script_args(f[0], path = f[0].short_path),
            "script": f[0].short_path,
            "want": int(want),
        })
    exe = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.symlink(output = exe, target_file = ctx.executable.checker, is_executable = True)
    runfiles = ctx.runfiles(
        files = ctx.files.cases + [cmd.executable],
        transitive_files = tc.tool_inputs,
    ).merge(ctx.attr.checker[DefaultInfo].default_runfiles).merge(cmd.runfiles if hasattr(cmd, "runfiles") else ctx.runfiles())
    return [
        DefaultInfo(executable = exe, runfiles = runfiles),
        RunEnvironmentInfo(environment = {
            "EXITCHECK_CASES": json.encode(cases),
            "EXITCHECK_TOOL": _runfiles_path(ctx, cmd.executable),
        }),
    ]

freecad_exit_status_test = rule(
    implementation = _exit_status_test_impl,
    test = True,
    doc = """Runs scripts in freecadcmd through freecad_script_args, and checks each exit status.

freecadcmd itself exits 0 when a script raises; the bootstrap must turn
that into a failure, or an action whose script fails would pass.
""",
    attrs = {
        "cases": attr.label_keyed_string_dict(
            doc = "Each script, and the exit status it must give.",
            allow_files = [".py"],
            mandatory = True,
        ),
        "checker": attr.label(
            doc = "The exitcheck binary.",
            executable = True,
            cfg = "target",
            mandatory = True,
        ),
    },
    toolchains = [_FREECAD],
)

# SPDX-License-Identifier: Apache-2.0
"""Test helpers for cam_job: test parts, and jobs the runner must refuse.

They live under private/ because they call run_tool, and run freecadcmd
from the FreeCAD toolchain.
"""

load("//cam/private:cam_job.bzl", "CAM_JOB_ATTRS", "cam_job_config")
load("//cam/private:freecad_toolchain.bzl", "FREECAD_TOOLCHAIN_TYPE", "freecad_script_args")
load("//cam/private:run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")
load("//cam/private:tool_library.bzl", "ToolLibraryInfo")

def _freecad_step_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".step")
    run_tool(
        ctx,
        FREECAD_TOOLCHAIN_TYPE,
        "freecadcmd",
        arguments = freecad_script_args(ctx.file.script),
        inputs = [ctx.file.script],
        outputs = [out],
        path_env = {"STEP_OUT": out},
        mnemonic = "FreecadStep",
        progress_message = "Making the test part %{label}",
    )
    return [DefaultInfo(files = depset([out]))]

freecad_step = rule(
    implementation = _freecad_step_impl,
    doc = "A test part: a script that freecadcmd runs writes a STEP file to $STEP_OUT.",
    attrs = {"script": attr.label(mandatory = True, allow_single_file = [".py"])},
    toolchains = [FREECAD_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)

def _runfiles_path(ctx, f):
    if f.short_path.startswith("../"):
        return f.short_path[3:]
    return ctx.workspace_name + "/" + f.short_path

def _cam_job_failure_test_impl(ctx):
    tc = ctx.toolchains[FREECAD_TOOLCHAIN_TYPE]
    cmd = tc.tools["freecadcmd"]
    lib = ctx.attr.tools[ToolLibraryInfo].file
    config = ctx.actions.declare_file(ctx.label.name + ".cam_job.json")

    # The test runs in its runfiles, from the main repository's directory,
    # so the runner sees each file at its short path.
    ctx.actions.write(config, json.encode_indent(cam_job_config(
        ctx,
        "unexpected.ngc",
        path = lambda f: f.short_path,
    )) + "\n")
    case = {
        "args": freecad_script_args(ctx.file._runner, path = ctx.file._runner.short_path),
        "env": {"CAM_JOB_CONFIG": config.short_path},
        "script": str(ctx.label),
        "want": 1,
        "want_output": ctx.attr.message,
    }
    exe = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.symlink(output = exe, target_file = ctx.executable.checker, is_executable = True)
    runfiles = ctx.runfiles(
        files = [config, lib, ctx.file.part, ctx.file.ops, ctx.file._runner, cmd.executable],
        transitive_files = tc.tool_inputs,
    ).merge(ctx.attr.checker[DefaultInfo].default_runfiles)
    return [
        DefaultInfo(executable = exe, runfiles = runfiles),
        RunEnvironmentInfo(environment = {
            "EXITCHECK_CASES": json.encode([case]),
            "EXITCHECK_TOOL": _runfiles_path(ctx, cmd.executable),
        }),
    ]

cam_job_failure_test = rule(
    implementation = _cam_job_failure_test_impl,
    test = True,
    doc = """Runs a cam_job's runner on a job it must refuse, and checks the refusal.

It takes cam_job's attributes, and `message`, which the runner's output
must hold; the runner must exit 1. A refused job fails its action, which
no passing test can build, so the test runs the same runner at test time.
""",
    attrs = dict(CAM_JOB_ATTRS, **{
        "message": attr.string(
            doc = "Text the refusal must hold.",
            mandatory = True,
        ),
        "checker": attr.label(
            doc = "The exitcheck binary.",
            executable = True,
            cfg = "target",
            mandatory = True,
        ),
    }),
    toolchains = [FREECAD_TOOLCHAIN_TYPE],
)

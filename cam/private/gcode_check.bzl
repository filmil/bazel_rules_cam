# SPDX-License-Identifier: Apache-2.0
"""gcode_check_test: a job's G-code against its machine (issue #10)."""

load(":cam_job.bzl", "CamJobInfo")
load(":machine.bzl", "MachineInfo")
load(":tool_library.bzl", "ToolLibraryInfo")

GCODE_CHECK_TOOLCHAIN_TYPE = Label("//cam:gcode_check_toolchain_type")

GCODE_CHECK_KINDS = ["envelope", "feed", "rapid", "spindle", "tool", "unsupported"]

def _gcode_check_toolchain_impl(ctx):
    return [platform_common.ToolchainInfo(checker = ctx.attr.checker[DefaultInfo])]

gcode_check_toolchain = rule(
    implementation = _gcode_check_toolchain_impl,
    doc = """A G-code checker, for `@hdlf_rules_cam//cam:gcode_check_toolchain_type`.

hdlf_rules_cam registers its own, built from source. The checker runs as the
test, and exits 0 when the G-code passes. Its arguments arrive in the
environment variable `GCODE_CHECK_ARGS`, one per line, followed by any the
test is given: `--gcode=G`, `--machine=M`, `--tools=T`, `--info=I` and
`--expect=KIND`. The paths are runfiles paths, from the test's working
directory. `M` is JSON: `{"envelope": [x, y, z], "max_feed": ...,
"max_spindle": ...}`; `T` is a canonical tool library; `I` is the job's
`{"stock": {"XMin": ..., "ZMax": ...}}`.
""",
    attrs = {
        "checker": attr.label(
            doc = "The checker binary.",
            mandatory = True,
            executable = True,
            cfg = "target",
        ),
    },
)

def _gcode_check_test_impl(ctx):
    job = ctx.attr.job[CamJobInfo]
    m = ctx.attr.machine[MachineInfo] if ctx.attr.machine else job.machine
    tools = ctx.attr.tools[ToolLibraryInfo].file if ctx.attr.tools else job.tools
    gcode = ctx.file.gcode or job.gcode
    machine = ctx.actions.declare_file(ctx.label.name + ".machine.json")
    ctx.actions.write(machine, json.encode_indent({
        "envelope": [m.envelope.x, m.envelope.y, m.envelope.z],
        "max_feed": m.max_feed,
        "max_spindle": m.max_spindle,
    }) + "\n")
    checker = ctx.toolchains[GCODE_CHECK_TOOLCHAIN_TYPE].checker
    exe = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.symlink(output = exe, target_file = checker.files_to_run.executable, is_executable = True)
    args = [
        "--gcode=" + gcode.short_path,
        "--machine=" + machine.short_path,
        "--tools=" + tools.short_path,
        "--info=" + job.info.short_path,
    ]
    if ctx.attr.expect:
        args.append("--expect=" + ctx.attr.expect)
    runfiles = ctx.runfiles(files = [gcode, machine, tools, job.info]).merge(checker.default_runfiles)
    return [
        DefaultInfo(executable = exe, runfiles = runfiles),
        RunEnvironmentInfo(environment = {"GCODE_CHECK_ARGS": "\n".join(args)}),
    ]

gcode_check_test = rule(
    implementation = _gcode_check_test_impl,
    test = True,
    doc = """Checks a job's G-code against its machine, its stock and its tool library.

```starlark
gcode_check_test(
    name = "bracket_gcode_test",
    job = ":bracket",
)
```

The test fails, and its report names each problem, when:

* `envelope`: on some axis, the moves (arcs included) span more than the
  machine's travel. The G-code is in the part's coordinates, and where
  the part sits on the machine is the operator's choice, so this checks
  that the job fits;
* `rapid`: a rapid move (G0) goes down to, or across at, a height below
  the stock's top while over the stock (a rapid straight up, out of a
  cut, is fine);
* `feed`: a feed rate above the machine's `max_feed`, or a cutting move
  with no feed rate;
* `spindle`: a spindle speed above the machine's `max_spindle`;
* `tool`: a tool number not in the tool library (GRBL's tool change,
  written as a comment, counts);
* `unsupported`: G-code the checker does not follow (arcs outside the XY
  plane, words it does not know), so it cannot vouch for the job.

The report is in the test's output and in its undeclared outputs
(`gcode_check_report.txt`). `machine`, `tools` and `gcode` check the job
against something else than what it was made with: another machine, or
a fixture. `expect` makes it a known-bad test (spec T3): it passes when
every violation is of that kind.

A person checks G-code before a machine runs it.
""",
    attrs = {
        "job": attr.label(
            doc = "The job (`cam_job`).",
            mandatory = True,
            providers = [CamJobInfo],
        ),
        "machine": attr.label(
            doc = "Check against this machine instead of the job's.",
            providers = [MachineInfo],
        ),
        "tools": attr.label(
            doc = "Check against this tool library instead of the job's.",
            providers = [ToolLibraryInfo],
        ),
        "gcode": attr.label(
            doc = "Check this G-code instead of the job's: a fixture.",
            allow_single_file = [".ngc", ".nc", ".gcode"],
        ),
        "expect": attr.string(
            doc = "Pass only when there are violations, all of this kind.",
            values = [""] + GCODE_CHECK_KINDS,
        ),
    },
    toolchains = [GCODE_CHECK_TOOLCHAIN_TYPE],
)

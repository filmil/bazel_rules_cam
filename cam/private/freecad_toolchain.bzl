# SPDX-License-Identifier: Apache-2.0
"""The FreeCAD toolchain: freecadcmd, its tree, and its environment."""

FREECAD_TOOLCHAIN_TYPE = Label("//cam:toolchain_type")

FreecadToolchainInfo = provider(
    doc = "What a FreeCAD toolchain offers beyond the `freecadcmd` tool.",
    fields = {
        "version": "The FreeCAD version freecadcmd reports, such as \"1.1.4\".",
        "posts": "Sorted list of the post-processors in the tree, by the " +
                 "name FreeCAD's PostProcessorFactory takes (\"linuxcnc\" " +
                 "for `linuxcnc_post.py`).",
    },
)

# freecadcmd's environment in every action: the same locale and zone
# whatever the host has. UTC0 is a POSIX TZ string, so glibc reads no zone
# file for it. FreeCAD starts its own Python with ignore_environment set,
# so PYTHONDONTWRITEBYTECODE does nothing there (freecad_script_args turns
# off .pyc writing from inside instead); it is for any Python process that
# FreeCAD starts.
FREECAD_ENV = {
    "LC_ALL": "C.UTF-8",
    "LANG": "C.UTF-8",
    "TZ": "UTC0",
    "PYTHONDONTWRITEBYTECODE": "1",
}

# Runs the script as __main__ with no .pyc writing, and exits with its
# status: freecadcmd itself reports an exception and exits 0.
_BOOTSTRAP = """import os, runpy, sys, traceback
sys.dont_write_bytecode = True
code = 0
try:
    runpy.run_path(%r, run_name="__main__")
except SystemExit as e:
    code = e.code if isinstance(e.code, int) else (0 if e.code is None else 1)
except BaseException:
    traceback.print_exc()
    code = 1
sys.stdout.flush()
sys.stderr.flush()
os._exit(code)
"""

def freecad_script_args(script, path = None):
    """freecadcmd's arguments that run a Python script, writing no .pyc file.

    Python would otherwise write `__pycache__` next to the script, and next
    to every module it imports. Where the sandbox does not isolate the file
    system, that is the source tree. FreeCAD ignores PYTHONDONTWRITEBYTECODE,
    so a bootstrap sets `sys.dont_write_bytecode` first, then runs the
    script as `__main__`.

    freecadcmd reports an exception in a script and still exits 0. The
    bootstrap exits 1 on an exception, and with the code of `sys.exit`, so
    the action fails when the script does.

    Args:
      script: the script's `File`.
      path: the path to give freecadcmd in place of `script.path`, such as
        a runfiles path for a test.

    Returns:
      A list of strings for `run_tool`'s `arguments`.
    """
    return ["-c", _BOOTSTRAP % (path if path != None else script.path)]

# FreeCAD keeps its user settings, macros and caches under these; run_tool
# points each at its own directory inside the action.
FREECAD_HOME_ENV = ["FREECAD_USER_HOME", "FREECAD_USER_DATA", "FREECAD_USER_TEMP"]

def _check_posts(posts):
    for p in posts:
        if not p or "/" in p or p.endswith(".py") or p.endswith("_post"):
            fail("posts: %r is not a post-processor name (want \"linuxcnc\", not \"linuxcnc_post.py\")" % p)
    return sorted(posts)

def _freecad_toolchain_impl(ctx):
    cmd = ctx.attr.freecadcmd[DefaultInfo].files_to_run
    if cmd == None or cmd.executable == None:
        fail("freecadcmd: %s is not executable" % ctx.attr.freecadcmd.label)
    return [platform_common.ToolchainInfo(
        tools = {"freecadcmd": cmd},
        tool_inputs = depset(ctx.files.data),
        env = FREECAD_ENV,
        home_env = FREECAD_HOME_ENV,
        freecad = FreecadToolchainInfo(
            version = ctx.attr.version,
            posts = _check_posts(ctx.attr.posts),
        ),
    )]

freecad_toolchain = rule(
    implementation = _freecad_toolchain_impl,
    doc = """A FreeCAD toolchain, for the toolchain type `@hdlf_rules_cam//cam:toolchain_type`.

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
""",
    attrs = {
        "freecadcmd": attr.label(
            doc = "The freecadcmd executable.",
            mandatory = True,
            executable = True,
            allow_single_file = True,
            cfg = "exec",
        ),
        "data": attr.label_list(
            doc = "The rest of the FreeCAD tree: libraries, Python, workbenches.",
            allow_files = True,
            cfg = "exec",
        ),
        "posts": attr.string_list(
            doc = "The post-processors the tree has, by name.",
            mandatory = True,
        ),
        "version": attr.string(
            doc = "The FreeCAD version freecadcmd reports.",
            mandatory = True,
        ),
    },
)

def _freecad_stub_toolchain_impl(ctx):
    return [platform_common.ToolchainInfo(
        tools = {"freecadcmd": ctx.attr.stub[DefaultInfo].files_to_run},
        env = FREECAD_ENV,
        home_env = FREECAD_HOME_ENV,
        freecad = FreecadToolchainInfo(
            version = ctx.attr.version,
            posts = _check_posts(ctx.attr.posts),
        ),
    )]

freecad_stub_toolchain = rule(
    implementation = _freecad_stub_toolchain_impl,
    doc = """swap_test's FreeCAD toolchain: the stub stands in for freecadcmd.

It offers the same `FreecadToolchainInfo`, environment and home variables
as the prebuilt toolchain, so a rule analyzes and runs the launcher the same
way under the stub.
""",
    attrs = {
        "stub": attr.label(mandatory = True, executable = True, cfg = "exec"),
        "posts": attr.string_list(mandatory = True),
        "version": attr.string(mandatory = True),
    },
)

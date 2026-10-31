# SPDX-License-Identifier: Apache-2.0
"""The one place where hdlf_rules_cam starts an action that runs a tool.

Every rule runs its tools through `run_tool`; the hermeticity lint fails on
`ctx.actions.run` or `ctx.actions.run_shell` anywhere else (spec section
"Toolchains", requirement 3).
"""

HELPERS_TOOLCHAIN_TYPE = Label("//cam:helpers_toolchain_type")

# A rule that calls run_tool lists these in its `toolchains`, together with
# the toolchain types of the tools it runs.
RUN_TOOL_TOOLCHAINS = [HELPERS_TOOLCHAIN_TYPE]

def run_tool(
        ctx,
        toolchain_type,
        tool,
        arguments = [],
        inputs = [],
        outputs = [],
        mnemonic = None,
        progress_message = None,
        env = {},
        home_env = [],
        tools = [],
        stdout = None,
        path_env = {}):
    """Runs a tool from a toolchain, with a private home, as one action.

    The tool comes from `ctx.toolchains[toolchain_type].tools[tool]`. It runs
    under the launcher, which points HOME, TMPDIR and the XDG directories,
    and each variable named in `home_env`, at a fresh directory inside the
    action, so the tool reads and writes no host configuration (spec H3).

    A toolchain can add to the tool's environment and inputs, through
    optional fields of its `ToolchainInfo`:

    * `env`: dict of fixed values;
    * `path_env`: dict from name to a `File`; the tool sees the file's
      absolute path;
    * `home_env`: list of names, as the argument of the same name;
    * `tool_inputs`: depset of files the tool reads, such as its data.

    Values the rule passes win over the toolchain's.

    Args:
      ctx: the rule context. The rule's `toolchains` must include
        `toolchain_type` and `RUN_TOOL_TOOLCHAINS`.
      toolchain_type: the toolchain type that provides the tool.
      tool: the tool's name in that toolchain's `tools` dict.
      arguments: the tool's arguments: strings or `Args` objects.
      inputs: the action's inputs: a list or a depset of files.
      outputs: the action's outputs. The launcher also passes them to the
        tool in RUN_TOOL_OUTPUTS, which the swap test's stub uses.
      mnemonic: the action's mnemonic.
      progress_message: the action's progress message.
      env: fixed environment values for the tool.
      home_env: names of variables to point at directories of their own in
        the private home, such as `FREECAD_USER_HOME`.
      tools: more executables the action needs, from toolchains.
      stdout: a declared output `File` that gets the tool's standard output.
        It is added to `outputs`.
      path_env: dict from name to a path relative to the execution root (a
        string) or a `File`; the tool sees it made absolute. It adds to the
        toolchain's `path_env`, and wins over `env` for the same name.
    """
    info = ctx.toolchains[toolchain_type]
    if info == None:
        fail("no toolchain for %s; add it to the rule's toolchains" % toolchain_type)
    if tool not in info.tools:
        fail("toolchain for %s has no tool %r; it has %s" % (toolchain_type, tool, sorted(info.tools.keys())))
    exe = info.tools[tool]
    launcher = ctx.toolchains[HELPERS_TOOLCHAIN_TYPE].helpers.launcher

    outputs = list(outputs)
    if stdout != None and stdout not in outputs:
        outputs.append(stdout)
    all_env = dict(getattr(info, "env", None) or {})
    all_env.update(env)
    abs_env = {k: v.path for k, v in (getattr(info, "path_env", None) or {}).items()}
    for k, v in path_env.items():
        abs_env[k] = v if type(v) == "string" else v.path
        all_env.pop(k, None)
    all_home_env = sorted({n: True for n in list(getattr(info, "home_env", None) or []) + list(home_env)}.keys())
    tool_inputs = getattr(info, "tool_inputs", None) or depset()
    if type(inputs) == "depset":
        all_inputs = depset(transitive = [inputs, tool_inputs])
    else:
        all_inputs = depset(inputs, transitive = [tool_inputs])

    pre = ctx.actions.args()
    for name in all_home_env:
        pre.add("--home-env", name)
    for k in sorted(all_env.keys()):
        pre.add("--env", "%s=%s" % (k, all_env[k]))
    for k in sorted(abs_env.keys()):
        if k not in all_env:
            pre.add("--env-abs", "%s=%s" % (k, abs_env[k]))
    if stdout != None:
        pre.add("--stdout", stdout.path)
    for out in outputs:
        # The path, so a directory output (a tree artifact) is one value.
        pre.add("--output", out.path)
    pre.add("--")
    pre.add(exe.executable)

    ctx.actions.run(
        executable = launcher,
        arguments = [pre] + list(arguments),
        inputs = all_inputs,
        outputs = outputs,
        tools = [exe] + list(tools),
        mnemonic = mnemonic,
        progress_message = progress_message,
    )

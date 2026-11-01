<!-- SPDX-License-Identifier: Apache-2.0 -->
# Instructions

Read `README.md` first.

The coding standard is the `ai-coding-sop` repository at
`https://github.com/filmil/ai-coding-sop`.
Its `AGENTS.md`, its `prose-readability` skill, its `git-commit-rules`
skill and its `hermetic`, `bazel` and `bazel-bzl-files` fragments apply
here in full.
Where this file and ai-coding-sop differ, ai-coding-sop wins; ask the
operator about a conflict that matters.

# Agent operating rules

A change is complete only when `bazel test //...` passes, the integration
module passes (`cd integration && bazel test //...`), and new tests cover
the change.

1. Edit text sources only: operations scripts (`.py`), tool tables
   (`.json`), `BUILD.bazel`, `MODULE.bazel`, `.bzl`, Go sources. Do not
   commit generated outputs, except the docs that
   `bazel run //docs:update_docs` writes.
2. Keep every action hermetic. No host tools, no host files outside the
   workspace, no host environment variables, no network in actions. Add
   each external file with `http_archive` or `http_file` and a pinned
   `sha256`. `--action_env=PATH`, `use_default_shell_env = True` and the
   tags `local` and `no-sandbox` are forbidden. If a tool cannot run
   hermetically, stop and ask; do not make it work with a host install.
3. Find targets with `bazel query` and `bazel cquery`. Run
   `bazel run //tools:buildifier` on every change.
4. Once there are golden files, update them only with
   `bazel run //:update_goldens`, and give the reason in the commit
   message.
5. A change to a job or its part attaches the G-code check report to the
   pull request: the report that `gcode_check_test` prints into its test
   log (`bazel-testlogs/<package>/<test>/test.log`), also kept as
   `gcode_check_report.txt` in the test's `test.outputs/`. It also
   attaches the job's `gcode_render` pictures and report, before and
   after: put them on a branch that is never merged, link them, and
   delete the branch after the merge.
6. One logical change per pull request: one rule, one feature, or one
   design change.
7. Stop and ask the operator before you:
   * delete a test, or make a tolerance larger;
   * change a toolchain version or a library version;
   * write G-code that a machine will run;
   * run a release or publish workflow, or set a secret.
8. When a test fails and the cause is not clear after three attempts, stop.
   Write the failure and your hypotheses in the pull request.

# Rules particular to this repository

* **Tools come from toolchains only.** Every action runs its tool through
  `run_tool` in `//cam/private` (issue #4), with the tool from
  `ctx.toolchains`.
  No private attribute names an external tool.
* **Public packages name no dev dependency.** A user of the module loads
  `//cam` and `//`, where `@stardoc`, `@bazel_lib`, `@buildifier_prebuilt`
  and `@hermetic_cc_toolchain` are not visible. Documentation targets live in
  `//docs` and development tools in `//tools`.
* **Documentation is generated.** After changing a `.bzl` file, run
  `bazel run //docs:update_docs`; `bazel test //...` fails until then.
* **Linux x86_64 only** in v1.
* **Python only where the CAD library needs it.** Operations scripts and
  the FreeCAD CAM runner are Python; the G-code checker and every other
  helper is Go.
* **Operations scripts.** Give feeds with units (`"600 mm/min"`); a plain
  number is mm/s. Do not use Adaptive clearing: its toolpath changes from
  run to run, and `cam_job` refuses it. Set each operation's
  `ToolController`.
* **The FreeCAD prebuilt** is made by `make-freecad-prebuilt.sh`, which the
  operator runs; a new FreeCAD version is a toolchain version change (rule
  7), with the new tarball's sha256 from two separate runs.
* **Commits for a BCR pull request carry no `Co-Authored-By` trailer**
  (the `bazel-bcr-publish` skill says why). This is about the pull request
  on the Bazel Central Registry's repository that the release makes, not
  about commits in this repository.

`CLAUDE.md` and `GEMINI.md` are symlinks to this file.

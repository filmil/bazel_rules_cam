# SPDX-License-Identifier: Apache-2.0
"""Tests of machine: what MachineInfo holds, and the values it refuses."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("//cam:defs.bzl", "MachineInfo")

def _info_impl(ctx):
    env = analysistest.begin(ctx)
    m = analysistest.target_under_test(env)[MachineInfo]
    asserts.equals(env, struct(x = 400.0, y = 300.5, z = 75.0), m.envelope)
    asserts.equals(env, 5000.0, m.max_feed)
    asserts.equals(env, 24000, m.max_spindle)
    asserts.equals(env, "grbl", m.post)
    return analysistest.end(env)

machine_info_test = analysistest.make(_info_impl)

def _fails_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, ctx.attr.message)
    return analysistest.end(env)

machine_fails_test = analysistest.make(
    _fails_impl,
    expect_failure = True,
    attrs = {"message": attr.string(mandatory = True)},
)

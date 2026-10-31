# SPDX-License-Identifier: Apache-2.0
"""Analysis test: a post-processor the toolchain lacks fails with a clear message."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _unknown_post_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, "post \"nosuch\": the FreeCAD 1.1.4 toolchain has [")
    return analysistest.end(env)

unknown_post_test = analysistest.make(
    _unknown_post_impl,
    expect_failure = True,
)

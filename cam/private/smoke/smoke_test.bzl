# SPDX-License-Identifier: Apache-2.0
"""Analysis tests: post-processor names that a rule or a toolchain must refuse."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _unknown_post_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, "post \"nosuch\": the FreeCAD 1.1.4 toolchain has [")
    return analysistest.end(env)

unknown_post_test = analysistest.make(
    _unknown_post_impl,
    expect_failure = True,
)

def _bad_posts_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, "is not a post-processor name")
    return analysistest.end(env)

bad_posts_test = analysistest.make(
    _bad_posts_impl,
    expect_failure = True,
)

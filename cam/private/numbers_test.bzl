# SPDX-License-Identifier: Apache-2.0
"""Unit tests of parse_number, the strict number parser of the rules."""

load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load(":numbers.bzl", "parse_number")

def _numbers_test_impl(ctx):
    env = unittest.begin(ctx)
    for text, want in [
        ("3", 3.0),
        ("-0.5", -0.5),
        ("+2", 2.0),
        (".25", 0.25),
        ("1.", 1.0),
        ("1e-3", 0.001),
        ("2.5E+2", 250.0),
        (" 7 ", 7.0),
        ("1e299", 1e299),
        ("1e300", 1e300),
        ("-1e300", -1e300),
        ("0.001e301", 1e298),
        ("0.001e309", 1e306),
        (".5e308", 5e307),
        ("0e500", 0.0),
        ("-0", 0.0),
        ("1e00001", 10.0),
        ("1e-9999", 0.0),
        ("9" * 308, float("9" * 308)),
    ]:
        asserts.equals(env, want, parse_number(text), "parse_number(%r)" % text)
    for text in ["", ".", "-", "--", "e3", "1e", "1e+", "1.2.3", "nan", "inf", "1x", "1e99999", "1e-10000", "1e308", "1e999", "-1e308", "-1e999", ".5e309", "9" * 309, "-" + "9" * 309]:
        asserts.equals(env, None, parse_number(text), "parse_number(%r)" % text)
    return unittest.end(env)

numbers_test = unittest.make(_numbers_test_impl)

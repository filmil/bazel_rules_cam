# SPDX-License-Identifier: Apache-2.0
"""A script that ends normally: freecadcmd must exit 0, writing no .pyc."""

import sys

if not sys.dont_write_bytecode:
    raise RuntimeError("the bootstrap did not turn off .pyc writing")

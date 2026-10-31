# SPDX-License-Identifier: Apache-2.0
"""A test part that is two solids: a job must refuse it."""

import os

import FreeCAD
import Part

a = Part.makeBox(10, 10, 10)
b = Part.makeBox(10, 10, 10, FreeCAD.Vector(20, 0, 0))
Part.makeCompound([a, b]).exportStep(os.environ["STEP_OUT"])

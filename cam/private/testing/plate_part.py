# SPDX-License-Identifier: Apache-2.0
"""A test part: a 40x30x10 mm block with a 20x10x4 mm pocket in its top.

freecad_step runs this in freecadcmd; it writes the part to $STEP_OUT.
"""

import os

import FreeCAD
import Part

block = Part.makeBox(40, 30, 10)
shape = block.cut(Part.makeBox(20, 10, 4, FreeCAD.Vector(10, 10, 6)))
shape.exportStep(os.environ["STEP_OUT"])

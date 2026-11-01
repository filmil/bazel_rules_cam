# SPDX-License-Identifier: Apache-2.0
"""A test part: a 40x30x6 mm plate with a 5 mm through hole at its center,
for FreeCAD's Helix (issue #31).

freecad_step runs this in freecadcmd; it writes the part to $STEP_OUT.
"""

import os

import FreeCAD
import Part

plate = Part.makeBox(40, 30, 6)
hole = Part.makeCylinder(2.5, 8, FreeCAD.Vector(20, 15, -1))
plate.cut(hole).exportStep(os.environ["STEP_OUT"])

# SPDX-License-Identifier: Apache-2.0
"""The holed plate's hole, by FreeCAD's Helix with T1 (3 mm), 1 mm a turn.
The hole is small enough (5 mm) that Helix leaves it with a rapid to its
center at full depth, which gcode_check_test must accept (issue #31)."""

import Path.Op.Helix as Helix


def put(op, prop, value):
    """Sets a depth property, clearing the expression it follows first."""
    op.setExpression(prop, None)
    setattr(op, prop, value)


def operations(job, model, tools):
    t1 = tools[1]
    t1.HorizFeed = "600 mm/min"
    t1.VertFeed = "200 mm/min"
    t1.SpindleSpeed = 12000

    wall = [
        "Face%d" % (i + 1)
        for i, f in enumerate(model.Shape.Faces)
        if f.Surface.__class__.__name__ == "Cylinder"
    ]
    helix = Helix.Create("Hole", parentJob=job)
    helix.ToolController = t1
    helix.Base = [(model, wall)]
    put(helix, "StepDown", "1 mm")
    put(helix, "FinalDepth", model.Shape.BoundBox.ZMin)

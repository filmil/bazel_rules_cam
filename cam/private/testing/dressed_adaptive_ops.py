# SPDX-License-Identifier: Apache-2.0
"""An Adaptive pocket inside a boundary dressup: still Adaptive, still
refused, and before FreeCAD computes it."""

import Path.Dressup.Boundary as Boundary
import Path.Op.Adaptive as Adaptive


def operations(job, model, tools):
    t1 = tools[1]
    t1.HorizFeed = "600 mm/min"
    t1.VertFeed = "200 mm/min"
    t1.SpindleSpeed = 12000
    op = Adaptive.Create("Adaptive", parentJob=job)
    op.ToolController = t1
    Boundary.Create(op)

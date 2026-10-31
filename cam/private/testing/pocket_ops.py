# SPDX-License-Identifier: Apache-2.0
"""Operations for the plate: clear the pocket and cut the outline, with T1."""

import Path.Op.PocketShape as PocketShape
import Path.Op.Profile as Profile


def floor_faces(model, z):
    """The names of the part's flat faces at height z."""
    return [
        "Face%d" % (i + 1)
        for i, f in enumerate(model.Shape.Faces)
        if f.BoundBox.ZLength < 1e-6 and abs(f.BoundBox.ZMin - z) < 1e-6
    ]


def operations(job, model, tools):
    t1 = tools[1]
    t1.HorizFeed = "600 mm/min"
    t1.VertFeed = "200 mm/min"
    t1.SpindleSpeed = 12000
    pocket = PocketShape.Create("Pocket", parentJob=job)
    pocket.ToolController = t1
    pocket.Base = [(model, floor_faces(model, 6))]
    profile = Profile.Create("Profile", parentJob=job)
    profile.ToolController = t1

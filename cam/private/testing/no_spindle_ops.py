# SPDX-License-Identifier: Apache-2.0
"""A profile whose tool has feeds but no spindle speed."""

import Path.Op.Profile as Profile


def operations(job, model, tools):
    t1 = tools[1]
    t1.HorizFeed = "600 mm/min"
    t1.VertFeed = "200 mm/min"
    Profile.Create("Profile", parentJob=job).ToolController = t1

# SPDX-License-Identifier: Apache-2.0
"""A profile that is turned off: the job would cut nothing."""

import Path.Op.Profile as Profile


def operations(job, model, tools):
    t1 = tools[1]
    t1.HorizFeed = "600 mm/min"
    t1.VertFeed = "200 mm/min"
    t1.SpindleSpeed = 12000
    op = Profile.Create("Profile", parentJob=job)
    op.ToolController = t1
    op.Active = False

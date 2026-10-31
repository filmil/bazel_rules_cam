# SPDX-License-Identifier: Apache-2.0
"""A profile at 9000 mm/min and 30000 rpm: above the test machine's limits."""

import Path.Op.Profile as Profile


def operations(job, model, tools):
    t1 = tools[1]
    t1.HorizFeed = "9000 mm/min"
    t1.VertFeed = "200 mm/min"
    t1.SpindleSpeed = 30000
    op = Profile.Create("Profile", parentJob=job)
    op.ToolController = t1

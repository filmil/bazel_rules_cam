# SPDX-License-Identifier: Apache-2.0
"""A profile at 30000 rpm: above the test machine's 24000."""

import Path.Op.Profile as Profile


def operations(job, model, tools):
    t1 = tools[1]
    t1.HorizFeed = "600 mm/min"
    t1.VertFeed = "200 mm/min"
    t1.SpindleSpeed = 30000
    Profile.Create("Profile", parentJob=job).ToolController = t1

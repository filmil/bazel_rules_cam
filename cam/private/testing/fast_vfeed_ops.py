# SPDX-License-Identifier: Apache-2.0
"""A profile plunging at 6000 mm/min: above the test machine's 5000."""

import Path.Op.Profile as Profile


def operations(job, model, tools):
    t1 = tools[1]
    t1.HorizFeed = "600 mm/min"
    t1.VertFeed = "6000 mm/min"
    t1.SpindleSpeed = 12000
    Profile.Create("Profile", parentJob=job).ToolController = t1

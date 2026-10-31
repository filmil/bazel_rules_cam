# SPDX-License-Identifier: Apache-2.0
"""The plate's outline with holding tags: a dressup on a Profile, which a
job takes (the dressup's tool is its operation's)."""

import Path.Dressup.Tags as Tags
import Path.Op.Profile as Profile


def operations(job, model, tools):
    t1 = tools[1]
    t1.HorizFeed = "600 mm/min"
    t1.VertFeed = "200 mm/min"
    t1.SpindleSpeed = 12000
    op = Profile.Create("Profile", parentJob=job)
    op.ToolController = t1
    # A tag dressup reads its operation's toolpath: compute that first.
    job.Document.recompute()
    Tags.Create(op)

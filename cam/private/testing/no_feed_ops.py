# SPDX-License-Identifier: Apache-2.0
"""A profile whose tool has no feeds or spindle speed: a job must refuse it."""

import Path.Op.Profile as Profile


def operations(job, model, tools):
    op = Profile.Create("Profile", parentJob=job)
    op.ToolController = tools[1]

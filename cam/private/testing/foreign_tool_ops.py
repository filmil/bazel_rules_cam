# SPDX-License-Identifier: Apache-2.0
"""A profile with a tool controller the script made: not the library's."""

import Path.Op.Profile as Profile
import Path.Tool.Controller as Controller


def operations(job, model, tools):
    rogue = Controller.Create(name="rogue", toolNumber=7, assignViewProvider=False)
    rogue.HorizFeed = "600 mm/min"
    rogue.VertFeed = "200 mm/min"
    rogue.SpindleSpeed = 12000
    Profile.Create("Profile", parentJob=job).ToolController = rogue

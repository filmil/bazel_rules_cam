# SPDX-License-Identifier: Apache-2.0
"""Runs in freecadcmd: one CAM job on a STEP part, checked, then posted.

The part is a 40x30x10 mm block with a 20x10x4 mm pocket. It goes out to
STEP and back in, as rules_cam gets its input. A 3 mm end mill clears the
pocket and cuts the outline. The G-code goes to $SMOKE_OUT only when every
check passes. freecad_script_args runs this, so an exception or sys.exit(1)
fails the action.
"""

import os
import re
import sys
import tempfile


def fail(msg):
    sys.stderr.write("freecad_smoke: %s\n" % msg)
    sys.exit(1)


def span(op, axis, moves=("G1", "G01")):
    """The least and greatest value of an axis in an op's cutting moves."""
    vs = [c.Parameters[axis] for c in op.Path.Commands if c.Name in moves and axis in c.Parameters]
    if not vs:
        fail("operation %s has no cutting move in %s" % (op.Name, axis))
    return min(vs), max(vs)


# G-code carries three decimals; FreeCAD's own paths are off by 1e-6.
TOL = 1e-3


def near(a, b):
    return abs(a - b) < TOL


def main():
    import FreeCAD
    import Import
    import Part

    want = os.environ["SMOKE_VERSION"]
    got = ".".join(FreeCAD.Version()[:3])
    if got != want:
        fail("freecadcmd is FreeCAD %s, the toolchain says %s" % (got, want))

    step = os.path.join(tempfile.mkdtemp(), "part.step")
    block = Part.makeBox(40, 30, 10)
    shape = block.cut(Part.makeBox(20, 10, 4, FreeCAD.Vector(10, 10, 6)))
    shape.exportStep(step)

    doc = FreeCAD.newDocument("smoke")
    Import.insert(step, doc.Name)
    doc.recompute()
    solids = [o for o in doc.Objects if hasattr(o, "Shape") and o.Shape.Solids]
    if len(solids) != 1:
        fail("the STEP file gave %d solids, want 1" % len(solids))

    import Path.Main.Job as PathJob
    import Path.Op.PocketShape as PathPocket
    import Path.Op.Profile as PathProfile
    from Path.Post.Processor import PostProcessorFactory

    job = PathJob.Create("Job", [solids[0]])
    model = job.Model.Group[0]
    tc = job.Tools.Group[0]
    tc.Label = "TC: 3mm Endmill"
    tc.Tool.Diameter = 3.0
    # Units always: a plain number is read as mm/s.
    tc.HorizFeed = "600 mm/min"
    tc.VertFeed = "200 mm/min"
    tc.SpindleSpeed = 12000

    floors = [
        "Face%d" % (i + 1)
        for i, f in enumerate(model.Shape.Faces)
        if abs(f.BoundBox.ZMin - 6) < 1e-6 and abs(f.BoundBox.ZMax - 6) < 1e-6
    ]
    if len(floors) != 1:
        fail("found %d pocket floors, want 1" % len(floors))
    pocket = PathPocket.Create("Pocket", parentJob=job)
    pocket.Base = [(model, floors)]
    profile = PathProfile.Create("Profile", parentJob=job)
    doc.recompute()

    for op in (pocket, profile):
        if not op.Path.Commands:
            fail("operation %s made no path" % op.Name)
        state = getattr(op, "State", ())
        if "Invalid" in state or "Error" in state:
            fail("operation %s is %s" % (op.Name, state))

    # The pocket: down to its floor, and the tool's centre within the
    # pocket less the tool radius (X 11.5 to 28.5, Y 11.5 to 18.5).
    zlo, _ = span(pocket, "Z")
    if not near(zlo, 6):
        fail("the pocket goes to Z %g, want 6 (its floor)" % zlo)
    for axis, lo, hi in (("X", 11.5, 28.5), ("Y", 11.5, 18.5)):
        a, b = span(pocket, axis)
        if a < lo - TOL or b > hi + TOL:
            fail("the pocket spans %s %g to %g, want it within %g to %g" % (axis, a, b, lo, hi))

    # The profile: through the part, by FreeCAD's default of 1 mm below
    # the bottom, and the outline plus the tool radius.
    zlo, _ = span(profile, "Z")
    if not near(zlo, -1):
        fail("the profile goes to Z %g, want -1" % zlo)
    for axis, lo, hi in (("X", -1.5, 41.5), ("Y", -1.5, 31.5)):
        a, b = span(profile, axis)
        if not (near(a, lo) and near(b, hi)):
            fail("the profile spans %s %g to %g, want %g to %g" % (axis, a, b, lo, hi))

    # No header: it holds the date and the document's path.
    job.PostProcessorArgs = "--no-header --no-show-editor"
    sections = PostProcessorFactory.get_post_processor(job, os.environ["SMOKE_POST"]).export()
    gcode = "".join(g for _, g in sections)
    feeds = set(re.findall(r"\bF([0-9.]+)", gcode))
    if {float(f) for f in feeds} != {200.0, 600.0}:
        fail("the G-code has feeds %s, want 200 and 600 mm/min" % sorted(feeds))
    with open(os.environ["SMOKE_OUT"], "w") as f:
        f.write(gcode)


main()

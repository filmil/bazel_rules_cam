# SPDX-License-Identifier: Apache-2.0
"""Runs in freecadcmd: one CAM job on a STEP part, checked, then posted.

The part is a 40x30x10 mm block with a 20x10x4 mm pocket. It goes out to
STEP and back in, as rules_cam gets its input. A 3 mm end mill clears the
pocket and cuts the outline. The G-code goes to $SMOKE_OUT only when every
check passes; freecadcmd reports a failing script and still exits 0, so the
missing output is what fails the action.
"""

import os
import sys
import tempfile
import traceback


def fail(msg):
    sys.stderr.write("freecad_smoke: %s\n" % msg)
    sys.stderr.flush()
    os._exit(1)


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

    def zs(op):
        return [c.Parameters["Z"] for c in op.Path.Commands if c.Name in ("G1", "G01") and "Z" in c.Parameters]

    if abs(min(zs(pocket)) - 6) > 1e-6:
        fail("the pocket goes to Z %g, want 6 (its floor)" % min(zs(pocket)))
    if min(zs(profile)) > 0:
        fail("the profile goes to Z %g, want it through the part" % min(zs(profile)))
    xs = [c.Parameters["X"] for c in profile.Path.Commands if "X" in c.Parameters]
    if abs(min(xs) + 1.5) > 1e-6 or abs(max(xs) - 41.5) > 1e-6:
        fail("the profile spans X %g to %g, want -1.5 to 41.5" % (min(xs), max(xs)))

    # No header: it holds the date and the document's path.
    job.PostProcessorArgs = "--no-header --no-show-editor"
    sections = PostProcessorFactory.get_post_processor(job, os.environ["SMOKE_POST"]).export()
    gcode = "".join(g for _, g in sections)
    if "F600" not in gcode:
        fail("the G-code has no F600 feed")
    with open(os.environ["SMOKE_OUT"], "w") as f:
        f.write(gcode)


try:
    main()
except SystemExit:
    raise
except BaseException:
    traceback.print_exc()
    fail("the script raised")
sys.stdout.flush()
os._exit(0)

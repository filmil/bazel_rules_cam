# SPDX-License-Identifier: Apache-2.0
"""cam_job's runner: one CAM job in freecadcmd, from a STEP part to G-code.

cam_job runs this through freecad_script_args, so an exception or
sys.exit(1) fails the action. $CAM_JOB_CONFIG names a JSON file:

    {"part": STEP, "tools": canonical tool library, "ops": operations
     script, "out": G-code to write,
     "stock": {"x": mm, "y": mm, "z_top": mm, "z_bottom": mm},
     "machine": {"max_feed": mm/min, "max_spindle": rpm, "post": name},
     "label": the target, for messages}

The operations script defines operations(job, model, tools): `model` is
the job's copy of the part, `tools` maps each tool number of the library
to its tool controller. It adds FreeCAD CAM operations to the job, and
sets each tool controller's feeds and spindle speed. The runner then
refuses what it cannot vouch for, and posts.
"""

import json
import os
import re
import sys


class JobError(Exception):
    """A problem with the job, reported as is."""


# FreeCAD's tool bit shape files, by the library's shape name.
SHAPE_FILES = {
    "endmill": "endmill.fcstd",
    "ballend": "ballend.fcstd",
    "bullnose": "bullnose.fcstd",
    "drill": "drill.fcstd",
    "vbit": "v-bit.fcstd",
    "chamfer": "chamfer.fcstd",
}

# The library's dimensions, by FreeCAD's tool bit property, with units.
DIMENSIONS = {
    "diameter": ("Diameter", "mm"),
    "length": ("Length", "mm"),
    "cutting_edge_height": ("CuttingEdgeHeight", "mm"),
    "shank_diameter": ("ShankDiameter", "mm"),
    "corner_radius": ("CornerRadius", "mm"),
    "tip_diameter": ("TipDiameter", "mm"),
    "tip_angle": ("TipAngle", "deg"),
    "cutting_edge_angle": ("CuttingEdgeAngle", "deg"),
}

# Operations whose output depends on more than their input: Adaptive stops
# its search on a CPU-time budget and draws from rand() (libarea's
# Adaptive.cpp), so its toolpath changes from run to run.
REFUSED_OPS = {"Path.Op.Adaptive": "Adaptive clearing is not deterministic (it runs on a CPU-time budget)"}

# A comment that holds a date or a time: posts write one into the header,
# which the runner turns off; any left over is dropped.
DATED = re.compile(r"^\s*[(;].*(\d{4}-\d{2}-\d{2}|\d{1,2}:\d{2}:\d{2}).*$")


def load_config():
    with open(os.environ["CAM_JOB_CONFIG"]) as f:
        return json.load(f)


def import_part(doc, path):
    import Part

    # Part.read gives the file's shape once; Import.insert would also make
    # an object per child of a compound, and count each solid twice.
    shape = Part.read(path)
    if len(shape.Solids) != 1:
        raise JobError("the part %s has %d solids; a job machines one" % (path, len(shape.Solids)))
    obj = doc.addObject("Part::Feature", "Part")
    obj.Shape = shape.Solids[0]
    doc.recompute()
    return obj


def set_stock(job, stock):
    st = job.Stock
    st.ExtXneg = st.ExtXpos = stock["x"]
    st.ExtYneg = st.ExtYpos = stock["y"]
    st.ExtZpos = stock["z_top"]
    st.ExtZneg = stock["z_bottom"]


def make_tools(doc, job, library):
    import Path.Tool.Controller as PathToolController
    from Path.Tool.toolbit import ToolBit

    # The job comes with a default 5 mm end mill; only the library's tools
    # may cut.
    for tc in list(job.Tools.Group):
        tool = getattr(tc, "Tool", None)
        job.Tools.removeObject(tc)
        doc.removeObject(tc.Name)
        if tool is not None:
            doc.removeObject(tool.Name)
    tools = {}
    for t in library["tools"]:
        bit = ToolBit.from_shape_id(SHAPE_FILES[t["shape"]]).attach_to_doc(doc=doc)
        bit.Label = t["name"]
        for key, (prop, unit) in DIMENSIONS.items():
            if key in t:
                setattr(bit, prop, "%r %s" % (float(t[key]), unit))
        if "flutes" in t:
            bit.Flutes = t["flutes"]
        tc = PathToolController.Create(
            name="T%d: %s" % (t["number"], t["name"]),
            tool=bit,
            toolNumber=t["number"],
            assignViewProvider=False,
        )
        tc.HorizFeed = tc.VertFeed = "0 mm/min"
        tc.SpindleSpeed = 0
        job.Proxy.addToolController(tc)
        tools[t["number"]] = tc
    return tools


class HeadlessInput:
    """FreeCAD's CAM asks the GUI (PathUtils.UserInput) which tool an op
    gets when a job has more than one. Without a GUI, FreeCAD 1.1.4's
    findToolController then fails with UnboundLocalError
    (https://github.com/FreeCAD/FreeCAD/issues/31849, fixed after 1.1.4).
    This gives each new op the lowest-numbered tool; the script sets
    op.ToolController to the one it wants."""

    def __init__(self, job):
        self.job = job

    def selectedToolController(self):
        return None

    def chooseToolController(self, controllers):
        return min(controllers, key=lambda tc: tc.ToolNumber)

    def chooseJob(self, jobs):
        if self.job in jobs:
            return self.job
        raise JobError("an operation asked for a job other than this one")

    def createJob(self):
        raise JobError("an operation asked to create a job; add operations to `job`")


def run_operations(path, job, model, tools):
    import runpy

    from PathScripts import PathUtils

    PathUtils.UserInput = HeadlessInput(job)

    g = runpy.run_path(path, run_name="cam_job_operations")
    fn = g.get("operations")
    if not callable(fn):
        raise JobError("the operations script %s defines no operations(job, model, tools)" % path)
    fn(job=job, model=model, tools=dict(tools))


def mm_per_min(q):
    return q.getValueAs("mm/min").Value


def check(job, tools, machine):
    ops = list(job.Operations.Group)
    if not ops:
        raise JobError("the operations script added no operations")
    ours = {tc.Name for tc in tools.values()}
    problems = []
    for op in ops:
        module = type(op.Proxy).__module__
        if module in REFUSED_OPS:
            problems.append("operation %s: %s; use Pocket or Profile" % (op.Label, REFUSED_OPS[module]))
            continue
        if not getattr(op, "Active", True):
            continue
        state = getattr(op, "State", ())
        if "Invalid" in state or "Error" in state:
            problems.append("operation %s failed (%s); see FreeCAD's message above" % (op.Label, state))
        if not op.Path.Commands:
            problems.append("operation %s made no toolpath" % op.Label)
        tc = getattr(op, "ToolController", None)
        if tc is None or tc.Name not in ours:
            problems.append("operation %s uses a tool that is not in the tool library" % op.Label)
    for number, tc in sorted(tools.items()):
        used = [op.Label for op in ops if getattr(op, "ToolController", None) is tc]
        if not used:
            continue
        for what, q in (("horizontal feed", tc.HorizFeed), ("vertical feed", tc.VertFeed)):
            v = mm_per_min(q)
            if not v > 0:
                problems.append("T%d (used by %s): no %s; set it, with units: \"600 mm/min\"" % (number, ", ".join(used), what))
            elif v > machine["max_feed"] + 1e-9:
                problems.append("T%d: %s %g mm/min is above the machine's %g" % (number, what, v, machine["max_feed"]))
        s = float(tc.SpindleSpeed)
        if not s > 0:
            problems.append("T%d (used by %s): no spindle speed" % (number, ", ".join(used)))
        elif s > machine["max_spindle"]:
            problems.append("T%d: spindle %g rpm is above the machine's %d" % (number, s, machine["max_spindle"]))
    if problems:
        raise JobError("\n".join(problems))


def post(job, name):
    from Path.Post.Processor import PostProcessorFactory

    job.PostProcessor = name
    # No header: it holds the date and the document's path.
    job.PostProcessorArgs = "--no-header --no-show-editor"
    sections = PostProcessorFactory.get_post_processor(job, name).export()
    if not sections:
        raise JobError("post-processor %s wrote nothing" % name)
    gcode = "".join(g for _, g in sections)
    lines = [l for l in gcode.splitlines(True) if not DATED.match(l)]
    return "".join(lines)


def main():
    cfg = load_config()
    import FreeCAD
    import Path.Main.Job as PathJob

    doc = FreeCAD.newDocument("cam_job")
    part = import_part(doc, cfg["part"])
    job = PathJob.Create("Job", [part])
    set_stock(job, cfg["stock"])
    with open(cfg["tools"]) as f:
        tools = make_tools(doc, job, json.load(f))
    run_operations(cfg["ops"], job, job.Model.Group[0], tools)
    doc.recompute()
    check(job, tools, cfg["machine"])
    gcode = post(job, cfg["machine"]["post"])
    with open(cfg["out"], "w") as f:
        f.write(gcode)


if __name__ == "__main__":
    try:
        main()
    except JobError as e:
        cfg_label = ""
        try:
            cfg_label = load_config().get("label", "")
        except Exception:
            pass
        for line in str(e).splitlines():
            sys.stderr.write("cam_job %s: %s\n" % (cfg_label, line))
        sys.exit(1)

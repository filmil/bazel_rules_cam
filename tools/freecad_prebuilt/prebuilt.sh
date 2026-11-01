#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Builds the FreeCAD 1.1.4 prebuilt for rules_cam and, with --upload,
# uploads it as a release asset of filmil/bazel_rules_cam (spec section
# "Prebuilts").
#
# The operator runs it through its Bazel tool, which embeds this file:
#
#   bazel run //tools:freecad_prebuilt -- --work=DIR [--upload]
#
# The tool passes FREECAD_WORK (the work directory) and EXPECTED_SHA256
# (the tarball's sha256 that cam/private/extensions.bzl pins). It:
#   1. downloads the official FreeCAD 1.1.4 x86_64 AppImage and checks its
#      sha256;
#   2. extracts its squashfs image with unsquashfs, without running the
#      AppImage;
#   3. prunes it to what freecadcmd needs for CAM (about 3.1 GB to 0.7 GB):
#      the CAM, Part, Import, Material, Draft, Sketcher, PartDesign, Mesh,
#      MeshPart and TechDraw workbenches, the Python stdlib, a few
#      site-packages, and the shared libraries those need;
#   4. precompiles every Python module (unchecked-hash .pyc), so Python
#      never writes into the tree, then checks the tree: freecadcmd runs,
#      reports 1.1.4, imports CAM, and leaves the tree unchanged;
#   5. makes a reproducible tarball (sorted, mtime 0, uid/gid 0, xz -T1);
#   6. checks the tarball's sha256 against EXPECTED_SHA256, and stops if
#      they differ (a new tarball is a toolchain change: AGENTS.md rule 7);
#   7. with --upload only, uploads it to the release prebuilt-freecad-1.1.4,
#      whose tag sits on the orphan branch `prebuilts`, so release
#      workflows that read the latest tag on main never see it.
# It is safe to run again: finished steps are skipped.
# At the end it prints the tarball's sha256 and size.
set -o errexit -o nounset -o pipefail
# The modes in the tarball come from the umask: the pinned tarball was made
# with 077 (files 0600 and 0700, directories 0700), so this sets it, and the
# bytes do not depend on the shell that runs it.
umask 077

FREECAD_VERSION=1.1.4
APPIMAGE_NAME="FreeCAD_${FREECAD_VERSION}-Linux-x86_64-py311.AppImage"
APPIMAGE_URL="https://github.com/FreeCAD/FreeCAD/releases/download/${FREECAD_VERSION}/${APPIMAGE_NAME}"
APPIMAGE_SHA256=f6dc6ba676e5ac96a565ebc8d657232f94c6158e85b4352141bd1a46f6b43434
REPO=filmil/bazel_rules_cam
TAG="prebuilt-freecad-${FREECAD_VERSION}"
NAME="freecad-${FREECAD_VERSION}-linux-x86_64"
WORK=${FREECAD_WORK:?the work directory: run this through bazel run //tools:freecad_prebuilt -- --work=DIR}
EXPECTED_SHA256=${EXPECTED_SHA256:?the pinned sha256}
DRY_RUN=1
[ "${1:-}" = "--upload" ] && DRY_RUN=0

die() { echo "ERROR: $*" >&2; exit 1; }
step() { echo; echo "== $*"; }

step "preconditions"
for t in curl sha256sum tar xz python3 readelf unsquashfs git; do
  command -v "$t" >/dev/null || die "$t is missing; install it and run again"
done
if [ "$DRY_RUN" = 0 ]; then
  gh auth status >/dev/null 2>&1 || die "gh is not logged in: run gh auth login"
fi
mkdir -p "$WORK/dl" "$WORK/out"
# The tree and the tarball need about 8 GB; a run whose tarball is already
# made (an --upload after a first run, say) needs none.
if [ ! -f "$WORK/out/freecad-${FREECAD_VERSION}-linux-x86_64.tar.xz" ]; then
  avail=$(df --output=avail -BG "$WORK" | tail -1 | tr -dc 0-9)
  [ "$avail" -ge 8 ] || die "less than 8 GB free on $WORK"
fi

step "download"
APPIMAGE="$WORK/dl/$APPIMAGE_NAME"
if [ -f "$APPIMAGE" ] && echo "$APPIMAGE_SHA256  $APPIMAGE" | sha256sum -c --status -; then
  echo "have $APPIMAGE_NAME"
else
  curl -fsSL -o "$APPIMAGE.part" "$APPIMAGE_URL" || die "download failed: $APPIMAGE_URL"
  echo "$APPIMAGE_SHA256  $APPIMAGE.part" | sha256sum -c --status - || die "sha256 mismatch for $APPIMAGE_URL"
  mv "$APPIMAGE.part" "$APPIMAGE"; echo "fetched $APPIMAGE_NAME"
fi

step "extract (the AppImage is not run)"
# The squashfs image starts right after the ELF runtime: at the end of the
# section header table.
OFFSET=$(python3 -I - "$APPIMAGE" <<'EOF'
import struct, sys
h = open(sys.argv[1], "rb").read(64)
assert h[:4] == b"\x7fELF" and h[4] == 2, "not a 64-bit ELF"
shoff, = struct.unpack_from("<Q", h, 0x28)
shentsize, shnum = struct.unpack_from("<HH", h, 0x3A)
print(shoff + shentsize * shnum)
EOF
)
echo "squashfs image at offset $OFFSET"
TREE="$WORK/out/$NAME"
if [ ! -d "$TREE" ]; then
  rm -rf "$TREE.part"
  unsquashfs -q -n -o "$OFFSET" -d "$TREE.part" "$APPIMAGE" >/dev/null
  [ -x "$TREE.part/usr/bin/freecadcmd" ] || die "no usr/bin/freecadcmd in the extracted tree; see ls $TREE.part"
  before=$(du -sm "$TREE.part" | cut -f1)

  step "prune"
  python3 -I - "$TREE.part" <<'EOF'
import os
import shutil
import subprocess
import sys

tree = sys.argv[1]
usr = os.path.join(tree, "usr")

KEEP_MODS = {"CAM", "Part", "Import", "Material", "Draft", "Sketcher", "PartDesign", "Mesh", "MeshPart", "TechDraw"}
KEEP_SITE = {
    "numpy", "numpy.libs", "yaml", "_yaml", "PySide6", "shiboken6", "freecad", "freecad.pkg",
    "_distutils_hack", "distutils-precedence.pth", "opencamlib",
}
KEEP_PYSIDE = {"__init__.py", "_config.py", "_git_pyside_version.py", "QtCore.cpython-311-x86_64-linux-gnu.so", "QtGui.cpython-311-x86_64-linux-gnu.so", "QtWidgets.cpython-311-x86_64-linux-gnu.so", "QtSvg.cpython-311-x86_64-linux-gnu.so", "QtCore.pyi", "__pycache__", "support", "py.typed"}
DROP_STDLIB = {"test", "idlelib", "tkinter", "turtledemo", "ensurepip", "lib2to3", "unittest/test"}
DROP_TOP = ["share/doc", "share/man", "share/locale", "share/icons", "share/gir-1.0", "share/mysql",
            "share/examples", "share/cups", "share/X11", "share/xkeyboard-config-2", "share/gtk-3.0",
            "share/terminfo", "share/qt6", "man", "fonts"]
removed = 0


def rm(p):
    global removed
    if os.path.islink(p) or os.path.isfile(p):
        removed += os.lstat(p).st_size
        os.unlink(p)
    elif os.path.isdir(p):
        for r, ds, fs in os.walk(p):
            for f in fs:
                removed += os.lstat(os.path.join(r, f)).st_size
        shutil.rmtree(p)


# 1. Workbenches (Mod and share/Mod), and binaries other than freecadcmd.
for base in ["Mod", "share/Mod"]:
    d = os.path.join(usr, base)
    for m in os.listdir(d):
        if m not in KEEP_MODS and not m.startswith("freecad"):
            rm(os.path.join(d, m))
for b in os.listdir(os.path.join(usr, "bin")):
    if b not in {"freecadcmd", "python", "python3", "python3.11"}:
        rm(os.path.join(usr, "bin", b))
for p in DROP_TOP:
    rm(os.path.join(usr, p))

# 2. Python: stdlib minus tests and GUI; only the listed site-packages.
py = os.path.join(usr, "lib", "python3.11")
for p in DROP_STDLIB:
    rm(os.path.join(py, p))
site = os.path.join(py, "site-packages")
for e in os.listdir(site):
    if e not in KEEP_SITE and not e.startswith(("PyYAML", "numpy-")):
        rm(os.path.join(site, e))
ps = os.path.join(site, "PySide6")
for e in os.listdir(ps):
    if e not in KEEP_PYSIDE:
        rm(os.path.join(ps, e))
for e in ["qt6", "openvino-2025.0.0", "vtk-9.3", "cmake", "pkgconfig", "gdk-pixbuf-2.0", "gtk-3.0", "girepository-1.0"]:
    rm(os.path.join(usr, "lib", e))


# 3. Shared libraries: the closure of DT_NEEDED from every ELF still here.
def is_elf(p):
    try:
        with open(p, "rb") as f:
            return f.read(4) == b"\x7fELF"
    except OSError:
        return False


def needed(p):
    out = subprocess.run(["readelf", "-d", p], capture_output=True, text=True).stdout
    return [l.split("[", 1)[1].split("]", 1)[0] for l in out.splitlines() if "(NEEDED)" in l]


lib = os.path.join(usr, "lib")
libs = {f: os.path.join(lib, f) for f in os.listdir(lib) if ".so" in f and not os.path.isdir(os.path.join(lib, f))}
roots = []
for r, ds, fs in os.walk(usr):
    if os.path.realpath(r) == os.path.realpath(lib):
        continue
    for f in fs:
        p = os.path.join(r, f)
        if not os.path.islink(p) and is_elf(p):
            roots.append(p)
# FreeCAD's own module libraries in usr/lib load by name from Python.
mods = {m.lower() for m in KEEP_MODS} | {"_partdesign", "patharea", "pathapp", "import", "materials", "part", "sketcher", "mesh", "meshpart", "freecad", "area", "pathsimulator", "camsimulator", "draftutils"}
for f, p in libs.items():
    stem = f.split(".so")[0].lower()
    if stem in mods or f.startswith("libFreeCADApp") or f.startswith("libFreeCADBase"):
        roots.append(p)
keep, todo, missing = set(), list(roots), set()
while todo:
    p = todo.pop()
    for n in needed(p):
        if n in keep:
            continue
        if n in libs:
            keep.add(n)
            q = libs[n]
            todo.append(os.path.realpath(q))
        else:
            missing.add(n)
# Keep each kept name's link chain.
for n in list(keep):
    q = libs[n]
    while os.path.islink(q):
        t = os.readlink(q)
        keep.add(os.path.basename(t))
        q = os.path.join(lib, t)
for f, p in libs.items():
    if f not in keep and not (f.split(".so")[0].lower() in mods) and not f.startswith(("libFreeCADApp", "libFreeCADBase", "libpython3")):
        rm(p)
print("removed %d MB" % (removed >> 20))
print("not in tree (host):", " ".join(sorted(missing)))

# 4. Links left dangling by the prune (they would fail a Bazel glob).
for r, ds, fs in os.walk(usr):
    for f in fs + ds:
        p = os.path.join(r, f)
        if os.path.islink(p) and not os.path.exists(p):
            os.unlink(p)
            print("removed dangling link", os.path.relpath(p, tree))
EOF
  echo "size: ${before} MB -> $(du -sm "$TREE.part" | cut -f1) MB"

  step "precompile"
  # Python writes a .pyc next to every module it imports that has none, and
  # FreeCAD ignores PYTHONDONTWRITEBYTECODE. In Bazel the tree is a fetched
  # repository, which an action must not change. So every module gets its
  # .pyc here, by the tree's own Python, in unchecked-hash mode: Python
  # loads it without checking it against the source, and never rewrites it.
  # The AppImage's own timestamp-checked ones go first, so each .pyc is made
  # the same way.
  find "$TREE.part" -type d -name __pycache__ -prune -exec rm -rf {} +
  # -B: the modules compileall itself imports (re, operator, ...) must not
  # get timestamp .pyc files first; -f: compile every module regardless.
  # compileall exits 1 when a module does not compile (some test modules of
  # bundled packages are Python 2); the check below is what counts.
  env -i PATH=/usr/bin:/bin "$TREE.part/usr/bin/python" -I -B -m compileall -q -f -j 1 \
      --invalidation-mode unchecked-hash -s "$TREE.part/" "$TREE.part/usr" > "$WORK/compileall.log" 2>&1 || true
  # Every module has its .pyc, and each is unchecked-hash (flags 1): Bazel
  # unpacks the tree with every mtime at 0, so Python would find a
  # timestamp .pyc stale and rewrite it inside the fetched repository.
  # Each module has its own .pyc, not just as many .pyc files as modules.
  # The tag (cpython-311) is the tree's Python's, not the host's.
  tag=$(env -i PATH=/usr/bin:/bin "$TREE.part/usr/bin/python" -I -B -c 'import sys; print(sys.implementation.cache_tag)')
  python3 -I - "$TREE.part" "$tag" <<'PYEOF' || die "compileall left modules without a .pyc; see $WORK/compileall.log"
import os, sys
root, tag = sys.argv[1], sys.argv[2]
mods = [os.path.join(r, f) for r, _, fs in os.walk(root) for f in fs if f.endswith(".py")]
missing = [m for m in mods if not os.path.exists(
    os.path.join(os.path.dirname(m), "__pycache__", os.path.basename(m)[:-3] + "." + tag + ".pyc"))]
print("\n".join(missing[:5]))
sys.exit(1 if missing or not mods else 0)
PYEOF
  python3 -I - "$TREE.part" <<'PYEOF' || die "some .pyc files are not unchecked-hash"
import os, sys
bad = [os.path.join(r, f) for r, _, fs in os.walk(sys.argv[1]) for f in fs
       if f.endswith(".pyc") and int.from_bytes(open(os.path.join(r, f), "rb").read(8)[4:8], "little") != 1]
print("\n".join(bad[:5]))
sys.exit(1 if bad else 0)
PYEOF
  echo "$(find "$TREE.part" -name '*.pyc' | wc -l) .pyc files"
  mv "$TREE.part" "$TREE"
fi

step "check"
# The check must not write into the tree: if a .pyc were missing, Python
# would write it here, and an action would do the same in Bazel.
home=$(mktemp -d)
trap 'rm -rf "$home"' EXIT
touch "$home/stamp"
v=$(env -i PATH=/usr/bin:/bin HOME="$home" "$TREE/usr/bin/freecadcmd" --version 2>&1 | tail -1)
if ! out=$(env -i PATH=/usr/bin:/bin HOME="$home" "$TREE/usr/bin/freecadcmd" -c \
  "import Import, Part, Path.Main.Job, Path.Op.Profile, Path.Op.PocketShape, Path.Op.Drilling, Path.Post.Processor" 2>&1); then
  echo "$out" | tail -20 >&2
  die "the pruned freecadcmd cannot import the CAM modules"
fi
changed=$(find "$TREE" -newer "$home/stamp" | head -3)
echo "$v"
case "$v" in "FreeCAD $FREECAD_VERSION "*) ;; *) die "the pruned freecadcmd does not run: $v";; esac
[ -z "$changed" ] || die "the check wrote into the tree: $changed; remove $TREE and run again"

step "reproducible tarball"
TARBALL="$WORK/out/$NAME.tar.xz"
if [ ! -f "$TARBALL" ]; then
  tar --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner \
      --pax-option=exthdr.name=%d/PaxHeaders/%f,delete=atime,delete=ctime \
      --format=posix -C "$WORK/out" -cf - "$NAME" | xz -T1 -9 > "$TARBALL.part"
  mv "$TARBALL.part" "$TARBALL"
fi
SHA=$(sha256sum "$TARBALL" | cut -d' ' -f1)
[ "$SHA" = "$EXPECTED_SHA256" ] || die "the tarball's sha256 is $SHA, not the pinned $EXPECTED_SHA256; $TARBALL is not the pinned prebuilt"
echo "sha256 matches the pinned $EXPECTED_SHA256"

if [ "$DRY_RUN" = 1 ]; then
  step "not uploading (give --upload)"
else
  step "upload to $REPO release $TAG"
  rc=0
  git ls-remote --exit-code "https://github.com/$REPO" refs/heads/prebuilts >/dev/null 2>&1 || rc=$?
  # 2: no such branch. Anything else (network, auth) is an error, not a
  # reason to create the branch.
  [ "$rc" = 0 ] || [ "$rc" = 2 ] || die "git ls-remote https://github.com/$REPO failed ($rc)"
  if [ "$rc" = 2 ]; then
    tmp=$(mktemp -d)
    git -C "$tmp" init -q
    git -C "$tmp" checkout -q --orphan prebuilts
    printf '# Prebuilt tool archives for rules_cam\n\nRelease assets only. The cam module extension pins each by sha256.\n' > "$tmp/README.md"
    git -C "$tmp" add README.md
    # The operator's own git identity makes the commit.
    git -C "$tmp" commit -q -m "chore: branch for prebuilt release tags"
    git -C "$tmp" -c credential.helper= -c credential.helper='!gh auth git-credential' \
      push -q "https://github.com/$REPO" prebuilts
    rm -rf "$tmp"
  fi
  if ! gh release view "$TAG" -R "$REPO" >/dev/null 2>&1; then
    gh release create "$TAG" -R "$REPO" --target prebuilts --prerelease --latest=false \
      --title "Prebuilt FreeCAD $FREECAD_VERSION (linux x86_64)" \
      --notes "Unpacked FreeCAD $FREECAD_VERSION AppImage, pruned to freecadcmd and the CAM workbench, for rules_cam's toolchain. Source: $APPIMAGE_URL (sha256 $APPIMAGE_SHA256). License: LGPL-2.0-or-later (FreeCAD); bundled libraries under their own licenses (packages.txt in the tree lists them). Built by bazel run //tools:freecad_prebuilt (tools/freecad_prebuilt)."
  fi
  gh release upload "$TAG" -R "$REPO" --clobber "$TARBALL"
fi

step "result"
echo "file:    $TARBALL"
echo "size:    $(du -m "$TARBALL" | cut -f1) MB"
echo "tree:    $(du -sm "$TREE" | cut -f1) MB"
echo "sha256:  $SHA"
[ "$DRY_RUN" = 1 ] || echo "url:     https://github.com/$REPO/releases/download/$TAG/$NAME.tar.xz"

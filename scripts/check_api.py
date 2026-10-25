#!/usr/bin/env python3
"""Compare the module's exported API with the latest v1 release.

The gate of docs/decisions/0024-PLAN-api-gate-and-deprecation.md, step 8, as
go-llmprovider-sdk and go-selfupdate-lib gate theirs: apidiff, pinned below
and run with `go run`, writes the API of each package in PACKAGES from the
base's tree, taken with `git archive`, and from the working tree, then
compares them. PACKAGES are the module's API: the root package and agentproc
(docs/decisions/0027-PLAN-agent-process-helper.md, step 2); the examples'
main packages are not compared. REPORTED are packages whose API is listed and
not gated: their incompatible changes are reported under a heading of their
own and do not fail the gate
(docs/decisions/0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 1).
A package missing from the base is compared against an empty package of the
same path written into the base's scratch copy, so its API reads as
additions; one missing from the working tree is a tool error.

  python3 scripts/check_api.py [BASE]

BASE is a git ref; it defaults to the newest v1.X.Y tag, by number. The
report lists the incompatible and the compatible changes. apidiff exits 0
either way, so its report is read; a line of it this script does not
recognise is a tool error, never a pass. It needs the tags, no commit, and
no network once the tool is cached: it asks the module mirror nothing about
this module.

Exit codes: 0 no incompatible change in a gated package, 1 an incompatible
change in one, 2 a tool error or no v1 tag.
"""

from __future__ import annotations

import io
import re
import subprocess
import sys
import tarfile
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
APIDIFF = "golang.org/x/exp/cmd/apidiff@v0.0.0-20261014143641-f0251eb4aa3e"
RELEASE = re.compile(r"^v1\.(\d+)\.(\d+)$")
SECTIONS = {"Incompatible changes:": "incompatible", "Compatible changes:": "compatible"}
# The module's gated packages, by directory: "" is the root.
PACKAGES = ("", "agentproc")
# The packages whose changes are reported and not gated, by directory.
REPORTED: tuple[str, ...] = ()


def fail(msg: str) -> None:
    """Report a tool error and exit 2."""
    print(f"check_api: {msg}", file=sys.stderr)
    sys.exit(2)


def run(args: list[str], cwd: Path) -> bytes:
    """Run args in cwd and return its output, or exit 2 naming what failed."""
    p = subprocess.run(args, cwd=cwd, capture_output=True, check=False)
    if p.returncode != 0:
        fail(f"{' '.join(args)} failed:\n{p.stderr.decode(errors='replace')}")
    return p.stdout


def module_path(root: Path) -> str:
    """The module path, from root's go.mod module line."""
    for line in (root / "go.mod").read_text(encoding="utf-8").splitlines():
        if line.startswith("module "):
            return line.split()[1]
    fail("go.mod has no module line")
    return ""


def latest_release(root: Path) -> str:
    """The newest v1.X.Y tag of root's repository, by number."""
    tags = run(["git", "tag", "--list", "v1.*"], root).decode().split()
    releases = sorted((tuple(int(n) for n in m.groups()), t) for t in tags if (m := RELEASE.match(t)))
    if not releases:
        fail("no v1.X.Y tag to compare against; fetch the tags, or pass BASE")
    return releases[-1][1]


def parse(report: str) -> dict[str, list[str]]:
    """Each change of apidiff's report, by section."""
    changes: dict[str, list[str]] = {"incompatible": [], "compatible": []}
    section = ""
    for line in report.splitlines():
        if not line.strip():
            continue
        if line in SECTIONS:
            section = SECTIONS[line]
        elif line.startswith("- ") and section:
            changes[section].append(line[2:])
        else:
            fail(f"apidiff's report has a line this script does not recognise: {line!r}")
    return changes


def has_package(directory: Path) -> bool:
    """Whether directory holds a Go package: a .go file that is not a test."""
    return any(not f.name.endswith("_test.go") for f in directory.glob("*.go"))


def compare(root: Path, base: str, gated: tuple[str, ...], reported: tuple[str, ...]) -> int:
    """Compare the packages of root's working tree with base's, print the
    report, and return the exit code: 1 when a gated package has an
    incompatible change."""
    if both := sorted(set(gated) & set(reported)):
        fail(f"{both} are both gated and reported")
    run(["git", "rev-parse", "--verify", "--quiet", f"{base}^{{commit}}"], root)
    module = module_path(root)
    changes: dict[str, list[str]] = {"incompatible": [], "compatible": []}
    reported_incompatible: list[str] = []
    with tempfile.TemporaryDirectory() as tmp:
        work = Path(tmp)
        old_tree = work / "base"
        with tarfile.open(fileobj=io.BytesIO(run(["git", "archive", base], root))) as tar:
            tar.extractall(old_tree, filter="data")
        for n, rel in enumerate(gated + reported):
            path = f"{module}/{rel}" if rel else module
            if not has_package(root / rel):
                fail(f"{path} is missing from the working tree")
            if not has_package(old_tree / rel):
                (old_tree / rel).mkdir(parents=True, exist_ok=True)
                (old_tree / rel / "doc.go").write_text(f"package {rel.rsplit('/', 1)[-1]}\n", encoding="utf-8")
            run(["go", "run", APIDIFF, "-w", str(work / f"base{n}.api"), path], old_tree)
            run(["go", "run", APIDIFF, "-w", str(work / f"tree{n}.api"), path], root)
            report = run(["go", "run", APIDIFF, str(work / f"base{n}.api"), str(work / f"tree{n}.api")], root).decode()
            for kind, found in parse(report).items():
                named = [f"{rel}: {c}" if rel else c for c in found]
                if kind == "incompatible" and rel in reported:
                    reported_incompatible += named
                else:
                    changes[kind] += named
    for kind in ("incompatible", "compatible"):
        for change in changes[kind]:
            print(f"check_api: {kind}: {change}")
    for change in reported_incompatible:
        print(f"check_api: incompatible, reported, not gated: {change}")
    print(f"check_api: against {base}: {len(changes['incompatible'])} incompatible, {len(changes['compatible'])} compatible")
    if reported:
        print(f"check_api: reported packages {', '.join(reported)}: {len(reported_incompatible)} incompatible, not gated")
    return 1 if changes["incompatible"] else 0


def main(argv: list[str]) -> int:
    if len(argv) > 1:
        fail("usage: check_api.py [BASE]")
    base = argv[0] if argv else latest_release(ROOT)
    return compare(ROOT, base, PACKAGES, REPORTED)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

"""Run planted breaks (docs/decisions/0031-MADR-verification-tools-in-repository.md):
each in its own scratch copy of the tree, outside the repository, never in it.

Usage: plants.py SPEC.json [--tree DIR] [--keep] [--out DIR]

SPEC.json is a list of plants:
  {"name": "...",
   "edits": [{"file": "agentproc/agentproc.go", "old": "...", "new": "..."}],
   "pkg": "./agentproc", "run": "^TestKill$", "want": "Kill took longer than",
   "host": "windows",            optional: run it there, by remote.py pkg
   "timeout": "300s"}            optional: go test's -timeout, default 300s
Each edit's "old" must occur exactly once in its file, or the plant is refused.
A plant fails as required when its test exits non-zero and its output holds
"want". The tree is the repository, or --tree DIR.

Exit 0 only when every plant failed as required.
"""
from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402

TOOLS = Path(__file__).resolve().parent
IGNORE = shutil.ignore_patterns(".git", ".cache", ".tools", ".gocache", "._*", ".DS_Store")
KEYS = {"name", "edits", "pkg", "run", "want"}


class Refused(Exception):
    """A plant that cannot be applied as written."""


def load(spec: Path) -> list[dict]:
    plants = json.loads(spec.read_text(encoding="utf-8"))
    if not isinstance(plants, list) or not plants:
        raise Refused(f"{spec}: want a non-empty list of plants")
    for i, p in enumerate(plants):
        missing = KEYS - set(p)
        if missing:
            raise Refused(f"{spec}: plant {i} lacks {', '.join(sorted(missing))}")
        if p.get("host") not in (None, *_common.HOST_VARS):
            raise Refused(f"{spec}: plant {p['name']!r}: host must be one of {', '.join(_common.HOST_VARS)}")
    return plants


def apply(copy: Path, edits: list[dict]) -> None:
    for e in edits:
        f = copy / e["file"]
        text = f.read_text(encoding="utf-8")
        n = text.count(e["old"])
        if n != 1:
            raise Refused(f"{e['file']}: the text to replace occurs {n} time(s), not once")
        f.write_text(text.replace(e["old"], e["new"]), encoding="utf-8")


def first_failure(text: str) -> str:
    """The first line a failing test wrote, else its --- FAIL line, else the
    output's last line."""
    for mark in ("_test.go:", "--- FAIL"):
        for line in text.splitlines():
            if mark in line:
                return line.strip()
    lines = text.strip().splitlines()
    return lines[-1].strip() if lines else ""


def run_plant(p: dict, tree: Path, work: Path, log: Path) -> tuple[bool, str]:
    copy = work / "".join(c if c.isalnum() else "-" for c in p["name"])
    if copy.exists():
        shutil.rmtree(copy)
    shutil.copytree(tree, copy, symlinks=True, ignore=IGNORE)
    try:
        apply(copy, p["edits"])
    except Refused as err:
        return False, f"refused: {err}"
    timeout = p.get("timeout", "300s")
    if p.get("host"):
        cmd = [sys.executable, "-B", str(TOOLS / "remote.py"), "pkg", "--host", p["host"], "--tree", str(copy), p["pkg"],
               f"-test.run={p['run']}", "-test.count=1", f"-test.timeout={timeout}"]
        status, text = _common.run(log, cmd, cwd=copy)
    else:
        status, text = _common.run(log, ["go", "test", "-count=1", f"-timeout={timeout}", "-run", p["run"], p["pkg"]], cwd=copy,
                                   env=dict(os.environ))
    return status != 0 and p["want"] in text, f"exit {status}: {first_failure(text)[:240]}"


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0], formatter_class=argparse.RawDescriptionHelpFormatter, epilog=__doc__)
    ap.add_argument("spec")
    ap.add_argument("--tree", help="the tree to copy (default: the repository)")
    ap.add_argument("--keep", action="store_true", help="keep the scratch copies")
    ap.add_argument("--out", help="the output directory, for the log (default .cache/tools/plants)")
    args = ap.parse_args(argv)
    try:
        plants = load(Path(args.spec))
    except Refused as err:
        print(f"plants: {err}")
        return 2
    tree = Path(args.tree).resolve() if args.tree else _common.repo()
    out = _common.out_dir("plants", args.out)
    log = out / "plants.log"
    log.unlink(missing_ok=True)
    work = _common.cache_dir("plants") / f"run-{os.getpid()}"
    work.mkdir(parents=True, exist_ok=True)
    held = 0
    try:
        for p in plants:
            ok, detail = run_plant(p, tree, work, log)
            held += ok
            where = f" on {p['host']}" if p.get("host") else ""
            print(f"{'FAILED AS REQUIRED' if ok else 'NOT AS REQUIRED'}: {p['name']} ({p['run']}{where}): {detail}", flush=True)
    finally:
        if not args.keep:
            shutil.rmtree(work, ignore_errors=True)
    print(f"{held}/{len(plants)} as required; log {log}")
    return 0 if held == len(plants) else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

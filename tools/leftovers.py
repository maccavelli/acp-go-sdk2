"""Check the Windows test host for test binaries left running
(docs/decisions/0031-MADR-verification-tools-in-repository.md).

It lists the host's processes with `tasklist` over SSH (ACP_WINDOWS_HOST) and
matches each row's image name exactly, never as a substring: a Go test binary
(a name ending in .test.exe), or a name --name gives.

Usage: leftovers.py [--name IMAGE ...]
Exit 0 when none is left, 1 when one is, 2 when the list could not be read.
"""
from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402


def leftover_rows(tasklist: str, names: set[str]) -> list[str]:
    """The rows of tasklist's output whose image name, its first field, is a
    test binary's or one of names, compared whole and without case."""
    wanted = {n.lower() for n in names}
    rows = []
    for line in tasklist.splitlines():
        fields = line.split()
        if not fields:
            continue
        image = fields[0].lower()
        if image.endswith(".test.exe") or image in wanted:
            rows.append(line.rstrip())
    return rows


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--name", action="append", default=[], help="another image name to match exactly")
    args = ap.parse_args(argv)
    host = _common.host("windows")
    r = subprocess.run(["ssh", "-o", "BatchMode=yes", host, "tasklist"], capture_output=True, text=True, timeout=120)
    rows = [l for l in r.stdout.splitlines() if l.strip()]
    if r.returncode != 0 or not rows:
        print(f"leftovers: tasklist failed (exit {r.returncode}): {r.stderr.strip()[:200]}")
        return 2
    left = leftover_rows(r.stdout, set(args.name))
    print(f"leftovers: {len(rows)} rows; test processes left: {len(left)}")
    for row in left:
        print("  " + row)
    return 1 if left else 0


if __name__ == "__main__":
    sys.exit(_common.main_guard(main, sys.argv[1:]))

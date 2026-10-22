"""List running processes without pgrep
(docs/decisions/0031-MADR-verification-tools-in-repository.md).

Usage: procs.py TEXT...
Prints each process whose command line contains any of the texts, this
process, its parent and its children left out. Exit 0 when none, 1 when some,
2 on a usage error.
"""
from __future__ import annotations

import os
import subprocess
import sys


def matching(needles: list[str]) -> list[str]:
    out = subprocess.run(["ps", "-axo", "pid=,ppid=,command="], capture_output=True, text=True, check=True).stdout
    mine = {os.getpid(), os.getppid()}
    found = []
    for line in out.splitlines():
        pid, _, rest = line.strip().partition(" ")
        ppid, _, command = rest.strip().partition(" ")
        if int(pid) in mine or int(ppid) == os.getpid():
            continue
        if any(n in command for n in needles):
            found.append(f"{pid} {command.strip()[:200]}")
    return found


def main(argv: list[str]) -> int:
    if not argv or argv[0] in ("-h", "--help"):
        print(__doc__)
        return 0 if argv else 2
    found = matching(argv)
    print("\n".join(found) if found else "none")
    return 1 if found else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

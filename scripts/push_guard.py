#!/usr/bin/env python3
"""Run the global pre-push disclosure guard before asking the owner to push.

Pushes to GitHub pass through a global pre-push guard, which refuses a push
when an outgoing commit carries an entry of its deny list (AGENTS.md,
"Identifiers"). This script runs that guard over the commits a push of BRANCH
to REMOTE would send, so a refusal is found before the owner pushes:

  python3 scripts/push_guard.py [--remote origin] [--branch main]

It gives the guard the line git's pre-push hook would: the branch's local
commit and the remote-tracking ref's. Fetch first, so that ref is current. It
exits with the guard's status, and 1 when the guard is missing or the branch
or its remote-tracking ref does not exist. It reads the repository and writes
nothing.
"""

from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path

GUARD = Path.home() / ".global-git-hooks" / "github-disclosure.py"


def git(root: Path, *args: str) -> str:
    """Run git in root and return its output, or exit 1 naming what failed."""
    p = subprocess.run(["git", "-C", str(root), *args], capture_output=True, text=True, check=False)
    if p.returncode != 0:
        sys.exit(f"push-guard: git {' '.join(args)}: {p.stderr.strip()}")
    return p.stdout.strip()


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--remote", default="origin")
    ap.add_argument("--branch", default="main")
    ap.add_argument("--guard", type=Path, default=GUARD, help="the guard script (default: %(default)s)")
    args = ap.parse_args()

    root = Path(git(Path(__file__).resolve().parent, "rev-parse", "--show-toplevel"))
    if not args.guard.is_file():
        print(f"push-guard: BLOCKED: the guard {args.guard} is missing; nothing was checked.", file=sys.stderr)
        return 1
    local = git(root, "rev-parse", "--verify", f"refs/heads/{args.branch}^{{commit}}")
    remote = git(root, "rev-parse", "--verify", f"refs/remotes/{args.remote}/{args.branch}^{{commit}}")
    url = git(root, "remote", "get-url", args.remote)
    count = git(root, "rev-list", "--count", f"{remote}..{local}")
    print(f"push-guard: {count} outgoing commit(s), {remote[:7]}..{local[:7]}, {args.branch} to {args.remote}")
    if "github.com" not in url:
        print(f"push-guard: {args.remote} is not on github.com; the guard checks only GitHub pushes.")

    line = f"refs/heads/{args.branch} {local} refs/heads/{args.branch} {remote}\n"
    p = subprocess.run([sys.executable, str(args.guard), "pre-push", args.remote, url], cwd=root, input=line,
                       text=True, check=False)
    print(f"push-guard: the guard {'passed' if p.returncode == 0 else 'REFUSED the push'} (exit {p.returncode}).")
    return p.returncode


if __name__ == "__main__":
    sys.exit(main())

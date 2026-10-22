"""Wait for a commit's CI run and watch it to its end
(docs/decisions/0031-MADR-verification-tools-in-repository.md).

Usage: watch_ci.py REV [--minutes N]
REV is resolved to its full commit id first, so a short id finds its run. It
waits up to N minutes (20) for the run to appear, watches it, prints each job's
conclusion, and exits 0 when the run succeeded, 1 when it did not, 2 when no
run appeared.
"""
from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402


def full_sha(root: Path, rev: str) -> str:
    out = subprocess.run(["git", "rev-parse", "--verify", f"{rev}^{{commit}}"], cwd=root, capture_output=True, text=True)
    if out.returncode:
        raise SystemExit(f"watch_ci: {rev!r} is not a commit")
    return out.stdout.strip()


def find_run(runs: list[dict], sha: str) -> int | None:
    for r in runs:
        if r.get("headSha") == sha:
            return r["databaseId"]
    return None


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("rev")
    ap.add_argument("--minutes", type=int, default=20)
    args = ap.parse_args(argv)
    root = _common.repo(Path.cwd())
    sha, repo = full_sha(root, args.rev), _common.origin_repo(root)
    run_id = None
    deadline = time.monotonic() + 60 * args.minutes
    while run_id is None and time.monotonic() < deadline:
        r = subprocess.run(["gh", "run", "list", "--repo", repo, "--commit", sha, "--limit", "10", "--json", "databaseId,headSha"],
                           capture_output=True, text=True, timeout=120)
        if r.returncode == 0:
            run_id = find_run(json.loads(r.stdout), sha)
        if run_id is None:
            time.sleep(10)
    if run_id is None:
        print(f"watch_ci: no CI run for {sha[:7]} within {args.minutes} minutes")
        return 2
    print(f"watch_ci: run {run_id} for {sha[:7]}", flush=True)
    subprocess.run(["gh", "run", "watch", str(run_id), "--repo", repo, "--exit-status", "--interval", "30"], capture_output=True, timeout=7200)
    v = subprocess.run(["gh", "run", "view", str(run_id), "--repo", repo, "--json", "conclusion,status,jobs,url"], capture_output=True, text=True,
                       timeout=120)
    info = json.loads(v.stdout)
    print(f"watch_ci: run {info['status']} {info['conclusion']}; {info['url']}")
    for job in info["jobs"]:
        print(f"  {job['conclusion'] or job['status']}: {job['name']}")
    return 0 if info["conclusion"] == "success" else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

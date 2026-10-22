"""Wait for a commit's CI runs and watch them to their end
(docs/decisions/0031-MADR-verification-tools-in-repository.md, and its
"Amendment 2026-10-22, after step 8").

Usage: watch_ci.py REV [--branch B] [--minutes N] [--more S] [--settle S] [--every S]

REV is resolved to its full commit id first, so a short id finds its runs. It
waits up to N minutes (20) for the commit's first run, then S seconds more
(30) for others, such as a tag's pushed after its branch; every run of the
commit is watched, or with --branch, that branch's alone. After each watch it
reads the run again until every job is "completed", up to --settle seconds
(180), every --every (10), and prints the run and each job as they then are.

Exit 0 only when every run, and every job of each, succeeded (a job may be
skipped); 1 when one did not, or did not settle or finish in time; 2 when no
run appeared.
"""
from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
from pathlib import Path
from typing import Callable

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402

PASSING = ("success", "skipped")
WATCH_TIMEOUT = 7200


def full_sha(root: Path, rev: str) -> str:
    out = subprocess.run(["git", "rev-parse", "--verify", f"{rev}^{{commit}}"], cwd=root, capture_output=True, text=True)
    if out.returncode:
        raise SystemExit(f"watch_ci: {rev!r} is not a commit")
    return out.stdout.strip()


def runs_for(runs: list[dict], sha: str, branch: str | None = None) -> list[dict]:
    """Every run of the commit, in the order given (gh lists the newest
    first), those of branch alone when it is given."""
    return [r for r in runs if r.get("headSha") == sha and (branch is None or r.get("headBranch") == branch)]


def settled(jobs: list[dict]) -> bool:
    """Every job's status is "completed"."""
    return bool(jobs) and all(j.get("status") == "completed" for j in jobs)


def verdict(run: dict, jobs: list[dict]) -> bool:
    """Success only when the run's conclusion and every job's is success,
    a job's skipped too."""
    return run.get("conclusion") == "success" and bool(jobs) and all(j.get("conclusion") in PASSING for j in jobs)


def wait_settled(fetch: Callable[[], dict], bound: float, every: float, clock: Callable[[], float] = time.monotonic,
                 sleep: Callable[[float], None] = time.sleep) -> tuple[dict, bool]:
    """Call fetch until the run it returns has settled jobs, or bound seconds
    have passed; return the last read and whether it settled."""
    end = clock() + bound
    while True:
        info = fetch()
        if info.get("status") == "completed" and settled(info.get("jobs", [])):
            return info, True
        if clock() >= end:
            return info, False
        sleep(every)


def watch(cmd: list[str], timeout: float = WATCH_TIMEOUT, runner: Callable[..., subprocess.CompletedProcess] = subprocess.run) -> tuple[int | None, str]:
    """Run `gh run watch`; its exit status, or None and why when it did not
    finish within timeout."""
    try:
        return runner(cmd, capture_output=True, timeout=timeout).returncode, ""
    except subprocess.TimeoutExpired:
        return None, f"did not finish within {timeout:g} s"


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0], formatter_class=argparse.RawDescriptionHelpFormatter, epilog=__doc__)
    ap.add_argument("rev")
    ap.add_argument("--branch", help="only this branch's runs")
    ap.add_argument("--minutes", type=float, default=20, help="how long to wait for the first run (20)")
    ap.add_argument("--more", type=float, default=30, help="seconds to wait for further runs once one has appeared (30)")
    ap.add_argument("--settle", type=float, default=180, help="seconds to wait for every job to settle (180)")
    ap.add_argument("--every", type=float, default=10, help="seconds between reads while settling (10)")
    args = ap.parse_args(argv)
    root = _common.repo(Path.cwd())
    sha, repo = full_sha(root, args.rev), _common.origin_repo(root)

    def listed() -> list[dict]:
        r = subprocess.run(["gh", "run", "list", "--repo", repo, "--commit", sha, "--limit", "20", "--json",
                            "databaseId,headSha,headBranch,event,workflowName"], capture_output=True, text=True, timeout=120)
        return runs_for(json.loads(r.stdout), sha, args.branch) if r.returncode == 0 else []

    deadline = time.monotonic() + 60 * args.minutes
    runs = listed()
    while not runs and time.monotonic() < deadline:
        time.sleep(10)
        runs = listed()
    if not runs:
        print(f"watch_ci: no CI run for {sha[:7]}{' on ' + args.branch if args.branch else ''} within {args.minutes:g} minutes")
        return 2
    time.sleep(args.more)
    seen = {r["databaseId"] for r in runs}
    runs += [r for r in listed() if r["databaseId"] not in seen]
    print(f"watch_ci: {len(runs)} run(s) for {sha[:7]}: {', '.join(str(r['databaseId']) for r in runs)}", flush=True)

    ok = True
    for r in runs:
        rid = str(r["databaseId"])
        status, why = watch(["gh", "run", "watch", rid, "--repo", repo, "--exit-status", "--interval", "30"])

        def fetch() -> dict:
            v = subprocess.run(["gh", "run", "view", rid, "--repo", repo, "--json", "status,conclusion,jobs,url,headBranch,event"],
                               capture_output=True, text=True, timeout=120)
            return json.loads(v.stdout) if v.returncode == 0 else {}

        info, calm = wait_settled(fetch, args.settle, args.every)
        good = status == 0 and calm and verdict(info, info.get("jobs", []))
        ok &= good
        print(f"watch_ci: run {rid} ({info.get('headBranch', r.get('headBranch'))}, {info.get('event', r.get('event'))}): "
              f"{info.get('status')} {info.get('conclusion')}; watch exit {status if status is not None else why}"
              f"{'' if calm else f'; jobs not settled after {args.settle:g} s'}; {info.get('url', '')}")
        for job in info.get("jobs", []):
            print(f"  {job.get('status')} {job.get('conclusion') or ''}: {job.get('name')}")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(_common.main_guard(main, sys.argv[1:]))

"""Read CI's job history from GitHub Actions, every log by its job id
(docs/decisions/0031-MADR-verification-tools-in-repository.md, "Amendment
2026-10-22, after step 11: CI's job history, read by attempt").

Usage:
  ci_jobs.py tally [--branch B] [--runs N] [--created RANGE] [--markdown]
      every job's first attempt in the N (40) most recent runs of B (main),
      counted by job, a Go version's patch made x, and by conclusion; each
      job that did not succeed listed with its failing step
  ci_jobs.py failure RUN [--attempt N] [--out DIR]
      each failed job of every attempt, or of attempt N: its failing step,
      the shuffle seed, the first fatal error or panic with the runtime:
      lines before it, and the FAIL lines; the whole log written to
      .cache/tools/ci_jobs/
  ci_jobs.py env RUN VAR [--attempt N] [--expect JOB=VALUE]... [--others VALUE]
      VAR's values in each step's env: block, per job of attempt N, by
      default the run's latest; with --expect, each named job's values must
      be exactly VALUE, and with --others, every other job's

A job's log is read from the REST endpoint actions/jobs/<id>/logs, which is
that job's own attempt; `gh run view --job ID --log` reads the run's latest
attempt whatever the id, so after a rerun it shows the rerun.

Exit 0 when every read succeeded and every expectation held; 1 when a read
failed or an expectation did not hold; 2 on a usage error.
"""
from __future__ import annotations

import argparse
import collections
import json
import re
import subprocess
import sys
from pathlib import Path
from typing import Callable

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402

Api = Callable[[str], str]

PATCH = re.compile(r"\b(\d+\.\d+)\.\d+\b")
# A log line's timestamp, and the byte-order mark GitHub puts before some.
STAMP = re.compile(r"^\ufeff?\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z ?")
ENV_LINE = re.compile(r"^  ([^:\s]+): ?(.*)$")
COLUMNS = ("success", "failure", "cancelled")
TEST_FAILS = 20


class ReadError(Exception):
    """A read from GitHub that failed."""


def gh_api(path: str) -> str:
    """GET path through `gh api`; a log holds terminal escape sequences, which
    gh refuses to print without --allow-escape-sequences."""
    p = subprocess.run(["gh", "api", "--allow-escape-sequences", path], capture_output=True, timeout=600)
    if p.returncode:
        raise ReadError(f"gh api {path}: exit {p.returncode}: {p.stderr.decode(errors='replace').strip()[:300]}")
    return p.stdout.decode("utf-8", errors="replace")


def jobs_of(api: Api, repo: str, run: int | str, attempt: int) -> list[dict]:
    """The jobs of one attempt of a run."""
    return json.loads(api(f"repos/{repo}/actions/runs/{run}/attempts/{attempt}/jobs?per_page=100"))["jobs"]


def job_log(api: Api, repo: str, job_id: int | str) -> str:
    """A job's log, by its id, so of the attempt the id belongs to."""
    return api(f"repos/{repo}/actions/jobs/{job_id}/logs")


def latest_attempt(api: Api, repo: str, run: int | str) -> int:
    """The run's latest attempt."""
    return int(json.loads(api(f"repos/{repo}/actions/runs/{run}"))["run_attempt"])


def first_attempts(api: Api, repo: str, runs: list[dict]) -> list[tuple[dict, list[dict]]]:
    """Each run with the jobs of its first attempt, whatever its latest: a
    rerun that passed hides nothing."""
    return [(r, jobs_of(api, repo, r["databaseId"], 1)) for r in runs]


def group(name: str) -> str:
    """A job's name with a Go version's patch made x, so that the jobs of one
    system and one minor version count together across patch releases."""
    return PATCH.sub(r"\1.x", name)


def failing_step(job: dict) -> str:
    """The name of the job's first failed step, "no step ran" when it has no
    steps (cancelled while queued), or "no step failed"."""
    steps = job.get("steps") or []
    if not steps:
        return "no step ran"
    return next((s["name"] for s in steps if s.get("conclusion") == "failure"), "no step failed")


def tally(runs: list[tuple[dict, list[dict]]]) -> tuple[dict[str, collections.Counter], list[dict]]:
    """Each group's job count and count per conclusion, and each job that did
    not succeed."""
    counts: dict[str, collections.Counter] = collections.defaultdict(collections.Counter)
    notable = []
    for run, jobs in runs:
        for j in jobs:
            g, c = group(j["name"]), j.get("conclusion") or j.get("status") or "unknown"
            counts[g]["jobs"] += 1
            counts[g][c] += 1
            if c != "success":
                notable.append({"run": run["databaseId"], "commit": run["headSha"][:7], "created": run["createdAt"],
                                "job": j["name"], "conclusion": c, "step": failing_step(j)})
    return dict(counts), notable


def markdown(counts: dict[str, collections.Counter]) -> str:
    """The groups as a table in the report's columns, a column more for any
    other conclusion."""
    others = sorted({k for c in counts.values() for k in c} - {"jobs", *COLUMNS})
    head = ["Job", "Jobs", *(c.capitalize() for c in COLUMNS), *(o.replace("_", " ").capitalize() for o in others)]
    rows = ["| " + " | ".join(head) + " |", "| " + " | ".join(":---" for _ in head) + " |"]
    for g in sorted(counts):
        c = counts[g]
        rows.append("| " + " | ".join([g, str(c["jobs"]), *(str(c[k]) for k in (*COLUMNS, *others))]) + " |")
    return "\n".join(rows)


def strip(line: str) -> str:
    """A log line without its timestamp."""
    return STAMP.sub("", line).rstrip("\r")


def env_values(log: str, var: str) -> list[str]:
    """var's value in each step's env: block, from the line `env:` to the
    group's end; nothing outside a block counts."""
    values, inside = [], False
    for raw in log.splitlines():
        line = strip(raw)
        if line == "env:":
            inside = True
            continue
        if inside:
            m = ENV_LINE.match(line)
            if not m:
                inside = False
                continue
            if m.group(1) == var:
                values.append(m.group(2).rstrip())
    return values


def failure_lines(log: str) -> list[str]:
    """What a crash's record takes from a failed job's log: the shuffle seed,
    the runtime: lines before the first fatal error or panic, up to five, and
    that line; the failed tests (the first 20), the failed packages, and the
    runner's errors."""
    lines = [strip(raw) for raw in log.splitlines()]
    out = [x for x in lines if x.startswith("-test.shuffle ")]
    for i, x in enumerate(lines):
        if x.startswith(("fatal error:", "panic:")):
            out += [y for y in lines[:i] if y.startswith("runtime:")][-5:] + [x]
            break
    out += [x for x in lines if x.startswith("--- FAIL")][:TEST_FAILS]
    out += [x for x in lines if x.startswith("FAIL\t")]
    out += [x for x in lines if x.startswith("##[error]")]
    return out


def holds(values: dict[str, list[str]], expect: dict[str, str], others: str | None) -> list[str]:
    """What does not hold: each named job's distinct values exactly its
    expected value, and, with others, every other job's exactly that."""
    problems = []
    for job, want in expect.items():
        if job not in values:
            problems.append(f"{job}: not in the attempt")
        elif sorted(set(values[job])) != [want]:
            problems.append(f"{job}: values {sorted(set(values[job]))}, want [{want!r}]")
    if others is not None:
        for job, got in values.items():
            if job not in expect and sorted(set(got)) != [others]:
                problems.append(f"{job}: values {sorted(set(got))}, want [{others!r}]")
    return problems


def run_list(repo: str, branch: str, limit: int, created: str | None) -> list[dict]:
    cmd = ["gh", "run", "list", "--repo", repo, "--branch", branch, "--limit", str(limit), "--json",
           "databaseId,headSha,createdAt,status,attempt"]
    if created:
        cmd += ["--created", created]
    p = subprocess.run(cmd, capture_output=True, text=True, timeout=600)
    if p.returncode:
        raise ReadError(f"gh run list: exit {p.returncode}: {p.stderr.strip()[:300]}")
    return json.loads(p.stdout)


def do_tally(args: argparse.Namespace, repo: str, api: Api) -> int:
    listed = run_list(repo, args.branch, args.runs, args.created)
    runs = [r for r in listed if r.get("status") == "completed"]
    left = [r for r in listed if r.get("status") != "completed"]
    if not runs:
        print(f"ci_jobs: no completed run of {args.branch}")
        return 1
    created = sorted(r["createdAt"] for r in runs)
    print(f"ci_jobs: {len(runs)} run(s) of {args.branch}, created {created[0]} to {created[-1]}; first attempts")
    if left:
        named = ", ".join(f"{r['databaseId']} ({r.get('status')})" for r in left)
        print(f"  left out, not completed: {named}")
    counts, notable = tally(first_attempts(api, repo, runs))
    for g in sorted(counts):
        c = counts[g]
        print(f"  {g}: {c['jobs']} job(s): " + ", ".join(f"{k} {v}" for k, v in sorted(c.items()) if k != "jobs"))
    print("not succeeded, first attempts:" if notable else "every job of every first attempt succeeded")
    for n in notable:
        print(f"  run {n['run']}, {n['commit']}, {n['created']}: {n['job']}: {n['conclusion']}, {n['step']}")
    if args.markdown:
        print()
        print(markdown(counts))
    return 0


def do_failure(args: argparse.Namespace, repo: str, api: Api) -> int:
    latest = latest_attempt(api, repo, args.run)
    attempts = [args.attempt] if args.attempt else list(range(1, latest + 1))
    out = _common.out_dir("ci_jobs", args.out)
    for a in attempts:
        failed = [j for j in jobs_of(api, repo, args.run, a) if j.get("conclusion") == "failure"]
        print(f"ci_jobs: run {args.run}, attempt {a} of {latest}: " + (f"{len(failed)} failed job(s)" if failed else "no failed job"))
        for j in failed:
            log = job_log(api, repo, j["id"])
            path = out / f"{args.run}-{a}-{j['id']}.log"
            path.write_text(log, encoding="utf-8")
            print(f"  {j['name']}, job {j['id']}: step \"{failing_step(j)}\"; {j.get('html_url', '')}")
            for line in failure_lines(log):
                print(f"    {line}")
            print(f"    log: {path}")
    return 0


def do_env(args: argparse.Namespace, repo: str, api: Api) -> int:
    expect = {}
    for e in args.expect:
        job, sep, value = e.partition("=")
        if not sep:
            raise SystemExit(f"ci_jobs: --expect {e!r} is not JOB=VALUE")
        expect[job] = value
    attempt = args.attempt or latest_attempt(api, repo, args.run)
    values = {}
    for j in jobs_of(api, repo, args.run, attempt):
        values[j["name"]] = env_values(job_log(api, repo, j["id"]), args.var)
        print(f"  {j['name']}: {args.var} in {len(values[j['name']])} env block(s), values {sorted(set(values[j['name']]))}")
    print(f"ci_jobs: run {args.run}, attempt {attempt}: {len(values)} job(s)")
    if not expect and args.others is None:
        return 0
    problems = holds(values, expect, args.others)
    for p in problems:
        print(f"  does not hold: {p}")
    print("holds" if not problems else "does NOT hold")
    return 1 if problems else 0


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0], formatter_class=argparse.RawDescriptionHelpFormatter, epilog=__doc__)
    sub = ap.add_subparsers(dest="cmd", required=True)
    t = sub.add_parser("tally", help="every job's first attempt, counted")
    t.add_argument("--branch", default="main", help="the runs of this branch (main)")
    t.add_argument("--runs", type=int, default=40, help="how many of the most recent runs (40)")
    t.add_argument("--created", help="gh run list's --created, as given, such as '<2026-10-22T20:00:00Z'")
    t.add_argument("--markdown", action="store_true", help="also print the groups as a table")
    f = sub.add_parser("failure", help="each failed job's step, seed and crash")
    f.add_argument("run")
    f.add_argument("--attempt", type=int, help="this attempt alone; by default every attempt")
    f.add_argument("--out", help="where to write the logs (.cache/tools/ci_jobs/)")
    e = sub.add_parser("env", help="a variable's value in each step's environment, per job")
    e.add_argument("run")
    e.add_argument("var")
    e.add_argument("--attempt", type=int, help="this attempt; by default the run's latest")
    e.add_argument("--expect", action="append", default=[], metavar="JOB=VALUE", help="the named job's only value")
    e.add_argument("--others", metavar="VALUE", help="every other job's only value ('' for the empty value)")
    args = ap.parse_args(argv)
    repo = _common.origin_repo(_common.repo(Path.cwd()))
    try:
        return {"tally": do_tally, "failure": do_failure, "env": do_env}[args.cmd](args, repo, gh_api)
    except ReadError as err:
        print(f"ci_jobs: {err}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(_common.main_guard(main, sys.argv[1:]))

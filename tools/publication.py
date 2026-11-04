"""Check a release's publication on GitHub and the Go module proxy
(docs/decisions/0031-MADR-verification-tools-in-repository.md): ten checks.

Usage: publication.py VERSION COMMIT BODY
  VERSION the tag, COMMIT the commit it must name (a prefix will do), BODY
  the file the release's body must equal (release_body.py writes it).

It reads GitHub (gh, the origin remote's repository) and the proxy, and writes
only a module cache, in the tool's cache directory, removed after. Its two CI
checks read the CI workflow's runs alone, ci.yml's: for main's head, its own,
from any ref, or its nearest ancestor's that every later change leaves
docs-safe; for the tag, the tag's (0049-PLAN-tooling-fixes.md, phase 1, T2).
Exit 0 only when every check holds.
"""
from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402


def same_body(published: str, expected: str) -> bool:
    """The release's body equals the file's, but for surrounding whitespace."""
    return published.strip() == expected.strip()


#: The CI workflow, .github/workflows/ci.yml, which scripts/check_ci_filters.py
#: pins: the one that runs Go. The Docs workflow's runs are not CI's.
CI_WORKFLOW = "ci.yml"


def ci_runs_command(repo: str, commit: str | None = None, branch: str | None = None) -> list[str]:
    """gh's listing of the CI workflow's runs, newest first: of a commit, from
    any ref, the tag's included, or of a branch or tag
    (0049-PLAN-tooling-fixes.md, phase 1, T2)."""
    cmd = ["gh", "run", "list", "--repo", repo, "--workflow", CI_WORKFLOW, "--json",
           "conclusion,status,headBranch,headSha,databaseId", "--limit", "20"]
    if commit:
        cmd += ["--commit", commit]
    if branch:
        cmd += ["--branch", branch]
    return cmd


def ci_of_head(head: str, runs_of, ancestors, docs_safe) -> tuple[dict | None, str | None]:
    """The CI run that stands for head, and the commit it ran on: head's own
    newest, from any ref; or, when head has none, the newest of the nearest
    first-parent ancestor from which every later change is docs-safe, since
    CI does not run for a docs-only push (0049-PLAN-tooling-fixes.md, phase
    1, T2; Q16 a). runs_of(commit) lists a commit's CI runs, ancestors(head)
    yields (commit, the paths changed from it to head), nearest first, and
    docs_safe(paths) answers as check_ci_filters.py --docs-safe."""
    runs = runs_of(head)
    if runs:
        return runs[0], head
    for commit, changed in ancestors(head):
        if not docs_safe(changed):
            return None, None
        runs = runs_of(commit)
        if runs:
            return runs[0], commit
    return None, None


def passed(r: dict | None) -> bool:
    """Whether run r completed with success."""
    return bool(r) and r["status"] == "completed" and r["conclusion"] == "success"


def contains(root: Path, ancestor: str, head: str) -> bool:
    """Whether commit head has ancestor in its history (the same commit
    included), fetching origin's main when head is not present locally."""
    if subprocess.run(["git", "cat-file", "-e", f"{head}^{{commit}}"], cwd=root, capture_output=True).returncode:
        subprocess.run(["git", "fetch", "-q", "origin", "main"], cwd=root, capture_output=True)
    return subprocess.run(["git", "merge-base", "--is-ancestor", ancestor, head], cwd=root, capture_output=True).returncode == 0


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("version")
    ap.add_argument("commit")
    ap.add_argument("body")
    args = ap.parse_args(argv)
    root = _common.repo(Path.cwd())
    mod, repo, tag, want = _common.module_path(root), _common.origin_repo(root), args.version, args.commit
    results: list[bool] = []

    def check(what: str, ok: bool, detail: str = "") -> None:
        results.append(ok)
        print(f"{'ok  ' if ok else 'MISS'} {what}{'  ' + detail if detail else ''}", flush=True)

    def run(*cmd: str, cwd: Path = root, env: dict[str, str] | None = None) -> tuple[int, str]:
        p = subprocess.run(cmd, cwd=cwd, env=env, capture_output=True, text=True, timeout=600)
        return p.returncode, (p.stdout + p.stderr).strip()

    rc, out = run("gh", "release", "view", tag, "--repo", repo, "--json", "isDraft,isPrerelease,body,tagName,name")
    rel = json.loads(out) if rc == 0 else {}
    check("release exists, not a draft or prerelease", rc == 0 and not rel["isDraft"] and not rel["isPrerelease"] and rel["name"] == tag,
          "" if rc == 0 else out[:120])
    check("its body is the extracted release text", rc == 0 and same_body(rel["body"], Path(args.body).read_text(encoding="utf-8")))
    rc, out = run("gh", "api", f"repos/{repo}/releases/latest", "--jq", ".tag_name")
    check("marked latest", rc == 0 and out == tag, out)

    rc, out = run("git", "ls-remote", "origin", f"refs/tags/{tag}^{{}}", f"refs/tags/{tag}", "refs/heads/main")
    refs = {line.split()[1]: line.split()[0] for line in out.splitlines() if len(line.split()) == 2}
    peeled = refs.get(f"refs/tags/{tag}^{{}}", "")
    check(f"tag {tag} is annotated and names {want}", peeled.startswith(want) and refs.get(f"refs/tags/{tag}") not in (None, peeled), peeled[:7])
    # What is published, not the local branch: that nothing is left unpushed
    # is make push-guard's (0031-PLAN, the deviation during step 6).
    origin_main = refs.get("refs/heads/main", "")
    check("the tag's commit is on origin's main", bool(peeled and origin_main) and contains(root, peeled, origin_main), origin_main[:7])
    def runs_of(commit: str) -> list[dict]:
        rc, out = run(*ci_runs_command(repo, commit=commit))
        return json.loads(out) if rc == 0 else []

    def ancestors(head: str):
        rc, out = run("git", "rev-list", "--first-parent", "--max-count=51", head)
        for commit in (out.split()[1:] if rc == 0 else []):
            rc, names = run("git", "diff", "--name-only", commit, head)
            yield commit, (names.split() if rc == 0 else ["<git diff failed>"])

    def docs_safe(paths: list[str]) -> bool:
        if not paths:
            return True
        rc, _ = run(sys.executable, "-B", str(root / "scripts" / "check_ci_filters.py"), "--docs-safe", *paths)
        return rc == 0

    m_run, on = ci_of_head(origin_main, runs_of, ancestors, docs_safe) if origin_main else (None, None)
    check("CI on origin main's head", passed(m_run),
          f"run {m_run['databaseId']} on {on[:7]}{', docs-safe since' if on != origin_main else ''}, {m_run['headBranch']}"
          if m_run and on else "no CI run of main's head, nor of an ancestor it changes only docs since")
    rc, out = run(*ci_runs_command(repo, branch=tag))
    runs = json.loads(out) if rc == 0 else []
    check("CI on the tag", bool(runs) and passed(runs[0]) and runs[0]["headSha"].startswith(want),
          f"run {runs[0]['databaseId']}, {runs[0]['headBranch']}" if runs else "no CI run of the tag")

    try:
        with urllib.request.urlopen(f"https://proxy.golang.org/{mod}/@v/{tag}.info", timeout=60) as r:
            info = json.loads(r.read())
        origin = info.get("Origin", {})
        check("proxy serves the .info, origin the tag at that commit", info.get("Version") == tag and
              origin.get("Ref") == f"refs/tags/{tag}" and origin.get("Hash", "").startswith(want), json.dumps(origin))
    except OSError as err:
        check("proxy serves the .info", False, str(err))

    scratch = _common.cache_dir("publication") / f"run-{os.getpid()}"
    (scratch / "mod").mkdir(parents=True)
    env = {k: v for k, v in os.environ.items() if k not in ("GONOSUMDB", "GONOSUMCHECK", "GOPRIVATE", "GOFLAGS")}
    env.update(GOPATH=str(scratch / "gopath"), GOMODCACHE=str(scratch / "gopath" / "pkg" / "mod"), GOPROXY="https://proxy.golang.org,direct",
               GOFLAGS="-mod=mod -modcacherw")
    try:
        run("go", "mod", "init", "example.invalid/consumer", cwd=scratch / "mod", env=env)
        rc, out = run("go", "list", "-m", f"{mod}@latest", cwd=scratch / "mod", env=env)
        check("go list -m @latest gives the tag", rc == 0 and out.endswith(tag), out)
        rc, out = run("go", "mod", "download", f"{mod}@{tag}", cwd=scratch / "mod", env=env)
        check("go mod download of the tag, checksum database on", rc == 0, out[-200:])
    finally:
        shutil.rmtree(scratch, ignore_errors=True)
    print(f"{sum(results)} of {len(results)} checks hold")
    return 0 if all(results) else 1


if __name__ == "__main__":
    sys.exit(_common.main_guard(main, sys.argv[1:]))

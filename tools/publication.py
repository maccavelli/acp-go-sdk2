"""Check a release's publication on GitHub and the Go module proxy
(docs/decisions/0031-MADR-verification-tools-in-repository.md): ten checks.

Usage: publication.py VERSION COMMIT BODY
  VERSION the tag, COMMIT the commit it must name (a prefix will do), BODY
  the file the release's body must equal (release_body.py writes it).

It reads GitHub (gh, the origin remote's repository) and the proxy, and writes
only a module cache, in the tool's cache directory, removed after.
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
    rc, out = run("gh", "run", "list", "--repo", repo, "--commit", origin_main, "--json", "conclusion,status", "--limit", "1")
    runs = json.loads(out) if rc == 0 and origin_main else []
    check("CI on origin main's head", bool(runs) and runs[0]["status"] == "completed" and runs[0]["conclusion"] == "success",
          json.dumps(runs[:1]))
    rc, out = run("gh", "run", "list", "--repo", repo, "--branch", tag, "--json", "conclusion,status,headSha,databaseId", "--limit", "1")
    runs = json.loads(out) if rc == 0 else []
    check("CI on the tag", bool(runs) and runs[0]["status"] == "completed" and runs[0]["conclusion"] == "success" and
          runs[0]["headSha"].startswith(want), json.dumps(runs[:1]))

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
    sys.exit(main(sys.argv[1:]))

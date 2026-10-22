"""The gate set a PLAN's step runs before its record
(docs/decisions/0031-MADR-verification-tools-in-repository.md), one gate at a
time, each exit status recorded before its output is read, each gate's output
in <out>/<gate>.log.

Usage: gates.py [--all] [--out DIR] [GATE ...]
  no GATE: every gate but the hosts' (windows, linux); --all: every gate.

Gates:
  regen      the generator run; the generated files unchanged
  precheck   make pre-add-check
  newfiles   scripts/go-precheck.sh on the untracked .go files
  nojsonv2   go test ./... with GOEXPERIMENT=nojsonv2
  winvet     GOOS=windows go vet ./..., both modules
  records    make check-records
  mdlint     make markdownlint
  idscan     tools/idscan.py on the changed files
  tools      make check-tools
  lint       make lint
  fix        go fix -diff ./..., both modules: any output fails
  tidy       go mod tidy -diff, both modules: any output fails
  api        make api
  race3      go test -race -count=3 -shuffle=on . (the root)
  floor      tools/floor.py
  platforms  tools/platforms.py
  windows    tools/remote.py tree --host windows (ACP_WINDOWS_HOST)
  linux      tools/remote.py tree --host linux (ACP_LINUX_HOST)

Exit 1 if any gate failed.
"""
from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402

TOOLS = Path(__file__).resolve().parent
HOSTS = ("windows", "linux")


def fails_on_output(status: int, out: str) -> int:
    """For a -diff command: a non-zero status, or any output, is a failure."""
    return status or (2 if out.strip() else 0)


class Gates:
    def __init__(self, root: Path, out: Path) -> None:
        self.root, self.out = root, out

    def run(self, gate: str, cmd: list[str], cwd: Path | None = None, env: dict[str, str] | None = None) -> tuple[int, str]:
        return _common.run(self.out / f"{gate}.log", cmd, cwd=cwd or self.root, env={**os.environ, **(env or {})})

    def git(self, *args: str) -> str:
        return self.run("git", ["git", *args])[1]

    def regen(self) -> int:
        before = self.git("diff", "--stat", "HEAD")
        env = {k: v for k, v in os.environ.items() if k not in ("GOPATH", "GOMODCACHE")}
        status, _ = _common.run(self.out / "regen.log", ["go", "run", "."], cwd=self.root / "cmd" / "generate", env=env)
        after = self.git("diff", "--stat", "HEAD")
        if before != after:
            print("  regen: the generated files CHANGED")
        return status or (before != after)

    def precheck(self) -> int:
        return self.run("precheck", ["make", "pre-add-check"])[0]

    def newfiles(self) -> int:
        new = [f for f in self.git("ls-files", "--others", "--exclude-standard").split() if f.endswith(".go")]
        if not new:
            print("  newfiles: no new .go files")
            return 0
        return self.run("newfiles", ["scripts/go-precheck.sh", *new], env={"GO_PRECHECK_SKIP_VULN": "1"})[0]

    def nojsonv2(self) -> int:
        return self.run("nojsonv2", ["go", "test", "-count=1", "./..."], env={"GOEXPERIMENT": "nojsonv2"})[0]

    def winvet(self) -> int:
        a = self.run("winvet", ["go", "vet", "./..."], env={"GOOS": "windows"})[0]
        b = self.run("winvet", ["go", "vet", "./..."], cwd=self.root / "cmd" / "generate", env={"GOOS": "windows"})[0]
        return a or b

    def records(self) -> int:
        return self.run("records", ["make", "check-records"])[0]

    def mdlint(self) -> int:
        return self.run("mdlint", ["make", "markdownlint"])[0]

    def idscan(self) -> int:
        return self.run("idscan", [sys.executable, "-B", str(TOOLS / "idscan.py")])[0]

    def tools(self) -> int:
        return self.run("tools", ["make", "check-tools"])[0]

    def lint(self) -> int:
        return self.run("lint", ["make", "lint"])[0]

    def fix(self) -> int:
        return max(fails_on_output(*self.run("fix", ["go", "fix", "-diff", "./..."], cwd=d)) for d in (self.root, self.root / "cmd" / "generate"))

    def tidy(self) -> int:
        return max(fails_on_output(*self.run("tidy", ["go", "mod", "tidy", "-diff"], cwd=d)) for d in (self.root, self.root / "cmd" / "generate"))

    def api(self) -> int:
        return self.run("api", ["make", "api"])[0]

    def race3(self) -> int:
        return self.run("race3", ["go", "test", "-race", "-count=3", "-shuffle=on", "."])[0]

    def floor(self) -> int:
        return self.run("floor", [sys.executable, "-B", str(TOOLS / "floor.py")])[0]

    def platforms(self) -> int:
        return self.run("platforms", [sys.executable, "-B", str(TOOLS / "platforms.py")])[0]

    def windows(self) -> int:
        return self.run("windows", [sys.executable, "-B", str(TOOLS / "remote.py"), "tree", "--host", "windows"])[0]

    def linux(self) -> int:
        return self.run("linux", [sys.executable, "-B", str(TOOLS / "remote.py"), "tree", "--host", "linux"])[0]


ORDER = ["regen", "precheck", "newfiles", "nojsonv2", "winvet", "records", "mdlint", "idscan", "tools", "lint", "fix", "tidy", "api",
         "race3", "floor", "platforms", "windows", "linux"]


def selected(names: list[str], everything: bool) -> list[str]:
    """The gates to run: those named, or every gate (--all), or every gate but
    the hosts'."""
    if names:
        unknown = [n for n in names if n not in ORDER]
        if unknown:
            _common.fail(f"gates: unknown gate(s) {', '.join(unknown)}; see --help", 2)
        return names
    return ORDER if everything else [g for g in ORDER if g not in HOSTS]


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0], formatter_class=argparse.RawDescriptionHelpFormatter, epilog=__doc__)
    ap.add_argument("--all", action="store_true", help="every gate, the hosts' included")
    ap.add_argument("--out", help="the output directory (default .cache/tools/gates)")
    ap.add_argument("gate", nargs="*")
    args = ap.parse_args(argv)
    names = selected(args.gate, args.all)
    root = _common.repo()
    out = _common.out_dir("gates", args.out)
    for n in names:
        (out / f"{n}.log").unlink(missing_ok=True)
    gates = Gates(root, out)
    failed = []
    for n in names:
        status = int(getattr(gates, n)())
        print(f"{'ok  ' if status == 0 else 'FAIL'} {n} (exit {status})", flush=True)
        if status:
            failed.append(n)
    print("all gates passed" if not failed else f"failed: {', '.join(failed)}")
    print(f"logs: {out}")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

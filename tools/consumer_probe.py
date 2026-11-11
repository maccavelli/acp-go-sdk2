"""The consumer before the tag
(docs/decisions/0054-PLAN-adopt-acp-schema-1-31-0-and-client-system-prompts.md,
deviation 7): consumer.py's program and test in a fresh module at go.mod's
`go` version, built with that toolchain forced, that requires this module
through a `replace` onto a tree: the repository, or --tree DIR, such as a
planted break's copy. It runs `go mod tidy` and `go vet`, builds and runs the
program, which must print consumer.WANT_RUN, runs `go test`, every test or
those --run names, and builds the program for Windows. Its go.mod must keep
the `go` line and the replace, and gain no toolchain line.

consumer.py VERSION fetches a release from the proxy, so it runs after the
tag. This runs before it, when consumer.py's program or test changes, and
says nothing of the proxy or the checksum database.
The caches are consumer.py's, in the tool's cache directory; the module is
there too and is removed after. Nothing is written in the tree.

Usage: consumer_probe.py [--tree DIR] [--run PATTERN]
Exit 0 when everything is as required; 2 when DIR holds no go.mod.
"""
from __future__ import annotations

import argparse
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402
import consumer  # noqa: E402
import floor  # noqa: E402


def go_mod(go: str, mod: str, tree: Path) -> str:
    """The probe module's go.mod: the floor's go line, and mod required
    through a replace onto tree."""
    return f"module example.invalid/consumer\n\ngo {go}\n\nrequire {mod} v0.0.0\n\nreplace {mod} => {tree}\n"


def go_mod_ok(text: str, go: str, mod: str, tree: Path) -> bool:
    """The probe's go.mod keeps the go line and the replace onto tree, and has
    no toolchain line."""
    replace = re.search(rf"^replace {re.escape(mod)} => {re.escape(str(tree))}$", text, re.M)
    return f"\ngo {go}\n" in text and "toolchain" not in text and replace is not None


def problems(outs: dict[str, tuple[int, str]], steps: list[str], go: str, want_run: str) -> list[str]:
    """What is not as required in the steps' exit statuses and outputs, each
    named by its step; empty when nothing is. A step of steps that outs does
    not hold did not run, because one before it failed."""
    found = [f"{name}: exit {status}" for name, (status, _) in outs.items() if status != 0]
    found += [f"{name}: not run" for name in steps if name not in outs]
    if "version" in outs and f"go version go{go} " not in outs["version"][1]:
        found.append(f"version: the toolchain is not go{go}")
    if "run" in outs and outs["run"][0] == 0 and outs["run"][1] != want_run:
        found.append("run: the program did not print consumer.WANT_RUN")
    return found


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--tree", help="the tree the replace names, such as a planted break's copy (default: the repository)")
    ap.add_argument("--run", help="go test's -run pattern (default: every test)")
    args = ap.parse_args(argv)
    tree = Path(args.tree).resolve() if args.tree else _common.repo()
    if not (tree / "go.mod").is_file():
        _common.fail(f"consumer_probe: no go.mod in {tree}", 2)
    mod, go = _common.module_path(tree), floor.go_line(tree / "go.mod")
    cache = _common.cache_dir("consumer")
    env = _common.go_env(GOTOOLCHAIN=f"go{go}", GOPATH=str(cache / "gopath"), GOMODCACHE=str(cache / "gopath" / "pkg" / "mod"),
                         GOCACHE=str(cache / "gocache"), GOFLAGS="-modcacherw")
    d = cache / f"probe-{os.getpid()}"
    d.mkdir(parents=True)
    (d / "go.mod").write_text(go_mod(go, mod, tree), encoding="utf-8")
    (d / "main.go").write_text(consumer.PROGRAM.replace("MODULE", mod), encoding="utf-8")
    (d / "consumer_test.go").write_text(consumer.TEST.replace("MODULE", mod), encoding="utf-8")
    test = ["go", "test", "-count=1", "-v", *(["-run", args.run] if args.run else []), "."]
    steps = (("version", ["go", "version"], {}), ("tidy", ["go", "mod", "tidy"], {}), ("vet", ["go", "vet", "."], {}),
             ("build", ["go", "build", "-o", "consumer", "."], {}), ("run", ["./consumer"], {}), ("test", test, {}),
             ("windows", ["go", "build", "-o", "consumer.exe", "."], {"GOOS": "windows", "GOARCH": "amd64"}))
    outs: dict[str, tuple[int, str]] = {}
    try:
        for name, cmd, extra in steps:
            p = subprocess.run(cmd, cwd=d, env={**env, **extra}, capture_output=True, text=True, timeout=1200)
            outs[name] = (p.returncode, (p.stdout + p.stderr).strip())
            shown = "" if name == "run" and p.returncode == 0 else outs[name][1]
            print(f"$ {' '.join(cmd)} {extra or ''}: exit {p.returncode}\n    " + shown.replace("\n", "\n    ")[:1500], flush=True)
            if p.returncode != 0:
                break
        found = problems(outs, [name for name, _, _ in steps], go, consumer.WANT_RUN)
        text = (d / "go.mod").read_text(encoding="utf-8")
        print("probe go.mod:\n    " + text.strip().replace("\n", "\n    "))
        if not go_mod_ok(text, go, mod, tree):
            found.append("go.mod: the go line, the replace or a toolchain line is not as required")
    finally:
        shutil.rmtree(d, ignore_errors=True)
    for line in found:
        print(f"consumer_probe: {line}")
    print("as required" if not found else "NOT AS REQUIRED")
    return 0 if not found else 1


if __name__ == "__main__":
    sys.exit(_common.main_guard(main, sys.argv[1:]))

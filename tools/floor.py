"""The floor gate (docs/decisions/0031-MADR-verification-tools-in-repository.md):
the module's tests at the Go version of go.mod's `go` line.

With GOTOOLCHAIN set to that version and the Go caches in the tool's cache
directory outside the repository (_common.cache_dir), it runs `go version`, `go vet ./...` and `go test -race -count=1
./...` in both modules, the root's tests with GOEXPERIMENT=goroutineleakprofile,
as the Makefile's LEAKGUARD_ENV does on 1.26. It fails when `go version` does
not name that toolchain.

Usage: floor.py [--out DIR]
"""
from __future__ import annotations

import argparse
import os
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402


def go_line(go_mod: Path) -> str:
    """The version on go.mod's `go` line."""
    m = re.search(r"^go (\d+\.\d+(?:\.\d+)?)\s*$", go_mod.read_text(encoding="utf-8"), re.M)
    if not m:
        _common.fail(f"floor: no go line in {go_mod}")
    return m.group(1)


def floor_env(version: str, cache: Path, base: dict[str, str]) -> dict[str, str]:
    """The environment of the floor's go commands: the toolchain forced, and
    the Go caches under cache, outside the repository."""
    env = {k: v for k, v in base.items() if k not in ("GOFLAGS", "GOEXPERIMENT", "GOTOOLCHAIN")}
    # -modcacherw: the module cache can then be deleted.
    env.update(GOTOOLCHAIN=f"go{version}", GOPATH=str(cache / "gopath"), GOMODCACHE=str(cache / "gopath" / "pkg" / "mod"),
               GOCACHE=str(cache / "gocache"), GOFLAGS="-modcacherw")
    return env


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--out", help="the output directory, for the log (default .cache/tools/floor)")
    args = ap.parse_args(argv)
    root = _common.repo()
    out = _common.out_dir("floor", args.out)
    log = out / "floor.log"
    log.unlink(missing_ok=True)
    version = go_line(root / "go.mod")
    env = floor_env(version, _common.cache_dir("floor"), dict(os.environ))
    runs = [
        (root, ["go", "version"], {}),
        (root, ["go", "vet", "./..."], {}),
        (root, ["go", "test", "-race", "-count=1", "./..."], {"GOEXPERIMENT": "goroutineleakprofile"}),
        (root / "cmd" / "generate", ["go", "vet", "./..."], {}),
        (root / "cmd" / "generate", ["go", "test", "-race", "-count=1", "./..."], {}),
    ]
    failed = 0
    for cwd, cmd, extra in runs:
        status, text = _common.run(log, cmd, cwd=cwd, env={**env, **extra})
        if cmd[1] == "version" and f"go version go{version} " not in text:
            print(f"floor: `go version` names {text.strip()!r}, not go{version}")
            status = status or 1
        last = text.strip().splitlines()[-1] if text.strip() else ""
        print(f"{'ok  ' if status == 0 else 'FAIL'} {cwd.relative_to(root) or '.'}: {' '.join(cmd)} (exit {status})  {last[:120]}", flush=True)
        failed |= status != 0
    print(f"floor gate at go{version}: {'passed' if not failed else 'FAILED'}; log {log}")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(_common.main_guard(main, sys.argv[1:]))

"""The per-platform checks of agentproc, clienthost, and the internal
packages they share their process code through
(docs/decisions/0031-MADR-verification-tools-in-repository.md;
docs/decisions/0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phase 7):
golangci-lint for GOOS=linux and GOOS=freebsd, which `make lint` builds for
neither, and `go vet` for each platform their files are built for.

Usage: platforms.py [--tree DIR] [--out DIR]
Exit 1 if any check fails.
"""
from __future__ import annotations

import argparse
import os
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402

# The packages with per-platform files.
PKGS = ["./agentproc/...", "./clienthost/...", "./internal/..."]

VET = [("darwin", "arm64"), ("linux", "amd64"), ("linux", "arm64"), ("freebsd", "amd64"), ("openbsd", "amd64"), ("netbsd", "amd64"),
       ("illumos", "amd64"), ("windows", "amd64"), ("js", "wasm"), ("wasip1", "wasm"), ("plan9", "amd64")]


def golangci_lint() -> str:
    """The golangci-lint the Makefile uses: GOLANGCI_LINT, or $GOPATH/bin's."""
    if os.environ.get("GOLANGCI_LINT"):
        return os.environ["GOLANGCI_LINT"]
    gopath = subprocess.run(["go", "env", "GOPATH"], capture_output=True, text=True, check=True).stdout.strip()
    return str(Path(gopath) / "bin" / "golangci-lint")


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--tree", help="the tree to check (default: this repository)")
    ap.add_argument("--out", help="the output directory (default .cache/tools/platforms)")
    args = ap.parse_args(argv)
    tree = Path(args.tree).resolve() if args.tree else _common.repo()
    out = _common.out_dir("platforms", args.out)
    lint = golangci_lint()
    checks = [(f"lint-{goos}", {"GOOS": goos}, [lint, "run", "-c", ".golangci.yml", *PKGS]) for goos in ("linux", "freebsd")]
    checks += [(f"vet-{goos}-{arch}", {"GOOS": goos, "GOARCH": arch}, ["go", "vet", *PKGS]) for goos, arch in VET]
    failed = []
    for name, env, cmd in checks:
        log = out / f"{name}.log"
        log.unlink(missing_ok=True)
        status, text = _common.run(log, cmd, cwd=tree, env={**os.environ, **env})
        last = text.strip().splitlines()[-1] if text.strip() else ""
        print(f"{'ok  ' if status == 0 else 'FAIL'} {name} (exit {status}){': ' + last[:160] if last else ''}", flush=True)
        if status:
            failed.append(name)
    print(f"{len(checks) - len(failed)} of {len(checks)} passed" + (f"; failed: {', '.join(failed)}" if failed else ""))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(_common.main_guard(main, sys.argv[1:]))

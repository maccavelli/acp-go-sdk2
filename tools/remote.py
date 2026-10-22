"""Run the tests on a test host over SSH
(docs/decisions/0031-MADR-verification-tools-in-repository.md).

The host is named by ACP_LINUX_HOST or ACP_WINDOWS_HOST, never here. Each run
uses a fresh directory, acp-tools-<random>, in the remote home, and removes it
after. The remote shell is a POSIX shell (Git's bash on Windows).

Usage:
  remote.py tree --host {linux,windows}
      copy the working tree's tracked and untracked-but-not-ignored files,
      then `go test -count=1 -shuffle=on ./...` in both modules
  remote.py pkg --host {linux,windows} [--goarch ARCH] [--tree DIR] PKG [FLAG ...]
      cross-compile PKG's test binary, from DIR when given (a copy with no
      .git), copy it, run it with the flags, each quoted for the remote
      shell (a -test.run pattern may hold `|`)

Exit with the tests' status: 0 when every run passed.
"""
from __future__ import annotations

import argparse
import io
import os
import secrets
import shlex
import subprocess
import sys
import tarfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402

SUMMARY = ("ok", "FAIL", "---", "go version", "?", "panic")


def remote_dir() -> str:
    """A fresh directory name, relative to the remote home."""
    return f"acp-tools-{secrets.token_hex(4)}"


def binary_command(directory: str, exe: str, flags: list[str]) -> str:
    """The remote command that runs a copied test binary with flags."""
    return f"cd {shlex.quote(directory)} && ./{shlex.quote(exe)} {' '.join(shlex.quote(f) for f in flags)}".rstrip()


def ssh(host: str) -> list[str]:
    return ["ssh", "-o", "BatchMode=yes", host]


def summary(text: str) -> str:
    return "\n".join(line for line in text.splitlines() if line.lstrip().startswith(SUMMARY))


def tree(kind: str, out: Path) -> int:
    host = _common.host(kind)
    root = _common.repo()
    log = out / f"tree-{kind}.log"
    log.unlink(missing_ok=True)
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w") as tar:
        for name in _common.files_of_tree(root):
            tar.add(root / name, arcname=name, recursive=False)
    d = remote_dir()
    status, text = _common.run(log, [*ssh(host), f"mkdir {d} && tar -xf - -C {d}"], stdin=buf.getvalue())
    if status:
        print(f"remote: copying the tree to the {kind} host failed (exit {status}); see {log}")
        return status
    failed = 0
    try:
        for mod in (".", "cmd/generate"):
            status, text = _common.run(log, [*ssh(host), f"cd {d}/{mod} && go version && go test -count=1 -shuffle=on ./... 2>&1"], timeout=3600)
            print(f"== {kind} {mod}: exit {status}\n{summary(text)}", flush=True)
            failed |= status != 0
    finally:
        _common.run(log, [*ssh(host), "rm", "-rf", d])
    return 1 if failed else 0


def pkg(kind: str, goarch: str, package: str, flags: list[str], out: Path, tree_dir: Path | None = None) -> int:
    host = _common.host(kind)
    root = tree_dir or _common.repo()
    log = out / f"pkg-{kind}.log"
    log.unlink(missing_ok=True)
    exe = "pkg.test.exe" if kind == "windows" else "pkg.test"
    local = out / f"{exe}-{kind}-{goarch}"
    env = {**os.environ, "GOOS": kind, "GOARCH": goarch, "CGO_ENABLED": "0"}
    status, text = _common.run(log, ["go", "test", "-c", "-o", str(local), package], cwd=root, env=env)
    if status:
        print(f"remote: building {package}'s test binary failed (exit {status})\n{text.strip()[-800:]}")
        return status
    d = remote_dir()
    try:
        steps = [[*ssh(host), "mkdir", d]]
        for cmd in steps:
            status, _ = _common.run(log, cmd)
            if status:
                return status
        status, _ = _common.run(log, [*ssh(host), f"cat > {d}/{exe}"], stdin=local.read_bytes())
        if status:
            return status
        if kind != "windows":
            _common.run(log, [*ssh(host), "chmod", "+x", f"{d}/{exe}"])
        status, text = _common.run(log, [*ssh(host), binary_command(d, exe, flags)], timeout=3600)
        print(f"== {kind} {package}: exit {status}\n" + "\n".join(
            line for line in text.splitlines() if not line.startswith(("Pseudo-terminal", "** "))))
        return status
    finally:
        _common.run(log, [*ssh(host), "rm", "-rf", d])
        local.unlink(missing_ok=True)


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0], formatter_class=argparse.RawDescriptionHelpFormatter, epilog=__doc__)
    sub = ap.add_subparsers(dest="cmd", required=True)
    t = sub.add_parser("tree")
    t.add_argument("--host", choices=sorted(_common.HOST_VARS), required=True)
    t.add_argument("--out")
    p = sub.add_parser("pkg")
    p.add_argument("--host", choices=sorted(_common.HOST_VARS), required=True)
    p.add_argument("--goarch", default="amd64")
    p.add_argument("--tree", help="build from this tree instead of the repository")
    p.add_argument("--out")
    p.add_argument("package")
    p.add_argument("flags", nargs=argparse.REMAINDER)
    args = ap.parse_args(argv)
    out = _common.out_dir("remote", args.out)
    if args.cmd == "tree":
        return tree(args.host, out)
    return pkg(args.host, args.goarch, args.package, args.flags, out, Path(args.tree).resolve() if args.tree else None)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

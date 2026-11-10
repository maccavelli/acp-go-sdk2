"""Run the tests on a test host over SSH
(docs/decisions/0031-MADR-verification-tools-in-repository.md).

The host is named by ACP_LINUX_HOST or ACP_WINDOWS_HOST, never here. Each run
uses a fresh directory, acp-tools-<random>, in the remote home, removes it
after, pass, fail or timeout, and checks it is gone. The remote shell is a
POSIX shell (Git's bash on Windows).

Usage:
  remote.py tree --host {linux,windows}
      copy the working tree's tracked and untracked-but-not-ignored files,
      then `go test -count=1 -shuffle=on ./...` in both modules
  remote.py pkg --host {linux,windows} [--goarch ARCH] [--tree DIR] PKG [FLAG ...]
      cross-compile PKG's test binary, from DIR when given (a copy with no
      .git), copy it, run it with the flags, each quoted for the remote
      shell (a -test.run pattern may hold `|`)
  remote.py test --host {linux,windows} [--module DIR] [--tree DIR] [--timeout SECONDS] -- ARG ...
      copy the tree, or DIR (git's files when it has .git, else every file
      but `._*` and .DS_Store), then `go test ARG ...` in the module DIR
      names (default `.`), each argument quoted for the remote shell;
      packages are arguments too, and none is added. With -race, the
      host's `go env CGO_ENABLED` must be 1, or the run is refused before
      the copy. SECONDS bounds the remote go test (default 3600)

Exit status: the first failing step's (the copy's, the build's, the tests';
124 when a step did not finish in time; for tree, 1 when either module
failed); 2 for a refusal; else 3 when the remote directory was left behind;
0 when every run passed and the directory is gone.
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
import time
from pathlib import Path, PurePosixPath

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402

SUMMARY = ("ok", "FAIL", "---", "go version", "?", "panic")
BANNER = ("Pseudo-terminal", "** ")
RACE_ON = {"-race", "--race", "-race=true", "--race=true", "-race=1", "--race=1"}
TIMED_OUT = 124
REFUSED = 2
LEFT_BEHIND = 3


class Refused(Exception):
    """A run the tool will not start, with the reason."""


def remote_dir() -> str:
    """A fresh directory name, relative to the remote home."""
    return f"acp-tools-{secrets.token_hex(4)}"


def binary_command(directory: str, exe: str, flags: list[str]) -> str:
    """The remote command that runs a copied test binary with flags."""
    return f"cd {shlex.quote(directory)} && ./{shlex.quote(exe)} {' '.join(shlex.quote(f) for f in flags)}".rstrip()


def go_test_command(directory: str, module: str, args: list[str]) -> str:
    """The remote command that runs go test with args in module of directory."""
    where = directory if module == "." else f"{directory}/{module}"
    return (f"cd {shlex.quote(where)} && go version && go env GOOS GOARCH CGO_ENABLED && "
            f"go test {' '.join(shlex.quote(a) for a in args)} 2>&1")


def wants_race(args: list[str]) -> bool:
    """Whether go test's arguments turn the race detector on."""
    return any(a in RACE_ON for a in args)


def module_dir(root: Path, module: str) -> str:
    """module as a relative POSIX path of a directory of root holding a
    go.mod; Refused otherwise."""
    p = PurePosixPath(module.replace("\\", "/") or ".")
    if p.is_absolute() or (module[:1].isalpha() and module[1:2] == ":"):
        raise Refused(f"--module {module}: want a directory of the tree, not an absolute path")
    if ".." in p.parts:
        raise Refused(f"--module {module}: leaves the tree")
    if not (root / p / "go.mod").is_file():
        raise Refused(f"--module {module}: no go.mod in {p}")
    return str(p)


def tree_files(root: Path) -> list[str]:
    """The files to copy from root: git's, when root has .git; else every
    regular file and symbolic link under it, links not followed, without
    macOS's `._*` and .DS_Store files."""
    if (root / ".git").exists():
        return _common.files_of_tree(root)
    names = []
    for here, dirs, files in os.walk(root):
        dirs.sort()
        for f in files:
            if f.startswith("._") or f == ".DS_Store":
                continue
            full = Path(here) / f
            if full.is_symlink() or full.is_file():
                names.append(full.relative_to(root).as_posix())
    return sorted(names)


def ssh(host: str) -> list[str]:
    return ["ssh", "-o", "BatchMode=yes", host]


def summary(text: str) -> str:
    return "\n".join(line for line in text.splitlines() if line.lstrip().startswith(SUMMARY))


def strip_banner(text: str) -> str:
    """text without the lines SSH writes about the session."""
    return "\n".join(line for line in text.splitlines() if not line.startswith(BANNER))


def step(log: Path, cmd: list[str], timeout: int = 3600, stdin: bytes | None = None, cwd: Path | None = None,
         env: dict[str, str] | None = None) -> tuple[int, str]:
    """_common.run, a timeout made a verdict: status 124 and "did not finish
    within N s", in the log too."""
    try:
        return _common.run(log, cmd, cwd=cwd, env=env, stdin=stdin, timeout=timeout)
    except subprocess.TimeoutExpired:
        text = f"did not finish within {timeout} s"
        log.parent.mkdir(parents=True, exist_ok=True)
        with open(log, "a", encoding="utf-8") as f:
            f.write(f"$ {' '.join(cmd)}\n{text}\nexit {TIMED_OUT}\n\n")
        return TIMED_OUT, text


def copy_tree(host: str, root: Path, names: list[str], d: str, log: Path) -> int:
    """Copy names, relative to root, into d in the remote home; the status."""
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w") as tar:
        for name in names:
            tar.add(root / name, arcname=name, recursive=False)
    status, _ = step(log, [*ssh(host), f"mkdir {shlex.quote(d)} && tar -xf - -C {shlex.quote(d)}"], stdin=buf.getvalue())
    return status


def remove(host: str, d: str, log: Path) -> bool:
    """Remove d in the remote home; true when it is gone."""
    q = shlex.quote(d)
    status, _ = step(log, [*ssh(host), f"rm -rf -- {q}; test ! -e {q}"], timeout=300)
    return status == 0


def removed(kind: str, host: str, d: str, log: Path) -> bool:
    """remove, its outcome printed."""
    gone = remove(host, d, log)
    print(f"remote: {kind} host: {d} {'removed' if gone else 'left behind in the remote home'}", flush=True)
    return gone


def outcome(status: int, gone: bool) -> int:
    """The exit status of a run whose first failing step gave status."""
    if status:
        return status
    return 0 if gone else LEFT_BEHIND


def tree(kind: str, out: Path) -> int:
    host = _common.host(kind)
    root = _common.repo()
    log = out / f"tree-{kind}.log"
    log.unlink(missing_ok=True)
    names = tree_files(root)
    d = remote_dir()
    status = 0
    try:
        status = copy_tree(host, root, names, d, log)
        if status:
            print(f"remote: copying the tree to the {kind} host failed (exit {status}); see {log}")
        else:
            failed = False
            for mod in (".", "cmd/generate"):
                s, text = step(log, [*ssh(host), f"cd {d}/{mod} && go version && go test -count=1 -shuffle=on ./... 2>&1"])
                print(f"== {kind} {mod}: exit {s}\n{summary(text) if s != TIMED_OUT else text}", flush=True)
                failed |= s != 0
            status = 1 if failed else 0
    finally:
        gone = removed(kind, host, d, log)
    return outcome(status, gone)


def pkg(kind: str, goarch: str, package: str, flags: list[str], out: Path, tree_dir: Path | None = None) -> int:
    host = _common.host(kind)
    root = tree_dir or _common.repo()
    log = out / f"pkg-{kind}.log"
    log.unlink(missing_ok=True)
    exe = "pkg.test.exe" if kind == "windows" else "pkg.test"
    local = out / f"{exe}-{kind}-{goarch}"
    env = {**os.environ, "GOOS": kind, "GOARCH": goarch, "CGO_ENABLED": "0"}
    status, text = step(log, ["go", "test", "-c", "-o", str(local), package], cwd=root, env=env)
    if status:
        print(f"remote: building {package}'s test binary failed (exit {status})\n{text.strip()[-800:]}")
        return status
    d = remote_dir()
    status = 0
    try:
        status, _ = step(log, [*ssh(host), "mkdir", d])
        if not status:
            status, _ = step(log, [*ssh(host), f"cat > {d}/{exe}"], stdin=local.read_bytes())
        if not status:
            if kind != "windows":
                step(log, [*ssh(host), "chmod", "+x", f"{d}/{exe}"])
            status, text = step(log, [*ssh(host), binary_command(d, exe, flags)])
            print(f"== {kind} {package}: exit {status}\n{strip_banner(text)}", flush=True)
    finally:
        gone = removed(kind, host, d, log)
        local.unlink(missing_ok=True)
    return outcome(status, gone)


def test(kind: str, module: str, tree_dir: Path | None, timeout: int, args: list[str], out: Path) -> int:
    if not args:
        print("remote: test needs go test's arguments after --, its packages among them")
        return REFUSED
    host = _common.host(kind)
    root = tree_dir or _common.repo()
    try:
        mod = module_dir(root, module)
    except Refused as err:
        print(f"remote: {err}")
        return REFUSED
    log = out / f"test-{kind}.log"
    log.unlink(missing_ok=True)
    if wants_race(args):
        status, text = step(log, [*ssh(host), "go env CGO_ENABLED"], timeout=300)
        lines = strip_banner(text).strip().splitlines()
        value = lines[-1].strip() if lines else ""
        if status:
            print(f"remote: reading go env CGO_ENABLED on the {kind} host failed (exit {status}); see {log}")
            return status
        if value != "1":
            print(f"remote: -race needs cgo, and `go env CGO_ENABLED` is {value!r} on the {kind} host")
            return REFUSED
    names = tree_files(root)
    d = remote_dir()
    status = 0
    try:
        status = copy_tree(host, root, names, d, log)
        if status:
            print(f"remote: copying the tree to the {kind} host failed (exit {status}); see {log}")
        else:
            start = time.monotonic()
            status, text = step(log, [*ssh(host), go_test_command(d, mod, args)], timeout=timeout)
            print(f"== {kind} {mod}: exit {status}, {time.monotonic() - start:.1f} s\n{strip_banner(text)}", flush=True)
    finally:
        gone = removed(kind, host, d, log)
    return outcome(status, gone)


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
    g = sub.add_parser("test")
    g.add_argument("--host", choices=sorted(_common.HOST_VARS), required=True)
    g.add_argument("--module", default=".", help="the module's directory in the tree (default .)")
    g.add_argument("--tree", help="copy this tree instead of the repository")
    g.add_argument("--timeout", type=int, default=3600, help="seconds the remote go test may take (default 3600)")
    g.add_argument("--out")
    g.add_argument("args", nargs=argparse.REMAINDER, help="go test's arguments, after --")
    args = ap.parse_args(argv)
    out = _common.out_dir("remote", args.out)
    if args.cmd == "tree":
        return tree(args.host, out)
    if args.cmd == "test":
        rest = args.args[1:] if args.args[:1] == ["--"] else args.args
        return test(args.host, args.module, Path(args.tree).resolve() if args.tree else None, args.timeout, rest, out)
    return pkg(args.host, args.goarch, args.package, args.flags, out, Path(args.tree).resolve() if args.tree else None)


if __name__ == "__main__":
    sys.exit(_common.main_guard(main, sys.argv[1:]))

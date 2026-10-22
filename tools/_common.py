"""Shared helpers for the verification tools
(docs/decisions/0031-MADR-verification-tools-in-repository.md).

The repository's root comes from git, the hosts from the environment, and
output goes under .cache/tools/, which git ignores. Nothing here names a host,
an account or a machine's path.
"""
from __future__ import annotations

import os
import re
import subprocess
import sys
from pathlib import Path

# The environment variables that name the SSH hosts the host tools reach.
HOST_VARS = {"linux": "ACP_LINUX_HOST", "windows": "ACP_WINDOWS_HOST"}


def repo(start: Path | None = None) -> Path:
    """The root of the repository holding start, by default this file."""
    here = start or Path(__file__).resolve().parent
    out = subprocess.run(["git", "rev-parse", "--show-toplevel"], cwd=here, capture_output=True, text=True, check=True)
    return Path(out.stdout.strip())


def host(kind: str) -> str:
    """The SSH host for kind, "linux" or "windows", from its environment
    variable; stops the tool, naming the variable, when it is unset."""
    var = HOST_VARS[kind]
    value = os.environ.get(var, "").strip()
    if not value:
        raise SystemExit(f"{var} is not set: set it to the SSH host or alias of the {kind} test host")
    return value


def out_dir(tool: str, given: str | None = None) -> Path:
    """The tool's output directory: given, or .cache/tools/<tool>/ in the
    repository; created."""
    d = Path(given) if given else repo() / ".cache" / "tools" / tool
    d.mkdir(parents=True, exist_ok=True)
    return d


def cache_dir(tool: str) -> Path:
    """The tool's cache directory, outside the repository, where a tree-wide
    lint would otherwise walk it: ACP_TOOLS_CACHE when set, else the user's
    cache directory, under acp-go-sdk2-tools; created
    (0031-MADR-verification-tools-in-repository.md, "Amendment 2026-10-22,
    step 3: the caches outside the repository")."""
    base = os.environ.get("ACP_TOOLS_CACHE", "").strip()
    if base:
        root = Path(base)
    elif sys.platform == "darwin":
        root = Path.home() / "Library" / "Caches" / "acp-go-sdk2-tools"
    elif sys.platform == "win32":
        root = Path(os.environ.get("LOCALAPPDATA") or Path.home() / "AppData" / "Local") / "acp-go-sdk2-tools"
    else:
        root = Path(os.environ.get("XDG_CACHE_HOME") or Path.home() / ".cache") / "acp-go-sdk2-tools"
    d = root / tool
    d.mkdir(parents=True, exist_ok=True)
    return d


def run(log: Path, cmd: list[str], cwd: Path | None = None, env: dict[str, str] | None = None, stdin: bytes | None = None,
        timeout: int = 3600) -> tuple[int, str]:
    """Run cmd, take its exit status before its output is read, append both to
    log, and return them."""
    proc = subprocess.run(cmd, cwd=cwd, env=env, input=stdin, capture_output=True, timeout=timeout)
    status = proc.returncode
    out = proc.stdout.decode(errors="replace") + proc.stderr.decode(errors="replace")
    log.parent.mkdir(parents=True, exist_ok=True)
    with open(log, "a", encoding="utf-8") as f:
        f.write(f"$ {' '.join(cmd)}  (cwd {cwd or Path.cwd()})\n{out}\nexit {status}\n\n")
    return status, out


def files_of_tree(root: Path) -> list[str]:
    """The tracked and the untracked but not ignored files under root, without
    macOS metadata files."""
    out = subprocess.run(["git", "ls-files", "-co", "--exclude-standard", "-z"], cwd=root, capture_output=True, check=True).stdout
    return [n for n in out.decode().split("\0") if n and not Path(n).name.startswith("._") and (root / n).is_file()]


def module_path(root: Path) -> str:
    """The module path on go.mod's module line."""
    for line in (root / "go.mod").read_text(encoding="utf-8").splitlines():
        if line.startswith("module "):
            return line.split()[1]
    raise SystemExit(f"no module line in {root / 'go.mod'}")


def github_repo(url: str) -> str:
    """OWNER/NAME of a GitHub remote URL, https or SSH form."""
    m = re.match(r"^(?:https://github\.com/|git@github\.com:|ssh://git@github\.com/)([^/]+)/([^/]+?)(?:\.git)?/?$", url.strip())
    if not m:
        raise SystemExit(f"not a GitHub remote: {url!r}")
    return f"{m.group(1)}/{m.group(2)}"


def origin_repo(root: Path) -> str:
    """OWNER/NAME of the repository's origin remote."""
    url = subprocess.run(["git", "remote", "get-url", "origin"], cwd=root, capture_output=True, text=True, check=True).stdout
    return github_repo(url)


def main_guard(main, argv: list[str]) -> int:
    """Run a tool's main; when its reader closes the pipe early, as `| head`
    does, end quietly with status 1 rather than a traceback. This is the
    handling Python's documentation of the signal module gives ("Note on
    SIGPIPE"): it points standard output at the null device because "Python
    flushes standard streams on exit", which could otherwise raise again."""
    try:
        status = main(argv)
        sys.stdout.flush()
        return status
    except BrokenPipeError:
        devnull = os.open(os.devnull, os.O_WRONLY)
        os.dup2(devnull, sys.stdout.fileno())
        return 1


def fail(message: str, status: int = 1) -> None:
    """Print message to standard error and exit with status."""
    print(message, file=sys.stderr)
    raise SystemExit(status)

"""Interoperability of package remote with the official TypeScript and Python
SDKs (docs/decisions/0043-PLAN-remote-http-and-websocket-transport.md, phase 7).

Each SDK is cloned into the tool's cache directory and checked out at its
revision: the TypeScript one gets `npm ci`, the Python one a venv made with
`uv venv` into which the clone is installed with its `http` extra and
Hypercorn, so the examples run against the pinned commit, not PyPI's release.
example/remote is built into the cache. Each pair starts its server, waits for
its port, runs its client under a 60 s bound, and requires the client's
success lines, in order. Each pair's output goes to <out>/<pair>.log.

The Python examples are fixed at localhost:8000, so the tool refuses to start
when that port is taken, and serves the Go side of the Python pairs there. The
Python clients send cwd=".", which the Go SDK refuses as not absolute, so the
tool runs copies of them with the absolute working directory instead.

Usage: interop.py [--ts REV] [--py REV] [--out DIR] [--tree DIR] [PAIR...]
With no PAIR, every pair runs. --tree builds example/remote from another tree,
such as a planted break's copy. Exit 0 when every pair run passed.
"""
from __future__ import annotations

import argparse
import os
import shutil
import signal
import socket
import subprocess
import sys
import time
from dataclasses import dataclass, field
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402

TS_REPO = "https://github.com/agentclientprotocol/typescript-sdk.git"
PY_REPO = "https://github.com/agentclientprotocol/python-sdk.git"
TS_REV = "08a0d30f023bf86574d49d59c4154e44d4386f55"
PY_REV = "9853d8b514d6340000bcbd01ec07808436db80cf"
# The commands the tool needs, and what each is for.
TOOLS = {"git": "the clones", "go": "example/remote", "node": "the TypeScript examples", "npm": "the TypeScript clone's npm ci",
         "npx": "the TypeScript examples", "uv": "the Python venv"}
PY_PORT = 8000
PAIR_BOUND = 60
START_BOUND = 30
STOP_BOUND = 10


@dataclass
class Pair:
    name: str
    server: list[str]
    server_cwd: Path | None
    client: list[str]
    client_cwd: Path | None
    port: int
    want: list[str]
    client_env: dict[str, str] = field(default_factory=dict)
    server_env: dict[str, str] = field(default_factory=dict)
    # The Go server is ended with an interrupt, so its Handler.Close and
    # Shutdown run; the peers' servers are terminated.
    interrupt: bool = False


def require_tools(which=shutil.which) -> None:
    """Stop, naming the command, when one the tool needs is not on PATH."""
    for cmd, purpose in TOOLS.items():
        if not which(cmd):
            _common.fail(f"interop: {cmd} is not on PATH; it is needed for {purpose}", 2)


def free_port() -> int:
    """A loopback TCP port that was free when asked."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def localhost_families(port: int = PY_PORT) -> list[str]:
    """The address families localhost resolves to, in resolver order."""
    out = []
    for family, *_ in socket.getaddrinfo("localhost", port, type=socket.SOCK_STREAM):
        name = {socket.AF_INET: "IPv4", socket.AF_INET6: "IPv6"}.get(family, str(family))
        if name not in out:
            out.append(name)
    return out


def listening(host: str, port: int) -> bool:
    """Whether something accepts a TCP connection at host:port, on any of its
    addresses."""
    try:
        infos = socket.getaddrinfo(host, port, type=socket.SOCK_STREAM)
    except OSError:
        return False
    for family, kind, proto, _, addr in infos:
        with socket.socket(family, kind, proto) as s:
            s.settimeout(0.5)
            try:
                s.connect(addr)
                return True
            except OSError:
                continue
    return False


def pairs(binary: Path, ts: Path, py_python: Path, py: Path, py_clients: Path, ts_port: int, go_port: int) -> list[Pair]:
    """The eight pairs: the Go client against each SDK's server, and each SDK's
    clients against the Go server, over each profile. The Python clients are
    patch_cwd's copies, in py_clients."""
    ts_server = ["npx", "tsx", "src/examples/http-server.ts"]
    py_server = [str(py_python), "examples/http_server.py"]
    go_ts = [str(binary), "serve", "-addr", f"127.0.0.1:{go_port}"]
    go_py = [str(binary), "serve", "-addr", f"localhost:{PY_PORT}"]
    loaded = ["stop reason: end_turn", "loaded: 1 updates"]
    return [
        Pair("ts-http", ts_server, ts, [str(binary), "dial", f"http://127.0.0.1:{ts_port}/acp"], None, ts_port, loaded,
             server_env={"PORT": str(ts_port)}),
        Pair("ts-ws", ts_server, ts, [str(binary), "dial", f"ws://127.0.0.1:{ts_port}/acp"], None, ts_port, loaded,
             server_env={"PORT": str(ts_port)}),
        Pair("py-http", py_server, py, [str(binary), "dial", f"http://localhost:{PY_PORT}/acp"], None, PY_PORT,
             ["stop reason: end_turn"]),
        Pair("py-ws", py_server, py, [str(binary), "dial", f"ws://localhost:{PY_PORT}/acp"], None, PY_PORT,
             ["stop reason: end_turn"]),
        Pair("go-ts-http", go_ts, None, ["npx", "tsx", "src/examples/http-client.ts"], ts, go_port, ["Done: end_turn"],
             client_env={"ACP_HTTP_URL": f"http://127.0.0.1:{go_port}/acp"}, interrupt=True),
        Pair("go-ts-ws", go_ts, None, ["npx", "tsx", "src/examples/ws-client.ts"], ts, go_port, ["Done: end_turn"],
             client_env={"ACP_WS_URL": f"ws://127.0.0.1:{go_port}/acp"}, interrupt=True),
        Pair("go-py-http", go_py, None, [str(py_python), str(py_clients / "http_client.py")], py, PY_PORT,
             ["stop reason: end_turn"], interrupt=True),
        Pair("go-py-ws", go_py, None, [str(py_python), str(py_clients / "ws_client.py")], py, PY_PORT,
             ["stop reason: end_turn"], interrupt=True),
    ]


def lines_in_order(output: str, want: list[str]) -> bool:
    """Whether each of want is a whole line of output, in want's order."""
    lines = [line.rstrip("\r") for line in output.splitlines()]
    i = 0
    for line in lines:
        if i < len(want) and line == want[i]:
            i += 1
    return i == len(want)


def patch_cwd(text: str, name: str) -> str:
    """A Python client example's text with its cwd=".", which the Go SDK
    refuses as not absolute, made the absolute working directory
    (0043-PLAN-remote-http-and-websocket-transport.md, "Deviation, 2026-10-30,
    phase 7's Python clients' relative cwd"). Stops, naming the file, when a
    text it changes does not occur exactly once."""
    for old in ("import asyncio\n", 'cwd="."'):
        if text.count(old) != 1:
            _common.fail(f"interop: {name} holds {old.strip()!r} {text.count(old)} times, want once: the example has changed", 2)
    return text.replace("import asyncio\n", "import asyncio\nimport os\n").replace('cwd="."', "cwd=os.getcwd()")


def checkout(log: Path, url: str, d: Path, rev: str) -> bool:
    """Clone url into d when absent, fetch when rev is not there, and check
    rev out. Returns whether HEAD changed."""
    if not (d / ".git").is_dir():
        status, _ = _common.run(log, ["git", "clone", "--quiet", url, str(d)])
        if status:
            _common.fail(f"interop: git clone {url} failed; see {log}")
    have = subprocess.run(["git", "-C", str(d), "cat-file", "-e", f"{rev}^{{commit}}"], capture_output=True).returncode == 0
    if not have:
        status, _ = _common.run(log, ["git", "fetch", "--quiet", "origin"], cwd=d)
        if status:
            _common.fail(f"interop: git fetch in {d} failed; see {log}")
    before = subprocess.run(["git", "-C", str(d), "rev-parse", "HEAD"], capture_output=True, text=True).stdout.strip()
    status, _ = _common.run(log, ["git", "checkout", "--quiet", "--detach", rev], cwd=d)
    if status:
        _common.fail(f"interop: git checkout {rev} in {d} failed; see {log}")
    after = subprocess.run(["git", "-C", str(d), "rev-parse", "HEAD"], capture_output=True, text=True).stdout.strip()
    return before != after


def setup(cache: Path, out: Path, tree: Path, ts_rev: str, py_rev: str) -> tuple[Path, Path, Path, Path, Path]:
    """The clones at their revisions, set up once per revision; returns the
    TypeScript clone, the Python clone, the venv's python, the example's
    binary, built from tree, and the directory of the Python clients'
    copies."""
    ts, py, venv = cache / "typescript-sdk", cache / "python-sdk", cache / "py-venv"
    log = out / "setup.log"
    log.unlink(missing_ok=True)
    changed = checkout(log, TS_REPO, ts, ts_rev)
    if changed or not (ts / "node_modules").is_dir():
        status, _ = _common.run(log, ["npm", "ci", "--no-audit", "--no-fund"], cwd=ts)
        if status:
            _common.fail(f"interop: npm ci failed; see {log}")
    changed = checkout(log, PY_REPO, py, py_rev)
    python = venv / ("Scripts/python.exe" if sys.platform == "win32" else "bin/python")
    if changed or not python.is_file():
        for cmd in (["uv", "venv", "--quiet", "--allow-existing", str(venv)],
                    ["uv", "pip", "install", "--quiet", "--reinstall-package", "agent-client-protocol", "--python", str(python),
                     ".[http]", "hypercorn>=0.17"]):
            status, _ = _common.run(log, cmd, cwd=py)
            if status:
                _common.fail(f"interop: {' '.join(cmd[:3])} failed; see {log}")
    clients = cache / "py-clients"
    clients.mkdir(exist_ok=True)
    for name in ("http_client.py", "ws_client.py"):
        (clients / name).write_text(patch_cwd((py / "examples" / name).read_text(encoding="utf-8"), name), encoding="utf-8")
    binary = cache / ("remote-example.exe" if sys.platform == "win32" else "remote-example")
    status, _ = _common.run(log, ["go", "build", "-o", str(binary), "./example/remote"], cwd=tree)
    if status:
        _common.fail(f"interop: go build ./example/remote failed; see {log}")
    return ts, py, python, binary, clients


def stop(proc: subprocess.Popen, interrupt: bool) -> str:
    """End the server's process group, an interrupt or a terminate first and a
    kill after STOP_BOUND; returns how it ended."""
    if proc.poll() is not None:
        return f"had exited {proc.returncode}"
    os.killpg(proc.pid, signal.SIGINT if interrupt else signal.SIGTERM)
    try:
        return f"exited {proc.wait(timeout=STOP_BOUND)}"
    except subprocess.TimeoutExpired:
        os.killpg(proc.pid, signal.SIGKILL)
        proc.wait()
        return f"killed after {STOP_BOUND} s"


def run_pair(p: Pair, log: Path) -> bool:
    """Run one pair; its server's and client's output go to log."""
    start = time.monotonic()
    with open(log, "w", encoding="utf-8") as f:
        f.write(f"$ {' '.join(p.server)}  (server, cwd {p.server_cwd or Path.cwd()})\n")
        f.flush()
        server = subprocess.Popen(p.server, cwd=p.server_cwd, env={**os.environ, **p.server_env}, stdout=f,
                                  stderr=subprocess.STDOUT, start_new_session=True)
        try:
            host = "localhost" if p.port == PY_PORT else "127.0.0.1"
            while not listening(host, p.port):
                if server.poll() is not None or time.monotonic() - start > START_BOUND:
                    f.write(f"\nthe server did not listen on {host}:{p.port} within {START_BOUND} s\n")
                    return False
                time.sleep(0.1)
            left = max(1, PAIR_BOUND - int(time.monotonic() - start))
            f.write(f"\n$ {' '.join(p.client)}  (client, cwd {p.client_cwd or Path.cwd()}, bound {left} s)\n")
            f.flush()
            # Its own process group, so the bound ends npx's node child too.
            client = subprocess.Popen(p.client, cwd=p.client_cwd, env={**os.environ, **p.client_env}, stdout=subprocess.PIPE,
                                      stderr=subprocess.STDOUT, start_new_session=True)
            try:
                raw, _ = client.communicate(timeout=left)
                status: int | str = client.returncode
            except subprocess.TimeoutExpired:
                os.killpg(client.pid, signal.SIGKILL)
                raw, _ = client.communicate()
                status = "timed out"
            out = raw.decode(errors="replace")
            f.write(f"{out}\nclient exit {status}\n")
            ok = status == 0 and lines_in_order(out, p.want)
            f.write(f"required lines {p.want}: {'found' if ok else 'NOT FOUND'}\n")
            return ok
        finally:
            f.write(f"server {stop(server, p.interrupt)}\n")


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--ts", default=TS_REV, help="the TypeScript SDK's revision")
    ap.add_argument("--py", default=PY_REV, help="the Python SDK's revision")
    ap.add_argument("--out", help="the logs' directory; by default .cache/tools/interop/")
    ap.add_argument("--tree", help="the tree example/remote is built from, such as a planted break's copy; by default the "
                                   "repository")
    ap.add_argument("pair", nargs="*", help="the pairs to run; by default every pair")
    args = ap.parse_args(argv)
    if sys.platform == "win32":
        _common.fail("interop: the pairs run on macOS or Linux", 2)
    require_tools()
    out = _common.out_dir("interop", args.out)
    if listening("localhost", PY_PORT):
        _common.fail(f"interop: localhost:{PY_PORT} is taken; the Python examples need it", 2)
    tree = Path(args.tree).resolve() if args.tree else _common.repo()
    ts, py, python, binary, clients = setup(_common.cache_dir("interop"), out, tree, args.ts, args.py)
    all_pairs = pairs(binary, ts, python, py, clients, free_port(), free_port())
    names = [p.name for p in all_pairs]
    unknown = [n for n in args.pair if n not in names]
    if unknown:
        _common.fail(f"interop: unknown pair(s) {unknown}; the pairs are {names}", 2)
    print(f"TypeScript {args.ts}, Python {args.py}; localhost resolves to {', '.join(localhost_families())}")
    failed = []
    for p in all_pairs:
        if args.pair and p.name not in args.pair:
            continue
        log = out / f"{p.name}.log"
        ok = run_pair(p, log)
        tail = log.read_text(encoding="utf-8", errors="replace").rstrip().splitlines()[-6:]
        print(f"{p.name}: {'ok' if ok else 'FAILED'} ({log})\n    " + "\n    ".join(tail), flush=True)
        if not ok:
            failed.append(p.name)
    print("all pairs passed" if not failed else f"failed: {' '.join(failed)}")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(_common.main_guard(main, sys.argv[1:]))

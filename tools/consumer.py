"""A consumer at the floor (docs/decisions/0031-MADR-verification-tools-in-repository.md):
a fresh module at go.mod's `go` version, built with that toolchain forced,
`go get`s VERSION from the proxy with the checksum database on, and builds and
runs a program that starts `sh -c "cat >/dev/null"` through agentproc with an
OutputTap, closes it, and prints DroppedTapBytes; it also builds the program
for Windows. Its go.mod must keep the `go` line and gain no toolchain line.
The caches are in the tool's cache directory; the module is removed after.

Usage: consumer.py VERSION
Exit 0 when everything is as required.
"""
from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402
import floor  # noqa: E402

PROGRAM = '''package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	acp "MODULE"
	"MODULE/agentproc"
)

type client struct{ acp.Client }

func (client) SessionUpdate(context.Context, acp.SessionNotification) error { return nil }

func main() {
	cmd := exec.Command("sh", "-c", "cat >/dev/null")
	p, err := (&agentproc.Command{Cmd: cmd, OutputTap: io.Discard}).Start(client{})
	if err != nil {
		fmt.Println("Start:", err)
		return
	}
	fmt.Println("Close:", p.Close())
	out, in := p.DroppedTapBytes()
	fmt.Println("dropped", out, in)
	fmt.Println("protocol version", acp.ProtocolVersionNumber)
}
'''
WANT_RUN = "Close: <nil>\ndropped 0 0\nprotocol version 1"


def go_mod_ok(text: str, go: str, mod: str, version: str) -> bool:
    """The consumer's go.mod keeps the go line, has no toolchain line, and
    requires the version."""
    return f"\ngo {go}\n" in text and "toolchain" not in text and f"{mod} {version}" in text


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("version")
    args = ap.parse_args(argv)
    root = _common.repo()
    mod, go = _common.module_path(root), floor.go_line(root / "go.mod")
    cache = _common.cache_dir("consumer")
    d = cache / f"module-{os.getpid()}"
    d.mkdir(parents=True)
    (d / "go.mod").write_text(f"module example.invalid/consumer\n\ngo {go}\n", encoding="utf-8")
    (d / "main.go").write_text(PROGRAM.replace("MODULE", mod), encoding="utf-8")
    env = {k: v for k, v in os.environ.items() if k not in ("GONOSUMDB", "GOEXPERIMENT", "GOFLAGS", "GOPRIVATE")}
    env.update(GOTOOLCHAIN=f"go{go}", GOPATH=str(cache / "gopath"), GOMODCACHE=str(cache / "gopath" / "pkg" / "mod"),
               GOCACHE=str(cache / "gocache"), GOPROXY="https://proxy.golang.org,direct", GOFLAGS="-modcacherw")
    ok, outs = True, {}
    try:
        for name, cmd, extra in (("version", ["go", "version"], {}), ("get", ["go", "get", f"{mod}@{args.version}"], {}),
                                 ("tidy", ["go", "mod", "tidy"], {}), ("build", ["go", "build", "-o", "consumer", "."], {}),
                                 ("run", ["./consumer"], {}),
                                 ("windows", ["go", "build", "-o", "consumer.exe", "."], {"GOOS": "windows", "GOARCH": "amd64"})):
            p = subprocess.run(cmd, cwd=d, env={**env, **extra}, capture_output=True, text=True, timeout=1200)
            outs[name] = (p.stdout + p.stderr).strip()
            print(f"$ {' '.join(cmd)} {extra or ''}: exit {p.returncode}\n    " + outs[name].replace("\n", "\n    ")[:700], flush=True)
            ok &= p.returncode == 0
        ok &= f"go version go{go} " in outs["version"] and outs["run"] == WANT_RUN
        text = (d / "go.mod").read_text(encoding="utf-8")
        print("consumer go.mod:\n    " + text.strip().replace("\n", "\n    "))
        ok &= go_mod_ok(text, go, mod, args.version)
    finally:
        shutil.rmtree(d, ignore_errors=True)
    print("as required" if ok else "NOT AS REQUIRED")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(_common.main_guard(main, sys.argv[1:]))

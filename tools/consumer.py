"""A consumer at the floor (docs/decisions/0031-MADR-verification-tools-in-repository.md):
a fresh module at go.mod's `go` version, built with that toolchain forced,
`go get`s VERSION from the proxy with the checksum database on, and builds and
runs a program that starts `sh -c "cat >/dev/null"` through agentproc with an
OutputTap, closes it, and prints DroppedTapBytes; it also builds the program
for Windows. Its client embeds ClientRefusals and implements only
SessionUpdate and RequestPermission, and it prints UpdateNotice's Kind and the
answer to a file read
(0036-PLAN-session-update-constructors-kind-and-client-refusals.md, step 6). Its
agent embeds AgentRefusals and implements only the seven ungated methods, and it
prints what IsMethodNotFound, PickPermissionOption and ContentBlock.Kind give
(0038-PLAN-consumer-helpers-kind-and-agent-refusals.md, step 8). It prints what
a CompactionTimeline and an ExtensionMux give, and its test, run with `go test`,
joins a ScriptAgent and a MemoryClient with acptest.Connect and requires
PresentNotice's fallback to arrive
(0039-PLAN-five-adopter-additions-on-schema-1-25-0.md, phase 8). It also checks
an elicitation answer through Fields, ParseValue and Check, and its test's pair
sets WithElicitationCheck
(0040-PLAN-check-elicitation-forms-answers-and-urls.md, phase 8), so VERSION is
v1.24.0 or later. Its go.mod must keep the `go` line and gain no toolchain line.
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
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	acp "MODULE"
	"MODULE/agentproc"
)

type client struct{ acp.ClientRefusals }

func (client) SessionUpdate(context.Context, acp.SessionNotification) error { return nil }

func (client) RequestPermission(context.Context, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
}

type agent struct{ acp.AgentRefusals }

func (agent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (agent) Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{ProtocolVersion: acp.ProtocolVersionNumber}, nil
}

func (agent) Cancel(context.Context, acp.CancelNotification) error { return nil }

func (agent) NewSession(context.Context, acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: "s"}, nil
}

func (agent) Prompt(context.Context, acp.PromptRequest) (acp.PromptResponse, error) {
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

func (agent) SetSessionConfigOption(context.Context, acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, nil
}

func (agent) SetSessionMode(context.Context, acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

var _ acp.Agent = agent{}

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
	fmt.Println("kind", acp.UpdateNotice(acp.NoticeSeverityInfo, "t").Kind())
	_, err = client{}.ReadTextFile(context.Background(), acp.ReadTextFileRequest{})
	fmt.Println("refused", err)
	_, err = agent{}.ListSessions(context.Background(), acp.ListSessionsRequest{})
	fmt.Println("agent refuses list", acp.IsMethodNotFound(err))
	opts := []acp.PermissionOption{
		{OptionId: "trap", Name: "Allow", Kind: acp.PermissionOptionKindRejectOnce},
		{OptionId: "once", Name: "Once", Kind: acp.PermissionOptionKindAllowOnce},
	}
	o, ok := acp.PickPermissionOption(opts, acp.PermissionOptionKindAllowOnce)
	fmt.Println("pick", o.OptionId, ok)
	fmt.Println("block", acp.TextBlock("x").Kind())
	var timeline acp.CompactionTimeline
	timeline.Apply(acp.UpdateCompaction("c", acp.CompactionStatusInProgress))
	e, ok := timeline.Apply(acp.UpdateCompactionSummaryText("c", "sum"))
	fmt.Println("compaction", e.Id, e.Status, len(e.Summary), ok, len(timeline.Open()))
	var mux acp.ExtensionMux
	acp.Handle(&mux, "_x/echo", func(_ context.Context, p map[string]string) (string, error) { return p["s"], nil })
	r, err := mux.HandleExtensionMethod(context.Background(), "_x/echo", json.RawMessage(`{"s":"hi"}`))
	fmt.Println("mux", r, err, mux.Methods(), len(acp.TraceMeta("00-a-b-01", "", "")))
	var form acp.ElicitationSchema
	if err := json.Unmarshal([]byte(`{"type":"object","required":["age"],"properties":{"age":{"type":"integer","maximum":150}}}`), &form); err != nil {
		fmt.Println("form:", err)
		return
	}
	field := form.Fields()[0]
	age, err := field.Schema.ParseValue("200")
	fmt.Println("elicitation", field.Name, field.Required, field.Schema.Kind(), err)
	fmt.Println("check", form.Check(map[string]acp.ElicitationContentValue{field.Name: age}))
}
'''
TEST = '''package main

import (
	"testing"
	"time"

	acp "MODULE"
	"MODULE/acptest"
)

func TestPresentNotice(t *testing.T) {
	client := &acptest.MemoryClient{}
	pair := acptest.Connect(t, &acptest.ScriptAgent{}, client, acp.WithElicitationCheck())
	if _, err := pair.Client.Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		t.Fatal(err)
	}
	if err := acp.PresentNotice(t.Context(), pair.Agent, "s", acp.Notice{Severity: acp.NoticeSeverityInfo, Title: "hello"}); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Second); len(client.Updates()) == 0; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the notice did not arrive")
		}
	}
	if kind := client.Updates()[0].Update.Kind(); kind != "agent_message_chunk" {
		t.Fatalf("a client without session.notices got %q, want the agent message fallback", kind)
	}
}
'''
WANT_RUN = ("Close: <nil>\ndropped 0 0\nprotocol version 1\nkind notice\n"
            'refused {"code":-32601,"message":"Method not found","data":{"method":"fs/read_text_file"}}\n'
            "agent refuses list true\npick once true\nblock text\n"
            "compaction c in_progress 1 true 1\nmux hi <nil> [_x/echo] 1\n"
            "elicitation age true integer <nil>\n"
            'check CreateElicitationAccept: content["age"]: maximum: is greater than 150')


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
    (d / "consumer_test.go").write_text(TEST.replace("MODULE", mod), encoding="utf-8")
    env = {k: v for k, v in os.environ.items() if k not in ("GONOSUMDB", "GOEXPERIMENT", "GOFLAGS", "GOPRIVATE")}
    env.update(GOTOOLCHAIN=f"go{go}", GOPATH=str(cache / "gopath"), GOMODCACHE=str(cache / "gopath" / "pkg" / "mod"),
               GOCACHE=str(cache / "gocache"), GOPROXY="https://proxy.golang.org,direct", GOFLAGS="-modcacherw")
    ok, outs = True, {}
    try:
        for name, cmd, extra in (("version", ["go", "version"], {}), ("get", ["go", "get", f"{mod}@{args.version}"], {}),
                                 ("tidy", ["go", "mod", "tidy"], {}), ("build", ["go", "build", "-o", "consumer", "."], {}),
                                 ("run", ["./consumer"], {}), ("test", ["go", "test", "-count=1", "."], {}),
                                 ("windows", ["go", "build", "-o", "consumer.exe", "."], {"GOOS": "windows", "GOARCH": "amd64"})):
            p = subprocess.run(cmd, cwd=d, env={**env, **extra}, capture_output=True, text=True, timeout=1200)
            outs[name] = (p.stdout + p.stderr).strip()
            print(f"$ {' '.join(cmd)} {extra or ''}: exit {p.returncode}\n    " + outs[name].replace("\n", "\n    ")[:700], flush=True)
            ok &= p.returncode == 0
            if not ok:
                break
        ok &= f"go version go{go} " in outs["version"] and outs.get("run") == WANT_RUN
        text = (d / "go.mod").read_text(encoding="utf-8")
        print("consumer go.mod:\n    " + text.strip().replace("\n", "\n    "))
        ok &= go_mod_ok(text, go, mod, args.version)
    finally:
        shutil.rmtree(d, ignore_errors=True)
    print("as required" if ok else "NOT AS REQUIRED")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(_common.main_guard(main, sys.argv[1:]))

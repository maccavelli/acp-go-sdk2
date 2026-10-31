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
(0040-PLAN-check-elicitation-forms-answers-and-urls.md, phase 8). It waits for
the agent with WaitContext and prints EndedCleanly of its connection's end,
prints what a ToolCallTimeline gives, and writes and reads a file and runs a
terminal through clienthost
(0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phase 17). It runs an
agent in process through InProcess and ServeAgent, a Transcript as both taps,
and prints the tail of a failing child's standard error; its test reads a
transcript back with ReadRecords, validates and decodes it, and replays it
with ConnectReplay (0042-PLAN-in-process-agent-runner.md, phase 11). It
serves a ScriptAgent through remote.NewHandler on loopback, dials it over
ws:// and http://, runs a prompt on each, and closes the handler, so the
proxy's copy of github.com/coder/websocket is fetched with the checksum
database on (0043-PLAN-remote-http-and-websocket-transport.md, phase 10). It
closes an in-process agent through Process.CloseContext, and, with a client
whose SessionUpdate ignores its context, closes another under a 300 ms bound
and prints errors.Is of its error for the bound's cause and
acp.ErrHandlersRunning (0042-PLAN-in-process-agent-runner.md, phase 17). It
opens a client connection with
WithNotificationOverflowPolicy(acp.OverflowCloseConnection), the opt-out from
the default OverflowBlock, initializes it, and prints that the two policies
differ (0044-PLAN-notification-queue-back-pressure.md, phase 4). So VERSION is
v1.29.0 or later. Its go.mod must keep the `go` line and gain no toolchain
line.
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	acp "MODULE"
	"MODULE/acptest"
	"MODULE/agentproc"
	"MODULE/clienthost"
	"MODULE/remote"
)

type client struct{ acp.ClientRefusals }

func (client) SessionUpdate(context.Context, acp.SessionNotification) error { return nil }

func (client) RequestPermission(context.Context, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
}

// hung is a client whose SessionUpdate ignores its context until release
// closes; entered closes at its first call.
type hung struct {
	client
	entered, release chan struct{}
	once             sync.Once
}

func (h *hung) SessionUpdate(context.Context, acp.SessionNotification) error {
	h.once.Do(func() { close(h.entered) })
	<-h.release
	return nil
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

func serve(ctx context.Context, in io.Reader, out io.Writer) error {
	return acp.ServeAgent(ctx, agent{}, in, out)
}

type host struct {
	clienthost.Terminals
	clienthost.Files
	clienthost.Refusals
}

func main() {
	cmd := exec.Command("sh", "-c", "cat >/dev/null")
	p, err := (&agentproc.Command{Cmd: cmd, OutputTap: io.Discard}).Start(client{})
	if err != nil {
		fmt.Println("Start:", err)
		return
	}
	fmt.Println("Close:", p.Close())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fmt.Println("wait", p.WaitContext(ctx), acp.EndedCleanly(p.Conn().Err()))
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
	var tools acp.ToolCallTimeline
	tools.Apply(acp.StartToolCall("t", "Read", acp.WithStartStatus(acp.ToolCallStatusPending)))
	call, ok := tools.Apply(acp.UpdateToolCall("t", acp.WithUpdateStatus(acp.ToolCallStatusCompleted)))
	fmt.Println("tool", call.Title, call.Status, ok, len(tools.Open()))
	dir, err := os.MkdirTemp("", "consumer")
	if err != nil {
		fmt.Println("temp:", err)
		return
	}
	defer os.RemoveAll(dir)
	h := &host{}
	h.MkdirAll = true
	path := filepath.Join(dir, "a", "f.txt")
	_, err = h.WriteTextFile(ctx, acp.WriteTextFileRequest{SessionId: "s", Path: path, Content: "one\\ntwo\\n"})
	read, rerr := h.ReadTextFile(ctx, acp.ReadTextFileRequest{SessionId: "s", Path: path, Line: new(uint32(2)), Limit: new(uint32(1))})
	fmt.Printf("files %v %v %q\\n", err, rerr, read.Content)
	_, err = h.ReadTextFile(ctx, acp.ReadTextFileRequest{SessionId: "s", Path: filepath.Join(dir, "missing")})
	fmt.Println("missing", acp.IsResourceNotFound(err))
	term, err := h.CreateTerminal(ctx, acp.CreateTerminalRequest{SessionId: "s", Command: "sh", Args: []string{"-c", "printf hi; exit 3"}})
	if err != nil {
		fmt.Println("terminal:", err)
		return
	}
	st, err := h.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{SessionId: "s", TerminalId: term.TerminalId})
	if err != nil || st.ExitCode == nil {
		fmt.Println("exit:", err, st.Signal)
		return
	}
	shown, err := h.TerminalOutput(ctx, acp.TerminalOutputRequest{SessionId: "s", TerminalId: term.TerminalId})
	fmt.Println("terminal", *st.ExitCode, shown.Output, shown.Truncated, err)
	_, err = h.ReleaseTerminal(ctx, acp.ReleaseTerminalRequest{SessionId: "s", TerminalId: term.TerminalId})
	_, gone := h.TerminalOutput(ctx, acp.TerminalOutputRequest{SessionId: "s", TerminalId: term.TerminalId})
	fmt.Println("release", err, acp.IsResourceNotFound(gone), h.Close())
	var log bytes.Buffer
	tr := agentproc.NewTranscript(&log)
	ip, err := (&agentproc.InProcess{Serve: serve, InputTap: tr.Input(), OutputTap: tr.Output()}).Start(client{})
	if err != nil {
		fmt.Println("InProcess:", err)
		return
	}
	res, err := ip.Conn().Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	fmt.Println("in-process", res.ProtocolVersion, err, ip.Close(), acp.EndedCleanly(ip.Conn().Err()))
	fmt.Println("transcript", tr.Close(), bytes.Count(log.Bytes(), []byte("\\n")))
	cp, err := (&agentproc.InProcess{Serve: serve}).Start(client{})
	if err != nil {
		fmt.Println("InProcess:", err)
		return
	}
	fmt.Println("close-context", cp.CloseContext(ctx))
	hc := &hung{entered: make(chan struct{}), release: make(chan struct{})}
	hp, err := (&agentproc.InProcess{Grace: 300 * time.Millisecond, Serve: func(ctx context.Context, in io.Reader, out io.Writer) error {
		turn := acptest.Turn{Updates: []acp.SessionUpdate{acp.UpdateAgentMessageText("hi")}, StopReason: acp.StopReasonEndTurn}
		return acp.ServeAgent(ctx, &acptest.ScriptAgent{Turns: []acptest.Turn{turn}}, in, out)
	}}).Start(hc)
	if err != nil {
		fmt.Println("InProcess:", err)
		return
	}
	_, ierr := hp.Conn().Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	hs, serr := hp.Conn().NewSession(ctx, acp.NewSessionRequest{Cwd: dir})
	if ierr != nil || serr != nil {
		fmt.Println("hung:", ierr, serr)
		return
	}
	go func() {
		_, _ = hp.Conn().Prompt(ctx, acp.PromptRequest{SessionId: hs.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}})
	}()
	select {
	case <-hc.entered:
	case <-ctx.Done():
		fmt.Println("hung: the update never came")
		return
	}
	bound, cancelBound := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancelBound()
	berr := hp.CloseContext(bound)
	close(hc.release)
	fmt.Println("bounded", errors.Is(berr, context.DeadlineExceeded), errors.Is(berr, acp.ErrHandlersRunning), hp.Wait())
	failing := exec.Command("sh", "-c", "echo starting >&2; echo failed >&2; exit 2")
	fp, err := (&agentproc.Command{Cmd: failing, StderrTailLines: 1, JoinStderrTail: true}).Start(client{})
	if err != nil {
		fmt.Println("Start:", err)
		return
	}
	var se *agentproc.StderrError
	joined := errors.As(fp.Wait(), &se) && se.Tail == fp.StderrTail()
	fmt.Printf("stderr %q %v\\n", fp.StderrTail(), joined)
	rh := remote.NewHandler(func(ctx context.Context, in io.Reader, out io.Writer) error {
		return acp.ServeAgent(ctx, &acptest.ScriptAgent{Turns: []acptest.Turn{{StopReason: acp.StopReasonEndTurn}}}, in, out)
	}, nil)
	srv := httptest.NewServer(rh)
	defer srv.Close()
	for _, url := range []string{"ws" + strings.TrimPrefix(srv.URL, "http"), srv.URL} {
		rc, err := remote.Dial(ctx, url, nil)
		if err != nil {
			fmt.Println("Dial:", err)
			return
		}
		rconn := acp.NewClientSideConnection(client{}, rc, rc)
		_, ierr := rconn.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
		sess, serr := rconn.NewSession(ctx, acp.NewSessionRequest{Cwd: dir})
		res, perr := rconn.Prompt(ctx, acp.PromptRequest{SessionId: sess.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}})
		scheme, _, _ := strings.Cut(url, ":")
		fmt.Println("remote", scheme, ierr, serr, perr, res.StopReason, rc.Close())
		<-rconn.Done()
	}
	fmt.Println("handler", rh.Close(ctx))
	ar, cw := io.Pipe()
	cr, aw := io.Pipe()
	oa := acp.NewAgentSideConnection(agent{}, aw, ar)
	oc := acp.NewClientSideConnection(client{}, cw, cr, acp.WithNotificationOverflowPolicy(acp.OverflowCloseConnection))
	ores, oerr := oc.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	fmt.Println("overflow", acp.OverflowBlock != acp.OverflowCloseConnection, ores.ProtocolVersion, oerr, oc.Close(), oa.Close())
}
'''
TEST = '''package main

import (
	"bytes"
	"testing"
	"time"

	acp "MODULE"
	"MODULE/acptest"
	"MODULE/agentproc"
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

func TestTranscriptReplay(t *testing.T) {
	var log bytes.Buffer
	tr := agentproc.NewTranscript(&log)
	p, err := (&agentproc.InProcess{Serve: serve, InputTap: tr.Input(), OutputTap: tr.Output()}).Start(client{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Conn().Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	records, err := acptest.ReadRecords(&log)
	if err != nil {
		t.Fatal(err)
	}
	acptest.ValidateGolden(t, records)
	sent, err := acptest.Decode[acp.InitializeRequest](records, acptest.Sent, "initialize")
	if err != nil || len(sent) != 1 {
		t.Fatalf("decoded %d initialize requests from %d records: %v", len(sent), len(records), err)
	}
	replay := acptest.ConnectReplay(t, records, client{})
	res, err := replay.Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	if err != nil || res.ProtocolVersion != acp.ProtocolVersionNumber {
		t.Fatalf("the replay answered %v, %v", res.ProtocolVersion, err)
	}
}
'''
WANT_RUN = ("Close: <nil>\nwait <nil> true\ndropped 0 0\nprotocol version 1\nkind notice\n"
            'refused {"code":-32601,"message":"Method not found","data":{"method":"fs/read_text_file"}}\n'
            "agent refuses list true\npick once true\nblock text\n"
            "compaction c in_progress 1 true 1\nmux hi <nil> [_x/echo] 1\n"
            "elicitation age true integer <nil>\n"
            'check CreateElicitationAccept: content["age"]: maximum: is greater than 150\n'
            "tool Read completed true 0\n"
            'files <nil> <nil> "two\\n"\nmissing true\n'
            "terminal 3 hi false <nil>\nrelease <nil> true <nil>\n"
            "in-process 1 <nil> <nil> true\ntranscript <nil> 2\n"
            "close-context <nil>\nbounded true true <nil>\n"
            'stderr "failed\\n" true\n'
            "remote ws <nil> <nil> <nil> end_turn <nil>\nremote http <nil> <nil> <nil> end_turn <nil>\nhandler <nil>\n"
            "overflow true 1 <nil> <nil> <nil>")


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

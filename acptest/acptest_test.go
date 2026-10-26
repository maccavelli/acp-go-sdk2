package acptest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// TestConnectRoundTrip initializes a pair, opens a session and plays one
// scripted turn of two updates, which the client records; the leak guard in
// TestMain requires the cleanup to leave no goroutine.
func TestConnectRoundTrip(t *testing.T) {
	agent := &acptest.ScriptAgent{Turns: []acptest.Turn{{
		Updates:    []acp.SessionUpdate{acp.UpdateAgentMessageText("one"), acp.UpdateAgentMessageText("two")},
		StopReason: acp.StopReasonEndTurn,
	}}}
	client := &acptest.MemoryClient{}
	pair := acptest.Connect(t, agent, client)
	ctx := t.Context()
	init, err := pair.Client.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	if err != nil || init.ProtocolVersion != acp.ProtocolVersionNumber {
		t.Fatalf("initialize: %+v, %v", init, err)
	}
	sess, err := pair.Client.NewSession(ctx, acp.NewSessionRequest{Cwd: acptest.Abs("/work")})
	if err != nil || sess.SessionId != "session-1" {
		t.Fatalf("session/new: %+v, %v", sess, err)
	}
	resp, err := pair.Client.Prompt(ctx, acp.PromptRequest{SessionId: sess.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}})
	if err != nil || resp.StopReason != acp.StopReasonEndTurn {
		t.Fatalf("session/prompt: %+v, %v", resp, err)
	}
	var texts []string
	for _, n := range client.Updates() {
		if n.SessionId != sess.SessionId || n.Update.AgentMessageChunk == nil {
			t.Fatalf("update %+v", n)
		}
		b, err := json.Marshal(n.Update.AgentMessageChunk.Content)
		if err != nil {
			t.Fatal(err)
		}
		texts = append(texts, string(b))
	}
	if want := []string{`{"text":"one","type":"text"}`, `{"text":"two","type":"text"}`}; strings.Join(texts, "|") != strings.Join(want, "|") {
		t.Errorf("updates %q, want %q", texts, want)
	}
	if _, err := pair.Client.Prompt(ctx, acp.PromptRequest{SessionId: sess.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("again")}}); code(err) != -32603 {
		t.Errorf("a prompt past the script: %v, want an internal error", err)
	}
}

// slowAgent is a ScriptAgent whose NewSession runs until its context ends
// and then takes a while to return.
type slowAgent struct {
	acptest.ScriptAgent
	started  chan struct{}
	finished *atomic.Bool
}

func (a *slowAgent) NewSession(ctx context.Context, _ acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	close(a.started)
	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)
	a.finished.Store(true)
	return acp.NewSessionResponse{}, ctx.Err()
}

// TestConnectCleanupWaits requires Connect's cleanup to wait for the pair's
// goroutines, a handler's included: a handler still running when its
// connection closes has returned by the time the test's cleanup has. The
// leak guard cannot see a cleanup that closes and does not wait, since the
// goroutines then end on their own.
func TestConnectCleanupWaits(t *testing.T) {
	var finished atomic.Bool
	done := make(chan error, 1)
	t.Run("pair", func(t *testing.T) {
		agent := &slowAgent{started: make(chan struct{}), finished: &finished}
		pair := acptest.Connect(t, agent, &acptest.MemoryClient{})
		if _, err := pair.Client.Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
			t.Fatal(err)
		}
		go func() {
			_, err := pair.Client.NewSession(context.Background(), acp.NewSessionRequest{Cwd: acptest.Abs("/work")})
			done <- err
		}()
		select {
		case <-agent.started:
		case err := <-done:
			t.Fatalf("session/new returned before its handler ran: %v", err)
		}
	})
	if !finished.Load() {
		t.Error("Connect's cleanup returned while a handler of the pair was still running")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Error("session/new succeeded on a closed pair")
		}
	case <-time.After(10 * time.Second):
		t.Error("session/new did not return once the test's cleanup had run: the pair was not closed")
	}
}

// code is the JSON-RPC code of err, or 0.
func code(err error) int {
	if re, ok := errors.AsType[*acp.RequestError](err); ok {
		return re.Code
	}
	return 0
}

// TestAbs requires Abs's result to be absolute on the host, a host-absolute
// path to come back cleaned, and any other under the temporary directory. The
// windows and linux gates run it on those hosts.
func TestAbs(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"/work/a.txt", "/", "a.txt", "dir/b", `C:\work\a.txt`} {
		got := acptest.Abs(p)
		if !filepath.IsAbs(got) {
			t.Errorf("Abs(%q) = %q, not absolute on this host", p, got)
		}
		switch {
		case filepath.IsAbs(p) && got != filepath.Clean(p):
			t.Errorf("Abs(%q) = %q, want it cleaned", p, got)
		case !filepath.IsAbs(p) && !strings.HasPrefix(got, os.TempDir()):
			t.Errorf("Abs(%q) = %q, want it under %s", p, got, os.TempDir())
		}
	}
}

// TestRecorder requires frames split at newlines across short writes and
// reads, JSON in encoding/json's canonical form, other lines as they were, no
// frame for an unended line, copies that are not aliases, and Close passed
// to what it wraps.
func TestRecorder(t *testing.T) {
	t.Parallel()
	var rec acptest.Recorder
	var out bytes.Buffer
	w := rec.Writer(&out)
	for _, chunk := range []string{`{"b":1,"a":`, `"<x>"}` + "\n" + `{"c"`, ":2}\nnot json\n", "unended"} {
		if _, err := w.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	canonical, err := json.Marshal(map[string]any{"a": "<x>", "b": 1})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{string(canonical), `{"c":2}`, "not json"}
	requireFrames(t, rec.Frames(), want)
	if !strings.HasSuffix(out.String(), "unended") {
		t.Errorf("the writer wrote %q", out.String())
	}

	frames := rec.Frames()
	frames[0][0] = 'X'
	frames[1] = json.RawMessage("changed")
	requireFrames(t, rec.Frames(), want)

	var fromReader acptest.Recorder
	r := fromReader.Reader(iotest.OneByteReader(strings.NewReader("{\"z\": 1, \"y\": [2]}\n\n")))
	if _, err := io.ReadAll(r); err != nil {
		t.Fatal(err)
	}
	requireFrames(t, fromReader.Frames(), []string{`{"y":[2],"z":1}`, ""})

	pr, pw := io.Pipe()
	cw := (&acptest.Recorder{}).Writer(pw)
	if err := cw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := pr.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Errorf("read after the recorder's Close: %v, want io.EOF", err)
	}
}

func requireFrames(t *testing.T, got []json.RawMessage, want []string) {
	t.Helper()
	var s []string
	for _, f := range got {
		s = append(s, string(f))
	}
	if strings.Join(s, "\n") != strings.Join(want, "\n") || len(s) != len(want) {
		t.Errorf("frames %q, want %q", s, want)
	}
}

// TestScriptAgent calls each of ScriptAgent's methods.
func TestScriptAgent(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	caps := acp.AgentCapabilities{LoadSession: true}
	a := &acptest.ScriptAgent{Capabilities: caps}
	init, err := a.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: 7})
	if err != nil || init.ProtocolVersion != acp.ProtocolVersionNumber || !init.AgentCapabilities.LoadSession {
		t.Errorf("Initialize: %+v, %v", init, err)
	}
	for i, want := range []acp.SessionId{"session-1", "session-2"} {
		if s, err := a.NewSession(ctx, acp.NewSessionRequest{}); err != nil || s.SessionId != want {
			t.Errorf("NewSession %d: %+v, %v", i+1, s, err)
		}
	}
	if _, err := a.Prompt(ctx, acp.PromptRequest{SessionId: "session-1"}); code(err) != -32603 {
		t.Errorf("Prompt with no turn: %v, want an internal error", err)
	}
	if _, err := a.Authenticate(ctx, acp.AuthenticateRequest{}); err != nil {
		t.Errorf("Authenticate: %v", err)
	}
	if err := a.Cancel(ctx, acp.CancelNotification{}); err != nil {
		t.Errorf("Cancel: %v", err)
	}
	if _, err := a.SetSessionMode(ctx, acp.SetSessionModeRequest{}); err != nil {
		t.Errorf("SetSessionMode: %v", err)
	}
	if _, err := a.SetSessionConfigOption(ctx, acp.SetSessionConfigOptionRequest{}); err != nil {
		t.Errorf("SetSessionConfigOption: %v", err)
	}
	if _, err := a.ListSessions(ctx, acp.ListSessionsRequest{}); code(err) != -32601 {
		t.Errorf("ListSessions, a capability-gated method: %v, want method not found", err)
	}
}

// TestMemoryClient reads with line and limit at their edges, a missing path
// and line 0, writes, answers permissions with and without a scripted
// outcome, and leaves a terminal method to ClientRefusals.
func TestMemoryClient(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	c := &acptest.MemoryClient{}
	c.SetFile("/f", "one\ntwo\nthree")
	u := func(n uint32) *uint32 { return &n }
	for _, tt := range []struct {
		name        string
		line, limit *uint32
		want        string
	}{
		{"whole", nil, nil, "one\ntwo\nthree"},
		{"first line", u(1), nil, "one\ntwo\nthree"},
		{"second line, one", u(2), u(1), "two\n"},
		{"last line", u(3), nil, "three"},
		{"past the last", u(4), nil, ""},
		{"no lines", nil, u(0), ""},
		{"limit past the end", u(2), u(9), "two\nthree"},
	} {
		got, err := c.ReadTextFile(ctx, acp.ReadTextFileRequest{Path: "/f", Line: tt.line, Limit: tt.limit})
		if err != nil || got.Content != tt.want {
			t.Errorf("%s: %q, %v; want %q", tt.name, got.Content, err, tt.want)
		}
	}
	if _, err := c.ReadTextFile(ctx, acp.ReadTextFileRequest{Path: "/f", Line: u(0)}); code(err) != -32602 {
		t.Errorf("line 0: %v, want invalid params", err)
	}
	if _, err := c.ReadTextFile(ctx, acp.ReadTextFileRequest{Path: "/missing"}); code(err) != -32002 {
		t.Errorf("a missing path: %v, want resource not found", err)
	}
	if _, err := c.WriteTextFile(ctx, acp.WriteTextFileRequest{Path: "/new", Content: "x"}); err != nil {
		t.Fatal(err)
	}
	if got, ok := c.File("/new"); !ok || got != "x" {
		t.Errorf("File after WriteTextFile: %q, %v", got, ok)
	}
	c.SetPermission("call-1", acp.NewRequestPermissionOutcomeSelected("allow"))
	for id, want := range map[acp.ToolCallId]string{"call-1": "allow", "call-2": ""} {
		r, err := c.RequestPermission(ctx, acp.RequestPermissionRequest{ToolCall: acp.ToolCallUpdate{ToolCallId: id}})
		switch {
		case err != nil:
			t.Errorf("%s: %v", id, err)
		case want == "" && r.Outcome.Cancelled == nil:
			t.Errorf("%s: %+v, want cancelled", id, r.Outcome)
		case want != "" && (r.Outcome.Selected == nil || string(r.Outcome.Selected.OptionId) != want):
			t.Errorf("%s: %+v, want %s selected", id, r.Outcome, want)
		}
	}
	if _, err := c.CreateTerminal(ctx, acp.CreateTerminalRequest{}); code(err) != -32601 {
		t.Errorf("CreateTerminal, a capability-gated method: %v, want method not found", err)
	}
}

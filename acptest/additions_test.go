package acptest_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
	"github.com/maccavelli/acp-go-sdk2/clienthost"
)

// clienthost.Terminals is a TerminalHost.
var _ acptest.TerminalHost = (*clienthost.Terminals)(nil)

// method is the "method" of a JSON-RPC frame, or "" for a response.
func method(t *testing.T, frame json.RawMessage) string {
	t.Helper()
	var m struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(frame, &m); err != nil {
		t.Fatalf("frame %s: %v", frame, err)
	}
	return m.Method
}

// TestConnectRecorded requires ConnectRecorded to record both directions of
// the client side, each frame with the way it went, in the order the client
// wrote and read them; Frames to be the records' frames; Wait to return once
// the frames are there; and ValidateGolden to pass a real session's records
// (0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phase 13).
func TestConnectRecorded(t *testing.T) {
	t.Parallel()
	var rec acptest.Recorder
	agent := &acptest.ScriptAgent{Turns: []acptest.Turn{{
		Updates: []acp.SessionUpdate{acp.UpdateAgentMessageText("one")}, StopReason: acp.StopReasonEndTurn,
	}}}
	pair := acptest.ConnectRecorded(t, agent, &acptest.MemoryClient{}, &rec)
	ctx := t.Context()
	if _, err := pair.Client.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		t.Fatal(err)
	}
	sess, err := pair.Client.NewSession(ctx, acp.NewSessionRequest{Cwd: acptest.Abs("/work"), McpServers: []acp.McpServer{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pair.Client.Prompt(ctx, acp.PromptRequest{SessionId: sess.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}}); err != nil {
		t.Fatal(err)
	}
	// initialize, session/new and session/prompt, each with its answer, and
	// the turn's update.
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := rec.Wait(waitCtx, 7); err != nil {
		t.Fatalf("Wait(7) = %v; %d frames", err, len(rec.Frames()))
	}
	records := rec.Records()
	var got []string
	for _, r := range records {
		got = append(got, string(r.Direction)+" "+method(t, r.Frame))
	}
	want := []string{"sent initialize", "received ", "sent session/new", "received ", "sent session/prompt", "received session/update", "received "}
	if !slices.Equal(got, want) {
		t.Errorf("records:\n %q\nwant\n %q", got, want)
	}
	frames := rec.Frames()
	for i, r := range records {
		if string(frames[i]) != string(r.Frame) {
			t.Errorf("Frames()[%d] = %s, Records()[%d] = %s", i, frames[i], i, r.Frame)
		}
	}
	acptest.ValidateGolden(t, records)
}

// TestRecorderWait requires Wait to return its context's cause while too few
// frames are recorded, and nil once enough are.
func TestRecorderWait(t *testing.T) {
	t.Parallel()
	var rec acptest.Recorder
	why := errors.New("the test's bound")
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(why)
	if err := rec.Wait(ctx, 1); !errors.Is(err, why) {
		t.Errorf("Wait(1) of an empty recorder = %v, want its context's cause", err)
	}
	w := rec.Writer(&strings.Builder{})
	done := make(chan error, 1)
	go func() { done <- rec.Wait(context.Background(), 2) }()
	_, _ = w.Write([]byte("{\"a\":1}\n{\"b\""))
	select {
	case err := <-done:
		t.Fatalf("Wait(2) returned %v after one frame", err)
	case <-time.After(100 * time.Millisecond):
	}
	_, _ = w.Write([]byte(":2}\n"))
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Wait(2) = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Wait(2) did not return after the second frame")
	}
	if err := rec.Wait(context.Background(), 0); err != nil {
		t.Errorf("Wait(0) = %v, want nil at once", err)
	}
}

// errorsTB is a testing.TB that keeps its errors, for a test to read.
type errorsTB struct {
	testing.TB
	mu   sync.Mutex
	errs []string
}

func (e *errorsTB) Helper() {}

func (e *errorsTB) Errorf(format string, args ...any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.errs = append(e.errs, fmt.Sprintf(format, args...))
}

// TestValidateGoldenRefuses requires ValidateGolden to fail each frame the
// protocol does not define, naming why, and to pass an extension method's
// and a well-formed exchange (0041-MADR-close-the-gaps-the-consumers-wait-on.md,
// item 8).
func TestValidateGoldenRefuses(t *testing.T) {
	t.Parallel()
	sent := func(frame string) acptest.Record {
		return acptest.Record{Direction: acptest.Sent, Frame: json.RawMessage(frame)}
	}
	received := func(frame string) acptest.Record {
		return acptest.Record{Direction: acptest.Received, Frame: json.RawMessage(frame)}
	}
	initialize := sent(`{"id":1,"jsonrpc":"2.0","method":"initialize","params":{"clientCapabilities":{},"protocolVersion":1}}`)
	for _, c := range []struct {
		name    string
		records []acptest.Record
		want    string
	}{
		{"a value its type refuses", []acptest.Record{sent(`{"id":1,"jsonrpc":"2.0","method":"session/new","params":{"cwd":"relative","mcpServers":[]}}`)},
			"session/new params:"},
		{"an unknown method", []acptest.Record{sent(`{"id":1,"jsonrpc":"2.0","method":"session/frobnicate","params":{}}`)},
			`unknown method "session/frobnicate"`},
		{"a method the wrong way", []acptest.Record{received(`{"id":1,"jsonrpc":"2.0","method":"session/prompt","params":{}}`)},
			`unknown method "session/prompt", received`},
		{"a response to no recorded request", []acptest.Record{received(`{"id":99,"jsonrpc":"2.0","result":{}}`)},
			"a response to no recorded request: id 99"},
		{"a response the same way as its request", []acptest.Record{initialize, sent(`{"id":1,"jsonrpc":"2.0","result":{}}`)},
			"a response to no recorded request: id 1"},
		{"a result its type refuses", []acptest.Record{initialize, received(`{"id":1,"jsonrpc":"2.0","result":{"agentCapabilities":{}}}`)},
			"initialize result:"},
		{"a notification sent as a request", []acptest.Record{sent(`{"id":1,"jsonrpc":"2.0","method":"session/cancel","params":{"sessionId":"s"}}`)},
			"is a notification, and was sent as a request"},
		{"a request sent without an id", []acptest.Record{sent(`{"jsonrpc":"2.0","method":"session/new","params":{}}`)},
			"is a request, and was sent without an id"},
		{"an error without a code", []acptest.Record{initialize, received(`{"error":{"message":"m"},"id":1,"jsonrpc":"2.0"}`)},
			"no integer code"},
		{"not JSON", []acptest.Record{sent(`not json`)}, "not a JSON object"},
		{"not JSON-RPC 2.0", []acptest.Record{sent(`{"id":1,"method":"initialize"}`)}, `"jsonrpc" is not "2.0"`},
		{"neither", []acptest.Record{sent(`{"jsonrpc":"2.0"}`)}, "neither a request, a notification nor a response"},
		{"an unknown direction", []acptest.Record{{Direction: "sideways", Frame: json.RawMessage(`{}`)}}, `direction "sideways"`},
	} {
		tb := &errorsTB{TB: t}
		acptest.ValidateGolden(tb, c.records)
		if len(tb.errs) != 1 || !strings.Contains(tb.errs[0], c.want) {
			t.Errorf("%s: ValidateGolden reported %q, want one error naming %q", c.name, tb.errs, c.want)
		}
	}
	tb := &errorsTB{TB: t}
	acptest.ValidateGolden(tb, []acptest.Record{
		sent(`{"jsonrpc":"2.0","method":"_x/notice","params":{"any":"thing"}}`),
		sent(`{"id":7,"jsonrpc":"2.0","method":"_x/ask","params":[1]}`),
		received(`{"id":7,"jsonrpc":"2.0","result":"anything"}`),
		initialize,
		received(`{"error":{"code":-32603,"message":"Internal error"},"id":1,"jsonrpc":"2.0"}`),
		received(`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":1}}`),
	})
	if len(tb.errs) != 0 {
		t.Errorf("ValidateGolden of extension frames, an error answer and a cancel_request reported %q, want nothing", tb.errs)
	}
}

// TestScriptAgentTurns requires a turn's Err to be the prompt's answer, its
// Usage to be reported, and a WaitCancel turn to wait until the session is
// cancelled and then answer cancelled.
func TestScriptAgentTurns(t *testing.T) {
	t.Parallel()
	usage := &acp.Usage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3} //nolint:staticcheck // SA1019: the turn's usage.
	agent := &acptest.ScriptAgent{Turns: []acptest.Turn{
		{Err: acp.NewInvalidParams(map[string]any{"why": "scripted"})},
		{StopReason: acp.StopReasonEndTurn, Usage: usage},
		{Updates: []acp.SessionUpdate{acp.UpdateAgentMessageText("working")}, WaitCancel: true},
	}}
	pair := acptest.Connect(t, agent, &acptest.MemoryClient{})
	ctx := t.Context()
	if _, err := pair.Client.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		t.Fatal(err)
	}
	sess, err := pair.Client.NewSession(ctx, acp.NewSessionRequest{Cwd: acptest.Abs("/work"), McpServers: []acp.McpServer{}})
	if err != nil {
		t.Fatal(err)
	}
	prompt := acp.PromptRequest{SessionId: sess.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}}
	if _, err := pair.Client.Prompt(ctx, prompt); code(err) != -32602 {
		t.Errorf("the Err turn's prompt = %v, want the scripted invalid params (-32602)", err)
	}
	resp, err := pair.Client.Prompt(ctx, prompt)
	if err != nil || resp.Usage == nil || resp.Usage.TotalTokens != 3 || resp.StopReason != acp.StopReasonEndTurn { //nolint:staticcheck // SA1019: the turn's usage.
		t.Errorf("the Usage turn's prompt = %+v, %v; want end_turn and the usage", resp, err)
	}
	answered := make(chan acp.PromptResponse, 1)
	go func() {
		resp, _ := pair.Client.Prompt(ctx, prompt)
		answered <- resp
	}()
	select {
	case resp := <-answered:
		t.Fatalf("the WaitCancel turn answered %+v before the cancel", resp)
	case <-time.After(300 * time.Millisecond):
	}
	if err := pair.Client.Cancel(ctx, acp.CancelNotification{SessionId: sess.SessionId}); err != nil {
		t.Fatal(err)
	}
	select {
	case resp := <-answered:
		if resp.StopReason != acp.StopReasonCancelled {
			t.Errorf("the WaitCancel turn answered %+v, want cancelled", resp)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the WaitCancel turn did not answer once cancelled")
	}
}

// TestMemoryClientElicitation requires CreateElicitation to answer from the
// script, a form's accept that the form refuses to reach the agent as an
// internal error, and no script to answer method-not-found.
func TestMemoryClientElicitation(t *testing.T) {
	t.Parallel()
	client := &acptest.MemoryClient{}
	pair := acptest.Connect(t, &acptest.ScriptAgent{}, client)
	ctx := t.Context()
	caps := acp.ClientCapabilities{Elicitation: &acp.ElicitationCapabilities{Form: &acp.ElicitationFormCapabilities{}}}
	if _, err := pair.Client.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber, ClientCapabilities: caps}); err != nil {
		t.Fatal(err)
	}
	var req acp.CreateElicitationRequest
	if err := json.Unmarshal([]byte(`{"sessionId":"s","mode":"form","message":"Your name?",`+
		`"requestedSchema":{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}}`), &req); err != nil {
		t.Fatal(err)
	}
	if _, err := pair.Agent.CreateElicitation(ctx, req); !acp.IsMethodNotFound(err) {
		t.Errorf("CreateElicitation with no script = %v, want method-not-found", err)
	}
	answer := map[string]acp.ElicitationContentValue{"name": acp.NewElicitationContentValueString("Ada")}
	client.SetElicitation(func(acp.CreateElicitationRequest) acp.CreateElicitationResponse {
		return acp.CreateElicitationResponse{Accept: &acp.CreateElicitationAccept{Content: answer}}
	})
	resp, err := pair.Agent.CreateElicitation(ctx, req)
	if err != nil || resp.Accept == nil || resp.Kind() != "accept" {
		t.Errorf("CreateElicitation with a script = %+v, %v; want its accept", resp, err)
	}
	answer = map[string]acp.ElicitationContentValue{}
	if _, err := pair.Agent.CreateElicitation(ctx, req); code(err) != -32603 {
		t.Errorf("CreateElicitation answered without a required field = %v, want an internal error (-32603)", err)
	}
}

// recordingHost is a TerminalHost that answers each method and keeps its
// name.
type recordingHost struct {
	mu    sync.Mutex
	calls []string
}

func (h *recordingHost) call(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, name)
}

func (h *recordingHost) CreateTerminal(context.Context, acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	h.call("create")
	return acp.CreateTerminalResponse{TerminalId: "t1"}, nil
}

func (h *recordingHost) TerminalOutput(context.Context, acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	h.call("output")
	return acp.TerminalOutputResponse{Output: "out"}, nil
}

func (h *recordingHost) WaitForTerminalExit(context.Context, acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	h.call("wait_for_exit")
	return acp.WaitForTerminalExitResponse{}, nil
}

func (h *recordingHost) KillTerminal(context.Context, acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	h.call("kill")
	return acp.KillTerminalResponse{}, nil
}

func (h *recordingHost) ReleaseTerminal(context.Context, acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	h.call("release")
	return acp.ReleaseTerminalResponse{}, nil
}

// TestMemoryClientTerminals requires MemoryClient to serve the five terminal
// methods through its Terminals, and to answer method-not-found without them.
func TestMemoryClientTerminals(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	terminalCaps := acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber, ClientCapabilities: acp.ClientCapabilities{Terminal: true}}
	bare := acptest.Connect(t, &acptest.ScriptAgent{}, &acptest.MemoryClient{})
	if _, err := bare.Client.Initialize(ctx, terminalCaps); err != nil {
		t.Fatal(err)
	}
	if _, err := bare.Agent.CreateTerminal(ctx, acp.CreateTerminalRequest{SessionId: "s", Command: "ls"}); !acp.IsMethodNotFound(err) {
		t.Errorf("terminal/create without Terminals = %v, want method-not-found", err)
	}
	host := &recordingHost{}
	pair := acptest.Connect(t, &acptest.ScriptAgent{}, &acptest.MemoryClient{Terminals: host})
	if _, err := pair.Client.Initialize(ctx, terminalCaps); err != nil {
		t.Fatal(err)
	}
	created, err := pair.Agent.CreateTerminal(ctx, acp.CreateTerminalRequest{SessionId: "s", Command: "ls"})
	if err != nil || created.TerminalId != "t1" {
		t.Fatalf("terminal/create = %+v, %v", created, err)
	}
	id := created.TerminalId
	out, err := pair.Agent.TerminalOutput(ctx, acp.TerminalOutputRequest{SessionId: "s", TerminalId: id})
	if err != nil || out.Output != "out" {
		t.Errorf("terminal/output = %+v, %v", out, err)
	}
	_, _ = pair.Agent.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{SessionId: "s", TerminalId: id})
	_, _ = pair.Agent.KillTerminal(ctx, acp.KillTerminalRequest{SessionId: "s", TerminalId: id})
	_, _ = pair.Agent.ReleaseTerminal(ctx, acp.ReleaseTerminalRequest{SessionId: "s", TerminalId: id})
	host.mu.Lock()
	defer host.mu.Unlock()
	if want := []string{"create", "output", "wait_for_exit", "kill", "release"}; !slices.Equal(host.calls, want) {
		t.Errorf("Terminals' calls = %q, want %q", host.calls, want)
	}
}

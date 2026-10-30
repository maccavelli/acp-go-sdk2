package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// wire is a raw peer of a connection: it writes lines to it and reads the
// lines it writes back.
type wire struct {
	t     *testing.T
	w     io.Writer
	lines chan string
}

// send writes s and a newline.
func (p wire) send(s string) {
	p.t.Helper()
	if err := p.trySend(s); err != nil {
		p.t.Fatal(err)
	}
}

// trySend writes s and a newline, and returns the write's error, for a test
// whose connection may have ended, and closed its input, first.
func (p wire) trySend(s string) error {
	_, err := io.WriteString(p.w, s+"\n")
	return err
}

// answer is a received JSON-RPC response, its id raw.
type answer struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *RequestError   `json:"error"`
}

// next returns the next line the connection wrote, decoded.
func (p wire) next(what string) answer {
	p.t.Helper()
	line := within(p.t, p.lines, what)
	var a answer
	if err := json.Unmarshal([]byte(line), &a); err != nil {
		p.t.Fatalf("%s: %q: %v", what, line, err)
	}
	return a
}

// rawWire starts a connection from start, reading from and writing to a raw
// peer.
func rawWire(t *testing.T, start func(w io.Writer, r io.Reader)) wire {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	// stop ends the reader below when the test ends: closing the pipes does
	// not unblock a send to a full lines, so a test that leaves more than its
	// capacity unread would otherwise leak the reader
	// (0029-PLAN-unix-client-death-pdeathsig.md, the deviation after step 3).
	stop := make(chan struct{})
	t.Cleanup(func() {
		close(stop)
		_ = inW.Close()
		_ = outW.Close()
	})
	start(outW, inR)
	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(nil, 1<<20)
		for sc.Scan() {
			select {
			case lines <- sc.Text():
			case <-stop:
				return
			}
		}
	}()
	return wire{t: t, w: inW, lines: lines}
}

// TestRawWireReaderStopsAtCleanup leaves more of the connection's lines unread
// than rawWire's reader can hold, and ends: the package's leak guard fails the
// run if the reader is still blocked (0029-PLAN-unix-client-death-pdeathsig.md,
// the deviation after step 3).
func TestRawWireReaderStopsAtCleanup(t *testing.T) {
	t.Parallel()
	p := agentWire(t)
	for range cap(p.lines) + 4 {
		p.send(probe)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(p.lines) < cap(p.lines) {
		if time.Now().After(deadline) {
			t.Fatalf("the connection wrote %d of the %d answers within 5s", len(p.lines), cap(p.lines)+4)
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond) // the reader has read another answer, and waits to send it
}

// agentWire is a raw peer of an agent whose session/new answers "s".
func agentWire(t *testing.T) wire {
	t.Helper()
	return rawWire(t, func(w io.Writer, r io.Reader) {
		negotiate(t, NewAgentSideConnection(agentFuncs{NewSessionFunc: func(context.Context, NewSessionRequest) (NewSessionResponse, error) {
			return NewSessionResponse{SessionId: "s"}, nil
		}}, w, r), allAdvertised())
	})
}

// probe is a request for a method no connection has; its answer, -32601 with
// id "probe", shows that the lines before it were not answered.
const probe = `{"jsonrpc":"2.0","id":"probe","method":"nope/x"}`

// expectNoAnswer requires the next answer to be the probe's.
func expectNoAnswer(t *testing.T, p wire, line string) {
	t.Helper()
	p.send(line)
	p.send(probe)
	if a := p.next("the probe's answer"); string(a.ID) != `"probe"` || a.Error == nil || a.Error.Code != -32601 {
		t.Errorf("%s was answered: got %s %+v before the probe's answer", line, a.ID, a.Error)
	}
}

// TestJSONRPCErrorAnswers requires each line that is not a valid Request
// object to be answered as JSON-RPC 2.0 requires, with its id when it has a
// valid one and null otherwise, a request with a null id to be answered with
// null, and a valid notification not to be answered
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 7).
func TestJSONRPCErrorAnswers(t *testing.T) {
	t.Parallel()
	cwd, err := json.Marshal(testAbsPath())
	if err != nil {
		t.Fatal(err)
	}
	newSession := `"method":"session/new","params":{"cwd":` + string(cwd) + `,"mcpServers":[]}`
	p := agentWire(t)
	for _, tt := range []struct {
		line, id string
		code     int // 0: a result
	}{
		{`not json`, "null", -32700},
		{`{"jsonrpc":"2.0","method":5,"id":7}`, "7", -32600},
		{`{"jsonrpc":"1.0","id":8,` + newSession + `}`, "8", -32600},
		{`{"jsonrpc":"2.0","id":null,` + newSession + `}`, "null", 0},
		{`{"jsonrpc":"2.0"}`, "null", -32600},
		{`[]`, "null", -32600},
		{`{"jsonrpc":"2.0","id":9,"method":"session/new","params":5}`, "9", -32600},
		{`{"jsonrpc": "2.0", "method": 1, "params": "bar"}`, "null", -32600},
		{`{"jsonrpc":"2.0","id":{"a":1},"method":"x"}`, "null", -32600},
	} {
		p.send(tt.line)
		a := p.next(tt.line)
		switch {
		case string(a.ID) != tt.id:
			t.Errorf("%s: answered with id %s, want %s", tt.line, a.ID, tt.id)
		case tt.code == 0 && (a.Error != nil || !strings.Contains(string(a.Result), `"sessionId":"s"`)):
			t.Errorf("%s: answered %s %+v, want the session", tt.line, a.Result, a.Error)
		case tt.code != 0 && (a.Error == nil || a.Error.Code != tt.code):
			t.Errorf("%s: answered %+v, want %d", tt.line, a.Error, tt.code)
		}
	}
	expectNoAnswer(t, p, `{"jsonrpc":"2.0","method":"nope/x"}`)
}

// TestJSONRPCNoReplyToResponses requires an error response without an id,
// and a result that answers no request, to go unanswered
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 7).
func TestJSONRPCNoReplyToResponses(t *testing.T) {
	t.Parallel()
	p := agentWire(t)
	expectNoAnswer(t, p, `{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}`)
	expectNoAnswer(t, p, `{"jsonrpc":"2.0","id":99,"result":{}}`)
}

// TestMalformedResponseFailsCall requires a response with neither or both of
// result and error, another jsonrpc, or an error that is not an object, to
// fail the call it answers (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 7).
func TestMalformedResponseFailsCall(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		`{"jsonrpc":"2.0","id":%s}`,
		`{"jsonrpc":"2.0","id":%s,"result":{"sessionId":"s"},"error":{"code":1,"message":"x"}}`,
		`{"jsonrpc":"1.0","id":%s,"result":{"sessionId":"s"}}`,
		`{"jsonrpc":"2.0","id":%s,"error":"boom"}`,
	} {
		t.Run(reply, func(t *testing.T) {
			t.Parallel()
			var c *ClientSideConnection
			p := rawWire(t, func(w io.Writer, r io.Reader) { c = NewClientSideConnection(&clientFuncs{}, w, r) })
			negotiate(t, c, allAdvertised())
			done := make(chan error, 1)
			go func() {
				_, err := c.NewSession(t.Context(), NewSessionRequest{Cwd: testAbsPath()})
				done <- err
			}()
			req := p.next("the request")
			p.send(fmt.Sprintf(reply, req.ID))
			if err := within(t, done, "NewSession"); err == nil || !strings.Contains(err.Error(), "malformed response") {
				t.Errorf("NewSession = %v, want a malformed-response error", err)
			}
		})
	}
}

// lineOf returns a request for a method no connection has, padded to n bytes.
func lineOf(t *testing.T, id, n int) string {
	t.Helper()
	head, tail := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"nope/x","params":{"p":"`, id), `"}}`
	if pad := n - len(head) - len(tail); pad >= 0 {
		return head + strings.Repeat("a", pad) + tail
	}
	t.Fatalf("a line of %d bytes is too short", n)
	return ""
}

// TestMaxMessageSize requires a line over the limit to be answered -32600
// with id null and reading to go on, and a line at the limit to be read
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 7).
func TestMaxMessageSize(t *testing.T) {
	t.Parallel()
	notFound := func(_ context.Context, method string, _ json.RawMessage) (any, *RequestError) {
		return nil, NewMethodNotFound(method)
	}
	for _, tt := range []struct {
		name  string
		limit int
		opts  []ConnectionOption
	}{
		{"WithMaxMessageSize(1024)", 1024, []ConnectionOption{WithMaxMessageSize(1024)}},
		{"the default", defaultMaxMessageSize, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := rawWire(t, func(w io.Writer, r io.Reader) { NewConnection(notFound, w, r, tt.opts...) })
			p.send(lineOf(t, 1, tt.limit+1))
			if a := p.next("the overlong line's answer"); string(a.ID) != "null" || a.Error == nil || a.Error.Code != -32600 ||
				!strings.Contains(fmt.Sprint(a.Error.Data), fmt.Sprintf("exceeds %d bytes", tt.limit)) {
				t.Errorf("overlong line answered %s %+v, want -32600 with id null naming the limit", a.ID, a.Error)
			}
			p.send(lineOf(t, 2, tt.limit))
			if a := p.next("the line at the limit"); string(a.ID) != "2" || a.Error == nil || a.Error.Code != -32601 {
				t.Errorf("line at the limit answered %s %+v, want -32601 with id 2", a.ID, a.Error)
			}
		})
	}
}

// sizeRecorder is a reader that sends the size of the buffer each Read is
// given on sizes, then waits for stop and reports the end of input.
type sizeRecorder struct {
	sizes chan<- int
	stop  <-chan struct{}
}

func (r sizeRecorder) Read(p []byte) (int, error) {
	r.sizes <- len(p)
	<-r.stop
	return 0, io.EOF
}

// TestReceiveReadBufferSize requires the connection's reader to read through
// a buffer of 128 KiB: its first Read is given 131,072 bytes
// (0030-MADR-record-initialize-on-arrival.md, the amendment of 2026-10-21).
func TestReceiveReadBufferSize(t *testing.T) {
	t.Parallel()
	sizes, stop := make(chan int, 1), make(chan struct{})
	c := NewConnection(nil, io.Discard, sizeRecorder{sizes: sizes, stop: stop})
	t.Cleanup(func() {
		close(stop)
		_ = c.Close()
		_ = c.Wait()
	})
	if n := within(t, sizes, "the reader's first Read"); n != 131072 {
		t.Errorf("the reader's first Read was given %d bytes, want 131072", n)
	}
}

// TestParamsNeverSentAsNull requires params that encode as null to be left
// out, and params that encode as neither an object nor an array to be refused
// before anything is written (0019-PLAN-behaviour-fixes.md, B10).
func TestParamsNeverSentAsNull(t *testing.T) {
	t.Parallel()
	var c *Connection
	p := rawWire(t, func(w io.Writer, r io.Reader) { c = NewConnection(nil, w, r) })
	for _, params := range []any{map[string]any(nil), (*NewSessionRequest)(nil), json.RawMessage("null")} {
		if err := c.SendNotification(t.Context(), "_test/n", params); err != nil {
			t.Fatalf("SendNotification(%T): %v", params, err)
		}
		line := within(t, p.lines, "the notification")
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatal(err)
		}
		if _, ok := m["params"]; ok {
			t.Errorf("%T sent as %s: params present", params, line)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- c.SendRequestNoResult(ctx, "_test/r", map[string]any(nil)) }()
	line := within(t, p.lines, "the request")
	if strings.Contains(line, `"params"`) {
		t.Errorf("a request with nil params sent as %s", line)
	}
	cancel()
	<-done
	if line := within(t, p.lines, "the cancel request"); !strings.Contains(line, ProtocolMethodCancelRequest) {
		t.Fatalf("expected the abandoned request's %s, got %s", ProtocolMethodCancelRequest, line)
	}

	for _, params := range []any{"bar", 42, true} {
		err := c.SendNotification(t.Context(), "_test/n", params)
		if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != -32602 {
			t.Errorf("SendNotification(%#v) = %v, want -32602", params, err)
		}
		// The peer never answers: a request that is written ends at the
		// deadline instead of with -32602.
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		_, err = SendRequest[json.RawMessage](c, ctx, "_test/r", params)
		cancel()
		if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != -32602 {
			t.Fatalf("SendRequest(%#v) = %v, want -32602", params, err)
		}
	}
	if err := c.SendNotification(t.Context(), "_test/after", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if line := within(t, p.lines, "the next notification"); !strings.Contains(line, `"_test/after"`) {
		t.Fatalf("refused params were written: next line %s", line)
	}
}

// TestNullParamsReceivedAsOmitted requires a request or notification whose
// params are null to reach its handler as one whose params are left out, as
// ACP's schema allows, while any other value that is not an object or an
// array stays an invalid request
// (0019-MADR-remediate-debugging-pass-findings.md, decision 13).
func TestNullParamsReceivedAsOmitted(t *testing.T) {
	t.Parallel()
	got := make(chan json.RawMessage, 2)
	handler := func(_ context.Context, _ string, params json.RawMessage) (any, *RequestError) {
		got <- params
		return map[string]any{}, nil
	}
	p := rawWire(t, func(w io.Writer, r io.Reader) { NewConnection(handler, w, r) })
	p.send(`{"jsonrpc":"2.0","id":1,"method":"_test/r","params":null}`)
	if params := within(t, got, "the request's handler"); params != nil {
		t.Errorf("request handler got params %s, want none", params)
	}
	if a := p.next("the request's answer"); a.Error != nil {
		t.Errorf("a request with null params answered %+v", a.Error)
	}
	p.send(`{"jsonrpc":"2.0","method":"_test/n","params":null}`)
	if params := within(t, got, "the notification's handler"); params != nil {
		t.Errorf("notification handler got params %s, want none", params)
	}
	p.send(`{"jsonrpc":"2.0","id":2,"method":"_test/r","params":"bar"}`)
	if a := p.next("the answer to string params"); a.Error == nil || a.Error.Code != -32600 {
		t.Errorf("string params answered %+v, want -32600", a)
	}
}

// TestMalformedInputNotLogged requires a malformed message to be logged by its
// fault and size, never by its content
// (0019-PLAN-behaviour-fixes.md, B15).
func TestMalformedInputNotLogged(t *testing.T) {
	t.Parallel()
	const marker = "CONTENT-MARKER-4711"
	for _, line := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"x","params":{"text":"` + marker + `"}`,
		`["` + marker + `"]`,
		`{"jsonrpc":"1.0","id":2,"method":"x","params":{"text":"` + marker + `"}}`,
		`{"jsonrpc":"2.0","result":{"text":"` + marker + `"}}`,
	} {
		var logs syncBuffer
		r, w := io.Pipe()
		c := NewConnection(nil, io.Discard, r, WithLogger(slog.New(slog.NewTextHandler(&logs, nil))))
		if _, err := io.WriteString(w, line+"\n"); err != nil {
			t.Fatal(err)
		}
		_ = w.Close()
		select {
		case <-c.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("the connection did not end")
		}
		out := logs.String()
		if strings.Contains(out, marker) {
			t.Errorf("content of %.30s… logged: %s", line, out)
		}
		if !strings.Contains(out, "size=") {
			t.Errorf("%.30s… logged without its size: %s", line, out)
		}
	}
}

// paramsAgent is a raw peer of an agent whose logout and session/list report
// their calls on called.
func paramsAgent(t *testing.T, called chan<- string) wire {
	t.Helper()
	return rawWire(t, func(w io.Writer, r io.Reader) {
		negotiate(t, NewAgentSideConnection(agentFuncs{
			LogoutFunc: func(context.Context, LogoutRequest) (LogoutResponse, error) {
				called <- AgentMethodLogout
				return LogoutResponse{}, nil
			},
			ListSessionsFunc: func(context.Context, ListSessionsRequest) (ListSessionsResponse, error) {
				called <- AgentMethodSessionList
				return ListSessionsResponse{Sessions: []SessionInfo{}}, nil
			},
		}, w, r), allAdvertised())
	})
}

// checkParamsLeftOut sends logout, session/list and session/new with the
// given params member: the first two reach their handlers, which JSON-RPC 2.0
// and ACP's schema allow, and session/new is refused -32602 naming its
// missing cwd.
func checkParamsLeftOut(t *testing.T, member string) {
	t.Helper()
	called := make(chan string, 4)
	p := paramsAgent(t, called)
	for i, method := range []string{AgentMethodLogout, AgentMethodSessionList} {
		p.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":%q%s}`, i+1, method, member))
		if a := p.next(method + "'s answer"); a.Error != nil {
			t.Errorf("%s with params %q answered %+v", method, member, a.Error)
		}
		if got := within(t, called, method+"'s handler"); got != method {
			t.Errorf("handler %s called for %s", got, method)
		}
	}
	p.send(`{"jsonrpc":"2.0","id":3,"method":"session/new"` + member + `}`)
	a := p.next("session/new's answer")
	if a.Error == nil || a.Error.Code != -32602 || !strings.Contains(a.Error.Error(), "cwd") {
		t.Errorf("session/new with params %q answered %+v, want -32602 naming cwd", member, a.Error)
	}
}

// TestOmittedParams requires a request whose params are left out to decode
// as one with empty params (0019-PLAN-behaviour-fixes.md, B3).
func TestOmittedParams(t *testing.T) {
	t.Parallel()
	checkParamsLeftOut(t, "")
}

// TestNullParams requires "params": null to behave as params left out
// (0019-MADR-remediate-debugging-pass-findings.md, decision 13).
func TestNullParams(t *testing.T) {
	t.Parallel()
	checkParamsLeftOut(t, `,"params":null`)
}

// TestMethodKindMismatch requires a method received as the other kind to be
// refused: a request sent without an id is not handled, logged, and holds up
// nothing; a notification sent with an id is answered -32601. Each side knows
// the kind of its own methods, mcp/message among them
// (0019-PLAN-behaviour-fixes.md, B12).
func TestMethodKindMismatch(t *testing.T) {
	t.Parallel()
	var agentLogs, clientLogs syncBuffer
	prompted := make(chan struct{}, 1)
	cancelled := make(chan struct{}, 2)
	mcp := make(chan struct{}, 1)
	agent := rawWire(t, func(w io.Writer, r io.Reader) {
		negotiate(t, NewAgentSideConnection(agentFuncs{
			PromptFunc: func(context.Context, PromptRequest) (PromptResponse, error) {
				prompted <- struct{}{}
				return PromptResponse{StopReason: StopReasonEndTurn}, nil
			},
			CancelFunc: func(context.Context, CancelNotification) error {
				cancelled <- struct{}{}
				return nil
			},
			UnstableMessageMcpFunc: func(context.Context, UnstableMessageMcpNotification) error {
				mcp <- struct{}{}
				return nil
			},
		}, w, r, WithLogger(slog.New(slog.NewTextHandler(&agentLogs, nil)))), allAdvertised())
	})

	agent.send(`{"jsonrpc":"2.0","method":"session/prompt","params":{"sessionId":"s","prompt":[{"type":"text","text":"hi"}]}}`)
	agent.send(`{"jsonrpc":"2.0","method":"session/cancel","params":{"sessionId":"s"}}`)
	within(t, cancelled, "the session/cancel queued behind a prompt sent without an id")
	select {
	case <-prompted:
		t.Error("a session/prompt sent without an id ran its handler")
	default:
	}
	if !strings.Contains(agentLogs.String(), "session/prompt is a request, and it was sent without an id") {
		t.Errorf("the dropped session/prompt was not logged: %s", agentLogs.String())
	}

	agent.send(`{"jsonrpc":"2.0","id":9,"method":"session/cancel","params":{"sessionId":"s"}}`)
	if a := agent.next("the answer to session/cancel sent with an id"); a.Error == nil || a.Error.Code != -32601 {
		t.Errorf("session/cancel with an id answered %+v, want -32601", a)
	}
	agent.send(`{"jsonrpc":"2.0","id":10,"method":"mcp/message","params":{}}`)
	if a := agent.next("the answer to mcp/message sent to the agent with an id"); a.Error == nil || a.Error.Code != -32601 {
		t.Errorf("mcp/message to the agent with an id answered %+v, want -32601", a)
	}
	select {
	case <-cancelled:
		t.Error("session/cancel sent with an id ran its handler")
	case <-mcp:
		t.Error("mcp/message sent to the agent with an id ran its handler")
	default:
	}

	client := rawWire(t, func(w io.Writer, r io.Reader) {
		negotiate(t, NewClientSideConnection(&clientFuncs{}, w, r,
			WithLogger(slog.New(slog.NewTextHandler(&clientLogs, nil)))), allAdvertised())
	})
	expectNoAnswer(t, client, `{"jsonrpc":"2.0","method":"mcp/message","params":{}}`)
	// The probe's answer can come before the notification queue reaches the
	// dropped message, so its log line is waited for.
	const dropped = "mcp/message is a request, and it was sent without an id"
	for deadline := time.Now().Add(2 * time.Second); !strings.Contains(clientLogs.String(), dropped); {
		if time.Now().After(deadline) {
			t.Fatalf("the dropped mcp/message was not logged: %s", clientLogs.String())
		}
		time.Sleep(time.Millisecond)
	}
}

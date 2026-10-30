package remote

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// httpVariants are the three ways the server is reached: HTTP/1.1, HTTP/2
// over TLS, and unencrypted HTTP/2. Each starts h and returns its URL and the
// client options that reach it.
var httpVariants = []struct {
	name  string
	start func(t *testing.T, h *Handler) (string, *DialOptions)
}{
	{"http1", func(t *testing.T, h *Handler) (string, *DialOptions) {
		t.Helper()
		return testServer(t, h).URL, nil
	}},
	{"tls-h2", func(t *testing.T, h *Handler) (string, *DialOptions) {
		t.Helper()
		ts := httptest.NewUnstartedServer(h)
		ts.EnableHTTP2 = true
		ts.StartTLS()
		t.Cleanup(func() { _ = h.Close(context.Background()); ts.Close() })
		return ts.URL, &DialOptions{HTTPClient: ts.Client()}
	}},
	{"h2c", func(t *testing.T, h *Handler) (string, *DialOptions) {
		t.Helper()
		ts := httptest.NewUnstartedServer(h)
		var sp, cp http.Protocols
		sp.SetHTTP1(true)
		sp.SetUnencryptedHTTP2(true)
		cp.SetUnencryptedHTTP2(true)
		ts.Config.Protocols = &sp
		ts.Start()
		t.Cleanup(func() { _ = h.Close(context.Background()); ts.Close() })
		return ts.URL, &DialOptions{HTTPClient: &http.Client{Transport: &http.Transport{Protocols: &cp}}}
	}},
}

// TestHTTPServerRoundTrip requires phase 4's round trip, permission and
// cancellations over the Streamable HTTP server, on each HTTP version
// (0043-PLAN-remote-http-and-websocket-transport.md, phase 6).
func TestHTTPServerRoundTrip(t *testing.T) {
	t.Parallel()
	for _, v := range httpVariants {
		t.Run(v.name, func(t *testing.T) {
			t.Parallel()
			httpRoundTrip(t, v.name, v.start)
		})
	}
}

// httpRoundTrip runs the round trip, the permission and the cancellations
// against servers start makes.
func httpRoundTrip(t *testing.T, name string, start func(*testing.T, *Handler) (string, *DialOptions)) {
	t.Helper()
	{
		url, opts := start(t, NewHandler(serveAgents(func() acp.Agent {
			return &acptest.ScriptAgent{Turns: []acptest.Turn{{
				Updates:    []acp.SessionUpdate{acp.UpdateAgentMessageText("one"), acp.UpdateAgentMessageText("two")},
				StopReason: acp.StopReasonEndTurn,
			}}}
		}), nil))
		client := &acptest.MemoryClient{}
		conn, _ := dialClient(t, url, client, opts)
		sid := startSession(t, conn)
		resp, err := conn.Prompt(t.Context(), acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}})
		if err != nil || resp.StopReason != acp.StopReasonEndTurn || len(client.Updates()) != 2 {
			t.Errorf("%s: Prompt = %v, %v, %d updates; want end_turn and 2", name, resp.StopReason, err, len(client.Updates()))
		}

		url, opts = start(t, NewHandler(serveAgents(func() acp.Agent { return &permissionAgent{} }), nil))
		pc := &acptest.MemoryClient{}
		pc.SetPermission("call-1", acp.NewRequestPermissionOutcomeSelected("allow"))
		conn, _ = dialClient(t, url, pc, opts)
		sid = startSession(t, conn)
		if resp, err := conn.Prompt(t.Context(), acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}}); err != nil || resp.StopReason != acp.StopReasonEndTurn {
			t.Errorf("%s: a prompt asking permission = %v, %v; want end_turn", name, resp.StopReason, err)
		}

		agents := make(chan *modeAgent, 1)
		url, opts = start(t, NewHandler(serveAgents(func() acp.Agent {
			a := &modeAgent{started: make(chan struct{})}
			a.Turns = []acptest.Turn{{Updates: []acp.SessionUpdate{acp.UpdateAgentMessageText("wait")}, WaitCancel: true}}
			agents <- a
			return a
		}), nil))
		cc := &acptest.MemoryClient{}
		mc, err := Dial(t.Context(), url, opts)
		if err != nil {
			t.Fatal(err)
		}
		received := &lineRecorder{}
		conn = acp.NewClientSideConnection(cc, mc, io.TeeReader(mc, received))
		t.Cleanup(func() {
			_ = mc.Close()
			<-conn.Done()
		})
		sid = startSession(t, conn)
		agent := <-agents
		done := make(chan acp.StopReason, 1)
		go func() {
			resp, _ := conn.Prompt(context.Background(), acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock("x")}})
			done <- resp.StopReason
		}()
		waitFor(t, name+": the waiting turn's update", func() bool { return len(cc.Updates()) == 1 })
		if err := conn.Cancel(t.Context(), acp.CancelNotification{SessionId: sid}); err != nil {
			t.Fatal(err)
		}
		if r := <-done; r != acp.StopReasonCancelled {
			t.Errorf("%s: a prompt ended by session/cancel stopped %q, want cancelled", name, r)
		}
		mctx, mcancel := context.WithCancel(t.Context())
		modeDone := make(chan error, 1)
		go func() {
			_, err := conn.SetSessionMode(mctx, acp.SetSessionModeRequest{SessionId: sid, ModeId: "plan"})
			modeDone <- err
		}()
		<-agent.started
		mcancel()
		if err := <-modeDone; !errors.Is(err, context.Canceled) {
			t.Errorf("%s: SetSessionMode under a cancelled context = %v, want context.Canceled", name, err)
		}
		waitFor(t, name+": the -32800 answer to session/set_mode", func() bool {
			_, ok := received.find(`"code":-32800`)
			return ok
		})
	}
}

// TestHTTPServerStatuses requires each refusal of the routing tree.
func TestHTTPServerStatuses(t *testing.T) {
	t.Parallel()
	h := NewHandler(scripted(nil), &HandlerOptions{MaxMessageSize: 1024})
	ts := testServer(t, h)
	none := &rawConn{t: t, url: ts.URL, client: http.DefaultClient}
	c := rawInit(t, ts.URL, nil)
	long := strings.Repeat("x", maxIDLength+1)
	for _, tc := range []struct {
		name   string
		conn   *rawConn
		method string
		body   string
		header map[string]string
		want   int
	}{
		{"not JSON's media type", c, http.MethodPost, `{}`, map[string]string{"Content-Type": "text/plain"}, http.StatusUnsupportedMediaType},
		{"over MaxMessageSize", c, http.MethodPost, `{"jsonrpc":"2.0","method":"x","params":{"p":"` + strings.Repeat("y", 1100) + `"}}`, nil, http.StatusRequestEntityTooLarge},
		{"not JSON", c, http.MethodPost, `nope`, nil, http.StatusBadRequest},
		{"a batch", c, http.MethodPost, `[{"jsonrpc":"2.0","method":"x"}]`, nil, http.StatusNotImplemented},
		{"not a JSON-RPC message", c, http.MethodPost, `{"x":1}`, nil, http.StatusBadRequest},
		{"an id too long", c, http.MethodPost, `{"jsonrpc":"2.0","id":"` + long[:900] + `","method":"x"}`, map[string]string{sessionIDHeader: long}, http.StatusBadRequest},
		{"no connection header", none, http.MethodPost, `{"jsonrpc":"2.0","id":2,"method":"session/new","params":{}}`, nil, http.StatusBadRequest},
		{"initialize on a connection", c, http.MethodPost, `{"jsonrpc":"2.0","id":2,"method":"initialize","params":{}}`, nil, http.StatusBadRequest},
		{"unknown connection", c, http.MethodPost, `{"jsonrpc":"2.0","method":"x"}`, map[string]string{connectionIDHeader: "nope"}, http.StatusNotFound},
		{"a session method with no session", c, http.MethodPost, `{"jsonrpc":"2.0","id":3,"method":"session/prompt","params":{"prompt":[]}}`, nil, http.StatusBadRequest},
		{"header and params differ", c, http.MethodPost, `{"jsonrpc":"2.0","id":4,"method":"session/prompt","params":{"sessionId":"a","prompt":[]}}`, map[string]string{sessionIDHeader: "b"}, http.StatusBadRequest},
		{"PUT", c, http.MethodPut, ``, nil, http.StatusMethodNotAllowed},
		{"OPTIONS", c, http.MethodOptions, ``, nil, http.StatusMethodNotAllowed},
		{"GET not accepting events", c, http.MethodGet, ``, map[string]string{"Accept": "application/json"}, http.StatusNotAcceptable},
		{"GET with no connection header", none, http.MethodGet, ``, map[string]string{"Accept": eventStreamType}, http.StatusBadRequest},
		{"GET for an unknown connection", c, http.MethodGet, ``, map[string]string{"Accept": eventStreamType, connectionIDHeader: "nope"}, http.StatusNotFound},
		{"DELETE with no connection header", none, http.MethodDelete, ``, nil, http.StatusBadRequest},
		{"DELETE of an unknown connection", c, http.MethodDelete, ``, map[string]string{connectionIDHeader: "nope"}, http.StatusNotFound},
	} {
		if got, _, body := tc.conn.do(tc.method, tc.body, tc.header); got != tc.want {
			t.Errorf("%s: %d %s, want %d", tc.name, got, strings.TrimSpace(body), tc.want)
		}
	}
	first := c.stream("")
	if first.status != http.StatusOK {
		t.Fatalf("the connection stream = %d, want 200", first.status)
	}
	if second := c.stream(""); second.status != http.StatusConflict {
		t.Errorf("a second receiver for the connection stream = %d, want 409", second.status)
	}
}

// TestHTTPServerClientShapes requires the requests of each official client
// to be served: session/load on a fresh connection with the header
// (TypeScript, Rust) and with params.sessionId alone (Python), and a response
// to the agent's request with and without the header.
func TestHTTPServerClientShapes(t *testing.T) {
	t.Parallel()
	ts := testServer(t, NewHandler(scripted(func(m message, _ []byte, write func(string)) bool {
		switch m.method {
		case "session/load":
			write(`{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"` + m.sessionID + `","update":{}}}`)
			write(`{"jsonrpc":"2.0","id":` + m.idKey + `,"result":{}}`)
		case "session/prompt":
			write(`{"jsonrpc":"2.0","id":"p-` + m.idKey + `","method":"session/request_permission","params":{"sessionId":"` + m.sessionID + `"}}`)
			write(`{"jsonrpc":"2.0","id":` + m.idKey + `,"result":{"stopReason":"end_turn"}}`)
		default:
			return false
		}
		return true
	}), nil))
	for _, shape := range []struct {
		name   string
		header bool
	}{{"with the header", true}, {"params alone", false}} {
		c := rawInit(t, ts.URL, nil)
		conn := c.stream("")
		header := ""
		if shape.header {
			header = "s9"
		}
		sess := c.stream("s9")
		id, status := c.request("session/load", `{"sessionId":"s9","cwd":"/w","mcpServers":[]}`, header)
		if status != http.StatusAccepted {
			t.Fatalf("%s: session/load = %d, want 202", shape.name, status)
		}
		if ev := sess.next(t); !strings.Contains(ev, `"session/update"`) {
			t.Errorf("%s: the session stream's first event = %s, want the replay", shape.name, ev)
		}
		if ev := conn.next(t); classify([]byte(ev)).idKey != id {
			t.Errorf("%s: the connection stream's event = %s, want session/load's answer", shape.name, ev)
		}
		pid, status := c.request("session/prompt", `{"sessionId":"s9","prompt":[]}`, header)
		if status != http.StatusAccepted {
			t.Fatalf("%s: session/prompt = %d, want 202", shape.name, status)
		}
		req := classify([]byte(sess.next(t)))
		if req.method != "session/request_permission" {
			t.Fatalf("%s: the session stream's event = %+v, want the permission request", shape.name, req)
		}
		answer := `{"jsonrpc":"2.0","id":` + req.idKey + `,"result":{"outcome":{"outcome":"cancelled"}}}`
		if status := c.post(answer, "other"); status != http.StatusBadRequest {
			t.Errorf("%s: the permission answer with another session's header = %d, want 400", shape.name, status)
		}
		if status := c.post(answer, header); status != http.StatusAccepted {
			t.Errorf("%s: the permission answer = %d, want 202", shape.name, status)
		}
		if ev := sess.next(t); classify([]byte(ev)).idKey != pid {
			t.Errorf("%s: the session stream's event = %s, want the prompt's answer", shape.name, ev)
		}
	}
}

// TestHTTPServerInitialize requires initialize's four outcomes: a result,
// an error, a serve that returns first, and no answer within the bound; each
// but the first ending the serve.
func TestHTTPServerInitialize(t *testing.T) {
	t.Parallel()
	ended := make(chan string, 4)
	newHandler := func(answer string, bound time.Duration) string {
		h := NewHandler(func(ctx context.Context, in io.Reader, out io.Writer) error {
			defer func() { ended <- answer }()
			if answer == "return" {
				return nil
			}
			line := make([]byte, 4096)
			n, _ := in.Read(line)
			m := classify(line[:n])
			switch answer {
			case "result":
				_, _ = io.WriteString(out, `{"jsonrpc":"2.0","id":`+m.idKey+`,"result":{"protocolVersion":1}}`+"\n")
			case "error":
				_, _ = io.WriteString(out, `{"jsonrpc":"2.0","id":`+m.idKey+`,"error":{"code":-32602,"message":"Invalid params"}}`+"\n")
			}
			_, _ = io.Copy(io.Discard, in)
			<-ctx.Done()
			return nil
		}, &HandlerOptions{Grace: 50 * time.Millisecond})
		if bound > 0 {
			h.initBound = bound
		}
		return testServer(t, h).URL
	}
	init := `{"jsonrpc":"2.0","id":7,"method":"initialize","params":{"protocolVersion":1}}`
	for _, tc := range []struct {
		answer     string
		bound      time.Duration
		wantStatus int
		wantID     bool
		wantBody   string
		ends       bool
	}{
		{"result", 0, http.StatusOK, true, `"result"`, false},
		{"error", 0, http.StatusOK, false, `"code":-32602`, true},
		{"return", 0, http.StatusInternalServerError, false, `"id":7,"error":{"code":-32603`, true},
		{"silent", 100 * time.Millisecond, http.StatusGatewayTimeout, false, "", true},
	} {
		c := &rawConn{t: t, url: newHandler(tc.answer, tc.bound), client: http.DefaultClient}
		status, header, body := c.do(http.MethodPost, init, nil)
		if status != tc.wantStatus || (header.Get(connectionIDHeader) != "") != tc.wantID || !strings.Contains(body, tc.wantBody) {
			t.Errorf("%s: initialize = %d, id %q, %s; want %d, id %v, a body holding %s", tc.answer, status, header.Get(connectionIDHeader), body, tc.wantStatus, tc.wantID, tc.wantBody)
		}
		if tc.ends {
			select {
			case <-ended:
			case <-time.After(waitBound):
				t.Errorf("%s: the serve did not end", tc.answer)
			}
		}
	}
}

// TestHTTPServerRouting requires two sessions on one connection to keep their
// updates, requests and answers on their own streams, and the connection's
// messages, session/new's answers and a $/ notification naming a session, on
// the connection's.
func TestHTTPServerRouting(t *testing.T) {
	t.Parallel()
	ts := testServer(t, NewHandler(scripted(func(m message, _ []byte, write func(string)) bool {
		switch m.method {
		case "session/fork":
			write(`{"jsonrpc":"2.0","id":` + m.idKey + `,"result":{"sessionId":"forked"}}`)
			return true
		case "session/resume":
			write(`{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"` + m.sessionID + `","update":{}}}`)
			write(`{"jsonrpc":"2.0","id":` + m.idKey + `,"result":{}}`)
			return true
		case "session/prompt":
		default:
			return false
		}
		s := m.sessionID
		write(`{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"` + s + `","update":{}}}`)
		write(`{"jsonrpc":"2.0","id":"q-` + s + `","method":"session/request_permission","params":{"sessionId":"` + s + `"}}`)
		write(`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":"q-` + s + `","sessionId":"` + s + `"}}`)
		write(`{"jsonrpc":"2.0","id":` + m.idKey + `,"result":{"stopReason":"end_turn"}}`)
		return true
	}), nil))
	c := rawInit(t, ts.URL, nil)
	conn := c.stream("")
	var sessions []string
	for range 2 {
		id, status := c.request("session/new", `{"cwd":"/w","mcpServers":[]}`, "")
		ev := conn.next(t)
		if status != http.StatusAccepted || classify([]byte(ev)).idKey != id {
			t.Fatalf("session/new = %d, the connection stream's event %s; want 202 and its answer", status, ev)
		}
		sessions = append(sessions, sessionOfAnswer(t, ev))
	}
	streams := map[string]*rawStream{}
	for _, s := range sessions {
		streams[s] = c.stream(s)
	}
	ids := map[string]string{}
	for _, s := range sessions {
		ids[s], _ = c.request("session/prompt", `{"sessionId":"`+s+`","prompt":[]}`, s)
	}
	for _, s := range sessions {
		got := []message{}
		for range 3 {
			got = append(got, classify([]byte(streams[s].next(t))))
		}
		if got[0].method != "session/update" || got[0].sessionID != s || got[1].method != "session/request_permission" ||
			got[2].kind != kindResponse || got[2].idKey != ids[s] {
			t.Errorf("session %s's stream = %+v, want its update, its request and its answer", s, got)
		}
	}
	for range 2 {
		if m := classify([]byte(conn.next(t))); m.method != "$/cancel_request" {
			t.Errorf("the connection stream's event = %+v, want the $/ notification", m)
		}
	}

	// session/fork's and session/resume's answers go to the connection
	// stream, as session/new's and session/load's do; the resumed session's
	// update goes to its own.
	forkID, status := c.request("session/fork", `{"sessionId":"`+sessions[0]+`","cwd":"/w","mcpServers":[]}`, sessions[0])
	if m := classify([]byte(conn.next(t))); status != http.StatusAccepted || m.kind != kindResponse || m.idKey != forkID {
		t.Errorf("session/fork = %d, the connection stream's event %+v; want 202 and its answer", status, m)
	}
	resumed := c.stream("resumed")
	resumeID, status := c.request("session/resume", `{"sessionId":"resumed","cwd":"/w","mcpServers":[]}`, "resumed")
	if m := classify([]byte(conn.next(t))); status != http.StatusAccepted || m.kind != kindResponse || m.idKey != resumeID {
		t.Errorf("session/resume = %d, the connection stream's event %+v; want 202 and its answer", status, m)
	}
	if m := classify([]byte(resumed.next(t))); m.method != "session/update" || m.sessionID != "resumed" {
		t.Errorf("the resumed session's stream = %+v, want its update", m)
	}
}

// TestHTTPServerHeld requires a session's messages, routed before its stream
// opens, to be held and sent in order once it does.
func TestHTTPServerHeld(t *testing.T) {
	t.Parallel()
	ts := testServer(t, NewHandler(scripted(func(m message, _ []byte, write func(string)) bool {
		if m.method != "session/prompt" {
			return false
		}
		for i := range 5 {
			write(`{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"` + m.sessionID + `","n":` + string(rune('0'+i)) + `}}`)
		}
		write(`{"jsonrpc":"2.0","id":` + m.idKey + `,"result":{"stopReason":"end_turn"}}`)
		return true
	}), nil))
	c := rawInit(t, ts.URL, nil)
	c.stream("")
	id, _ := c.request("session/prompt", `{"sessionId":"late","prompt":[]}`, "late")
	s := c.stream("late")
	for i := range 5 {
		if ev := s.next(t); !strings.Contains(ev, `"n":`+string(rune('0'+i))) {
			t.Fatalf("held event %d = %s, want n %d", i, ev, i)
		}
	}
	if ev := s.next(t); classify([]byte(ev)).idKey != id {
		t.Errorf("the last held event = %s, want the prompt's answer", ev)
	}
}

// TestHTTPServerOverflow requires a connection holding more than
// MaxBuffered for a stream nobody receives to end with ErrOverflow.
func TestHTTPServerOverflow(t *testing.T) {
	t.Parallel()
	causes := make(chan error, 1)
	ts := testServer(t, NewHandler(func(ctx context.Context, in io.Reader, out io.Writer) error {
		_ = scripted(func(m message, _ []byte, write func(string)) bool {
			if m.method != "session/prompt" {
				return false
			}
			for range 100 {
				write(`{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"nobody","pad":"` + strings.Repeat("z", 1024) + `"}}`)
			}
			return true
		})(ctx, in, out)
		<-ctx.Done()
		causes <- context.Cause(ctx)
		return nil
	}, &HandlerOptions{MaxBuffered: 64 * 1024, Grace: 50 * time.Millisecond}))
	c := rawInit(t, ts.URL, nil)
	c.stream("")
	c.request("session/prompt", `{"sessionId":"nobody","prompt":[]}`, "nobody")
	select {
	case cause := <-causes:
		if !errors.Is(cause, ErrOverflow) {
			t.Errorf("the serve's cause = %v, want ErrOverflow", cause)
		}
	case <-time.After(waitBound):
		t.Fatal("holding over MaxBuffered did not end the connection")
	}
}

// TestHTTPServerProvisional requires a stream opened before session/load to
// receive its replay, one nothing names to close after Grace, and one whose
// load is answered with an error to close.
func TestHTTPServerProvisional(t *testing.T) {
	t.Parallel()
	ts := testServer(t, NewHandler(scripted(func(m message, _ []byte, write func(string)) bool {
		if m.method != "session/load" {
			return false
		}
		if m.sessionID == "bad" {
			write(`{"jsonrpc":"2.0","id":` + m.idKey + `,"error":{"code":-32002,"message":"Resource not found"}}`)
			return true
		}
		write(`{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"` + m.sessionID + `","update":{}}}`)
		write(`{"jsonrpc":"2.0","id":` + m.idKey + `,"result":{}}`)
		return true
	}), &HandlerOptions{Grace: 100 * time.Millisecond}))
	c := rawInit(t, ts.URL, nil)
	c.stream("")
	good := c.stream("good")
	c.request("session/load", `{"sessionId":"good","cwd":"/w","mcpServers":[]}`, "good")
	if ev := good.next(t); !strings.Contains(ev, "session/update") {
		t.Errorf("the stream opened before the load = %s, want its replay", ev)
	}
	c.stream("ghost").ended(t, "a stream nothing names")
	bad := c.stream("bad")
	c.request("session/load", `{"sessionId":"bad","cwd":"/w","mcpServers":[]}`, "bad")
	bad.ended(t, "a stream whose load failed")
	select {
	case e, ok := <-good.events:
		if ok || e != "" {
			t.Errorf("the loaded session's stream = %s, %v; want it open with no event", e, ok)
		} else {
			t.Error("the loaded session's stream ended, want it open")
		}
	case <-time.After(300 * time.Millisecond):
	}
}

// TestHTTPServerDelete requires DELETE to end the serve with ErrDeleted,
// close the streams, and make the connection unknown.
func TestHTTPServerDelete(t *testing.T) {
	t.Parallel()
	causes := make(chan error, 1)
	ts := testServer(t, NewHandler(func(ctx context.Context, in io.Reader, out io.Writer) error {
		var m message
		buf := make([]byte, 4096)
		n, _ := in.Read(buf)
		m = classify(buf[:n])
		_, _ = io.WriteString(out, `{"jsonrpc":"2.0","id":`+m.idKey+`,"result":{"protocolVersion":1}}`+"\n")
		<-ctx.Done()
		causes <- context.Cause(ctx)
		return nil
	}, &HandlerOptions{Grace: 50 * time.Millisecond}))
	c := rawInit(t, ts.URL, nil)
	s := c.stream("")
	if status, _, _ := c.do(http.MethodDelete, "", nil); status != http.StatusAccepted {
		t.Fatalf("DELETE = %d, want 202", status)
	}
	select {
	case cause := <-causes:
		if !errors.Is(cause, ErrDeleted) {
			t.Errorf("the serve's cause = %v, want ErrDeleted", cause)
		}
	case <-time.After(waitBound):
		t.Fatal("DELETE did not end the serve")
	}
	s.ended(t, "the connection stream after DELETE")
	if status := c.post(`{"jsonrpc":"2.0","method":"x"}`, ""); status != http.StatusNotFound {
		t.Errorf("a POST after DELETE = %d, want 404", status)
	}
}

// TestHTTPServerIdle requires a connection whose connection stream has ended
// and not come back within the idle bound to end with ErrDisconnected.
func TestHTTPServerIdle(t *testing.T) {
	t.Parallel()
	causes := make(chan error, 1)
	h := NewHandler(func(ctx context.Context, in io.Reader, out io.Writer) error {
		buf := make([]byte, 4096)
		n, _ := in.Read(buf)
		_, _ = io.WriteString(out, `{"jsonrpc":"2.0","id":`+classify(buf[:n]).idKey+`,"result":{}}`+"\n")
		<-ctx.Done()
		causes <- context.Cause(ctx)
		return nil
	}, &HandlerOptions{Grace: 50 * time.Millisecond})
	h.idleBound = 100 * time.Millisecond
	ts := testServer(t, h)
	c := rawInit(t, ts.URL, nil)
	s := c.stream("")
	s.stop()
	select {
	case cause := <-causes:
		if !errors.Is(cause, ErrDisconnected) {
			t.Errorf("the serve's cause = %v, want ErrDisconnected", cause)
		}
	case <-time.After(waitBound):
		t.Fatal("a connection with no stream was not ended")
	}
}

// TestHTTPServerKeepAlive requires an idle stream to receive comments every
// KeepAlive.
func TestHTTPServerKeepAlive(t *testing.T) {
	t.Parallel()
	ts := testServer(t, NewHandler(scripted(nil), &HandlerOptions{KeepAlive: 50 * time.Millisecond}))
	c := rawInit(t, ts.URL, nil)
	if n := countComments(t, c, 400*time.Millisecond); n < 3 {
		t.Errorf("%d comments in 400 ms at a 50 ms KeepAlive, want 3 or more", n)
	}
}

// TestHTTPServerWriteTimeout requires a stream to outlive the server's
// WriteTimeout (phase 2, P5).
func TestHTTPServerWriteTimeout(t *testing.T) {
	t.Parallel()
	h := NewHandler(scripted(func(m message, _ []byte, write func(string)) bool {
		if m.method != "_test/ping" {
			return false
		}
		write(`{"jsonrpc":"2.0","method":"_test/pong"}`)
		return true
	}), nil)
	ts := httptest.NewUnstartedServer(h)
	ts.Config.WriteTimeout = 100 * time.Millisecond
	ts.Start()
	t.Cleanup(func() { _ = h.Close(context.Background()); ts.Close() })
	c := rawInit(t, ts.URL, nil)
	s := c.stream("")
	time.Sleep(300 * time.Millisecond)
	c.post(`{"jsonrpc":"2.0","method":"_test/ping"}`, "")
	if ev := s.next(t); !strings.Contains(ev, "_test/pong") {
		t.Errorf("an event 300 ms into the stream under a 100 ms WriteTimeout = %s, want the pong", ev)
	}
}

// TestHTTPServerOrigin requires POST, GET and DELETE to refuse a foreign
// origin, as the WebSocket upgrade does, unless OriginPatterns matches it.
func TestHTTPServerOrigin(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		patterns []string
		origin   string
		refused  bool
	}{
		{nil, "", false},
		{nil, "https://app.example", true},
		{[]string{"app.example"}, "https://app.example", false},
		{[]string{"https://app.example"}, "https://app.example", false},
	} {
		ts := testServer(t, NewHandler(scripted(nil), &HandlerOptions{OriginPatterns: tc.patterns}))
		c := rawInit(t, ts.URL, nil)
		h := map[string]string{"Origin": tc.origin, "Accept": eventStreamType}
		for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodDelete} {
			body := ""
			if method == http.MethodPost {
				body = `{"jsonrpc":"2.0","method":"x"}`
			}
			if method == http.MethodGet {
				// A stream would not end; its refusal is all this checks.
				if !tc.refused {
					continue
				}
			}
			status, _, _ := c.do(method, body, h)
			if (status == http.StatusForbidden) != tc.refused {
				t.Errorf("patterns %v, origin %q: %s = %d, want refused %v", tc.patterns, tc.origin, method, status, tc.refused)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), waitBound)
		hdr := http.Header{}
		if tc.origin != "" {
			hdr.Set("Origin", tc.origin)
		}
		wc, resp, _ := websocket.Dial(ctx, wsURL(ts), &websocket.DialOptions{HTTPHeader: hdr}) //nolint:bodyclose // Dial owns the body.
		cancel()
		if resp == nil || (resp.StatusCode == http.StatusForbidden) != tc.refused {
			t.Errorf("patterns %v, origin %q: the WebSocket upgrade = %v, want refused %v", tc.patterns, tc.origin, resp, tc.refused)
		}
		if wc != nil {
			_ = wc.Close(websocket.StatusNormalClosure, "")
		}
	}
}

// TestHTTPServerTranscript requires phase 4's transcript test over
// Streamable HTTP.
func TestHTTPServerTranscript(t *testing.T) {
	t.Parallel()
	ts := testServer(t, NewHandler(serveAgents(func() acp.Agent {
		return &acptest.ScriptAgent{Turns: []acptest.Turn{{
			Updates: []acp.SessionUpdate{acp.UpdateAgentMessageText("hello")}, StopReason: acp.StopReasonEndTurn,
		}}}
	}), nil))
	transcriptOf(t, ts.URL)
}

// TestHTTPServerQueued requires a POST answered 202 once its line is queued,
// before the agent reads it, so the agent's answer cannot overtake it; the
// queued lines read in order; a POST past the queue's bound waiting for the
// agent; and a POST waiting so answered 404 when the connection ends
// (0043-PLAN-remote-http-and-websocket-transport.md, "Deviation, 2026-10-30,
// phase 7's 202 after the agent's answer").
func TestHTTPServerQueued(t *testing.T) {
	t.Parallel()
	note := func(n int) string { return fmt.Sprintf(`{"jsonrpc":"2.0","method":"_test/%d"}`, n) }
	// With the limit at 2, the feed holds one line the agent has not read and
	// the queue two more: the fourth POST waits.
	fill := func(c *rawConn) <-chan postResult {
		t.Helper()
		for n := range 3 {
			if status := c.post(note(n), ""); status != http.StatusAccepted {
				t.Fatalf("POST %d while the agent reads nothing = %d, want 202", n, status)
			}
		}
		waiting := c.postAsync(note(3))
		select {
		case r := <-waiting:
			t.Fatalf("a POST past the queue's bound was answered (%v) before the agent read", r)
		case <-time.After(300 * time.Millisecond):
		}
		return waiting
	}

	release := make(chan struct{})
	got := make(chan string, 8)
	c := rawInit(t, blockedServer(t, release, got), nil)
	waiting := fill(c)
	close(release)
	select {
	case r := <-waiting:
		if r.err != nil || r.status != http.StatusAccepted {
			t.Errorf("the waiting POST, once the agent read: %v, want 202", r)
		}
	case <-time.After(waitBound):
		t.Fatalf("the waiting POST was not answered within %v of the agent's reading", waitBound)
	}
	for n := range 4 {
		select {
		case method := <-got:
			if want := fmt.Sprintf("_test/%d", n); method != want {
				t.Errorf("the agent read %s at %d, want %s", method, n, want)
			}
		case <-time.After(waitBound):
			t.Fatalf("the agent did not read line %d within %v", n, waitBound)
		}
	}

	ended := rawInit(t, blockedServer(t, make(chan struct{}), nil), nil)
	waiting = fill(ended)
	if status, _, _ := ended.do(http.MethodDelete, "", nil); status != http.StatusAccepted {
		t.Fatalf("DELETE = %d, want 202", status)
	}
	select {
	case r := <-waiting:
		if r.err != nil || r.status != http.StatusNotFound {
			t.Errorf("the waiting POST, once the connection ended: %v, want 404", r)
		}
	case <-time.After(waitBound):
		t.Fatalf("the waiting POST was not answered within %v of the connection's end", waitBound)
	}
}

// blockedServer serves connections whose agent answers initialize, then
// reads nothing until release is closed or its context ends, and then sends
// each method it reads on got.
func blockedServer(t *testing.T, release <-chan struct{}, got chan<- string) string {
	t.Helper()
	h := NewHandler(func(ctx context.Context, in io.Reader, out io.Writer) error {
		sc := bufio.NewScanner(in)
		if !sc.Scan() {
			return sc.Err()
		}
		_, _ = io.WriteString(out, `{"jsonrpc":"2.0","id":`+classify(sc.Bytes()).idKey+`,"result":{"protocolVersion":1}}`+"\n")
		select {
		case <-release:
		case <-ctx.Done():
			return nil
		}
		for sc.Scan() {
			got <- classify(sc.Bytes()).method
		}
		return sc.Err()
	}, &HandlerOptions{Grace: 50 * time.Millisecond})
	h.inboxLimit = 2
	return testServer(t, h).URL
}

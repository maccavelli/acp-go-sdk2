package remote

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// ladderGrace is the Grace the ladder tests use.
const ladderGrace = 100 * time.Millisecond

// dialRaw opens a WebSocket to ts with no client behind it.
func dialRaw(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), waitBound)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil) //nolint:bodyclose // Dial owns the body.
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

// TestHandlerLadderInputEnd requires a serve that returns at its input's end
// to be ended in the first step, with no cause given to it, when its client
// closes the socket (0043-PLAN-remote-http-and-websocket-transport.md,
// phase 4).
func TestHandlerLadderInputEnd(t *testing.T) {
	t.Parallel()
	ended := make(chan error, 1)
	ts := testServer(t, NewHandler(func(ctx context.Context, in io.Reader, _ io.Writer) error {
		_, _ = io.Copy(io.Discard, in)
		ended <- context.Cause(ctx)
		return nil
	}, &HandlerOptions{Grace: ladderGrace}))
	c := dialRaw(t, wsURL(ts))
	start := time.Now()
	_ = c.Close(websocket.StatusNormalClosure, "")
	select {
	case cause := <-ended:
		if cause != nil {
			t.Errorf("the serve's cause at its input's end = %v, want none", cause)
		}
		if d := time.Since(start); d >= ladderGrace {
			t.Errorf("the serve's input ended after %v, want within the first step, under %v", d, ladderGrace)
		}
	case <-time.After(waitBound):
		t.Fatal("the serve's input did not end")
	}
}

// TestHandlerLadderCause requires a serve that ignores its input's end and
// returns at its context's to see ErrDisconnected when its client goes.
func TestHandlerLadderCause(t *testing.T) {
	t.Parallel()
	causes := make(chan error, 1)
	ts := testServer(t, NewHandler(func(ctx context.Context, _ io.Reader, _ io.Writer) error {
		<-ctx.Done()
		causes <- context.Cause(ctx)
		return nil
	}, &HandlerOptions{Grace: ladderGrace}))
	c := dialRaw(t, wsURL(ts))
	_ = c.Close(websocket.StatusNormalClosure, "")
	select {
	case cause := <-causes:
		if !errors.Is(cause, ErrDisconnected) {
			t.Errorf("the serve's cause = %v, want ErrDisconnected", cause)
		}
	case <-time.After(waitBound):
		t.Fatal("the serve's context did not end")
	}
}

// TestHandlerLadderAbandon requires a serve that ignores its input, its
// context and its pipes to be abandoned: Close returns ErrAbandoned within
// three graces and a slack. The test then releases it, so the leak guard
// passes.
func TestHandlerLadderAbandon(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	started := make(chan struct{})
	h := NewHandler(func(context.Context, io.Reader, io.Writer) error {
		close(started)
		<-release
		return nil
	}, &HandlerOptions{Grace: ladderGrace})
	ts := testServer(t, h)
	dialRaw(t, wsURL(ts))
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), waitBound)
	defer cancel()
	start := time.Now()
	err := h.Close(ctx)
	d := time.Since(start)
	close(release)
	if !errors.Is(err, ErrAbandoned) {
		t.Errorf("Close = %v, want ErrAbandoned", err)
	}
	if limit := 3*ladderGrace + 2*time.Second; d > limit {
		t.Errorf("Close took %v, want under %v", d, limit)
	}
}

// TestHandlerClose requires Close to end every live connection with
// ErrHandlerClosed, and a handshake after it to be answered 503.
func TestHandlerClose(t *testing.T) {
	t.Parallel()
	causes := make(chan error, 2)
	started := make(chan struct{}, 2)
	h := NewHandler(func(ctx context.Context, _ io.Reader, _ io.Writer) error {
		started <- struct{}{}
		<-ctx.Done()
		causes <- context.Cause(ctx)
		return nil
	}, &HandlerOptions{Grace: ladderGrace})
	ts := testServer(t, h)
	dialRaw(t, wsURL(ts))
	dialRaw(t, wsURL(ts))
	<-started
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), waitBound)
	defer cancel()
	if err := h.Close(ctx); err != nil {
		t.Fatalf("Close = %v", err)
	}
	for range 2 {
		if cause := <-causes; !errors.Is(cause, ErrHandlerClosed) {
			t.Errorf("a serve's cause = %v, want ErrHandlerClosed", cause)
		}
	}
	_, resp, err := websocket.Dial(ctx, wsURL(ts), nil) //nolint:bodyclose // Dial owns the body.
	if err == nil || resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("a handshake after Close = %v, %v; want 503", resp, err)
	}
}

// TestHandlerServeReturnsFirst requires a serve that returns after its last
// message to have that message read, then the end, as a clean end of the
// connection.
func TestHandlerServeReturnsFirst(t *testing.T) {
	t.Parallel()
	const update = `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"bye"}}}}`
	ts := testServer(t, NewHandler(func(_ context.Context, _ io.Reader, out io.Writer) error {
		_, err := io.WriteString(out, update+"\n")
		return err
	}, nil))
	ctx, cancel := context.WithTimeout(context.Background(), waitBound)
	defer cancel()
	c, err := Dial(ctx, wsURL(ts), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(c)
	if err != nil || string(b) != update+"\n" {
		t.Errorf("read = %q, %v; want the update, then io.EOF", b, err)
	}
	_ = c.Close()

	c2, err := Dial(ctx, wsURL(ts), nil)
	if err != nil {
		t.Fatal(err)
	}
	conn := acp.NewClientSideConnection(&acptest.MemoryClient{}, c2, c2)
	select {
	case <-conn.Done():
		if err := conn.Err(); !acp.EndedCleanly(err) {
			t.Errorf("the connection's end = %v, want a clean end", err)
		}
	case <-time.After(waitBound):
		t.Fatal("the connection did not end after its serve returned")
	}
	_ = c2.Close()
}

// TestHandlerRefusals requires NewHandler to panic on a nil serve, and a
// method the endpoint does not serve to be answered 405 with Allow.
func TestHandlerRefusals(t *testing.T) {
	t.Parallel()
	func() {
		defer func() {
			if r := recover(); r == nil || !strings.Contains(r.(string), "nil serve") {
				t.Errorf("NewHandler(nil) panicked with %v, want one naming the nil serve", r)
			}
		}()
		NewHandler(nil, nil)
	}()
	ts := testServer(t, NewHandler(serveAgents(func() acp.Agent { return &acptest.ScriptAgent{} }), nil))
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, ts.URL, strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed || resp.Header.Get("Allow") != "GET, POST, DELETE" {
		t.Errorf("a PUT = %d, Allow %q; want 405 and GET, POST, DELETE", resp.StatusCode, resp.Header.Get("Allow"))
	}
}

// hostRequest sends a request of method to ts with Host host and, unless
// empty, Origin origin: a POST of initialize, a GET of the connection's
// stream, a DELETE, or a GET with "Upgrade: websocket", the handshake's
// headers set. It returns the status.
func hostRequest(t *testing.T, ts *httptest.Server, method, host, origin string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), waitBound)
	defer cancel()
	var body io.Reader
	if method == http.MethodPost {
		body = strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}`)
	}
	verb := method
	if method == "UPGRADE" {
		verb = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, verb, ts.URL, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = host
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	switch method {
	case http.MethodPost:
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
	case http.MethodGet:
		req.Header.Set("Accept", "text/event-stream")
	case "UPGRADE":
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Sec-WebSocket-Version", "13")
		req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s with Host %s: %v", method, host, err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// TestHandlerHostRule requires a handler on a loopback listener to refuse,
// 403, a request whose Host is a name other than localhost's, as a DNS
// rebinding sends with an Origin of the same name, on every method and the
// WebSocket upgrade; to serve one whose Host is an IP literal, localhost or a
// name under .localhost; and, with AllowedHosts, to serve the names listed
// and refuse the rest (0049-PLAN-subpackage-fixes.md, phase 4, P2).
func TestHandlerHostRule(t *testing.T) {
	t.Parallel()
	ts := testServer(t, NewHandler(serveAgents(func() acp.Agent { return &acptest.ScriptAgent{} }), nil))
	port := ts.URL[strings.LastIndex(ts.URL, ":"):]
	rebound := "attacker.example" + port
	for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodDelete, "UPGRADE"} {
		for _, origin := range []string{"http://" + rebound, ""} {
			if got := hostRequest(t, ts, method, rebound, origin); got != http.StatusForbidden {
				t.Errorf("%s with Host %s and Origin %q = %d, want 403", method, rebound, origin, got)
			}
		}
	}
	for _, host := range []string{"127.0.0.1" + port, "[::1]" + port, "localhost" + port, "agent.localhost" + port, "LOCALHOST." + port} {
		if got := hostRequest(t, ts, http.MethodPost, host, ""); got != http.StatusOK {
			t.Errorf("POST initialize with Host %s = %d, want 200", host, got)
		}
	}
	if got := hostRequest(t, ts, "UPGRADE", "localhost"+port, ""); got != http.StatusSwitchingProtocols {
		t.Errorf("the WebSocket upgrade with Host localhost%s = %d, want 101", port, got)
	}

	listed := testServer(t, NewHandler(serveAgents(func() acp.Agent { return &acptest.ScriptAgent{} }),
		&HandlerOptions{AllowedHosts: []string{"agent.example"}}))
	port = listed.URL[strings.LastIndex(listed.URL, ":"):]
	if got := hostRequest(t, listed, http.MethodPost, "agent.example"+port, ""); got != http.StatusOK {
		t.Errorf("POST initialize with a listed Host = %d, want 200", got)
	}
	if got := hostRequest(t, listed, http.MethodPost, "localhost"+port, ""); got != http.StatusForbidden {
		t.Errorf("POST initialize with Host localhost, AllowedHosts set without it = %d, want 403", got)
	}
}

// TestHandlerHostRuleOffLoopback requires a handler whose listener is not
// loopback, or does not say, to serve any Host with AllowedHosts nil, and
// only a listed one with it set (0049-PLAN-subpackage-fixes.md, phase 4, P2).
func TestHandlerHostRuleOffLoopback(t *testing.T) {
	t.Parallel()
	public := &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 8080}
	status := func(h *Handler, host string, local net.Addr) int {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "http://"+host+"/", nil)
		if local != nil {
			req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, local))
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	open := NewHandler(serveAgents(func() acp.Agent { return &acptest.ScriptAgent{} }), nil)
	listed := NewHandler(serveAgents(func() acp.Agent { return &acptest.ScriptAgent{} }), &HandlerOptions{AllowedHosts: []string{"agent.example"}})
	t.Cleanup(func() {
		_ = open.Close(context.Background())
		_ = listed.Close(context.Background())
	})
	for _, c := range []struct {
		name  string
		h     *Handler
		host  string
		local net.Addr
		want  bool
	}{
		{"any name, not loopback", open, "attacker.example:8080", public, true},
		{"any name, no listener address", open, "attacker.example:8080", nil, true},
		{"a listed name", listed, "agent.example:8080", public, true},
		{"a name not listed", listed, "attacker.example:8080", public, false},
	} {
		if got := status(c.h, c.host, c.local); (got != http.StatusForbidden) != c.want {
			t.Errorf("%s: DELETE with Host %s = %d; want allowed %v", c.name, c.host, got, c.want)
		}
	}
}

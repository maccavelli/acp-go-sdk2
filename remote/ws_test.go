package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
	"github.com/maccavelli/acp-go-sdk2/agentproc"
)

// TestWebSocketRoundTrip requires a client over Dial and an agent behind
// NewHandler to run initialize, session/new and a prompt with two updates
// (0043-PLAN-remote-http-and-websocket-transport.md, phase 4).
func TestWebSocketRoundTrip(t *testing.T) {
	t.Parallel()
	ts := testServer(t, NewHandler(serveAgents(func() acp.Agent {
		return &acptest.ScriptAgent{Turns: []acptest.Turn{{
			Updates:    []acp.SessionUpdate{acp.UpdateAgentMessageText("one"), acp.UpdateAgentMessageText("two")},
			StopReason: acp.StopReasonEndTurn,
		}}}
	}), nil))
	client := &acptest.MemoryClient{}
	conn, _ := dialClient(t, wsURL(ts), client, nil)
	sid := startSession(t, conn)
	resp, err := conn.Prompt(t.Context(), acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}})
	if err != nil || resp.StopReason != acp.StopReasonEndTurn {
		t.Fatalf("Prompt = %v, %v; want end_turn", resp.StopReason, err)
	}
	if n := len(client.Updates()); n != 2 {
		t.Errorf("%d updates, want 2", n)
	}
}

// TestWebSocketPermission requires an agent's session/request_permission to
// reach the client and its answer to reach the agent.
func TestWebSocketPermission(t *testing.T) {
	t.Parallel()
	ts := testServer(t, NewHandler(serveAgents(func() acp.Agent { return &permissionAgent{} }), nil))
	client := &acptest.MemoryClient{}
	client.SetPermission("call-1", acp.NewRequestPermissionOutcomeSelected("allow"))
	conn, _ := dialClient(t, wsURL(ts), client, nil)
	sid := startSession(t, conn)
	resp, err := conn.Prompt(t.Context(), acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}})
	if err != nil || resp.StopReason != acp.StopReasonEndTurn {
		t.Fatalf("Prompt = %v, %v; want end_turn after the permission was granted", resp.StopReason, err)
	}
}

// TestWebSocketCancel requires both cancellations to cross the transport:
// session/cancel and $/cancel_request ending a prompt, which is answered
// stopReason cancelled, and $/cancel_request ending another request, which is
// answered -32800 (README.md, "Cancellation").
func TestWebSocketCancel(t *testing.T) {
	t.Parallel()
	agents := make(chan *modeAgent, 1)
	ts := testServer(t, NewHandler(serveAgents(func() acp.Agent {
		a := &modeAgent{started: make(chan struct{})}
		a.Turns = []acptest.Turn{
			{Updates: []acp.SessionUpdate{acp.UpdateAgentMessageText("first")}, WaitCancel: true},
			{Updates: []acp.SessionUpdate{acp.UpdateAgentMessageText("second")}, WaitCancel: true},
		}
		agents <- a
		return a
	}), nil))
	ctx, cancel := context.WithTimeout(context.Background(), waitBound)
	defer cancel()
	c, err := Dial(ctx, wsURL(ts), nil)
	if err != nil {
		t.Fatal(err)
	}
	sent, received := &lineRecorder{}, &lineRecorder{}
	client := &acptest.MemoryClient{}
	conn := acp.NewClientSideConnection(client, io.MultiWriter(c, sent), io.TeeReader(c, received))
	t.Cleanup(func() {
		_ = c.Close()
		<-conn.Done()
	})
	sid := startSession(t, conn)
	agent := <-agents

	prompt := func(want int) <-chan acp.StopReason {
		done := make(chan acp.StopReason, 1)
		go func() {
			resp, err := conn.Prompt(context.Background(), acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock("x")}})
			if err != nil {
				t.Errorf("Prompt = %v", err)
			}
			done <- resp.StopReason
		}()
		waitFor(t, "the turn's update", func() bool { return len(client.Updates()) >= want })
		return done
	}
	stopped := func(done <-chan acp.StopReason, how string) {
		select {
		case r := <-done:
			if r != acp.StopReasonCancelled {
				t.Errorf("a prompt ended by %s stopped %q, want cancelled", how, r)
			}
		case <-time.After(waitBound):
			t.Fatalf("a prompt ended by %s did not return within %v", how, waitBound)
		}
	}

	done := prompt(1)
	if err := conn.Cancel(ctx, acp.CancelNotification{SessionId: sid}); err != nil {
		t.Fatal(err)
	}
	stopped(done, "session/cancel")

	done = prompt(2)
	var id string
	waitFor(t, "the second prompt's frame", func() bool {
		var found []string
		for _, l := range sent.all() {
			if m := classify([]byte(l)); m.method == acp.AgentMethodSessionPrompt {
				found = append(found, m.idKey)
			}
		}
		if len(found) == 2 {
			id = found[1]
			return true
		}
		return false
	})
	if err := conn.Connection().SendNotification(ctx, "$/cancel_request", map[string]any{"requestId": json.RawMessage(id)}); err != nil {
		t.Fatal(err)
	}
	stopped(done, "$/cancel_request")

	mctx, mcancel := context.WithCancel(ctx)
	modeDone := make(chan error, 1)
	go func() {
		_, err := conn.SetSessionMode(mctx, acp.SetSessionModeRequest{SessionId: sid, ModeId: "plan"})
		modeDone <- err
	}()
	<-agent.started
	mcancel()
	if err := <-modeDone; !errors.Is(err, context.Canceled) {
		t.Errorf("SetSessionMode under a cancelled context = %v, want context.Canceled", err)
	}
	waitFor(t, "the -32800 answer to session/set_mode", func() bool {
		_, ok := received.find(`"code":-32800`)
		return ok
	})
	if _, ok := sent.find(`"method":"$/cancel_request"`); !ok {
		t.Error("no $/cancel_request was sent")
	}
}

// TestWebSocketLargeMessage requires a message past the library's default
// read limit, 32768 bytes, and one at MaxMessageSize to arrive, and one byte
// past MaxMessageSize to end the connection on the side that received it.
func TestWebSocketLargeMessage(t *testing.T) {
	t.Parallel()
	const limit = 128 * 1024
	big := strings.Repeat("a", 64*1024)
	ts := testServer(t, NewHandler(serveAgents(func() acp.Agent {
		return &acptest.ScriptAgent{Turns: []acptest.Turn{{
			Updates:    []acp.SessionUpdate{acp.UpdateAgentMessageText(big)},
			StopReason: acp.StopReasonEndTurn,
		}}}
	}), &HandlerOptions{MaxMessageSize: limit}))
	client := &acptest.MemoryClient{}
	conn, _ := dialClient(t, wsURL(ts), client, &DialOptions{MaxMessageSize: limit})
	sid := startSession(t, conn)
	if _, err := conn.Prompt(t.Context(), acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock(big)}}); err != nil {
		t.Fatalf("a 64 KiB prompt and a 64 KiB update: %v", err)
	}

	// The server side: a raw client sends a frame at the limit, then one past it.
	ctx, cancel := context.WithTimeout(context.Background(), waitBound)
	defer cancel()
	lines := make(chan int, 4)
	h := NewHandler(func(_ context.Context, in io.Reader, _ io.Writer) error {
		b, err := io.ReadAll(in)
		lines <- bytes.Count(b, []byte("\n"))
		return err
	}, &HandlerOptions{MaxMessageSize: limit, Grace: 100 * time.Millisecond})
	raw := testServer(t, h)
	c, _, err := websocket.Dial(ctx, wsURL(raw), nil) //nolint:bodyclose // Dial owns the body.
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	c.SetReadLimit(-1)
	at := `"` + strings.Repeat("b", limit-2) + `"`
	if err := c.Write(ctx, websocket.MessageText, []byte(at)); err != nil {
		t.Fatal(err)
	}
	_ = c.Write(ctx, websocket.MessageText, []byte(at+" "))
	select {
	case n := <-lines:
		if n != 1 {
			t.Errorf("the serve read %d lines, want the 1 at the limit", n)
		}
	case <-time.After(waitBound):
		t.Fatal("a frame past MaxMessageSize did not end the connection")
	}

	// The client side: the agent writes a line at the client's limit, then one
	// past it.
	const clientLimit = 512
	atClient := `"` + strings.Repeat("c", clientLimit-2) + `"`
	over := testServer(t, NewHandler(func(_ context.Context, _ io.Reader, out io.Writer) error {
		_, err := io.WriteString(out, atClient+"\n"+atClient+" \n")
		return err
	}, nil))
	cc, err := Dial(ctx, wsURL(over), &DialOptions{MaxMessageSize: clientLimit})
	if err != nil {
		t.Fatal(err)
	}
	defer cc.Close()
	if b, err := io.ReadAll(cc); err == nil || string(b) != atClient+"\n" {
		t.Errorf("reading a message at the client's limit, then one past it = %q, %v; want the first and an error", b, err)
	}
}

// TestWebSocketConnectionID requires the 101 to carry Acp-Connection-Id,
// crypto/rand.Text's base32, distinct per connection.
func TestWebSocketConnectionID(t *testing.T) {
	t.Parallel()
	ts := testServer(t, NewHandler(serveAgents(func() acp.Agent { return &acptest.ScriptAgent{} }), nil))
	ctx, cancel := context.WithTimeout(context.Background(), waitBound)
	defer cancel()
	base32 := regexp.MustCompile(`^[A-Z2-7]{26,}$`)
	seen := map[string]bool{}
	for range 2 {
		c, resp, err := websocket.Dial(ctx, wsURL(ts), nil) //nolint:bodyclose // Dial owns the body.
		if err != nil {
			t.Fatal(err)
		}
		id := resp.Header.Get(connectionIDHeader)
		if !base32.MatchString(id) || seen[id] {
			t.Errorf("Acp-Connection-Id = %q, want 26 or more base32 characters, new each time", id)
		}
		seen[id] = true
		_ = c.Close(websocket.StatusNormalClosure, "")
	}
}

// TestWebSocketBinaryIgnored requires a binary frame to be ignored, logged
// once, and a text frame after it served.
func TestWebSocketBinaryIgnored(t *testing.T) {
	t.Parallel()
	var logs logBuffer
	ts := testServer(t, NewHandler(serveAgents(func() acp.Agent { return &acptest.ScriptAgent{} }), &HandlerOptions{Logger: logs.logger()}))
	ctx, cancel := context.WithTimeout(context.Background(), waitBound)
	defer cancel()
	c, _, err := websocket.Dial(ctx, wsURL(ts), nil) //nolint:bodyclose // Dial owns the body.
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	for range 2 {
		if err := c.Write(ctx, websocket.MessageBinary, []byte{1, 2, 3}); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Write(ctx, websocket.MessageText, []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}`)); err != nil {
		t.Fatal(err)
	}
	typ, b, err := c.Read(ctx)
	if err != nil || typ != websocket.MessageText || !bytes.Contains(b, []byte(`"id":1,"result"`)) {
		t.Fatalf("the answer after two binary frames = %d %s %v, want initialize's result", typ, b, err)
	}
	if n := strings.Count(logs.String(), "binary WebSocket frame ignored"); n != 1 {
		t.Errorf("binary frames logged %d times, want once", n)
	}
}

// TestWebSocketOrigin requires a handshake from a foreign origin to be
// refused, unless OriginPatterns names it.
func TestWebSocketOrigin(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		patterns []string
		want     int
	}{{nil, http.StatusForbidden}, {[]string{"app.example"}, http.StatusSwitchingProtocols}} {
		ts := testServer(t, NewHandler(serveAgents(func() acp.Agent { return &acptest.ScriptAgent{} }), &HandlerOptions{OriginPatterns: tc.patterns}))
		ctx, cancel := context.WithTimeout(context.Background(), waitBound)
		c, resp, _ := websocket.Dial(ctx, wsURL(ts), &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://app.example"}}}) //nolint:bodyclose // Dial owns the body.
		cancel()
		if resp == nil || resp.StatusCode != tc.want {
			t.Errorf("OriginPatterns %v: a handshake from https://app.example = %v, want %d", tc.patterns, resp, tc.want)
		}
		if c != nil {
			_ = c.Close(websocket.StatusNormalClosure, "")
		}
	}
}

// TestWebSocketHTTP2Client requires Dial to complete the handshake through a
// caller's transport that would speak unencrypted HTTP/2 alone, which fails
// an upgrade (phase 2, P1b).
func TestWebSocketHTTP2Client(t *testing.T) {
	t.Parallel()
	h := NewHandler(serveAgents(func() acp.Agent { return &acptest.ScriptAgent{} }), nil)
	ts := httptest.NewUnstartedServer(h)
	var sp http.Protocols
	sp.SetHTTP1(true)
	sp.SetUnencryptedHTTP2(true)
	ts.Config.Protocols = &sp
	ts.Start()
	t.Cleanup(func() {
		_ = h.Close(context.Background())
		ts.Close()
	})
	var cp http.Protocols
	cp.SetUnencryptedHTTP2(true)
	conn, _ := dialClient(t, wsURL(ts), &acptest.MemoryClient{}, &DialOptions{HTTPClient: &http.Client{Transport: &http.Transport{Protocols: &cp}}})
	startSession(t, conn)
}

// TestWebSocketCookies requires a cookie set on the handshake to be sent on
// a later Dial through the same client, and a client with no Jar to keep each
// connection's cookies to itself.
func TestWebSocketCookies(t *testing.T) {
	t.Parallel()
	h := NewHandler(serveAgents(func() acp.Agent { return &acptest.ScriptAgent{} }), nil)
	var mu sync.Mutex
	var cookies []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		ck, err := r.Cookie("affinity")
		if err == nil {
			cookies = append(cookies, ck.Value)
		} else {
			cookies = append(cookies, "")
		}
		mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "affinity", Value: "node-7", Path: "/"})
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(func() {
		_ = h.Close(context.Background())
		ts.Close()
	})
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	shared := &http.Client{Jar: jar}
	for range 2 {
		conn, c := dialClient(t, wsURL(ts), &acptest.MemoryClient{}, &DialOptions{HTTPClient: shared})
		startSession(t, conn)
		_ = c.Close()
	}
	for range 2 {
		conn, c := dialClient(t, wsURL(ts), &acptest.MemoryClient{}, nil)
		startSession(t, conn)
		_ = c.Close()
	}
	mu.Lock()
	defer mu.Unlock()
	if want := []string{"", "node-7", "", ""}; strings.Join(cookies, ",") != strings.Join(want, ",") {
		t.Errorf("cookies on the four handshakes = %q, want %q", cookies, want)
	}
}

// TestWebSocketKeepAlive requires a client that stops answering pings to be
// ended with ErrDisconnected.
func TestWebSocketKeepAlive(t *testing.T) {
	t.Parallel()
	causes := make(chan error, 1)
	ts := testServer(t, NewHandler(func(ctx context.Context, _ io.Reader, _ io.Writer) error {
		<-ctx.Done()
		causes <- context.Cause(ctx)
		return nil
	}, &HandlerOptions{KeepAlive: 50 * time.Millisecond, Grace: 50 * time.Millisecond}))
	ctx, cancel := context.WithTimeout(context.Background(), waitBound)
	defer cancel()
	// A raw client that never reads answers no ping.
	c, _, err := websocket.Dial(ctx, wsURL(ts), nil) //nolint:bodyclose // Dial owns the body.
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	select {
	case cause := <-causes:
		if !errors.Is(cause, ErrDisconnected) {
			t.Errorf("the serve's cause = %v, want ErrDisconnected", cause)
		}
	case <-time.After(waitBound):
		t.Fatal("a client answering no ping was not ended")
	}
}

// TestWebSocketTranscript requires a transcript of a remote session, a
// Transcript as both taps of the client's streams, to read back with
// ReadRecords and validate with ValidateGolden.
func TestWebSocketTranscript(t *testing.T) {
	t.Parallel()
	ts := testServer(t, NewHandler(serveAgents(func() acp.Agent {
		return &acptest.ScriptAgent{Turns: []acptest.Turn{{
			Updates: []acp.SessionUpdate{acp.UpdateAgentMessageText("hello")}, StopReason: acp.StopReasonEndTurn,
		}}}
	}), nil))
	transcriptOf(t, wsURL(ts))
}

// transcriptOf runs a session against url with a Transcript as the client's
// taps, and validates what it recorded.
func transcriptOf(t *testing.T, url string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), waitBound)
	defer cancel()
	c, err := Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	var file bytes.Buffer
	tr := agentproc.NewTranscript(&file)
	conn := acp.NewClientSideConnection(&acptest.MemoryClient{}, io.MultiWriter(c, tr.Input()), io.TeeReader(c, tr.Output()))
	sid := startSession(t, conn)
	if _, err := conn.Prompt(ctx, acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}}); err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	<-conn.Done()
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	records, err := acptest.ReadRecords(&file)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) < 7 {
		t.Errorf("%d records, want initialize, session/new and session/prompt both ways and the update", len(records))
	}
	acptest.ValidateGolden(t, records)
}

// TestDialSchemes requires Dial to refuse a scheme it does not speak, and to
// make no request for http://, whose first request is the first Write's.
func TestDialSchemes(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	if _, err := Dial(ctx, "ftp://example.com/acp", nil); err == nil || !strings.Contains(err.Error(), `"ftp"`) {
		t.Errorf("Dial(ftp://…) = %v, want an error naming the scheme", err)
	}
	c, err := Dial(ctx, "http://127.0.0.1:1/acp", nil)
	if err != nil {
		t.Errorf("Dial(http://…) to a closed port = %v, want nil: no request until a Write", err)
	} else if err := c.Close(); err != nil {
		t.Errorf("Close before any Write = %v, want nil", err)
	}
}

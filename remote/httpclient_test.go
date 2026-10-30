package remote

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// initLine is an initialize request with id 1, as one line.
func initLine(t *testing.T) []byte {
	t.Helper()
	return jsonLine(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": 1}})
}

// posts returns the POSTs among reqs whose body holds sub.
func posts(reqs []fakeRequest, sub string) []fakeRequest {
	var out []fakeRequest
	for _, r := range reqs {
		if r.method == http.MethodPost && strings.Contains(r.body, sub) {
			out = append(out, r)
		}
	}
	return out
}

// TestHTTPClientInitialize requires the first POST to carry no connection
// header, its 200 body to be the first line read, and the connection's
// stream to be opened with the id (0043-PLAN-remote-http-and-websocket-transport.md,
// phase 5).
func TestHTTPClientInitialize(t *testing.T) {
	t.Parallel()
	f := newFakeServer(t, tsMode, newLoadAgent).start(false)
	ctx, cancel := context.WithTimeout(context.Background(), waitBound)
	defer cancel()
	c, err := Dial(ctx, f.ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write(initLine(t)); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if m := classify([]byte(line)); m.kind != kindResponse || m.idKey != "1" {
		t.Errorf("the first line read = %s, want initialize's answer", line)
	}
	reqs := f.all()
	if len(reqs) < 2 {
		t.Fatalf("requests = %+v, want the POST and the stream's GET", reqs)
	}
	if r := reqs[0]; r.method != http.MethodPost || r.connID != "" || r.protoVersion != "" {
		t.Errorf("the first request = %+v, want a POST with no connection header and no protocol version", r)
	}
	if r := reqs[1]; r.method != http.MethodGet || r.connID != f.conn().id || r.session != "" || r.status != http.StatusOK {
		t.Errorf("the second request = %+v, want the connection stream's GET with id %s", r, f.conn().id)
	}
}

// TestHTTPClientNotInitializeFirst requires a first line that is not
// initialize to fail, with nothing sent.
func TestHTTPClientNotInitializeFirst(t *testing.T) {
	t.Parallel()
	f := newFakeServer(t, tsMode, newLoadAgent).start(false)
	c, err := Dial(t.Context(), f.ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	line := jsonLine(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "session/new", "params": map[string]any{}})
	if _, err := c.Write(line); !errors.Is(err, errNotInitializeFirst) {
		t.Errorf("Write(session/new) first = %v, want errNotInitializeFirst", err)
	}
	if reqs := f.all(); len(reqs) != 0 {
		t.Errorf("requests = %+v, want none", reqs)
	}
	if _, err := c.Read(make([]byte, 8)); !errors.Is(err, errNotInitializeFirst) {
		t.Errorf("Read after = %v, want errNotInitializeFirst", err)
	}
}

// TestHTTPClientInitializeRefused requires a refusal that carries the
// agent's JSON-RPC error, with no connection id, to reach Initialize as that
// error, and any other refusal as an error naming its status.
func TestHTTPClientInitializeRefused(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusOK, http.StatusInternalServerError, http.StatusGatewayTimeout} {
		f := newFakeServer(t, tsMode, newLoadAgent)
		f.initFail = status
		f.start(false)
		conn, _ := dialClient(t, f.ts.URL, &acptest.MemoryClient{}, nil)
		_, err := conn.Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
		var re *acp.RequestError
		switch status {
		case http.StatusGatewayTimeout:
			if err == nil || !strings.Contains(err.Error(), "504") {
				t.Errorf("initialize answered 504: Initialize = %v, want an error naming 504", err)
			}
		default:
			if !errors.As(err, &re) || re.Code != -32603 {
				t.Errorf("initialize answered %d with the agent's error: Initialize = %v, want that -32603 error", status, err)
			}
		}
		select {
		case <-conn.Done():
		case <-time.After(waitBound):
			t.Errorf("initialize answered %d: the connection did not end", status)
		}
	}
}

// TestHTTPClientHeaders requires each kind of message to carry the headers of
// step 3: the connection's id, the protocol version and the caller's header
// after initialize; the session on a session request, notification or
// response to the agent's session request; none on $/cancel_request, whatever
// its params name.
func TestHTTPClientHeaders(t *testing.T) {
	t.Parallel()
	f := newFakeServer(t, tsMode, func() acp.Agent { return &permissionAgent{} }).start(false)
	client := &acptest.MemoryClient{}
	client.SetPermission("call-1", acp.NewRequestPermissionOutcomeSelected("allow"))
	conn, _ := dialClient(t, f.ts.URL, client, &DialOptions{Header: http.Header{"X-Test": {"yes"}}})
	sid := startSession(t, conn)
	ctx := t.Context()
	if _, err := conn.Prompt(ctx, acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}}); err != nil {
		t.Fatal(err)
	}
	if err := conn.Cancel(ctx, acp.CancelNotification{SessionId: sid}); err != nil {
		t.Fatal(err)
	}
	if err := conn.Connection().SendNotification(ctx, "$/cancel_request", map[string]any{"requestId": 99, "sessionId": string(sid)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the $/cancel_request POST", func() bool { return len(posts(f.all(), `"$/cancel_request"`)) == 1 })
	reqs := f.all()
	if r := reqs[0]; r.connID != "" || r.protoVersion != "" || r.custom != "yes" {
		t.Errorf("initialize's POST = %+v, want no id, no protocol version, the caller's header", r)
	}
	for _, r := range reqs[1:] {
		if r.connID == "" || r.protoVersion != "1" || r.custom != "yes" {
			t.Errorf("a later request = %+v, want the id, Acp-Protocol-Version 1 and the caller's header", r)
		}
	}
	for _, tc := range []struct{ sub, session string }{
		{`"session/new"`, ""},
		{`"session/prompt"`, string(sid)},
		{`"outcome"`, string(sid)},
		{`"session/cancel"`, string(sid)},
		{`"$/cancel_request"`, ""},
	} {
		got := posts(reqs, tc.sub)
		if len(got) != 1 || got[0].session != tc.session || got[0].status != http.StatusAccepted {
			t.Errorf("the POST holding %s = %+v, want one, Acp-Session-Id %q, 202", tc.sub, got, tc.session)
		}
	}
}

// TestHTTPClientSessionStream requires session/new's answer, on the
// connection stream, to open the session's stream before it is read, and the
// session's updates to arrive on it.
func TestHTTPClientSessionStream(t *testing.T) {
	t.Parallel()
	f := newFakeServer(t, tsMode, newLoadAgent)
	f.getDelay = 200 * time.Millisecond
	f.start(false)
	client := &acptest.MemoryClient{}
	conn, _ := dialClient(t, f.ts.URL, client, nil)
	sid := startSession(t, conn)
	if !f.conn().attached(string(sid)) {
		t.Error("NewSession returned before the session's stream was open")
	}
	if _, err := conn.Prompt(t.Context(), acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}}); err != nil {
		t.Fatal(err)
	}
	if n := len(client.Updates()); n != 1 {
		t.Errorf("%d updates, want the turn's 1, from the session's stream", n)
	}
}

// loadOnFresh makes a session on one connection to f, closes it, and loads
// the session on a second, whose client it returns.
func loadOnFresh(t *testing.T, f *fakeServer) (*acp.ClientSideConnection, *acptest.MemoryClient, acp.SessionId) {
	t.Helper()
	return attachOnFresh(t, f, acp.AgentMethodSessionLoad)
}

// attachOnFresh makes a session on one connection to f, closes it, and
// loads or resumes the session, as method says, on a second, whose client it
// returns.
func attachOnFresh(t *testing.T, f *fakeServer, method string) (*acp.ClientSideConnection, *acptest.MemoryClient, acp.SessionId) {
	t.Helper()
	first, c1 := dialClient(t, f.ts.URL, &acptest.MemoryClient{}, nil)
	sid := startSession(t, first)
	_ = c1.Close()
	client := &acptest.MemoryClient{}
	conn, _ := dialClient(t, f.ts.URL, client, nil)
	ctx := t.Context()
	if _, err := conn.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		t.Fatal(err)
	}
	var err error
	if method == acp.AgentMethodSessionResume {
		_, err = conn.ResumeSession(ctx, acp.ResumeSessionRequest{SessionId: sid, Cwd: acptest.Abs("/work")})
	} else {
		_, err = conn.LoadSession(ctx, acp.LoadSessionRequest{SessionId: sid, Cwd: acptest.Abs("/work")})
	}
	if err != nil {
		t.Fatalf("%s: %s = %v", f.mode, method, err)
	}
	return conn, client, sid
}

// TestHTTPClientLoadStreamFirst requires session/load and session/resume to
// open their session's stream before their POST, and a 404 to that GET to be
// retried after the answer.
func TestHTTPClientLoadStreamFirst(t *testing.T) {
	t.Parallel()
	for _, method := range []string{acp.AgentMethodSessionLoad, acp.AgentMethodSessionResume} {
		for _, mode := range []fakeMode{tsMode, pyMode} {
			f := newFakeServer(t, mode, newLoadAgent).start(false)
			_, _, sid := attachOnFresh(t, f, method)
			var gets []fakeRequest
			firstGet, firstPost := -1, -1
			for i, r := range f.all() {
				if r.connID != "conn-2" {
					continue
				}
				if r.method == http.MethodGet && r.session == string(sid) {
					gets = append(gets, r)
					if firstGet < 0 {
						firstGet = i
					}
				}
				if r.method == http.MethodPost && strings.Contains(r.body, `"`+method+`"`) && firstPost < 0 {
					firstPost = i
				}
			}
			if firstGet < 0 || firstPost < 0 || firstGet > firstPost {
				t.Errorf("%s %s: the session's first GET at %d, the first POST at %d; want the GET first", mode, method, firstGet, firstPost)
			}
			if mode == pyMode && (len(gets) != 2 || gets[0].status != http.StatusNotFound || gets[1].status != http.StatusOK) {
				t.Errorf("pyMode %s: the session's GETs = %+v, want 404 before it, then 200 after its answer", method, gets)
			}
		}
	}
}

// TestHTTPClientLoadEachMode requires session/load on a fresh connection,
// and a prompt after it, to complete against each official server's routing:
// the replay and the turn's update each read once.
func TestHTTPClientLoadEachMode(t *testing.T) {
	t.Parallel()
	for _, mode := range []fakeMode{tsMode, rustMode, pyMode} {
		f := newFakeServer(t, mode, newLoadAgent).start(false)
		conn, client, sid := loadOnFresh(t, f)
		resp, err := conn.Prompt(t.Context(), acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}})
		if err != nil || resp.StopReason != acp.StopReasonEndTurn {
			t.Errorf("%s: Prompt after load = %v, %v; want end_turn", mode, resp.StopReason, err)
			continue
		}
		var texts []string
		for _, u := range client.Updates() {
			if u.Update.AgentMessageChunk != nil && u.Update.AgentMessageChunk.Content.Text != nil {
				texts = append(texts, u.Update.AgentMessageChunk.Content.Text.Text)
			}
		}
		if strings.Join(texts, ",") != "replayed,after load" {
			t.Errorf("%s: updates = %q, want the replay, then the turn's", mode, texts)
		}
	}
}

// TestHTTPClientHeaderRetry requires a POST answered 404 while it named a
// session to be sent once more without it, a second 404 to end the
// connection, and no retry where none is needed.
func TestHTTPClientHeaderRetry(t *testing.T) {
	t.Parallel()
	py := newFakeServer(t, pyMode, newLoadAgent).start(false)
	conn, _, sid := loadOnFresh(t, py)
	loads := posts(py.all(), `"session/load"`)
	if len(loads) != 2 || loads[0].session != string(sid) || loads[0].status != http.StatusNotFound || loads[1].session != "" || loads[1].status != http.StatusAccepted {
		t.Errorf("pyMode: session/load's POSTs = %+v, want 404 with the header, then 202 without", loads)
	}
	py.forget(py.lookup("conn-2"))
	if _, err := conn.Prompt(t.Context(), acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}}); err == nil {
		t.Error("Prompt on a connection the server lost = nil, want an error")
	}
	select {
	case <-conn.Done():
	case <-time.After(waitBound):
		t.Error("a POST answered 404 twice did not end the connection")
	}

	ts := newFakeServer(t, tsMode, newLoadAgent).start(false)
	loadOnFresh(t, ts)
	if loads := posts(ts.all(), `"session/load"`); len(loads) != 1 || loads[0].status != http.StatusAccepted {
		t.Errorf("tsMode: session/load's POSTs = %+v, want one, 202", loads)
	}
}

// TestHTTPClientOrder requires messages written one after another to reach
// the server in that order, though the server answers each after a jitter.
func TestHTTPClientOrder(t *testing.T) {
	t.Parallel()
	f := newFakeServer(t, tsMode, newLoadAgent).start(false)
	conn, _ := dialClient(t, f.ts.URL, &acptest.MemoryClient{}, nil)
	startSession(t, conn)
	const n = 200
	for i := range n {
		if err := conn.Connection().SendNotification(t.Context(), "_test/seq", map[string]any{"n": i}); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, "every notification's POST", func() bool { return len(posts(f.all(), `"_test/seq"`)) == n })
	for i, r := range posts(f.all(), `"_test/seq"`) {
		if !strings.Contains(r.body, `"n":`+strconv.Itoa(i)+`}`) {
			t.Fatalf("notification %d to arrive = %s, want n %d", i, r.body, i)
		}
	}
}

// TestHTTPClientPostRefused requires a POST answered other than 202 to end
// the connection with an error naming the status, after the lines already
// received are read.
func TestHTTPClientPostRefused(t *testing.T) {
	t.Parallel()
	f := newFakeServer(t, tsMode, newLoadAgent).start(false)
	c, err := Dial(t.Context(), f.ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	r := bufio.NewReader(c)
	if _, err := c.Write(initLine(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(jsonLine(t, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "session/new", "params": map[string]any{"cwd": acptest.Abs("/w"), "mcpServers": []any{}}})); err != nil {
		t.Fatal(err)
	}
	hc := c.t.(*httpClient)
	waitFor(t, "session/new's answer queued", func() bool {
		hc.in.mu.Lock()
		defer hc.in.mu.Unlock()
		return len(hc.in.lines) > 0
	})
	f.mu.Lock()
	f.failNext = http.StatusBadRequest
	f.mu.Unlock()
	if _, err := c.Write(jsonLine(t, map[string]any{"jsonrpc": "2.0", "method": "session/cancel", "params": map[string]any{"sessionId": "session-1"}})); err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("Write answered 400 = %v, want an error naming 400", err)
	}
	line, err := r.ReadString('\n')
	if err != nil || classify([]byte(line)).idKey != "2" {
		t.Errorf("the line queued before the refusal = %q, %v; want session/new's answer", line, err)
	}
	if _, err := r.ReadString('\n'); err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("Read after the refusal = %v, want an error naming 400", err)
	}
}

// TestHTTPClientStreamEnds requires a session stream's end to leave the
// connection open, and the connection stream's end to end it: cleanly, when
// the stream ended cleanly, as a server ends it after its agent's serve has
// returned, and as a child's output ends.
func TestHTTPClientStreamEnds(t *testing.T) {
	t.Parallel()
	f := newFakeServer(t, tsMode, newLoadAgent).start(false)
	conn, c := dialClient(t, f.ts.URL, &acptest.MemoryClient{}, nil)
	sid := startSession(t, conn)
	hc := c.t.(*httpClient)
	f.conn().closeStream(string(sid))
	waitFor(t, "the session's stream ended", func() bool {
		hc.smu.Lock()
		defer hc.smu.Unlock()
		return !hc.streams[string(sid)]
	})
	if _, err := conn.NewSession(t.Context(), acp.NewSessionRequest{Cwd: acptest.Abs("/work")}); err != nil {
		t.Errorf("NewSession after a session stream's end = %v, want the connection still open", err)
	}
	f.conn().closeStream("")
	select {
	case <-conn.Done():
		if err := conn.Err(); !acp.EndedCleanly(err) {
			t.Errorf("the connection's end = %v, want a clean end after the stream's clean end", err)
		}
	case <-time.After(waitBound):
		t.Error("the connection stream's end did not end the connection")
	}
}

// TestHTTPClientClose requires Close to send DELETE with the connection's id,
// and Read to return io.EOF after it.
func TestHTTPClientClose(t *testing.T) {
	t.Parallel()
	f := newFakeServer(t, tsMode, newLoadAgent).start(false)
	c, err := Dial(t.Context(), f.ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(initLine(t)); err != nil {
		t.Fatal(err)
	}
	id := f.conn().id
	if err := c.Close(); err != nil {
		t.Fatalf("Close = %v", err)
	}
	var deleted bool
	for _, r := range f.all() {
		deleted = deleted || (r.method == http.MethodDelete && r.connID == id && r.status == http.StatusAccepted)
	}
	if !deleted {
		t.Errorf("requests = %+v, want a DELETE with %s", f.all(), id)
	}
	if b, err := io.ReadAll(c); err != nil {
		t.Errorf("ReadAll after Close = %q, %v; want io.EOF", b, err)
	}
}

// TestHTTPClientCookies requires a cookie set on initialize's answer to be
// sent on every later request.
func TestHTTPClientCookies(t *testing.T) {
	t.Parallel()
	f := newFakeServer(t, tsMode, newLoadAgent)
	f.setCookie = true
	f.start(false)
	conn, _ := dialClient(t, f.ts.URL, &acptest.MemoryClient{}, nil)
	sid := startSession(t, conn)
	if _, err := conn.Prompt(t.Context(), acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}}); err != nil {
		t.Fatal(err)
	}
	reqs := f.all()
	for _, r := range reqs[1:] {
		if r.cookie != "node-7" {
			t.Errorf("a request after initialize = %+v, want the cookie", r)
		}
	}
}

// TestSSEReader requires the event-stream format's line endings, multi-line
// data, ignored fields and comments, and a bound on an event.
func TestSSEReader(t *testing.T) {
	t.Parallel()
	in := ": hello\r\n" + "event: message\nid: 7\nretry: 10\ndata: one\n\n" + "data: two\rdata:three\r\r" + "data: four\r\n\r\n" + "data: unended"
	r := newSSEReader(strings.NewReader(in), 64)
	var got []string
	for {
		d, err := r.next()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				t.Errorf("the stream's end = %v, want io.EOF", err)
			}
			break
		}
		got = append(got, string(d))
	}
	if want := "one|two\nthree|four"; strings.Join(got, "|") != want {
		t.Errorf("events = %q, want %q", got, want)
	}
	big := newSSEReader(strings.NewReader("data: "+strings.Repeat("x", 10)+"\ndata: "+strings.Repeat("y", 10)+"\n\n"), 15)
	if _, err := big.next(); !errors.Is(err, errLineTooLong) {
		t.Errorf("an event of 21 bytes over a 15-byte bound = %v, want errLineTooLong", err)
	}
}

// TestHTTPClientProtocols requires the caller's client to choose the HTTP
// version: HTTP/1.1 on http://, HTTP/2 by ALPN over TLS, and unencrypted
// HTTP/2 when the caller's transport asks.
func TestHTTPClientProtocols(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		setup func(f *fakeServer) *DialOptions
		want  string
	}{
		{"http", func(f *fakeServer) *DialOptions { f.start(false); return nil }, "HTTP/1.1"},
		{"tls", func(f *fakeServer) *DialOptions { f.start(true); return &DialOptions{HTTPClient: f.ts.Client()} }, "HTTP/2.0"},
		{"h2c", func(f *fakeServer) *DialOptions {
			var sp, cp http.Protocols
			sp.SetHTTP1(true)
			sp.SetUnencryptedHTTP2(true)
			cp.SetUnencryptedHTTP2(true)
			f.ts.Config.Protocols = &sp
			f.start(false)
			return &DialOptions{HTTPClient: &http.Client{Transport: &http.Transport{Protocols: &cp}}}
		}, "HTTP/2.0"},
	} {
		f := newFakeServer(t, tsMode, newLoadAgent)
		opts := tc.setup(f)
		conn, _ := dialClient(t, f.ts.URL, &acptest.MemoryClient{}, opts)
		startSession(t, conn)
		for _, r := range f.all() {
			if r.proto != tc.want {
				t.Errorf("%s: a request over %s, want %s", tc.name, r.proto, tc.want)
			}
		}
	}
}

package remote

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// scripted returns a serve that hands each message it reads to reply, which
// answers through write, one line each. initialize and session/new are
// answered when reply returns false for them. It returns at its input's end.
func scripted(reply func(m message, line []byte, write func(string)) bool) serveFunc {
	return func(_ context.Context, in io.Reader, out io.Writer) error {
		var mu sync.Mutex
		write := func(s string) {
			mu.Lock()
			defer mu.Unlock()
			_, _ = io.WriteString(out, s+"\n")
		}
		sessions := 0
		sc := bufio.NewScanner(in)
		sc.Buffer(make([]byte, 0, 64*1024), defaultMaxMessageSize+1)
		for sc.Scan() {
			line := append([]byte(nil), sc.Bytes()...)
			m := classify(line)
			if reply != nil && reply(m, line, write) {
				continue
			}
			switch m.method {
			case "initialize":
				write(`{"jsonrpc":"2.0","id":` + m.idKey + `,"result":{"protocolVersion":1}}`)
			case "session/new":
				sessions++
				write(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"sessionId":"s%d"}}`, m.idKey, sessions))
			}
		}
		return sc.Err()
	}
}

// rawConn is a Streamable HTTP connection driven by hand, for what the
// client never sends and for exact routing checks.
type rawConn struct {
	t      *testing.T
	url    string
	client *http.Client
	id     string
	next   int
}

// rawInit POSTs initialize to url and requires a 200 with a connection id.
func rawInit(t *testing.T, url string, client *http.Client) *rawConn {
	t.Helper()
	if client == nil {
		client = http.DefaultClient
	}
	c := &rawConn{t: t, url: url, client: client, next: 100}
	status, header, body := c.do(http.MethodPost, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}`, nil)
	if status != http.StatusOK || header.Get(connectionIDHeader) == "" {
		t.Fatalf("initialize = %d %s, id %q; want 200 and an id", status, body, header.Get(connectionIDHeader))
	}
	c.id = header.Get(connectionIDHeader)
	return c
}

// do sends one request with the connection's id, when it has one, and the
// given headers.
func (c *rawConn) do(method, body string, header map[string]string) (int, http.Header, string) {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), waitBound)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, c.url, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", jsonType)
	}
	if c.id != "" {
		req.Header.Set(connectionIDHeader, c.id)
	}
	for k, v := range header {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(b)
}

// post POSTs body, with session as Acp-Session-Id when not "", and returns
// the status.
func (c *rawConn) post(body, session string) int {
	c.t.Helper()
	h := map[string]string{}
	if session != "" {
		h[sessionIDHeader] = session
	}
	status, _, _ := c.do(http.MethodPost, body, h)
	return status
}

// postResult is what a POST sent by postAsync got.
type postResult struct {
	status int
	err    error
}

func (r postResult) String() string { return fmt.Sprintf("status %d, error %v", r.status, r.err) }

// postAsync POSTs body on a goroutine of its own, under waitBound, and sends
// what it got on the channel returned: rawConn.do fails the test, which a
// goroutine must not.
func (c *rawConn) postAsync(body string) <-chan postResult {
	got := make(chan postResult, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), waitBound)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, strings.NewReader(body))
		if err != nil {
			got <- postResult{err: err}
			return
		}
		req.Header.Set("Content-Type", jsonType)
		req.Header.Set(connectionIDHeader, c.id)
		resp, err := c.client.Do(req)
		if err != nil {
			got <- postResult{err: err}
			return
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		got <- postResult{status: resp.StatusCode}
	}()
	return got
}

// request POSTs a request of method with params, a fresh id, and session as
// the header when not "", and returns the id's key and the status.
func (c *rawConn) request(method, params, session string) (string, int) {
	c.t.Helper()
	c.next++
	id := strconv.Itoa(c.next)
	return id, c.post(`{"jsonrpc":"2.0","id":`+id+`,"method":"`+method+`","params":`+params+`}`, session)
}

// rawStream is one GET stream: each event's data on events, closed at the
// stream's end.
type rawStream struct {
	status int
	events chan string
	stop   context.CancelFunc
}

// stream opens session's stream, or the connection's for "".
func (c *rawConn) stream(session string) *rawStream {
	c.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Accept", eventStreamType)
	req.Header.Set(connectionIDHeader, c.id)
	if session != "" {
		req.Header.Set(sessionIDHeader, session)
	}
	resp, err := c.client.Do(req) //nolint:bodyclose // The reading goroutine below closes it.
	if err != nil {
		cancel()
		c.t.Fatal(err)
	}
	s := &rawStream{status: resp.StatusCode, events: make(chan string, 1024), stop: cancel}
	done := make(chan struct{})
	c.t.Cleanup(func() {
		cancel()
		<-done
	})
	go func() {
		defer close(done)
		defer close(s.events)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return
		}
		r := newSSEReader(resp.Body, defaultMaxMessageSize)
		for {
			d, err := r.next()
			if err != nil {
				return
			}
			s.events <- string(d)
		}
	}()
	return s
}

// next returns the stream's next event, or fails the test after waitBound.
func (s *rawStream) next(t *testing.T) string {
	t.Helper()
	select {
	case e, ok := <-s.events:
		if !ok {
			t.Fatal("the stream ended before its next event")
		}
		return e
	case <-time.After(waitBound):
		t.Fatalf("no event within %v", waitBound)
	}
	return ""
}

// ended requires the stream to end within waitBound, with no more events.
func (s *rawStream) ended(t *testing.T, what string) {
	t.Helper()
	select {
	case e, ok := <-s.events:
		if ok {
			t.Errorf("%s: an event %s, want the stream's end", what, e)
		}
	case <-time.After(waitBound):
		t.Errorf("%s: the stream did not end within %v", what, waitBound)
	}
}

// sessionOfAnswer returns the sessionId of a session/new answer.
func sessionOfAnswer(t *testing.T, ev string) string {
	t.Helper()
	m := classify([]byte(ev))
	if m.kind != kindResponse || m.resultSessionID == "" {
		t.Fatalf("event %s, want an answer naming a session", ev)
	}
	return m.resultSessionID
}

// countComments reads body for d and counts the keep-alive comments in it.
func countComments(t *testing.T, c *rawConn, d time.Duration) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", eventStreamType)
	req.Header.Set(connectionIDHeader, c.id)
	resp, err := c.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, resp.Body)
	return strings.Count(buf.String(), keepAliveComment)
}

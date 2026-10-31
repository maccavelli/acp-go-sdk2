package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

// The sending chain's tests (0046-PLAN-sending-and-receiving-middleware.md,
// phase 3). Each names the MADR's Confirmation item it shows.

// testTraceparent is the W3C trace context a tracing middleware adds.
const testTraceparent = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

// tracing is a sending middleware that reports each method on seen and adds
// _meta.traceparent to the params it passes on.
func tracing(seen chan<- string) Middleware {
	return func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
			seen <- method
			p := map[string]any{}
			if len(params) > 0 {
				if err := json.Unmarshal(params, &p); err != nil {
					return nil, NewInternalError(map[string]any{"error": err.Error()})
				}
			}
			p["_meta"] = TraceMeta(testTraceparent, "", "")
			b, err := json.Marshal(p)
			if err != nil {
				return nil, NewInternalError(map[string]any{"error": err.Error()})
			}
			return next(ctx, method, b)
		}
	}
}

// sentLine is a line the peer read: its id, method and params.
type sentLine struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params struct {
		Meta map[string]any `json:"_meta"`
	} `json:"params"`
}

// sendPath is one way the application sends: build starts the connection
// with opt and returns the call, method is what it sends, and reply the
// result the peer answers, empty for a notification.
type sendPath struct {
	name, method, reply string
	build               func(t *testing.T, w io.Writer, r io.Reader, opt ConnectionOption) func(context.Context) error
}

// sendPaths lists every sending path D4 names.
func sendPaths() []sendPath {
	raw := func(call func(c *Connection, ctx context.Context) error) func(*testing.T, io.Writer, io.Reader, ConnectionOption) func(context.Context) error {
		return func(_ *testing.T, w io.Writer, r io.Reader, opt ConnectionOption) func(context.Context) error {
			c := NewConnection(nil, w, r, opt)
			return func(ctx context.Context) error { return call(c, ctx) }
		}
	}
	client := func(negotiated bool, call func(c *ClientSideConnection, ctx context.Context) error) func(*testing.T, io.Writer, io.Reader, ConnectionOption) func(context.Context) error {
		return func(t *testing.T, w io.Writer, r io.Reader, opt ConnectionOption) func(context.Context) error {
			t.Helper()
			c := NewClientSideConnection(&clientFuncs{}, w, r, opt)
			if negotiated {
				negotiate(t, c, allAdvertised())
			}
			return func(ctx context.Context) error { return call(c, ctx) }
		}
	}
	agent := func(call func(a *AgentSideConnection, ctx context.Context) error) func(*testing.T, io.Writer, io.Reader, ConnectionOption) func(context.Context) error {
		return func(t *testing.T, w io.Writer, r io.Reader, opt ConnectionOption) func(context.Context) error {
			t.Helper()
			a := NewAgentSideConnection(agentFuncs{}, w, r, opt)
			negotiate(t, a, allAdvertised())
			return func(ctx context.Context) error { return call(a, ctx) }
		}
	}
	params := map[string]any{"a": 1}
	return []sendPath{
		{"SendRequest", "_x/r", "{}", raw(func(c *Connection, ctx context.Context) error {
			_, err := SendRequest[map[string]any](c, ctx, "_x/r", params)
			return err
		})},
		{"SendRequestNoResult", "_x/r", "{}", raw(func(c *Connection, ctx context.Context) error {
			return c.SendRequestNoResult(ctx, "_x/r", params)
		})},
		{"SendNotification", "_x/n", "", raw(func(c *Connection, ctx context.Context) error {
			return c.SendNotification(ctx, "_x/n", params)
		})},
		{"a generated request", AgentMethodSessionNew, `{"sessionId":"s"}`, client(true, func(c *ClientSideConnection, ctx context.Context) error {
			_, err := c.NewSession(ctx, NewSessionRequest{Cwd: testAbsPath(), McpServers: []McpServer{}})
			return err
		})},
		{"a generated notification", ClientMethodSessionUpdate, "", agent(func(a *AgentSideConnection, ctx context.Context) error {
			return a.SessionUpdate(ctx, SessionNotification{SessionId: "s", Update: chunkUpdate()})
		})},
		{"Prompt", AgentMethodSessionPrompt, `{"stopReason":"end_turn"}`, client(true, func(c *ClientSideConnection, ctx context.Context) error {
			_, err := c.Prompt(ctx, PromptRequest{SessionId: "s", Prompt: []ContentBlock{TextBlock("hi")}})
			return err
		})},
		{"Initialize", AgentMethodInitialize, initializeAnswer, client(false, func(c *ClientSideConnection, ctx context.Context) error {
			_, err := c.Initialize(ctx, InitializeRequest{})
			return err
		})},
		{"the agent's CallExtension", "_x/call", "{}", agent(func(a *AgentSideConnection, ctx context.Context) error {
			_, err := a.CallExtension(ctx, "_x/call", params)
			return err
		})},
		{"the client's CallExtension", "_x/call", "{}", client(true, func(c *ClientSideConnection, ctx context.Context) error {
			_, err := c.CallExtension(ctx, "_x/call", params)
			return err
		})},
		{"the agent's NotifyExtension", "_x/note", "", agent(func(a *AgentSideConnection, ctx context.Context) error {
			return a.NotifyExtension(ctx, "_x/note", params)
		})},
		{"the client's NotifyExtension", "_x/note", "", client(true, func(c *ClientSideConnection, ctx context.Context) error {
			return c.NotifyExtension(ctx, "_x/note", params)
		})},
	}
}

// TestSendingMiddlewareAllPaths requires every sending path to pass the
// chain, which sees its method and adds _meta.traceparent, which the peer
// reads (Confirmation 4).
func TestSendingMiddlewareAllPaths(t *testing.T) {
	t.Parallel()
	for _, tc := range sendPaths() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seen := make(chan string, 4)
			var call func(context.Context) error
			p := rawWire(t, func(w io.Writer, r io.Reader) { call = tc.build(t, w, r, WithSendingMiddleware(tracing(seen))) })
			done := make(chan error, 1)
			go func() { done <- call(t.Context()) }()
			var m sentLine
			if err := json.Unmarshal([]byte(within(t, p.lines, "the message")), &m); err != nil {
				t.Fatal(err)
			}
			if got := within(t, seen, "the middleware"); got != tc.method || m.Method != tc.method {
				t.Errorf("the middleware saw %q and the peer read %q, want %q", got, m.Method, tc.method)
			}
			if got := m.Params.Meta["traceparent"]; got != testTraceparent {
				t.Errorf("the peer read _meta.traceparent %v, want the middleware's", got)
			}
			if tc.reply != "" {
				p.send(`{"jsonrpc":"2.0","id":` + string(m.ID) + `,"result":` + tc.reply + `}`)
			}
			if err := within(t, done, "the call"); err != nil {
				t.Errorf("the call = %v, want nil", err)
			}
		})
	}
}

// answerNext reads the next line p's connection wrote, a request, and
// answers it with reply, a result's or an error's member.
func answerNext(t *testing.T, p wire, reply string) {
	t.Helper()
	req := p.next("a request")
	p.send(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,` + reply + `}`)
}

// TestSendingMiddlewareOrder requires the first registered sending
// middleware to be outermost, across repeated options (Confirmation 4).
func TestSendingMiddlewareOrder(t *testing.T) {
	t.Parallel()
	var tr trail
	var c *Connection
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		c = NewConnection(nil, w, r, WithSendingMiddleware(named(&tr, "a"), named(&tr, "b")), WithSendingMiddleware(named(&tr, "c")))
	})
	done := make(chan error, 1)
	go func() { done <- c.SendRequestNoResult(t.Context(), "_x/r", nil) }()
	answerNext(t, p, `"result":{}`)
	if err := within(t, done, "the call"); err != nil {
		t.Fatal(err)
	}
	if got, want := tr.take(), "a b c c b a"; got != want {
		t.Errorf("%q, want %q", got, want)
	}
}

// TestSendingMiddlewareResult requires a middleware to see the raw result
// and the peer's error, and what it returns instead to be decoded and checked
// as the peer's result would be (Confirmation 4).
func TestSendingMiddlewareResult(t *testing.T) {
	t.Parallel()
	t.Run("raw, error and replaced", func(t *testing.T) {
		t.Parallel()
		seen := make(chan string, 1)
		mw := func(next MethodHandler) MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
				r, e := next(ctx, method, params)
				raw, _ := r.(json.RawMessage)
				code := 0
				if e != nil {
					code = e.Code
				}
				seen <- fmt.Sprintf("%s %s %d", method, string(raw), code)
				switch method {
				case "_x/replace":
					return json.RawMessage(`{"n":2}`), e
				case "_x/struct":
					return struct {
						N int `json:"n"`
					}{3}, e
				}
				return r, e
			}
		}
		var c *Connection
		p := rawWire(t, func(w io.Writer, r io.Reader) { c = NewConnection(nil, w, r, WithSendingMiddleware(mw)) })
		for _, tc := range []struct {
			method, reply, saw string
			n, code            int
		}{
			{"_x/raw", `"result":{"n":1}`, `_x/raw {"n":1} 0`, 1, 0},
			{"_x/err", `"error":{"code":-32602,"message":"Invalid params"}`, "_x/err  -32602", 0, -32602},
			{"_x/replace", `"result":{"n":1}`, `_x/replace {"n":1} 0`, 2, 0},
			{"_x/struct", `"result":{"n":1}`, `_x/struct {"n":1} 0`, 3, 0},
		} {
			type result struct {
				v   map[string]int
				err error
			}
			done := make(chan result, 1)
			go func() {
				v, err := SendRequest[map[string]int](c, t.Context(), tc.method, nil)
				done <- result{v, err}
			}()
			answerNext(t, p, tc.reply)
			got := within(t, done, tc.method)
			if s := within(t, seen, tc.method+"'s middleware"); s != tc.saw {
				t.Errorf("the middleware saw %q, want %q", s, tc.saw)
			}
			if tc.code != 0 {
				if re, ok := errors.AsType[*RequestError](got.err); !ok || re.Code != tc.code {
					t.Errorf("%s = %v, want code %d", tc.method, got.err, tc.code)
				}
				continue
			}
			if got.err != nil || got.v["n"] != tc.n {
				t.Errorf("%s = %v, %v; want n %d", tc.method, got.v, got.err, tc.n)
			}
		}
	})
	t.Run("a replaced result is checked", func(t *testing.T) {
		t.Parallel()
		mw := func(next MethodHandler) MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
				r, e := next(ctx, method, params)
				if e != nil {
					return r, e
				}
				return json.RawMessage(`{"sessions":[{"sessionId":"s","cwd":"relative"}]}`), nil
			}
		}
		var c *ClientSideConnection
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			c = NewClientSideConnection(&clientFuncs{}, w, r, WithSendingMiddleware(mw))
			negotiate(t, c, allAdvertised())
		})
		done := make(chan error, 1)
		go func() {
			_, err := c.ListSessions(t.Context(), ListSessionsRequest{})
			done <- err
		}()
		answerNext(t, p, `"result":{"sessions":[]}`)
		if err := within(t, done, "ListSessions"); !errors.As(err, new(*ValidationError)) {
			t.Errorf("ListSessions = %v, want the replaced result refused with a *ValidationError", err)
		}
	})
}

// TestSendingMiddlewareSkipsCancelRequest requires the connection's own
// $/cancel_request, sent when a caller's context ends, not to pass the chain
// (Confirmation 4).
func TestSendingMiddlewareSkipsCancelRequest(t *testing.T) {
	t.Parallel()
	seen := make(chan string, 4)
	var c *Connection
	p := rawWire(t, func(w io.Writer, r io.Reader) { c = NewConnection(nil, w, r, WithSendingMiddleware(tracing(seen))) })
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- c.SendRequestNoResult(ctx, "_x/slow", nil) }()
	p.next("the request")
	cancel()
	var m sentLine
	if err := json.Unmarshal([]byte(within(t, p.lines, "the cancel")), &m); err != nil || m.Method != ProtocolMethodCancelRequest {
		t.Fatalf("the peer read %+v (%v), want %s", m, err, ProtocolMethodCancelRequest)
	}
	if got := within(t, seen, "the request's middleware"); got != "_x/slow" {
		t.Errorf("the middleware saw %q first, want _x/slow", got)
	}
	select {
	case got := <-seen:
		t.Errorf("the middleware saw %q, the connection's own cancel", got)
	default:
	}
	within(t, done, "the call")
}

// pendingCount is how many requests c waits on.
func pendingCount(c *Connection) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending)
}

// TestSendingMiddlewareMisuse requires each rule D10 names, broken, to fail
// the call with ErrSendingMiddleware, write nothing more, and leave no
// request pending (Confirmation 5). Each runs in a synctest bubble.
func TestSendingMiddlewareMisuse(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		answers int // requests the peer answers first: a call next made legitimately
		mw      func(next MethodHandler) MethodHandler
	}{
		{"next called twice", 1, func(next MethodHandler) MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
				if _, e := next(ctx, method, params); e != nil {
					return nil, e
				}
				return next(ctx, method, params)
			}
		}},
		{"a context that does not carry the call", 0, func(next MethodHandler) MethodHandler {
			return func(_ context.Context, method string, params json.RawMessage) (any, *RequestError) {
				return next(context.Background(), method, params)
			}
		}},
		{"another method", 0, func(next MethodHandler) MethodHandler {
			return func(ctx context.Context, _ string, params json.RawMessage) (any, *RequestError) {
				return next(ctx, "_x/other", params)
			}
		}},
		{"params that are not an object or an array", 0, func(next MethodHandler) MethodHandler {
			return func(ctx context.Context, method string, _ json.RawMessage) (any, *RequestError) {
				return next(ctx, method, json.RawMessage(`"str"`))
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				var c *Connection
				p := rawWire(t, func(w io.Writer, r io.Reader) { c = NewConnection(nil, w, r, WithSendingMiddleware(tc.mw)) })
				done := make(chan error, 1)
				go func() { done <- c.SendRequestNoResult(t.Context(), "_x/r", map[string]any{}) }()
				for range tc.answers {
					answerNext(t, p, `"result":{}`)
				}
				err := within(t, done, "the call")
				if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != -32603 || !errors.Is(err, ErrSendingMiddleware) {
					t.Errorf("the call = %v, want -32603 wrapping ErrSendingMiddleware", err)
				}
				synctest.Wait()
				if n := len(p.lines); n != 0 {
					t.Errorf("the peer read %d more lines, want none", n)
				}
				if n := pendingCount(c); n != 0 {
					t.Errorf("%d requests pending, want none", n)
				}
			})
		})
	}
	t.Run("a replaced result of initialize", func(t *testing.T) {
		t.Parallel()
		c, p, _ := initializeClient(t, WithSendingMiddleware(func(next MethodHandler) MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
				r, e := next(ctx, method, params)
				if method != AgentMethodInitialize || e != nil {
					return r, e
				}
				return json.RawMessage(`{"protocolVersion":1}`), nil
			}
		}))
		_, err := initializeAgainst(t.Context(), t, c, p, "")
		if !errors.Is(err, ErrSendingMiddleware) {
			t.Errorf("Initialize = %v, want ErrSendingMiddleware", err)
		}
		if c.negotiation.Load() == nil {
			t.Error("no record of initialize: the reader records the answer before the chain sees it, and it stays")
		}
	})
}

// TestSendingMiddlewareShortCircuitInitialize requires a middleware that
// refuses initialize without calling next to fail Initialize with its error,
// record nothing, and release a request held for the attempt, which is then
// refused -32601; a later Initialize succeeds (Confirmation 7;
// 0030-MADR-record-initialize-on-arrival.md, question 2). It runs in a
// synctest bubble.
func TestSendingMiddlewareShortCircuitInitialize(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		entered, release := make(chan struct{}), make(chan struct{})
		var refuse atomic.Bool
		refuse.Store(true)
		c, p, ext := initializeClient(t, WithSendingMiddleware(func(next MethodHandler) MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
				if method == AgentMethodInitialize && refuse.Load() {
					close(entered)
					<-release
					return nil, NewAuthRequired(nil)
				}
				return next(ctx, method, params)
			}
		}))
		done := make(chan error, 1)
		go func() {
			_, err := c.Initialize(t.Context(), InitializeRequest{})
			done <- err
		}()
		within(t, entered, "the middleware")
		p.send(extRequest)
		synctest.Wait()
		if n := len(p.lines); n != 0 {
			t.Fatalf("the peer read %d lines while initialize was in flight, want none: the request is held", n)
		}
		refuse.Store(false)
		close(release)
		if re, ok := errors.AsType[*RequestError](within(t, done, "Initialize")); !ok || re.Code != -32000 {
			t.Errorf("Initialize = %v, want the middleware's -32000", re)
		}
		if a := p.next("the held request's answer"); string(a.ID) != `"a1"` || a.Error == nil || a.Error.Code != -32601 {
			t.Errorf("the held request was answered %s %+v, want -32601", a.ID, a.Error)
		}
		if c.negotiation.Load() != nil {
			t.Error("a record of an initialize the middleware refused")
		}
		if _, err := initializeAgainst(t.Context(), t, c, p, ""); err != nil {
			t.Errorf("the second Initialize = %v, want nil", err)
		}
		select {
		case m := <-ext:
			t.Errorf("the handler ran for %s, refused while held", m)
		default:
		}
	})
}

// TestSendingMiddlewareInitializeRecord requires a middleware that observes
// initialize to leave the record as the peer answered, and one that returns
// an error after next to fail Initialize with it while the record stays
// (Confirmation 7).
func TestSendingMiddlewareInitializeRecord(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		fail bool
	}{{"observed", false}, {"an error after next", true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, p, _ := initializeClient(t, WithSendingMiddleware(func(next MethodHandler) MethodHandler {
				return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
					r, e := next(ctx, method, params)
					if tc.fail && e == nil {
						return nil, NewInternalError(map[string]any{"error": "refused after the answer"})
					}
					return r, e
				}
			}))
			resp, err := initializeAgainst(t.Context(), t, c, p, "")
			if tc.fail {
				if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != -32603 || errors.Is(err, ErrSendingMiddleware) {
					t.Errorf("Initialize = %v, want the middleware's -32603", err)
				}
			} else if err != nil || resp.ProtocolVersion != ProtocolVersionNumber {
				t.Errorf("Initialize = %+v, %v; want the answer", resp, err)
			}
			if n := c.negotiation.Load(); n == nil || n.Response.ProtocolVersion != ProtocolVersionNumber {
				t.Errorf("the record %+v, want the peer's answer", n)
			}
		})
	}
}

// TestSendingMiddlewareKeepsBarrier requires a request a notification handler
// sends under its context, through a middleware that derives that context,
// to be answered while the handler runs, and the reader to read on past a
// full queue meanwhile (0019-PLAN-behaviour-fixes.md, B1;
// 0044-MADR-notification-queue-back-pressure.md, D3) (Confirmation 7). It
// runs in a synctest bubble.
func TestSendingMiddlewareKeepsBarrier(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		var c *Connection
		asked := make(chan error, 1)
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			c = NewConnection(func(ctx context.Context, method string, _ json.RawMessage) (any, *RequestError) {
				if method == "_x/n" {
					asked <- c.SendRequestNoResult(ctx, "_x/q", map[string]any{})
				}
				return nil, nil
			}, w, r, WithSendingMiddleware(passThrough), WithMaxQueuedNotifications(1))
		})
		p.send(`{"jsonrpc":"2.0","method":"_x/n"}`)
		req := p.next("the handler's request")
		// With a queue of one and _x/n running, the second waits past the
		// limit: only 0044's guard reads it, and the answer after it. Under
		// OverflowBlock the reader reads on to twice the queue, so a third
		// would close the connection.
		for range 2 {
			p.send(`{"jsonrpc":"2.0","method":"_x/m"}`)
		}
		p.send(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,"result":{}}`)
		if err := within(t, asked, "the handler's request's return"); err != nil {
			t.Errorf("the handler's request = %v, want nil", err)
		}
	})
}

// TestRequestIDFromContextSending requires a sending middleware to see the
// id the peer receives, false for a notification, and, inside a received
// request's handler, the sent request's id and false for a sent notification,
// not the received request's id (Confirmation 6).
func TestRequestIDFromContextSending(t *testing.T) {
	t.Parallel()
	ids := make(chan string, 4)
	mw := func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
			id, ok := RequestIDFromContext(ctx)
			ids <- fmt.Sprintf("%s %s %v", method, string(id), ok)
			return next(ctx, method, params)
		}
	}
	var c *Connection
	handled := make(chan error, 1)
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		c = NewConnection(func(ctx context.Context, method string, _ json.RawMessage) (any, *RequestError) {
			if method == "_x/r" {
				err := c.SendNotification(ctx, "_x/note", map[string]any{})
				if err == nil {
					err = c.SendRequestNoResult(ctx, "_x/q", map[string]any{})
				}
				handled <- err
			}
			return map[string]any{}, nil
		}, w, r, WithSendingMiddleware(mw))
	})
	expect := func(want string) {
		t.Helper()
		if got := within(t, ids, want); got != want {
			t.Errorf("%q, want %q", got, want)
		}
	}
	done := make(chan error, 1)
	go func() { done <- c.SendRequestNoResult(t.Context(), "_x/top", nil) }()
	top := p.next("the request")
	expect("_x/top " + string(top.ID) + " true")
	p.send(`{"jsonrpc":"2.0","id":` + string(top.ID) + `,"result":{}}`)
	within(t, done, "the request's return")
	go func() { done <- c.SendNotification(t.Context(), "_x/n", nil) }()
	p.next("the notification")
	expect("_x/n  false")
	within(t, done, "the notification's return")

	p.send(`{"jsonrpc":"2.0","id":"recv","method":"_x/r"}`)
	p.next("the handler's notification")
	expect("_x/note  false")
	q := p.next("the handler's request")
	expect("_x/q " + string(q.ID) + " true")
	p.send(`{"jsonrpc":"2.0","id":` + string(q.ID) + `,"result":{}}`)
	if err := within(t, handled, "the handler"); err != nil {
		t.Errorf("the handler's sends = %v, want nil", err)
	}
	p.next("the received request's answer")
}

// TestSendingMiddlewareSentFlag requires Prompt to send session/cancel after
// its context ends when the middleware wrote the request, and not when the
// middleware refused it unwritten (Confirmation 4). Each runs in a synctest
// bubble.
func TestSendingMiddlewareSentFlag(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		refuse bool
	}{{"written", false}, {"refused unwritten", true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				var c *ClientSideConnection
				p := rawWire(t, func(w io.Writer, r io.Reader) {
					c = NewClientSideConnection(&clientFuncs{}, w, r, WithSendingMiddleware(func(next MethodHandler) MethodHandler {
						return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
							if tc.refuse && method == AgentMethodSessionPrompt {
								cancel()
								return nil, NewRequestCancelled(nil)
							}
							return next(ctx, method, params)
						}
					}))
					negotiate(t, c, allAdvertised())
				})
				done := make(chan error, 1)
				go func() {
					_, err := c.Prompt(ctx, PromptRequest{SessionId: "s", Prompt: []ContentBlock{TextBlock("hi")}})
					done <- err
				}()
				if !tc.refuse {
					p.next("the prompt")
					cancel()
				}
				if err := within(t, done, "Prompt"); err == nil {
					t.Fatal("Prompt = nil, want its context's end")
				}
				synctest.Wait()
				var methods []string
				for len(p.lines) > 0 {
					var m sentLine
					if err := json.Unmarshal([]byte(<-p.lines), &m); err != nil {
						t.Fatal(err)
					}
					methods = append(methods, m.Method)
				}
				if got := slices.Contains(methods, AgentMethodSessionCancel); got == tc.refuse {
					t.Errorf("the peer read %q after Prompt returned; session/cancel sent %v, want %v", methods, got, !tc.refuse)
				}
			})
		})
	}
}

package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

// The receiving chain's tests (0046-PLAN-sending-and-receiving-middleware.md,
// phase 2). Each names the MADR's Confirmation item it shows.

// trail records, in order, the names middleware and handlers add; it is safe
// for concurrent use.
type trail struct {
	mu sync.Mutex
	s  []string
}

func (tr *trail) add(s string) {
	tr.mu.Lock()
	tr.s = append(tr.s, s)
	tr.mu.Unlock()
}

// take returns what was added, space-separated, and empties the trail.
func (tr *trail) take() string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	out := strings.Join(tr.s, " ")
	tr.s = nil
	return out
}

// named is a middleware that adds name to tr before and after next.
func named(tr *trail, name string) Middleware {
	return func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
			tr.add(name)
			r, e := next(ctx, method, params)
			tr.add(name)
			return r, e
		}
	}
}

// passThrough is a middleware that calls next under a context it derives,
// as a tracing middleware does.
type passKey struct{}

func passThrough(next MethodHandler) MethodHandler {
	return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
		return next(context.WithValue(ctx, passKey{}, method), method, params)
	}
}

// notified is a middleware that signals on done once a notification's chain
// inside it has returned.
func notified(done chan<- struct{}) Middleware {
	return func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
			r, e := next(ctx, method, params)
			if IsNotification(ctx) {
				done <- struct{}{}
			}
			return r, e
		}
	}
}

// TestReceivingMiddlewareOrder requires the first registered middleware to
// be outermost, across repeated options, for a request and a notification
// (Confirmation 1).
func TestReceivingMiddlewareOrder(t *testing.T) {
	t.Parallel()
	var tr trail
	done := make(chan struct{}, 1)
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		NewConnection(func(context.Context, string, json.RawMessage) (any, *RequestError) {
			tr.add("handler")
			return map[string]any{}, nil
		}, w, r, WithReceivingMiddleware(notified(done), named(&tr, "a"), named(&tr, "b")), WithReceivingMiddleware(named(&tr, "c")))
	})
	p.send(`{"jsonrpc":"2.0","id":1,"method":"_x/r"}`)
	if a := p.next("the request's answer"); a.Error != nil {
		t.Fatalf("answer %+v, want a result", a)
	}
	const want = "a b c handler c b a"
	if got := tr.take(); got != want {
		t.Errorf("request: %q, want %q", got, want)
	}
	p.send(`{"jsonrpc":"2.0","method":"_x/n"}`)
	within(t, done, "the notification's chain")
	if got := tr.take(); got != want {
		t.Errorf("notification: %q, want %q", got, want)
	}
}

// TestReceivingMiddlewareConstruction requires a nil middleware, and one that
// returns a nil handler, to panic when the connection is built, naming its
// position across the option's calls, and each middleware to be called once,
// not once per message (Confirmation 1).
func TestReceivingMiddlewareConstruction(t *testing.T) {
	t.Parallel()
	pass := func(next MethodHandler) MethodHandler { return next }
	for _, tc := range []struct {
		name string
		opts []ConnectionOption
		want string
	}{
		{"a nil middleware", []ConnectionOption{WithReceivingMiddleware(pass, nil)}, "acp: WithReceivingMiddleware: middleware 1 is nil"},
		{"a nil handler", []ConnectionOption{WithReceivingMiddleware(pass), WithReceivingMiddleware(func(MethodHandler) MethodHandler { return nil })},
			"acp: WithReceivingMiddleware: middleware 1 returned a nil handler"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if r := recover(); r != tc.want {
					t.Errorf("panic %v, want %q", r, tc.want)
				}
			}()
			buildConnection(nil, io.Discard, strings.NewReader(""), tc.opts...)
		})
	}
	t.Run("called once", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			NewConnection(func(context.Context, string, json.RawMessage) (any, *RequestError) {
				return map[string]any{}, nil
			}, w, r, WithReceivingMiddleware(func(next MethodHandler) MethodHandler {
				calls.Add(1)
				return next
			}))
		})
		for i := range 10 {
			p.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"_x/r"}`, i))
			p.next("an answer")
		}
		if got := calls.Load(); got != 1 {
			t.Errorf("the middleware was called %d times over ten messages, want once", got)
		}
	})
}

// TestReceivingMiddlewareNilHandler requires a nil handler under a chain to
// answer a request -32601 and to drop a notification without a log, as it
// does without one (Confirmation 1). It runs in a synctest bubble, so every
// log has been written when it checks.
func TestReceivingMiddlewareNilHandler(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		var logs syncBuffer
		done := make(chan struct{}, 1)
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			NewConnection(nil, w, r, WithReceivingMiddleware(notified(done)),
				WithLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))))
		})
		p.send(`{"jsonrpc":"2.0","method":"x/n"}`)
		within(t, done, "the notification's chain")
		p.send(`{"jsonrpc":"2.0","id":1,"method":"x/r"}`)
		if a := p.next("the request's answer"); a.Error == nil || a.Error.Code != -32601 {
			t.Errorf("answer %+v, want -32601", a)
		}
		synctest.Wait()
		if strings.Contains(logs.String(), "failed to handle notification") {
			t.Errorf("the dropped notification was logged: %q", logs.String())
		}
	})
}

// sessionNewLine is a session/new request with id 1 and cwd.
func sessionNewLine(t *testing.T, cwd string) (line, params string) {
	t.Helper()
	b, err := json.Marshal(cwd)
	if err != nil {
		t.Fatal(err)
	}
	params = `{"cwd":` + string(b) + `,"mcpServers":[]}`
	return `{"jsonrpc":"2.0","id":1,"method":"session/new","params":` + params + `}`, params
}

// TestReceivingMiddlewareShortCircuit requires a middleware that returns an
// error to answer the request with it, while the rewrite hook, the
// dispatch's validation and the handler do not run (Confirmation 2).
func TestReceivingMiddlewareShortCircuit(t *testing.T) {
	t.Parallel()
	var handled, rewritten atomic.Bool
	var asc *AgentSideConnection
	refuse := func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
			if method == AgentMethodSessionNew {
				return nil, NewAuthRequired(nil)
			}
			return next(ctx, method, params)
		}
	}
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		asc = NewAgentSideConnection(agentFuncs{NewSessionFunc: func(context.Context, NewSessionRequest) (NewSessionResponse, error) {
			handled.Store(true)
			return NewSessionResponse{SessionId: "s"}, nil
		}}, w, r, WithReceivingMiddleware(refuse), WithInboundRewrite(func(context.Context, string, any) error {
			rewritten.Store(true)
			return nil
		}))
		negotiate(t, asc, allAdvertised())
	})
	line, _ := sessionNewLine(t, testAbsPath())
	p.send(line)
	if a := p.next("the refusal"); a.Error == nil || a.Error.Code != -32000 {
		t.Fatalf("answer %+v, want -32000", a)
	}
	if handled.Load() || rewritten.Load() {
		t.Errorf("handler ran %v, rewrite hook ran %v; want neither", handled.Load(), rewritten.Load())
	}
	if got := asc.conn.RefusedMessages(); got != 0 {
		t.Errorf("RefusedMessages = %d, want 0: the dispatch never ran", got)
	}
}

// seenByMiddleware is what TestReceivingMiddlewareSeesRawAndResult's
// middleware saw of one message.
type seenByMiddleware struct {
	raw    string
	result any
	err    *RequestError
}

// TestReceivingMiddlewareSeesRawAndResult requires a middleware to see the
// params as the peer sent them, before the rewrite hook; params it passes on
// to be what the hook and the handler see; and the result after the
// dispatch's check, before it is encoded (Confirmation 3).
func TestReceivingMiddlewareSeesRawAndResult(t *testing.T) {
	t.Parallel()
	other := testAbsPath("other")
	seen := make(chan seenByMiddleware, 1)
	var hookCwd, handlerCwd atomic.Value
	mw := func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
			s := seenByMiddleware{raw: string(params)}
			if method == AgentMethodSessionNew {
				var p map[string]any
				if err := json.Unmarshal(params, &p); err != nil {
					return nil, NewInternalError(nil)
				}
				p["cwd"] = other
				b, err := json.Marshal(p)
				if err != nil {
					return nil, NewInternalError(nil)
				}
				params = b
			}
			s.result, s.err = next(ctx, method, params)
			seen <- s
			return s.result, s.err
		}
	}
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		asc := NewAgentSideConnection(agentFuncs{
			NewSessionFunc: func(_ context.Context, req NewSessionRequest) (NewSessionResponse, error) {
				handlerCwd.Store(req.Cwd)
				return NewSessionResponse{SessionId: "s"}, nil
			},
			ListSessionsFunc: func(context.Context, ListSessionsRequest) (ListSessionsResponse, error) {
				return ListSessionsResponse{Sessions: []SessionInfo{{SessionId: "s", Cwd: "relative"}}}, nil
			},
		}, w, r, WithReceivingMiddleware(mw), WithInboundRewrite(func(_ context.Context, _ string, params any) error {
			if req, ok := params.(*NewSessionRequest); ok {
				hookCwd.Store(req.Cwd)
			}
			return nil
		}), WithLogger(slog.New(slog.DiscardHandler)))
		negotiate(t, asc, allAdvertised())
	})
	line, params := sessionNewLine(t, testAbsPath())
	p.send(line)
	p.next("session/new's answer")
	s := within(t, seen, "the middleware's view of session/new")
	if s.raw != params {
		t.Errorf("raw params %s, want %s as sent", s.raw, params)
	}
	if hookCwd.Load() != other || handlerCwd.Load() != other {
		t.Errorf("the hook saw cwd %v and the handler %v, want %q, the middleware's", hookCwd.Load(), handlerCwd.Load(), other)
	}
	if r, ok := s.result.(*NewSessionResponse); !ok || r.SessionId != "s" || s.err != nil {
		t.Errorf("result %T %+v, err %v; want *NewSessionResponse s", s.result, s.result, s.err)
	}
	p.send(`{"jsonrpc":"2.0","id":2,"method":"session/list","params":{}}`)
	p.next("session/list's answer")
	s = within(t, seen, "the middleware's view of session/list")
	if s.result != nil || s.err == nil || s.err.Code != -32603 {
		t.Errorf("result %v, err %v; want the checked result's -32603", s.result, s.err)
	}
}

// TestReceivingMiddlewareCountsInBudget requires a request's concurrency
// slot and bytes to be held until its middleware returns (Confirmation 7).
func TestReceivingMiddlewareCountsInBudget(t *testing.T) {
	t.Parallel()
	entered, release := make(chan struct{}), make(chan struct{})
	free := sync.OnceFunc(func() { close(release) })
	t.Cleanup(free)
	hold := func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
			if method == "_x/hold" {
				close(entered)
				<-release
			}
			return next(ctx, method, params)
		}
	}
	var c *Connection
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		c = NewConnection(func(context.Context, string, json.RawMessage) (any, *RequestError) {
			return map[string]any{}, nil
		}, w, r, WithReceivingMiddleware(hold), WithMaxConcurrentRequests(1))
	})
	line := `{"jsonrpc":"2.0","id":1,"method":"_x/hold"}`
	p.send(line)
	within(t, entered, "the held request's middleware")
	if got := c.QueuedBytes(); got != int64(len(line)) {
		t.Errorf("QueuedBytes = %d while the middleware holds the request, want %d", got, len(line))
	}
	p.send(`{"jsonrpc":"2.0","id":2,"method":"_x/r"}`)
	if a := p.next("the second request's answer"); string(a.ID) != "2" || a.Error == nil || a.Error.Code != -32603 {
		t.Errorf("answer %+v, want -32603 for id 2: the slot is held", a)
	}
	free()
	if a := p.next("the held request's answer"); string(a.ID) != "1" || a.Error != nil {
		t.Errorf("answer %+v, want a result for id 1", a)
	}
	if got := c.QueuedBytes(); got != 0 {
		t.Errorf("QueuedBytes = %d after the answer, want 0", got)
	}
}

// TestRequestIDFromContextReceiving requires RequestIDFromContext to give a
// request's id to its middleware and its handler, null included, a copy each
// time, and false for a notification (Confirmation 6).
func TestRequestIDFromContextReceiving(t *testing.T) {
	t.Parallel()
	views := make(chan string, 2)
	view := func(ctx context.Context, who string) {
		id, ok := RequestIDFromContext(ctx)
		// string(id): under Go 1.27's default build json.RawMessage is
		// jsontext.Value, whose String prints a nil value as null.
		views <- fmt.Sprintf("%s %s %v", who, string(id), ok)
		for i := range id {
			id[i] = 'x' // must not reach the handler or the answer
		}
	}
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		NewConnection(func(ctx context.Context, _ string, _ json.RawMessage) (any, *RequestError) {
			view(ctx, "handler")
			return map[string]any{}, nil
		}, w, r, WithReceivingMiddleware(func(next MethodHandler) MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
				view(ctx, "middleware")
				return next(ctx, method, params)
			}
		}))
	})
	for _, id := range []string{"7", `"a"`, "null"} {
		p.send(`{"jsonrpc":"2.0","id":` + id + `,"method":"_x/r"}`)
		for _, who := range []string{"middleware", "handler"} {
			if got, want := within(t, views, who), who+" "+id+" true"; got != want {
				t.Errorf("%q, want %q", got, want)
			}
		}
		if a := p.next("the answer"); string(a.ID) != id {
			t.Errorf("answered id %s, want %s", a.ID, id)
		}
	}
	p.send(`{"jsonrpc":"2.0","method":"_x/n"}`)
	for _, who := range []string{"middleware", "handler"} {
		if got, want := within(t, views, who), who+"  false"; got != want {
			t.Errorf("%q, want %q", got, want)
		}
	}
}

// TestReceivingMiddlewareKeepsOrdering requires the response barrier and a
// notification handler's own request (0019-PLAN-behaviour-fixes.md, B1) to
// hold with a middleware that derives the handler's context
// (Confirmation 7). Each runs in a synctest bubble.
func TestReceivingMiddlewareKeepsOrdering(t *testing.T) {
	t.Parallel()
	t.Run("the response barrier", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			var c *Connection
			p := rawWire(t, func(w io.Writer, r io.Reader) {
				c = NewConnection(func(_ context.Context, method string, _ json.RawMessage) (any, *RequestError) {
					if method == "_x/n" {
						close(started)
						<-release
					}
					return nil, nil
				}, w, r, WithReceivingMiddleware(passThrough))
			})
			returned := make(chan error, 1)
			go func() { returned <- c.SendRequestNoResult(t.Context(), "_x/r", map[string]any{}) }()
			req := p.next("the request")
			p.send(`{"jsonrpc":"2.0","method":"_x/n"}`)
			p.send(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,"result":{}}`)
			within(t, started, "the notification's handler")
			synctest.Wait()
			select {
			case err := <-returned:
				t.Fatalf("SendRequestNoResult returned %v while the notification before its answer ran", err)
			default:
			}
			close(release)
			if err := within(t, returned, "SendRequestNoResult's return"); err != nil {
				t.Errorf("SendRequestNoResult = %v, want nil", err)
			}
		})
	})
	t.Run("a notification handler's own request", func(t *testing.T) {
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
				}, w, r, WithReceivingMiddleware(passThrough))
			})
			p.send(`{"jsonrpc":"2.0","method":"_x/n"}`)
			req := p.next("the handler's request")
			p.send(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,"result":{}}`)
			if err := within(t, asked, "the handler's request's return"); err != nil {
				t.Errorf("the handler's request = %v, want nil", err)
			}
		})
	})
}

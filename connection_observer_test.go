package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

// The observer's tests (0046-PLAN-sending-and-receiving-middleware.md, phase
// 4). Each names the MADR's Confirmation item it shows.

// observedWant is what one Observation must hold: code is the answered
// *RequestError's code, 0 for none, and is an error Err must wrap.
type observedWant struct {
	kind   ObservationKind
	method string
	id     string
	size   int
	code   int
	is     error
}

// recordObservations returns an observer that sends each Observation on the
// channel it returns.
func recordObservations() (func(Observation), chan Observation) {
	ch := make(chan Observation, 16)
	return func(o Observation) { ch <- o }, ch
}

// expectObserved requires the next Observation on ch to be want, and, once
// the bubble is idle, no other to follow.
func expectObserved(t *testing.T, ch chan Observation, want observedWant) {
	t.Helper()
	o := within(t, ch, want.kind.String())
	if o.Kind != want.kind || o.Method != want.method || string(o.ID) != want.id || o.Size != want.size {
		t.Errorf("observed %v %q id %q size %d, want %v %q id %q size %d", o.Kind, o.Method, string(o.ID), o.Size,
			want.kind, want.method, want.id, want.size)
	}
	switch {
	case want.code != 0:
		if re, ok := errors.AsType[*RequestError](o.Err); !ok || re.Code != want.code {
			t.Errorf("observed Err %v, want code %d", o.Err, want.code)
		}
	case want.is != nil:
		if !errors.Is(o.Err, want.is) {
			t.Errorf("observed Err %v, want %v", o.Err, want.is)
		}
	case o.Err != nil:
		t.Errorf("observed Err %v, want nil", o.Err)
	}
	synctest.Wait()
	if n := len(ch); n != 0 {
		t.Errorf("%d more observations, want it reported once: %+v", n, <-ch)
	}
}

// holding is a handler that holds _x/hold requests and _x/n notifications
// until release is closed, signalling entered as each begins; any other
// message is answered at once.
func holding(entered chan<- struct{}, release <-chan struct{}) MethodHandler {
	return func(_ context.Context, method string, _ json.RawMessage) (any, *RequestError) {
		if method == "_x/hold" || method == "_x/n" {
			entered <- struct{}{}
			<-release
		}
		return map[string]any{}, nil
	}
}

// TestObserverReportsEachKind requires each kind of D9, at each of its sites,
// to be reported once with its method, id, size and error
// (Confirmation 8). Each runs in a synctest bubble.
func TestObserverReportsEachKind(t *testing.T) {
	t.Parallel()
	const (
		tooLong    = `{"jsonrpc":"2.0","method":"_x/abcdefghij"}`
		hold       = `{"jsonrpc":"2.0","id":1,"method":"_x/hold"}`
		notifyLine = `{"jsonrpc":"2.0","method":"_x/n"}`
		badJSON    = `{oops`
		notObject  = `[1]`
		badRequest = `{"jsonrpc":"1.0","id":5,"method":"m"}`
		cancelIn   = `{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":9}}`
		cancelBad  = `{"jsonrpc":"2.0","method":"$/cancel_request","params":[1]}`
		second     = `{"jsonrpc":"2.0","id":2,"method":"_x/r"}`
		duringStop = `{"jsonrpc":"2.0","id":4,"method":"_x/r"}`
		unknownID  = `{"jsonrpc":"2.0","id":77,"result":{}}`
		unknownErr = `{"jsonrpc":"2.0","id":78,"error":{"code":-1,"message":"x"}}`
		noID       = `{"jsonrpc":"2.0","result":{}}`
	)
	padded := `{"jsonrpc":"2.0","id":3,"method":"_x/r","params":{"pad":"` + strings.Repeat("x", 40) + `"}}`
	type step func(t *testing.T, c *Connection, p wire, entered chan struct{})
	for _, tc := range []struct {
		name string
		opts []ConnectionOption
		run  step
		want observedWant
	}{
		{"a line over the limit", []ConnectionOption{WithMaxMessageSize(10)}, func(_ *testing.T, _ *Connection, p wire, _ chan struct{}) {
			p.send(tooLong)
			p.next("the answer")
		}, observedWant{kind: ObservedFault, size: len(tooLong), code: -32600}},
		{"invalid JSON", nil, func(_ *testing.T, _ *Connection, p wire, _ chan struct{}) {
			p.send(badJSON)
			p.next("the answer")
		}, observedWant{kind: ObservedFault, size: len(badJSON), code: -32700}},
		{"not an object", nil, func(_ *testing.T, _ *Connection, p wire, _ chan struct{}) {
			p.send(notObject)
			p.next("the answer")
		}, observedWant{kind: ObservedFault, size: len(notObject), code: -32600}},
		{"an invalid request", nil, func(_ *testing.T, _ *Connection, p wire, _ chan struct{}) {
			p.send(badRequest)
			p.next("the answer")
		}, observedWant{kind: ObservedFault, id: "5", size: len(badRequest), code: -32600}},
		{"a cancel received", nil, func(_ *testing.T, _ *Connection, p wire, _ chan struct{}) {
			p.send(cancelIn)
		}, observedWant{kind: ObservedCancelReceived, method: ProtocolMethodCancelRequest, id: "9", size: len(cancelIn)}},
		{"a cancel received whose params do not decode", nil, func(_ *testing.T, _ *Connection, p wire, _ chan struct{}) {
			p.send(cancelBad)
		}, observedWant{kind: ObservedCancelReceived, method: ProtocolMethodCancelRequest, size: len(cancelBad)}},
		{"a cancel sent", nil, func(t *testing.T, c *Connection, p wire, _ chan struct{}) {
			t.Helper()
			ctx, cancel := context.WithCancel(t.Context())
			go func() { _ = c.SendRequestNoResult(ctx, "_x/slow", nil) }()
			p.next("the request")
			cancel()
			p.next("the cancel")
		}, observedWant{kind: ObservedCancelSent, method: ProtocolMethodCancelRequest, id: "1"}},
		{"a request over the concurrency limit", []ConnectionOption{WithMaxConcurrentRequests(1)}, func(t *testing.T, _ *Connection, p wire, entered chan struct{}) {
			t.Helper()
			p.send(hold)
			within(t, entered, "the held request")
			p.send(second)
			p.next("the refusal")
		}, observedWant{kind: ObservedRefused, method: "_x/r", id: "2", size: len(second), code: -32603}},
		{"a request past the byte budget", []ConnectionOption{WithMaxMessageSize(100), WithMaxQueuedBytes(100)}, func(t *testing.T, _ *Connection, p wire, entered chan struct{}) {
			t.Helper()
			if len(hold)+len(padded) <= 100 || len(padded) > 100 {
				t.Fatalf("lines of %d and %d bytes do not pass a budget of 100 together", len(hold), len(padded))
			}
			p.send(hold)
			within(t, entered, "the held request")
			p.send(padded)
			p.next("the refusal")
		}, observedWant{kind: ObservedRefused, method: "_x/r", id: "3", size: len(padded), code: -32603}},
		{"a request during Shutdown", nil, func(t *testing.T, c *Connection, p wire, entered chan struct{}) {
			t.Helper()
			p.send(hold)
			within(t, entered, "the held request")
			go func() { _ = c.Shutdown(t.Context()) }()
			synctest.Wait()
			p.send(duringStop)
			p.next("the refusal")
		}, observedWant{kind: ObservedRefused, method: "_x/r", id: "4", size: len(duringStop), code: -32603}},
		{"a response to an unknown id", nil, func(_ *testing.T, _ *Connection, p wire, _ chan struct{}) {
			p.send(unknownID)
		}, observedWant{kind: ObservedUnmatchedResponse, id: "77", size: len(unknownID)}},
		{"an error response to an unknown id", nil, func(_ *testing.T, _ *Connection, p wire, _ chan struct{}) {
			p.send(unknownErr)
		}, observedWant{kind: ObservedUnmatchedResponse, id: "78", size: len(unknownErr), code: -1}},
		{"a response without an id", nil, func(_ *testing.T, _ *Connection, p wire, _ chan struct{}) {
			p.send(noID)
		}, observedWant{kind: ObservedUnmatchedResponse, size: len(noID)}},
		{"a notification dropped", []ConnectionOption{WithNotificationOverflowPolicy(OverflowDropNewest), WithMaxQueuedNotifications(1)},
			func(t *testing.T, _ *Connection, p wire, entered chan struct{}) {
				t.Helper()
				p.send(notifyLine)
				within(t, entered, "the running notification")
				p.send(notifyLine)
				p.send(notifyLine)
			}, observedWant{kind: ObservedDropped, method: "_x/n", size: len(notifyLine)}},
		{"the overflow that closed the connection", []ConnectionOption{WithNotificationOverflowPolicy(OverflowCloseConnection), WithMaxQueuedNotifications(1)},
			func(t *testing.T, _ *Connection, p wire, entered chan struct{}) {
				t.Helper()
				p.send(notifyLine)
				within(t, entered, "the running notification")
				p.send(notifyLine)
				_ = p.trySend(notifyLine)
			}, observedWant{kind: ObservedDropped, method: "_x/n", size: len(notifyLine), is: ErrNotificationQueueOverflow}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				fn, ch := recordObservations()
				entered, release := make(chan struct{}, 4), make(chan struct{})
				t.Cleanup(sync.OnceFunc(func() { close(release) }))
				var c *Connection
				p := rawWire(t, func(w io.Writer, r io.Reader) {
					c = NewConnection(holding(entered, release), w, r, append([]ConnectionOption{WithObserver(fn), quietLogs()}, tc.opts...)...)
				})
				tc.run(t, c, p, entered)
				expectObserved(t, ch, tc.want)
			})
		})
	}
}

// written is an io.Writer that keeps what is written through it, each write
// once it has returned.
type written struct {
	w  io.Writer
	mu sync.Mutex
	b  strings.Builder
}

func (r *written) Write(p []byte) (int, error) {
	n, err := r.w.Write(p)
	r.mu.Lock()
	r.b.Write(p[:n])
	r.mu.Unlock()
	return n, err
}

// has reports whether what has been written holds each of s.
func (r *written) has(s ...string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range s {
		if !strings.Contains(r.b.String(), x) {
			return false
		}
	}
	return true
}

// acting is what TestObserverRunsAfterActing's observer inspects when it is
// called: the connection, what it has written, the context of the request
// _x/hold runs, and the drop handler's count.
type acting struct {
	c     *Connection
	out   *written
	held  atomic.Pointer[context.Context]
	drops atomic.Int64
}

// handler holds _x/hold requests, keeping their contexts, until release is
// closed or their context ends, and _x/n notifications until release is
// closed, signalling entered as each begins; it answers any other message at
// once.
func (a *acting) handler(entered chan<- struct{}, release <-chan struct{}) MethodHandler {
	return func(ctx context.Context, method string, _ json.RawMessage) (any, *RequestError) {
		switch method {
		case "_x/hold":
			a.held.Store(&ctx)
			entered <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
			}
		case "_x/n":
			entered <- struct{}{}
			<-release
		}
		return map[string]any{}, nil
	}
}

// unless is "" when ok, and what otherwise.
func unless(ok bool, what string) string {
	if ok {
		return ""
	}
	return what
}

// TestObserverRunsAfterActing requires the observer to be called after the
// connection has acted on the message: the answer to a fault or a refusal
// written, the request a $/cancel_request names cancelled, the connection's
// own $/cancel_request written, the drop handler called; and, for the
// notification whose overflow closes the connection, before it closes
// (0046-MADR-sending-and-receiving-middleware.md, "Amendment, 2026-11-01: when
// the observer runs"). Each runs in a synctest bubble, and its observer
// records what it finds when it is first called for the case's kind.
func TestObserverRunsAfterActing(t *testing.T) {
	t.Parallel()
	const (
		hold       = `{"jsonrpc":"2.0","id":1,"method":"_x/hold"}`
		notifyLine = `{"jsonrpc":"2.0","method":"_x/n"}`
	)
	padded := `{"jsonrpc":"2.0","id":3,"method":"_x/r","params":{"pad":"` + strings.Repeat("x", 40) + `"}}`
	for _, tc := range []struct {
		name  string
		kind  ObservationKind
		opts  func(a *acting) []ConnectionOption
		run   func(t *testing.T, a *acting, p wire, entered chan struct{})
		check func(a *acting) string
	}{
		{"a fault", ObservedFault, nil, func(_ *testing.T, _ *acting, p wire, _ chan struct{}) {
			p.send(`{oops`)
		}, func(a *acting) string {
			return unless(a.out.has(`"code":-32700`), "the observer ran before the fault's answer was written")
		}},
		{"a refusal over the concurrency limit", ObservedRefused, func(*acting) []ConnectionOption {
			return []ConnectionOption{WithMaxConcurrentRequests(1)}
		}, func(t *testing.T, _ *acting, p wire, entered chan struct{}) {
			t.Helper()
			p.send(hold)
			within(t, entered, "the held request")
			p.send(`{"jsonrpc":"2.0","id":2,"method":"_x/r"}`)
		}, func(a *acting) string {
			return unless(a.out.has(`"id":2,"error"`), "the observer ran before the refusal was written")
		}},
		{"a refusal past the byte budget", ObservedRefused, func(*acting) []ConnectionOption {
			return []ConnectionOption{WithMaxMessageSize(100), WithMaxQueuedBytes(100)}
		}, func(t *testing.T, _ *acting, p wire, entered chan struct{}) {
			t.Helper()
			p.send(hold)
			within(t, entered, "the held request")
			p.send(padded)
		}, func(a *acting) string {
			return unless(a.out.has(`"id":3,"error"`), "the observer ran before the refusal was written")
		}},
		{"a cancel received", ObservedCancelReceived, nil, func(t *testing.T, _ *acting, p wire, entered chan struct{}) {
			t.Helper()
			p.send(`{"jsonrpc":"2.0","id":9,"method":"_x/hold"}`)
			within(t, entered, "the held request")
			p.send(`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":9}}`)
		}, func(a *acting) string {
			ctx := a.held.Load()
			return unless(ctx != nil && (*ctx).Err() != nil, "the observer ran before the request it names was cancelled")
		}},
		{"a cancel sent", ObservedCancelSent, nil, func(t *testing.T, a *acting, p wire, _ chan struct{}) {
			t.Helper()
			ctx, cancel := context.WithCancel(t.Context())
			go func() { _ = a.c.SendRequestNoResult(ctx, "_x/slow", nil) }()
			p.next("the request")
			cancel()
		}, func(a *acting) string {
			return unless(a.out.has(`"$/cancel_request"`, `"requestId":1`), "the observer ran before the $/cancel_request was written")
		}},
		{"a notification dropped", ObservedDropped, func(a *acting) []ConnectionOption {
			return []ConnectionOption{WithNotificationOverflowPolicy(OverflowDropNewest), WithMaxQueuedNotifications(1),
				WithNotificationDropHandler(func(string, uint64) { a.drops.Add(1) })}
		}, func(t *testing.T, _ *acting, p wire, entered chan struct{}) {
			t.Helper()
			p.send(notifyLine)
			within(t, entered, "the running notification")
			p.send(notifyLine)
			p.send(notifyLine)
		}, func(a *acting) string {
			return unless(a.drops.Load() >= 1, "the observer ran before the drop handler")
		}},
		{"the overflow that closed the connection", ObservedDropped, func(*acting) []ConnectionOption {
			return []ConnectionOption{WithNotificationOverflowPolicy(OverflowCloseConnection), WithMaxQueuedNotifications(1)}
		}, func(t *testing.T, _ *acting, p wire, entered chan struct{}) {
			t.Helper()
			p.send(notifyLine)
			within(t, entered, "the running notification")
			p.send(notifyLine)
			_ = p.trySend(notifyLine)
		}, func(a *acting) string {
			select {
			case <-a.c.Done():
				return "the observer ran after the connection closed"
			default:
				return ""
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				a := &acting{}
				found := make(chan string, 1)
				observe := func(o Observation) {
					if o.Kind != tc.kind {
						return
					}
					select {
					case found <- tc.check(a):
					default:
					}
				}
				entered, release := make(chan struct{}, 4), make(chan struct{})
				t.Cleanup(sync.OnceFunc(func() { close(release) }))
				opts := []ConnectionOption{WithObserver(observe), quietLogs()}
				if tc.opts != nil {
					opts = append(opts, tc.opts(a)...)
				}
				p := rawWire(t, func(w io.Writer, r io.Reader) {
					a.out = &written{w: w}
					a.c = NewConnection(a.handler(entered, release), a.out, r, opts...)
				})
				tc.run(t, a, p, entered)
				if what := within(t, found, tc.kind.String()); what != "" {
					t.Errorf("%v: %s", tc.kind, what)
				}
			})
		})
	}
}

// quietLogs is a logger that discards, for tests that provoke warnings.
func quietLogs() ConnectionOption { return WithLogger(slog.New(slog.DiscardHandler)) }

// TestObserverNoLockHeld requires the observer never to run under a lock of
// the connection's: one that calls its accessors at every report does not
// deadlock (Confirmation 8). It runs in a synctest bubble, which fails on a
// deadlock.
func TestObserverNoLockHeld(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		var c *Connection
		kinds := make(chan ObservationKind, 8)
		entered, release := make(chan struct{}, 4), make(chan struct{})
		t.Cleanup(sync.OnceFunc(func() { close(release) }))
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			c = NewConnection(holding(entered, release), w, r, quietLogs(), WithMaxConcurrentRequests(1),
				WithNotificationOverflowPolicy(OverflowDropNewest), WithMaxQueuedNotifications(1),
				WithObserver(func(o Observation) {
					_, _, _ = c.QueuedBytes(), c.RefusedMessages(), c.DroppedNotifications()
					kinds <- o.Kind
				}))
		})
		p.send(`{"jsonrpc":"2.0","method":"_x/n"}`)
		within(t, entered, "the running notification")
		p.send(`{"jsonrpc":"2.0","method":"_x/n"}`)
		p.send(`{"jsonrpc":"2.0","method":"_x/n"}`)
		p.send(`{"jsonrpc":"2.0","id":1,"method":"_x/hold"}`)
		within(t, entered, "the held request")
		p.send(`{"jsonrpc":"2.0","id":2,"method":"_x/r"}`)
		p.next("the refusal")
		// The refusal is reported on the answering goroutine once it is
		// written, so the order of the kinds waits for it
		// (0049-PLAN-runtime-fixes.md, phase 1).
		synctest.Wait()
		p.send(`{"jsonrpc":"2.0","id":77,"result":{}}`)
		p.send(`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":1}}`)
		synctest.Wait()
		var got []ObservationKind
		for len(kinds) > 0 {
			got = append(got, <-kinds)
		}
		want := []ObservationKind{ObservedDropped, ObservedRefused, ObservedUnmatchedResponse, ObservedCancelReceived}
		if !slices.Equal(got, want) {
			t.Errorf("observed %v, want %v", got, want)
		}
	})
}

// TestObserverAndChainsDisjoint requires every message, with both chains and
// the observer set, to be seen once: through a chain, or by the observer
// (Confirmation 8). It runs in a synctest bubble.
func TestObserverAndChainsDisjoint(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		var seen []string
		add := func(s string) {
			mu.Lock()
			seen = append(seen, s)
			mu.Unlock()
		}
		tap := func(chain string) Middleware {
			return func(next MethodHandler) MethodHandler {
				return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
					add(chain + " " + method)
					return next(ctx, method, params)
				}
			}
		}
		var c *Connection
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			c = NewConnection(func(context.Context, string, json.RawMessage) (any, *RequestError) {
				return map[string]any{}, nil
			}, w, r, quietLogs(), WithReceivingMiddleware(tap("receive")), WithSendingMiddleware(tap("send")),
				WithObserver(func(o Observation) { add("observe " + o.Kind.String()) }))
		})
		done := make(chan error, 2)
		go func() { done <- c.SendRequestNoResult(t.Context(), "_x/out", nil) }()
		answerNext(t, p, `"result":{}`) // a matched response: the call's result
		within(t, done, "the request")
		if err := c.SendNotification(t.Context(), "_x/outn", nil); err != nil {
			t.Fatal(err)
		}
		p.next("the notification")
		p.send(`{"jsonrpc":"2.0","id":"in","method":"_x/in"}`)
		p.next("the answer")
		p.send(`{"jsonrpc":"2.0","method":"_x/inn"}`)
		p.send(`{oops`)
		p.next("the fault's answer")
		p.send(`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":"in"}}`)
		p.send(`{"jsonrpc":"2.0","id":999,"result":{}}`)
		ctx, cancel := context.WithCancel(t.Context())
		go func() { done <- c.SendRequestNoResult(ctx, "_x/cancelled", nil) }()
		p.next("the request to cancel")
		cancel()
		p.next("the cancel")
		within(t, done, "the cancelled request")
		synctest.Wait()
		mu.Lock()
		got := slices.Sorted(slices.Values(seen))
		mu.Unlock()
		want := slices.Sorted(slices.Values([]string{
			"send _x/out", "send _x/outn", "send _x/cancelled",
			"receive _x/in", "receive _x/inn",
			"observe fault", "observe cancel_received", "observe unmatched_response", "observe cancel_sent",
		}))
		if !slices.Equal(got, want) {
			t.Errorf("seen %q, want each message once: %q", got, want)
		}
	})
}

// TestObserverUnset requires a connection without an observer to copy
// nothing at a site: observe allocates nothing (Confirmation 8 and 9).
func TestObserverUnset(t *testing.T) {
	c := buildConnection(nil, io.Discard, strings.NewReader(""))
	id := json.RawMessage(`7`)
	e := NewInternalError(nil)
	if n := testing.AllocsPerRun(100, func() { c.observe(ObservedRefused, "m", id, 10, e) }); n != 0 {
		t.Errorf("observe without an observer allocated %v times, want 0", n)
	}
}

// TestObserverCopiesID requires the ID an observer receives to be its own: a
// change to the bytes the connection passed does not reach it (D9).
func TestObserverCopiesID(t *testing.T) {
	t.Parallel()
	var got json.RawMessage
	c := buildConnection(nil, io.Discard, strings.NewReader(""), WithObserver(func(o Observation) { got = o.ID }))
	id := json.RawMessage(`"abc"`)
	c.observe(ObservedRefused, "m", id, 0, nil)
	copy(id, `"xyz"`)
	if string(got) != `"abc"` {
		t.Errorf("the observer's ID became %s with the connection's bytes, want its own copy \"abc\"", got)
	}
}

// TestObservationKindString requires each kind's name, and a value that is
// not a kind to print as ObservationKind(n).
func TestObservationKindString(t *testing.T) {
	t.Parallel()
	for k, want := range map[ObservationKind]string{
		ObservedFault: "fault", ObservedCancelReceived: "cancel_received", ObservedCancelSent: "cancel_sent",
		ObservedRefused: "refused", ObservedUnmatchedResponse: "unmatched_response", ObservedDropped: "dropped",
		0: "ObservationKind(0)", 7: "ObservationKind(7)",
	} {
		if got := k.String(); got != want {
			t.Errorf("%d: %q, want %q", int(k), got, want)
		}
	}
}

// TestWithObserverLastWins requires the last WithObserver given to be the
// one called, and nil to remove it, as ConnectionOption's rule says.
func TestWithObserverLastWins(t *testing.T) {
	t.Parallel()
	var got []string
	first := func(Observation) { got = append(got, "first") }
	second := func(Observation) { got = append(got, "second") }
	buildConnection(nil, io.Discard, strings.NewReader(""), WithObserver(first), WithObserver(second)).observe(ObservedFault, "", nil, 0, nil)
	buildConnection(nil, io.Discard, strings.NewReader(""), WithObserver(first), WithObserver(nil)).observe(ObservedFault, "", nil, 0, nil)
	if !slices.Equal(got, []string{"second"}) {
		t.Errorf("observers called %q, want only the last given, and none after nil", got)
	}
}

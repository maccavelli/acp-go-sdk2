package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// initializeAnswer is a raw agent's answer to initialize.
const initializeAnswer = `{"protocolVersion":1,"agentCapabilities":{},"authMethods":[]}`

// Extension messages a raw agent sends: a notification, and a request whose
// answer has the id "a1".
const (
	extNotification = `{"jsonrpc":"2.0","method":"_x/hello","params":{}}`
	extRequest      = `{"jsonrpc":"2.0","id":"a1","method":"_x/ask","params":{}}`
)

// initializeClient starts a client with opts over a raw agent; ext receives
// the method of each extension message the client handles.
func initializeClient(t *testing.T, opts ...ConnectionOption) (c *ClientSideConnection, p wire, ext chan string) {
	t.Helper()
	ext = make(chan string, 8)
	p = rawWire(t, func(w io.Writer, r io.Reader) {
		c = NewClientSideConnection(&clientFuncs{HandleExtensionMethodFunc: func(_ context.Context, method string, _ json.RawMessage) (any, error) {
			ext <- method
			return map[string]any{}, nil
		}}, w, r, append([]ConnectionOption{WithLogger(slog.New(slog.DiscardHandler))}, opts...)...)
	})
	return c, p, ext
}

// initializeAgainst calls Initialize under ctx while p reads the request and
// answers it, followed by more in the same write, and returns what Initialize
// returned.
func initializeAgainst(ctx context.Context, t *testing.T, c *ClientSideConnection, p wire, more string) (InitializeResponse, error) {
	t.Helper()
	return initializeWith(ctx, t, c, p, func(id string) string {
		return fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, id, initializeAnswer) + more
	})
}

// initializeWith calls Initialize under ctx while p reads the request and
// writes what reply makes of its id, in one write, and returns what
// Initialize returned.
func initializeWith(ctx context.Context, t *testing.T, c *ClientSideConnection, p wire, reply func(id string) string) (InitializeResponse, error) {
	t.Helper()
	type result struct {
		resp InitializeResponse
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := c.Initialize(ctx, InitializeRequest{})
		done <- result{resp, err}
	}()
	req := p.next("the initialize request")
	if s := reply(string(req.ID)); s != "" {
		p.send(s)
	}
	r := within(t, done, "Initialize")
	return r.resp, r.err
}

// judgement is the gate's verdict on an extension message, as the validation
// hook sees it once the gate, and any hold, has judged it.
type judgement struct {
	method string
	err    error
}

// judged is a validation hook that sends each extension message's verdict on
// seen and leaves it as it is.
func judged(seen chan<- judgement) ConnectionOption {
	return WithInboundValidation(func(_ context.Context, method string, _ any, err error) error {
		if method == "_x/hello" || method == "_x/ask" {
			seen <- judgement{method, err}
		}
		return err
	})
}

// verdicts collects the verdicts on the extension notification and request,
// each within within's bound.
func verdicts(t *testing.T, seen <-chan judgement) map[string]error {
	t.Helper()
	got := map[string]error{}
	for range 2 {
		j := within(t, seen, "the gate's verdict on an extension message")
		got[j.method] = j.err
	}
	return got
}

// TestInitializeRecordedOnArrival requires an extension notification and an
// extension request that the agent writes in the same write as its
// initialize answer to be handled, not refused as sent before initialize
// completed: the reader records the answer before it reads them, 200 times
// (0030-MADR-record-initialize-on-arrival.md, I6 and question 4).
func TestInitializeRecordedOnArrival(t *testing.T) {
	t.Parallel()
	for i := range 200 {
		c, p, ext := initializeClient(t)
		if _, err := initializeAgainst(t.Context(), t, c, p, "\n"+extNotification+"\n"+extRequest); err != nil {
			t.Fatalf("trial %d: Initialize = %v", i, err)
		}
		if a := p.next("the extension request's answer"); string(a.ID) != `"a1"` || a.Error != nil {
			t.Fatalf("trial %d: the extension request sent with the answer was answered %s %+v, want a result", i, a.ID, a.Error)
		}
		got := []string{within(t, ext, "an extension handler"), within(t, ext, "the other extension handler")}
		slices.Sort(got)
		if want := []string{"_x/ask", "_x/hello"}; !slices.Equal(got, want) {
			t.Fatalf("trial %d: the extension handlers ran for %q, want %q", i, got, want)
		}
		if err := c.conn.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// TestAgentRecordsBeforeAnswering requires the agent to have recorded
// initialize before its answer is written: a session/new the client sends
// the moment it reads the answer is answered with a session, 200 times
// (0030-MADR-record-initialize-on-arrival.md, I1 and I8).
func TestAgentRecordsBeforeAnswering(t *testing.T) {
	t.Parallel()
	cwd, err := json.Marshal(testAbsPath())
	if err != nil {
		t.Fatal(err)
	}
	for i := range 200 {
		var a *AgentSideConnection
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			a = NewAgentSideConnection(agentFuncs{NewSessionFunc: func(context.Context, NewSessionRequest) (NewSessionResponse, error) {
				return NewSessionResponse{SessionId: "s"}, nil
			}}, w, r, WithLogger(slog.New(slog.DiscardHandler)))
		})
		p.send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"clientCapabilities":{}}}`)
		if ans := p.next("the initialize answer"); string(ans.ID) != "1" || ans.Error != nil {
			t.Fatalf("trial %d: initialize answered %s %+v", i, ans.ID, ans.Error)
		}
		p.send(`{"jsonrpc":"2.0","id":2,"method":"session/new","params":{"cwd":` + string(cwd) + `,"mcpServers":[]}}`)
		if ans := p.next("session/new's answer"); string(ans.ID) != "2" || ans.Error != nil || !strings.Contains(string(ans.Result), `"sessionId":"s"`) {
			t.Fatalf("trial %d: session/new sent the moment the answer was read was answered %s %s %+v, want the session", i, ans.ID, ans.Result, ans.Error)
		}
		if err := a.conn.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// TestInitializeHooksRunOnce requires the inbound hooks to see the
// initialize answer once, and the record and Initialize's result to be the
// answer as the rewrite hook left it
// (0030-PLAN-record-initialize-on-arrival.md, the deviation during step 2).
func TestInitializeHooksRunOnce(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	c, p, _ := initializeClient(t, WithInboundRewrite(func(_ context.Context, method string, params any) error {
		if r, ok := params.(*InitializeResponse); ok && method == AgentMethodInitialize {
			calls.Add(1)
			r.AgentInfo = &Implementation{Name: "rewritten", Version: "1"}
		}
		return nil
	}))
	resp, err := initializeAgainst(t.Context(), t, c, p, "")
	if err != nil {
		t.Fatalf("Initialize = %v", err)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("the rewrite hook saw the initialize answer %d times, want once", n)
	}
	if resp.AgentInfo == nil || resp.AgentInfo.Name != "rewritten" {
		t.Errorf("Initialize returned agentInfo %+v, want the rewrite hook's", resp.AgentInfo)
	}
	if n, ok := c.Negotiated(); !ok || n.Response.AgentInfo == nil || n.Response.AgentInfo.Name != "rewritten" {
		t.Errorf("the record is %+v, %v; want the rewrite hook's agentInfo", n.Response.AgentInfo, ok)
	}
}

// TestInitializeRefusedByHookRecordsNothing requires an initialize answer the
// validation hook refuses to fail Initialize and record nothing: an extension
// request the agent sends afterwards is refused as sent before initialize
// completed. A refused notification has no answer to show it, so the
// request shows it (0030-PLAN-record-initialize-on-arrival.md, the deviation
// during step 2).
func TestInitializeRefusedByHookRecordsNothing(t *testing.T) {
	t.Parallel()
	c, p, _ := initializeClient(t, WithInboundValidation(func(_ context.Context, _ string, params any, err error) error {
		if _, ok := params.(*InitializeResponse); ok {
			return errors.New("refused by the hook")
		}
		return err
	}))
	if _, err := initializeAgainst(t.Context(), t, c, p, ""); err == nil || !strings.Contains(err.Error(), "refused by the hook") {
		t.Fatalf("Initialize = %v, want the hook's refusal", err)
	}
	if n, ok := c.Negotiated(); ok {
		t.Errorf("a record was made of an answer the hook refused: %+v", n)
	}
	p.send(extRequest)
	if a := p.next("the extension request's answer"); a.Error == nil || a.Error.Code != -32601 ||
		!strings.Contains(fmt.Sprint(a.Error.Data), ErrNotInitialized.Error()) {
		t.Errorf("an extension request after the refused answer was answered %+v, want -32601 for %q", a.Error, ErrNotInitialized)
	}
}

// TestInitializeGiveUpAgrees requires Initialize's return and the record to
// agree when the caller's context ends, or the connection closes, while the
// reader is committing the answer it has claimed: a rewrite hook on the
// reader ends one or the other, and Initialize then returns the answer it
// recorded, 200 times each (0030-PLAN-record-initialize-on-arrival.md, the
// deviation during step 2).
func TestInitializeGiveUpAgrees(t *testing.T) {
	t.Parallel()
	for _, how := range []string{"the caller's context ends", "the connection closes"} {
		t.Run(how, func(t *testing.T) {
			t.Parallel()
			for i := range 200 {
				ctx, cancel := context.WithCancel(t.Context())
				var c *ClientSideConnection
				c, p, _ := initializeClient(t, WithInboundRewrite(func(_ context.Context, _ string, params any) error {
					if _, ok := params.(*InitializeResponse); ok {
						if how == "the caller's context ends" {
							cancel()
						} else {
							_ = c.conn.Close()
						}
						// The caller sees the end while the commit runs.
						time.Sleep(5 * time.Millisecond)
					}
					return nil
				}))
				_, err := initializeAgainst(ctx, t, c, p, "")
				_, recorded := c.Negotiated()
				cancel()
				if err != nil || !recorded {
					t.Fatalf("trial %d: Initialize = %v with a record %v; want the answer it recorded", i, err, recorded)
				}
				_ = c.conn.Close()
			}
		})
	}
}

// TestHeldUntilInitialized requires an extension notification and an
// extension request that the agent writes before its initialize answer, in
// the same write, to be held until the answer is recorded and then handled,
// 200 times (0030-MADR-record-initialize-on-arrival.md, question 2).
func TestHeldUntilInitialized(t *testing.T) {
	t.Parallel()
	for i := range 200 {
		c, p, ext := initializeClient(t)
		if _, err := initializeWith(t.Context(), t, c, p, func(id string) string {
			return extNotification + "\n" + extRequest + "\n" + fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, id, initializeAnswer)
		}); err != nil {
			t.Fatalf("trial %d: Initialize = %v", i, err)
		}
		if a := p.next("the extension request's answer"); string(a.ID) != `"a1"` || a.Error != nil {
			t.Fatalf("trial %d: the extension request sent before the answer was answered %s %+v, want a result", i, a.ID, a.Error)
		}
		got := []string{within(t, ext, "an extension handler"), within(t, ext, "the other extension handler")}
		slices.Sort(got)
		if want := []string{"_x/ask", "_x/hello"}; !slices.Equal(got, want) {
			t.Fatalf("trial %d: the extension handlers ran for %q, want %q", i, got, want)
		}
		if err := c.conn.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// TestHeldRefusedWhenInitializeFails requires the extension messages held
// while initialize is in flight to be refused, as sent before initialize
// completed, once it fails: on an error answer, on an answer with another
// protocol version, when the caller's context ends first, and when the
// connection closes first. None waits past within's bound after Initialize
// returns, no handler runs, and the request, while the connection lives, is
// answered -32601 (0030-MADR-record-initialize-on-arrival.md, questions 2
// and 3).
func TestHeldRefusedWhenInitializeFails(t *testing.T) {
	t.Parallel()
	held := extNotification + "\n" + extRequest
	for _, tt := range []struct {
		name  string
		reply func(id string, end func()) string
		alive bool // the connection lives after Initialize returns
	}{
		{"an error answer", func(id string, _ func()) string {
			return held + "\n" + fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"error":{"code":-32603,"message":"no"}}`, id)
		}, true},
		{"another protocol version", func(id string, _ func()) string {
			return held + "\n" + fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":2,"agentCapabilities":{},"authMethods":[]}}`, id)
		}, true},
		{"the caller's context ends first", func(_ string, end func()) string {
			time.AfterFunc(50*time.Millisecond, end)
			return held
		}, true},
		{"the connection closes first", func(_ string, end func()) string {
			time.AfterFunc(50*time.Millisecond, end)
			return held
		}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			seen := make(chan judgement, 4)
			c, p, ext := initializeClient(t, judged(seen))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			end := cancel
			if !tt.alive {
				end = func() { _ = c.conn.Close() }
			}
			if _, err := initializeWith(ctx, t, c, p, func(id string) string { return tt.reply(id, end) }); err == nil {
				t.Fatal("Initialize = nil, want its failure")
			}
			for method, err := range verdicts(t, seen) {
				if !errors.Is(err, ErrNotInitialized) {
					t.Errorf("%s, held while initialize was in flight, was judged %v, want ErrNotInitialized", method, err)
				}
			}
			if tt.alive {
				// A caller whose context ends cancels its request first.
				line := within(t, p.lines, "the extension request's answer")
				if strings.Contains(line, `"method":"`+ProtocolMethodCancelRequest+`"`) {
					line = within(t, p.lines, "the extension request's answer")
				}
				var a answer
				if err := json.Unmarshal([]byte(line), &a); err != nil || string(a.ID) != `"a1"` || a.Error == nil || a.Error.Code != -32601 {
					t.Errorf("the held extension request was answered %s (%v), want -32601", line, err)
				}
			}
			select {
			case m := <-ext:
				t.Errorf("the handler ran for %s, refused while held", m)
			default:
			}
		})
	}
}

// TestNotHeldBeforeInitialize requires an extension notification and an
// extension request received before Initialize is called to be refused at
// once, as before: no initialize is in flight to hold them for
// (0030-MADR-record-initialize-on-arrival.md, question 2).
func TestNotHeldBeforeInitialize(t *testing.T) {
	t.Parallel()
	seen := make(chan judgement, 4)
	_, p, ext := initializeClient(t, judged(seen))
	p.send(extNotification)
	p.send(extRequest)
	for method, err := range verdicts(t, seen) {
		if !errors.Is(err, ErrNotInitialized) {
			t.Errorf("%s, received before Initialize, was judged %v, want ErrNotInitialized", method, err)
		}
	}
	if a := p.next("the extension request's answer"); string(a.ID) != `"a1"` || a.Error == nil || a.Error.Code != -32601 {
		t.Errorf("the extension request was answered %s %+v, want -32601", a.ID, a.Error)
	}
	select {
	case m := <-ext:
		t.Errorf("the handler ran for %s before initialize", m)
	default:
	}
}

// TestHeldNotificationsFillQueue requires notifications held while initialize
// is in flight to stay in the notification queue: three, each in its own
// write 100 ms apart, overflow a queue of one, and the connection ends, where
// refused at once each would have left it
// (0030-MADR-record-initialize-on-arrival.md, "Amendment 2026-10-21, step 9:
// what a held message holds", M1).
func TestHeldNotificationsFillQueue(t *testing.T) {
	t.Parallel()
	c, p, _ := initializeClient(t, WithMaxQueuedNotifications(1))
	done := make(chan error, 1)
	go func() {
		_, err := c.Initialize(t.Context(), InitializeRequest{})
		done <- err
	}()
	req := p.next("the initialize request")
	for range 3 {
		p.send(extNotification)
		time.Sleep(100 * time.Millisecond)
	}
	select {
	case <-c.Done():
	default:
		p.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, initializeAnswer))
	}
	if err := within(t, done, "Initialize"); err == nil {
		t.Error("Initialize = nil, want an error: the connection ended before the answer")
	}
	within(t, c.Done(), "the connection's end")
	if err := c.Err(); !errors.Is(err, ErrNotificationQueueOverflow) {
		t.Errorf("the connection ended with %v, want ErrNotificationQueueOverflow", err)
	}
}

// TestHeldRequestsKeepSlots requires a request held while initialize is in
// flight to keep its request slot: with one slot, a second request before the
// answer is answered -32603 at once, and the first, once the answer is
// recorded, with a result (the amendment above, M3).
func TestHeldRequestsKeepSlots(t *testing.T) {
	t.Parallel()
	c, p, _ := initializeClient(t, WithMaxConcurrentRequests(1))
	done := make(chan error, 1)
	go func() {
		_, err := c.Initialize(t.Context(), InitializeRequest{})
		done <- err
	}()
	req := p.next("the initialize request")
	p.send(`{"jsonrpc":"2.0","id":"r1","method":"_x/ask","params":{}}`)
	time.Sleep(100 * time.Millisecond)
	p.send(`{"jsonrpc":"2.0","id":"r2","method":"_x/ask","params":{}}`)
	if a := p.next("the second request's answer"); string(a.ID) != `"r2"` || a.Error == nil || a.Error.Code != -32603 ||
		!strings.Contains(fmt.Sprint(a.Error.Data), "too many concurrent requests") {
		t.Fatalf("before the answer, the client wrote %s %+v; want r2 answered -32603, too many concurrent requests", a.ID, a.Error)
	}
	p.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, initializeAnswer))
	if err := within(t, done, "Initialize"); err != nil {
		t.Fatalf("Initialize = %v", err)
	}
	if a := p.next("the first request's answer"); string(a.ID) != `"r1"` || a.Error != nil {
		t.Errorf("the request held in its slot was answered %s %+v, want a result", a.ID, a.Error)
	}
}

// TestInitializeHookHoldsReader requires the inbound hooks to see the
// initialize answer on the reader: while a rewrite hook blocks on it, the
// line after the answer, not JSON, which the reader answers itself, is not
// answered; once the hook returns it is answered -32700 and Initialize
// returns the answer (the amendment above, M4; "Amendment 2026-10-21, step 2:
// the commit's checks, and a caller that gives up").
func TestInitializeHookHoldsReader(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	c, p, _ := initializeClient(t, WithInboundRewrite(func(_ context.Context, _ string, params any) error {
		if _, ok := params.(*InitializeResponse); ok {
			<-release
		}
		return nil
	}))
	done := make(chan error, 1)
	go func() {
		_, err := c.Initialize(t.Context(), InitializeRequest{})
		done <- err
	}()
	req := p.next("the initialize request")
	p.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, initializeAnswer) + "\nnot json")
	select {
	case line := <-p.lines:
		close(release)
		t.Fatalf("while the hook held the answer, the client wrote %s, want nothing: the reader runs the hook", line)
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	if a := p.next("the answer to the line that is not JSON"); a.Error == nil || a.Error.Code != -32700 {
		t.Errorf("the line after the answer was answered %+v, want -32700", a.Error)
	}
	if err := within(t, done, "Initialize"); err != nil {
		t.Errorf("Initialize = %v, want the answer", err)
	}
}

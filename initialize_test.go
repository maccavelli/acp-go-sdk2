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
	p.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, initializeAnswer) + more)
	r := within(t, done, "Initialize")
	return r.resp, r.err
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

package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"
)

// refusalWire is a raw peer of an agent with record n and opts, whose
// session/new answers "s", and the agent.
func refusalWire(t *testing.T, n *Negotiation, opts ...ConnectionOption) (wire, *AgentSideConnection) {
	t.Helper()
	var asc *AgentSideConnection
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		asc = NewAgentSideConnection(agentFuncs{NewSessionFunc: func(context.Context, NewSessionRequest) (NewSessionResponse, error) {
			return NewSessionResponse{SessionId: "s"}, nil
		}}, w, r, opts...)
		negotiate(t, asc, n)
	})
	return p, asc
}

// newSessionLine is a session/new request with id and cwd.
func newSessionLine(t *testing.T, id, cwd string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": AgentMethodSessionNew,
		"params": map[string]any{"cwd": cwd, "mcpServers": []any{}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// eventually waits up to 2s for count to return want, and reports what it
// last saw.
func eventually(t *testing.T, count func() uint64, want uint64, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		got := count()
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("%s: RefusedMessages() = %d, want %d", what, got, want)
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestRefusedMessages requires RefusedMessages to count each message
// checkInbound refuses, once, whatever refused it, and nothing else
// (0038-PLAN-consumer-helpers-kind-and-agent-refusals.md, step 6).
func TestRefusedMessages(t *testing.T) {
	t.Parallel()
	t.Run("rewrite hook", testRefusedRewrite)
	t.Run("validation", testRefusedValidation)
	t.Run("validation accepted by the hook", testRefusedAccepted)
	t.Run("gate", testRefusedGate)
	t.Run("malformed and unknown", testRefusedNotDecoded)
	t.Run("extension notification", testRefusedNotification)
	t.Run("response", testRefusedResponse)
	t.Run("concurrent", testRefusedConcurrent)
}

func testRefusedRewrite(t *testing.T) {
	t.Parallel()
	refuse := WithInboundRewrite(func(context.Context, string, any) error { return errors.New("refused by the hook") })
	p, asc := refusalWire(t, allAdvertised(), refuse)
	p.send(newSessionLine(t, "1", testAbsPath("w")))
	if a := p.next("rewrite refusal"); a.Error == nil {
		t.Fatalf("answered %s, want an error", a.Result)
	}
	eventually(t, asc.Connection().RefusedMessages, 1, "rewrite")
}

func testRefusedValidation(t *testing.T) {
	t.Parallel()
	p, asc := refusalWire(t, allAdvertised())
	p.send(newSessionLine(t, "1", "relative"))
	if a := p.next("validation refusal"); a.Error == nil || a.Error.Code != -32602 {
		t.Fatalf("answered %+v, want -32602", a.Error)
	}
	eventually(t, asc.Connection().RefusedMessages, 1, "validation")
}

func testRefusedAccepted(t *testing.T) {
	t.Parallel()
	accept := WithInboundValidation(func(context.Context, string, any, error) error { return nil })
	p, asc := refusalWire(t, allAdvertised(), accept)
	p.send(newSessionLine(t, "1", "relative"))
	if a := p.next("accepted"); a.Error != nil {
		t.Fatalf("answered %+v, want the handler's result", a.Error)
	}
	if got := asc.Connection().RefusedMessages(); got != 0 {
		t.Errorf("accepted: RefusedMessages() = %d, want 0", got)
	}
}

func testRefusedGate(t *testing.T) {
	t.Parallel()
	p, asc := refusalWire(t, versionOnly())
	p.send(`{"jsonrpc":"2.0","id":"1","method":"session/list","params":{}}`)
	if a := p.next("gate refusal"); a.Error == nil || a.Error.Code != -32601 {
		t.Fatalf("answered %+v, want -32601", a.Error)
	}
	eventually(t, asc.Connection().RefusedMessages, 1, "gate")
}

func testRefusedNotDecoded(t *testing.T) {
	t.Parallel()
	p, asc := refusalWire(t, allAdvertised())
	p.send("not json")
	if a := p.next("malformed"); a.Error == nil || a.Error.Code != -32700 {
		t.Fatalf("answered %+v, want -32700", a.Error)
	}
	p.send(probe)
	if a := p.next("unknown method"); a.Error == nil || a.Error.Code != -32601 {
		t.Fatalf("answered %+v, want -32601", a.Error)
	}
	if got := asc.Connection().RefusedMessages(); got != 0 {
		t.Errorf("malformed and unknown: RefusedMessages() = %d, want 0", got)
	}
}

func testRefusedNotification(t *testing.T) {
	t.Parallel()
	refuse := WithInboundRewrite(func(context.Context, string, any) error { return errors.New("refused by the hook") })
	p, asc := refusalWire(t, allAdvertised(), refuse)
	p.send(`{"jsonrpc":"2.0","method":"_test/note","params":{}}`)
	eventually(t, asc.Connection().RefusedMessages, 1, "extension notification")
}

func testRefusedResponse(t *testing.T) {
	t.Parallel()
	var csc *ClientSideConnection
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		csc = NewClientSideConnection(&clientFuncs{}, w, r)
		negotiate(t, csc, allAdvertised())
	})
	errs := make(chan error, 1)
	go func() {
		_, err := csc.ListSessions(listCtx(t), ListSessionsRequest{})
		errs <- err
	}()
	req := p.next("the session/list request")
	p.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"sessions":[{"sessionId":"s","cwd":"relative"}]}}`, req.ID))
	if err := within(t, errs, "ListSessions"); err == nil {
		t.Fatal("a response with a relative cwd was accepted")
	}
	eventually(t, csc.Connection().RefusedMessages, 1, "response")
}

func testRefusedConcurrent(t *testing.T) {
	t.Parallel()
	const n = 50
	p, asc := refusalWire(t, allAdvertised())
	for i := range n {
		p.send(newSessionLine(t, strconv.Itoa(i), "relative"))
	}
	for i := range n {
		if a := p.next(fmt.Sprintf("answer %d", i)); a.Error == nil {
			t.Fatalf("answer %d: %s, want an error", i, a.Result)
		}
	}
	eventually(t, asc.Connection().RefusedMessages, n, "concurrent")
}

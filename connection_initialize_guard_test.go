package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"
)

// The agent's guard on its initialize answer
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, R2, and its Q2 a): the
// agent records the answer its dispatch gives, so a receiving middleware that
// replaces or changes it is refused, as the sending side refuses one
// (0046-MADR-sending-and-receiving-middleware.md, D10).

// initializeStep is what a receiving middleware does with initialize.
type initializeStep func(next MethodHandler, ctx context.Context, params json.RawMessage) (any, *RequestError)

// onInitialize is a receiving middleware that runs step for initialize and
// passes every other message on.
func onInitialize(step initializeStep) Middleware {
	return func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
			if method != AgentMethodInitialize {
				return next(ctx, method, params)
			}
			return step(next, ctx, params)
		}
	}
}

// guardedPair connects a client to an agent built with mw, and returns both.
func guardedPair(t *testing.T, mw Middleware) (*ClientSideConnection, *AgentSideConnection) {
	t.Helper()
	p := newShutdownPipes(t, &shutdownClient{})
	as := NewAgentSideConnection(&shutdownAgent{}, p.agentOut, p.agentIn, WithReceivingMiddleware(mw), quietLogs())
	t.Cleanup(func() { _ = as.Close() })
	return p.client, as
}

// TestInitializeGuardRefusesAChangedAnswer requires an agent's initialize,
// answered by a receiving middleware with a result other than the
// dispatch's, to be answered -32603 instead: a replaced result, one changed
// in place, and one given without the dispatch running. The client then has
// no record.
func TestInitializeGuardRefusesAChangedAnswer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		step initializeStep
	}{
		{"a replaced result", func(next MethodHandler, ctx context.Context, params json.RawMessage) (any, *RequestError) {
			res, e := next(ctx, AgentMethodInitialize, params)
			if r, ok := res.(*InitializeResponse); ok {
				out := *r
				out.AgentCapabilities.LoadSession = true
				return &out, nil
			}
			return res, e
		}},
		{"a result changed in place", func(next MethodHandler, ctx context.Context, params json.RawMessage) (any, *RequestError) {
			res, e := next(ctx, AgentMethodInitialize, params)
			if r, ok := res.(*InitializeResponse); ok {
				r.AgentCapabilities.LoadSession = true
			}
			return res, e
		}},
		{"a result without the dispatch", func(MethodHandler, context.Context, json.RawMessage) (any, *RequestError) {
			return &InitializeResponse{ProtocolVersion: ProtocolVersionNumber}, nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, _ := guardedPair(t, onInitialize(tc.step))
			_, err := client.Initialize(t.Context(), InitializeRequest{ProtocolVersion: ProtocolVersionNumber})
			re, ok := errors.AsType[*RequestError](err)
			if !ok || re.Code != -32603 || fmt.Sprint(re.Data) != "map[error:a receiving middleware replaced initialize's result]" {
				t.Fatalf("Initialize = %v, want -32603 naming the replaced result", err)
			}
			if _, recorded := client.Negotiated(); recorded {
				t.Error("the client recorded an initialize the agent refused")
			}
		})
	}
}

// TestInitializeGuardAllowsAnErrorAndAPassThrough requires a receiving
// middleware that fails initialize with an error to have that error
// answered, as on the sending side, and one that passes the dispatch's
// answer on to leave the client's record and the agent's the same.
func TestInitializeGuardAllowsAnErrorAndAPassThrough(t *testing.T) {
	t.Parallel()
	t.Run("an error", func(t *testing.T) {
		t.Parallel()
		client, _ := guardedPair(t, onInitialize(func(next MethodHandler, ctx context.Context, params json.RawMessage) (any, *RequestError) {
			if _, e := next(ctx, AgentMethodInitialize, params); e != nil {
				return nil, e
			}
			return nil, NewInternalError(map[string]any{"error": "the middleware's own refusal"})
		}))
		_, err := client.Initialize(t.Context(), InitializeRequest{ProtocolVersion: ProtocolVersionNumber})
		if re, ok := errors.AsType[*RequestError](err); !ok || fmt.Sprint(re.Data) != "map[error:the middleware's own refusal]" {
			t.Errorf("Initialize = %v, want the middleware's own error", err)
		}
	})
	t.Run("a pass-through", func(t *testing.T) {
		t.Parallel()
		client, agent := guardedPair(t, onInitialize(func(next MethodHandler, ctx context.Context, params json.RawMessage) (any, *RequestError) {
			return next(ctx, AgentMethodInitialize, params)
		}))
		if _, err := client.Initialize(t.Context(), InitializeRequest{ProtocolVersion: ProtocolVersionNumber}); err != nil {
			t.Fatal(err)
		}
		cn, cok := client.Negotiated()
		an, aok := agent.Negotiated()
		cb, _ := json.Marshal(cn.Response)
		ab, _ := json.Marshal(an.Response)
		if !cok || !aok || string(cb) != string(ab) {
			t.Errorf("records: client %v %s, agent %v %s; want both, the same", cok, cb, aok, ab)
		}
	})
}

// TestInitializeGuardOnlyOnTheAgent requires a raw Connection, which records
// nothing, to answer a method named initialize with what its receiving chain
// returns: the guard is the agent side connection's.
func TestInitializeGuardOnlyOnTheAgent(t *testing.T) {
	t.Parallel()
	replace := func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
			if _, e := next(ctx, method, params); e != nil {
				return nil, e
			}
			return map[string]any{"replaced": true}, nil
		}
	}
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		NewConnection(func(context.Context, string, json.RawMessage) (any, *RequestError) {
			return map[string]any{}, nil
		}, w, r, WithReceivingMiddleware(replace), quietLogs())
	})
	p.send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if a := p.next("the answer"); a.Error != nil || string(a.Result) != `{"replaced":true}` {
		t.Errorf("answered %s %+v, want the chain's result", a.Result, a.Error)
	}
}

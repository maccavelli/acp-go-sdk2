package acp

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
)

// The benchmarks of the wire paths that 0019-PLAN-idioms-performance.md
// measures (D13). `make bench` runs them six times; CI does not run them.

// sinkBytes keeps the compiler from discarding an encoding.
var sinkBytes []byte

// chunkUpdate returns the update an agent streams most: a text chunk.
func chunkUpdate() SessionUpdate {
	return SessionUpdate{AgentMessageChunk: &SessionUpdateAgentMessageChunk{Content: TextBlock("hello, this is a streamed chunk of text")}}
}

func BenchmarkSessionUpdateMarshal(b *testing.B) {
	u := chunkUpdate()
	for b.Loop() {
		out, err := json.Marshal(u)
		if err != nil {
			b.Fatal(err)
		}
		sinkBytes = out
	}
}

func BenchmarkSessionUpdateUnmarshal(b *testing.B) {
	raw, err := json.Marshal(chunkUpdate())
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		var u SessionUpdate
		if err := json.Unmarshal(raw, &u); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionNotificationUnmarshal(b *testing.B) {
	raw, err := json.Marshal(SessionNotification{SessionId: "s", Update: chunkUpdate()})
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		var n SessionNotification
		if err := json.Unmarshal(raw, &n); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCanonicalIDKey(b *testing.B) {
	raw := json.RawMessage("12345")
	for b.Loop() {
		if _, err := canonicalJSONRPCIDKey(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateOutboundValue(b *testing.B) {
	p := PromptRequest{SessionId: "s", Prompt: []ContentBlock{TextBlock("hi")}}
	for b.Loop() {
		if err := validateOutbound(p); err != nil {
			b.Fatal(err)
		}
	}
}

// benchPair connects an agent built from a and a client built from c over
// pipes, both negotiated with every capability, logging nothing, and given
// opts.
func benchPair(b *testing.B, a agentFuncs, c *clientFuncs, opts ...ConnectionOption) (*AgentSideConnection, *ClientSideConnection) {
	b.Helper()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	b.Cleanup(func() { _ = c2aW.Close(); _ = a2cW.Close() })
	opts = append([]ConnectionOption{WithLogger(slog.New(slog.DiscardHandler))}, opts...)
	asc := NewAgentSideConnection(a, a2cW, c2aR, opts...)
	csc := NewClientSideConnection(c, c2aW, a2cR, opts...)
	asc.negotiation.Store(allAdvertised())
	csc.negotiation.Store(allAdvertised())
	return asc, csc
}

func BenchmarkPromptRoundTrip(b *testing.B) {
	_, csc := benchPair(b, agentFuncs{PromptFunc: func(context.Context, PromptRequest) (PromptResponse, error) {
		return PromptResponse{StopReason: StopReasonEndTurn}, nil
	}}, &clientFuncs{})
	ctx := b.Context()
	req := PromptRequest{SessionId: "s", Prompt: []ContentBlock{TextBlock("hi")}}
	for b.Loop() {
		if _, err := csc.Prompt(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}

// benchPass is a middleware that wraps next in a call of its own, as a
// tracing or metrics middleware does, so the chain's cost is measured.
func benchPass(next MethodHandler) MethodHandler {
	return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
		return next(ctx, method, params)
	}
}

// BenchmarkPromptRoundTripMiddleware is BenchmarkPromptRoundTrip with one
// middleware each way on both sides
// (0046-PLAN-sending-and-receiving-middleware.md, step 5.4).
func BenchmarkPromptRoundTripMiddleware(b *testing.B) {
	_, csc := benchPair(b, agentFuncs{PromptFunc: func(context.Context, PromptRequest) (PromptResponse, error) {
		return PromptResponse{StopReason: StopReasonEndTurn}, nil
	}}, &clientFuncs{}, WithReceivingMiddleware(benchPass), WithSendingMiddleware(benchPass))
	ctx := b.Context()
	req := PromptRequest{SessionId: "s", Prompt: []ContentBlock{TextBlock("hi")}}
	for b.Loop() {
		if _, err := csc.Prompt(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSessionUpdateNotifyObserved is BenchmarkSessionUpdateNotify with
// an observer set, which these messages never call (step 5.4).
func BenchmarkSessionUpdateNotifyObserved(b *testing.B) {
	got := make(chan struct{}, 1)
	asc, _ := benchPair(b, agentFuncs{}, &clientFuncs{SessionUpdateFunc: func(context.Context, SessionNotification) error {
		got <- struct{}{}
		return nil
	}}, WithObserver(func(Observation) {}))
	ctx := b.Context()
	n := SessionNotification{SessionId: "s", Update: chunkUpdate()}
	for b.Loop() {
		if err := asc.SessionUpdate(ctx, n); err != nil {
			b.Fatal(err)
		}
		<-got
	}
}

func BenchmarkSessionUpdateNotify(b *testing.B) {
	got := make(chan struct{}, 1)
	asc, _ := benchPair(b, agentFuncs{}, &clientFuncs{SessionUpdateFunc: func(context.Context, SessionNotification) error {
		got <- struct{}{}
		return nil
	}})
	ctx := b.Context()
	n := SessionNotification{SessionId: "s", Update: chunkUpdate()}
	for b.Loop() {
		if err := asc.SessionUpdate(ctx, n); err != nil {
			b.Fatal(err)
		}
		<-got
	}
}

package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// within returns the next value of ch, or fails the test after ten seconds.
func within[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not happen", what)
		panic("unreachable")
	}
}

// promptPair connects an agent built from a and a client over pipes.
func promptPair(t *testing.T, a agentFuncs) *ClientSideConnection {
	t.Helper()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aW.Close()
		_ = a2cW.Close()
	})
	NewAgentSideConnection(a, a2cW, c2aR)
	return NewClientSideConnection(&clientFuncs{}, c2aW, a2cR)
}

// prompt runs Prompt for session s on its own goroutine and returns where its
// answer arrives.
func prompt(t *testing.T, c *ClientSideConnection, s SessionId) <-chan PromptResponse {
	t.Helper()
	done := make(chan PromptResponse, 1)
	go func() {
		resp, err := c.Prompt(context.Background(), PromptRequest{SessionId: s})
		if err != nil {
			t.Errorf("Prompt(%s): %v", s, err)
		}
		done <- resp
	}()
	return done
}

// TestPromptCancelledAnswersCancelled requires a turn cancelled by
// session/cancel to be answered with the cancelled stop reason whatever its
// handler returns, keeping the rest of a result it returns, with
// ErrPromptCancelled as its context's cause
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 5).
func TestPromptCancelledAnswersCancelled(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		answer func(ctx context.Context) (PromptResponse, error)
		meta   bool
	}{
		{"context error", func(ctx context.Context) (PromptResponse, error) { return PromptResponse{}, ctx.Err() }, false},
		{"wrapped context error", func(ctx context.Context) (PromptResponse, error) {
			return PromptResponse{}, fmt.Errorf("model call: %w", ctx.Err())
		}, false},
		{"end_turn with _meta", func(context.Context) (PromptResponse, error) {
			return PromptResponse{StopReason: StopReasonEndTurn, Meta: map[string]any{"k": "v"}}, nil
		}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			started, causes := make(chan struct{}, 1), make(chan error, 1)
			c := promptPair(t, agentFuncs{PromptFunc: func(ctx context.Context, _ PromptRequest) (PromptResponse, error) {
				started <- struct{}{}
				<-ctx.Done()
				causes <- context.Cause(ctx)
				return tt.answer(ctx)
			}})
			done := prompt(t, c, "s")
			within(t, started, "the turn")
			if err := c.Cancel(t.Context(), CancelNotification{SessionId: "s"}); err != nil {
				t.Fatal(err)
			}
			if cause := within(t, causes, "the turn's cancel"); cause != ErrPromptCancelled { //nolint:errorlint // the cause itself, not one it wraps
				t.Errorf("context.Cause = %v, want ErrPromptCancelled", cause)
			}
			resp := within(t, done, "the answer")
			if resp.StopReason != StopReasonCancelled {
				t.Errorf("stopReason %q, want cancelled", resp.StopReason)
			}
			if got := resp.Meta["k"]; tt.meta && got != "v" {
				t.Errorf("_meta %v, want the handler's", resp.Meta)
			}
		})
	}
}

// TestPromptCancelRequestAnswersCancelled requires a turn cancelled by
// $/cancel_request for its request to be answered with the cancelled stop
// reason, with context.Canceled as its cause
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 5).
func TestPromptCancelRequestAnswersCancelled(t *testing.T) {
	t.Parallel()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aW.Close()
		_ = a2cW.Close()
	})
	started, causes := make(chan struct{}, 1), make(chan error, 1)
	NewAgentSideConnection(agentFuncs{PromptFunc: func(ctx context.Context, _ PromptRequest) (PromptResponse, error) {
		started <- struct{}{}
		<-ctx.Done()
		causes <- context.Cause(ctx)
		return PromptResponse{}, ctx.Err()
	}}, a2cW, c2aR)
	lines := make(chan string, 4)
	go func() {
		sc := bufio.NewScanner(a2cR)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	write := func(s string) {
		if _, err := io.WriteString(c2aW, s+"\n"); err != nil {
			t.Error(err)
		}
	}
	write(`{"jsonrpc":"2.0","id":7,"method":"session/prompt","params":{"sessionId":"s","prompt":[]}}`)
	within(t, started, "the turn")
	write(`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":7}}`)
	if cause := within(t, causes, "the turn's cancel"); !errors.Is(cause, context.Canceled) {
		t.Errorf("context.Cause = %v, want context.Canceled", cause)
	}
	var msg struct {
		ID     json.RawMessage `json:"id"`
		Result *PromptResponse `json:"result"`
		Error  *RequestError   `json:"error"`
	}
	if err := json.Unmarshal([]byte(within(t, lines, "the answer")), &msg); err != nil {
		t.Fatal(err)
	}
	if string(msg.ID) != "7" || msg.Error != nil || msg.Result == nil || msg.Result.StopReason != StopReasonCancelled {
		t.Errorf("answer id %s result %+v error %v, want id 7 with stopReason cancelled", msg.ID, msg.Result, msg.Error)
	}
}

// TestPromptCancelAfterReturnChangesNothing requires a session/cancel that
// arrives after the handler returned, and after the turn was removed, to leave
// the handler's answer as it is (0017-MADR, P1 as decided at acceptance;
// 0017-PLAN-remediate-acp-v1-conformance-violations.md, step 5). It sets the
// package's promptReturned, so it does not run in parallel.
func TestPromptCancelAfterReturnChangesNothing(t *testing.T) {
	returned, release := make(chan struct{}, 1), make(chan struct{})
	promptReturned = func() {
		returned <- struct{}{}
		<-release
	}
	t.Cleanup(func() { promptReturned = nil })
	cancelled := make(chan struct{}, 1)
	c := promptPair(t, agentFuncs{
		PromptFunc: func(context.Context, PromptRequest) (PromptResponse, error) {
			return PromptResponse{StopReason: StopReasonEndTurn}, nil
		},
		CancelFunc: func(context.Context, CancelNotification) error {
			cancelled <- struct{}{}
			return nil
		},
	})
	done := prompt(t, c, "s")
	within(t, returned, "the handler's return")
	if err := c.Cancel(t.Context(), CancelNotification{SessionId: "s"}); err != nil {
		t.Fatal(err)
	}
	// The agent's Cancel runs after the case looked for the session's turn.
	within(t, cancelled, "the agent's Cancel")
	close(release)
	if resp := within(t, done, "the answer"); resp.StopReason != StopReasonEndTurn {
		t.Errorf("stopReason %q, want the handler's end_turn", resp.StopReason)
	}
}

// TestPromptSupersededKeepsSuccessorCancel requires a turn superseded by a
// later prompt for its session to be answered cancelled, with a cause that
// wraps ErrPromptCancelled, and session/cancel after it returned to cancel its
// successor (the MADR's probe; 0017-PLAN-remediate-acp-v1-conformance-violations.md, step 5).
func TestPromptSupersededKeepsSuccessorCancel(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	started, causes := make(chan int32, 2), make(chan error, 2)
	c := promptPair(t, agentFuncs{PromptFunc: func(ctx context.Context, _ PromptRequest) (PromptResponse, error) {
		n := calls.Add(1)
		started <- n
		<-ctx.Done()
		causes <- context.Cause(ctx)
		return PromptResponse{}, ctx.Err()
	}})
	first := prompt(t, c, "s")
	within(t, started, "the first turn")
	second := prompt(t, c, "s")
	within(t, started, "the second turn")
	cause := within(t, causes, "the first turn's cancel")
	if !errors.Is(cause, ErrPromptCancelled) || !strings.Contains(cause.Error(), "superseded") {
		t.Errorf("first turn's cause %v, want one wrapping ErrPromptCancelled that says superseded", cause)
	}
	if resp := within(t, first, "the first answer"); resp.StopReason != StopReasonCancelled {
		t.Errorf("first turn stopReason %q, want cancelled", resp.StopReason)
	}
	if err := c.Cancel(t.Context(), CancelNotification{SessionId: "s"}); err != nil {
		t.Fatal(err)
	}
	if cause := within(t, causes, "the second turn's cancel"); cause != ErrPromptCancelled { //nolint:errorlint // the cause itself, not one it wraps
		t.Errorf("second turn's cause %v, want ErrPromptCancelled", cause)
	}
	if resp := within(t, second, "the second answer"); resp.StopReason != StopReasonCancelled {
		t.Errorf("second turn stopReason %q, want cancelled", resp.StopReason)
	}
}

// TestPromptCancelRightAfterPrompt writes session/prompt and session/cancel for
// one session back to back, for 200 sessions, and requires every turn to be
// cancelled and answered cancelled: a cancel handled before its turn is stored
// still applies to it, ordered by receipt
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 5 deviation).
func TestPromptCancelRightAfterPrompt(t *testing.T) {
	t.Parallel()
	const n = 200
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aW.Close()
		_ = a2cW.Close()
	})
	var missed atomic.Int32
	NewAgentSideConnection(agentFuncs{PromptFunc: func(ctx context.Context, _ PromptRequest) (PromptResponse, error) {
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
			missed.Add(1)
		}
		return PromptResponse{}, ctx.Err()
	}}, a2cW, c2aR)
	answers := make(chan string, n)
	go func() {
		sc := bufio.NewScanner(a2cR)
		for sc.Scan() {
			answers <- sc.Text()
		}
	}()
	go func() {
		for i := range n {
			s := fmt.Sprintf("s%d", i)
			_, _ = fmt.Fprintf(c2aW, "%s\n%s\n",
				fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"session/prompt","params":{"sessionId":%q,"prompt":[]}}`, i+1, s),
				fmt.Sprintf(`{"jsonrpc":"2.0","method":"session/cancel","params":{"sessionId":%q}}`, s))
		}
	}()
	for range n {
		var msg struct {
			Result *PromptResponse `json:"result"`
		}
		line := within(t, answers, "an answer")
		if err := json.Unmarshal([]byte(line), &msg); err != nil || msg.Result == nil || msg.Result.StopReason != StopReasonCancelled {
			t.Errorf("answer %s, want stopReason cancelled", line)
		}
	}
	if m := missed.Load(); m != 0 {
		t.Errorf("%d of %d turns were not cancelled", m, n)
	}
}

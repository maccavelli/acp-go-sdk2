package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
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

// promptPair connects an agent built from a with opts and a client over pipes.
func promptPair(t *testing.T, a agentFuncs, opts ...ConnectionOption) *ClientSideConnection {
	t.Helper()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aW.Close()
		_ = a2cW.Close()
	})
	negotiate(t, NewAgentSideConnection(a, a2cW, c2aR, opts...), allAdvertised())
	csc := NewClientSideConnection(&clientFuncs{}, c2aW, a2cR)
	negotiate(t, csc, allAdvertised())
	return csc
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
	asc := NewAgentSideConnection(agentFuncs{PromptFunc: func(ctx context.Context, _ PromptRequest) (PromptResponse, error) {
		started <- struct{}{}
		<-ctx.Done()
		causes <- context.Cause(ctx)
		return PromptResponse{}, ctx.Err()
	}}, a2cW, c2aR)
	negotiate(t, asc, allAdvertised())
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
	asc := NewAgentSideConnection(agentFuncs{PromptFunc: func(ctx context.Context, _ PromptRequest) (PromptResponse, error) {
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
			missed.Add(1)
		}
		return PromptResponse{}, ctx.Err()
	}}, a2cW, c2aR)
	negotiate(t, asc, allAdvertised())
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

// permissionPair connects an agent and a client whose RequestPermission is
// handle, over pipes, and returns the agent's connection and the client's.
func permissionPair(t *testing.T, a agentFuncs, handle func(context.Context, RequestPermissionRequest) (RequestPermissionResponse, error)) (*AgentSideConnection, *ClientSideConnection) {
	t.Helper()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aW.Close()
		_ = a2cW.Close()
	})
	asc := NewAgentSideConnection(a, a2cW, c2aR)
	csc := NewClientSideConnection(&clientFuncs{RequestPermissionFunc: handle}, c2aW, a2cR)
	negotiate(t, asc, allAdvertised())
	negotiate(t, csc, allAdvertised())
	return asc, csc
}

// askPermission sends a permission request for tool call id in session s from
// the agent on its own goroutine, and returns where its answer arrives.
func askPermission(t *testing.T, asc *AgentSideConnection, s SessionId, id ToolCallId) <-chan RequestPermissionResponse {
	t.Helper()
	answer := make(chan RequestPermissionResponse, 1)
	go func() {
		resp, err := asc.RequestPermission(context.Background(), RequestPermissionRequest{SessionId: s, ToolCall: ToolCallUpdate{ToolCallId: id}})
		if err != nil {
			t.Errorf("RequestPermission(%s, %s): %v", s, id, err)
		}
		answer <- resp
	}()
	return answer
}

// TestCancelAnswersPendingPermissions requires session/cancel, sent with the
// client's Cancel, to answer every pending permission request of its session
// with the cancelled outcome at once, whether or not the handler watches its
// context, and to leave another session's alone; the watching handler sees
// ErrPromptCancelled (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 6).
func TestCancelAnswersPendingPermissions(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	started, causes := make(chan ToolCallId, 3), make(chan error, 1)
	asc, c := permissionPair(t, agentFuncs{}, func(ctx context.Context, p RequestPermissionRequest) (RequestPermissionResponse, error) {
		started <- p.ToolCall.ToolCallId
		if p.ToolCall.ToolCallId == "watch" {
			<-ctx.Done()
			causes <- context.Cause(ctx)
			return RequestPermissionResponse{}, ctx.Err()
		}
		<-release
		return RequestPermissionResponse{Outcome: NewRequestPermissionOutcomeSelected("allow")}, nil
	})
	watch, ignore, other := askPermission(t, asc, "s", "watch"), askPermission(t, asc, "s", "ignore"), askPermission(t, asc, "t", "other")
	for range 3 {
		within(t, started, "a permission request")
	}
	if err := c.Cancel(t.Context(), CancelNotification{SessionId: "s"}); err != nil {
		t.Fatal(err)
	}
	for name, ch := range map[string]<-chan RequestPermissionResponse{"watch": watch, "ignore": ignore} {
		if resp := within(t, ch, name+"'s answer"); resp.Outcome.Cancelled == nil {
			t.Errorf("%s answered %+v, want the cancelled outcome", name, resp.Outcome)
		}
	}
	if cause := within(t, causes, "the watching handler's cancel"); cause != ErrPromptCancelled { //nolint:errorlint // the cause itself, not one it wraps
		t.Errorf("context.Cause = %v, want ErrPromptCancelled", cause)
	}
	select {
	case resp := <-other:
		t.Errorf("session t's request answered %+v by session s's cancel", resp.Outcome)
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	if resp := within(t, other, "session t's answer"); resp.Outcome.Selected == nil {
		t.Errorf("session t's request answered %+v, want its handler's selected outcome", resp.Outcome)
	}
}

// TestPromptCancelAnswersPendingPermissions requires the session/cancel that
// Prompt sends when its context ends to answer the session's pending
// permission requests with the cancelled outcome
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 6).
func TestPromptCancelAnswersPendingPermissions(t *testing.T) {
	t.Parallel()
	var asc *AgentSideConnection
	answers := make(chan RequestPermissionResponse, 1)
	asked := make(chan struct{}, 1)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	asc, c := permissionPair(t, agentFuncs{PromptFunc: func(ctx context.Context, p PromptRequest) (PromptResponse, error) {
		resp, err := asc.RequestPermission(context.Background(), RequestPermissionRequest{SessionId: p.SessionId, ToolCall: ToolCallUpdate{ToolCallId: "t1"}})
		if err != nil {
			t.Errorf("RequestPermission: %v", err)
		}
		answers <- resp
		<-ctx.Done()
		return PromptResponse{}, ctx.Err()
	}}, func(context.Context, RequestPermissionRequest) (RequestPermissionResponse, error) {
		asked <- struct{}{}
		<-release
		return RequestPermissionResponse{Outcome: NewRequestPermissionOutcomeSelected("allow")}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _, _ = c.Prompt(ctx, PromptRequest{SessionId: "s"}) }()
	within(t, asked, "the permission request")
	cancel()
	if resp := within(t, answers, "the permission answer"); resp.Outcome.Cancelled == nil {
		t.Errorf("answered %+v, want the cancelled outcome", resp.Outcome)
	}
}

// TestPermissionAfterCancelHandledNormally requires a permission request that
// arrives after its session's cancel to get its handler's answer
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 6).
func TestPermissionAfterCancelHandledNormally(t *testing.T) {
	t.Parallel()
	asc, c := permissionPair(t, agentFuncs{}, func(context.Context, RequestPermissionRequest) (RequestPermissionResponse, error) {
		return RequestPermissionResponse{Outcome: NewRequestPermissionOutcomeSelected("allow")}, nil
	})
	if err := c.Cancel(t.Context(), CancelNotification{SessionId: "s"}); err != nil {
		t.Fatal(err)
	}
	if resp := within(t, askPermission(t, asc, "s", "t1"), "the answer"); resp.Outcome.Selected == nil || resp.Outcome.Selected.OptionId != "allow" {
		t.Errorf("answered %+v, want the handler's selected outcome", resp.Outcome)
	}
}

// TestCloseSessionCancelsTurn requires session/close to cancel the session's
// running turn, with ErrPromptCancelled as its cause and the cancelled stop
// reason as its answer, as if session/cancel had been sent
// (0019-PLAN-behaviour-fixes.md, B4).
func TestCloseSessionCancelsTurn(t *testing.T) {
	t.Parallel()
	started, causes := make(chan struct{}, 1), make(chan error, 1)
	c := promptPair(t, agentFuncs{PromptFunc: func(ctx context.Context, _ PromptRequest) (PromptResponse, error) {
		started <- struct{}{}
		<-ctx.Done()
		causes <- context.Cause(ctx)
		return PromptResponse{}, ctx.Err()
	}})
	answer := prompt(t, c, "s")
	within(t, started, "the turn")
	if _, err := c.CloseSession(t.Context(), CloseSessionRequest{SessionId: "s"}); err != nil {
		t.Fatal(err)
	}
	if cause := within(t, causes, "the turn's end"); !errors.Is(cause, ErrPromptCancelled) {
		t.Errorf("context.Cause = %v, want ErrPromptCancelled", cause)
	}
	if resp := within(t, answer, "the prompt's answer"); resp.StopReason != StopReasonCancelled {
		t.Errorf("the prompt was answered %q, want cancelled", resp.StopReason)
	}
}

// TestCloseSessionAnswersPendingPermissions requires a session/close the agent
// answers with success to answer the session's pending permission requests
// with the cancelled outcome, and to leave another session's alone
// (0019-PLAN-behaviour-fixes.md, B4).
func TestCloseSessionAnswersPendingPermissions(t *testing.T) {
	t.Parallel()
	release, started := make(chan struct{}), make(chan ToolCallId, 2)
	asc, c := permissionPair(t, agentFuncs{}, func(_ context.Context, p RequestPermissionRequest) (RequestPermissionResponse, error) {
		started <- p.ToolCall.ToolCallId
		<-release
		return RequestPermissionResponse{Outcome: NewRequestPermissionOutcomeSelected("allow")}, nil
	})
	mine, other := askPermission(t, asc, "s", "mine"), askPermission(t, asc, "t", "other")
	within(t, started, "a permission request")
	within(t, started, "a permission request")
	if _, err := c.CloseSession(t.Context(), CloseSessionRequest{SessionId: "s"}); err != nil {
		t.Fatal(err)
	}
	if resp := within(t, mine, "the closed session's answer"); resp.Outcome.Cancelled == nil {
		t.Errorf("the closed session's request answered %+v, want the cancelled outcome", resp.Outcome)
	}
	select {
	case resp := <-other:
		t.Errorf("session t's request answered %+v by session s's close", resp.Outcome)
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	if resp := within(t, other, "session t's answer"); resp.Outcome.Selected == nil {
		t.Errorf("session t's request answered %+v, want its handler's", resp.Outcome)
	}
}

// TestFailedCloseLeavesPermissionsPending requires a session/close the agent
// refuses to leave the session's pending permission requests to their
// handlers, the turn possibly still running (0019-PLAN-behaviour-fixes.md, B4).
func TestFailedCloseLeavesPermissionsPending(t *testing.T) {
	t.Parallel()
	release, started := make(chan struct{}), make(chan ToolCallId, 1)
	asc, c := permissionPair(t, agentFuncs{CloseSessionFunc: func(context.Context, CloseSessionRequest) (CloseSessionResponse, error) {
		return CloseSessionResponse{}, NewInvalidParams(map[string]any{"error": "unknown session"})
	}}, func(_ context.Context, p RequestPermissionRequest) (RequestPermissionResponse, error) {
		started <- p.ToolCall.ToolCallId
		<-release
		return RequestPermissionResponse{Outcome: NewRequestPermissionOutcomeSelected("allow")}, nil
	})
	answer := askPermission(t, asc, "s", "mine")
	within(t, started, "the permission request")
	if _, err := c.CloseSession(t.Context(), CloseSessionRequest{SessionId: "s"}); err == nil {
		t.Fatal("the refused close returned no error")
	}
	select {
	case resp := <-answer:
		t.Fatalf("a refused close answered the pending request %+v", resp.Outcome)
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	if resp := within(t, answer, "the handler's answer"); resp.Outcome.Selected == nil {
		t.Errorf("answered %+v, want the handler's selected outcome", resp.Outcome)
	}
}

// receiptAgent is an agent whose turns run until cancelled or two seconds,
// handled through its dispatch with the receipt numbers a test gives, as the
// receive loop would give them.
type receiptAgent struct {
	t       *testing.T
	asc     *AgentSideConnection
	started chan struct{}
}

func newReceiptAgent(t *testing.T, opts ...ConnectionOption) *receiptAgent {
	t.Helper()
	ra := &receiptAgent{t: t, started: make(chan struct{}, 4)}
	ra.asc = NewAgentSideConnection(agentFuncs{PromptFunc: func(ctx context.Context, _ PromptRequest) (PromptResponse, error) {
		ra.started <- struct{}{}
		select {
		case <-ctx.Done():
			return PromptResponse{}, ctx.Err()
		case <-time.After(2 * time.Second):
			return PromptResponse{StopReason: StopReasonEndTurn}, nil
		}
	}}, io.Discard, idleReader(t), opts...)
	negotiate(t, ra.asc, allAdvertised())
	return ra
}

// prompt handles session/prompt for session s with receipt r on its own
// goroutine and returns where its stop reason arrives.
func (ra *receiptAgent) prompt(r uint64) <-chan StopReason {
	out := make(chan StopReason, 1)
	go func() {
		res, rerr := ra.asc.handle(context.WithValue(ra.t.Context(), receiptKey{}, r), AgentMethodSessionPrompt,
			json.RawMessage(`{"sessionId":"s","prompt":[]}`))
		resp, ok := res.(PromptResponse)
		if rerr != nil || !ok {
			ra.t.Errorf("prompt %d: %v %v", r, res, rerr)
		}
		out <- resp.StopReason
	}()
	return out
}

// cancel handles session/cancel for session s with receipt r.
func (ra *receiptAgent) cancel(r uint64) {
	if _, rerr := ra.asc.handle(context.WithValue(ra.t.Context(), receiptKey{}, r), AgentMethodSessionCancel,
		json.RawMessage(`{"sessionId":"s"}`)); rerr != nil {
		ra.t.Fatalf("cancel %d: %v", r, rerr)
	}
}

// stored waits until session s's turn of receipt r is stored.
func (ra *receiptAgent) stored(r uint64) {
	for deadline := time.Now().Add(2 * time.Second); ; {
		ra.asc.mu.Lock()
		found := slices.ContainsFunc(ra.asc.sessionCancels["s"], func(t *promptTurn) bool { return t.receipt == r })
		ra.asc.mu.Unlock()
		if found {
			return
		}
		if time.Now().After(deadline) {
			ra.t.Fatalf("turn %d was not stored", r)
		}
		time.Sleep(time.Millisecond)
	}
}

// TestCancelOrderedAcrossTwoPrompts requires a cancel to apply to every prompt
// of its session received before it, whatever order the turns are stored in
// (0019-PLAN-behaviour-fixes.md, B5).
func TestCancelOrderedAcrossTwoPrompts(t *testing.T) {
	t.Parallel()
	t.Run("a stored turn and one stored after the cancel", func(t *testing.T) {
		t.Parallel()
		ra := newReceiptAgent(t)
		first := ra.prompt(1)
		within(t, ra.started, "turn 1")
		ra.stored(1)
		ra.cancel(3)
		second := ra.prompt(2)
		if got := within(t, first, "turn 1's answer"); got != StopReasonCancelled {
			t.Errorf("turn 1, received before the cancel, answered %q", got)
		}
		if got := within(t, second, "turn 2's answer"); got != StopReasonCancelled {
			t.Errorf("turn 2, received before the cancel and stored after it, answered %q", got)
		}
	})
	t.Run("two turns stored after the cancel", func(t *testing.T) {
		t.Parallel()
		ra := newReceiptAgent(t)
		ra.cancel(3)
		first := ra.prompt(1)
		if got := within(t, first, "turn 1's answer"); got != StopReasonCancelled {
			t.Errorf("turn 1, received before the cancel, answered %q", got)
		}
		second := ra.prompt(2)
		if got := within(t, second, "turn 2's answer"); got != StopReasonCancelled {
			t.Errorf("turn 2, received before the cancel, answered %q", got)
		}
	})
}

// TestLateCancelsBounded requires the kept cancels to stay bounded: with no
// request in flight a cancel keeps at most itself, a cancel kept while an
// older request runs stays until that request returns, and the next cancel
// then removes it (0019-PLAN-behaviour-fixes.md, B7).
func TestLateCancelsBounded(t *testing.T) {
	t.Parallel()
	release, listing := make(chan struct{}), make(chan struct{}, 1)
	handled := make(chan struct{}, 1024)
	asc, c := permissionPair(t, agentFuncs{
		CancelFunc: func(context.Context, CancelNotification) error {
			handled <- struct{}{}
			return nil
		},
		ListSessionsFunc: func(context.Context, ListSessionsRequest) (ListSessionsResponse, error) {
			listing <- struct{}{}
			<-release
			return ListSessionsResponse{Sessions: []SessionInfo{}}, nil
		},
	}, nil)
	kept := func() int {
		asc.mu.Lock()
		defer asc.mu.Unlock()
		return len(asc.lateCancels)
	}
	cancelAll := func(prefix string, n int) {
		t.Helper()
		for i := range n {
			if err := c.Cancel(t.Context(), CancelNotification{SessionId: SessionId(fmt.Sprintf("%s%d", prefix, i))}); err != nil {
				t.Fatal(err)
			}
		}
		for range n {
			within(t, handled, "a cancel")
		}
	}

	cancelAll("idle-", 500)
	if n := kept(); n > 1 {
		t.Fatalf("%d cancels kept with no request in flight, want at most 1", n)
	}

	done := make(chan error, 1)
	go func() {
		_, err := c.ListSessions(t.Context(), ListSessionsRequest{})
		done <- err
	}()
	within(t, listing, "the long request")
	cancelAll("busy-", 100)
	if n := kept(); n != 100 {
		t.Fatalf("%d cancels kept while an older request runs, want 100", n)
	}
	close(release)
	if err := within(t, done, "the long request's answer"); err != nil {
		t.Fatal(err)
	}
	// The request's receipt is removed after its answer is written.
	for deadline := time.Now().Add(2 * time.Second); ; {
		asc.conn.mu.Lock()
		running := len(asc.conn.inflightReceipts)
		asc.conn.mu.Unlock()
		if running == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the long request stayed in flight")
		}
		time.Sleep(time.Millisecond)
	}
	cancelAll("after-", 1)
	if n := kept(); n != 1 {
		t.Fatalf("%d cancels kept after the request returned, want 1", n)
	}
}

// overlapAgent is an agent whose turns run until cancelled, connected to a
// client under the prompt-overlap policy opts give. Its handler sends each
// turn's context on started before it waits, and its cause on causes once
// cancelled; it then returns, or, when hold is not nil, waits for hold to close.
type overlapAgent struct {
	c       *ClientSideConnection
	calls   atomic.Int32
	started chan context.Context
	causes  chan error
}

func newOverlapAgent(t *testing.T, hold <-chan struct{}, opts ...ConnectionOption) *overlapAgent {
	t.Helper()
	oa := &overlapAgent{started: make(chan context.Context, 4), causes: make(chan error, 4)}
	oa.c = promptPair(t, agentFuncs{PromptFunc: func(ctx context.Context, _ PromptRequest) (PromptResponse, error) {
		oa.calls.Add(1)
		oa.started <- ctx
		<-ctx.Done()
		oa.causes <- context.Cause(ctx)
		if hold != nil {
			<-hold
		}
		return PromptResponse{}, ctx.Err()
	}}, opts...)
	return oa
}

// TestPromptOverlapRefuse requires a prompt for a session whose turn is
// running to be answered -32603 at once under PromptOverlapRefuse, without
// calling the handler or cancelling the running turn, and a prompt for another
// session to run (0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md, step 3).
func TestPromptOverlapRefuse(t *testing.T) {
	t.Parallel()
	oa := newOverlapAgent(t, nil, WithPromptOverlapPolicy(PromptOverlapRefuse))
	first := prompt(t, oa.c, "s")
	running := within(t, oa.started, "the running turn")
	refusing, stop := context.WithTimeout(t.Context(), 10*time.Second)
	defer stop()
	_, err := oa.c.Prompt(refusing, PromptRequest{SessionId: "s"})
	var re *RequestError
	if !errors.As(err, &re) || re.Code != -32603 {
		t.Fatalf("second prompt: %v, want -32603", err)
	}
	if data, _ := re.Data.(map[string]any); data["error"] != "a prompt turn is already running for this session" || data["sessionId"] != "s" {
		t.Errorf("refusal data %v, want the error and sessionId s", re.Data)
	}
	if n := oa.calls.Load(); n != 1 {
		t.Errorf("handler called %d times, want once", n)
	}
	if running.Err() != nil {
		t.Errorf("the running turn was cancelled: %v", context.Cause(running))
	}
	other := prompt(t, oa.c, "other")
	if ctx := within(t, oa.started, "another session's turn"); ctx.Err() != nil {
		t.Errorf("another session's turn was cancelled: %v", context.Cause(ctx))
	}
	for _, s := range []SessionId{"s", "other"} {
		if err := oa.c.Cancel(t.Context(), CancelNotification{SessionId: s}); err != nil {
			t.Fatal(err)
		}
	}
	for _, done := range []<-chan PromptResponse{first, other} {
		if resp := within(t, done, "a turn's answer"); resp.StopReason != StopReasonCancelled {
			t.Errorf("stopReason %q, want cancelled", resp.StopReason)
		}
	}
}

// TestPromptOverlapRefuseAfterCancel requires a prompt sent after
// session/cancel to run under PromptOverlapRefuse, though the cancelled turn's
// handler has not returned (0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md,
// step 3).
func TestPromptOverlapRefuseAfterCancel(t *testing.T) {
	t.Parallel()
	hold := make(chan struct{})
	oa := newOverlapAgent(t, hold, WithPromptOverlapPolicy(PromptOverlapRefuse))
	first := prompt(t, oa.c, "s")
	within(t, oa.started, "the first turn")
	if err := oa.c.Cancel(t.Context(), CancelNotification{SessionId: "s"}); err != nil {
		t.Fatal(err)
	}
	within(t, oa.causes, "the first turn's cancel")
	second := prompt(t, oa.c, "s")
	if ctx := within(t, oa.started, "the prompt sent after the cancel"); ctx.Err() != nil {
		t.Errorf("the prompt sent after the cancel was cancelled: %v", context.Cause(ctx))
	}
	close(hold)
	if resp := within(t, first, "the first answer"); resp.StopReason != StopReasonCancelled {
		t.Errorf("first turn stopReason %q, want cancelled", resp.StopReason)
	}
	if err := oa.c.Cancel(t.Context(), CancelNotification{SessionId: "s"}); err != nil {
		t.Fatal(err)
	}
	if resp := within(t, second, "the second answer"); resp.StopReason != StopReasonCancelled {
		t.Errorf("second turn stopReason %q, want cancelled", resp.StopReason)
	}
}

// TestPromptOverlapConcurrent requires every prompt for a session to reach the
// handler under PromptOverlapConcurrent, a session/cancel to cancel every turn
// received before it and no turn received after it, and a kept cancel to
// cancel a prompt received before it and stored after it
// (0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md, step 3).
func TestPromptOverlapConcurrent(t *testing.T) {
	t.Parallel()
	t.Run("over the wire", func(t *testing.T) {
		t.Parallel()
		oa := newOverlapAgent(t, nil, WithPromptOverlapPolicy(PromptOverlapConcurrent))
		first := prompt(t, oa.c, "s")
		ctx1 := within(t, oa.started, "the first turn")
		second := prompt(t, oa.c, "s")
		ctx2 := within(t, oa.started, "the second turn")
		for i, ctx := range []context.Context{ctx1, ctx2} {
			if ctx.Err() != nil {
				t.Errorf("turn %d cancelled by the other: %v", i+1, context.Cause(ctx))
			}
		}
		if err := oa.c.Cancel(t.Context(), CancelNotification{SessionId: "s"}); err != nil {
			t.Fatal(err)
		}
		for i, done := range []<-chan PromptResponse{first, second} {
			if cause := within(t, oa.causes, "a turn's cancel"); cause != ErrPromptCancelled { //nolint:errorlint // the cause itself, not one it wraps
				t.Errorf("a turn's cause %v, want ErrPromptCancelled", cause)
			}
			if resp := within(t, done, "a turn's answer"); resp.StopReason != StopReasonCancelled {
				t.Errorf("turn %d stopReason %q, want cancelled", i+1, resp.StopReason)
			}
		}
		third := prompt(t, oa.c, "s")
		if ctx := within(t, oa.started, "the prompt sent after the cancel"); ctx.Err() != nil {
			t.Errorf("the prompt sent after the cancel was cancelled: %v", context.Cause(ctx))
		}
		if err := oa.c.Cancel(t.Context(), CancelNotification{SessionId: "s"}); err != nil {
			t.Fatal(err)
		}
		within(t, third, "the third answer")
	})
	t.Run("a kept cancel", func(t *testing.T) {
		t.Parallel()
		ra := newReceiptAgent(t, WithPromptOverlapPolicy(PromptOverlapConcurrent))
		first := ra.prompt(1)
		within(t, ra.started, "turn 1")
		ra.stored(1)
		ra.cancel(3)
		second := ra.prompt(2)
		if got := within(t, first, "turn 1's answer"); got != StopReasonCancelled {
			t.Errorf("turn 1, received before the cancel, answered %q", got)
		}
		if got := within(t, second, "turn 2's answer"); got != StopReasonCancelled {
			t.Errorf("turn 2, received before the cancel and stored after it, answered %q", got)
		}
	})
}

// TestPromptOverlapDefault requires no option, PromptOverlapSupersede and a
// value the package does not define all to supersede the running turn
// (0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md, step 3).
func TestPromptOverlapDefault(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		opts []ConnectionOption
	}{
		{"no option", nil},
		{"supersede", []ConnectionOption{WithPromptOverlapPolicy(PromptOverlapSupersede)}},
		{"undefined", []ConnectionOption{WithPromptOverlapPolicy(PromptOverlapPolicy(99))}},
		{"negative", []ConnectionOption{WithPromptOverlapPolicy(PromptOverlapPolicy(-1))}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			oa := newOverlapAgent(t, nil, tt.opts...)
			first := prompt(t, oa.c, "s")
			within(t, oa.started, "the first turn")
			second := prompt(t, oa.c, "s")
			within(t, oa.started, "the second turn")
			if cause := within(t, oa.causes, "the first turn's cancel"); !errors.Is(cause, ErrPromptCancelled) ||
				!strings.Contains(cause.Error(), "superseded") {
				t.Errorf("first turn's cause %v, want one wrapping ErrPromptCancelled that says superseded", cause)
			}
			if resp := within(t, first, "the first answer"); resp.StopReason != StopReasonCancelled {
				t.Errorf("first turn stopReason %q, want cancelled", resp.StopReason)
			}
			if err := oa.c.Cancel(t.Context(), CancelNotification{SessionId: "s"}); err != nil {
				t.Fatal(err)
			}
			within(t, second, "the second answer")
		})
	}
}

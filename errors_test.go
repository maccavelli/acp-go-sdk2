package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"
)

func TestToReqErr_ContextCanceledMapsToRequestCancelled(t *testing.T) {
	wrapped := errors.Join(context.Canceled, errors.New("extra context"))
	re := toReqErr(wrapped)
	if re == nil {
		t.Fatal("expected request error")
	}
	if re.Code != -32800 {
		t.Fatalf("expected code -32800, got %d", re.Code)
	}
}

func TestToReqErr_WrappedRequestErrorKeepsCode(t *testing.T) {
	inner := NewInvalidParams(map[string]any{"error": "missing"})
	wrapped := fmt.Errorf("session: %w", inner)
	re := toReqErr(wrapped)
	if re == nil {
		t.Fatal("expected request error")
	}
	if re.Code != -32602 {
		t.Fatalf("expected code -32602, got %d", re.Code)
	}
	if re != inner {
		t.Fatal("expected the wrapped request error")
	}
}

func TestToReqErr_DeadlineExceededMapsToInternalError(t *testing.T) {
	re := toReqErr(context.DeadlineExceeded)
	if re == nil {
		t.Fatal("expected request error")
	}
	if re.Code != -32603 {
		t.Fatalf("expected code -32603, got %d", re.Code)
	}
}

// TestRemediationErrors requires the text of the errors 0017 adds, that each
// is found through a wrap, and the wire form of NewResourceNotFound
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 3).
func TestRemediationErrors(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		err  error
		want string
	}{
		{&CapabilityError{Method: "session/load", Capability: "loadSession"}, "session/load: capability loadSession not advertised"},
		{&CapabilityError{Method: "session/new", Capability: "sessionCapabilities.additionalDirectories", Field: "additionalDirectories"},
			"session/new: capability sessionCapabilities.additionalDirectories not advertised: additionalDirectories"},
		{&CapabilityError{Method: "session/prompt", Capability: "promptCapabilities.image", Field: "prompt[1]", Variant: "image"},
			"session/prompt: capability promptCapabilities.image not advertised: prompt[1]"},
		{&CapabilityError{Method: "session/update", Capability: "session.notices", Variant: "notice"},
			"session/update: capability session.notices not advertised: variant notice"},
		{&ProtocolVersionError{Requested: 2}, "acp: protocol version 2 is not supported"},
		{&ProtocolVersionError{Requested: 1, Answered: 2}, "acp: agent answered protocol version 2 to a request for 1"},
		{ErrPromptCancelled, "acp: prompt turn cancelled"},
		{ErrNotInitialized, "acp: initialize has not completed"},
	} {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("%#v: Error() = %q, want %q", tt.err, got, tt.want)
		}
	}

	if err := fmt.Errorf("%w: superseded by a new prompt", ErrPromptCancelled); !errors.Is(err, ErrPromptCancelled) {
		t.Errorf("errors.Is(%v, ErrPromptCancelled) = false", err)
	}
	if err := fmt.Errorf("session/new: %w", ErrNotInitialized); !errors.Is(err, ErrNotInitialized) {
		t.Errorf("errors.Is(%v, ErrNotInitialized) = false", err)
	}
	var ce *CapabilityError
	if err := fmt.Errorf("send: %w", &CapabilityError{Method: "logout", Capability: "auth.logout"}); !errors.As(err, &ce) || ce.Method != "logout" {
		t.Errorf("errors.As(%v, *CapabilityError) = %v", err, ce)
	}
	var pe *ProtocolVersionError
	if err := fmt.Errorf("initialize: %w", &ProtocolVersionError{Requested: 1, Answered: 2}); !errors.As(err, &pe) || pe.Answered != 2 {
		t.Errorf("errors.As(%v, *ProtocolVersionError) = %v", err, pe)
	}

	b, err := json.Marshal(NewResourceNotFound("file:///a"))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"code":-32002,"message":"Resource not found","data":{"uri":"file:///a"}}`; string(b) != want {
		t.Errorf("NewResourceNotFound encodes as %s, want %s", b, want)
	}
}

// TestToReqErrTypedNil requires a nil *RequestError held in a non-nil error,
// returned directly or by a handler, to be an internal error, never success
// (0019-PLAN-behaviour-fixes.md, B8).
func TestToReqErrTypedNil(t *testing.T) {
	t.Parallel()
	var nilRE *RequestError
	var err error = nilRE
	if got := toReqErr(err); got == nil || got.Code != -32603 {
		t.Fatalf("toReqErr(typed nil) = %v, want -32603", got)
	}

	asc := NewAgentSideConnection(agentFuncs{NewSessionFunc: func(context.Context, NewSessionRequest) (NewSessionResponse, error) {
		var none *RequestError
		return NewSessionResponse{SessionId: "s"}, none
	}}, io.Discard, idleReader(t))
	negotiate(t, asc, allAdvertised())
	cwd, err := json.Marshal(testAbsPath())
	if err != nil {
		t.Fatal(err)
	}
	result, rerr := asc.handle(t.Context(), AgentMethodSessionNew, json.RawMessage(`{"cwd":`+string(cwd)+`,"mcpServers":[]}`))
	if rerr == nil || rerr.Code != -32603 {
		t.Fatalf("a handler's typed-nil *RequestError answered result %v, error %v; want -32603", result, rerr)
	}
}

// TestLocalErrorsUnwrap requires an error the connection makes for a local
// failure to keep that failure as its cause, with its code unchanged: a
// cancelled context, a deadline, a disconnect while waiting
// (0019-PLAN-api-decisions.md, B19).
func TestLocalErrorsUnwrap(t *testing.T) {
	t.Parallel()
	c := NewConnection(nil, io.Discard, idleReader(t))

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := SendRequest[json.RawMessage](c, cancelled, "_test/r", map[string]any{})
	checkLocal(t, "a cancelled request", err, context.Canceled, -32800)
	err = c.SendNotification(cancelled, "_test/n", map[string]any{})
	checkLocal(t, "a cancelled notification", err, context.Canceled, -32603)

	expired, stop := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer stop()
	_, err = SendRequest[json.RawMessage](c, expired, "_test/r", map[string]any{})
	checkLocal(t, "a request past its deadline", err, context.DeadlineExceeded, -32603)

	r, w := io.Pipe()
	d := NewConnection(nil, io.Discard, r)
	done := make(chan error, 1)
	go func() {
		_, err := SendRequest[json.RawMessage](d, t.Context(), "_test/r", map[string]any{})
		done <- err
	}()
	waitForPendingRequests(t, d, 1, 2*time.Second)
	_ = w.Close()
	checkLocal(t, "a request whose peer disconnected", within(t, done, "the disconnect"), ErrPeerDisconnected, -32603)
}

// checkLocal requires err to be a *RequestError with code that wraps cause.
func checkLocal(t *testing.T, what string, err, cause error, code int) {
	t.Helper()
	re, ok := errors.AsType[*RequestError](err)
	if !ok || re.Code != code {
		t.Errorf("%s: %v, want a *RequestError with code %d", what, err, code)
	}
	if !errors.Is(err, cause) {
		t.Errorf("%s: %v does not wrap %v", what, err, cause)
	}
}

// TestPeerErrorsWrapNothing requires an error a peer answered to wrap no
// local cause, even with the code a local cancel has
// (0019-PLAN-api-decisions.md, B19).
func TestPeerErrorsWrapNothing(t *testing.T) {
	t.Parallel()
	var c *Connection
	p := rawWire(t, func(w io.Writer, r io.Reader) { c = NewConnection(nil, w, r) })
	done := make(chan error, 1)
	go func() {
		_, err := SendRequest[json.RawMessage](c, t.Context(), "_test/r", map[string]any{})
		done <- err
	}()
	line := within(t, p.lines, "the request")
	var req struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		t.Fatal(err)
	}
	p.send(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,"error":{"code":-32800,"message":"Request cancelled"}}`)
	err := within(t, done, "the answer")
	re, ok := errors.AsType[*RequestError](err)
	if !ok || re.Code != -32800 {
		t.Fatalf("the peer's answer: %v, want its -32800", err)
	}
	if errors.Unwrap(err) != nil || errors.Is(err, context.Canceled) {
		t.Errorf("the peer's -32800 unwraps to %v", errors.Unwrap(err))
	}
}

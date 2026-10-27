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

// TestErrorCodesPinned requires the numbers errors.go compares to equal the
// ErrorCode constants they stand for
// (0038-PLAN-consumer-helpers-kind-and-agent-refusals.md, step 3).
func TestErrorCodesPinned(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name      string
		got, want int
	}{
		{"method not found", codeMethodNotFound, int(ErrorCodeMethodNotFound)},
		{"authentication required", codeAuthenticationRequired, int(ErrorCodeAuthenticationRequired)},
		{"request cancelled", codeRequestCancelled, int(ErrorCodeRequestCancelled)},
		{"resource not found", codeResourceNotFound, int(ErrorCodeResourceNotFound)},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, want %d", c.name, c.got, c.want)
		}
	}
}

// peerAnswer sends method over a raw connection pair whose peer answers it
// with answer, and returns the error the sender gets.
func peerAnswer(t *testing.T, answer *RequestError) error {
	t.Helper()
	toA, fromB := io.Pipe()
	toB, fromA := io.Pipe()
	a := NewConnection(func(context.Context, string, json.RawMessage) (any, *RequestError) { return nil, answer }, fromA, toA)
	b := NewConnection(nil, fromB, toB)
	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err := SendRequest[json.RawMessage](b, ctx, "_test/method", map[string]any{})
	return err
}

// TestIsMethodNotFound requires a peer's -32601 and a local method gate to
// count, wrapped or not, and a value gate not to.
func TestIsMethodNotFound(t *testing.T) {
	t.Parallel()
	peer := peerAnswer(t, NewMethodNotFound("_test/method"))
	w := &countingWriter{}
	asc, csc := sides(t, w, versionOnly())
	_, methodGate := csc.ListSessions(t.Context(), ListSessionsRequest{})
	valueGate := asc.SessionUpdate(t.Context(), SessionNotification{SessionId: "s", Update: UpdateNotice(NoticeSeverityInfo, "t")})
	if ce, ok := errors.AsType[*CapabilityError](valueGate); !ok || ce.Variant == "" {
		t.Fatalf("the value gate's error is %v, want a *CapabilityError with a Variant", valueGate)
	}
	for _, c := range []struct {
		name string
		err  error
		want bool
	}{
		{"peer's -32601", peer, true},
		{"peer's -32601, wrapped", fmt.Errorf("listing: %w", peer), true},
		{"method gate", methodGate, true},
		{"method gate, wrapped", fmt.Errorf("listing: %w", methodGate), true},
		{"value gate", valueGate, false},
		{"invalid params", NewInvalidParams(nil), false},
		{"nil", nil, false},
	} {
		if got := IsMethodNotFound(c.err); got != c.want {
			t.Errorf("%s: IsMethodNotFound(%v) = %v, want %v", c.name, c.err, got, c.want)
		}
	}
}

// TestIsResourceNotFound requires a peer's -32002, wrapped or not, and
// nothing else (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 3).
func TestIsResourceNotFound(t *testing.T) {
	t.Parallel()
	peer := peerAnswer(t, NewResourceNotFound("/work/missing.txt"))
	for _, c := range []struct {
		name string
		err  error
		want bool
	}{
		{"peer's -32002", peer, true},
		{"peer's -32002, wrapped", fmt.Errorf("reading: %w", peer), true},
		{"local", NewResourceNotFound("/work/missing.txt"), true},
		{"method not found", NewMethodNotFound("fs/read_text_file"), false},
		{"internal error", NewInternalError(nil), false},
		{"nil", nil, false},
	} {
		if got := IsResourceNotFound(c.err); got != c.want {
			t.Errorf("%s: IsResourceNotFound(%v) = %v, want %v", c.name, c.err, got, c.want)
		}
	}
}

// TestIsAuthRequired requires -32000, wrapped or not, and nothing else.
func TestIsAuthRequired(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		err  error
		want bool
	}{
		{"-32000", NewAuthRequired(nil), true},
		{"-32000, wrapped", fmt.Errorf("prompt: %w", NewAuthRequired(nil)), true},
		{"peer's -32000", peerAnswer(t, NewAuthRequired(nil)), true},
		{"-32603", NewInternalError(nil), false},
		{"nil", nil, false},
	} {
		if got := IsAuthRequired(c.err); got != c.want {
			t.Errorf("%s: IsAuthRequired(%v) = %v, want %v", c.name, c.err, got, c.want)
		}
	}
}

// TestIsCancelled requires context.Canceled, ErrPromptCancelled and -32800,
// wrapped or not, and not a deadline.
func TestIsCancelled(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		err  error
		want bool
	}{
		{"context.Canceled", context.Canceled, true},
		{"context.Canceled, wrapped", fmt.Errorf("prompt: %w", context.Canceled), true},
		{"ErrPromptCancelled", ErrPromptCancelled, true},
		{"-32800", NewRequestCancelled(nil), true},
		{"-32800, wrapped", fmt.Errorf("prompt: %w", NewRequestCancelled(nil)), true},
		{"context.DeadlineExceeded", context.DeadlineExceeded, false},
		{"-32603", NewInternalError(nil), false},
		{"nil", nil, false},
	} {
		if got := IsCancelled(c.err); got != c.want {
			t.Errorf("%s: IsCancelled(%v) = %v, want %v", c.name, c.err, got, c.want)
		}
	}
}

// TestReason requires data.error when it is a non-empty string, and the
// message otherwise.
func TestReason(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		e    *RequestError
		want string
	}{
		{"data.error", &RequestError{Code: -32602, Message: "Invalid params", Data: map[string]any{"error": "cwd must be absolute"}}, "cwd must be absolute"},
		{"empty data.error", &RequestError{Code: -32602, Message: "Invalid params", Data: map[string]any{"error": ""}}, "Invalid params"},
		{"data.error a number", &RequestError{Code: -32602, Message: "Invalid params", Data: map[string]any{"error": 1.0}}, "Invalid params"},
		{"no data.error", &RequestError{Code: -32601, Message: "Method not found", Data: map[string]any{"method": "x"}}, "Method not found"},
		{"nil data", &RequestError{Code: -32603, Message: "Internal error"}, "Internal error"},
		{"nil error", nil, ""},
	} {
		if got := c.e.Reason(); got != c.want {
			t.Errorf("%s: Reason() = %q, want %q", c.name, got, c.want)
		}
	}
}

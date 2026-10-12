package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
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

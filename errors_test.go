package acp

import (
	"context"
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

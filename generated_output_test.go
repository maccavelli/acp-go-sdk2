package acp

import (
	"encoding/json"
	"strings"
	"testing"
)

// These tests cover what the generator emits beyond the populated goldens.

func TestUnsetUnionFailsByName(t *testing.T) {
	t.Parallel()
	_, err := json.Marshal(RequestPermissionResponse{})
	if err == nil {
		t.Fatal("encoded a RequestPermissionResponse with no outcome set")
	}
	for _, want := range []string{"RequestPermissionOutcome", "no variant set"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

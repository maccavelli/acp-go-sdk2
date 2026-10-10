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

func TestNilRequiredArrayEncodesEmpty(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(NewSessionRequest{Cwd: "/home/<user>/project"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"mcpServers":[]`) {
		t.Errorf("got %s, want it to contain \"mcpServers\":[]", b)
	}
	var got NewSessionRequest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("decoded %s fails Validate: %v", b, err)
	}
}

package acp

import (
	"encoding/json"
	"strings"
	"testing"
)

// The schema requires message; decoding checks its presence, and Validate
// checks the union has one variant set.
func TestCreateElicitationRequest_MessageIsRequired(t *testing.T) {
	if err := (&CreateElicitationRequest{}).Validate(); err == nil {
		t.Fatal("a request with no variant set validated")
	}
	var r CreateElicitationRequest
	if err := json.Unmarshal([]byte(`{"mode":"form","requestedSchema":{}}`), &r); err == nil || !strings.Contains(err.Error(), `missing required property "message"`) {
		t.Fatalf("form without message decoded: %v", err)
	}
	ok := &CreateElicitationRequest{Form: &CreateElicitationForm{Message: "need a name", Mode: "form"}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
}

package acp

import "testing"

func TestCreateElicitationRequest_ValidateRequiresMessage(t *testing.T) {
	if err := (&CreateElicitationRequest{}).Validate(); err == nil {
		t.Fatal("expected message is required")
	}
	missing := &CreateElicitationRequest{Form: &CreateElicitationForm{Mode: "form"}}
	if err := missing.Validate(); err == nil {
		t.Fatal("expected message is required")
	}
	ok := &CreateElicitationRequest{Form: &CreateElicitationForm{Message: "need a name", Mode: "form"}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
}

package acp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// These tests cover how the generated types follow the schema's keywords: see
// docs/decisions/0012-PLAN-schema-fidelity-framework.md.

// TestIntegerFormatsBoundDecoding requires an integer field to reject a value
// outside the range of its declared format.
func TestIntegerFormatsBoundDecoding(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in string
		into     any
	}{
		{"uint64 outputByteLimit, negative", `{"sessionId":"s","command":"c","outputByteLimit":-1}`, &CreateTerminalRequest{}},
		{"uint32 line, 2^32", `{"sessionId":"s","path":"p","line":4294967296}`, &ReadTextFileRequest{}},
		{"uint16 protocolVersion, 65536", `{"protocolVersion":65536}`, &InitializeRequest{}},
		{"int64 size, 2^63", `{"name":"n","uri":"u","size":9223372036854775808}`, &ResourceLink{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := json.Unmarshal([]byte(tt.in), tt.into)
			if err == nil || !strings.Contains(err.Error(), "cannot unmarshal number") {
				t.Errorf("decoded %s: %v", tt.in, err)
			}
		})
	}
	var r ResourceLink
	if err := json.Unmarshal([]byte(`{"name":"n","uri":"u","size":9007199254740993}`), &r); err != nil || r.Size == nil || *r.Size != 9007199254740993 {
		t.Errorf("size 2^53+1: %v %v", err, r.Size)
	}
}

// TestTypedAdditionalProperties requires an object whose additionalProperties
// names a schema to be a map of that type.
func TestTypedAdditionalProperties(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		field any
		want  string
	}{
		{"AuthMethodTerminalInline.Env", AuthMethodTerminalInline{}.Env, "map[string]string"},
		{"AuthMethodTerminal.Env", AuthMethodTerminal{}.Env, "map[string]string"},
		{"CreateElicitationAccept.Content", CreateElicitationAccept{}.Content, "map[string]acp.ElicitationContentValue"},
		{"ElicitationAcceptAction.Content", ElicitationAcceptAction{}.Content, "map[string]acp.ElicitationContentValue"},
		{"ElicitationSchema.Properties", ElicitationSchema{}.Properties, "map[string]acp.ElicitationPropertySchema"},
		{"UnstableSetProviderRequest.Headers", UnstableSetProviderRequest{}.Headers, "map[string]string"},
	} {
		if got := reflect.TypeOf(tt.field).String(); got != tt.want {
			t.Errorf("%s is %s, want %s", tt.name, got, tt.want)
		}
	}
	var ok AuthMethodTerminal
	if err := json.Unmarshal([]byte(`{"id":"i","name":"n","env":{"A":"1"}}`), &ok); err != nil {
		t.Fatal(err)
	}
	var bad AuthMethodTerminal
	if err := json.Unmarshal([]byte(`{"id":"i","name":"n","env":{"A":1}}`), &bad); err == nil {
		t.Error("a number in env decoded")
	}
}

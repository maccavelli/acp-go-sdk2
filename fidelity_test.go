package acp

import (
	"encoding/json"
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

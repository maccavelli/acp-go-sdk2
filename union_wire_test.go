package acp

import (
	"encoding/json"
	"strings"
	"testing"
)

// These tests cover how the generated unions read and write the wire: see
// docs/decisions/0011-PLAN-wire-fixes.md.

// TestUnionDecodeErrorsNameTheirPlace requires a union decode failure to name
// the union and, for a variant, the variant and the underlying cause.
func TestUnionDecodeErrorsNameTheirPlace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		into any
		want []string
	}{
		{"known variant with a bad payload", `{"type":"text","text":5}`, &ContentBlock{}, []string{"ContentBlock", `type "text"`, "cannot unmarshal number"}},
		{"variant found by its keys, with a bad payload", `{"uri":"u","text":5}`, &EmbeddedResourceResource{},
			[]string{"EmbeddedResourceResource: variant TextResourceContents", "cannot unmarshal number"}},
		{"no variant matches", `true`, &RequestId{}, []string{"RequestId: no variant matches"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := json.Unmarshal([]byte(tt.in), tt.into)
			if err == nil {
				t.Fatalf("decoded %s", tt.in)
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}

package acp

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestUnionVariantMissingKey requires a union that decodes its chosen variant
// from the keys it holds to give, for a variant without a required key, the
// error decoding the variant itself gives, under the union's prefix, as before
// the variant shared the union's keys (0019-PLAN-idioms-performance.md, D2).
// Each message is the one the generated code gave before that change. Only a
// discriminated union reaches the variant with a required key missing: the
// other paths choose a variant by its required keys.
func TestUnionVariantMissingKey(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		union   any
		variant any
		payload string
		want    string
	}{
		{"SessionUpdate", new(SessionUpdate), new(SessionUpdateAgentMessageChunk),
			`{"sessionUpdate":"agent_message_chunk"}`,
			`SessionUpdate: sessionUpdate "agent_message_chunk": SessionUpdateAgentMessageChunk: missing required property "content"`},
		{"ContentBlock", new(ContentBlock), new(ContentBlockText),
			`{"type":"text"}`,
			`ContentBlock: type "text": ContentBlockText: missing required property "text"`},
		{"CreateElicitationRequest", new(CreateElicitationRequest), new(CreateElicitationUrl),
			`{"elicitationId":"s","mode":"url","requestId":"s","url":"s"}`,
			`CreateElicitationRequest: mode "url": CreateElicitationUrl: missing required property "message"`},
		{"McpServer", new(McpServer), new(McpServerHttpInline),
			`{"name":"s","type":"http","url":"s"}`,
			`McpServer: type "http": McpServerHttpInline: missing required property "headers"`},
	} {
		err := json.Unmarshal([]byte(tt.payload), tt.union)
		if err == nil || err.Error() != tt.want {
			t.Errorf("%s: %s decoded with %v, want %q", tt.name, tt.payload, err, tt.want)
		}
		direct := json.Unmarshal([]byte(tt.payload), tt.variant)
		if direct == nil || !strings.HasSuffix(tt.want, ": "+direct.Error()) {
			t.Errorf("%s: the variant itself gives %v; through the union %v", tt.name, direct, err)
		}
	}
}

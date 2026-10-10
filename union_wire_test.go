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

// TestUnionKeepsIntegerDigits requires an integer above 2^53 to keep every
// digit through a union, alone and nested in another union.
func TestUnionKeepsIntegerDigits(t *testing.T) {
	t.Parallel()
	const digits = "9007199254740993"
	size := 9007199254740993
	block := ContentBlock{ResourceLink: &ContentBlockResourceLink{Name: "n", Uri: "u", Size: &size}}
	for name, v := range map[string]any{
		"ContentBlock":  block,
		"SessionUpdate": SessionUpdate{AgentMessageChunk: &SessionUpdateAgentMessageChunk{Content: block}},
	} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), `"size":`+digits) {
			t.Errorf("%s: %s does not carry size %s", name, b, digits)
		}
	}
	b, err := json.Marshal(block)
	if err != nil {
		t.Fatal(err)
	}
	var back ContentBlock
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.ResourceLink == nil || back.ResourceLink.Size == nil || *back.ResourceLink.Size != size {
		t.Errorf("decoded %s, want size %d", b, size)
	}
}

// TestContentBlockKeepsMetaAndAnnotations requires a content block to encode
// its _meta and annotations, whether built in Go or decoded from the wire.
func TestContentBlockKeepsMetaAndAnnotations(t *testing.T) {
	t.Parallel()
	prio := 0.5
	built := ContentBlock{Text: &ContentBlockText{Text: "hi", Meta: map[string]any{"k": "v"}, Annotations: &Annotations{Priority: &prio}}}
	var decoded ContentBlock
	if err := json.Unmarshal([]byte(`{"type":"image","data":"AAAA","mimeType":"image/png","_meta":{"k":"v"},"annotations":{"priority":0.5}}`), &decoded); err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string]ContentBlock{"built text block": built, "decoded image block": decoded} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{`"_meta":{"k":"v"}`, `"annotations":{"priority":0.5}`} {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s: %s lacks %s", name, b, want)
			}
		}
	}
}

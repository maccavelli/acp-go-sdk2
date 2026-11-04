package acp

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestToolCallUpdateEmptyCollectionsOnWire requires a tool_call_update's
// content and locations, which the schema says it replaces, to be sent as []
// when empty, so a peer clears them: WithUpdateContent and WithUpdateLocations
// with an empty or a nil slice send [], an update without them sends neither
// key, and a ToolCallUpdate sends [] for an empty slice and nothing for nil. A
// received [] stays apart from an absent key, and a client's timeline that
// held content holds none after the update
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, S2).
func TestToolCallUpdateEmptyCollectionsOnWire(t *testing.T) {
	t.Parallel()
	encode := func(t *testing.T, v any) string {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	for _, tc := range []struct {
		name       string
		opts       []ToolCallUpdateOpt
		with, none []string
	}{
		{"empty", []ToolCallUpdateOpt{WithUpdateContent([]ToolCallContent{}), WithUpdateLocations([]ToolCallLocation{})},
			[]string{`"content":[]`, `"locations":[]`}, nil},
		{"nil", []ToolCallUpdateOpt{WithUpdateContent(nil), WithUpdateLocations(nil)},
			[]string{`"content":[]`, `"locations":[]`}, nil},
		{"no option", nil, nil, []string{`"content"`, `"locations"`}},
	} {
		got := encode(t, SessionNotification{SessionId: "s", Update: UpdateToolCall("t1", tc.opts...)})
		for _, w := range tc.with {
			if !strings.Contains(got, w) {
				t.Errorf("%s: sent %s, want %s", tc.name, got, w)
			}
		}
		for _, w := range tc.none {
			if strings.Contains(got, w) {
				t.Errorf("%s: sent %s, want no %s", tc.name, got, w)
			}
		}
	}
	if got := encode(t, ToolCallUpdate{ToolCallId: "t1", Content: []ToolCallContent{}}); !strings.Contains(got, `"content":[]`) || strings.Contains(got, `"locations"`) {
		t.Errorf("ToolCallUpdate with empty content and nil locations sent %s", got)
	}

	var cleared, untouched SessionUpdate
	if err := json.Unmarshal([]byte(`{"sessionUpdate":"tool_call_update","toolCallId":"t1","content":[]}`), &cleared); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"sessionUpdate":"tool_call_update","toolCallId":"t1"}`), &untouched); err != nil {
		t.Fatal(err)
	}
	if c := cleared.ToolCallUpdate.Content; c == nil || len(c) != 0 {
		t.Errorf("received [] decoded as %#v, want an empty slice", c)
	}
	if c := untouched.ToolCallUpdate.Content; c != nil {
		t.Errorf("an absent content decoded as %#v, want nil", c)
	}
	if got := encode(t, cleared); !strings.Contains(got, `"content":[]`) {
		t.Errorf("a received [] was sent back as %s", got)
	}

	var tl ToolCallTimeline
	tl.Apply(StartToolCall("t1", "Read", WithStartContent([]ToolCallContent{{Content: &ToolCallContentContent{Content: TextBlock("a")}}})))
	var wire SessionUpdate
	if err := json.Unmarshal([]byte(encode(t, UpdateToolCall("t1", WithUpdateContent(nil)))), &wire); err != nil {
		t.Fatal(err)
	}
	if e, _ := tl.Apply(wire); len(e.Content) != 0 {
		t.Errorf("the timeline holds %d content items after an update that cleared them", len(e.Content))
	}
}

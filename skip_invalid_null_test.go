package acp

import (
	"context"
	"encoding/json"
	"io"
	"slices"
	"testing"
)

// textContent is a tool call's content block holding text, as raw JSON.
const textContent = `{"type":"content","content":{"type":"text","text":"hi"}}`

// TestSkipInvalidItemsDropNull requires a null item of an array marked
// x-deserialize-skip-invalid-items to be dropped on decode, as an item that
// fails to decode is, so the message validates: through skipInvalid and,
// for a patch, through patchItems
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G14).
func TestSkipInvalidItemsDropNull(t *testing.T) {
	t.Parallel()
	// message is what a decoded message offers: the exported Validate.
	type message = interface{ Validate() error }
	toolCall := func(content string) string {
		return `{"sessionId":"s","update":{"sessionUpdate":"tool_call","toolCallId":"t","title":"x","content":` + content + `}}`
	}
	for _, tc := range []struct {
		name  string
		raw   string
		value func() message
		check func(v message) string
	}{
		{
			"a tool call's content", toolCall(`[null,` + textContent + `]`),
			func() message { return new(SessionNotification) },
			func(v message) string {
				if c := v.(*SessionNotification).Update.ToolCall.Content; len(c) != 1 || c[0].Content == nil {
					return "content is not the text block alone"
				}
				return ""
			},
		},
		{
			"a tool call's content of null alone", toolCall(`[null]`),
			func() message { return new(SessionNotification) },
			func(v message) string {
				if c := v.(*SessionNotification).Update.ToolCall.Content; len(c) != 0 {
					return "content is not empty"
				}
				return ""
			},
		},
		{
			"a compaction summary, a patch",
			`{"sessionId":"s","update":{"sessionUpdate":"compaction_update","compactionId":"c","status":"completed","summary":[null,{"type":"text","text":"s"}]}}`,
			func() message { return new(SessionNotification) },
			func(v message) string {
				s := v.(*SessionNotification).Update.CompactionUpdate.Summary
				if items, ok := s.Value(); !ok || len(items) != 1 || items[0].Text == nil {
					return "summary is not a patch of the text block alone"
				}
				return ""
			},
		},
		{
			"initialize's auth methods",
			`{"protocolVersion":1,"agentCapabilities":{},"authMethods":[null,{"type":"agent","id":"openai","name":"OpenAI"}]}`,
			func() message { return new(InitializeResponse) },
			func(v message) string {
				if m := v.(*InitializeResponse).AuthMethods; len(m) != 1 || m[0].ID() != "openai" {
					return "authMethods is not openai alone"
				}
				return ""
			},
		},
		{
			"a session's additional directories",
			`{"sessions":[{"sessionId":"s","cwd":"/w","additionalDirectories":[null,"/a"]}]}`,
			func() message { return new(ListSessionsResponse) },
			func(v message) string {
				if d := v.(*ListSessionsResponse).Sessions[0].AdditionalDirectories; !slices.Equal(d, []string{"/a"}) {
					return "additionalDirectories is not /a alone"
				}
				return ""
			},
		},
	} {
		v := tc.value()
		if err := json.Unmarshal([]byte(tc.raw), v); err != nil {
			t.Errorf("%s: decode: %v", tc.name, err)
			continue
		}
		if msg := tc.check(v); msg != "" {
			t.Errorf("%s: %s: %+v", tc.name, msg, v)
		}
		if err := v.Validate(); err != nil {
			t.Errorf("%s: Validate = %v, want nil", tc.name, err)
		}
	}
}

// TestSkipInvalidItemsNullReceived requires a received message with a null
// item in a skip-invalid array to be accepted, the item dropped: a
// session/update reaches the client's handler, and a session/list result
// reaches the caller, where both were refused before
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G14).
func TestSkipInvalidItemsNullReceived(t *testing.T) {
	t.Parallel()
	var got []ToolCallContent
	csc := NewClientSideConnection(&clientFuncs{SessionUpdateFunc: func(_ context.Context, n SessionNotification) error {
		got = n.Update.ToolCall.Content
		return nil
	}}, io.Discard, idleReader(t))
	negotiate(t, csc, allAdvertised())
	raw := `{"sessionId":"s","update":{"sessionUpdate":"tool_call","toolCallId":"t","title":"x","content":[null,` + textContent + `]}}`
	if _, rerr := csc.handle(t.Context(), ClientMethodSessionUpdate, json.RawMessage(raw)); rerr != nil {
		t.Errorf("session/update with a null content item: answered %v, want accepted", rerr)
	}
	if len(got) != 1 || got[0].Content == nil {
		t.Errorf("the handler's content = %+v, want the text block alone", got)
	}

	peer := rawPeer(t, `{"sessions":[{"sessionId":"s","cwd":"/w","additionalDirectories":[null,"/a"]}]}`)
	negotiate(t, peer, allAdvertised())
	resp, err := peer.ListSessions(t.Context(), ListSessionsRequest{})
	if err != nil {
		t.Fatalf("ListSessions with a null directory: %v, want accepted", err)
	}
	if d := resp.Sessions[0].AdditionalDirectories; !slices.Equal(d, []string{"/a"}) {
		t.Errorf("additionalDirectories = %q, want /a alone", d)
	}
}

package acp_test

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// entityJSON is e as JSON text, for comparing a whole entity.
func entityJSON(t *testing.T, e acp.ToolCallEntity) string {
	t.Helper()
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// toolText is the content of a text block, as a tool call carries it.
func toolText(s string) acp.ToolCallContent { return acp.ToolContent(acp.TextBlock(s)) }

// TestToolCallTimeline requires a tool_call to be the entity and each
// tool_call_update to change only the fields it carries: content and
// locations replaced, never appended, an empty collection clearing them, and
// every other field left as it was; a tool_call for an id already seen to
// replace it whole; and an update that is not a tool call's to return false
// (0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phase 11).
func TestToolCallTimeline(t *testing.T) {
	t.Parallel()
	var tl acp.ToolCallTimeline
	loc := []acp.ToolCallLocation{{Path: "/work/a.go"}}
	e, ok := tl.Apply(acp.StartToolCall("t", "Read", acp.WithStartKind(acp.ToolKindRead), acp.WithStartStatus(acp.ToolCallStatusPending),
		acp.WithStartContent([]acp.ToolCallContent{toolText("a")}), acp.WithStartLocations(loc), acp.WithStartRawInput(json.RawMessage(`{"p":1}`))))
	if !ok || e.Title != "Read" || e.Kind != acp.ToolKindRead || e.Status != acp.ToolCallStatusPending || len(e.Content) != 1 || len(e.Locations) != 1 {
		t.Fatalf("Apply(tool_call) = %+v, %v", e, ok)
	}
	before := entityJSON(t, e)
	steps := []struct {
		name   string
		update acp.ToolCallUpdateOpt
		check  func(acp.ToolCallEntity) bool
	}{
		{"status", acp.WithUpdateStatus(acp.ToolCallStatusInProgress), func(e acp.ToolCallEntity) bool { return e.Status == acp.ToolCallStatusInProgress }},
		{"title", acp.WithUpdateTitle("Read a.go"), func(e acp.ToolCallEntity) bool { return e.Title == "Read a.go" }},
		{"kind", acp.WithUpdateKind(acp.ToolKindSearch), func(e acp.ToolCallEntity) bool { return e.Kind == acp.ToolKindSearch }},
		{"content", acp.WithUpdateContent([]acp.ToolCallContent{toolText("b")}), func(e acp.ToolCallEntity) bool {
			return len(e.Content) == 1 && e.Content[0].Content != nil && e.Content[0].Content.Content.Text.Text == "b"
		}},
		{"locations", acp.WithUpdateLocations([]acp.ToolCallLocation{{Path: "/work/b.go"}, {Path: "/work/c.go"}}), func(e acp.ToolCallEntity) bool {
			return len(e.Locations) == 2 && e.Locations[0].Path == "/work/b.go"
		}},
		{"rawInput", acp.WithUpdateRawInput(json.RawMessage(`{"p":2}`)), func(e acp.ToolCallEntity) bool { return string(e.RawInput) == `{"p":2}` }},
		{"rawOutput", acp.WithUpdateRawOutput(json.RawMessage(`"done"`)), func(e acp.ToolCallEntity) bool { return string(e.RawOutput) == `"done"` }},
		{"empty content", acp.WithUpdateContent([]acp.ToolCallContent{}), func(e acp.ToolCallEntity) bool { return len(e.Content) == 0 }},
	}
	prev := before
	for _, s := range steps {
		got, ok := tl.Apply(acp.UpdateToolCall("t", s.update))
		if !ok || !s.check(got) {
			t.Errorf("%s: Apply(tool_call_update) = %+v, %v", s.name, got, ok)
		}
		// Only the field the update carries changed: put it back, and the
		// entity is the one before.
		var was, now map[string]json.RawMessage
		if err := json.Unmarshal([]byte(prev), &was); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(entityJSON(t, got)), &now); err != nil {
			t.Fatal(err)
		}
		changed := 0
		for k := range was {
			if string(was[k]) != string(now[k]) {
				changed++
			}
		}
		if changed != 1 {
			t.Errorf("%s: %d fields changed, want 1: %s, then %s", s.name, changed, prev, entityJSON(t, got))
		}
		prev = entityJSON(t, got)
	}
	got, _ := tl.Apply(acp.UpdateToolCall("t"))
	if entityJSON(t, got) != prev {
		t.Errorf("an update carrying nothing changed the call: %s, then %s", prev, entityJSON(t, got))
	}
	// A decoded null raw value arrives absent; one built in Go is null.
	got, _ = tl.Apply(acp.UpdateToolCall("t", acp.WithUpdateRawInput(json.RawMessage("null")), acp.WithUpdateRawOutput(json.RawMessage(" null "))))
	if entityJSON(t, got) != prev {
		t.Errorf("a null rawInput and rawOutput changed the call: %s, then %s", prev, entityJSON(t, got))
	}
	again, _ := tl.Apply(acp.StartToolCall("t", "Again"))
	if again.Title != "Again" || again.Content != nil || again.Locations != nil || again.RawInput != nil || again.Kind != "" {
		t.Errorf("a second tool_call = %+v, want it to replace the call whole", again)
	}
	if e, ok := tl.Apply(acp.UpdateAgentMessageText("x")); ok || e.Id != "" {
		t.Errorf("Apply(agent_message_chunk) = %+v, %v; want the zero entity and false", e, ok)
	}
}

// TestToolCallTimelineUnseen requires an update for an id never seen to start
// the call pending, its other fields from the update, as question 4 decided.
func TestToolCallTimelineUnseen(t *testing.T) {
	t.Parallel()
	var tl acp.ToolCallTimeline
	e, ok := tl.Apply(acp.UpdateToolCall("u", acp.WithUpdateTitle("Unseen")))
	if !ok || e.Id != "u" || e.Status != acp.ToolCallStatusPending || e.Title != "Unseen" {
		t.Errorf("Apply(an update for an unseen id) = %+v, %v; want it started pending, titled", e, ok)
	}
	if got, ok := tl.Get("u"); !ok || got.Status != acp.ToolCallStatusPending {
		t.Errorf("Get(u) = %+v, %v", got, ok)
	}
	if _, ok := tl.Get("never"); ok {
		t.Error("Get of an id never applied = true")
	}
}

// TestToolCallTimelineOpen requires Open to list the calls of any status but
// completed and failed, an unknown one included, in the order first seen.
func TestToolCallTimelineOpen(t *testing.T) {
	t.Parallel()
	var tl acp.ToolCallTimeline
	for _, u := range []acp.SessionUpdate{
		acp.StartToolCall("a", "A", acp.WithStartStatus(acp.ToolCallStatusPending)),
		acp.StartToolCall("b", "B", acp.WithStartStatus(acp.ToolCallStatusInProgress)),
		acp.StartToolCall("c", "C"),
		acp.UpdateToolCall("d", acp.WithUpdateStatus("waiting")),
		acp.UpdateToolCall("b", acp.WithUpdateStatus(acp.ToolCallStatusCompleted)),
		acp.UpdateToolCall("c", acp.WithUpdateStatus(acp.ToolCallStatusFailed)),
	} {
		tl.Apply(u)
	}
	if got, want := tl.Open(), []acp.ToolCallId{"a", "d"}; !slices.Equal(got, want) {
		t.Errorf("Open = %q, want %q", got, want)
	}
}

// TestToolCallTimelineCopies requires the entities Apply and Get return to
// share nothing with the timeline's.
func TestToolCallTimelineCopies(t *testing.T) {
	t.Parallel()
	var tl acp.ToolCallTimeline
	name := "tool"
	u := acp.StartToolCall("t", "T", acp.WithStartContent([]acp.ToolCallContent{toolText("a")}),
		acp.WithStartLocations([]acp.ToolCallLocation{{Path: "/work/a.go"}}), acp.WithStartRawInput(json.RawMessage(`{"p":1}`)))
	u.ToolCall.Name = &name
	e, _ := tl.Apply(u)
	e.Content[0] = toolText("changed")
	e.Locations[0].Path = "/changed"
	e.RawInput[2] = 'q'
	*e.Name = "changed"
	name = "changed too"
	u.ToolCall.Content[0] = toolText("changed")
	got, _ := tl.Get("t")
	if got.Content[0].Content.Content.Text.Text != "a" || got.Locations[0].Path != "/work/a.go" || string(got.RawInput) != `{"p":1}` ||
		*got.Name != "tool" {
		t.Errorf("the timeline shares what Apply was given or returned: %s", entityJSON(t, got))
	}
}

// TestToolCallTimelineReplay applies testdata/toolcalls/lifecycle.jsonl, one
// session/update params object a line, as a live stream and again into a
// fresh timeline, as session/load replays it, and requires the same entities
// and the expected ones: a null title, rawInput and name left as they were,
// an empty locations clearing them, an unrelated update ignored, and an
// unseen id started pending.
func TestToolCallTimelineReplay(t *testing.T) {
	t.Parallel()
	want := map[acp.ToolCallId]string{
		"t1": `{"Id":"t1","Title":"Read a.go","Kind":"read","Status":"completed","Name":null,` +
			`"Content":[{"content":{"text":"whole","type":"text"},"type":"content"}],"Locations":[],` +
			`"RawInput":{"path":"/work/a.go"},"RawOutput":{"ok":true}}`,
		"t2": `{"Id":"t2","Title":"Unseen","Kind":"","Status":"in_progress","Name":null,"Content":null,"Locations":null,"RawInput":null,"RawOutput":null}`,
		"t3": `{"Id":"t3","Title":"Edit","Kind":"edit","Status":"failed","Name":"edit_file","Content":null,"Locations":null,"RawInput":null,"RawOutput":null}`,
	}
	updates := readUpdates(t, filepath.Join("testdata", "toolcalls", "lifecycle.jsonl"))
	var live, replay acp.ToolCallTimeline
	for _, u := range updates {
		live.Apply(u)
	}
	for _, u := range updates {
		replay.Apply(u)
	}
	if got, want := live.Open(), []acp.ToolCallId{"t2"}; !slices.Equal(got, want) || !slices.Equal(replay.Open(), want) {
		t.Errorf("Open: live %q, replay %q, want %q", got, replay.Open(), want)
	}
	for id, w := range want {
		l, _ := live.Get(id)
		r, _ := replay.Get(id)
		if lj, rj := entityJSON(t, l), entityJSON(t, r); lj != w || rj != w {
			t.Errorf("%s:\n live   %s\n replay %s\n want   %s", id, lj, rj, w)
		}
	}
}

package acp_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// compactionUpdate is a compaction_update for id with status, its patches
// set by set.
func compactionUpdate(id acp.CompactionId, status acp.CompactionStatus, set func(*acp.SessionCompactionUpdate)) acp.SessionUpdate {
	u := acp.UpdateCompaction(id, status)
	if set != nil {
		set(u.CompactionUpdate)
	}
	return u
}

// summaryText is the text of each block of an entity's summary.
func summaryText(t *testing.T, e acp.CompactionEntity) []string {
	t.Helper()
	var out []string
	for _, b := range e.Summary {
		if b.Text == nil {
			t.Fatalf("summary block %+v is not text", b)
		}
		out = append(out, b.Text.Text)
	}
	return out
}

// TestCompactionTimeline applies sequences of updates and requires the
// entity the last one leaves, by the protocol's rules
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 3).
func TestCompactionTimeline(t *testing.T) {
	t.Parallel()
	text := func(s string) acp.ContentBlock { return acp.TextBlock(s) }
	for _, tt := range []struct {
		name    string
		updates []acp.SessionUpdate
		want    acp.CompactionEntity
		summary []string
	}{
		{"first seen, then applied", []acp.SessionUpdate{compactionUpdate("a", acp.CompactionStatusCompleted, nil)},
			acp.CompactionEntity{Id: "a", Status: acp.CompactionStatusCompleted, Seen: true}, nil},
		{"each patch set", []acp.SessionUpdate{compactionUpdate("a", acp.CompactionStatusInProgress, func(u *acp.SessionCompactionUpdate) {
			u.Error = acp.PatchValue("e")
			u.Meta = acp.PatchValue(map[string]any{"k": "v"})
			u.Summary = acp.PatchValue([]acp.ContentBlock{text("s")})
		})}, acp.CompactionEntity{Id: "a", Status: acp.CompactionStatusInProgress, Error: "e", Meta: map[string]any{"k": "v"}, Seen: true}, []string{"s"}},
		{"each patch omitted keeps it", []acp.SessionUpdate{
			compactionUpdate("a", acp.CompactionStatusInProgress, func(u *acp.SessionCompactionUpdate) {
				u.Error = acp.PatchValue("e")
				u.Meta = acp.PatchValue(map[string]any{"k": "v"})
				u.Summary = acp.PatchValue([]acp.ContentBlock{text("s")})
			}),
			compactionUpdate("a", acp.CompactionStatusFailed, nil),
		}, acp.CompactionEntity{Id: "a", Status: acp.CompactionStatusFailed, Error: "e", Meta: map[string]any{"k": "v"}, Seen: true}, []string{"s"}},
		{"each patch null clears it", []acp.SessionUpdate{
			compactionUpdate("a", acp.CompactionStatusInProgress, func(u *acp.SessionCompactionUpdate) {
				u.Error = acp.PatchValue("e")
				u.Meta = acp.PatchValue(map[string]any{"k": "v"})
				u.Summary = acp.PatchValue([]acp.ContentBlock{text("s")})
			}),
			compactionUpdate("a", acp.CompactionStatusInProgress, func(u *acp.SessionCompactionUpdate) {
				u.Error = acp.PatchNull[string]()
				u.Meta = acp.PatchNull[map[string]any]()
				u.Summary = acp.PatchNull[[]acp.ContentBlock]()
			}),
		}, acp.CompactionEntity{Id: "a", Status: acp.CompactionStatusInProgress, Seen: true}, nil},
		{"an empty summary clears it", []acp.SessionUpdate{
			acp.UpdateCompactionSummaryText("a", "one"),
			compactionUpdate("a", acp.CompactionStatusInProgress, func(u *acp.SessionCompactionUpdate) { u.Summary = acp.PatchValue([]acp.ContentBlock{}) }),
		}, acp.CompactionEntity{Id: "a", Status: acp.CompactionStatusInProgress, Seen: true}, nil},
		{"chunks append in order", []acp.SessionUpdate{
			compactionUpdate("a", acp.CompactionStatusInProgress, nil),
			acp.UpdateCompactionSummaryText("a", "one"),
			acp.UpdateCompactionSummaryText("a", "two"),
		}, acp.CompactionEntity{Id: "a", Status: acp.CompactionStatusInProgress, Seen: true}, []string{"one", "two"}},
		{"a concrete summary replaces the chunks", []acp.SessionUpdate{
			acp.UpdateCompactionSummaryText("a", "one"),
			acp.UpdateCompactionSummaryText("a", "two"),
			compactionUpdate("a", acp.CompactionStatusCompleted, func(u *acp.SessionCompactionUpdate) {
				u.Summary = acp.PatchValue([]acp.ContentBlock{text("final")})
			}),
		}, acp.CompactionEntity{Id: "a", Status: acp.CompactionStatusCompleted, Seen: true}, []string{"final"}},
		{"a chunk first seen starts the compaction", []acp.SessionUpdate{acp.UpdateCompactionSummaryText("a", "orphan")},
			acp.CompactionEntity{Id: "a", Status: acp.CompactionStatusInProgress}, []string{"orphan"}},
		{"an unknown status is kept", []acp.SessionUpdate{compactionUpdate("a", "summarizing", nil)},
			acp.CompactionEntity{Id: "a", Status: "summarizing", Seen: true}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var tl acp.CompactionTimeline
			var last acp.CompactionEntity
			for _, u := range tt.updates {
				e, ok := tl.Apply(u)
				if !ok {
					t.Fatalf("Apply(%+v) = false", u)
				}
				last = e
			}
			if got := summaryText(t, last); !slices.Equal(got, tt.summary) {
				t.Errorf("summary %q, want %q", got, tt.summary)
			}
			last.Summary = nil
			if !reflect.DeepEqual(last, tt.want) {
				t.Errorf("entity %+v, want %+v", last, tt.want)
			}
			if got, ok := tl.Get(tt.want.Id); !ok || got.Status != tt.want.Status || got.Seen != tt.want.Seen {
				t.Errorf("Get: %+v, %v", got, ok)
			}
		})
	}
}

// TestCompactionTimelineOpen requires Open to list the compactions in
// progress or of an unknown status, in the order first seen, and no terminal
// one; and a non-compaction update to change nothing.
func TestCompactionTimelineOpen(t *testing.T) {
	t.Parallel()
	var tl acp.CompactionTimeline
	for _, u := range []acp.SessionUpdate{
		compactionUpdate("z", acp.CompactionStatusInProgress, nil),
		compactionUpdate("done", acp.CompactionStatusCompleted, nil),
		compactionUpdate("odd", "summarizing", nil),
		compactionUpdate("bad", acp.CompactionStatusFailed, nil),
		compactionUpdate("stop", acp.CompactionStatusCancelled, nil),
		acp.UpdateCompactionSummaryText("chunk", "x"),
	} {
		tl.Apply(u)
	}
	if got, want := tl.Open(), []acp.CompactionId{"z", "odd", "chunk"}; !slices.Equal(got, want) {
		t.Errorf("Open = %q, want %q", got, want)
	}
	if e, ok := tl.Apply(acp.UpdateAgentMessageText("hi")); ok || !reflect.DeepEqual(e, acp.CompactionEntity{}) {
		t.Errorf("a non-compaction update: %+v, %v", e, ok)
	}
	if _, ok := tl.Get("missing"); ok {
		t.Error("Get of an id never seen reported one")
	}
}

// TestCompactionTimelineCopies requires the entities Apply and Get return to
// share nothing with the timeline's.
func TestCompactionTimelineCopies(t *testing.T) {
	t.Parallel()
	var tl acp.CompactionTimeline
	e, _ := tl.Apply(compactionUpdate("a", acp.CompactionStatusInProgress, func(u *acp.SessionCompactionUpdate) {
		u.Meta = acp.PatchValue(map[string]any{"k": "v"})
		u.Summary = acp.PatchValue([]acp.ContentBlock{acp.TextBlock("s")})
	}))
	e.Meta["k"] = "changed"
	e.Summary[0] = acp.TextBlock("changed")
	got, _ := tl.Get("a")
	if got.Meta["k"] != "v" || summaryText(t, got)[0] != "s" {
		t.Errorf("the timeline shares what Apply returned: %+v", got)
	}
}

// TestCompactionTimelineReplay applies each fixture of testdata/compaction,
// one session/update params object a line, as a live stream and again into
// a fresh timeline, as session/load replays it, and requires the same
// entities and the expected ones.
func TestCompactionTimelineReplay(t *testing.T) {
	t.Parallel()
	want := map[string]map[acp.CompactionId]string{
		"lifecycle.jsonl": {
			"c1": `{"Id":"c1","Status":"completed","Summary":[{"text":"Final summary.","type":"text"}],"Error":"","Meta":null,"Seen":true}`,
			"c2": `{"Id":"c2","Status":"failed","Summary":null,"Error":"out of context","Meta":{"k":"v"},"Seen":true}`,
		},
		"clear-and-unknown.jsonl": {
			"c3": `{"Id":"c3","Status":"summarizing","Summary":null,"Error":"","Meta":null,"Seen":true}`,
			"c4": `{"Id":"c4","Status":"in_progress","Summary":null,"Error":"","Meta":null,"Seen":true}`,
		},
	}
	for name, entities := range want {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			updates := readUpdates(t, filepath.Join("testdata", "compaction", name))
			var live, replay acp.CompactionTimeline
			for _, u := range updates {
				live.Apply(u)
			}
			for _, u := range updates {
				replay.Apply(u)
			}
			if !slices.Equal(live.Open(), replay.Open()) {
				t.Errorf("Open: live %q, replay %q", live.Open(), replay.Open())
			}
			for id, w := range entities {
				l, _ := live.Get(id)
				r, _ := replay.Get(id)
				lb, err := json.Marshal(l)
				if err != nil {
					t.Fatal(err)
				}
				rb, err := json.Marshal(r)
				if err != nil {
					t.Fatal(err)
				}
				if string(lb) != w || string(rb) != w {
					t.Errorf("%s:\n live   %s\n replay %s\n want   %s", id, lb, rb, w)
				}
			}
		})
	}
}

// readUpdates reads the updates of a fixture, one SessionNotification a line.
func readUpdates(t *testing.T, path string) []acp.SessionUpdate {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []acp.SessionUpdate
	s := bufio.NewScanner(f)
	for s.Scan() {
		if strings.TrimSpace(s.Text()) == "" {
			continue
		}
		var n acp.SessionNotification
		if err := json.Unmarshal(s.Bytes(), &n); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		out = append(out, n.Update)
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

package acp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"
)

// stableUpdateBuilders is a constructor for each stable SessionUpdate variant,
// by its sessionUpdate value; set says whether it writes the variant's
// SessionUpdate field, which the six helpers before
// 0036-MADR-session-update-constructors-kind-and-client-refusals.md leave
// empty (its question 1).
var stableUpdateBuilders = map[string]struct {
	build func() SessionUpdate
	set   bool
}{
	"user_message_chunk":  {func() SessionUpdate { return UpdateUserMessageText("hi") }, false},
	"agent_message_chunk": {func() SessionUpdate { return UpdateAgentMessageText("hi") }, false},
	"agent_thought_chunk": {func() SessionUpdate { return UpdateAgentThoughtText("hm") }, false},
	"tool_call":           {func() SessionUpdate { return StartToolCall("c", "Read") }, false},
	"tool_call_update":    {func() SessionUpdate { return UpdateToolCall("c", WithUpdateStatus(ToolCallStatusCompleted)) }, false},
	"plan": {func() SessionUpdate {
		return UpdatePlan(PlanEntry{Content: "a", Priority: PlanEntryPriorityLow, Status: PlanEntryStatusPending})
	}, false},
	"available_commands_update": {func() SessionUpdate {
		return UpdateAvailableCommands([]AvailableCommand{{Name: "web", Description: "Search"}})
	}, true},
	"current_mode_update": {func() SessionUpdate { return UpdateCurrentMode("code") }, true},
	"config_option_update": {func() SessionUpdate {
		return UpdateConfigOptions([]SessionConfigOption{NewSessionConfigOptionBoolean("b", "B", true)})
	}, true},
	"session_info_update":      {func() SessionUpdate { return UpdateSessionTitle(PatchValue("T")) }, true},
	"usage_update":             {func() SessionUpdate { return UpdateUsage(1, 2) }, true},
	"notice":                   {func() SessionUpdate { return UpdateNotice(NoticeSeverityInfo, "t") }, true},
	"compaction_update":        {func() SessionUpdate { return UpdateCompaction("k", CompactionStatusCompleted) }, true},
	"compaction_summary_chunk": {func() SessionUpdate { return UpdateCompactionSummaryText("k", "s") }, true},
}

// stableUpdateValues returns the sessionUpdate value of each SessionUpdate
// variant in the stable schema file.
func stableUpdateValues(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("schema/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Defs map[string]struct {
			OneOf []struct {
				Properties struct {
					SessionUpdate struct {
						Const string `json:"const"`
					} `json:"sessionUpdate"`
				} `json:"properties"`
			} `json:"oneOf"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, v := range s.Defs["SessionUpdate"].OneOf {
		out = append(out, v.Properties.SessionUpdate.Const)
	}
	if len(out) == 0 {
		t.Fatal("schema/schema.json: no SessionUpdate variants")
	}
	return out
}

// TestStableSessionUpdateConstructors requires a constructor for every stable
// SessionUpdate variant, each giving a value that validates and survives a
// round trip (0036-PLAN-session-update-constructors-kind-and-client-refusals.md,
// step 2).
func TestStableSessionUpdateConstructors(t *testing.T) {
	t.Parallel()
	for _, value := range stableUpdateValues(t) {
		b, ok := stableUpdateBuilders[value]
		if !ok {
			t.Errorf("stable variant %q has no constructor", value)
			continue
		}
		u := b.build()
		if err := u.Validate(); err != nil {
			t.Errorf("%s: Validate: %v", value, err)
		}
		raw, err := json.Marshal(u)
		if err != nil {
			t.Fatalf("%s: marshal: %v", value, err)
		}
		var disc struct {
			SessionUpdate string `json:"sessionUpdate"`
		}
		if err := json.Unmarshal(raw, &disc); err != nil || disc.SessionUpdate != value {
			t.Errorf("%s: sent sessionUpdate %q (%v)", value, disc.SessionUpdate, err)
		}
		var back SessionUpdate
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatalf("%s: unmarshal: %v", value, err)
		}
		if b.set {
			if !reflect.DeepEqual(back, u) {
				t.Errorf("%s: round trip differs\n got: %#v\nwant: %#v", value, back, u)
			}
			continue
		}
		again, err := json.Marshal(back)
		if err != nil || string(again) != string(raw) {
			t.Errorf("%s: round trip sent %s, want %s (%v)", value, again, raw, err)
		}
	}
	empty := UpdateNotice(NoticeSeverityInfo, "")
	err := empty.Validate()
	if ve, ok := errors.AsType[*ValidationError](err); !ok || ve.Field != "title" {
		t.Errorf("UpdateNotice with an empty title: Validate = %v, want a *ValidationError on title", err)
	}
}

// TestGatedConstructorsRefused requires the gated constructors' updates to be
// refused, unwritten, to a client that advertised neither session.notices nor
// session.compaction.
func TestGatedConstructorsRefused(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		update     SessionUpdate
		capability string
		variant    string
	}{
		{UpdateNotice(NoticeSeverityInfo, "t"), "session.notices", "notice"},
		{UpdateCompaction("k", CompactionStatusCompleted), "session.compaction", "compaction_update"},
		{UpdateCompactionSummaryText("k", "s"), "session.compaction", "compaction_summary_chunk"},
	} {
		w := &countingWriter{}
		asc, _ := sides(t, w, versionOnly())
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		err := asc.SessionUpdate(ctx, SessionNotification{SessionId: "s", Update: c.update})
		cancel()
		ce, ok := errors.AsType[*CapabilityError](err)
		if !ok || ce.Method != ClientMethodSessionUpdate || ce.Capability != c.capability || ce.Variant != c.variant {
			t.Errorf("%s: send = %v, want *CapabilityError{%s, %s, Variant %q}", c.variant, err, ClientMethodSessionUpdate, c.capability, c.variant)
		}
		if got := w.count(); got != 0 {
			t.Errorf("%s: %d bytes written, want none", c.variant, got)
		}
	}
}

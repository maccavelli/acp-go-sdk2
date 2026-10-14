package acp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestPatch requires Patch[T] to hold absent, null, and a value, to send each
// as the key's absence, null, and the value, empty values included, and to
// read each back (0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md, step 2).
func TestPatch(t *testing.T) {
	t.Parallel()
	type holder struct {
		S Patch[string]         `json:"s,omitzero"`
		L Patch[[]int]          `json:"l,omitzero"`
		M Patch[map[string]any] `json:"m,omitzero"`
		U Patch[StateUpdate]    `json:"u,omitzero"`
	}
	running := StateUpdate{Running: &StateUpdateRunning{State: "running"}}
	for _, tt := range []struct {
		name string
		h    holder
		want string
	}{
		{"absent", holder{}, `{}`},
		{"null", holder{S: PatchNull[string](), L: PatchNull[[]int](), M: PatchNull[map[string]any](), U: PatchNull[StateUpdate]()},
			`{"s":null,"l":null,"m":null,"u":null}`},
		{"values", holder{S: PatchValue("x"), L: PatchValue([]int{1}), M: PatchValue(map[string]any{"a": true}), U: PatchValue(running)},
			`{"s":"x","l":[1],"m":{"a":true},"u":{"state":"running"}}`},
		{"empty values", holder{S: PatchValue(""), L: PatchValue([]int{}), M: PatchValue(map[string]any{})},
			`{"s":"","l":[],"m":{}}`},
	} {
		b, err := json.Marshal(tt.h)
		if err != nil || string(b) != tt.want {
			t.Errorf("%s: sent %s, %v; want %s", tt.name, b, err, tt.want)
			continue
		}
		var back holder
		if err := json.Unmarshal(b, &back); err != nil {
			t.Errorf("%s: %s did not decode: %v", tt.name, b, err)
			continue
		}
		if again, _ := json.Marshal(back); string(again) != tt.want {
			t.Errorf("%s: read back and sent again as %s, want %s", tt.name, again, tt.want)
		}
	}

	var zero Patch[string]
	if !zero.IsZero() || zero.IsNull() {
		t.Errorf("zero value: IsZero %v IsNull %v, want absent", zero.IsZero(), zero.IsNull())
	}
	if v, ok := zero.Value(); ok || v != "" {
		t.Errorf("zero value: Value() = %q, %v", v, ok)
	}
	if n := PatchNull[string](); n.IsZero() || !n.IsNull() {
		t.Errorf("PatchNull: IsZero %v IsNull %v, want null", n.IsZero(), n.IsNull())
	} else if _, ok := n.Value(); ok {
		t.Error("PatchNull holds a value")
	}
	if p := PatchValue("x"); p.IsZero() || p.IsNull() {
		t.Errorf("PatchValue: IsZero %v IsNull %v, want a value", p.IsZero(), p.IsNull())
	} else if v, ok := p.Value(); !ok || v != "x" {
		t.Errorf("PatchValue: Value() = %q, %v", v, ok)
	}
	if b, _ := json.Marshal(PatchValue([]int(nil))); string(b) != "null" {
		t.Errorf("a value holding a nil slice is sent as %s, want null, as documented", b)
	}
	x, y, null := PatchValue("x"), PatchValue("x"), PatchNull[string]()
	if x != y || x == null || null != PatchNull[string]() {
		t.Error("Patch[string] does not compare by state and value")
	}
}

// patchState is what TestPatchFields reads of a field of any Patch type.
type patchState interface {
	IsZero() bool
	IsNull() bool
}

// TestPatchFields requires each of the 24 fields of the 12 patch properties to
// read an omitted key as absent, null as null, a value as that value, and a
// malformed value as absent, and to send each back as it was read, the
// malformed one as nothing (0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md,
// step 2).
func TestPatchFields(t *testing.T) {
	t.Parallel()
	type field struct{ key, name, value string }
	text := `[{"type":"text","text":"s"}]`
	compaction := []field{{"summary", "Summary", text}, {"error", "Error", `"boom"`}, {"_meta", "Meta", `{"a":1}`}}
	info := []field{{"title", "Title", `"t"`}, {"updatedAt", "UpdatedAt", `"2026-10-14T00:00:00Z"`}}
	subagent := []field{{"title", "Title", `"t"`}, {"description", "Description", `"d"`}, {"capabilities", "Capabilities", `{}`},
		{"state", "State", `{"state":"running"}`}, {"_meta", "Meta", `{"a":1}`}}
	message := []field{{"content", "Content", text}, {"_meta", "Meta", `{"a":1}`}}
	types := []struct {
		name   string
		newOf  func() any
		base   string
		fields []field
	}{
		{"CompactionUpdate", func() any { return &CompactionUpdate{} }, `"compactionId":"c1","status":"failed"`, compaction},
		{"SessionCompactionUpdate", func() any { return &SessionCompactionUpdate{} },
			`"sessionUpdate":"compaction_update","compactionId":"c1","status":"failed"`, compaction},
		{"SessionInfoUpdate", func() any { return &SessionInfoUpdate{} }, ``, info},
		{"SessionSessionInfoUpdate", func() any { return &SessionSessionInfoUpdate{} }, `"sessionUpdate":"session_info_update"`, info},
		{"SubagentUpdate", func() any { return &SubagentUpdate{} }, `"sessionId":"child"`, subagent},
		{"SessionSubagentUpdate", func() any { return &SessionSubagentUpdate{} }, `"sessionUpdate":"subagent_update","sessionId":"child"`, subagent},
		{"SessionMessage", func() any { return &SessionMessage{} }, `"messageId":"m1"`, message},
		{"SessionUpdateSessionMessage", func() any { return &SessionUpdateSessionMessage{} }, `"sessionUpdate":"session_message","messageId":"m1"`, message},
	}
	count := 0
	for _, ty := range types {
		for _, fl := range ty.fields {
			count++
			for _, c := range []struct {
				state, wire  string
				null, absent bool
			}{
				{"omitted", "", false, true},
				{"null", "null", true, false},
				{"a value", fl.value, false, false},
				{"malformed", "5", false, true},
			} {
				obj := ty.base
				if c.wire != "" {
					if obj != "" {
						obj += ","
					}
					obj += `"` + fl.key + `":` + c.wire
				}
				in := "{" + obj + "}"
				v := ty.newOf()
				if err := json.Unmarshal([]byte(in), v); err != nil {
					t.Errorf("%s.%s %s: %s did not decode: %v", ty.name, fl.key, c.state, in, err)
					continue
				}
				p, ok := reflect.ValueOf(v).Elem().FieldByName(fl.name).Interface().(patchState)
				if !ok {
					t.Errorf("%s.%s: %s is not a Patch", ty.name, fl.key, reflect.ValueOf(v).Elem().FieldByName(fl.name).Type())
					continue
				}
				if p.IsZero() != c.absent || p.IsNull() != c.null {
					t.Errorf("%s.%s %s: IsZero %v IsNull %v", ty.name, fl.key, c.state, p.IsZero(), p.IsNull())
				}
				out, err := json.Marshal(v)
				if err != nil {
					t.Errorf("%s.%s %s: encode: %v", ty.name, fl.key, c.state, err)
					continue
				}
				var sent map[string]json.RawMessage
				if err := json.Unmarshal(out, &sent); err != nil {
					t.Fatal(err)
				}
				got, present := sent[fl.key]
				switch {
				case c.absent && present:
					t.Errorf("%s.%s %s: sent %s, want the key left out", ty.name, fl.key, c.state, out)
				case !c.absent && !present:
					t.Errorf("%s.%s %s: sent %s without the key", ty.name, fl.key, c.state, out)
				case !c.absent && !reflect.DeepEqual(decodeTree(t, got), decodeTree(t, []byte(c.wire))):
					t.Errorf("%s.%s %s: sent %s, want %s", ty.name, fl.key, c.state, got, c.wire)
				}
			}
		}
	}
	if count != 24 || len(types) != 8 {
		t.Errorf("%d fields of %d types checked, want the 24 fields of 8 types", count, len(types))
	}
}

// TestPatchFieldsValidate requires Validate to check a patch field's value,
// each element of an array included, and not a null one
// (0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md, step 2).
func TestPatchFieldsValidate(t *testing.T) {
	t.Parallel()
	u := SessionCompactionUpdate{SessionUpdate: "compaction_update", CompactionId: "c1", Status: CompactionStatusFailed,
		Summary: PatchValue([]ContentBlock{TextBlock("ok"), {}})}
	if err := u.Validate(); err == nil || !strings.Contains(err.Error(), "summary[1]") {
		t.Errorf("a summary holding an empty block: Validate = %v, want an error naming summary[1]", err)
	}
	u.Summary = PatchNull[[]ContentBlock]()
	if err := u.Validate(); err != nil {
		t.Errorf("a null summary: Validate = %v", err)
	}
	s := SubagentUpdate{SessionId: "child", State: PatchValue(StateUpdate{})}
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "state") {
		t.Errorf("a state holding no variant: Validate = %v, want an error naming state", err)
	}
	s.State = PatchNull[StateUpdate]()
	if err := s.Validate(); err != nil {
		t.Errorf("a null state: Validate = %v", err)
	}
}

package acp

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestSessionUpdateKind requires Kind to name every SessionUpdate variant as
// MarshalJSON sends it, and an Unknown payload as UnmarshalJSON reads it
// (0036-PLAN-session-update-constructors-kind-and-client-refusals.md, step 3).
func TestSessionUpdateKind(t *testing.T) {
	t.Parallel()
	t.Run("every variant", testKindEveryVariant)
	t.Run("each constructor", testKindEachConstructor)
	t.Run("unknown", testKindUnknown)
	t.Run("decoding agrees", testKindDecodingAgrees)
	t.Run("zero and order", testKindZeroAndOrder)
}

// kindUnknownCases are Unknown payloads, the Kind each gives, and whether
// UnmarshalJSON decodes the payload into Unknown.
var kindUnknownCases = []struct {
	payload string
	want    string
	decodes bool
}{
	{`{"sessionUpdate":"future_update"}`, "future_update", true},
	{`{"SessionUpdate":"folded"}`, "folded", true},
	{`{"sessionUpdate":"a","SESSIONUPDATE":"b"}`, "b", true},
	{`{"sessionUpdate":""}`, "unknown", true},
	{`{"sessionUpdate":1}`, "unknown", false},
	{`{}`, "unknown", false},
	{`[]`, "unknown", false},
	{`not json`, "unknown", false},
}

// sentKind returns the sessionUpdate u is sent with.
func sentKind(t *testing.T, u SessionUpdate) string {
	t.Helper()
	raw, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var disc struct {
		SessionUpdate string `json:"sessionUpdate"`
	}
	if err := json.Unmarshal(raw, &disc); err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
	return disc.SessionUpdate
}

func testKindEveryVariant(t *testing.T) {
	t.Parallel()
	var variants []schemaVariant
	for _, su := range schemaUnions() {
		if su.Type == "SessionUpdate" {
			variants = su.Variants
		}
	}
	if len(variants) == 0 {
		t.Fatal("schemaUnions: no SessionUpdate variants")
	}
	seen := map[string]bool{}
	for _, vr := range variants {
		var u SessionUpdate
		f := reflect.ValueOf(&u).Elem().FieldByName(vr.Field)
		if !f.IsValid() || f.Kind() != reflect.Pointer {
			t.Errorf("%s: no pointer field %s", vr.Value, vr.Field)
			continue
		}
		f.Set(reflect.New(f.Type().Elem()))
		seen[vr.Field] = true
		if got := u.Kind(); got != vr.Value {
			t.Errorf("%s: Kind() = %q, want %q", vr.Field, got, vr.Value)
		}
	}
	for f := range reflect.TypeFor[SessionUpdate]().Fields() {
		if f.Type.Kind() == reflect.Pointer && !seen[f.Name] {
			t.Errorf("field %s is not among schemaUnions' variants", f.Name)
		}
	}
}

func testKindEachConstructor(t *testing.T) {
	t.Parallel()
	for value, b := range stableUpdateBuilders {
		u := b.build()
		if got, sent := u.Kind(), sentKind(t, u); got != sent {
			t.Errorf("%s: Kind() = %q, sent %q", value, got, sent)
		}
	}
}

func testKindUnknown(t *testing.T) {
	t.Parallel()
	for _, c := range kindUnknownCases {
		if got := (SessionUpdate{Unknown: json.RawMessage(c.payload)}).Kind(); got != c.want {
			t.Errorf("%s: Kind() = %q, want %q", c.payload, got, c.want)
		}
	}
}

func testKindDecodingAgrees(t *testing.T) {
	t.Parallel()
	for _, c := range kindUnknownCases {
		if !c.decodes {
			continue
		}
		var u SessionUpdate
		if err := json.Unmarshal([]byte(c.payload), &u); err != nil {
			t.Errorf("%s: unmarshal: %v", c.payload, err)
			continue
		}
		if string(u.Unknown) != c.payload {
			t.Errorf("%s: Unknown = %s, want the payload", c.payload, u.Unknown)
		}
		if got := u.Kind(); got != c.want {
			t.Errorf("%s: decoded Kind() = %q, want %q", c.payload, got, c.want)
		}
	}
}

func testKindZeroAndOrder(t *testing.T) {
	t.Parallel()
	if got := (SessionUpdate{}).Kind(); got != "" {
		t.Errorf("zero: Kind() = %q, want \"\"", got)
	}
	two := SessionUpdate{
		UserMessageChunk: &SessionUpdateUserMessageChunk{Content: TextBlock("hi")},
		Notice:           &SessionUpdateNotice{Severity: NoticeSeverityInfo, Title: "t"},
	}
	if got, sent := two.Kind(), sentKind(t, two); got != "user_message_chunk" || got != sent {
		t.Errorf("two set: Kind() = %q, sent %q, want \"user_message_chunk\"", got, sent)
	}
}

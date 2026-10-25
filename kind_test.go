package acp

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
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

// kindOf calls the Kind method of the union value v, a reflect.Value of the
// union's type.
func kindOf(t *testing.T, v reflect.Value) string {
	t.Helper()
	m := v.MethodByName("Kind")
	if !m.IsValid() {
		t.Fatalf("%s has no Kind method", v.Type())
	}
	return m.Call(nil)[0].String()
}

// unionKindTypes are the unions of
// 0038-PLAN-consumer-helpers-kind-and-agent-refusals.md, step 5, by name.
var unionKindTypes = map[string]reflect.Type{
	"ContentBlock":             reflect.TypeFor[ContentBlock](),
	"ToolCallContent":          reflect.TypeFor[ToolCallContent](),
	"RequestPermissionOutcome": reflect.TypeFor[RequestPermissionOutcome](),
	"SessionConfigOption":      reflect.TypeFor[SessionConfigOption](),
}

// TestUnionKinds requires the Kind of each of the four unions to name every
// variant as schemaUnions() lists it, an Unknown payload as UnmarshalJSON
// reads it, and "" for the zero value
// (0038-PLAN-consumer-helpers-kind-and-agent-refusals.md, step 5).
func TestUnionKinds(t *testing.T) {
	t.Parallel()
	found := 0
	for _, su := range schemaUnions() {
		rt, ok := unionKindTypes[su.Type]
		if !ok {
			continue
		}
		found++
		t.Run(su.Type, func(t *testing.T) {
			t.Parallel()
			testUnionVariants(t, rt, su)
			testUnionUnknown(t, rt, su.Key)
			if got := kindOf(t, reflect.New(rt).Elem()); got != "" {
				t.Errorf("zero: Kind() = %q, want \"\"", got)
			}
		})
	}
	if found != len(unionKindTypes) {
		t.Fatalf("schemaUnions() lists %d of the %d unions", found, len(unionKindTypes))
	}
}

func testUnionVariants(t *testing.T, rt reflect.Type, su schemaUnion) {
	t.Helper()
	seen := map[string]bool{}
	for _, vr := range su.Variants {
		u := reflect.New(rt).Elem()
		f := u.FieldByName(vr.Field)
		if !f.IsValid() || f.Kind() != reflect.Pointer {
			t.Errorf("%s: no pointer field %s", su.Type, vr.Field)
			continue
		}
		f.Set(reflect.New(f.Type().Elem()))
		seen[vr.Field] = true
		if got := kindOf(t, u); got != vr.Value {
			t.Errorf("%s.%s: Kind() = %q, want %q", su.Type, vr.Field, got, vr.Value)
		}
	}
	for f := range rt.Fields() {
		if f.Type.Kind() == reflect.Pointer && !seen[f.Name] {
			t.Errorf("%s: field %s is not among schemaUnions' variants", su.Type, f.Name)
		}
	}
}

func testUnionUnknown(t *testing.T, rt reflect.Type, key string) {
	t.Helper()
	upper := strings.ToUpper(key)
	title := strings.ToUpper(key[:1]) + key[1:]
	for _, c := range []struct {
		payload string
		want    string
		decodes bool
	}{
		{`{"` + key + `":"future"}`, "future", true},
		{`{"` + title + `":"folded"}`, "folded", true},
		{`{"` + key + `":"a","` + upper + `":"b"}`, "b", true},
		{`{"` + key + `":""}`, "unknown", true},
		{`{"` + key + `":1}`, "unknown", false},
		{`{}`, "unknown", false},
		{`[]`, "unknown", false},
		{`not json`, "unknown", false},
	} {
		u := reflect.New(rt).Elem()
		u.FieldByName("Unknown").Set(reflect.ValueOf(json.RawMessage(c.payload)))
		if got := kindOf(t, u); got != c.want {
			t.Errorf("%s %s: Kind() = %q, want %q", rt.Name(), c.payload, got, c.want)
		}
		if !c.decodes {
			continue
		}
		p := reflect.New(rt)
		if err := json.Unmarshal([]byte(c.payload), p.Interface()); err != nil {
			t.Errorf("%s %s: unmarshal: %v", rt.Name(), c.payload, err)
			continue
		}
		if got := kindOf(t, p.Elem()); got != c.want {
			t.Errorf("%s %s: decoded Kind() = %q, want %q", rt.Name(), c.payload, got, c.want)
		}
	}
}

// TestKindCoversKeyedStableUnions requires a Kind method on every union in
// the stable schema whose every variant carries its discriminator and that
// has no catch-all, so a schema bump that adds one fails here
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md, question 1).
func TestKindCoversKeyedStableUnions(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("schema/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	var covered []string
	for _, su := range schemaUnions() {
		if _, stable := s.Defs[su.Type]; !stable || su.Key == "" || su.CatchAll != "" {
			continue
		}
		if slices.ContainsFunc(su.Variants, func(v schemaVariant) bool { return v.Value == "" }) {
			continue
		}
		covered = append(covered, su.Type)
		rt := reflect.TypeOf(typeByName[su.Type].New()).Elem()
		if m, ok := rt.MethodByName("Kind"); !ok || m.Type.NumOut() != 1 || m.Type.Out(0).Kind() != reflect.String {
			t.Errorf("%s: every variant carries %q and it is stable, but it has no Kind() string", su.Type, su.Key)
		}
	}
	want := []string{"ContentBlock", "RequestPermissionOutcome", "SessionConfigOption", "SessionUpdate", "ToolCallContent"}
	if slices.Sort(covered); !slices.Equal(covered, want) {
		t.Errorf("keyed stable unions %v, want %v; a change here needs a Kind() or a reason in the MADR", covered, want)
	}
}

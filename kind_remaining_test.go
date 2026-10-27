package acp

import (
	"encoding/json"
	"reflect"
	"testing"
)

// remainingKindTypes are the unions of
// 0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phase 12, by name.
var remainingKindTypes = map[string]reflect.Type{
	"AuthMethod":                    reflect.TypeFor[AuthMethod](),
	"McpServer":                     reflect.TypeFor[McpServer](),
	"CreateElicitationRequest":      reflect.TypeFor[CreateElicitationRequest](),
	"CreateElicitationResponse":     reflect.TypeFor[CreateElicitationResponse](),
	"MultiSelectItems":              reflect.TypeFor[MultiSelectItems](),
	"SetSessionConfigOptionRequest": reflect.TypeFor[SetSessionConfigOptionRequest](),
	"PlanUpdateContent":             reflect.TypeFor[PlanUpdateContent](),
}

// keylessKinds are the names Kind gives the variants sent with no
// discriminator, as the schema names them.
var keylessKinds = map[string]string{
	"AuthMethod.Agent":                      "agent",
	"McpServer.Stdio":                       "stdio",
	"SetSessionConfigOptionRequest.ValueId": "value_id",
	"MultiSelectItems.Titled":               "titled",
}

// TestRemainingUnionKinds requires the Kind of each of the seven unions to
// name every variant as schemaUnions() lists it, a variant sent with no
// discriminator by the schema's name for it, a catch-all by the value it
// holds, an Unknown payload as UnmarshalJSON reads it, and "" for the zero
// value (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 10).
func TestRemainingUnionKinds(t *testing.T) {
	t.Parallel()
	found := 0
	for _, su := range schemaUnions() {
		rt, ok := remainingKindTypes[su.Type]
		if !ok {
			continue
		}
		found++
		t.Run(su.Type, func(t *testing.T) {
			t.Parallel()
			seen := map[string]bool{}
			for _, vr := range su.Variants {
				if vr.Field == su.CatchAll {
					continue // below: it holds its discriminator
				}
				u := reflect.New(rt).Elem()
				f := u.FieldByName(vr.Field)
				if !f.IsValid() || f.Kind() != reflect.Pointer {
					t.Fatalf("%s: no pointer field %s", su.Type, vr.Field)
				}
				f.Set(reflect.New(f.Type().Elem()))
				seen[vr.Field] = true
				want := vr.Value
				if want == "" {
					want = keylessKinds[su.Type+"."+vr.Field]
				}
				if want == "" {
					t.Errorf("%s.%s has no discriminator value and no name in keylessKinds", su.Type, vr.Field)
				}
				if got := kindOf(t, u); got != want {
					t.Errorf("%s.%s: Kind() = %q, want %q", su.Type, vr.Field, got, want)
				}
			}
			if su.CatchAll != "" {
				u := reflect.New(rt).Elem()
				f := u.FieldByName(su.CatchAll)
				other := reflect.New(f.Type().Elem())
				other.Elem().FieldByIndex(discriminatorField(t, other.Elem().Type(), su.Key)).SetString("_future")
				f.Set(other)
				seen[su.CatchAll] = true
				if got := kindOf(t, u); got != "_future" {
					t.Errorf("%s.%s holding %q: Kind() = %q, want it as it arrived", su.Type, su.CatchAll, "_future", got)
				}
			}
			for f := range rt.Fields() {
				if f.Type.Kind() == reflect.Pointer && !seen[f.Name] {
					t.Errorf("%s: field %s is not among schemaUnions' variants", su.Type, f.Name)
				}
			}
			if su.Unknown {
				testUnionUnknown(t, rt, su.Key)
			}
			if got := kindOf(t, reflect.New(rt).Elem()); got != "" {
				t.Errorf("zero: Kind() = %q, want \"\"", got)
			}
		})
	}
	if found != len(remainingKindTypes) {
		t.Fatalf("schemaUnions() lists %d of the %d unions", found, len(remainingKindTypes))
	}
}

// discriminatorField is the index of the string field of rt whose JSON name
// is key.
func discriminatorField(t *testing.T, rt reflect.Type, key string) []int {
	t.Helper()
	for f := range rt.Fields() {
		if f.Tag.Get("json") == key && f.Type.Kind() == reflect.String {
			return f.Index
		}
	}
	t.Fatalf("%s has no string field %q", rt, key)
	return nil
}

// TestRemainingUnionKindsDecoded requires each variant of the seven unions,
// decoded from JSON as a peer sends it, to give its Kind
// (0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phase 12).
func TestRemainingUnionKindsDecoded(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		union, payload, want string
	}{
		{"AuthMethod", `{"type":"terminal","id":"t","name":"T"}`, "terminal"},
		{"AuthMethod", `{"id":"a","name":"A"}`, "agent"},
		{"AuthMethod", `{"type":"future","id":"x","name":"X"}`, "future"},
		{"McpServer", `{"type":"http","name":"m","url":"https://example.invalid","headers":[]}`, "http"},
		{"McpServer", `{"type":"sse","name":"m","url":"https://example.invalid","headers":[]}`, "sse"},
		{"McpServer", `{"type":"acp","name":"m","serverId":"x"}`, "acp"},
		{"McpServer", `{"name":"m","command":"/bin/server","args":[],"env":[]}`, "stdio"},
		{"McpServer", `{"type":"ws","name":"m"}`, "ws"},
		{"CreateElicitationRequest", `{"sessionId":"s","mode":"form","message":"m","requestedSchema":{"type":"object","properties":{}}}`, "form"},
		{"CreateElicitationRequest", `{"sessionId":"s","mode":"url","message":"m","url":"https://example.invalid","elicitationId":"e"}`, "url"},
		{"CreateElicitationRequest", `{"mode":"_x","message":"m"}`, "_x"},
		{"CreateElicitationResponse", `{"action":"accept","content":{}}`, "accept"},
		{"CreateElicitationResponse", `{"action":"decline"}`, "decline"},
		{"CreateElicitationResponse", `{"action":"cancel"}`, "cancel"},
		{"CreateElicitationResponse", `{"action":"_later"}`, "_later"},
		{"MultiSelectItems", `{"type":"string","enum":["a"]}`, "string"},
		{"MultiSelectItems", `{"type":"x-custom"}`, "x-custom"},
		{"MultiSelectItems", `{"anyOf":[{"const":"a","title":"A"}]}`, "titled"},
		{"SetSessionConfigOptionRequest", `{"sessionId":"s","configId":"c","type":"boolean","value":true}`, "boolean"},
		{"SetSessionConfigOptionRequest", `{"sessionId":"s","configId":"c","value":"v"}`, "value_id"},
		{"SetSessionConfigOptionRequest", `{"sessionId":"s","configId":"c","type":"future","value":1}`, "future"},
		{"PlanUpdateContent", `{"type":"items","planId":"p","entries":[]}`, "items"},
		{"PlanUpdateContent", `{"type":"file","planId":"p","uri":"file:///plan.md"}`, "file"},
		{"PlanUpdateContent", `{"type":"markdown","planId":"p","content":"# Plan"}`, "markdown"},
		{"PlanUpdateContent", `{"type":"future","planId":"p"}`, "future"},
	} {
		p := reflect.New(remainingKindTypes[c.union])
		if err := json.Unmarshal([]byte(c.payload), p.Interface()); err != nil {
			t.Errorf("%s %s: unmarshal: %v", c.union, c.payload, err)
			continue
		}
		if got := kindOf(t, p.Elem()); got != c.want {
			t.Errorf("%s %s: decoded Kind() = %q, want %q", c.union, c.payload, got, c.want)
		}
	}
}

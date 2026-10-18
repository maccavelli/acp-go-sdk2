package model

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

func TestExpandAllOf(t *testing.T) {
	t.Parallel()
	schema := schemaOf(t, `{
		"Base":{"type":"object","required":["a","b"],"properties":{"a":{"type":"string"},"b":{"type":"integer"}}},
		"Deep":{"allOf":[{"$ref":"#/$defs/Base"}],"required":["c"],"properties":{"c":{"type":"string"}}},
		"List":{"type":"array","items":{"type":"string"}},
		"Pick":{"enum":["x"]},
		"Either":{"anyOf":[{"type":"string"}]},
		"One":{"oneOf":[{"type":"string"}]}}`)
	d := defOf(t, `{"description":"Local.","required":["a","z"],"properties":{"a":{"type":"boolean"},"z":{"type":"string"}},
		"allOf":[null,{"$ref":"#/$defs/Deep"}]}`)
	got := expandAllOf(schema, d)
	if got.Type != schemaTypeObject || got.Description != "Local." || got.AllOf != nil {
		t.Errorf("merged head: type %v description %q allOf %v", got.Type, got.Description, got.AllOf)
	}
	if want := []string{"a", "z", "c", "b"}; !slices.Equal(got.Required, want) {
		t.Errorf("required: got %v, want %v", got.Required, want)
	}
	if len(got.Properties) != 4 || got.Properties["a"].Type != "boolean" {
		t.Errorf("properties: %v, a = %v", len(got.Properties), got.Properties["a"].Type)
	}
	if len(d.Properties) != 2 || len(d.Required) != 2 {
		t.Error("expandAllOf changed its input")
	}
	for _, tt := range []struct {
		ref   string
		check func(*load.Definition) bool
	}{
		{"List", func(m *load.Definition) bool { return m.Type == "array" && m.Items != nil }},
		{"Pick", func(m *load.Definition) bool { return len(m.Enum) == 1 }},
		{"Either", func(m *load.Definition) bool { return len(m.AnyOf) == 1 }},
		{"One", func(m *load.Definition) bool { return len(m.OneOf) == 1 }},
	} {
		m := expandAllOf(schema, defOf(t, `{"allOf":[{"$ref":"#/$defs/`+tt.ref+`"}]}`))
		if !tt.check(m) || m.Ref != "" {
			t.Errorf("allOf of %s: merged %+v", tt.ref, m)
		}
	}
	if m := expandAllOf(schema, defOf(t, `{"allOf":[{"$ref":"#/$defs/Missing"}]}`)); m.Ref != "#/$defs/Missing" {
		t.Errorf("an unresolved $ref is kept: got %q", m.Ref)
	}
	if expandAllOf(schema, nil) != nil {
		t.Error("nil is not returned as nil")
	}
}

func TestSingleRefVariant(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ def, want string }{
		{`{"allOf":[{"$ref":"#/$defs/A"}]}`, "#/$defs/A"},
		{`{"allOf":[{"$ref":"#/$defs/A"}],"properties":{"x":{"type":"string"}}}`, ""},
		{`{"allOf":[{"$ref":"#/$defs/A"},{"$ref":"#/$defs/B"}]}`, ""},
		{`{"allOf":[{"type":"string"}]}`, ""},
		{`{"type":"object","allOf":[{"$ref":"#/$defs/A"}]}`, ""},
	} {
		if got := singleRefVariant(defOf(t, tt.def)); got != tt.want {
			t.Errorf("singleRefVariant(%s) = %q, want %q", tt.def, got, tt.want)
		}
	}
}

func TestPrimaryType(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ def, want string }{
		{`{"type":"string"}`, "string"},
		{`{"type":["null","integer"]}`, "integer"},
		{`{"type":["null"]}`, "null"},
		{`{"allOf":[null,{"$ref":"#/$defs/A"},{"type":"object"}]}`, "object"},
		{`{"allOf":[{"$ref":"#/$defs/A"}]}`, ""},
		{`{}`, ""},
	} {
		if got := primaryType(defOf(t, tt.def)); got != tt.want {
			t.Errorf("primaryType(%s) = %q, want %q", tt.def, got, tt.want)
		}
	}
	if primaryType(nil) != "" || primaryType(&load.Definition{Type: 7}) != "" {
		t.Error("nil or a non-string type is not empty")
	}
}

func TestGenerateNestedTypeName(t *testing.T) {
	t.Parallel()
	used := map[string]bool{"Taken": true, "TakenField": true, "TakenFieldInline": true}
	for _, tt := range []struct{ parent, field, want string }{
		{"RequestPermissionRequest", "toolCall", "RequestPermissionToolCall"},
		{"SessionNotification", "update", "SessionUpdate"},
		{"AgentNotification", "ExtNotification", "AgentExtNotification"},
		{"ContentChunk", "chunkData", "ContentChunkData"},
		{"Parent", "child_name", "ParentChildName"},
		{"Taken", "field", "TakenFieldInline2"},
	} {
		got, err := generateNestedTypeName(tt.parent, tt.field, used)
		if err != nil || got != tt.want {
			t.Errorf("generateNestedTypeName(%q, %q) = %q, %v; want %q", tt.parent, tt.field, got, err, tt.want)
		}
	}
	full := map[string]bool{"PField": true, "PFieldInline": true}
	for i := 2; i < 1000; i++ {
		full["PFieldInline"+itoa(i)] = true
	}
	if _, err := generateNestedTypeName("P", "field", full); err == nil {
		t.Error("no error when every candidate is taken")
	}
}

// itoa formats a small positive integer.
func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}

// TestEnumValueKinds requires each enum kind's values, their constants, and
// the schema description of a value that has one as its Doc
// (0019-PLAN-examples-docs-tests-tooling.md, I12).
func TestEnumValueKinds(t *testing.T) {
	t.Parallel()
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{
		"Consts":{"oneOf":[{"const":"a-b","description":"The first value."},{"const":"c"}]},
		"Open":{"anyOf":[{"type":"string","const":"known","description":"A known value."},{"type":"string"}]},
		"Plain":{"type":"string","enum":["p"]},
		"Codes":{"anyOf":[{"type":"integer","format":"int32","const":-1,"title":"Minus one","description":"Below zero."}]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		kind Kind
		want []EnumValue
	}{
		{"Consts", KindStringConstUnion, []EnumValue{
			{Value: "a-b", Const: "ConstsAB", Doc: []string{"The first value."}},
			{Value: "c", Const: "ConstsC"},
		}},
		{"Open", KindOpenEnum, []EnumValue{{Value: "known", Const: "OpenKnown", Doc: []string{"A known value."}}}},
		{"Plain", KindEnum, []EnumValue{{Value: "p", Const: "PlainP"}}},
		{"Codes", KindIntegerEnum, []EnumValue{{Value: "-1", Const: "CodesMinusOne", Doc: []string{"Below zero."}}}},
	} {
		ty := m.Type(tt.name)
		if ty.Kind != tt.kind || !reflect.DeepEqual(ty.Enum, tt.want) {
			t.Errorf("%s: kind %v enum %v, want %v %v", tt.name, ty.Kind, ty.Enum, tt.kind, tt.want)
		}
	}
	for _, def := range []string{`{"anyOf":[{"type":"string","const":1}]}`, `{"anyOf":[{"type":"string"},{"type":"string","properties":{"a":{}}}]}`,
		`{"anyOf":[{"type":"string"}]}`, `{"anyOf":[null]}`} {
		if isOpenStringEnum(defOf(t, def)) {
			t.Errorf("isOpenStringEnum(%s) is true", def)
		}
	}
	// An enum of anything but strings has no Go type
	// (0019-PLAN-generator-hardening.md, G4).
	if _, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{"Nums":{"enum":[1,2.5]}}`)}); err == nil ||
		!strings.Contains(err.Error(), "Nums: enum value 1 is not a string") {
		t.Errorf("a number enum: Build = %v", err)
	}
	if isStringConstUnion(defOf(t, `{"oneOf":[{"const":1}]}`)) || isStringConstUnion(defOf(t, `{"oneOf":[null]}`)) {
		t.Error("a non-string or null const counts as a string const union")
	}
}

// TestCheckTypable accepts the inline objects the model types, and those in a
// not, which only constrains; the refusals are engine cases
// (testdata/engine/refuse-allof-*.txtar and refuse-*inline*.txtar).
func TestCheckTypable(t *testing.T) {
	t.Parallel()
	schema := schemaOf(t, `{
		"A":{"type":"object","properties":{"a":{"type":"string"}}},
		"Nested":{"type":"object","properties":{"inner":{"type":"object","properties":{"x":{"type":"string"}}}}},
		"Wrapped":{"type":"object","properties":{"a":{"allOf":[{"$ref":"#/$defs/A"}],"description":"One A."}}},
		"Union":{"anyOf":[{"type":"object","properties":{"k":{"type":"string"}}},{"allOf":[{"$ref":"#/$defs/A"}]}],
			"not":{"anyOf":[{"type":"object","properties":{"k":{"const":"x"}}}]}}}`)
	if err := checkRefs(schema); err != nil {
		t.Errorf("checkRefs refused shapes the model types: %v", err)
	}
}

package model

import (
	"slices"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

func TestUnionDiscriminator(t *testing.T) {
	t.Parallel()
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{
		"Declared":{"discriminator":{"propertyName":"kind"},"oneOf":[
			{"type":"object","properties":{"kind":{"const":"a","type":"string"},"other":{"const":"z","type":"string"}}}]},
		"Derived":{"oneOf":[{"type":"object","properties":{"zeta":{"const":"a","type":"string"},"alpha":{"const":"b","type":"string"}}}]},
		"Plain":{"anyOf":[{"type":"string"},{"type":"integer"}]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, key, field string
		src              DiscSource
	}{
		{"Declared", "kind", "Kind", DiscDeclared},
		{"Derived", "alpha", "Alpha", DiscDerived},
		{"Plain", "", "", DiscNone},
	} {
		u := m.Type(tt.name)
		if u.DiscKey != tt.key || u.DiscField != tt.field || u.DiscSource != tt.src {
			t.Errorf("%s: got key %q field %q source %v, want %q %q %v", tt.name, u.DiscKey, u.DiscField, u.DiscSource, tt.key, tt.field, tt.src)
		}
	}
	if got := m.Type("Declared").Variants[0].DiscValue; got != "a" {
		t.Errorf("Declared variant value: %q", got)
	}
}

func TestVariantFacts(t *testing.T) {
	t.Parallel()
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{
		"Item":{"type":"object","required":["z","id"],"properties":{"id":{"type":"string"},"z":{"type":"string"}}},
		"Ref":{"type":"object","properties":{"a":{"type":"string"}}},
		"V":{"anyOf":[
			{"type":"object","description":"Obj.","required":["type","b"],"properties":{"type":{"const":"obj","type":"string"},"b":{"type":"string"},"a":{"const":"k","type":"string"}}},
			{"title":"items","type":"array","items":{"$ref":"#/$defs/Item"}},
			{"type":"null"},
			{"$ref":"#/$defs/Ref"}]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	vs := m.Type("V").Variants
	obj, arr, null, ref := vs[0], vs[1], vs[2], vs[3]
	if !obj.IsObject || obj.IsArray || obj.IsNull || !obj.HasSharedFields || !slices.Equal(obj.Required, []string{"type", "b"}) ||
		len(obj.Doc) != 1 || obj.Doc[0] != "Obj." {
		t.Errorf("object variant: %+v", obj)
	}
	if want := [][2]string{{"a", "k"}, {"type", "obj"}}; !slices.Equal(obj.ConstPairs, want) {
		t.Errorf("object variant consts: got %v, want %v", obj.ConstPairs, want)
	}
	if !arr.IsArray || arr.IsObject || !slices.Equal(arr.ArrayItemRequired, []string{"id", "z"}) {
		t.Errorf("array variant: %+v", arr)
	}
	if !null.IsNull || null.HasSharedFields {
		t.Errorf("null variant: %+v", null)
	}
	// A $ref variant is not expanded, so it has no properties of its own: it is
	// matched by discriminator or by trial decode, never by required keys.
	if ref.IsObject || ref.HasSharedFields || ref.Inline != nil {
		t.Errorf("$ref variant: %+v", ref)
	}
}

func TestValidateKinds(t *testing.T) {
	t.Parallel()
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{
		"One":{"oneOf":[{"type":"string"},{"type":"integer"}]},
		"Shared":{"required":["s","n","list"],"properties":{"s":{"type":"string"},"n":{"type":"integer"},"list":{"type":"array"}},
			"anyOf":[{"type":"object","properties":{"x":{"type":"string"}}}]},
		"ToolCallUpdate":{"type":"object","properties":{"toolCallId":{"type":"string"}}},
		"FooRequest":{"type":"object","required":["name","n","ids"],"properties":{"name":{"type":"string"},"n":{"type":"integer"},"ids":{"type":"array"},"opt":{"type":"string"}}},
		"BarResponse":{"type":"object"},
		"Plain":{"type":"object","required":["a"],"properties":{"a":{"type":"string"}}},
		"KindNotification":{"type":"string","enum":["a"]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		kind   ValidateKind
		checks []RequiredCheck
	}{
		{"One", ValidateExactlyOne, nil},
		{"Shared", ValidateShared, []RequiredCheck{{"List", "list", "array"}, {"S", "s", "string"}}},
		{"ToolCallUpdate", ValidateToolCall, []RequiredCheck{{"ToolCallId", "toolCallId", "string"}}},
		{"FooRequest", ValidateFields, []RequiredCheck{{"Ids", "ids", "array"}, {"Name", "name", "string"}}},
		{"BarResponse", ValidateFields, nil},
		{"Plain", ValidateNone, nil},
		{"KindNotification", ValidateNone, nil},
	} {
		got := m.Type(tt.name)
		if got.Validate != tt.kind || !slices.Equal(got.Checks, tt.checks) {
			t.Errorf("%s: got %v %v, want %v %v", tt.name, got.Validate, got.Checks, tt.kind, tt.checks)
		}
	}
}

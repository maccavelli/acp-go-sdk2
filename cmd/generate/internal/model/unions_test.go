package model

import (
	"slices"
	"strings"
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
			{"type":"object","description":"Obj.","required":["b"],"properties":{"b":{"type":"string"}}},
			{"title":"items","type":"array","items":{"$ref":"#/$defs/Item"}},
			{"type":"null"},
			{"$ref":"#/$defs/Ref"}]},
		"W":{"anyOf":[
			{"type":"object","required":["type","b"],"properties":{"type":{"const":"obj","type":"string"},"b":{"type":"string"},"a":{"const":"k","type":"string"}}},
			{"type":"object","required":["type"],"properties":{"type":{"const":"other","type":"string"}}}]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	// V mixes shapes, so it has no discriminator; W's variants carry constants.
	vs := m.Type("V").Variants
	obj, arr, null, ref := vs[0], vs[1], vs[2], vs[3]
	if !obj.IsObject || obj.IsArray || obj.IsNull || !obj.HasSharedFields || !slices.Equal(obj.Required, []string{"b"}) ||
		len(obj.Doc) != 1 || obj.Doc[0] != "Obj." {
		t.Errorf("object variant: %+v", obj)
	}
	w := m.Type("W").Variants[0]
	if want := [][2]string{{"a", "k"}, {"type", "obj"}}; !slices.Equal(w.ConstPairs, want) || !slices.Equal(w.Required, []string{"type", "b"}) {
		t.Errorf("object variant consts: got %v required %v, want %v", w.ConstPairs, w.Required, want)
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

// TestValidations requires Validate where the schema states something to
// check, and only the checks it states: every union and RPC object; a
// minLength; a bound outside the Go type's range, and none at its edge; a
// nested value in each shape; and, to a fixed point, every struct that holds
// one. A validated type deeper than one level is refused.
func TestValidations(t *testing.T) {
	t.Parallel()
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{
		"U":{"oneOf":[{"type":"string"},{"type":"integer"}]},
		"BarResponse":{"type":"object"},
		"Plain":{"type":"object","required":["a"],"properties":{"a":{"type":"string"}}},
		"Str":{"type":"object","properties":{"t":{"type":"string","minLength":2}}},
		"Num":{"type":"object","properties":{
			"i":{"type":"integer","format":"int32","minimum":5},
			"j":{"type":"integer","format":"uint32","minimum":0},
			"d":{"type":["number","null"],"format":"double","maximum":1}}},
		"Mid":{"type":"object","properties":{"s":{"allOf":[{"$ref":"#/$defs/Str"}]}}},
		"Chain":{"type":"object","properties":{"c":{"allOf":[{"$ref":"#/$defs/Mid"}]}}},
		"Shapes":{"type":"object","properties":{
			"v":{"allOf":[{"$ref":"#/$defs/U"}]},
			"p":{"anyOf":[{"$ref":"#/$defs/U"},{"type":"null"}]},
			"s":{"type":"array","items":{"$ref":"#/$defs/U"}},
			"m":{"type":"object","additionalProperties":{"$ref":"#/$defs/U"}}}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		kind    CheckKind
		key     string
		shape   RefKind
		bound   float64
		integer bool
	}
	got := func(name string) []row {
		v := m.Type(name).Validation
		if v == nil {
			return nil
		}
		out := []row{}
		for _, c := range v.Checks {
			out = append(out, row{c.Kind, c.Key, c.Shape, c.Bound, c.Integer})
		}
		return out
	}
	for _, tt := range []struct {
		name string
		want []row // nil: no Validate
	}{
		{"U", []row{}},
		{"BarResponse", []row{}},
		{"Plain", nil},
		{"Str", []row{{CheckMinLength, "t", RefNamed, 2, true}}},
		{"Num", []row{{CheckMaximum, "d", RefPointer, 1, false}, {CheckMinimum, "i", RefNamed, 5, true}}},
		{"Mid", []row{{CheckNested, "s", RefNamed, 0, false}}},
		{"Chain", []row{{CheckNested, "c", RefNamed, 0, false}}},
		{"Shapes", []row{{CheckNested, "m", RefMap, 0, false}, {CheckNested, "p", RefPointer, 0, false},
			{CheckNested, "s", RefSlice, 0, false}, {CheckNested, "v", RefNamed, 0, false}}},
	} {
		if g := got(tt.name); !slices.Equal(g, tt.want) {
			t.Errorf("%s: checks %v, want %v", tt.name, g, tt.want)
		}
	}
	if !m.Validates("U") || m.Validates("Plain") {
		t.Error("Validates disagrees with Validation")
	}
	_, err = Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{
		"U":{"oneOf":[{"type":"string"},{"type":"integer"}]},
		"Bad":{"type":"object","properties":{"x":{"type":"array","items":{"type":"array","items":{"$ref":"#/$defs/U"}}}}}}`)})
	if want := "Bad.x: [][]U holds a validated type in a shape Validate does not descend into"; err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("error %v, want one containing %q", err, want)
	}
}

// TestTitleOnlyVariantTakesSchemaType requires a title-only variant to take
// the type its schema declares, whatever its title and description say.
func TestTitleOnlyVariantTakesSchemaType(t *testing.T) {
	t.Parallel()
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{"U":{"anyOf":[
		{"title":"FooExt","description":"An extension payload.","type":"string"},
		{"type":"object","properties":{"a":{"type":"string"}}}]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	in := m.Type("U").Variants[0].Inline
	if in == nil || in.Kind != InlineNamed || in.Type.String() != "string" {
		t.Errorf("variant FooExt: %+v, want InlineNamed string", in)
	}
}

// flattenDefs has an object with a property of its own and an anyOf of two
// references, and a union whose variant is an allOf of it.
const flattenDefs = `{
	"SessionScope":{"type":"object","required":["sessionId"],"properties":{"sessionId":{"type":"string"}}},
	"RequestScope":{"type":"object","required":["requestId"],"properties":{"requestId":{"type":"integer"}}},
	"Mode":{"type":"object","required":["schema"],"properties":{"schema":{"type":"string"}},
		"anyOf":[{"title":"Session","allOf":[{"$ref":"#/$defs/SessionScope"}]},{"$ref":"#/$defs/RequestScope"}]},
	"Ask":{"oneOf":[{"type":"object","required":["mode"],"properties":{"mode":{"type":"string","const":"form"}},
		"allOf":[{"$ref":"#/$defs/Mode"}]},{"type":"object","required":["mode"],"properties":{"mode":{"type":"string","const":"x"}}}]}}`

// TestFlattenedMembers requires such an object to be a struct with its members
// flattened, and a variant that is an allOf of it to get the same members
// (0011-MADR-harden-code-generator-engine.md, "Found in step 7 and decided").
func TestFlattenedMembers(t *testing.T) {
	t.Parallel()
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, flattenDefs)})
	if err != nil {
		t.Fatal(err)
	}
	describe := func(ms []*Member) string {
		var out []string
		for _, mb := range ms {
			out = append(out, mb.Field+":"+mb.Type+strings.Join(mb.Required, ","))
		}
		return strings.Join(out, " ")
	}
	mode := m.Type("Mode")
	if mode.Kind != KindObject || len(mode.Fields) != 1 || mode.Fields[0].Key != "schema" {
		t.Fatalf("Mode: kind %v, fields %d", mode.Kind, len(mode.Fields))
	}
	if got := describe(mode.Members); got != "Session:SessionScopesessionId RequestScope:RequestScoperequestId" {
		t.Errorf("Mode members: %s", got)
	}
	if mode.Validation == nil || len(mode.Validation.Members) != 2 {
		t.Errorf("Mode Validation: %+v, want one requiring exactly one of two members", mode.Validation)
	}
	form := m.Type("Ask").Variants[0].Inline
	if form == nil || describe(form.Members) != describe(mode.Members) {
		t.Errorf("Ask's form variant: %+v, want Mode's members", form)
	}
	if form != nil && form.Validation == nil {
		t.Error("Ask's form variant has no Validate")
	}
	if m.Type("Ask").Variants[1].Inline.Members != nil {
		t.Error("Ask's other variant got members")
	}

	clash := strings.Replace(flattenDefs, `"properties":{"schema":{"type":"string"}}`, `"properties":{"session":{"type":"string"}}`, 1)
	clash = strings.Replace(clash, `"required":["schema"]`, `"required":["session"]`, 1)
	if _, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, clash)}); err == nil || !strings.Contains(err.Error(), "both become field Session") {
		t.Errorf("a member named as a property: %v", err)
	}
}

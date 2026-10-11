package model

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

// Shapes the committed schema does not use, found by the mutation run of
// 0011-PLAN-engine-tests.md, step 9. Each test names what it pins.

func buildDefs(t *testing.T, defs string) (*Model, error) {
	t.Helper()
	return Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, defs)})
}

func fieldOf(t *testing.T, m *Model, typ, key string) *Field {
	t.Helper()
	for _, f := range m.Type(typ).Fields {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("%s has no field %q", typ, key)
	return nil
}

// TestAllOfOfOneInlineSchema requires a property that is an allOf of one schema
// without a $ref to take that schema's type, its items included.
func TestAllOfOfOneInlineSchema(t *testing.T) {
	t.Parallel()
	m, err := buildDefs(t, `{"T":{"type":"object","required":["p","q"],"properties":{
		"p":{"allOf":[{"type":"string"}]},
		"q":{"allOf":[{"type":"array","items":{"type":"string"}}]}}}}`)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"p": "string", "q": "[]string"} {
		if got := fieldOf(t, m, "T", key).Type.String(); got != want {
			t.Errorf("%s is %s, want %s", key, got, want)
		}
	}
}

// TestNullableWithNullFirst requires an anyOf of null and a reference to be
// the optional reference whichever member comes first.
func TestNullableWithNullFirst(t *testing.T) {
	t.Parallel()
	for _, members := range []string{`{"type":"null"},{"$ref":"#/$defs/X"}`, `{"$ref":"#/$defs/X"},{"type":"null"}`} {
		m, err := buildDefs(t, `{"X":{"type":"object","properties":{"a":{"type":"string"}}},"T":{"type":"object","properties":{"p":{"anyOf":[`+members+`]}}}}`)
		if err != nil {
			t.Fatal(err)
		}
		if got := fieldOf(t, m, "T", "p").Type.String(); got != "*X" {
			t.Errorf("anyOf [%s] is %s, want *X", members, got)
		}
	}
}

// TestHalfLinkedDefinitionIgnored requires a definition with x-method but no
// x-side, or the reverse, to link to no method, whatever its name.
func TestHalfLinkedDefinitionIgnored(t *testing.T) {
	t.Parallel()
	for _, defs := range []string{`{"Thing":{"type":"object","x-method":"a/b"}}`, `{"Thing":{"type":"object","x-side":"agent"}}`} {
		if _, err := buildDefs(t, defs); err != nil {
			t.Errorf("%s: %v", defs, err)
		}
	}
}

// TestIntegerEnumMembers requires an anyOf of integers to be an integer enum
// only when every member is a plain integer: a null member, a $ref, properties,
// or an allOf makes it something else.
func TestIntegerEnumMembers(t *testing.T) {
	t.Parallel()
	for _, odd := range []string{`null`, `{"type":"integer","$ref":"#/$defs/I"}`, `{"type":"integer","properties":{"x":{"type":"string"}}}`, `{"type":"integer","allOf":[{"$ref":"#/$defs/I"}]}`} {
		var def load.Definition
		if err := json.Unmarshal([]byte(`{"anyOf":[{"title":"A","type":"integer","const":1},`+odd+`]}`), &def); err != nil {
			t.Fatal(err)
		}
		if isIntegerEnum(&def) {
			t.Errorf("a member %s: an integer enum", odd)
		}
	}
}

// TestReferenceWithPropertiesIsNotNested requires a property with a $ref to
// be that reference, though it also lists properties.
func TestReferenceWithPropertiesIsNotNested(t *testing.T) {
	t.Parallel()
	m, err := buildDefs(t, `{"X":{"type":"object","properties":{"a":{"type":"string"}}},
		"T":{"type":"object","properties":{"p":{"$ref":"#/$defs/X","type":"object","properties":{"b":{"type":"string"}}}}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := fieldOf(t, m, "T", "p").Type.String(); got != "X" || len(m.Type("T").NestedStructs) != 0 {
		t.Errorf("p is %s with nested structs %v, want X and none", got, m.Type("T").NestedStructs)
	}
}

// TestExpandAllOfKeepsOwnUnion requires a definition's own anyOf to win over
// the anyOf of the definition its allOf names.
func TestExpandAllOfKeepsOwnUnion(t *testing.T) {
	t.Parallel()
	s := schemaOf(t, `{"Base":{"anyOf":[{"type":"string"}]}}`)
	var d load.Definition
	if err := json.Unmarshal([]byte(`{"allOf":[{"$ref":"#/$defs/Base"}],"anyOf":[{"type":"integer"},{"type":"boolean"}]}`), &d); err != nil {
		t.Fatal(err)
	}
	if got := expandAllOf(s, &d); len(got.AnyOf) != 2 || primaryType(got.AnyOf[0]) != "integer" {
		t.Errorf("anyOf %v, want the definition's own two members", got.AnyOf)
	}
}

// TestOpenStringEnumMembers requires each member of an open string enum to be
// a plain string: one with a $ref, a oneOf, or an allOf is not.
func TestOpenStringEnumMembers(t *testing.T) {
	t.Parallel()
	for _, odd := range []string{`{"type":"string","$ref":"#/$defs/S"}`, `{"type":"string","oneOf":[{"type":"string"}]}`, `{"type":"string","allOf":[{"type":"string"}]}`} {
		var def load.Definition
		if err := json.Unmarshal([]byte(`{"anyOf":[{"type":"string","const":"a"},`+odd+`]}`), &def); err != nil {
			t.Fatal(err)
		}
		if isOpenStringEnum(&def) {
			t.Errorf("a member %s: an open string enum", odd)
		}
	}
}

// TestBoundOnNamedPrimitiveRefused requires each bound a named primitive type
// states, alone, to be refused, since its Validate would not run.
func TestBoundOnNamedPrimitiveRefused(t *testing.T) {
	t.Parallel()
	for _, def := range []string{`{"type":"integer","format":"uint32","minimum":1}`, `{"type":"integer","format":"uint32","maximum":10}`, `{"type":"string","minLength":1}`} {
		_, err := buildDefs(t, `{"N":`+def+`}`)
		if err == nil || !strings.Contains(err.Error(), "a bound on a named primitive type is not checked") {
			t.Errorf("%s: %v", def, err)
		}
	}
}

// TestNestedArraysWithoutValidation requires a field Validate does not descend
// into, holding nothing that validates, to be accepted and not checked.
func TestNestedArraysWithoutValidation(t *testing.T) {
	t.Parallel()
	m, err := buildDefs(t, `{"T":{"type":"object","properties":{"grid":{"type":"array","items":{"type":"array","items":{"type":"string"}}}}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if m.Validates("T") || slices.ContainsFunc(m.Type("T").Fields, func(f *Field) bool { return f.Key == "grid" && f.Type.String() != "[][]string" }) {
		t.Errorf("T validates %v, fields %v", m.Validates("T"), m.Type("T").Fields)
	}
}

// TestMethodNameCollisionNamesInterface requires two methods that become one
// Go name on an interface to be refused naming that interface.
func TestMethodNameCollisionNamesInterface(t *testing.T) {
	t.Parallel()
	p := testProtocol()
	p.Methods[2].GoName = "Prompt"
	_, err := Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, protocolDefs), Protocol: p})
	if err == nil || !strings.Contains(err.Error(), "interface Agent:") || !strings.Contains(err.Error(), "both become Prompt") {
		t.Errorf("error %v, want interface Agent and both become Prompt", err)
	}
}

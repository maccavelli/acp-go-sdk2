package acp_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// fieldNames returns the names of fs, and those that are required.
func fieldNames(fs []acp.ElicitationField) (names, required []string) {
	for _, f := range fs {
		names = append(names, f.Name)
		if f.Required {
			required = append(required, f.Name)
		}
	}
	return names, required
}

// wireOrder returns the keys of the properties object json.Marshal writes for
// s, in the order it writes them.
func wireOrder(t *testing.T, s acp.ElicitationSchema) []string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		t.Fatal(err)
	}
	d := json.NewDecoder(bytes.NewReader(top["properties"]))
	if tok, err := d.Token(); err != nil || tok != json.Delim('{') {
		t.Fatalf("properties is not an object: %s", top["properties"])
	}
	var keys []string
	for d.More() {
		tok, err := d.Token()
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		if err := d.Decode(&skip); err != nil {
			t.Fatal(err)
		}
	}
	return keys
}

// decodeSchema decodes a requestedSchema, failing the test on an error.
func decodeSchema(t *testing.T, raw string) acp.ElicitationSchema {
	t.Helper()
	var s acp.ElicitationSchema
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return s
}

// TestElicitationFields orders a form's fields as MarshalJSON writes them:
// the order the agent wrote, or sorted for a schema built in Go, each name
// once, a missing one skipped; required only for a declared name.
func TestElicitationFields(t *testing.T) {
	t.Parallel()
	str := acp.NewElicitationPropertySchemaString()
	built := acp.ElicitationSchema{Properties: map[string]acp.ElicitationPropertySchema{"b": str, "a": str, "c": str}}
	repeated := built
	repeated.PropertyOrder = []string{"c", "zz", "c", "a"}
	for _, tc := range []struct {
		name               string
		schema             acp.ElicitationSchema
		wantNames, wantReq []string
	}{
		{
			name:      "decoded",
			schema:    decodeSchema(t, `{"type":"object","properties":{"b":{"type":"string"},"a":{"type":"string"},"c":{"type":"string"}},"required":["c","zz"]}`),
			wantNames: []string{"b", "a", "c"}, wantReq: []string{"c"},
		},
		{name: "built in Go", schema: built, wantNames: []string{"a", "b", "c"}},
		{name: "repeated and missing", schema: repeated, wantNames: []string{"c", "a", "b"}},
	} {
		names, req := fieldNames(tc.schema.Fields())
		if !slices.Equal(names, tc.wantNames) || !slices.Equal(req, tc.wantReq) {
			t.Errorf("%s: Fields = %q, required %q; want %q, required %q", tc.name, names, req, tc.wantNames, tc.wantReq)
		}
		if wire := wireOrder(t, tc.schema); !slices.Equal(names, wire) {
			t.Errorf("%s: Fields = %q, but MarshalJSON writes %q", tc.name, names, wire)
		}
	}
}

// TestElicitationFieldsMatchWire requires Fields to give MarshalJSON's order
// for random property sets and orders, repeats and missing names included.
func TestElicitationFieldsMatchWire(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(40, 2))
	for i := range 20 {
		s := acp.ElicitationSchema{Properties: map[string]acp.ElicitationPropertySchema{}}
		for j := range 1 + r.IntN(6) {
			s.Properties[fmt.Sprintf("p%d", r.IntN(9)+j*10)] = acp.NewElicitationPropertySchemaBoolean()
		}
		for range r.IntN(8) {
			s.PropertyOrder = append(s.PropertyOrder, fmt.Sprintf("p%d", r.IntN(60)))
		}
		names, _ := fieldNames(s.Fields())
		if wire := wireOrder(t, s); !slices.Equal(names, wire) {
			t.Errorf("case %d, order %q: Fields = %q, but MarshalJSON writes %q", i, s.PropertyOrder, names, wire)
		}
	}
}

// TestElicitationPropertyKind names each property variant, a type this
// package does not know, and the zero value; Fields keeps such a property.
func TestElicitationPropertyKind(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		p    acp.ElicitationPropertySchema
		want string
	}{
		{acp.NewElicitationPropertySchemaString(), "string"},
		{acp.NewElicitationPropertySchemaNumber(), "number"},
		{acp.NewElicitationPropertySchemaInteger(), "integer"},
		{acp.NewElicitationPropertySchemaBoolean(), "boolean"},
		{acp.NewElicitationPropertySchemaArray(acp.MultiSelectItems{}), "array"},
		{acp.NewElicitationPropertySchemaOther("x-color"), "x-color"},
		{acp.ElicitationPropertySchema{}, ""},
	} {
		if got := tc.p.Kind(); got != tc.want {
			t.Errorf("Kind = %q, want %q", got, tc.want)
		}
	}
	s := decodeSchema(t, `{"properties":{"color":{"type":"x-color","palette":"web"},"name":{"type":"string"}},"required":["color"]}`)
	fs := s.Fields()
	if len(fs) != 2 || fs[0].Name != "color" || !fs[0].Required || fs[0].Schema.Kind() != "x-color" {
		t.Errorf("Fields of a form with an unknown property type = %+v, want color first, required, of kind x-color", fs)
	}
}

// TestElicitationDefaults types each declared default as an answer, leaves
// out a property without one and one of an unknown type, and copies a list.
func TestElicitationDefaults(t *testing.T) {
	t.Parallel()
	s := decodeSchema(t, `{"properties":{
		"name":{"type":"string","default":"Ann"},
		"ratio":{"type":"number","default":0.5},
		"count":{"type":"integer","default":9007199254740993},
		"ok":{"type":"boolean","default":false},
		"tags":{"type":"array","items":{"type":"string","enum":["a","b"]},"default":["a"]},
		"plain":{"type":"string"},
		"color":{"type":"x-color","default":"red"}}}`)
	got := s.Defaults()
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"count":9007199254740993,"name":"Ann","ok":false,"ratio":0.5,"tags":["a"]}`; string(b) != want {
		t.Errorf("Defaults = %s, want %s", b, want)
	}
	if got["count"].Integer == nil || got["ratio"].Number == nil || got["ok"].Boolean == nil || got["tags"].StringArray == nil {
		t.Errorf("Defaults' variants = %+v, want Integer, Number, Boolean and StringArray", got)
	}
	(*got["tags"].StringArray)[0] = "changed"
	if d := s.Properties["tags"].Array.Default; d[0] != "a" {
		t.Errorf("changing a default changed the schema: %q", d)
	}
	if d := (acp.ElicitationSchema{}).Defaults(); d != nil {
		t.Errorf("Defaults of a form without defaults = %v, want nil", d)
	}
}

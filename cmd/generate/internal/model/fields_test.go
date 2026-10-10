package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

func defOf(t *testing.T, js string) *load.Definition {
	t.Helper()
	var d load.Definition
	if err := json.Unmarshal([]byte(js), &d); err != nil {
		t.Fatal(err)
	}
	return &d
}

func TestTypeRefs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		def            string
		plain, nilable string
	}{
		{`{"type":"string"}`, "string", "string"},
		{`{"type":"integer"}`, "int64", "int64"},
		{`{"type":"number"}`, "float64", "float64"},
		{`{"type":"boolean"}`, "bool", "bool"},
		{`{"type":["integer","null"]}`, "int64", "*int64"},
		{`{"$ref":"#/$defs/Foo"}`, "Foo", "Foo"},
		{`{"anyOf":[{"$ref":"#/$defs/Foo"},{"type":"null"}]}`, "any", "*Foo"},
		{`{"anyOf":[{"const":"x"},{"type":"null"}]}`, "any", "*string"},
		{`{"allOf":[{"$ref":"#/$defs/Foo"}]}`, "Foo", "Foo"},
		{`{"type":"array","items":{"$ref":"#/$defs/Foo"}}`, "[]Foo", "[]Foo"},
		{`{"type":"array","items":{"type":"array","items":{"type":"string"}}}`, "[][]string", "[][]string"},
		{`{"type":"object"}`, "map[string]any", "map[string]any"},
		{`{"type":"string","enum":["a"]}`, "string", "string"},
		{`{}`, "any", "any"},
	}
	for _, tt := range tests {
		d := defOf(t, tt.def)
		if got := typeRef(d).String(); got != tt.plain {
			t.Errorf("typeRef(%s) = %s, want %s", tt.def, got, tt.plain)
		}
		if got := optionalTypeRef(d).String(); got != tt.nilable {
			t.Errorf("optionalTypeRef(%s) = %s, want %s", tt.def, got, tt.nilable)
		}
	}
	if typeRef(nil).String() != "any" || optionalTypeRef(nil).String() != "any" {
		t.Error("a nil schema is not any")
	}
}

// objectModel builds a model whose Obj covers each field rule.
func objectModel(t *testing.T) *Model {
	t.Helper()
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{
		"Foo":{"type":"string"},
		"Obj":{"type":"object","required":["name","list","ref","inner"],"properties":{
			"name":{"type":"string","description":"The name."},
			"count":{"type":["integer","null"],"format":"uint64","minimum":0,"maximum":9,"x-deserialize-default-on-error":true},
			"tags":{"type":"array","items":{"type":"string"},"default":[],"description":"Tags."},
			"list":{"type":"array","items":{"type":"string"},"x-deserialize-skip-invalid-items":true},
			"mode":{"type":"string","default":"auto","minLength":1},
			"env":{"type":"object","additionalProperties":{"type":"string"}},
			"ref":{"$ref":"#/$defs/Foo"},
			"inner":{"type":"object","description":"Inner.","required":["ids"],"properties":{
				"ids":{"type":"array","items":{"type":"string"}},
				"note":{"type":"string"}}}}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// fieldByKey returns the field of fields with JSON name key.
func fieldByKey(t *testing.T, fields []*Field, key string) *Field {
	t.Helper()
	for _, f := range fields {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("no field %q", key)
	return nil
}

func TestObjectFields(t *testing.T) {
	t.Parallel()
	obj := objectModel(t).Type("Obj")
	want := []struct {
		key, name, typ, tag string
		fill                bool
		doc                 string
	}{
		{"count", "Count", "*uint64", "count,omitempty", false, ""},
		{"env", "Env", "map[string]string", "env,omitempty", false, ""},
		{"inner", "Inner", "ObjInner", "inner", false, "Inner."},
		{"list", "List", "[]string", "list", true, ""},
		{"mode", "Mode", "string", "mode,omitempty", false, `Defaults to "auto" if unset.`},
		{"name", "Name", "string", "name", false, "The name."},
		{"ref", "Ref", "Foo", "ref", false, ""},
		{"tags", "Tags", "[]string", "tags", false, "Tags.||Defaults to [] if unset."},
	}
	if len(obj.Fields) != len(want) {
		t.Fatalf("got %d fields, want %d", len(obj.Fields), len(want))
	}
	for i, w := range want {
		f := obj.Fields[i]
		doc := strings.Join(f.Doc, "|")
		if f.Key != w.key || f.Name != w.name || f.Type.String() != w.typ || f.Tag != w.tag || f.Fill != w.fill || doc != w.doc {
			t.Errorf("field %d: got {%s %s %s %q fill=%v doc=%q}, want %+v", i, f.Key, f.Name, f.Type, f.Tag, f.Fill, doc, w)
		}
	}
}

func TestObjectFieldDefaultsAndFacts(t *testing.T) {
	t.Parallel()
	m := objectModel(t)
	fields := m.Type("Obj").Fields
	if d := fieldByKey(t, fields, "tags").Default; d == nil || d.JSON != "[]" || d.Kind != DefaultArray || !d.Nilable || d.AllowNull {
		t.Errorf("tags default: %+v", d)
	}
	if d := fieldByKey(t, fields, "mode").Default; d == nil || d.JSON != `"auto"` || d.Kind != DefaultScalar || d.Nilable {
		t.Errorf("mode default: %+v", d)
	}
	c := fieldByKey(t, fields, "count").Facts
	if c.Format != "uint64" || c.Minimum == nil || *c.Minimum != 0 || c.Maximum == nil || *c.Maximum != 9 || !c.DefaultOnError {
		t.Errorf("count facts: %+v", c)
	}
	if !fieldByKey(t, fields, "list").Facts.SkipInvalidItems {
		t.Error("list: SkipInvalidItems not recorded")
	}
	if ml := fieldByKey(t, fields, "mode").Facts.MinLength; ml == nil || *ml != 1 {
		t.Errorf("mode MinLength: %v", ml)
	}
	if v := fieldByKey(t, fields, "env").Facts.Values; v == nil || v.String() != "string" {
		t.Errorf("env additional values: %v", v)
	}
	if u := m.Type("Foo").Underlying; u == nil || u.String() != "string" {
		t.Errorf("Foo underlying: %v", u)
	}
}

func TestNestedStructs(t *testing.T) {
	t.Parallel()
	obj := objectModel(t).Type("Obj")
	if len(obj.NestedStructs) != 1 {
		t.Fatalf("nested structs: %d", len(obj.NestedStructs))
	}
	ns := obj.NestedStructs[0]
	if ns.Name != "ObjInner" || strings.Join(ns.Doc, "|") != "Inner." || len(ns.Fields) != 2 {
		t.Fatalf("nested struct: %+v", ns)
	}
	if ids, note := ns.Fields[0], ns.Fields[1]; ids.Tag != "ids" || !ids.Fill || note.Tag != "note,omitempty" || note.Fill {
		t.Errorf("nested fields: %+v %+v", ids, note)
	}
}

func TestInlineVariants(t *testing.T) {
	t.Parallel()
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{
		"Ref":{"type":"object","properties":{"a":{"type":"string"}}},
		"V":{"required":["shared"],"properties":{"shared":{"type":"string"}},"anyOf":[
			{"type":"object","description":"Object.","required":["kind"],"properties":{"kind":{"type":"string"},"opt":{"type":"array","items":{"type":"string"}}}},
			{"title":"ExtPayload","description":"An extension payload."},
			{"title":"list","type":"array","items":{"type":"string"}},
			{"title":"bare"},
			{"type":"null"},
			{"$ref":"#/$defs/Ref"}]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	vs := m.Type("V").Variants
	obj := vs[0].Inline
	if obj == nil || obj.Kind != InlineStruct || strings.Join(obj.Doc, "|") != "Object." || len(obj.Fields) != 3 {
		t.Fatalf("object variant: %+v", obj)
	}
	tags := []string{}
	for _, f := range obj.Fields {
		tags = append(tags, f.Tag)
	}
	if got := strings.Join(tags, " "); got != "kind opt,omitempty shared" {
		t.Errorf("object variant tags: %s", got)
	}
	for i, want := range []InlineKind{InlineEmpty, InlineNamed, InlineEmpty, InlineEmpty} {
		if in := vs[i+1].Inline; in == nil || in.Kind != want {
			t.Errorf("variant %d: got %+v, want kind %v", i+2, in, want)
		}
	}
	if got := vs[2].Inline.Type.String(); got != "[]string" {
		t.Errorf("array variant type: %s", got)
	}
	if vs[4].Inline.Doc != nil {
		t.Errorf("null variant has a doc: %v", vs[4].Inline.Doc)
	}
	if vs[5].Inline != nil {
		t.Errorf("$ref variant has an inline type: %+v", vs[5].Inline)
	}
}

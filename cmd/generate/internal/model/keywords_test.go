package model

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

// TestKeywordRegistryMatchesLoader requires the registry to give a treatment
// to exactly the keywords load.Definition declares.
func TestKeywordRegistryMatchesLoader(t *testing.T) {
	t.Parallel()
	var declared []string
	rt := reflect.TypeFor[load.Definition]()
	for i := range rt.NumField() {
		name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			declared = append(declared, name)
		}
	}
	slices.Sort(declared)
	if got := slices.Sorted(maps.Keys(keywords)); !slices.Equal(got, declared) {
		t.Errorf("registry:\n%v\nload.Definition:\n%v", got, declared)
	}
	for k, tr := range keywords {
		if tr == 0 {
			t.Errorf("keyword %q has no treatment", k)
		}
	}
}

// schemaChildren lists the subschemas of a raw schema node: property values,
// items, additionalProperties, not, unevaluatedProperties, and the members of
// anyOf, oneOf, and allOf. Keyword values that are not schemas are skipped.
func schemaChildren(node map[string]any) []map[string]any {
	var out []map[string]any
	add := func(v any) {
		if m, ok := v.(map[string]any); ok {
			out = append(out, m)
		}
	}
	if props, ok := node["properties"].(map[string]any); ok {
		for _, k := range slices.Sorted(maps.Keys(props)) {
			add(props[k])
		}
	}
	for _, k := range []string{"items", "additionalProperties", "not", "unevaluatedProperties"} {
		add(node[k])
	}
	for _, k := range []string{"anyOf", "oneOf", "allOf"} {
		if list, ok := node[k].([]any); ok {
			for _, v := range list {
				add(v)
			}
		}
	}
	return out
}

// TestCommittedSchemaKeywordsRegistered requires every keyword used by a
// schema node of the committed schema files to be registered, and every
// format on an integer or number node to have a Go type.
func TestCommittedSchemaKeywordsRegistered(t *testing.T) {
	t.Parallel()
	used := map[string]int{}
	for _, file := range []string{"schema.json", "schema.unstable.json"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "schema", file))
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Defs map[string]map[string]any `json:"$defs"`
		}
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatal(err)
		}
		var walk func(path string, node map[string]any)
		walk = func(path string, node map[string]any) {
			for k := range node {
				used[k]++
				if _, ok := keywords[k]; !ok {
					t.Errorf("%s %s: keyword %q is not registered", file, path, k)
				}
			}
			if f, ok := node["format"].(string); ok && numericFormat(primaryTypeOf(node), f) == "" && primaryTypeOf(node) != schemaTypeString {
				t.Errorf("%s %s: format %q has no Go type", file, path, f)
			}
			for _, c := range schemaChildren(node) {
				walk(path+".", c)
			}
		}
		for _, name := range slices.Sorted(maps.Keys(doc.Defs)) {
			walk(name, doc.Defs[name])
		}
	}
	if len(used) == 0 {
		t.Fatal("no keyword found")
	}
}

// primaryTypeOf is the first non-null type of a raw schema node.
func primaryTypeOf(node map[string]any) string {
	switch v := node["type"].(type) {
	case string:
		return v
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && s != schemaTypeNull {
				return s
			}
		}
	}
	return ""
}

func TestUnknownNumericFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, prop string
		want       string
	}{
		{"integer", `{"type":"integer","format":"int128"}`, `T.properties.n: format "int128" on integer has no Go type`},
		{"number", `{"type":["number","null"],"format":"decimal"}`, `T.properties.n: format "decimal" on number has no Go type`},
		{"array item", `{"type":"array","items":{"type":"integer","format":"u64"}}`, `T.properties.n.items: format "u64" on integer has no Go type`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{"T":{"type":"object","properties":{"n":`+tt.prop+`}}}`)})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %v, want one containing %q", err, tt.want)
			}
		})
	}
	// A string format is an annotation.
	if _, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{"T":{"type":"object","properties":{"n":{"type":"string","format":"email"}}}}`)}); err != nil {
		t.Errorf("string format refused: %v", err)
	}
}

// TestNumericFormatTypes requires each numeric format to give its Go type, to
// a required property, an optional one, an array item, a named primitive, and
// a union variant.
func TestNumericFormatTypes(t *testing.T) {
	t.Parallel()
	cases := []struct{ typ, format, want string }{
		{"integer", "", "int64"}, {"integer", "int", "int64"}, {"integer", "int8", "int8"}, {"integer", "int16", "int16"},
		{"integer", "int32", "int32"}, {"integer", "int64", "int64"}, {"integer", "uint", "uint64"}, {"integer", "uint8", "uint8"},
		{"integer", "uint16", "uint16"}, {"integer", "uint32", "uint32"}, {"integer", "uint64", "uint64"},
		{"number", "", "float64"}, {"number", "float", "float32"}, {"number", "double", "float64"},
	}
	for _, c := range cases {
		node := `{"type":"` + c.typ + `"`
		if c.format != "" {
			node += `,"format":"` + c.format + `"`
		}
		node += "}"
		m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{
			"N":`+node+`,
			"T":{"type":"object","required":["r"],"properties":{"r":`+node+`,"o":`+node+`,
				"n":{"type":["`+c.typ+`","null"]`+strings.TrimPrefix(strings.TrimSuffix(node, "}"), `{"type":"`+c.typ+`"`)+`},
				"m":{"anyOf":[`+node+`,{"type":"null"}]},
				"a":{"type":"array","items":`+node+`}}},
			"U":{"anyOf":[{"title":"num",`+strings.TrimPrefix(node, "{")+`,{"type":"string","title":"str"}]}}`)})
		if err != nil {
			t.Fatalf("%s %q: %v", c.typ, c.format, err)
		}
		got := map[string]string{}
		for _, f := range m.Type("T").Fields {
			got[f.Key] = f.Type.String()
		}
		// An optional property is a value with omitempty; only a nullable one is a pointer.
		want := map[string]string{"r": c.want, "o": c.want, "n": "*" + c.want, "m": "*" + c.want, "a": "[]" + c.want}
		for k, w := range want {
			if got[k] != w {
				t.Errorf("%s %q: property %s is %s, want %s", c.typ, c.format, k, got[k], w)
			}
		}
		if u := m.Type("N").Underlying; u == nil || u.String() != c.want {
			t.Errorf("%s %q: named primitive is %v, want %s", c.typ, c.format, u, c.want)
		}
		if in := m.Type("U").Variants[0].Inline; in == nil || in.Type.String() != c.want {
			t.Errorf("%s %q: variant is %+v, want %s", c.typ, c.format, in, c.want)
		}
	}
}

// structValueFields lists, as owner.key, every field of m tagged omitempty
// whose Go type is a struct value: a named object, a union, or a nested struct.
func structValueFields(m *Model) []string {
	nested := map[string]bool{}
	for _, ty := range m.Types {
		for _, ns := range ty.NestedStructs {
			nested[ns.Name] = true
		}
	}
	var out []string
	visit := func(owner string, fields []*Field) {
		for _, f := range fields {
			if f.Type.Kind != RefNamed || !strings.HasSuffix(f.Tag, ",omitempty") {
				continue
			}
			if ty := m.Type(f.Type.Name); (ty != nil && (ty.Kind == KindObject || ty.Kind == KindUnion)) || nested[f.Type.Name] {
				out = append(out, owner+"."+f.Key)
			}
		}
	}
	for _, ty := range m.Types {
		visit(ty.Name, ty.Fields)
		for _, ns := range ty.NestedStructs {
			visit(ns.Name, ns.Fields)
		}
		for _, v := range ty.Variants {
			if v.Inline != nil {
				visit(v.TypeName, v.Inline.Fields)
			}
		}
	}
	return out
}

// TestNoOmitemptyOnStructValues requires no struct-valued field to carry
// omitempty, which encoding/json never applies to a struct.
func TestNoOmitemptyOnStructValues(t *testing.T) {
	t.Parallel()
	if got := structValueFields(committedModel(t)); len(got) != 0 {
		t.Errorf("struct-valued fields with omitempty: %v", got)
	}
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{
		"Obj":{"type":"object","properties":{"a":{"type":"string"}}},
		"Uni":{"oneOf":[{"type":"string"},{"type":"integer"}]},
		"T":{"type":"object","properties":{
			"obj":{"allOf":[{"$ref":"#/$defs/Obj"}]},
			"uni":{"allOf":[{"$ref":"#/$defs/Uni"}]},
			"inner":{"type":"object","properties":{"b":{"type":"string"}}},
			"s":{"type":"string"}}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if got := structValueFields(m); len(got) != 0 {
		t.Errorf("synthetic struct-valued fields with omitempty: %v", got)
	}
	for _, f := range m.Type("T").Fields {
		if f.Key == "s" && f.Tag != "s,omitempty" {
			t.Errorf("optional string tag %q", f.Tag)
		}
	}
}

// TestSkipInvalidItemsNeedsAnArray requires x-deserialize-skip-invalid-items
// on a node that is not an array to be refused.
func TestSkipInvalidItemsNeedsAnArray(t *testing.T) {
	t.Parallel()
	_, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{"T":{"type":"object","properties":{
		"n":{"type":"string","x-deserialize-skip-invalid-items":true}}}}`)})
	if want := `T.properties.n: x-deserialize-skip-invalid-items on string, not an array`; err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("error %v, want one containing %q", err, want)
	}
	if _, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{"T":{"type":"object","properties":{
		"n":{"type":["array","null"],"items":{"type":"string"},"x-deserialize-skip-invalid-items":true}}}}`)}); err != nil {
		t.Errorf("nullable array refused: %v", err)
	}
}

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

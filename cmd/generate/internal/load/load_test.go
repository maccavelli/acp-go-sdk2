package load

import (
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestParseSchemaRejectsUnknownKeywords(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, schema string
		want         []string
	}{
		{
			name:   "top level of a definition",
			schema: `{"$defs":{"A":{"type":"object"},"B":{"type":"string","x-new":1}}}`,
			want:   []string{`definition "B"`, `unknown field "x-new"`},
		},
		{
			name:   "nested in a property",
			schema: `{"$defs":{"C":{"type":"object","properties":{"p":{"type":"string","x-nested":true}}}}}`,
			want:   []string{`definition "C"`, `unknown field "x-nested"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseSchema([]byte(tt.schema))
			if err == nil {
				t.Fatal("parsed a schema with an unknown keyword")
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}

func TestParseSchemaKeepsModeledKeywords(t *testing.T) {
	t.Parallel()
	s, err := parseSchema([]byte(`{"$schema":"x","$defs":{"N":{"type":"integer","format":"uint64","minimum":0,"maximum":9,
		"x-deserialize-default-on-error":true},"M":{"type":"object","additionalProperties":{"type":"string"},
		"x-deserialize-skip-invalid-items":true,"not":{"required":["a"]},"unevaluatedProperties":false},"S":{"type":"string","minLength":1},"Z":null}}`))
	if err != nil {
		t.Fatal(err)
	}
	n, m := s.Defs["N"], s.Defs["M"]
	if n.Format != "uint64" || n.Minimum == nil || *n.Minimum != 0 || n.Maximum == nil || *n.Maximum != 9 || !n.XDeserializeDefaultOnError {
		t.Errorf("N decoded as %+v", n)
	}
	if m.AdditionalProperties == nil || m.AdditionalProperties.Type != "string" || !m.XDeserializeSkipInvalidItems ||
		m.Not == nil || m.UnevaluatedProperties == nil {
		t.Errorf("M decoded as %+v", m)
	}
	if ml := s.Defs["S"].MinLength; ml == nil || *ml != 1 {
		t.Errorf("S.MinLength = %v", ml)
	}
	if z, ok := s.Defs["Z"]; !ok || z != nil {
		t.Errorf("a null definition decoded as %v, present %v", z, ok)
	}
}

func TestParseMetaRejectsUnknownKeys(t *testing.T) {
	t.Parallel()
	_, err := parseMeta([]byte(`{"version":1,"agentMethods":{},"clientMethods":{},"protocolMethods":{},"extra":1}`))
	if err == nil || !strings.Contains(err.Error(), `unknown field "extra"`) {
		t.Fatalf("got %v, want an unknown field error naming extra", err)
	}
}

// TestKeywordsMatchExactly requires every decoder of the loader to refuse a
// key that names a keyword in another letter case, which encoding/json would
// match as the keyword (0019-PLAN-generator-hardening.md, G13).
func TestKeywordsMatchExactly(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, input string
		parse       func([]byte) error
		want        string
	}{
		{"definition", `{"$defs":{"T":{"type":"object","Properties":{}}}}`, schemaParse, `unknown field "Properties"`},
		{"nested", `{"$defs":{"T":{"type":"object","properties":{"a":{"TYPE":"string"}}}}}`, schemaParse, `unknown field "TYPE"`},
		{"discriminator", `{"$defs":{"T":{"discriminator":{"PropertyName":"kind"},"oneOf":[]}}}`, schemaParse,
			`unknown field "PropertyName"`},
		{"top level", `{"$DEFS":{}}`, schemaParse, `unknown field "$DEFS"`},
		{"meta", `{"version":1,"AgentMethods":{},"clientMethods":{}}`, metaParse, `unknown field "AgentMethods"`},
	} {
		if err := tt.parse([]byte(tt.input)); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: got %v, want an error containing %q", tt.name, err, tt.want)
		}
	}
	if err := schemaParse([]byte(`{"$defs":{"T":{"type":"object","properties":{"a":{"type":"string"}},"required":["a"]}}}`)); err != nil {
		t.Errorf("exact keywords: %v", err)
	}
}

func schemaParse(b []byte) error { _, err := parseSchema(b); return err }

func metaParse(b []byte) error { _, err := parseMeta(b); return err }

// TestDeepCopyDefinitionCoversEveryField fails when a field of Definition is
// left out of the fixture, or when deepCopyDefinition shares a pointer, map,
// or slice field with its input.
func TestDeepCopyDefinitionCoversEveryField(t *testing.T) {
	t.Parallel()
	lo, hi, n, b := 0.0, 9.0, 1, true
	child := func() *Definition { return &Definition{Type: "string"} }
	orig := &Definition{
		Description:                  "d",
		Type:                         []any{"string", "null"},
		Properties:                   map[string]*Definition{"p": child()},
		Required:                     []string{"p"},
		Enum:                         []any{"a"},
		Items:                        child(),
		Ref:                          "#/$defs/X",
		AnyOf:                        []*Definition{child()},
		OneOf:                        []*Definition{child()},
		AllOf:                        []*Definition{child()},
		DocsIgnore:                   true,
		Title:                        "t",
		Const:                        "c",
		XSide:                        "agent",
		XMethod:                      "m",
		Default:                      "x",
		Discriminator:                &Discriminator{PropertyName: "type"},
		Format:                       "uint64",
		Minimum:                      &lo,
		Maximum:                      &hi,
		MinLength:                    &n,
		AdditionalProperties:         child(),
		Not:                          child(),
		UnevaluatedProperties:        child(),
		XDeserializeDefaultOnError:   true,
		XDeserializeSkipInvalidItems: true,
		Deprecated:                   true,
		boolSchema:                   &b,
	}
	ov := reflect.ValueOf(orig).Elem()
	for i := range ov.NumField() {
		if ov.Field(i).IsZero() {
			t.Errorf("the fixture leaves %s zero; set it so its copy is checked", ov.Type().Field(i).Name)
		}
	}
	cp := deepCopyDefinition(orig)
	if !reflect.DeepEqual(orig, cp) {
		t.Fatal("the copy differs from the original")
	}
	cv := reflect.ValueOf(cp).Elem()
	for i := range ov.NumField() {
		switch ov.Field(i).Kind() {
		case reflect.Pointer, reflect.Map, reflect.Slice:
			if ov.Field(i).Pointer() == cv.Field(i).Pointer() {
				t.Errorf("%s is shared between the original and the copy", ov.Type().Field(i).Name)
			}
		}
	}
}

func TestMergeProvenanceMatchesInputs(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "..", "..", "..", "schema")
	sm, err := ReadMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	ss, err := ReadSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	um, _, err := ReadMetaUnstable(dir)
	if err != nil {
		t.Fatal(err)
	}
	us, _, err := ReadSchemaUnstable(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, merged, prov, err := MergeStableAndUnstable(sm, ss, um, us)
	if err != nil {
		t.Fatal(err)
	}

	wantDefs := map[string]bool{}
	for name := range merged.Defs {
		if _, ok := ss.Defs[name]; !ok {
			wantDefs[name] = true
		}
	}
	stable := map[string]bool{}
	for _, m := range []map[string]string{sm.AgentMethods, sm.ClientMethods, sm.ProtocolMethods} {
		for _, w := range m {
			stable[w] = true
		}
	}
	wantWires := map[string]bool{}
	for _, m := range []map[string]string{um.AgentMethods, um.ClientMethods, um.ProtocolMethods} {
		for _, w := range m {
			if !stable[w] {
				wantWires[w] = true
			}
		}
	}
	if !maps.Equal(prov.UnstableDefs, wantDefs) {
		t.Errorf("UnstableDefs: got %d, want %d; extra %v", len(prov.UnstableDefs), len(wantDefs), diff(prov.UnstableDefs, wantDefs))
	}
	if !maps.Equal(prov.UnstableWires, wantWires) {
		t.Errorf("UnstableWires: got %v, want %v", slices.Sorted(maps.Keys(prov.UnstableWires)), slices.Sorted(maps.Keys(wantWires)))
	}
	if prov.UnstableWires["$/cancel_request"] {
		t.Error("$/cancel_request is a stable protocol method")
	}
	t.Logf("unstable definitions: %d; unstable wires: %d", len(prov.UnstableDefs), len(prov.UnstableWires))
}

// diff returns the keys of a that b lacks, sorted.
func diff(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

package model

import (
	"encoding/json"
	"fmt"
	"go/token"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/util"
)

// schemaOf decodes a JSON object of definitions into a schema.
func schemaOf(t *testing.T, defs string) *load.Schema {
	t.Helper()
	var s load.Schema
	if err := json.Unmarshal([]byte(`{"$defs":`+defs+`}`), &s); err != nil {
		t.Fatal(err)
	}
	return &s
}

func TestBuildRefuses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		meta *load.Meta
		defs string
		want []string
	}{
		{
			name: "enum constants collide",
			defs: `{"Kind":{"type":"string","enum":["a-b","a_b"]}}`,
			want: []string{`Kind enum value "a_b"`, "KindAB is already declared", `Kind enum value "a-b"`},
		},
		{
			name: "enum constant collides with a type",
			defs: `{"Kind":{"type":"string","enum":["a"]},"KindA":{"type":"string"}}`,
			want: []string{`Kind enum value "a"`, "KindA is already declared by definition KindA"},
		},
		{
			name: "struct fields collide",
			defs: `{"F":{"type":"object","properties":{"foo_bar":{"type":"string"},"fooBar":{"type":"string"}}}}`,
			want: []string{"F:", `"fooBar" and "foo_bar" both become field FooBar`},
		},
		{
			name: "field name is not an identifier",
			defs: `{"Thing":{"type":"object","properties":{"2fa":{"type":"string"}},"required":["2fa"]}}`,
			want: []string{"Thing.properties.2fa", `"2fa" is not an exported Go identifier`},
		},
		{
			name: "dangling reference",
			defs: `{"R":{"type":"object","properties":{"m":{"$ref":"#/$defs/Missing"}},"required":["m"]}}`,
			want: []string{"R.properties.m", `"#/$defs/Missing" names no definition`},
		},
		{
			name: "non-local reference",
			defs: `{"R":{"type":"object","properties":{"m":{"$ref":"https://example.com/s.json"}}}}`,
			want: []string{"R.properties.m", "is not a reference into $defs"},
		},
		{
			name: "null property schema",
			defs: `{"N":{"type":"object","properties":{"p":null}}}`,
			want: []string{"N.properties.p", "property schema is null"},
		},
		{
			name: "two constructor parameters with one name",
			defs: `{"Union":{"oneOf":[
				{"type":"object","properties":{"kind":{"const":"x","type":"string"},"type":{"type":"string"},"typeValue":{"type":"string"}},"required":["kind","type","typeValue"]},
				{"type":"object","properties":{"kind":{"const":"y","type":"string"}},"required":["kind"]}]}}`,
			want: []string{"Union variant 1", "parameters type and typeValue are both named typeValue"},
		},
		{
			name: "union wrapper fields collide",
			defs: `{"Pick":{"anyOf":[
				{"title":"foo","type":"object","properties":{"a":{"type":"string"}}},
				{"title":"Foo","type":"object","properties":{"b":{"type":"string"}}}]}}`,
			want: []string{"Pick variant 2", "field Foo is already used by variant 1"},
		},
		{
			name: "definition takes a name the generator declares",
			defs: `{"Agent":{"type":"string"}}`,
			want: []string{"the generator", "Agent is already declared by definition Agent"},
		},
		{
			name: "method constants collide",
			meta: &load.Meta{AgentMethods: map[string]string{"a_b": "w/1", "aB": "w/2"}},
			defs: `{}`,
			want: []string{"AgentMethodAB is already declared"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			meta := tt.meta
			if meta == nil {
				meta = &load.Meta{}
			}
			_, err := Build(Input{Meta: meta, Schema: schemaOf(t, tt.defs)})
			if err == nil {
				t.Fatal("Build accepted the schema")
			}
			if !strings.HasPrefix(err.Error(), "model: ") {
				t.Errorf("error %q does not start with model:", err)
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}

func TestBuildNames(t *testing.T) {
	t.Parallel()
	schema := schemaOf(t, `{
		"U":{"oneOf":[
			{"type":"object","properties":{"type":{"const":"x","type":"string"},"n":{"type":"string"}},"required":["type","n"]},
			{"type":"object","properties":{"type":{"const":"y","type":"string"}},"required":["type"]}]},
		"Parent":{"type":"object","properties":{"URLValue":{"type":"object","properties":{"a":{"type":"string"}}}}},
		"Kind":{"type":"string","enum":["one_two"]},
		"NewSessionRequest":{"type":"object","x-side":"agent","x-method":"session/new"},
		"NewSessionResponse":{"type":"object","x-side":"agent","x-method":"session/new"}}`)
	m, err := Build(Input{Meta: &load.Meta{AgentMethods: map[string]string{"session_new": "session/new"}}, Schema: schema})
	if err != nil {
		t.Fatal(err)
	}

	u := m.Type("U")
	if u.Kind != KindUnion || !u.ExactlyOne {
		t.Fatalf("U: kind %v exactly-one %v", u.Kind, u.ExactlyOne)
	}
	for i, want := range [][3]string{{"UX", "X", "x"}, {"UY", "Y", "y"}} {
		v := u.Variants[i]
		if got := [3]string{v.TypeName, v.FieldName, v.DiscValue}; got != want {
			t.Errorf("U variant %d: got type, field, value %q, want %q", i+1, got, want)
		}
	}
	if len(u.Helpers) != 2 || u.Helpers[0].Name != "NewUX" || len(u.Helpers[0].Params) != 1 || len(u.Helpers[1].Params) != 0 ||
		u.Helpers[0].Params[0] != (Param{Key: "n", Name: "n", Field: "N", Type: named("string")}) {
		t.Errorf("U helpers: %+v %+v", u.Helpers[0], u.Helpers[1])
	}
	if got := m.Type("Parent").Nested["URLValue"]; got != "ParentURLValue" {
		t.Errorf("Parent.URLValue nested type: got %q, want ParentURLValue", got)
	}
	if got := m.Type("Kind").Enum; len(got) != 1 || got[0] != (EnumValue{Value: "one_two", Const: "KindOneTwo"}) {
		t.Errorf("Kind enum: %+v", got)
	}
	if got := m.AgentMethodConsts["session_new"]; got != "AgentMethodSessionNew" {
		t.Errorf("method constant: got %q", got)
	}
}

func TestClassifyOrder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		def  string
		want Kind
	}{
		{`{"type":"string","enum":["a"]}`, KindEnum},
		{`{"oneOf":[{"const":"a"},{"const":"b"}]}`, KindStringConstUnion},
		{`{"anyOf":[{"type":"string","const":"a"},{"type":"string"}]}`, KindOpenEnum},
		{`{"anyOf":[{"type":"string"},{"type":"integer"}]}`, KindUnion},
		{`{"oneOf":[{"type":"string"},{"type":"integer"}]}`, KindUnion},
		{`{"type":"object","properties":{"a":{"type":"string"}}}`, KindObject},
		{`{"type":["integer","null"]}`, KindPrimitive},
		{`{"type":"object"}`, KindEmptyObject},
		{`{"type":"array","items":{"type":"string"}}`, KindAny},
		{`true`, KindAny},
	}
	for _, tt := range tests {
		var d load.Definition
		if err := json.Unmarshal([]byte(tt.def), &d); err != nil {
			t.Fatal(err)
		}
		if got := classify(&d); got != tt.want {
			t.Errorf("classify(%s) = %v, want %v", tt.def, got, tt.want)
		}
	}
	if got := classify(nil); got != KindNone {
		t.Errorf("classify(nil) = %v", got)
	}
}

// FuzzExportedName checks that a field name the model accepts is an exported
// identifier that a second pass leaves unchanged.
func FuzzExportedName(f *testing.F) {
	for _, s := range []string{"UX", "URLValue", "mcpServerHTTP", "aBC", "foo_bar", "a-b", "2fa", "_meta", "", "ü", "x1y2"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		name, err := fieldName("T", raw)
		if err != nil {
			return
		}
		if !token.IsIdentifier(name) || !token.IsExported(name) {
			t.Fatalf("fieldName(%q) = %q, not an exported identifier", raw, name)
		}
		if again := util.ToExportedField(name); again != name {
			t.Fatalf("fieldName(%q) = %q, but a second pass gives %q", raw, name, again)
		}
	})
}

// TestNestedNameExhausted requires a nested struct or an inline variant whose
// every candidate name is taken, up to the thousandth, to be refused by name.
func TestNestedNameExhausted(t *testing.T) {
	t.Parallel()
	taken := func(base string, defs map[string]*load.Definition) {
		defs[base] = &load.Definition{Type: "string"}
		defs[base+"Inline"] = &load.Definition{Type: "string"}
		for i := 2; i < 1000; i++ {
			defs[fmt.Sprintf("%sInline%d", base, i)] = &load.Definition{Type: "string"}
		}
	}
	inline := &load.Definition{Type: "object", Properties: map[string]*load.Definition{"x": {Type: "string"}}}
	tests := []struct {
		name string
		defs map[string]*load.Definition
		want string
	}{
		{"nested struct", map[string]*load.Definition{"Outer": {Type: "object", Properties: map[string]*load.Definition{"inner": inline}}},
			`Outer.properties.inner: Outer: no unique nested type name for "inner"`},
		{"inline variant", map[string]*load.Definition{"U": {AnyOf: []*load.Definition{
			{Title: "Box", Type: "object", Properties: map[string]*load.Definition{"x": {Type: "string"}}},
			{Title: "Bag", Type: "object", Properties: map[string]*load.Definition{"y": {Type: "string"}}}}}},
			`U: no unique nested type name for "Box"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			base := map[string]string{"nested struct": "OuterInner", "inline variant": "UBox"}[tt.name]
			taken(base, tt.defs)
			_, err := Build(Input{Meta: &load.Meta{}, Schema: &load.Schema{Defs: tt.defs}})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want one containing %s", err, tt.want)
			}
		})
	}
}

// TestSideOf pins the side of every binding. Build reaches it only for a
// rule's binding, which must be an optional agent interface.
func TestSideOf(t *testing.T) {
	t.Parallel()
	for b, want := range map[Binding]string{BindAgent: "agent", BindAgentOptional: "agent", BindAgentExperimental: "agent",
		BindClient: "client", BindClientExperimental: "client", BindUnknown: ""} {
		if got := sideOf(b); got != want {
			t.Errorf("sideOf(%v) = %q, want %q", b, got, want)
		}
	}
}

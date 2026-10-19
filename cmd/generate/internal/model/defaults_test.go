package model

import (
	"reflect"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

// TestRenderDefaults requires each case of the default renderer to give the
// Go value decoding the default gives: nothing for a zero value, a literal,
// a constant, a composite of written-out items, new of a pointer's value,
// and a raw value's JSON (0019-PLAN-idioms-performance.md, D3). A null
// default is no default, so null is tested as an item.
func TestRenderDefaults(t *testing.T) {
	t.Parallel()
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{
		"Mode":{"type":"string","enum":["auto","manual"]},
		"Inner":{"type":"object","properties":{"x":{"type":"boolean","default":false},"n":{"type":["integer","null"],"format":"int32"}}},
		"Opts":{"type":"object","properties":{
			"off":{"type":"boolean","default":false},
			"on":{"type":"boolean","default":true},
			"empty":{"type":"string","default":""},
			"name":{"type":"string","default":"auto"},
			"zeroN":{"type":"integer","format":"int32","default":0},
			"n":{"type":"integer","format":"int32","default":3},
			"f":{"type":"number","format":"double","default":1.5},
			"mode":{"allOf":[{"$ref":"#/$defs/Mode"}],"default":"manual"},
			"inner":{"allOf":[{"$ref":"#/$defs/Inner"}],"default":{"n":null}},
			"labels":{"type":"object","additionalProperties":{"type":"string"},"default":{}},
			"pairs":{"type":"object","additionalProperties":{"type":"boolean"},"default":{"b":false,"a":true}},
			"list":{"type":"array","items":{"type":"string"},"default":[]},
			"names":{"type":"array","items":{"type":"string"},"default":["a",""]},
			"inners":{"type":"array","items":{"$ref":"#/$defs/Inner"},"default":[{}]},
			"title":{"type":["string","null"],"default":"x"},
			"count":{"type":["integer","null"],"format":"int32","default":3},
			"modeP":{"anyOf":[{"$ref":"#/$defs/Mode"},{"type":"null"}],"default":"auto"},
			"raw":{"description":"Anything.","default":{"a":1}},
			"raws":{"type":"array","items":{"description":"Anything."},"default":[null,1]}}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	str := func(s string) Value { return Value{Kind: ValueString, Text: s} }
	for _, tt := range []struct {
		key  string
		want Value
	}{
		{"off", Value{}},
		{"on", Value{Kind: ValueBool, Text: "true"}},
		{"empty", Value{}},
		{"name", str(`"auto"`)},
		{"zeroN", Value{}},
		{"n", Value{Kind: ValueNumber, Text: "3"}},
		{"f", Value{Kind: ValueNumber, Text: "1.5"}},
		{"mode", Value{Kind: ValueConst, Text: "ModeManual"}},
		{"inner", Value{}},
		{"labels", Value{Kind: ValueMap, Type: wrap(RefMap, named("string"))}},
		{"pairs", Value{Kind: ValueMap, Type: wrap(RefMap, named(goBool)), Keys: []string{"a", "b"},
			Items: []Value{{Kind: ValueBool, Text: "true"}, {Kind: ValueBool, Text: "false"}}}},
		{"list", Value{Kind: ValueSlice, Type: wrap(RefSlice, named("string"))}},
		{"names", Value{Kind: ValueSlice, Type: wrap(RefSlice, named("string")), Items: []Value{str(`"a"`), str(`""`)}}},
		{"inners", Value{Kind: ValueSlice, Type: wrap(RefSlice, named("Inner")), Items: []Value{{Kind: ValueStruct}}}},
		{"title", Value{Kind: ValuePointer, Type: named("string"), Items: []Value{str(`"x"`)}}},
		{"count", Value{Kind: ValuePointer, Type: named(goInt32), Items: []Value{{Kind: ValueNumber, Text: "3"}}}},
		{"modeP", Value{Kind: ValuePointer, Type: named("Mode"), Items: []Value{{Kind: ValueConst, Text: "ModeAuto"}}}},
		{"raw", Value{Kind: ValueRaw, Type: rawJSON(), Text: `{"a":1}`}},
		{"raws", Value{Kind: ValueSlice, Type: wrap(RefSlice, rawJSON()),
			Items: []Value{{Kind: ValueRaw, Type: rawJSON(), Text: "null"}, {Kind: ValueRaw, Type: rawJSON(), Text: "1"}}}},
	} {
		f := fieldByKey(t, m.Type("Opts").Fields, tt.key)
		if f.Default == nil {
			t.Errorf("%s: no default", tt.key)
			continue
		}
		if got := f.Default.Value; !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: default %s renders %+v, want %+v", tt.key, f.Default.JSON, got, tt.want)
		}
	}
}

// TestRenderDefaultsRefuses requires a default no rule renders to be refused,
// naming the definition and the property.
func TestRenderDefaultsRefuses(t *testing.T) {
	t.Parallel()
	const defs = `"U":{"oneOf":[{"title":"S","type":"string"},{"title":"I","type":"integer"}]},
		"Inner":{"type":"object","required":["id"],"properties":{"id":{"type":"string"},"x":{"type":"boolean","default":false}}},
		"Loose":{"type":"object","properties":{"x":{"type":"boolean","default":true}}},`
	for _, tt := range []struct{ name, prop, want string }{
		{"union", `{"allOf":[{"$ref":"#/$defs/U"}],"default":"s"}`, `Opts.p: default "s": no Go value of U, a union, is rendered`},
		{"union item", `{"type":"array","items":{"$ref":"#/$defs/U"},"default":["s"]}`, `Opts.p: default ["s"]: item 0: no Go value of U, a union, is rendered`},
		{"non-zero struct", `{"allOf":[{"$ref":"#/$defs/Inner"}],"default":{"id":"a"}}`, "Opts.p: default {\"id\":\"a\"}: Inner.id is not the zero struct"},
		{"a field's non-zero default", `{"allOf":[{"$ref":"#/$defs/Loose"}],"default":{}}`, "Opts.p: default {}: Loose.x is not the zero struct"},
		{"no required key", `{"allOf":[{"$ref":"#/$defs/Inner"}],"default":{"x":false}}`, `Opts.p: default {"x":false}: Inner lacks the required key "id"`},
	} {
		_, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{`+defs+`"Opts":{"type":"object","properties":{"p":`+tt.prop+`}}}`)})
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error %v, want one containing %q", tt.name, err, tt.want)
		}
	}
}

package load

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"testing"
)

// TestMergeChanges requires Provenance.Changes to name, for each stable
// definition the merge replaced, the properties the unstable version adds and
// changes and the union entries it adds and changes, by index in its list; and
// nothing for a definition the merge kept or one only the unstable schema has.
func TestMergeChanges(t *testing.T) {
	t.Parallel()
	str := func() *Definition { return &Definition{Type: "string"} }
	tagged := func(key, value, desc string) *Definition {
		return &Definition{Type: "object", Description: desc, Properties: map[string]*Definition{key: {Type: "string", Const: value}}}
	}
	root := func() map[string]*Definition {
		return map[string]*Definition{
			"OldRequest":  {Type: "object", XSide: "agent", XMethod: "x/old", Properties: map[string]*Definition{"c": ref("Caps"), "u": ref("Update"), "s": ref("Server"), "k": ref("Kept")}},
			"OldResponse": {Type: "object", XSide: "agent", XMethod: "x/old"},
			"Kept":        str(),
		}
	}
	stableDefs := root()
	stableDefs["Caps"] = &Definition{Type: "object", Properties: map[string]*Definition{"a": str(), "b": {Type: "string", Default: "x"}}}
	stableDefs["Update"] = &Definition{Discriminator: &Discriminator{PropertyName: "kind"}, OneOf: []*Definition{
		tagged("kind", "one", ""), tagged("kind", "two", ""), {Title: "three", Type: "string"},
	}}
	stableDefs["Server"] = &Definition{AnyOf: []*Definition{ref("Kept"), {Title: "x", Type: "integer"}}}
	unstableDefs := root()
	unstableDefs["Caps"] = &Definition{Type: "object", Properties: map[string]*Definition{"a": str(), "b": {Type: "string", Default: "y"}, "n": ref("Fresh")}}
	unstableDefs["Update"] = &Definition{Discriminator: &Discriminator{PropertyName: "kind"}, OneOf: []*Definition{
		tagged("kind", "one", ""), tagged("kind", "new", ""), tagged("kind", "two", "Changed."), {Title: "three", Type: "string"}, {Type: "null"},
	}}
	unstableDefs["Server"] = &Definition{AnyOf: []*Definition{ref("Fresh"), ref("Kept"), {Title: "x", Type: "number"}}}
	unstableDefs["Fresh"] = str()

	meta := &Meta{Version: 1, AgentMethods: map[string]string{"old": "x/old"}}
	_, _, prov, err := MergeStableAndUnstable(meta, &Schema{Defs: stableDefs}, meta, &Schema{Defs: unstableDefs})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]*Changes{
		"Caps":   {AddedProperties: []string{"n"}, ChangedProperties: []string{"b"}},
		"Update": {AddedEntries: []int{1, 4}, ChangedEntries: []int{2}},
		"Server": {AddedEntries: []int{0}, ChangedEntries: []int{2}},
	}
	show := func(c *Changes) string {
		if c == nil {
			return "none"
		}
		return fmt.Sprintf("%+v", *c)
	}
	names := slices.Sorted(maps.Keys(want))
	for name := range prov.Changes {
		if want[name] == nil {
			names = append(names, name)
		}
	}
	for _, name := range names {
		if got := prov.Changes[name]; !reflect.DeepEqual(got, want[name]) {
			t.Errorf("%s: Changes %s, want %s", name, show(got), show(want[name]))
		}
	}
	if !prov.UnstableDefs["Fresh"] {
		t.Errorf("Fresh is not an unstable definition: %v", prov.UnstableDefs)
	}
}

// TestUnionEntryKey requires an entry to be named by its declared
// discriminator's value, else its const-valued properties, its title, its
// reference or its allOf references, in that order, and "" by none.
func TestUnionEntryKey(t *testing.T) {
	t.Parallel()
	consts := &Definition{Title: "t", Properties: map[string]*Definition{"kind": {Const: "a"}, "other": {Const: "z"}, "free": {Type: "string"}}}
	for i, tt := range []struct {
		entry *Definition
		disc  string
		want  string
	}{
		{consts, "kind", "const kind=a"},
		{consts, "", "const kind=a,other=z"},
		{consts, "missing", "const kind=a,other=z"},
		{&Definition{Title: "t", Ref: "#/$defs/R"}, "", "title t"},
		{ref("R"), "", "ref #/$defs/R"},
		{&Definition{AllOf: []*Definition{ref("A"), {Type: "object"}, ref("B")}}, "", "allOf #/$defs/A,#/$defs/B"},
		{&Definition{Type: "null"}, "", ""},
		{nil, "kind", ""},
	} {
		if got := UnionEntryKey(tt.entry, tt.disc); got != tt.want {
			t.Errorf("case %d: UnionEntryKey(entry, %q) = %q, want %q", i, tt.disc, got, tt.want)
		}
	}
}

package load

import (
	"maps"
	"testing"
)

// TestMergeOrigins requires Provenance.Origins to name, for each merged
// definition that is not the stable one as it is, the file and key it was
// copied from: a stable definition the unstable schema changed, a definition
// only the unstable schema has, and a copy renamed with the Unstable prefix.
func TestMergeOrigins(t *testing.T) {
	t.Parallel()
	str := func() *Definition { return &Definition{Type: "string"} }
	obj := func(props map[string]*Definition) *Definition { return &Definition{Type: "object", Properties: props} }
	stableMeta := &Meta{Version: 1, AgentMethods: map[string]string{"old": "x/old"}}
	stableSchema := &Schema{Defs: map[string]*Definition{
		"OldRequest":  {Type: "object", XSide: "agent", XMethod: "x/old", Properties: map[string]*Definition{"s": ref("Shared")}},
		"OldResponse": {Type: "object", XSide: "agent", XMethod: "x/old"},
		"Shared":      obj(map[string]*Definition{"a": str()}),
		"Kept":        str(),
	}}
	unstableMeta := &Meta{Version: 1, AgentMethods: map[string]string{"old": "x/old", "new": "x/new"}}
	unstableSchema := &Schema{Defs: map[string]*Definition{
		"OldRequest":  {Type: "object", XSide: "agent", XMethod: "x/old", Properties: map[string]*Definition{"s": ref("Shared")}},
		"OldResponse": {Type: "object", XSide: "agent", XMethod: "x/old"},
		"Shared":      obj(map[string]*Definition{"a": str(), "b": str()}),
		"Kept":        str(),
		"NewRequest":  {Type: "object", XSide: "agent", XMethod: "x/new", Properties: map[string]*Definition{"e": ref("Extra")}},
		"NewResponse": {Type: "object", XSide: "agent", XMethod: "x/new"},
		"Extra":       obj(map[string]*Definition{"z": str()}),
	}}
	_, schema, prov, err := MergeStableAndUnstable(stableMeta, stableSchema, unstableMeta, unstableSchema)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Origin{
		"Shared":              {File: UnstableSchemaFile, Key: "Shared"},
		"UnstableNewRequest":  {File: UnstableSchemaFile, Key: "NewRequest"},
		"UnstableNewResponse": {File: UnstableSchemaFile, Key: "NewResponse"},
		"UnstableExtra":       {File: UnstableSchemaFile, Key: "Extra"},
	}
	if !maps.Equal(prov.Origins, want) {
		t.Errorf("Origins = %v, want %v", prov.Origins, want)
	}
	for name := range schema.Defs {
		if _, ok := want[name]; !ok && schema.Defs[name] != stableSchema.Defs[name] {
			t.Errorf("%s has no origin but is not the stable definition", name)
		}
	}
}

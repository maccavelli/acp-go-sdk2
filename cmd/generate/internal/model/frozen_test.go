package model

import (
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

// TestFrozenDeprecations requires the frozen mark on what the manifest
// keeps and on nothing else: a type from the live unstable schema has none;
// a mark the schema gives is kept as the schema's; and an interface whose
// methods are not all frozen has none, though its frozen method does
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 1).
func TestFrozenDeprecations(t *testing.T) {
	t.Parallel()
	manifest := func(key string) load.Origin { return load.Origin{File: load.ManifestFile, Key: key} }
	prov := &load.Provenance{
		UnstableDefs: map[string]bool{"Frozen": true, "Marked": true, "Live": true, "NewRequest": true, "NewResponse": true,
			"LiveRequest": true, "LiveResponse": true},
		UnstableWires: map[string]bool{"x/new": true, "x/live": true},
		Origins: map[string]load.Origin{"Frozen": manifest("Frozen"), "Marked": manifest("Marked"),
			"Live": {File: load.UnstableSchemaFile, Key: "Live"}, "NewRequest": manifest("NewRequest"),
			"NewResponse": manifest("NewResponse")},
	}
	m, err := Build(Input{
		Meta: &load.Meta{AgentMethods: map[string]string{"new": "x/new", "live": "x/live"}},
		Schema: schemaOf(t, `{
			"Frozen":{"type":"string"},
			"Marked":{"type":"string","deprecated":true},
			"Live":{"type":"string"},
			"NewRequest":{"type":"object","x-side":"agent","x-method":"x/new"},
			"NewResponse":{"type":"object","x-side":"agent","x-method":"x/new"},
			"LiveRequest":{"type":"object","x-side":"agent","x-method":"x/live"},
			"LiveResponse":{"type":"object","x-side":"agent","x-method":"x/live"}}`),
		Provenance: prov,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name         string
		mark, frozen bool
	}{
		{"Frozen", true, true},
		{"Marked", true, false},
		{"Live", false, false},
		{"LiveRequest", false, false},
	} {
		d := m.Type(tt.name).Deprecation
		if (d != nil) != tt.mark || (d != nil && d.Frozen != tt.frozen) {
			t.Errorf("%s: mark %+v, want mark %v frozen %v", tt.name, d, tt.mark, tt.frozen)
		}
	}
	for _, mi := range m.AgentMethods {
		if want := mi.Wire == "x/new"; (mi.Deprecation != nil && mi.Deprecation.Frozen) != want {
			t.Errorf("%s: mark %+v, want frozen %v", mi.Wire, mi.Deprecation, want)
		}
	}
	for _, in := range m.Interfaces {
		if in.Deprecation != nil {
			t.Errorf("%s is marked, but one of its methods is not frozen", in.Name)
		}
	}
}

package model

import (
	"slices"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

func TestUnstableMarking(t *testing.T) {
	t.Parallel()
	notice := []string{"**UNSTABLE**", "", "This capability is not part of the spec yet, and may be removed or changed at any point."}
	prov := &load.Provenance{
		UnstableDefs:  map[string]bool{"Fresh": true, "Described": true, "Marked": true, "NewRequest": true, "NewResponse": true},
		UnstableWires: map[string]bool{"x/new": true},
	}
	m, err := Build(Input{
		Meta: &load.Meta{AgentMethods: map[string]string{"new": "x/new", "old": "x/old"}},
		Schema: schemaOf(t, `{
			"Fresh":{"type":"string"},
			"Described":{"type":"string","description":"Described."},
			"Marked":{"type":"string","description":"**UNSTABLE**\n\nAlready."},
			"Old":{"type":"string","description":"Old."},
			"NewRequest":{"type":"object","x-side":"agent","x-method":"x/new","description":"New."},
			"NewResponse":{"type":"object","x-side":"agent","x-method":"x/new"},
			"OldRequest":{"type":"object","x-side":"agent","x-method":"x/old","description":"Old method."},
			"OldResponse":{"type":"object","x-side":"agent","x-method":"x/old"}}`),
		Provenance: prov,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		doc  []string
	}{
		{"Fresh", notice},
		{"Described", append(slices.Clone(notice), "", "Described.")},
		{"Marked", []string{"**UNSTABLE**", "", "Already."}},
		{"Old", []string{"Old."}},
	} {
		ty := m.Type(tt.name)
		if !slices.Equal(ty.Doc, tt.doc) {
			t.Errorf("%s doc:\n got %q\nwant %q", tt.name, ty.Doc, tt.doc)
		}
		if ty.Unstable != prov.UnstableDefs[tt.name] {
			t.Errorf("%s: Unstable = %v", tt.name, ty.Unstable)
		}
	}
	byWire := map[string]*Method{}
	for _, mi := range m.AgentMethods {
		byWire[mi.Wire] = mi
	}
	if got, want := byWire["x/new"].Doc, append(slices.Clone(notice), "", "New."); !slices.Equal(got, want) {
		t.Errorf("x/new doc:\n got %q\nwant %q", got, want)
	}
	if got := byWire["x/old"].Doc; !slices.Equal(got, []string{"Old method."}) {
		t.Errorf("x/old doc: %q", got)
	}
}

// TestTypeOrigin requires each type to name the file and key of its
// definition: the merge's origin when it has one, else the stable schema
// under the type's own name.
func TestTypeOrigin(t *testing.T) {
	t.Parallel()
	prov := &load.Provenance{Origins: map[string]load.Origin{"UnstableFresh": {File: load.UnstableSchemaFile, Key: "Fresh"}}}
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{"UnstableFresh":{"type":"string"},"Old":{"type":"string"}}`), Provenance: prov})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := m.Type("UnstableFresh").Origin, (load.Origin{File: load.UnstableSchemaFile, Key: "Fresh"}); got != want {
		t.Errorf("UnstableFresh: origin %+v, want %+v", got, want)
	}
	if got, want := m.Type("Old").Origin, (load.Origin{File: load.StableSchemaFile, Key: "Old"}); got != want {
		t.Errorf("Old: origin %+v, want %+v", got, want)
	}
	plain, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{"Old":{"type":"string"}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if got := plain.Type("Old").Origin; got.File != load.StableSchemaFile || got.Key != "Old" {
		t.Errorf("Old without provenance: origin %+v", got)
	}
}

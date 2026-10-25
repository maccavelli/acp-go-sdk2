package model

import (
	"slices"
	"strconv"
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

// TestUnstableEntries requires the fields and variants of a stable type that
// the merge's Changes name as added to be marked unstable, and no other: not
// a changed property's field, a changed entry's variant, nor the fields of a
// type the merge did not replace.
func TestUnstableEntries(t *testing.T) {
	t.Parallel()
	prov := &load.Provenance{Changes: map[string]*load.Changes{
		"Caps":   {AddedProperties: []string{"n"}, ChangedProperties: []string{"b"}},
		"Update": {AddedEntries: []int{1}, ChangedEntries: []int{2}},
	}}
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{
		"Caps":{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"string"},"n":{"type":"string"}}},
		"Other":{"type":"object","properties":{"n":{"type":"string"}}},
		"Update":{"discriminator":{"propertyName":"kind"},"oneOf":[
			{"type":"object","properties":{"kind":{"const":"one","type":"string"}}},
			{"type":"object","properties":{"kind":{"const":"new","type":"string"}}},
			{"type":"object","properties":{"kind":{"const":"two","type":"string"}}}]}}`), Provenance: prov})
	if err != nil {
		t.Fatal(err)
	}
	for _, ty := range []string{"Caps", "Other"} {
		for _, f := range m.Type(ty).Fields {
			if want := ty == "Caps" && f.Key == "n"; f.Unstable != want {
				t.Errorf("%s.%s: Unstable = %v, want %v", ty, f.Key, f.Unstable, want)
			}
		}
	}
	for i, v := range m.Type("Update").Variants {
		if want := i == 1; v.Unstable != want {
			t.Errorf("Update variant %d (%s): Unstable = %v, want %v", i, v.DiscValue, v.Unstable, want)
		}
	}
}

// TestUnstableEntriesOfCommittedSchema pins, on the schema in the repository,
// what the unstable schema adds to or changes in the stable definitions the
// merge replaces (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, G2 and
// G10): ten fields, two properties whose default changes, and six variants.
func TestUnstableEntriesOfCommittedSchema(t *testing.T) {
	t.Parallel()
	in := committedInput(t)
	m, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	var replaced, changed, changedEntries, fields, variants []string
	for name, c := range in.Provenance.Changes {
		replaced = append(replaced, name)
		for _, key := range c.ChangedProperties {
			changed = append(changed, name+"."+key)
		}
		for _, i := range c.ChangedEntries {
			changedEntries = append(changedEntries, name+"["+strconv.Itoa(i)+"]")
		}
	}
	for _, ty := range m.Types {
		for _, f := range ty.Fields {
			if f.Unstable {
				fields = append(fields, ty.Name+"."+f.Name)
			}
		}
		for _, v := range ty.Variants {
			if v.Unstable {
				variants = append(variants, ty.Name+"."+v.FieldName+" "+v.TypeName)
			}
		}
	}
	for _, tt := range []struct {
		what      string
		got, want []string
	}{
		{"replaced definitions", replaced, []string{"AgentCapabilities", "ClientCapabilities", "InitializeResponse", "McpCapabilities",
			"McpServer", "PromptResponse", "SessionCapabilities", "SessionUpdate"}},
		{"changed properties", changed, []string{"AgentCapabilities.mcpCapabilities", "InitializeResponse.agentCapabilities"}},
		{"changed union entries", changedEntries, nil},
		{"unstable fields", fields, []string{"AgentCapabilities.Nes", "AgentCapabilities.PositionEncoding", "AgentCapabilities.Providers",
			"ClientCapabilities.Nes", "ClientCapabilities.Plan", "ClientCapabilities.PositionEncodings", "ClientCapabilities.Subagents",
			"McpCapabilities.Acp", "PromptResponse.Usage", "SessionCapabilities.Fork"}},
		{"unstable variants", variants, []string{"McpServer.Acp McpServerAcpInline", "SessionUpdate.PlanRemoved SessionUpdatePlanRemoved",
			"SessionUpdate.PlanUpdate SessionPlanUpdate", "SessionUpdate.SessionMessage SessionUpdateSessionMessage",
			"SessionUpdate.SessionMessageChunk SessionUpdateSessionMessageChunk", "SessionUpdate.SubagentUpdate SessionSubagentUpdate"}},
	} {
		slices.Sort(tt.got)
		if !slices.Equal(tt.got, tt.want) {
			t.Errorf("%s:\n got %q\nwant %q", tt.what, tt.got, tt.want)
		}
	}
}

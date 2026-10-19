package model

import (
	"slices"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

// TestConstFields requires a variant's ConstFields to name the field of each
// constant when every one is a string field the constant encodes in, and to
// be nil otherwise, so the union encodes that variant through a map
// (0019-PLAN-idioms-performance.md, D1).
func TestConstFields(t *testing.T) {
	t.Parallel()
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{
		"K":{"type":"string","enum":["k","l"]},
		"Name":{"type":"string"},
		"Ref":{"type":"object","properties":{"kind":{"type":"string","const":"ref"},"tag":{"type":"string","const":"r"}},"required":["kind","tag"]},
		"U":{"oneOf":[
			{"title":"Str","type":"object","properties":{"kind":{"type":"string","const":"str"}},"required":["kind"]},
			{"title":"Enum","type":"object","properties":{"kind":{"type":"string","const":"enum"},"tag":{"allOf":[{"$ref":"#/$defs/K"}],"const":"k"}},"required":["kind","tag"]},
			{"title":"Prim","type":"object","properties":{"kind":{"type":"string","const":"prim"},"tag":{"allOf":[{"$ref":"#/$defs/Name"}],"const":"n"}},"required":["kind","tag"]},
			{"$ref":"#/$defs/Ref"},
			{"title":"Ptr","type":"object","properties":{"kind":{"type":"string","const":"ptr"},"tag":{"type":["string","null"],"const":"p"}},"required":["kind"]},
			{"title":"Raw","type":"object","properties":{"kind":{"type":"string","const":"raw"},"tag":{"const":"w"}},"required":["kind"]},
			{"title":"EmptyOmitted","type":"object","properties":{"kind":{"type":"string","const":"eo"},"tag":{"type":"string","const":""}},"required":["kind"]},
			{"title":"EmptyKept","type":"object","properties":{"kind":{"type":"string","const":"ek"},"tag":{"type":"string","const":""}},"required":["kind","tag"]}
		],"discriminator":{"propertyName":"kind"}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"str":  {"Kind"},
		"enum": {"Kind", "Tag"},
		"prim": {"Kind", "Tag"},
		"ref":  {"Kind", "Tag"},
		"ptr":  nil,
		"raw":  nil,
		"eo":   nil,
		"ek":   {"Kind", "Tag"},
	}
	seen := 0
	for _, vi := range m.Type("U").Variants {
		w, ok := want[vi.DiscValue]
		if !ok {
			t.Errorf("variant %s: not in the table", vi.DiscValue)
			continue
		}
		seen++
		if !slices.Equal(vi.ConstFields, w) || (w == nil) != (vi.ConstFields == nil) {
			t.Errorf("variant %s, pairs %v: ConstFields %v, want %v", vi.DiscValue, vi.ConstPairs, vi.ConstFields, w)
		}
	}
	if seen != len(want) {
		t.Errorf("%d variants checked, want %d", seen, len(want))
	}
}

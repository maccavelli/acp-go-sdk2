package model

import (
	"strings"
	"testing"
)

// deprecationDefs adds to protocolDefs a node the schema marks deprecated of
// each kind, beside one it does not mark, and marks CloseRequest, the
// request of the agent method s/close.
var deprecationDefs = strings.NewReplacer(
	`"Update":`, `"Gone":{"type":"object","description":"Gone.","deprecated":true},
		"Hold":{"type":"object","properties":{"a":{"type":"string","deprecated":true},"b":{"type":"string"}}},
		"Pick":{"oneOf":[{"type":"object","required":["kind"],"properties":{"kind":{"const":"x","type":"string"}},"deprecated":true},
			{"type":"object","required":["kind"],"properties":{"kind":{"const":"y","type":"string"}}}]},
		"Lvl":{"oneOf":[{"type":"string","const":"lo","deprecated":true},{"type":"string","const":"hi"}]},
		"ScopeA":{"type":"object","required":["a1"],"properties":{"a1":{"type":"string"}}},
		"ScopeB":{"type":"object","required":["b1"],"properties":{"b1":{"type":"string"}}},
		"Flat":{"type":"object","required":["s"],"properties":{"s":{"type":"string"}},"anyOf":[
			{"title":"First","allOf":[{"$ref":"#/$defs/ScopeA"}],"deprecated":true},{"title":"Second","allOf":[{"$ref":"#/$defs/ScopeB"}]}]},
		"Update":`,
	`"CloseRequest":{"type":"object",`, `"CloseRequest":{"deprecated":true,"type":"object",`,
).Replace(protocolDefs)

// deprecationMarks holds the marks TestDeprecationNotes reads: the marked
// node of each kind, and the unmarked one beside it.
type deprecationMarks struct {
	gone, holdA, holdB, pickX, pickY, lvlLo, lvlHi, first, second, closeReq, closeResp, method *Deprecation
}

// readMarks reads the marks of deprecationDefs's nodes from m.
func readMarks(m *Model) deprecationMarks {
	var k deprecationMarks
	k.gone = m.Type("Gone").Deprecation
	for _, f := range m.Type("Hold").Fields {
		switch f.Key {
		case "a":
			k.holdA = f.Deprecation
		case "b":
			k.holdB = f.Deprecation
		}
	}
	for _, vi := range m.Type("Pick").Variants {
		switch vi.DiscValue {
		case "x":
			k.pickX = vi.Deprecation
		case "y":
			k.pickY = vi.Deprecation
		}
	}
	for _, ev := range m.Type("Lvl").Enum {
		switch ev.Value {
		case "lo":
			k.lvlLo = ev.Deprecation
		case "hi":
			k.lvlHi = ev.Deprecation
		}
	}
	for _, mb := range m.Type("Flat").Members {
		switch mb.Field {
		case "First":
			k.first = mb.Deprecation
		case "Second":
			k.second = mb.Deprecation
		}
	}
	k.closeReq, k.closeResp = m.Type("CloseRequest").Deprecation, m.Type("CloseResponse").Deprecation
	for _, mi := range m.AgentMethods {
		if mi.Wire == "s/close" {
			k.method = mi.Deprecation
		}
	}
	return k
}

// TestDeprecationNotes requires the schema's marks on the model's
// declarations, each without a note when the table has none; a method marked
// by its request definition, naming it; a note's text on each declaration of
// its node; and the refusal of a note the table's checks cannot use, or whose
// node the schema lacks or does not mark (0024-PLAN-api-gate-and-deprecation.md).
func TestDeprecationNotes(t *testing.T) {
	t.Parallel()
	build := func(notes ...DeprecationNote) (*Model, error) {
		p := testProtocol()
		p.DeprecationNotes = notes
		return Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, deprecationDefs), Protocol: p})
	}
	plain, err := build()
	if err != nil {
		t.Fatal(err)
	}
	k := readMarks(plain)
	for name, d := range map[string]*Deprecation{"Gone": k.gone, "Hold.a": k.holdA, `Pick "x"`: k.pickX, `Lvl "lo"`: k.lvlLo,
		"Flat First": k.first, "CloseRequest": k.closeReq} {
		if d == nil || d.Note != "" {
			t.Errorf("%s: %+v, want a mark without a note", name, d)
		}
	}
	for name, d := range map[string]*Deprecation{"Hold.b": k.holdB, `Pick "y"`: k.pickY, `Lvl "hi"`: k.lvlHi, "Flat Second": k.second,
		"CloseResponse": k.closeResp} {
		if d != nil {
			t.Errorf("%s: %+v, want no mark", name, d)
		}
	}
	if k.method == nil || k.method.Definition != "CloseRequest" || k.method.Note != "" {
		t.Errorf("s/close's mark = %+v, want CloseRequest's", k.method)
	}
	for _, ty := range plain.Types {
		if ty.Deprecation != nil && ty.Name != "Gone" && ty.Name != "CloseRequest" {
			t.Errorf("%s is marked, and the schema does not mark it", ty.Name)
		}
	}
	for _, mi := range append(plain.AgentMethods, plain.ClientMethods...) {
		if mi.Deprecation != nil && mi.Wire != "s/close" {
			t.Errorf("%s is marked, and its definitions are not", mi.Wire)
		}
	}

	m, err := build(DeprecationNote{Definition: "Gone", Note: "Use Kept."},
		DeprecationNote{Definition: "Hold", Property: "a", Note: "Use b."},
		DeprecationNote{Definition: "Pick", Variant: "x", Note: "Use y."},
		DeprecationNote{Definition: "Lvl", Variant: "lo", Note: "Use hi."},
		DeprecationNote{Definition: "Flat", Variant: "First", Note: "Use Second."},
		DeprecationNote{Definition: "CloseRequest", Note: "Close by other means."})
	if err != nil {
		t.Fatal(err)
	}
	k = readMarks(m)
	for name, tc := range map[string]struct {
		d    *Deprecation
		want string
	}{
		"Gone": {k.gone, "Use Kept."}, "Hold.a": {k.holdA, "Use b."}, `Pick "x"`: {k.pickX, "Use y."}, `Lvl "lo"`: {k.lvlLo, "Use hi."},
		"Flat First": {k.first, "Use Second."}, "CloseRequest": {k.closeReq, "Close by other means."},
		"s/close": {k.method, "Close by other means."},
	} {
		if tc.d == nil || tc.d.Note != tc.want {
			t.Errorf("%s: %+v, want the note %q", name, tc.d, tc.want)
		}
	}

	for _, tt := range []struct {
		name  string
		notes []DeprecationNote
		want  string
	}{
		{"no definition", []DeprecationNote{{Note: "x"}}, "deprecation note : it needs a definition and a note"},
		{"no note", []DeprecationNote{{Definition: "Gone"}}, "deprecation note Gone: it needs a definition and a note"},
		{"a property and a variant", []DeprecationNote{{Definition: "Hold", Property: "a", Variant: "x", Note: "n"}},
			`deprecation note Hold.a "x": it names both a property and a variant`},
		{"two notes", []DeprecationNote{{Definition: "Gone", Note: "a"}, {Definition: "Gone", Note: "b"}}, "deprecation note Gone: two notes"},
		{"an unmarked definition", []DeprecationNote{{Definition: "Update", Note: "n"}},
			"deprecation note Update: the schema does not mark it deprecated"},
		{"an unmarked property", []DeprecationNote{{Definition: "Hold", Property: "b", Note: "n"}},
			"deprecation note Hold.b: the schema does not mark it deprecated"},
		{"an unmarked variant", []DeprecationNote{{Definition: "Pick", Variant: "y", Note: "n"}},
			`deprecation note Pick "y": the schema does not mark it deprecated`},
		{"an absent definition", []DeprecationNote{{Definition: "Nope", Note: "n"}},
			"deprecation note Nope: the schema does not mark it deprecated"},
		{"an absent variant", []DeprecationNote{{Definition: "Pick", Variant: "z", Note: "n"}},
			`deprecation note Pick "z": the schema does not mark it deprecated`},
	} {
		if _, err := build(tt.notes...); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: Build = %v, want an error containing %q", tt.name, err, tt.want)
		}
	}
}

package model

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// keyLessDefs adds, to protocolDefs, unions with a key-less variant:
//   - Fits, without a catch-all, whose key-less variant is titled and states the
//     fallback's source;
//   - Silent, the same but for a description that does not state it;
//   - Caught, with a catch-all beside its titled key-less variant;
//   - Plain, an object.
const keyLessDefs = `"Fits":{"anyOf":[
		{"type":"object","properties":{"type":{"type":"string","const":"a"}},"required":["type"]},
		{"title":"loose","description":"Unknown types also decode as this.","type":"object","properties":{"x":{"type":"string"}},"required":["x"]}]},
	"Silent":{"anyOf":[
		{"type":"object","properties":{"type":{"type":"string","const":"a"}},"required":["type"]},
		{"title":"loose","description":"The default.","type":"object","properties":{"x":{"type":"string"}},"required":["x"]}]},
	"Caught":{"anyOf":[
		{"type":"object","properties":{"type":{"type":"string","const":"a"}},"required":["type"]},
		{"title":"other","type":"object","properties":{"type":{"type":"string"}},"required":["type"],
			"not":{"anyOf":[{"type":"object","properties":{"type":{"type":"string","const":"a"}},"required":["type"]}]}},
		{"title":"plain","description":"Unknown types also decode as this.","type":"object","properties":{"y":{"type":"string"}},"required":["y"]}]},
	"Plain":{"type":"object","properties":{"x":{"type":"string"}}},`

// TestKeyLessTitlesAndFallbacks requires a key-less variant of a union
// without a catch-all to take its title as its own name, and one in a union
// with a catch-all not to; a key-less fallback rule to mark the variant whose
// description states its source, a rule whose definition the schema lacks to
// apply nothing, and Build to refuse a rule it cannot use
// (0049-PLAN-surface-fixes.md, phase 1, deviations 1 and 2).
func TestKeyLessTitlesAndFallbacks(t *testing.T) {
	t.Parallel()
	const src = "Unknown types also decode as this."
	withDefs := func(extra string) string {
		return strings.Replace(protocolDefs, `"Update":`, extra+`"Update":`, 1)
	}
	build := func(defs string, rules ...FallbackRule) (*Model, error) {
		p := testProtocol()
		p.KeyLessFallbacks = rules
		return Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, defs), Protocol: p})
	}
	ok := keyLessDefs
	m, err := build(withDefs(ok), FallbackRule{"Fits", src}, FallbackRule{"Nope", src})
	if err != nil {
		t.Fatal(err)
	}
	keyLess := func(name string) *Variant {
		t.Helper()
		for _, vi := range m.Type(name).Variants {
			if vi.KeyLess {
				return vi
			}
		}
		t.Fatalf("%s has no key-less variant", name)
		return nil
	}
	if vi := keyLess("Fits"); vi.Title != "loose" || !vi.Fallback {
		t.Errorf("Fits, ruled: Title %q Fallback %v, want loose and true", vi.Title, vi.Fallback)
	}
	if vi := keyLess("Silent"); vi.Title != "loose" || vi.Fallback {
		t.Errorf("Silent, no rule: Title %q Fallback %v, want loose and false", vi.Title, vi.Fallback)
	}
	if vi := keyLess("Caught"); vi.Title != "" || vi.Fallback {
		t.Errorf("Caught, a catch-all: Title %q Fallback %v, want none", vi.Title, vi.Fallback)
	}

	for _, tt := range []struct {
		name string
		rule FallbackRule
		want string
	}{
		{"not stated", FallbackRule{"Silent", src}, `Silent: its key-less variant's description does not state "Unknown types also decode as this."`},
		{"a catch-all", FallbackRule{"Caught", src}, "Caught: a key-less fallback rule needs a discriminated union without a catch-all"},
		{"not a union", FallbackRule{"Plain", src}, "Plain: a key-less fallback rule needs a discriminated union without a catch-all"},
		{"no source", FallbackRule{"Fits", ""}, `key-less fallback rule "Fits": it needs a definition and a source`},
	} {
		if _, err := build(withDefs(ok), tt.rule); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: Build = %v, want an error containing %q", tt.name, err, tt.want)
		}
	}
	if _, err := build(withDefs(ok), FallbackRule{"Fits", src}, FallbackRule{"Fits", src}); err == nil ||
		!strings.Contains(err.Error(), "key-less fallback rule Fits: two rules") {
		t.Errorf("two rules: Build = %v", err)
	}
}

// fallbackWording is how the schema says a key-less variant takes an unknown
// discriminator's payload.
var fallbackWording = regexp.MustCompile("(?i)unknown .{0,40}values? .{0,60}deserialize into this variant")

// checkCommittedKeyLessFallbacks requires every key-less variant of the
// committed schemas whose description says it takes unknown values to be one a
// key-less fallback rule marks, and those to be the only ones marked
// (0049-PLAN-surface-fixes.md, phase 1, deviation 2).
func checkCommittedKeyLessFallbacks(t *testing.T, m *Model) {
	t.Helper()
	var said, marked []string
	for _, ty := range m.Types {
		for _, vi := range ty.Variants {
			if vi.KeyLess && fallbackWording.MatchString(strings.Join(strings.Fields(vi.description), " ")) {
				said = append(said, ty.Name+"."+vi.FieldName)
			}
			if vi.Fallback {
				marked = append(marked, ty.Name+"."+vi.FieldName)
			}
		}
	}
	if !slices.Equal(said, marked) || !slices.Equal(marked, []string{"SetSessionConfigOptionRequest.ValueId"}) {
		t.Errorf("key-less variants that say they take unknown values: %v; marked by a rule: %v; want both [SetSessionConfigOptionRequest.ValueId]", said, marked)
	}
}

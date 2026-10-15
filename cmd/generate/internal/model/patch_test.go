package model

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// patchedDefs adds Patched, an object with a nullable lenient string, a
// nullable lenient array, a required string, a nullable string without
// x-deserialize-default-on-error, and a nullable lenient string with a
// default, to protocolDefs.
const patchedDefs = `"Patched":{"type":"object","required":["id"],"properties":{
	"id":{"type":"string"},
	"title":{"type":["string","null"],"x-deserialize-default-on-error":true},
	"items":{"type":["array","null"],"items":{"type":"string"},"x-deserialize-default-on-error":true,"x-deserialize-skip-invalid-items":true},
	"strict":{"type":["string","null"]},
	"withDefault":{"type":["string","null"],"default":"x","x-deserialize-default-on-error":true}}},`

// TestPatchRules requires a patch rule to make its property's field a Patch
// of the type it had, tagged omitzero; a rule whose property the schema lacks
// to apply nothing; and the table's checks and Build's to refuse a rule they
// cannot use (0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md, step 2).
func TestPatchRules(t *testing.T) {
	t.Parallel()
	defs := strings.Replace(protocolDefs, `"Update":`, patchedDefs+`"Update":`, 1)
	build := func(rules ...PatchRule) (*Model, error) {
		p := testProtocol()
		p.PatchFields = rules
		return Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, defs), Protocol: p})
	}
	m, err := build(PatchRule{"Patched", "title", "s"}, PatchRule{"Patched", "items", "s"}, PatchRule{"Patched", "nope", "s"},
		PatchRule{"Nope", "title", "s"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*Field{}
	for _, f := range m.Type("Patched").Fields {
		got[f.Key] = f
	}
	if f := got["title"]; !f.Patch || f.Type.String() != "Patch[string]" || f.Tag != "title,omitzero" {
		t.Errorf("title: Patch %v, %s, tag %q", f.Patch, f.Type, f.Tag)
	}
	if f := got["items"]; !f.Patch || f.Type.String() != "Patch[[]string]" || f.Tag != "items,omitzero" {
		t.Errorf("items: Patch %v, %s, tag %q", f.Patch, f.Type, f.Tag)
	}
	if f := got["strict"]; f.Patch || f.Type.String() != "*string" {
		t.Errorf("strict, no rule: Patch %v, %s", f.Patch, f.Type)
	}

	for _, tt := range []struct {
		name string
		rule PatchRule
		want string
	}{
		{"required", PatchRule{"Patched", "id", "s"}, "a patch rule needs an optional property"},
		{"default", PatchRule{"Patched", "withDefault", "s"}, "a patch rule needs a property without a default"},
		{"strict", PatchRule{"Patched", "strict", "s"}, "a patch rule needs x-deserialize-default-on-error"},
		{"no source", PatchRule{"Patched", "title", ""}, "it needs a definition, a property, and a source"},
	} {
		if _, err := build(tt.rule); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: Build = %v, want an error containing %q", tt.name, err, tt.want)
		}
	}
	if _, err := build(PatchRule{"Patched", "title", "s"}, PatchRule{"Patched", "title", "t"}); err == nil ||
		!strings.Contains(err.Error(), "patch rule Patched.title: two rules") {
		t.Errorf("two rules: Build = %v", err)
	}
}

// retainedDefs adds Held, an object with a required integer, an optional raw
// value whose description states the rule's source, one that does not, a
// required raw value that does, and an optional string that does, to
// protocolDefs.
const retainedDefs = `"Held":{"type":"object","required":["code","need"],"properties":{
	"code":{"type":"integer"},
	"data":{"description":"Data; explicit null is retained."},
	"other":{"description":"Other data."},
	"need":{"description":"Needed; explicit null is retained."},
	"str":{"type":["string","null"],"description":"Text; explicit null is retained."}}},`

// TestNullRetainedRules requires a null-retained rule to make its property's
// field keep null, another optional raw field to read null as absent, a rule
// whose definition or property the schema lacks to apply nothing, and the
// table's checks and Build's to refuse a rule they cannot use
// (0019-PLAN-generator-hardening.md, G3).
func TestNullRetainedRules(t *testing.T) {
	t.Parallel()
	defs := strings.Replace(protocolDefs, `"Update":`, retainedDefs+`"Update":`, 1)
	const src = "explicit null is retained"
	build := func(rules ...PatchRule) (*Model, error) {
		p := testProtocol()
		p.NullRetained = rules
		return Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, defs), Protocol: p})
	}
	m, err := build(PatchRule{"Held", "data", src}, PatchRule{"Held", "nope", src}, PatchRule{"Nope", "data", src})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*Field{}
	for _, f := range m.Type("Held").Fields {
		got[f.Key] = f
	}
	if f := got["data"]; f.NullIsAbsent || f.Type.Kind != RefRaw {
		t.Errorf("data, ruled: NullIsAbsent %v, %s", f.NullIsAbsent, f.Type)
	}
	if f := got["other"]; !f.NullIsAbsent {
		t.Errorf("other, no rule: NullIsAbsent %v", f.NullIsAbsent)
	}

	for _, tt := range []struct {
		name string
		rule PatchRule
		want string
	}{
		{"required", PatchRule{"Held", "need", src}, "Held.need: a null-retained rule needs an optional property"},
		{"not raw", PatchRule{"Held", "str", src}, "Held.str: a null-retained rule needs a raw property, not *string"},
		{"not stated", PatchRule{"Held", "other", src}, `Held.other: its description does not state "explicit null is retained"`},
		{"no source", PatchRule{"Held", "data", ""}, "it needs a definition, a property, and a source"},
	} {
		if _, err := build(tt.rule); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: Build = %v, want an error containing %q", tt.name, err, tt.want)
		}
	}
	if _, err := build(PatchRule{"Held", "data", src}, PatchRule{"Held", "data", "Data"}); err == nil ||
		!strings.Contains(err.Error(), "null-retained rule Held.data: two rules") {
		t.Errorf("two rules: Build = %v", err)
	}
}

// retainedWording is how the schema says that an explicit null is kept apart
// from omission (0019-PLAN-generator-hardening.md, G3).
var retainedWording = regexp.MustCompile("(?i)explicit [`']?null[`']? is (retained|preserved)")

// checkCommittedNullRetained requires every property of the committed schemas
// whose description says an explicit null is retained, and every definition
// that says so of a property, to have a null-retained rule of ACP, and the
// rules to leave exactly UnstableMcpError.data an optional raw field that
// keeps null.
func checkCommittedNullRetained(t *testing.T, m *Model) {
	t.Helper()
	rules := map[[2]string]bool{}
	ruled := map[string]bool{}
	for _, r := range ACP.NullRetained {
		rules[[2]string{r.Definition, r.Property}] = true
		ruled[r.Definition] = true
	}
	for name, def := range m.schema.Defs {
		if retainedWording.MatchString(def.Description) && !ruled[name] {
			t.Errorf("%s: its description says an explicit null is retained, and no null-retained rule names it", name)
		}
		for key, p := range expandAllOf(m.schema, def).Properties {
			if retainedWording.MatchString(p.Description) && !rules[[2]string{name, key}] {
				t.Errorf("%s.%s: its description says an explicit null is retained, and no null-retained rule names it", name, key)
			}
		}
	}
	var kept []string
	for _, ty := range m.Types {
		for _, f := range ty.Fields {
			if f.Type.Kind == RefRaw && !f.Required && !f.NullIsAbsent {
				kept = append(kept, ty.Name+"."+f.Key)
			}
		}
	}
	if !slices.Equal(kept, []string{"UnstableMcpError.data"}) {
		t.Errorf("optional raw fields that keep null: %v, want [UnstableMcpError.data]", kept)
	}
}

// patchWording is how the schema says that null differs from omission, and
// sameWording how it says that they do not
// (0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md, issue #5).
var (
	patchWording = regexp.MustCompile(`(?i)\bclears?\b|unchanged|patch semantics|retain|preserv`)
	sameWording  = regexp.MustCompile(`(?i)are equivalent|both mean`)
)

// patchExceptions are the properties the patch wording matches whose text
// says null and omission are the same, by the names of the merged schema. A
// property a null-retained rule names is not a patch field either.
var patchExceptions = [][2]string{
	{"SessionMessage", "recipientSessionId"}, {"SessionMessage", "senderSessionId"},
	{"SessionMessageChunk", "recipientSessionId"}, {"SessionMessageChunk", "senderSessionId"},
}

// checkCommittedPatchFields requires every property of the committed schemas
// whose description gives null a meaning omission does not have to be a
// patch rule of ACP or an exception, every definition that states patch
// semantics for its fields to have a rule, and every rule to give the model
// its Patch fields: the 24 of 8 types the MADR counts.
func checkCommittedPatchFields(t *testing.T, m *Model) {
	t.Helper()
	rules := map[[2]string]bool{}
	ruled := map[string]bool{}
	for _, r := range ACP.PatchFields {
		rules[[2]string{r.Definition, r.Property}] = true
		ruled[r.Definition] = true
	}
	for name, def := range m.schema.Defs {
		if d := def.Description; strings.Contains(d, "null") && regexp.MustCompile(`(?i)patch semantics|patch fields`).MatchString(d) && !ruled[name] {
			t.Errorf("%s: its description states patch semantics, and no patch rule names it", name)
		}
		for key, p := range expandAllOf(m.schema, def).Properties {
			d := p.Description
			if !strings.Contains(d, "null") || !patchWording.MatchString(d) || sameWording.MatchString(d) {
				continue
			}
			k := [2]string{name, key}
			retained := slices.ContainsFunc(ACP.NullRetained, func(r PatchRule) bool { return r.Definition == name && r.Property == key })
			if !rules[k] && !slices.Contains(patchExceptions, k) && !retained {
				t.Errorf("%s.%s: its description gives null a meaning of its own, and no patch rule names it: %q", name, key, d)
			}
		}
	}
	var fields []string
	for _, ty := range m.Types {
		lists := [][]*Field{ty.Fields}
		for _, vi := range ty.Variants {
			if vi.Inline != nil {
				lists = append(lists, vi.Inline.Fields)
			}
		}
		for i, fs := range lists {
			for _, f := range fs {
				if f.Patch != (f.Type.Kind == RefPatch) {
					t.Errorf("%s.%s: Patch %v but type %s", ty.Name, f.Key, f.Patch, f.Type)
				}
				if f.Patch {
					owner := ty.Name
					if i > 0 {
						owner += " variant"
					}
					fields = append(fields, owner+"."+f.Key)
				}
			}
		}
	}
	if len(fields) != 24 {
		t.Errorf("%d Patch fields, want 24: %v", len(fields), fields)
	}
	for r := range rules {
		found := slices.ContainsFunc(fields, func(f string) bool { return strings.HasSuffix(f, "."+r[1]) && strings.HasPrefix(f, r[0]) })
		if !found {
			t.Errorf("patch rule %s.%s: no Patch field", r[0], r[1])
		}
	}
}

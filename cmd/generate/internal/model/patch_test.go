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

// orderedDefs adds, to protocolDefs, an object with a typed map, an array and
// a string; a union with a typed map of its own; an object with flattened
// members and one that keeps extra keys, each with a typed map.
const orderedDefs = `"Ordered":{"type":"object","properties":{
	"props":{"type":"object","additionalProperties":{"type":"string"}},
	"list":{"type":"array","items":{"type":"string"}},
	"note":{"type":"string"}}},
	"OrdU":{"type":"object","properties":{"m":{"type":"object","additionalProperties":{"type":"string"}}},
		"oneOf":[{"title":"A","type":"object","properties":{"a":{"type":"string"}},"required":["a"]}]},
	"OrdA":{"type":"object","properties":{"a":{"type":"string"}},"required":["a"]},
	"OrdB":{"type":"object","properties":{"b":{"type":"string"}},"required":["b"]},
	"OrdF":{"type":"object","properties":{"m":{"type":"object","additionalProperties":{"type":"string"}}},
		"anyOf":[{"$ref":"#/$defs/OrdA"},{"$ref":"#/$defs/OrdB"}]},
	"OrdX":{"type":"object","properties":{"m":{"type":"object","additionalProperties":{"type":"string"}}},"additionalProperties":true},`

// TestKeyOrderRules requires a key-order rule to give its type the order of
// its map, a rule whose definition or property the schema lacks to apply
// nothing, and the table's checks and Build's to refuse a rule they cannot
// use (0022-PLAN-keep-elicitation-property-order-before-v1-15-0.md).
func TestKeyOrderRules(t *testing.T) {
	t.Parallel()
	defs := strings.Replace(protocolDefs, `"Update":`, orderedDefs+`"Update":`, 1)
	build := func(rules ...OrderRule) (*Model, error) {
		p := testProtocol()
		p.KeyOrders = rules
		return Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, defs), Protocol: p})
	}
	m, err := build(OrderRule{"Ordered", "props", "PropsOrder", "s"}, OrderRule{"Nope", "props", "X", "s"},
		OrderRule{"Ordered", "nope", "Y", "s"})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Type("Ordered").KeyOrders; len(got) != 1 || *got[0] != (KeyOrder{Field: "PropsOrder", Map: "Props", Key: "props"}) {
		t.Errorf("Ordered.KeyOrders = %v", got)
	}
	for _, tt := range []struct {
		name  string
		rules []OrderRule
		want  string
	}{
		{"no field", []OrderRule{{"Ordered", "props", "", "s"}}, "it needs a definition, a property, a field, and a source"},
		{"an unexported field", []OrderRule{{"Ordered", "props", "order", "s"}}, `field "order" is not an exported Go identifier`},
		{"two rules", []OrderRule{{"Ordered", "props", "A", "s"}, {"Ordered", "props", "B", "s"}}, "key-order rule Ordered.props: two rules"},
		{"not an object", []OrderRule{{"OrdU", "m", "MOrder", "s"}}, "OrdU.m: a key-order rule needs a property of an object"},
		{"not a map", []OrderRule{{"Ordered", "list", "ListOrder", "s"}}, "Ordered.list: a key-order rule needs a typed map, not []string"},
		{"flattened members", []OrderRule{{"OrdF", "m", "MOrder", "s"}}, "OrdF.m: a key-order rule on a struct with flattened members"},
		{"kept extra keys", []OrderRule{{"OrdX", "m", "MOrder", "s"}}, "OrdX.m: a key-order rule on a struct with flattened members or kept extra keys"},
		{"a field taken", []OrderRule{{"Ordered", "props", "Note", "s"}}, "Ordered.props: the key-order field Note is already a field of Ordered"},
	} {
		if _, err := build(tt.rules...); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: Build = %v, want an error containing %q", tt.name, err, tt.want)
		}
	}
}

// checkCommittedKeyOrders requires ACP's key-order rule to give the committed
// ElicitationSchema its PropertyOrder, and no other type an order.
func checkCommittedKeyOrders(t *testing.T, m *Model) {
	t.Helper()
	for _, ty := range m.Types {
		want := 0
		if ty.Name == "ElicitationSchema" {
			want = 1
		}
		if len(ty.KeyOrders) != want {
			t.Errorf("%s: %d key orders, want %d", ty.Name, len(ty.KeyOrders), want)
		}
	}
	if got := m.Type("ElicitationSchema").KeyOrders; len(got) != 1 ||
		*got[0] != (KeyOrder{Field: "PropertyOrder", Map: "Properties", Key: "properties"}) {
		t.Errorf("ElicitationSchema.KeyOrders = %v", got)
	}
}

// TestDocNotes requires a doc note's lines after its type's description and a
// blank line, or alone when the type has no description; a note on a union
// to reach its inline variants' types, and not its wrapper fields; a note
// whose definition the schema lacks to apply nothing; and the table's checks
// to refuse a note they cannot use (0024-PLAN-documentation.md, A11).
func TestDocNotes(t *testing.T) {
	t.Parallel()
	defs := strings.Replace(protocolDefs, `"Update":`, `"Noted":{"type":"object","description":"Said by the schema."},
		"NotedUnion":{"description":"A union.","oneOf":[
			{"title":"Inline","type":"object","description":"An inline variant.","required":["a"],"properties":{"a":{"type":"string"}}},
			{"$ref":"#/$defs/ListCaps"}]},"Update":`, 1)
	build := func(notes ...DocNote) (*Model, error) {
		p := testProtocol()
		p.DocNotes = notes
		return Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, defs), Protocol: p})
	}
	plain, err := build()
	if err != nil {
		t.Fatal(err)
	}
	m, err := build(DocNote{"Noted", []string{"First.", "Second."}}, DocNote{"Update", []string{"Only."}},
		DocNote{"Nope", []string{"Absent."}}, DocNote{"NotedUnion", []string{"Both."}})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Type("Noted").Doc; !slices.Equal(got, []string{"Said by the schema.", "", "First.", "Second."}) {
		t.Errorf("Noted.Doc = %q", got)
	}
	if got := m.Type("Update").Doc; !slices.Equal(got, []string{"Only."}) {
		t.Errorf("Update.Doc = %q", got)
	}
	u := m.Type("NotedUnion")
	if !slices.Equal(u.Doc, []string{"A union.", "", "Both."}) {
		t.Errorf("NotedUnion.Doc = %q", u.Doc)
	}
	if len(u.Variants) != 2 || u.Variants[0].Inline == nil || u.Variants[1].Inline != nil {
		t.Fatalf("NotedUnion's variants: %+v", u.Variants)
	}
	if got := u.Variants[0].Inline.Doc; !slices.Equal(got, []string{"An inline variant.", "", "Both."}) {
		t.Errorf("the inline variant's Doc = %q", got)
	}
	if got := u.Variants[0].Doc; !slices.Equal(got, []string{"An inline variant."}) {
		t.Errorf("the inline variant's wrapper field Doc = %q, want it unchanged", got)
	}
	for _, ty := range m.Types {
		if ty.Name != "Noted" && ty.Name != "Update" && ty.Name != "NotedUnion" && !slices.Equal(ty.Doc, plain.Type(ty.Name).Doc) {
			t.Errorf("%s.Doc = %q, want it unchanged: %q", ty.Name, ty.Doc, plain.Type(ty.Name).Doc)
		}
	}
	for _, tt := range []struct {
		name  string
		notes []DocNote
		want  string
	}{
		{"no definition", []DocNote{{"", []string{"x"}}}, `doc note "": it needs a definition and lines`},
		{"no lines", []DocNote{{"Noted", nil}}, `doc note "Noted": it needs a definition and lines`},
		{"two notes", []DocNote{{"Noted", []string{"a"}}, {"Noted", []string{"b"}}}, "doc note Noted: two notes"},
	} {
		if _, err := build(tt.notes...); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: Build = %v, want an error containing %q", tt.name, err, tt.want)
		}
	}
}

// checkCommittedDocNotes requires every doc note of ACP to name a committed
// definition with a description, and to end that type's doc comment, and its
// inline variants', after a blank line: the ten envelope types among them.
func checkCommittedDocNotes(t *testing.T, m *Model) {
	t.Helper()
	if len(ACP.DocNotes) != 9 {
		t.Errorf("%d doc notes, want 9", len(ACP.DocNotes))
	}
	var variants []string
	for _, note := range ACP.DocNotes {
		ty := m.Type(note.Definition)
		if ty == nil {
			t.Errorf("doc note %s: no such type", note.Definition)
			continue
		}
		ends := func(what string, doc []string) {
			n := len(doc) - len(note.Lines)
			if n < 2 || doc[n-1] != "" || !slices.Equal(doc[n:], note.Lines) {
				t.Errorf("%s.Doc = %q, want a description, a blank line, and %q", what, doc, note.Lines)
			}
		}
		ends(ty.Name, ty.Doc)
		for _, v := range ty.Variants {
			if v.Inline != nil {
				ends(v.TypeName, v.Inline.Doc)
				variants = append(variants, v.TypeName)
			}
		}
	}
	slices.Sort(variants)
	if want := []string{"AgentError", "AgentResult", "ClientError", "ClientResult"}; !slices.Equal(variants, want) {
		t.Errorf("noted variants %v, want %v", variants, want)
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

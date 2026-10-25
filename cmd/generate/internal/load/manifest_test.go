package load

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// writeSchemaDir writes the named schema files into a new directory.
func writeSchemaDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// manifestInputs are a stable and an unstable schema whose merge has every
// part a manifest freezes: an unstable method with a copied subgraph, an
// unstable-only definition kept under its own name, a changed property, an
// added property and an added union entry.
var manifestInputs = map[string]string{
	"meta.json":          `{"version": 1, "agentMethods": {"old": "x/old"}, "clientMethods": {}}`,
	"meta.unstable.json": `{"version": 1, "agentMethods": {"old": "x/old", "new": "x/new"}, "clientMethods": {}}`,
	"schema.json": `{"$defs": {
		"OldRequest": {"type": "object", "properties": {"c": {"$ref": "#/$defs/Caps"}, "p": {"$ref": "#/$defs/Pick"}}, "x-side": "agent", "x-method": "x/old"},
		"OldResponse": {"type": "object", "x-side": "agent", "x-method": "x/old"},
		"Caps": {"type": "object", "properties": {"a": {"type": "string", "default": "x"}}},
		"Pick": {"discriminator": {"propertyName": "kind"}, "oneOf": [
			{"type": "object", "properties": {"kind": {"const": "a", "type": "string"}}},
			{"type": "object", "properties": {"kind": {"const": "b", "type": "string"}}}]}}}`,
	"schema.unstable.json": `{"$defs": {
		"OldRequest": {"type": "object", "properties": {"c": {"$ref": "#/$defs/Caps"}, "p": {"$ref": "#/$defs/Pick"}}, "x-side": "agent", "x-method": "x/old"},
		"OldResponse": {"type": "object", "x-side": "agent", "x-method": "x/old"},
		"Caps": {"type": "object", "properties": {"a": {"type": "string", "default": "y"}, "n": {"$ref": "#/$defs/Fresh"}}},
		"Fresh": {"type": "string", "description": "Only <unstable> & new."},
		"Pick": {"discriminator": {"propertyName": "kind"}, "oneOf": [
			{"type": "object", "properties": {"kind": {"const": "a", "type": "string"}}},
			{"type": "object", "properties": {"kind": {"const": "n", "type": "string"}}},
			{"type": "object", "properties": {"kind": {"const": "b", "type": "string"}}}]},
		"NewRequest": {"type": "object", "properties": {"f": {"$ref": "#/$defs/Fresh"}, "c": {"$ref": "#/$defs/Caps"}}, "x-side": "agent", "x-method": "x/new"},
		"NewResponse": {"type": "object", "x-side": "agent", "x-method": "x/new"}}}`,
}

// TestWriteUnstableManifest requires the writer to freeze a manifest whose
// merge is MergeStableAndUnstable's, with each origin naming the manifest; to
// keep a fragment's text unescaped; and to refuse when the manifest exists
// and when an unstable file is missing.
func TestWriteUnstableManifest(t *testing.T) {
	t.Parallel()
	dir := writeSchemaDir(t, manifestInputs)
	if err := WriteUnstableManifest(dir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "Only <unstable> & new.") {
		t.Errorf("the manifest escapes its fragments' text:\n%s", b)
	}
	m, found, err := ReadUnstableManifest(dir)
	if err != nil || !found {
		t.Fatalf("read: found %v, %v", found, err)
	}
	if got, want := m.Definitions, map[string]string{"Fresh": "Fresh", "UnstableCaps": "Caps", "UnstableFresh": "Fresh",
		"UnstableNewRequest": "NewRequest", "UnstableNewResponse": "NewResponse"}; !equalMaps(got, want) {
		t.Errorf("definitions %v, want %v", got, want)
	}
	if c := m.Changes["Pick"]; c == nil || !slices.Equal(c.Entries, []string{"const kind=a", "const kind=n", "const kind=b"}) ||
		!slices.Equal(c.FrozenEntries, []string{"const kind=n"}) {
		t.Errorf("Pick's change: %+v", c)
	}
	if c := m.Changes["Caps"]; c == nil || !slices.Equal(c.Properties, []string{"a", "n"}) {
		t.Errorf("Caps's change: %+v", c)
	}
	sm, ss := mustStable(t, dir)
	um, _, _ := ReadMetaUnstable(dir)
	us, _, _ := ReadSchemaUnstable(dir)
	meta, schema, prov, err := MergeStableAndUnstable(sm, ss, um, us)
	if err != nil {
		t.Fatal(err)
	}
	gotMeta, gotSchema, gotProv, err := MergeManifest(sm, ss, m)
	if err != nil {
		t.Fatal(err)
	}
	if err := sameMerge(meta, schema, prov, gotMeta, gotSchema, gotProv); err != nil {
		t.Error(err)
	}

	if err := WriteUnstableManifest(dir); err == nil || !strings.Contains(err.Error(), "frozen once") {
		t.Errorf("second write: %v, want a refusal", err)
	}
	stableOnly := writeSchemaDir(t, map[string]string{"meta.json": manifestInputs["meta.json"], "schema.json": manifestInputs["schema.json"]})
	if err := WriteUnstableManifest(stableOnly); err == nil || !strings.Contains(err.Error(), "one is missing") {
		t.Errorf("write without unstable files: %v, want a refusal", err)
	}
	if _, err := os.Stat(filepath.Join(stableOnly, ManifestFile)); !os.IsNotExist(err) {
		t.Errorf("a refused write left %s: %v", ManifestFile, err)
	}
}

func mustStable(t *testing.T, dir string) (*Meta, *Schema) {
	t.Helper()
	sm, err := ReadMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	ss, err := ReadSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	return sm, ss
}

func equalMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// TestApplyChange requires a change to take each listed property from the
// frozen version and to rebuild a union in the frozen order: a frozen key's
// entry from the frozen version, another key's from the stable definition,
// a key the stable definition dropped left out, then the stable entries the
// order does not name, in their order.
func TestApplyChange(t *testing.T) {
	t.Parallel()
	entry := func(v, desc string) *Definition {
		return &Definition{Type: "object", Description: desc, Properties: map[string]*Definition{"kind": {Type: "string", Const: v}}}
	}
	union := func(entries ...*Definition) *Definition {
		return &Definition{Discriminator: &Discriminator{PropertyName: "kind"}, OneOf: entries,
			Properties: map[string]*Definition{"kind": {Type: "string"}}}
	}
	frozen := union(entry("a", "frozen a"), entry("n", "frozen n"), entry("b", "frozen b"), entry("gone", ""))
	frozen.Properties["extra"] = &Definition{Type: "integer"}
	stable := union(entry("a", "stable a"), entry("b", "stable b"), entry("c", "stable c"), &Definition{Type: "null"})
	c := &ManifestChange{Properties: []string{"extra"},
		Entries: []string{"const kind=a", "const kind=n", "const kind=b", "const kind=gone"}, FrozenEntries: []string{"const kind=n"}}
	d, err := applyChange(stable, frozen, c)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range d.OneOf {
		got = append(got, UnionEntryKey(e, "kind")+" "+e.Description)
	}
	if want := []string{"const kind=a stable a", "const kind=n frozen n", "const kind=b stable b", "const kind=c stable c", " "}; !slices.Equal(got, want) {
		t.Errorf("entries %q, want %q", got, want)
	}
	if p := d.Properties["extra"]; p == nil || p.Type != "integer" {
		t.Errorf("extra: %+v", p)
	}
	if len(stable.OneOf) != 4 || stable.Properties["extra"] != nil {
		t.Error("applyChange changed the stable definition")
	}

	for _, tt := range []struct {
		name    string
		s, u    *Definition
		c       *ManifestChange
		wantErr string
	}{
		{"no stable", nil, frozen, c, "the stable schema has no such definition"},
		{"no frozen", stable, nil, c, "$defs does not hold its frozen version"},
		{"null change", stable, frozen, nil, "the change is null"},
		{"missing property", stable, frozen, &ManifestChange{Properties: []string{"zz"}}, `no property "zz"`},
		{"missing frozen entry", stable, frozen, &ManifestChange{Entries: []string{"const kind=q"}, FrozenEntries: []string{"const kind=q"}}, `no entry "const kind=q"`},
	} {
		if _, err := applyChange(tt.s, tt.u, tt.c); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("%s: %v, want %q", tt.name, err, tt.wantErr)
		}
	}
}

// TestManifestChangeRefusesUnkeyedEntries requires the writer to refuse a
// changed union whose entries cannot all be named, or share a name, since
// their order could not be frozen.
func TestManifestChangeRefusesUnkeyedEntries(t *testing.T) {
	t.Parallel()
	c := &Changes{AddedEntries: []int{1}}
	for _, tt := range []struct {
		name    string
		u       *Definition
		wantErr string
	}{
		{"unkeyed", &Definition{AnyOf: []*Definition{ref("A"), {Type: "null"}}}, "union entry 1 has no key"},
		{"shared", &Definition{AnyOf: []*Definition{ref("A"), ref("A")}}, `share the key "ref #/$defs/A"`},
	} {
		if _, err := manifestChange(tt.u, c); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("%s: %v, want %q", tt.name, err, tt.wantErr)
		}
	}
}

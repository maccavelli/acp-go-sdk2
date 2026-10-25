package main

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// TestUnstableManifestComplete requires the repository's manifest and the
// unstable-origin API of acp to name each other: every type, field, variant
// and method the model marks unstable has its manifest entry, and every entry
// is generated (0039-PLAN-five-adopter-additions-on-schema-1-25-0.md, 2c).
func TestUnstableManifestComplete(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(repoRoot, "schema")
	man, found, err := load.ReadUnstableManifest(dir)
	if err != nil || !found {
		t.Fatalf("the repository's manifest: found %v, %v", found, err)
	}
	stable, err := load.ReadSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, merged, _, err := loadSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := buildModel(t, dir)

	types, fields, variants, methods := modelMarks(t, m)
	wantFields, wantVariants := manifestMarks(t, man, stable, merged, m)
	var wantMethods []string
	for side, entries := range map[string]map[string]string{"agent": man.Methods.Agent, "client": man.Methods.Client, "protocol": man.Methods.Protocol} {
		for key, wire := range entries {
			wantMethods = append(wantMethods, side+" "+key+" "+wire)
		}
	}
	for _, tt := range []struct {
		what      string
		got, want []string
	}{
		{"types", types, slices.Collect(maps.Keys(man.Definitions))},
		{"fields", fields, wantFields},
		{"variants", variants, wantVariants},
		{"methods", methods, wantMethods},
	} {
		slices.Sort(tt.got)
		slices.Sort(tt.want)
		if !slices.Equal(tt.got, tt.want) {
			t.Errorf("unstable %s and the manifest differ:\n model    %q\n manifest %q", tt.what, tt.got, tt.want)
		}
	}
	if len(types) == 0 || len(fields) == 0 || len(variants) == 0 || len(methods) == 0 {
		t.Errorf("found %d types, %d fields, %d variants, %d methods; the committed manifest has each", len(types), len(fields), len(variants), len(methods))
	}

	// While the schema is the one the manifest was frozen from, the manifest
	// is what that schema's merge gives, so an entry missing from it, or one
	// it holds that the merge does not, fails here by name. After a bump, the
	// live unstable schema is expected to differ.
	version, err := os.ReadFile(filepath.Join(dir, "version"))
	if err != nil {
		t.Fatal(err)
	}
	if v := strings.TrimSpace(string(version)); v != man.Schema {
		t.Logf("schema %s, manifest frozen from %s: the manifest is not compared with the live merge", v, man.Schema)
		return
	}
	if err := load.ManifestReproduces(dir, man); err != nil {
		t.Errorf("the manifest is not what schema %s's merge gives: %v", man.Schema, err)
	}
}

// modelMarks lists what m marks unstable: types by name, fields as
// Type.key, variants as Type[index], and methods as "side key wire". A type
// marked unstable must be generated and come from the manifest.
func modelMarks(t *testing.T, m *model.Model) (types, fields, variants, methods []string) {
	t.Helper()
	for _, ty := range m.Types {
		if ty.Unstable {
			types = append(types, ty.Name)
			if ty.Kind == model.KindNone || ty.Origin.File != load.ManifestFile {
				t.Errorf("%s: kind %v, origin %+v; want a generated type from %s", ty.Name, ty.Kind, ty.Origin, load.ManifestFile)
			}
		}
		for _, f := range ty.Fields {
			if f.Unstable {
				fields = append(fields, ty.Name+"."+f.Key)
			}
		}
		for i, v := range ty.Variants {
			if v.Unstable {
				variants = append(variants, ty.Name+"["+strconv.Itoa(i)+"]")
			}
		}
	}
	for _, mi := range slices.Concat(m.AgentMethods, m.ClientMethods, m.Protocol) {
		if mi.Unstable {
			methods = append(methods, mi.Side+" "+mi.Key+" "+mi.Wire)
		}
	}
	return types, fields, variants, methods
}

// manifestMarks lists, in modelMarks's form, the fields and variants the
// manifest's changes add to the stable definitions: a listed property the
// stable definition lacks, and a frozen entry it lacks, at its index in the
// merged union. Every listed property and frozen entry must be generated.
func manifestMarks(t *testing.T, man *load.Manifest, stable, merged *load.Schema, m *model.Model) (fields, variants []string) {
	t.Helper()
	for _, name := range slices.Sorted(maps.Keys(man.Changes)) {
		c, s, d, ty := man.Changes[name], stable.Defs[name], merged.Defs[name], m.Type(name)
		if s == nil || d == nil || ty == nil {
			t.Errorf("change of %s: stable %v, merged %v, type %v", name, s != nil, d != nil, ty != nil)
			continue
		}
		for _, key := range c.Properties {
			if !slices.ContainsFunc(ty.Fields, func(f *model.Field) bool { return f.Key == key }) {
				t.Errorf("%s.%s is in the manifest and not generated", name, key)
			}
			if _, ok := s.Properties[key]; !ok {
				fields = append(fields, name+"."+key)
			}
		}
		disc := ""
		if d.Discriminator != nil {
			disc = d.Discriminator.PropertyName
		}
		list, stableList := d.OneOf, s.OneOf
		if len(d.AnyOf) > 0 {
			list, stableList = d.AnyOf, s.AnyOf
		}
		keyed := func(key string) func(*load.Definition) bool {
			return func(e *load.Definition) bool { return load.UnionEntryKey(e, disc) == key }
		}
		for _, key := range c.FrozenEntries {
			i := slices.IndexFunc(list, keyed(key))
			switch {
			case i < 0:
				t.Errorf("%s entry %q is in the manifest and not generated", name, key)
			case !slices.ContainsFunc(stableList, keyed(key)):
				variants = append(variants, name+"["+strconv.Itoa(i)+"]")
			}
		}
	}
	return fields, variants
}

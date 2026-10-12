package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/enginetest"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// TestHelperParams requires every constructor of the committed schema and
// of each accepting engine case to take the required keys of its variant's
// Go type: its parameters' fields, with the discriminator field when the
// constructor writes the variant's value, are exactly that type's required
// fields (0014-PLAN-union-constructors-take-every-required-key.md, step 3).
// A catch-all's discriminator is not counted: the constructor neither
// writes nor takes it until step 5.
func TestHelperParams(t *testing.T) {
	t.Parallel()
	models := map[string]*model.Model{"committed schema": buildModel(t, filepath.Join(repoRoot, "schema"))}
	for _, path := range engineCases(t) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		a, err := enginetest.ParseArchive(b)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := a.File("want.api"); !ok {
			continue
		}
		dir := t.TempDir()
		for _, f := range a.Files {
			if strings.HasSuffix(f.Name, ".json") {
				if err := os.WriteFile(filepath.Join(dir, f.Name), f.Data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}
		models[filepath.Base(path)] = buildModel(t, dir)
	}
	var helpers int
	for name, m := range models {
		for _, typ := range m.Types {
			for _, h := range typ.Helpers {
				helpers++
				vi := typ.Variants[h.Variant]
				var got []string
				for _, p := range h.Params {
					got = append(got, p.Field)
				}
				if typ.DiscField != "" && vi.DiscValue != "" {
					got = append(got, typ.DiscField)
				}
				want := requiredFields(m, vi)
				if h.Variant == typ.CatchAll {
					want = slices.DeleteFunc(want, func(f string) bool { return f == typ.DiscField })
				}
				if missing, extra := setDiff(want, got), setDiff(got, want); len(missing) > 0 || len(extra) > 0 {
					t.Errorf("%s: %s: missing the required fields %v, has the fields %v the type does not require", name, h.Name, missing, extra)
				}
			}
		}
	}
	if helpers == 0 {
		t.Fatal("no constructor was checked")
	}
}

// buildModel builds the model of the schema directory dir as run does.
func buildModel(t *testing.T, dir string) *model.Model {
	t.Helper()
	meta, schema, prov, err := loadSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := model.Build(model.Input{Meta: meta, Schema: schema, Provenance: prov, Protocol: &model.ACP})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// requiredFields returns the Go names of the required fields of the type
// that holds variant vi: its inline struct, or the definition it names.
func requiredFields(m *model.Model, vi *model.Variant) []string {
	var fields []*model.Field
	switch {
	case vi.Inline != nil:
		fields = vi.Inline.Fields
	case m.Type(vi.TypeName) != nil:
		fields = m.Type(vi.TypeName).Fields
	}
	var out []string
	for _, f := range fields {
		if f.Required {
			out = append(out, f.Name)
		}
	}
	return out
}

// setDiff returns the elements of a that b does not hold, sorted.
func setDiff(a, b []string) []string {
	var out []string
	for _, x := range a {
		if !slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	slices.Sort(out)
	return out
}

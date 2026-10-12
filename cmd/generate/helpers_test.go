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
// fields, a catch-all's discriminator among them. A Value constructor takes
// one parameter, value, of the type the variant's type is declared with
// (0014-PLAN-union-constructors-take-every-required-key.md, steps 3 and 5).
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
				if h.Value {
					if want := valueType(m, vi); len(h.Params) != 1 || h.Params[0].Name != "value" || h.Params[0].Type.String() != want {
						t.Errorf("%s: %s: parameters %+v, want one, value %s", name, h.Name, h.Params, want)
					}
					continue
				}
				var got []string
				for _, p := range h.Params {
					got = append(got, p.Field)
				}
				if typ.DiscField != "" && vi.DiscValue != "" {
					got = append(got, typ.DiscField)
				}
				want := requiredFields(m, vi)
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

// valueType is the parameter type of a Value constructor for vi: the type
// of its inline named type, the underlying type of a named primitive it
// refers to, or the type it refers to.
func valueType(m *model.Model, vi *model.Variant) string {
	if vi.Inline != nil {
		return vi.Inline.Type.String()
	}
	if target := m.Type(vi.TypeName); target != nil && target.Kind == model.KindPrimitive {
		return target.Underlying.String()
	}
	return vi.TypeName
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

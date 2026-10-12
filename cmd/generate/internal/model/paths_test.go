package model

import (
	"slices"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

// pathDefs has a property of each shape an absolute-path rule takes, a
// definition an inline union variant merges through allOf, and a property
// of each shape a rule refuses.
const pathDefs = `{
	"Req":{"type":"object","properties":{"cwd":{"type":"string"},"dirs":{"type":"array","items":{"type":"string"}},
		"opt":{"type":["string","null"]},"n":{"type":"integer"},"e":{"type":"string","enum":["a"]},"ns":{"type":"array","items":{"type":"integer"}}},
		"required":["cwd"]},
	"Diff":{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]},
	"Content":{"oneOf":[
		{"type":"object","properties":{"type":{"const":"diff","type":"string"}},"required":["type"],"allOf":[{"$ref":"#/$defs/Diff"}]},
		{"type":"object","properties":{"type":{"const":"x","type":"string"}},"required":["type"]}]}}`

func pathRule(def, prop string) PathRule {
	return PathRule{Definition: def, Property: prop, Source: "s"}
}

// absolutePathChecks returns, by struct, the fields Validate checks as
// absolute paths and their shapes.
func absolutePathChecks(m *Model) map[string][]string {
	out := map[string][]string{}
	add := func(name string, v *Validation) {
		if v == nil {
			return
		}
		for _, c := range v.Checks {
			if c.Kind == CheckAbsolutePath {
				out[name] = append(out[name], c.Field+"/"+map[RefKind]string{RefNamed: "value", RefPointer: "pointer", RefSlice: "slice"}[c.Shape])
			}
		}
	}
	for _, t := range m.Types {
		add(t.Name, t.Validation)
		for _, vi := range t.Variants {
			if vi.Inline != nil {
				add(vi.TypeName, vi.Inline.Validation)
			}
		}
	}
	return out
}

// TestAbsolutePathRules requires a rule to check its property in each shape,
// in the definition's type and in an inline variant built from it through
// allOf, and to be skipped where the schema lacks its definition or property
// (0015-PLAN-validate-absolute-protocol-paths.md, step 4).
func TestAbsolutePathRules(t *testing.T) {
	t.Parallel()
	p := &Protocol{AbsolutePaths: []PathRule{
		pathRule("Req", "cwd"), pathRule("Req", "dirs"), pathRule("Req", "opt"), pathRule("Diff", "path"),
		pathRule("Absent", "x"), pathRule("Req", "missing"),
	}}
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, pathDefs), Protocol: p})
	if err != nil {
		t.Fatal(err)
	}
	var applied []string
	for _, pp := range m.AbsolutePaths {
		applied = append(applied, pp.Type+"."+pp.Property)
	}
	if want := []string{"Req.cwd", "Req.dirs", "Req.opt", "Diff.path"}; !slices.Equal(applied, want) {
		t.Errorf("rules applied %v, want %v", applied, want)
	}
	got := absolutePathChecks(m)
	want := map[string][]string{
		"Req":         {"Cwd/value", "Dirs/slice", "Opt/pointer"},
		"Diff":        {"Path/value"},
		"ContentDiff": {"Path/value"},
	}
	for name, w := range want {
		if c := got[name]; !slices.Equal(c, w) {
			t.Errorf("%s: absolute-path checks %v, want %v", name, c, w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("absolute-path checks in %v, want only %v", got, want)
	}
	for _, f := range m.Type("Req").Fields {
		if f.Facts.AbsolutePath != slices.Contains([]string{"cwd", "dirs", "opt"}, f.Key) {
			t.Errorf("Req.%s: Facts.AbsolutePath %v", f.Key, f.Facts.AbsolutePath)
		}
	}
	if !m.Validates("Diff") || !m.Validates("ContentDiff") {
		t.Error("a type whose only check is a path check does not validate")
	}
}

// TestAbsolutePathRuleRefusals requires Build to refuse a rule on a property
// that is not a string, an optional string, or an array of strings, and the
// table to refuse an incomplete or repeated rule.
func TestAbsolutePathRuleRefusals(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		rules []PathRule
		want  []string
	}{
		{"integer", []PathRule{pathRule("Req", "n")}, []string{"Req.n", "an absolute-path rule needs a string"}},
		{"enum", []PathRule{pathRule("Req", "e")}, []string{"Req.e", "an absolute-path rule needs a string"}},
		{"array of integers", []PathRule{pathRule("Req", "ns")}, []string{"Req.ns", "an absolute-path rule needs a string"}},
		{"no source", []PathRule{{Definition: "Req", Property: "cwd"}}, []string{`absolute-path rule "Req"."cwd"`, "needs a definition, a property, and a source"}},
		{"no property", []PathRule{{Definition: "Req", Source: "s"}}, []string{"needs a definition, a property, and a source"}},
		{"two rules", []PathRule{pathRule("Req", "cwd"), pathRule("Req", "cwd")}, []string{"absolute-path rule Req.cwd", "two rules"}},
	} {
		_, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, pathDefs), Protocol: &Protocol{AbsolutePaths: tt.rules}})
		if err == nil {
			t.Errorf("%s: Build accepted the table", tt.name)
			continue
		}
		for _, w := range tt.want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: error %q does not contain %q", tt.name, err, w)
			}
		}
	}
}

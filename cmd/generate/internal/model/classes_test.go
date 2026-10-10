package model

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

// committedModel builds the model of the schema in the repository.
func committedModel(t *testing.T) *Model {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "..", "schema")
	sm, err := load.ReadMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	ss, err := load.ReadSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	um, _, err := load.ReadMetaUnstable(dir)
	if err != nil {
		t.Fatal(err)
	}
	us, _, err := load.ReadSchemaUnstable(dir)
	if err != nil {
		t.Fatal(err)
	}
	meta, schema, prov, err := load.MergeStableAndUnstable(sm, ss, um, us)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Build(Input{Meta: meta, Schema: schema, Provenance: prov, Protocol: &ACP})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestUnionClassesOfCommittedSchema checks the classes of
// 0011-PLAN-wire-fixes.md, W1, against the schema in the repository.
func TestUnionClassesOfCommittedSchema(t *testing.T) {
	t.Parallel()
	m := committedModel(t)
	var closed, catchAll, defaulted, keyLess, withUnknown []string
	catchAllTypes := map[string]string{}
	for _, ty := range m.Types {
		if ty.Kind != KindUnion {
			continue
		}
		if ty.DiscKey == "" {
			if ty.CatchAll != -1 || ty.HasUnknown {
				t.Errorf("%s has no discriminator but CatchAll %d HasUnknown %v", ty.Name, ty.CatchAll, ty.HasUnknown)
			}
			continue
		}
		hasKeyLess := false
		for _, v := range ty.Variants {
			if v.KeyLess {
				hasKeyLess = true
				keyLess = append(keyLess, ty.Name+"."+v.TypeName)
			}
		}
		switch {
		case ty.CatchAll >= 0:
			catchAll = append(catchAll, ty.Name)
			v := ty.Variants[ty.CatchAll]
			catchAllTypes[ty.Name] = v.TypeName
			if v.Inline == nil || !v.Inline.KeepExtra {
				t.Errorf("%s: catch-all %s does not keep extra keys", ty.Name, v.TypeName)
			}
		case hasKeyLess:
			defaulted = append(defaulted, ty.Name)
		default:
			closed = append(closed, ty.Name)
		}
		if ty.HasUnknown {
			withUnknown = append(withUnknown, ty.Name)
		}
	}
	check := func(what string, got, want []string) {
		t.Helper()
		if !slices.Equal(got, want) {
			t.Errorf("%s:\n got %v\nwant %v", what, got, want)
		}
	}
	check("closed", closed, []string{"ContentBlock", "PlanUpdateContent", "RequestPermissionOutcome", "SessionConfigOption", "SessionUpdate",
		"ToolCallContent", "UnstableNesSuggestion"})
	check("with a catch-all", catchAll, []string{"CreateElicitationRequest", "CreateElicitationResponse", "ElicitationPropertySchema",
		"IdleStateUpdate", "MultiSelectItems", "StateUpdate"})
	check("defaulted", defaulted, []string{"AuthMethod", "McpServer", "SetSessionConfigOptionRequest", "UnstableMcpServer"})
	check("key-less variants", keyLess, []string{"AuthMethod.AuthMethodAgent", "IdleStateUpdate.IdleStateUpdateNone", "McpServer.McpServerStdio",
		"MultiSelectItems.TitledMultiSelectItems", "SetSessionConfigOptionRequest.SetSessionConfigOptionValueId", "UnstableMcpServer.McpServerStdio"})
	check("with Unknown", withUnknown, slices.Sorted(slices.Values(append(slices.Clone(closed), defaulted...))))
	for union, other := range catchAllTypes {
		if !strings.HasSuffix(other, "Other") {
			t.Errorf("%s: catch-all is %s", union, other)
		}
	}
}

// TestFitKeysOfCommittedSchema requires each variant of a discriminated union
// to list its required keys other than the discriminator, in schema order,
// and no variant of another union to list any.
func TestFitKeysOfCommittedSchema(t *testing.T) {
	t.Parallel()
	m := committedModel(t)
	want := map[string][]string{
		"ContentBlock.Text":                     {"text"},
		"ContentBlock.Image":                    {"data", "mimeType"},
		"SessionUpdate.SessionInfoUpdate":       nil,
		"SessionUpdate.ToolCallUpdate":          {"toolCallId"},
		"RequestPermissionOutcome.Cancelled":    nil,
		"RequestPermissionOutcome.Selected":     {"optionId"},
		"AuthMethod.Agent":                      {"id", "name"},
		"IdleStateUpdate.None":                  nil,
		"IdleStateUpdate.Other":                 nil,
		"CreateElicitationRequest.Url":          {"elicitationId", "url"},
		"SetSessionConfigOptionRequest.ValueId": {"value"},
	}
	seen := 0
	for _, ty := range m.Types {
		if ty.Kind != KindUnion {
			continue
		}
		for _, v := range ty.Variants {
			if v.Skip {
				continue
			}
			if ty.DiscKey == "" {
				if v.FitKeys != nil {
					t.Errorf("%s.%s: FitKeys %v in a union without a discriminator", ty.Name, v.FieldName, v.FitKeys)
				}
				continue
			}
			rest := slices.DeleteFunc(slices.Clone(v.Required), func(k string) bool { return k == ty.DiscKey })
			if !slices.Equal(v.FitKeys, rest) {
				t.Errorf("%s.%s: FitKeys %v, want %v", ty.Name, v.FieldName, v.FitKeys, rest)
			}
			if w, ok := want[ty.Name+"."+v.FieldName]; ok {
				seen++
				if !slices.Equal(v.FitKeys, w) {
					t.Errorf("%s.%s: FitKeys %v, want %v", ty.Name, v.FieldName, v.FitKeys, w)
				}
			}
		}
	}
	if seen != len(want) {
		t.Errorf("found %d of the %d listed variants", seen, len(want))
	}
}

// TestCatchAllNeedsNot requires the schema's not before a value-less variant
// counts as the catch-all.
func TestCatchAllNeedsNot(t *testing.T) {
	t.Parallel()
	m, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, `{"U":{"anyOf":[
		{"type":"object","required":["type"],"properties":{"type":{"const":"a","type":"string"}}},
		{"title":"loose","type":"object","required":["type"],"properties":{"type":{"type":"string"}}}]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	u := m.Type("U")
	if u.CatchAll != -1 || !u.HasUnknown || !u.Variants[1].KeyLess {
		t.Errorf("CatchAll %d HasUnknown %v KeyLess %v; want -1 true true", u.CatchAll, u.HasUnknown, u.Variants[1].KeyLess)
	}
}

func TestUnionClassRefusals(t *testing.T) {
	t.Parallel()
	const known = `{"type":"object","required":["type"],"properties":{"type":{"const":"a","type":"string"}}}`
	const other = `{"title":"other","type":"object","required":["type"],"properties":{"type":{"type":"string"}},"not":{"required":["type"]}}`
	tests := []struct {
		name, defs string
		want       []string
	}{
		{"two catch-alls", `{"U":{"anyOf":[` + known + `,` + other + `,` + strings.Replace(other, `"other"`, `"other2"`, 1) + `]}}`, []string{"U variant 3", "variant 2 is already the catch-all"}},
		{"array variant in a discriminated union", `{"U":{"anyOf":[` + known + `,{"title":"list","type":"array","items":{"type":"string"}}]}}`,
			[]string{"U variant 2", `discriminated by "type" must be an object`}},
		{"variant field named Unknown", `{"U":{"oneOf":[{"type":"object","required":["type"],"properties":{"type":{"const":"unknown","type":"string"}}},` + known + `]}}`,
			[]string{"U:", "takes the field Unknown"}},
		{"catch-all property named extra", `{"U":{"anyOf":[` + known + `,{"title":"other","type":"object","required":["type"],
			"properties":{"type":{"type":"string"},"extra":{"type":"string"}},"not":{"required":["type"]}}]}}`, []string{"U variant 2", `property "extra" becomes field Extra`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Build(Input{Meta: &load.Meta{}, Schema: schemaOf(t, tt.defs)})
			if err == nil {
				t.Fatal("Build accepted the schema")
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}

// TestDiscriminatorExemptOnDecode requires every inline variant of a
// discriminated union to exempt the discriminator from its required keys, and
// no $ref variant's own type to require it, since a payload placed without
// the discriminator decodes through the variant's type.
func TestDiscriminatorExemptOnDecode(t *testing.T) {
	t.Parallel()
	m := committedModel(t)
	inline, named := 0, 0
	for _, ty := range m.Types {
		if ty.Kind != KindUnion {
			continue
		}
		for _, v := range ty.Variants {
			if v.Skip {
				continue
			}
			switch {
			case v.Inline != nil:
				inline++
				if v.Inline.Exempt != ty.DiscKey {
					t.Errorf("%s.%s: Exempt %q, want %q", ty.Name, v.FieldName, v.Inline.Exempt, ty.DiscKey)
				}
			case ty.DiscKey != "":
				named++
				for _, f := range m.Type(v.TypeName).Fields {
					if f.Key == ty.DiscKey && f.Required {
						t.Errorf("%s: variant type %s requires the discriminator %q", ty.Name, v.TypeName, ty.DiscKey)
					}
				}
			}
		}
	}
	if inline == 0 || named == 0 {
		t.Errorf("checked %d inline and %d named variants", inline, named)
	}
}

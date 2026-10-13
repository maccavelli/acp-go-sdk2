package model

import (
	"strings"
	"testing"
)

// gatesOf returns the checks of type name's gates, or nil.
func gatesOf(m *Model, name string) []GateCheck {
	for _, g := range m.Gated {
		if g.Type == name {
			return g.Checks
		}
	}
	return nil
}

// TestCapabilityRulesApply requires a variant rule on a union, and one on the
// union a property holds, to give those types gates with the capability's Go
// path, and a type holding a gated one to call into it; a rule whose
// definition or property the schema lacks, and every rule of a schema without
// the initialize rule, to apply nothing
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 10).
func TestCapabilityRulesApply(t *testing.T) {
	t.Parallel()
	rules := []CapabilityRule{
		{Side: "agent", Definition: "Verdict", Variant: "chosen", Capability: "session.list", Source: "s", Kind: GateVariant},
		{Side: "agent", Definition: "AskResponse", Property: "verdict", Variant: "withdrawn", Capability: "loadSession", Source: "s", Kind: GateVariant},
		{Side: "agent", Definition: "Nope", Variant: "x", Capability: "loadSession", Source: "s", Kind: GateVariant},
		{Side: "agent", Definition: "PromptRequest", Property: "nope", Capability: "loadSession", Source: "s", Kind: GateNonEmpty},
	}
	p := testProtocol()
	p.CapabilityRules = rules
	m, err := Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, protocolDefs), Protocol: p})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Gated) != 2 {
		t.Fatalf("%d gated types, want Verdict and AskResponse: %+v", len(m.Gated), m.Gated)
	}
	v := gatesOf(m, "Verdict")
	if len(v) != 1 || v[0].Kind != GateCheckVariant || v[0].Field != "" || v[0].Value != "chosen" || v[0].Variant == "" ||
		len(v[0].Steps) != 2 || v[0].Steps[1] != (CapabilityStep{Field: "List", Pointer: true}) || v[0].FromRequest {
		t.Errorf("Verdict: %+v", v)
	}
	a := gatesOf(m, "AskResponse")
	if len(a) != 2 || a[0].Kind != GateCheckVariant || a[0].Field != "Verdict" || a[0].Key != "verdict" || a[0].Shape != RefNamed ||
		a[0].Value != "withdrawn" || len(a[0].Steps) != 1 || a[0].Steps[0] != (CapabilityStep{Field: "LoadSession"}) ||
		a[1].Kind != GateCheckNested || a[1].Field != "Verdict" || a[1].Shape != RefNamed {
		t.Errorf("AskResponse: %+v", a)
	}

	p = testProtocol()
	p.Methods = p.Methods[:5]
	p.CapabilityRules = rules
	plain, err := Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, protocolDefs), Protocol: p})
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.Gated) != 0 {
		t.Errorf("without the initialize rule, %d types have gates", len(plain.Gated))
	}
}

// TestCapabilityRulesRefuse requires the table's checks of a rule's fields,
// and Build's of its subject.
func TestCapabilityRulesRefuse(t *testing.T) {
	t.Parallel()
	ok := CapabilityRule{Side: "agent", Definition: "Verdict", Variant: "chosen", Capability: "loadSession", Source: "s", Kind: GateVariant}
	with := func(f func(r *CapabilityRule)) []CapabilityRule {
		r := ok
		f(&r)
		return []CapabilityRule{r}
	}
	for _, tt := range []struct {
		name  string
		rules []CapabilityRule
		want  string
	}{
		{"no kind", with(func(r *CapabilityRule) { r.Kind = 0 }), `capability rule Verdict "chosen": no gate kind`},
		{"unknown side", with(func(r *CapabilityRule) { r.Side = "protocol" }), `side must be "agent" or "client"`},
		{"no source", with(func(r *CapabilityRule) { r.Source = "" }), "it lacks a field its kind needs"},
		{"variant without a capability", with(func(r *CapabilityRule) { r.Capability = "" }), "it lacks a field its kind needs"},
		{"non-empty without a property", with(func(r *CapabilityRule) { r.Kind, r.Variant = GateNonEmpty, "" }), "it lacks a field its kind needs"},
		{"terminal auth with a capability", with(func(r *CapabilityRule) { r.Kind, r.Property = GateNotTerminalAuthMethod, "id" }),
			"or has one it does not take"},
		{"two rules", []CapabilityRule{ok, ok}, `capability rule Verdict "chosen": two rules`},
		{"a variant the union lacks", with(func(r *CapabilityRule) { r.Variant = "maybe" }), `Verdict has no variant "maybe"`},
		{"a variant of a type that is not a union", with(func(r *CapabilityRule) { r.Definition = "Update" }), "Update is not a union"},
		{"non-empty on a property that is not an array", with(func(r *CapabilityRule) {
			r.Definition, r.Property, r.Variant, r.Kind = "PromptRequest", "sessionId", "", GateNonEmpty
		}), "sessionId is SessionId, not an array"},
		{"a capability the schema lacks", with(func(r *CapabilityRule) { r.Capability = "nope" }),
			`capability "nope": AgentCapabilities has no property "nope"`},
		{"terminal auth without a list", with(func(r *CapabilityRule) {
			r.Definition, r.Property, r.Variant, r.Capability, r.Kind = "PromptRequest", "sessionId", "terminal", "", GateNotTerminalAuthMethod
		}), `InitResponse has no array of a union whose variant "terminal" holds a SessionId`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := testProtocol()
			p.CapabilityRules = tt.rules
			_, err := Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, protocolDefs), Protocol: p})
			if err == nil || !strings.Contains(err.Error(), "protocol table: ") || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Build = %v, want an error containing %q", err, tt.want)
			}
		})
	}
}

// checkCommittedGates requires every capability rule of ACP to resolve on
// the committed schema, as one check of its definition's gates.
func checkCommittedGates(t *testing.T, m *Model) {
	t.Helper()
	own := map[string][]GateCheck{}
	n := 0
	for _, g := range m.Gated {
		for _, c := range g.Checks {
			if c.Kind != GateCheckNested {
				own[g.Type] = append(own[g.Type], c)
				n++
			}
		}
	}
	if n != len(ACP.CapabilityRules) || n != 18 {
		t.Errorf("%d capability rules resolve of %d, want 18: %v", n, len(ACP.CapabilityRules), own)
	}
	for _, r := range ACP.CapabilityRules {
		found := false
		for _, c := range own[r.Definition] {
			found = found || (c.Value == r.Variant && c.Capability == r.Capability && (r.Property == "" || c.Key == r.Property))
		}
		if !found {
			t.Errorf("%s: no check of %s", r.name(), r.Definition)
		}
	}
	if a := own["AuthenticateRequest"]; len(a) != 1 || a[0].Kind != GateCheckTerminalAuth || a[0].List != "AuthMethods" ||
		a[0].Variant != "Terminal" || a[0].ID != "Id" || a[0].Field != "MethodId" {
		t.Errorf("AuthenticateRequest: %+v", a)
	}
}

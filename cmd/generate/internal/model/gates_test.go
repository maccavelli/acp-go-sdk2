package model

import (
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
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

	// A copy the merge made of a rule's definition under another name gets
	// the rule too (0019-PLAN-behaviour-fixes.md, B6).
	copied := strings.Replace(protocolDefs, `"Update":`, `"UnstableVerdict":{"oneOf":[{"type":"object","required":["verdict"],"properties":{"verdict":{"const":"withdrawn","type":"string"}}},
		{"type":"object","required":["verdict","choice"],"properties":{"verdict":{"const":"chosen","type":"string"},"choice":{"type":"string"}}}],
		"discriminator":{"propertyName":"verdict"}},"Update":`, 1)
	prov := &load.Provenance{UnstableDefs: map[string]bool{"UnstableVerdict": true},
		Origins: map[string]load.Origin{"UnstableVerdict": {File: load.UnstableSchemaFile, Key: "Verdict"}}}
	p = testProtocol()
	p.CapabilityRules = rules[:1]
	withCopy, err := Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, copied), Provenance: prov, Protocol: p})
	if err != nil {
		t.Fatal(err)
	}
	if c := gatesOf(withCopy, "UnstableVerdict"); len(c) != 1 || c[0].Value != "chosen" || c[0].Capability != "session.list" {
		t.Errorf("UnstableVerdict, the unstable copy of Verdict: %+v", c)
	}
	if c := gatesOf(withCopy, "Verdict"); len(c) != 1 || c[0].Value != "chosen" {
		t.Errorf("Verdict beside its copy: %+v", c)
	}

	// An undeclared-variant rule resolves to the union's catch-all field and
	// its discriminator's field (0019-PLAN-api-decisions.md, B20).
	withOther := strings.Replace(protocolDefs, `"Update":`, `"Choice":{"anyOf":[
		{"type":"object","required":["kind"],"properties":{"kind":{"const":"plain","type":"string"}}},
		{"title":"other","type":"object","required":["kind"],"properties":{"kind":{"type":"string"}},
		 "not":{"anyOf":[{"type":"object","required":["kind"],"properties":{"kind":{"const":"plain","type":"string"}}}]}}],
		"discriminator":{"propertyName":"kind"}},"Update":`, 1)
	p = testProtocol()
	p.CapabilityRules = []CapabilityRule{{Side: "agent", Definition: "Choice", Capability: "session.list", Source: "s", Kind: GateUndeclaredVariant}}
	undeclared, err := Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, withOther), Protocol: p})
	if err != nil {
		t.Fatal(err)
	}
	if c := gatesOf(undeclared, "Choice"); len(c) != 1 || c[0].Kind != GateCheckUndeclaredVariant || c[0].Variant != "Other" ||
		c[0].Disc != "Kind" || c[0].Capability != "session.list" || c[0].Value != "" || c[0].Field != "" {
		t.Errorf("Choice: %+v", c)
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
		{"undeclared variant with a variant", with(func(r *CapabilityRule) { r.Kind = GateUndeclaredVariant }),
			"or has one it does not take"},
		{"undeclared variant without a capability", with(func(r *CapabilityRule) { r.Kind, r.Variant, r.Capability = GateUndeclaredVariant, "", "" }),
			"it lacks a field its kind needs"},
		{"undeclared variant of a union without a catch-all", with(func(r *CapabilityRule) { r.Kind, r.Variant = GateUndeclaredVariant, "" }),
			"Verdict is not a union with a catch-all variant"},
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
// the committed schema, as one check of its definition's gates and one of
// each unstable copy of that definition: the three MCP rules on
// UnstableMcpServer, the type of session/fork's servers
// (0019-PLAN-behaviour-fixes.md, B6); and the undeclared elicitation mode on
// the catch-all's mode (0019-PLAN-api-decisions.md, B20).
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
	if len(ACP.CapabilityRules) != 19 || n != 22 {
		t.Errorf("%d checks resolve from %d capability rules, want 22 from 19: %v", n, len(ACP.CapabilityRules), own)
	}
	for _, r := range ACP.CapabilityRules {
		for _, ty := range m.ruleTypes(r.Definition) {
			found := false
			for _, c := range own[ty.Name] {
				found = found || (c.Value == r.Variant && c.Capability == r.Capability && (r.Property == "" || c.Key == r.Property))
			}
			if !found {
				t.Errorf("%s: no check of %s", r.name(), ty.Name)
			}
		}
	}
	if u := own["UnstableMcpServer"]; len(u) != 3 {
		t.Errorf("UnstableMcpServer: %d checks, want the three MCP rules: %+v", len(u), u)
	}
	if a := own["AuthenticateRequest"]; len(a) != 1 || a[0].Kind != GateCheckTerminalAuth || a[0].List != "AuthMethods" ||
		a[0].Variant != "Terminal" || a[0].ID != "Id" || a[0].Field != "MethodId" {
		t.Errorf("AuthenticateRequest: %+v", a)
	}
	if e := own["CreateElicitationRequest"]; len(e) != 3 || e[2].Kind != GateCheckUndeclaredVariant || e[2].Variant != "Other" ||
		e[2].Disc != "Mode" || e[2].Capability != "elicitation" {
		t.Errorf("CreateElicitationRequest: %+v", e)
	}
}

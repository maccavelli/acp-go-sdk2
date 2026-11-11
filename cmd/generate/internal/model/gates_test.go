package model

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
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
	// Reached by a method's params, as a gate must be (0049-PLAN-tooling-fixes.md, phase 5, F-C).
	copied = strings.Replace(copied, `"SessionNotification":{"type":"object","x-side":"client","x-method":"s/update"}`,
		`"SessionNotification":{"type":"object","x-side":"client","x-method":"s/update","properties":{"verdict":{"$ref":"#/$defs/UnstableVerdict"}}}`, 1)
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
	withOther = strings.Replace(withOther, `"SessionNotification":{"type":"object","x-side":"client","x-method":"s/update"}`,
		`"SessionNotification":{"type":"object","x-side":"client","x-method":"s/update","properties":{"choice":{"$ref":"#/$defs/Choice"}}}`, 1)
	p = testProtocol()
	p.CapabilityRules = []CapabilityRule{{Side: "agent", Definition: "Choice", Capability: "session.list", Source: "s", Kind: GateUndeclaredVariant}}
	undeclared, err := Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, withOther), Protocol: p})
	if err != nil {
		t.Fatal(err)
	}
	if c := gatesOf(undeclared, "Choice"); len(c) != 1 || c[0].Kind != GateCheckUndeclaredVariant || c[0].Variant != "Other" ||
		c[0].Disc != "Kind" || c[0].Capability != "session.list" || c[0].Value != "" || c[0].Field != "" || c[0].RawKey != "" {
		t.Errorf("Choice: %+v", c)
	}
}

// TestCapabilityRulesApplyRawUnknown requires an undeclared-variant rule on a
// union without a catch-all variant, which keeps an undeclared value in its
// raw Unknown, to read the discriminator by its key: on the union itself, or
// on the union a property holds; and a capability that is a required object
// to name the prefix (0049-PLAN-surface-fixes.md, phase 4, S9).
func TestCapabilityRulesApplyRawUnknown(t *testing.T) {
	t.Parallel()
	p := testProtocol()
	p.CapabilityRules = []CapabilityRule{
		{Side: "agent", Definition: "Verdict", Capability: "session", Source: "s", Kind: GateUndeclaredVariant},
		{Side: "agent", Definition: "AskResponse", Property: "verdict", Capability: "session", Source: "s", Kind: GateUndeclaredVariant},
	}
	raw, err := Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, protocolDefs), Protocol: p})
	if err != nil {
		t.Fatal(err)
	}
	if c := gatesOf(raw, "Verdict"); len(c) != 1 || c[0].Kind != GateCheckUndeclaredVariant || c[0].Variant != "Unknown" ||
		c[0].RawKey != "verdict" || c[0].Disc != "" || c[0].Field != "" || c[0].Capability != "session" || len(c[0].Steps) != 0 {
		t.Errorf("Verdict, raw: %+v", c)
	}
	if c := gatesOf(raw, "AskResponse"); len(c) < 1 || c[0].Kind != GateCheckUndeclaredVariant || c[0].Variant != "Unknown" ||
		c[0].RawKey != "verdict" || c[0].Field != "Verdict" || c[0].Key != "verdict" || c[0].Shape != RefNamed {
		t.Errorf("AskResponse, raw: %+v", c)
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
		{"undeclared variant of a type that is not a union", with(func(r *CapabilityRule) {
			r.Definition, r.Kind, r.Variant = "Update", GateUndeclaredVariant, ""
		}), "Update is not a union with a catch-all variant or Unknown"},
		{"undeclared variant of a property that is not a union", with(func(r *CapabilityRule) {
			r.Definition, r.Property, r.Kind, r.Variant = "PromptRequest", "sessionId", GateUndeclaredVariant, ""
		}), "SessionId is not a union with a catch-all variant or Unknown"},
		{"undeclared variant naming a capability the schema lacks", with(func(r *CapabilityRule) {
			r.Kind, r.Variant, r.Capability = GateUndeclaredVariant, "", "nope"
		}), `capability "nope": AgentCapabilities has no property "nope"`},
		{"advertised auth without a property", with(func(r *CapabilityRule) { r.Kind, r.Variant, r.Capability = GateAdvertisedAuthMethod, "", "" }),
			"it lacks a field its kind needs"},
		{"advertised auth with a variant", with(func(r *CapabilityRule) { r.Kind, r.Property, r.Capability = GateAdvertisedAuthMethod, "id", "" }),
			"or has one it does not take"},
		{"advertised auth with a capability", with(func(r *CapabilityRule) { r.Kind, r.Property, r.Variant = GateAdvertisedAuthMethod, "id", "" }),
			"or has one it does not take"},
		{"advertised auth without a list", with(func(r *CapabilityRule) {
			r.Definition, r.Property, r.Variant, r.Capability, r.Kind = "PromptRequest", "sessionId", "", "", GateAdvertisedAuthMethod
		}), "InitResponse has no array of a union every variant of which holds a SessionId"},
		// The four kinds of 0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md, D5.
		{"present without a property", with(func(r *CapabilityRule) { r.Kind, r.Variant = GatePresent, "" }), "it lacks a field its kind needs"},
		{"present with a variant", with(func(r *CapabilityRule) { r.Kind, r.Property = GatePresent, "x" }), "or has one it does not take"},
		{"value without a variant", with(func(r *CapabilityRule) { r.Kind, r.Property, r.Variant = GateValue, "x", "" }),
			"it lacks a field its kind needs"},
		{"value without a capability", with(func(r *CapabilityRule) { r.Kind, r.Property, r.Capability = GateValue, "x", "" }),
			"it lacks a field its kind needs"},
		{"undeclared value with a variant", with(func(r *CapabilityRule) { r.Kind, r.Property = GateUndeclaredValue, "x" }),
			"or has one it does not take"},
		{"undeclared value without a capability", with(func(r *CapabilityRule) {
			r.Kind, r.Property, r.Variant, r.Capability = GateUndeclaredValue, "x", "", ""
		}), "it lacks a field its kind needs"},
		{"only variants without a property", with(func(r *CapabilityRule) { r.Kind = GateOnlyVariants }), "it lacks a field its kind needs"},
		{"only variants without a variant", with(func(r *CapabilityRule) { r.Kind, r.Property, r.Variant = GateOnlyVariants, "x", "" }),
			"it lacks a field its kind needs"},
		{"only variants with a prefix of one segment", with(func(r *CapabilityRule) {
			r.Definition, r.Property, r.Kind = "AskResponse", "verdict", GateOnlyVariants
		}), `capability "loadSession": an only-variants prefix needs a property under a capability`},
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
// the catch-all's mode (0019-PLAN-api-decisions.md, B20). The three subagent
// variants of SessionUpdate add three rules and three checks
// (0019-PLAN-issue-fixes.md, B24). Phase 4 of 0049-PLAN-surface-fixes.md adds
// eight of each: the boolean set request, the advertised auth method, the
// prompt's undeclared block, the two plan updates and the three NES kinds.
// 0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md, D5, adds six
// of each: the system prompt on the three session requests, and, on
// SystemPrompt, its override mode, its unlisted modes and its text-only
// content; no copy of SystemPrompt exists, so each resolves once.
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
	if len(ACP.CapabilityRules) != 36 || n != 39 {
		t.Errorf("%d checks resolve from %d capability rules, want 39 from 36: %v", n, len(ACP.CapabilityRules), own)
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
	checkAuthenticateGates(t, own["AuthenticateRequest"])
	if pr := own["PromptRequest"]; len(pr) != 4 || pr[3].Kind != GateCheckUndeclaredVariant || pr[3].Variant != "Unknown" ||
		pr[3].RawKey != "type" || pr[3].Field != "Prompt" || pr[3].Shape != RefSlice || pr[3].Capability != "promptCapabilities" {
		t.Errorf("PromptRequest: %+v", pr)
	}
	if e := own["CreateElicitationRequest"]; len(e) != 3 || e[2].Kind != GateCheckUndeclaredVariant || e[2].Variant != "Other" ||
		e[2].Disc != "Mode" || e[2].Capability != "elicitation" {
		t.Errorf("CreateElicitationRequest: %+v", e)
	}
}

// checkAuthenticateGates requires AuthenticateRequest's two checks: the
// terminal rule first, then the advertised one, which reads every variant's
// id and an Unknown entry's "id" (0049-PLAN-surface-fixes.md, phase 4, S8),
// and names the terminal variant its list of accepted ids leaves out
// (0050-PLAN-auth-method-ids-and-the-prompt-result-for-issue-14.md, phase 1).
func checkAuthenticateGates(t *testing.T, a []GateCheck) {
	t.Helper()
	if len(a) != 2 || a[0].Kind != GateCheckTerminalAuth || a[0].List != "AuthMethods" ||
		a[0].Variant != "Terminal" || a[0].ID != "Id" || a[0].Field != "MethodId" ||
		a[1].Kind != GateCheckAdvertisedAuth || a[1].List != "AuthMethods" || a[1].Field != "MethodId" || a[1].Elem != "AuthMethod" ||
		a[1].IDType != "AuthMethodId" || a[1].RawKey != "id" || a[1].Terminal != "Terminal" ||
		!slices.Equal(a[1].Advertised, []VariantField{{Variant: "Terminal", Field: "Id"}, {Variant: "Agent", Field: "Id"}}) {
		t.Errorf("AuthenticateRequest: %+v", a)
	}
}

// TestGatesDescendIntoArrayVariant requires a union whose variant is a named
// array of a gated type to have gates that call each item's, where it had
// none, and the type that holds the union to call into it
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G19, Q25 a).
func TestGatesDescendIntoArrayVariant(t *testing.T) {
	t.Parallel()
	defs := strings.Replace(protocolDefs, `"Update":`, `"Verdicts":{"anyOf":[{"title":"Many","type":"array","items":{"$ref":"#/$defs/Verdict"}},
		{"title":"None","type":"string"}]},"Update":`, 1)
	defs = strings.Replace(defs, `"SessionNotification":{"type":"object","x-side":"client","x-method":"s/update"}`,
		`"SessionNotification":{"type":"object","x-side":"client","x-method":"s/update","properties":{"verdicts":{"$ref":"#/$defs/Verdicts"}}}`, 1)
	p := testProtocol()
	p.CapabilityRules = []CapabilityRule{{Side: "agent", Definition: "Verdict", Variant: "chosen", Capability: "session.list", Source: "s", Kind: GateVariant}}
	m, err := Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, defs), Protocol: p})
	if err != nil {
		t.Fatal(err)
	}
	if c := gatesOf(m, "Verdicts"); len(c) != 1 || c[0].Kind != GateCheckNested || c[0].Field != "Many" || c[0].Shape != RefSlice || !c[0].Items {
		t.Errorf("Verdicts: %+v, want one nested check of the Many variant's items", c)
	}
	if c := gatesOf(m, "SessionNotification"); len(c) != 1 || c[0].Field != "Verdicts" {
		t.Errorf("SessionNotification: %+v, want its call into Verdicts", c)
	}
}

// TestCapabilityRuleOnCopiedProperty requires a capability rule whose property
// the definition holds but cannot gate everywhere to be refused, naming the
// copies: a union's property, which only its variants' structs hold, where the
// rule applied and gated nothing
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G20, Q28 a); and a
// definition's property that an allOf also copies into an inline variant,
// where the rule gated the definition alone (0049-PLAN-generator-fixes.md,
// phase 6, deviation 2).
func TestCapabilityRuleOnCopiedProperty(t *testing.T) {
	t.Parallel()
	dirs := `"dirs":{"type":"array","items":{"type":"string"}}`
	union := strings.Replace(protocolDefs, `"LoadSessionRequest":{"type":"object","x-side":"agent","x-method":"s/load"}`,
		`"LoadSessionRequest":{"type":"object","x-side":"agent","x-method":"s/load","properties":{`+dirs+`},
		"anyOf":[{"title":"A","type":"object","properties":{"a":{"type":"string"}}},{"title":"B","type":"object","properties":{"b":{"type":"string"}}}]}`, 1)
	copied := strings.Replace(protocolDefs, `"Update":`, `"D":{"type":"object","properties":{`+dirs+`}},
		"U":{"oneOf":[{"title":"Wrap","type":"object","allOf":[{"$ref":"#/$defs/D"}],"properties":{"w":{"type":"string"}}},{"title":"Other","type":"object","properties":{"x":{"type":"string"}}}]},"Update":`, 1)
	for _, tt := range []struct {
		name, defs, definition string
		want                   []string
	}{
		{"a union's property", union, "LoadSessionRequest",
			[]string{"capability rule LoadSessionRequest.dirs: the property is in copies (", "not a field of LoadSessionRequest"}},
		{"a property an allOf copies", copied, "D",
			[]string{"capability rule D.dirs: the property is also in copies (", "which the rule does not reach"}},
	} {
		p := testProtocol()
		p.CapabilityRules = []CapabilityRule{{Side: "agent", Definition: tt.definition, Property: "dirs", Capability: "loadSession", Source: "s", Kind: GateNonEmpty}}
		_, err := Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, tt.defs), Protocol: p})
		for _, w := range tt.want {
			if err == nil || !strings.Contains(err.Error(), w) {
				t.Errorf("%s: Build = %v, want %q", tt.name, err, w)
			}
		}
	}
}

// The schema's wordings that ask for a gate or a path rule
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G29): a method or a
// property available only with a capability; an update a side may send only
// when the peer advertised one; and a path that must be absolute.
var (
	capabilityWording = regexp.MustCompile("(?i)only available if the (agent|client) supports the")
	sendWording       = regexp.MustCompile("(?i)MUST only send this (update|notification|request)")
	absPathWording    = regexp.MustCompile("(?i)must be an absolute path")
)

// wordingGaps lists each place of m's schema whose description carries one of
// those wordings and that no rule of p answers. The capability and the send
// wordings ask for any gate that applies: a method gate on the definition's
// x-method, or a capability rule on the definition, the property, or the
// union variant, a variant's payload type included; the absolute-path wording
// asks for a path rule (0049-PLAN-generator-fixes.md, phase 8, deviation 1).
func wordingGaps(m *Model, p *Protocol) []string {
	w := newWordingIndex(m, p)
	var gaps []string
	for _, name := range slices.Sorted(maps.Keys(m.schema.Defs)) {
		if def := m.schema.Defs[name]; def != nil {
			gaps = append(gaps, w.gapsOf(name, def)...)
		}
	}
	return gaps
}

// wordingIndex is what p's rules answer, for wordingGaps.
type wordingIndex struct {
	m                     *Model
	methodGated, defRuled map[string]bool
	payload               map[string]bool
	paths, propRuled      map[[2]string]bool
	variantRuled          map[string]map[string]bool
}

func newWordingIndex(m *Model, p *Protocol) *wordingIndex {
	w := &wordingIndex{m: m, methodGated: map[string]bool{}, defRuled: map[string]bool{}, payload: map[string]bool{},
		paths: map[[2]string]bool{}, propRuled: map[[2]string]bool{}, variantRuled: map[string]map[string]bool{}}
	for _, r := range p.Methods {
		if r.Capability != "" {
			w.methodGated[r.Wire] = true
		}
	}
	for _, r := range p.AbsolutePaths {
		w.paths[[2]string{r.Definition, r.Property}] = true
	}
	for _, r := range p.CapabilityRules {
		switch {
		case r.Variant != "":
			union := unionOf(m, r.Definition, r.Property)
			if w.variantRuled[union] == nil {
				w.variantRuled[union] = map[string]bool{}
			}
			w.variantRuled[union][r.Variant] = true
		case r.Property != "":
			w.propRuled[[2]string{r.Definition, r.Property}] = true
		default:
			w.defRuled[r.Definition] = true
		}
	}
	// A gated variant's payload type is gated with it.
	for union := range w.variantRuled {
		def := m.schema.Defs[union]
		if def == nil {
			continue
		}
		members := slices.Concat(def.AnyOf, def.OneOf)
		for _, v := range members {
			if v == nil || !w.variantGated(union, v, members) {
				continue
			}
			for _, part := range append([]*load.Definition{v}, v.AllOf...) {
				if name, ok := strings.CutPrefix(part.Ref, refPrefix); ok {
					w.payload[name] = true
				}
			}
		}
	}
	return w
}

// variantGated reports whether a capability rule names the variant v of union
// by its discriminator value.
func (w *wordingIndex) variantGated(union string, v *load.Definition, members []*load.Definition) bool {
	key, _ := discriminatorKey(w.m.schema, w.m.schema.Defs[union], members)
	value, _ := discConst(expandAllOf(w.m.schema, v).Properties[key]).(string)
	return value != "" && w.variantRuled[union][value]
}

// gapsOf lists the gaps of the definition name: its own description, its
// properties', and its variants'.
func (w *wordingIndex) gapsOf(name string, def *load.Definition) []string {
	asks := func(text string) bool { return capabilityWording.MatchString(text) || sendWording.MatchString(text) }
	gated := w.defRuled[name] || w.payload[name] || (def.XMethod != "" && w.methodGated[def.XMethod])
	var gaps []string
	if asks(def.Description) && !gated {
		gaps = append(gaps, name+": its description asks for a gate, and no rule gates it")
	}
	props := expandAllOf(w.m.schema, def).Properties
	for _, key := range slices.Sorted(maps.Keys(props)) {
		prop := props[key]
		if prop == nil {
			continue
		}
		if asks(prop.Description) && !gated && !w.propRuled[[2]string{name, key}] {
			gaps = append(gaps, name+"."+key+": its description asks for a gate, and no rule gates it")
		}
		if absPathWording.MatchString(prop.Description) && !w.paths[[2]string{name, key}] {
			gaps = append(gaps, name+"."+key+": its description asks for an absolute path, and no path rule names it")
		}
	}
	members := slices.Concat(def.AnyOf, def.OneOf)
	for i, v := range members {
		if v != nil && asks(v.Description) && !gated && !w.variantGated(name, v, members) {
			gaps = append(gaps, fmt.Sprintf("%s variant %d: its description asks for a gate, and no rule gates it", name, i+1))
		}
	}
	return gaps
}

// unionOf names the union a capability rule's variant belongs to: its
// definition, or the union its property holds, through items, an allOf of
// one and a $ref.
func unionOf(m *Model, definition, property string) string {
	if property == "" {
		return definition
	}
	def := m.schema.Defs[definition]
	if def == nil {
		return ""
	}
	d := expandAllOf(m.schema, def).Properties[property]
	for d != nil {
		switch {
		case d.Ref != "":
			return strings.TrimPrefix(d.Ref, refPrefix)
		case d.Items != nil:
			d = d.Items
		case len(d.AllOf) == 1:
			d = d.AllOf[0]
		default:
			return ""
		}
	}
	return ""
}

// checkCommittedWording requires every place of the committed schemas whose
// wording asks for a gate or an absolute path to have its rule in ACP's table
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G29).
func checkCommittedWording(t *testing.T, m *Model) {
	t.Helper()
	for _, g := range wordingGaps(m, &ACP) {
		t.Error(g)
	}
}

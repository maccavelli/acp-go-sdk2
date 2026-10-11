package model

import (
	"slices"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

// protocolDefs is a schema every rule of testProtocol applies to.
const protocolDefs = `{
	"SessionId":{"type":"string"},
	"PromptRequest":{"type":"object","x-side":"agent","x-method":"s/prompt","required":["sessionId"],
		"properties":{"sessionId":{"allOf":[{"$ref":"#/$defs/SessionId"}]}}},
	"PromptResponse":{"type":"object","x-side":"agent","x-method":"s/prompt"},
	"CancelNotification":{"type":"object","x-side":"agent","x-method":"s/cancel","required":["sessionId"],
		"properties":{"sessionId":{"allOf":[{"$ref":"#/$defs/SessionId"}]}}},
	"LoadSessionRequest":{"type":"object","x-side":"agent","x-method":"s/load"},
	"LoadSessionResponse":{"type":"object","x-side":"agent","x-method":"s/load"},
	"SessionNotification":{"type":"object","x-side":"client","x-method":"s/update"},
	"AgentCapabilities":{"type":"object","properties":{"loadSession":{"type":"boolean"},"session":{"allOf":[{"$ref":"#/$defs/SessionCaps"}]}}},
	"SessionCaps":{"type":"object","properties":{"list":{"type":"object"}}},
	"Block":{"oneOf":[{"type":"object","required":["type","text"],"properties":{"type":{"const":"text","type":"string"},"text":{"type":"string"}}}]},
	"Update":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}`

func protocolMeta() *load.Meta {
	return &load.Meta{
		AgentMethods:  map[string]string{"prompt": "s/prompt", "cancel": "s/cancel", "load": "s/load"},
		ClientMethods: map[string]string{"update": "s/update"},
	}
}

// testProtocol is a table with one rule of each kind, for protocolDefs.
func testProtocol() *Protocol {
	return &Protocol{
		Interfaces: []Interface{{Binding: BindAgentOptional, Name: "Loader", Var: "loader", Doc: []string{"Loader loads."}}},
		Methods: []MethodRule{
			{Side: "agent", Wire: "s/load", Binding: BindAgentOptional, Capability: "loadSession"},
			{Side: "agent", Wire: "s/prompt", PromptTurn: true, SessionKey: "sessionId"},
			{Side: "agent", Wire: "s/cancel", CancelsPromptTurn: true, SessionKey: "sessionId", Capability: "session.list"},
			{Side: "client", Wire: "s/update", GoName: "SessionUpdate"},
		},
		HandWrittenHelpers: []string{"Block"},
	}
}

// methodByWire returns the method of m with the given wire name, or nil.
func methodByWire(m *Model, wire string) *Method {
	for _, mi := range slices.Concat(m.AgentMethods, m.ClientMethods) {
		if mi.Wire == wire {
			return mi
		}
	}
	return nil
}

func interfaceNames(m *Model) []string {
	var out []string
	for _, in := range m.Interfaces {
		out = append(out, in.Name)
	}
	return out
}

// TestProtocolTableApplies requires each kind of rule to change the model,
// and a nil table to change nothing.
func TestProtocolTableApplies(t *testing.T) {
	t.Parallel()
	m, err := Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, protocolDefs), Protocol: testProtocol()})
	if err != nil {
		t.Fatal(err)
	}
	loader, prompt, cancel, update := methodByWire(m, "s/load"), methodByWire(m, "s/prompt"), methodByWire(m, "s/cancel"), methodByWire(m, "s/update")
	if loader.Binding != BindAgentOptional || loader.Capability != "loadSession" {
		t.Errorf("s/load: binding %v capability %q", loader.Binding, loader.Capability)
	}
	if !prompt.PromptTurn || prompt.SessionField != "SessionId" || prompt.Canceller != cancel {
		t.Errorf("s/prompt: PromptTurn %v SessionField %q Canceller %v", prompt.PromptTurn, prompt.SessionField, prompt.Canceller)
	}
	if !cancel.CancelsPromptTurn || cancel.SessionField != "SessionId" || cancel.Capability != "session.list" {
		t.Errorf("s/cancel: CancelsPromptTurn %v SessionField %q capability %q", cancel.CancelsPromptTurn, cancel.SessionField, cancel.Capability)
	}
	if update.GoName != "SessionUpdate" {
		t.Errorf("s/update: GoName %q", update.GoName)
	}
	if got := interfaceNames(m); !slices.Equal(got, []string{"Agent", "Loader", "AgentExperimental", "Client", "ClientExperimental"}) {
		t.Errorf("interfaces %v", got)
	}
	if in := m.Interfaces[1]; in.Var != "loader" || !slices.Equal(in.Doc, []string{"Loader loads."}) {
		t.Errorf("interface Loader: %+v", in)
	}
	if h := m.Type("Block").Helpers; len(h) != 0 {
		t.Errorf("Block has %d generated helpers", len(h))
	}

	plain, err := Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, protocolDefs)})
	if err != nil {
		t.Fatal(err)
	}
	if b := methodByWire(plain, "s/load").Binding; b != BindAgent {
		t.Errorf("without a table, s/load binds %v", b)
	}
	if g := methodByWire(plain, "s/update").GoName; g != "Session" {
		t.Errorf("without a table, s/update is %q", g)
	}
	if p := methodByWire(plain, "s/prompt"); p.PromptTurn || p.Canceller != nil || p.SessionField != "" {
		t.Errorf("without a table, s/prompt is a prompt turn: %+v", p)
	}
	if got := interfaceNames(plain); !slices.Equal(got, []string{"Agent", "AgentExperimental", "Client", "ClientExperimental"}) {
		t.Errorf("without a table, interfaces %v", got)
	}
	if len(plain.Type("Block").Helpers) != 1 {
		t.Error("without a table, Block lost its helper")
	}
}

// TestProtocolTableSkipsAbsentSubjects requires the full table to build a
// schema that carries none of its subjects, applying nothing.
func TestProtocolTableSkipsAbsentSubjects(t *testing.T) {
	t.Parallel()
	m, err := Build(Input{Meta: &load.Meta{}, Protocol: &ACP, Schema: schemaOf(t, `{
		"U":{"oneOf":[{"type":"object","required":["type"],"properties":{"type":{"const":"a","type":"string"}}}]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if got := interfaceNames(m); !slices.Equal(got, []string{"Agent", "AgentExperimental", "Client", "ClientExperimental"}) {
		t.Errorf("interfaces %v", got)
	}
	if len(m.Type("U").Helpers) != 1 {
		t.Error("U lost its helper")
	}
}

func TestProtocolTableRefuses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		table  func(p *Protocol)
		defs   func(s string) string
		meta   func(m *load.Meta)
		wanted []string
	}{
		{name: "two rules for one method", table: func(p *Protocol) { p.Methods = append(p.Methods, MethodRule{Side: "client", Wire: "s/update"}) },
			wanted: []string{`client method "s/update"`, "two rules"}},
		{name: "unknown side", table: func(p *Protocol) { p.Methods[3].Side = "protocol" },
			wanted: []string{`protocol method "s/update"`, `side must be "agent" or "client"`}},
		{name: "binding no interface declares", table: func(p *Protocol) { p.Interfaces = nil },
			wanted: []string{`agent method "s/load"`, "no interface of the table has its binding"}},
		{name: "interface of a required binding", table: func(p *Protocol) { p.Interfaces[0].Binding = BindAgent },
			wanted: []string{"interface Loader", "binding must be optional"}},
		{name: "binding of the other side", table: func(p *Protocol) { p.Methods[3].Binding = BindAgentOptional },
			wanted: []string{`client method "s/update"`, "binds to an agent interface"}},
		{name: "prompt turn on the client side", table: func(p *Protocol) { p.Methods[3].PromptTurn, p.Methods[3].SessionKey = true, "sessionId" },
			wanted: []string{`client method "s/update"`, "only agent methods"}},
		{name: "prompt turn without a session key", table: func(p *Protocol) { p.Methods[1].SessionKey = "" },
			wanted: []string{`agent method "s/prompt"`, "needs a session key"}},
		{name: "session key on a plain rule", table: func(p *Protocol) { p.Methods[3].SessionKey = "sessionId" },
			wanted: []string{`client method "s/update"`, "session key without a prompt-turn role"}},
		{name: "unexported Go name", table: func(p *Protocol) { p.Methods[3].GoName = "update" },
			wanted: []string{`client method "s/update"`, `"update" is not an exported Go identifier`}},
		{name: "prompt turn on a notification", table: func(p *Protocol) { p.Methods[2].PromptTurn, p.Methods[2].CancelsPromptTurn = true, false },
			wanted: []string{`agent method "s/cancel"`, "a prompt turn must be a request with a result"}},
		{name: "prompt turn without a result", defs: func(s string) string {
			return strings.Replace(s, `"PromptResponse":{"type":"object"`, `"PromptResponse":{"type":"null"`, 1)
		}, wanted: []string{`agent method "s/prompt"`, "a prompt turn must be a request with a result"}},
		{name: "cancel on a request", table: func(p *Protocol) { p.Methods[0].CancelsPromptTurn, p.Methods[0].SessionKey = true, "sessionId" },
			wanted: []string{`agent method "s/load"`, "a prompt-turn cancel must be a notification"}},
		{name: "session key not required", defs: func(s string) string {
			return strings.Replace(s, `"x-method":"s/prompt","required":["sessionId"],`, `"x-method":"s/prompt",`, 1)
		}, wanted: []string{`agent method "s/prompt"`, `"sessionId" is not a required property of PromptRequest`}},
		{name: "session key not a string", defs: func(s string) string {
			return strings.Replace(s, `"SessionId":{"type":"string"}`, `"SessionId":{"type":"integer"}`, 1)
		}, wanted: []string{`agent method "s/cancel"`, `"sessionId" of CancelNotification is not a string`}},
		{name: "prompt turn without its cancel", meta: func(m *load.Meta) { delete(m.AgentMethods, "cancel") },
			wanted: []string{`agent method "s/prompt"`, "needs one prompt-turn cancel on its side, found 0"}},
		{name: "two cancels", table: func(p *Protocol) {
			p.Methods = append(p.Methods, MethodRule{Side: "agent", Wire: "s/stop", CancelsPromptTurn: true, SessionKey: "sessionId"})
		}, defs: func(s string) string {
			return strings.Replace(s, `"Update":`, `"StopNotification":{"type":"object","x-side":"agent","x-method":"s/stop","required":["sessionId"],
				"properties":{"sessionId":{"allOf":[{"$ref":"#/$defs/SessionId"}]}}},"Update":`, 1)
		}, meta: func(m *load.Meta) { m.AgentMethods["stop"] = "s/stop" },
			wanted: []string{`agent method "s/prompt"`, "needs one prompt-turn cancel on its side, found 2"}},
		{name: "cancel with another session type", defs: func(s string) string {
			return strings.Replace(s, `"x-method":"s/cancel","required":["sessionId"],
		"properties":{"sessionId":{"allOf":[{"$ref":"#/$defs/SessionId"}]}}`, `"x-method":"s/cancel","required":["sessionId"],
		"properties":{"sessionId":{"type":"string"}}`, 1)
		}, wanted: []string{`agent method "s/prompt"`, "its cancel s/cancel keys the session by SessionId string, not SessionId SessionId"}},
		{name: "capability that does not resolve", table: func(p *Protocol) { p.Methods[2].Capability = "session.missing" },
			wanted: []string{`agent method "s/cancel"`, `capability "session.missing"`, `SessionCaps has no property "missing"`}},
		{name: "hand-written helpers on a non-union", table: func(p *Protocol) { p.HandWrittenHelpers = []string{"Update"} },
			wanted: []string{"hand-written helpers for Update", "not a union"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, defs, meta := testProtocol(), protocolDefs, protocolMeta()
			if tt.table != nil {
				tt.table(p)
			}
			if tt.defs != nil {
				defs = tt.defs(defs)
				if defs == protocolDefs {
					t.Fatal("the defs edit did not apply")
				}
			}
			if tt.meta != nil {
				tt.meta(meta)
			}
			_, err := Build(Input{Meta: meta, Schema: schemaOf(t, defs), Protocol: p})
			if err == nil {
				t.Fatal("Build accepted the table")
			}
			for _, w := range append([]string{"protocol table: "}, tt.wanted...) {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}

// capabilityDoc returns the doc comment of the capability a path names,
// from the side's capabilities type.
func capabilityDoc(t *testing.T, m *Model, side, path string) string {
	t.Helper()
	ty := m.Type(map[string]string{"agent": "AgentCapabilities", "client": "ClientCapabilities"}[side])
	segs := strings.Split(path, ".")
	for i, seg := range segs {
		var f *Field
		for _, c := range ty.Fields {
			if c.Key == seg {
				f = c
			}
		}
		if f == nil {
			t.Fatalf("%s: no %q in %s", path, seg, ty.Name)
		}
		if i == len(segs)-1 {
			return strings.Join(f.Doc, " ")
		}
		r := f.Type
		for r.Kind == RefPointer {
			r = *r.Elem
		}
		ty = m.Type(r.Name)
	}
	return ""
}

// TestProtocolTableAppliesToCommittedSchema requires every rule of ACP to
// apply to the schema in the repository.
func TestProtocolTableAppliesToCommittedSchema(t *testing.T) {
	t.Parallel()
	m := committedModel(t)
	gates := 0
	for _, r := range ACP.Methods {
		mi := methodByWire(m, r.Wire)
		if mi == nil || mi.Side != r.Side {
			t.Errorf("%s method %q: not in the committed meta files", r.Side, r.Wire)
			continue
		}
		if r.Binding != BindUnknown && mi.Binding != r.Binding {
			t.Errorf("%s: binding %v, want %v", r.Wire, mi.Binding, r.Binding)
		}
		if r.GoName != "" && mi.GoName != r.GoName {
			t.Errorf("%s: GoName %q, want %q", r.Wire, mi.GoName, r.GoName)
		}
		if mi.PromptTurn != r.PromptTurn || mi.CancelsPromptTurn != r.CancelsPromptTurn {
			t.Errorf("%s: PromptTurn %v CancelsPromptTurn %v", r.Wire, mi.PromptTurn, mi.CancelsPromptTurn)
		}
		if r.Capability != "" {
			gates++
			doc := capabilityDoc(t, m, r.Side, r.Capability)
			// Doc comments render the schema's backticks as single quotes.
			named := "'" + r.Wire + "'"
			switch {
			case strings.HasPrefix(r.Wire, "terminal/"):
				named = "'terminal/*'"
			case r.Wire == "logout":
				named = "the logout method"
			}
			if !strings.Contains(doc, named) {
				t.Errorf("%s: capability %s does not name %s: %q", r.Wire, r.Capability, named, doc)
			}
		}
	}
	if gates != 14 {
		t.Errorf("%d capability gates, want 14", gates)
	}
	prompt := methodByWire(m, "session/prompt")
	if prompt.SessionField != "SessionId" || prompt.Canceller == nil || prompt.Canceller.Wire != "session/cancel" {
		t.Errorf("session/prompt: SessionField %q Canceller %+v", prompt.SessionField, prompt.Canceller)
	}
	if got := interfaceNames(m); !slices.Equal(got, []string{"Agent", "AgentLoader", "AgentExperimental", "Client", "ClientExperimental"}) {
		t.Errorf("interfaces %v", got)
	}
	for _, name := range ACP.HandWrittenHelpers {
		ty := m.Type(name)
		if ty == nil || ty.Kind != KindUnion || len(ty.Helpers) != 0 {
			t.Errorf("hand-written helpers for %s: %+v", name, ty)
		}
	}
}

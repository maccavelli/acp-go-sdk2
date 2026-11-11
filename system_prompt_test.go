package acp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// The client system prompt of ACP schema 1.30.0 and later
// (0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md, D5 and
// D6): a client sends systemPrompt on session/new, session/load and
// session/resume only when the agent advertises
// sessionCapabilities.systemPrompt, and override only when it advertises
// sessionCapabilities.systemPrompt.mode.override; no mode the schema does not
// list, no block but text, and no empty content. Each rule is enforced on what
// is sent and on what is received.

// systemPromptJSON is a systemPrompt property of mode and content blocks, the
// blocks as JSON, for a session request's text.
func systemPromptJSON(mode, blocks string) string {
	return `"systemPrompt":{"mode":` + mode + `,"content":[` + blocks + `]}`
}

// textBlockJSON is one text block.
const textBlockJSON = `{"type":"text","text":"Be brief."}`

// newSessionWith is a session/new request that carries prompt, a systemPrompt
// property, or nothing when prompt is empty.
func newSessionWith(t *testing.T, prompt string) NewSessionRequest {
	t.Helper()
	if prompt != "" {
		prompt = "," + prompt
	}
	return decoded[NewSessionRequest](t, `{"cwd":`+jsonOf(t, testAbsPath())+`,"mcpServers":[]`+prompt+`}`)
}

// newSession is session/new from the client, with params.
func newSession(params any) gatedMethod {
	return gatedMethod{wire: AgentMethodSessionNew, toAgent: true, params: params, send: fromClient((*ClientSideConnection).NewSession)}
}

// appendOnly is a record that advertises everything, the system prompt in
// append mode only.
func appendOnly() *Negotiation {
	n := allAdvertised()
	n.Response.AgentCapabilities.SessionCapabilities.SystemPrompt = &SessionSystemPromptCapabilities{}
	return n
}

// refusedBothWays requires m, sent with record n, to return a
// *CapabilityError naming capability, field and variant and to write nothing,
// and, received with n, to be answered -32602 without reaching the handler.
func refusedBothWays(t *testing.T, m gatedMethod, n *Negotiation, capability, field, variant string) {
	t.Helper()
	w := &countingWriter{}
	asc, csc := sides(t, w, n)
	// A send the gate let through waits for an answer that never comes, so it
	// ends at the deadline instead of hanging the test.
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	err := m.send(ctx, asc, csc, m.params)
	cancel()
	ce, ok := errors.AsType[*CapabilityError](err)
	if !ok || ce.Method != m.wire || ce.Capability != capability || ce.Field != field || ce.Variant != variant {
		t.Errorf("send = %v, want *CapabilityError{%s, %s, Field %q, Variant %q}", err, m.wire, capability, field, variant)
	}
	if got := w.count(); got != 0 {
		t.Errorf("%d bytes written, want none", got)
	}
	if rerr, ran := receive(t, m, n); rerr == nil || rerr.Code != -32602 || ran {
		t.Errorf("received: %v, handler ran %v, want -32602 and no handler", rerr, ran)
	}
}

// TestSystemPromptGated requires a systemPrompt on session/new, session/load
// and session/resume to need the agent's sessionCapabilities.systemPrompt, as
// checkValueGate checks a rule: without it, the typed method returns a
// *CapabilityError naming the field and writes nothing, and a received one is
// answered -32602 without reaching the handler; with it, it is written and
// handled. The handler receives the prompt as it was sent.
func TestSystemPromptGated(t *testing.T) {
	t.Parallel()
	dir := jsonOf(t, testAbsPath())
	prompt := systemPromptJSON(`"append"`, textBlockJSON)
	advertise := func(n *Negotiation, on bool) {
		n.Response.AgentCapabilities.SessionCapabilities.SystemPrompt = when(on, &SessionSystemPromptCapabilities{})
	}
	gate := func(m gatedMethod) valueGate {
		m.capability, m.toAgent, m.advertise = "sessionCapabilities.systemPrompt", true, advertise
		return valueGate{gatedMethod: m, field: "systemPrompt"}
	}
	for _, g := range []valueGate{
		gate(gatedMethod{wire: AgentMethodSessionNew, params: newSessionWith(t, prompt), send: fromClient((*ClientSideConnection).NewSession)}),
		gate(gatedMethod{wire: AgentMethodSessionLoad,
			params: decoded[LoadSessionRequest](t, `{"sessionId":"s","cwd":`+dir+`,"mcpServers":[],`+prompt+`}`),
			send:   fromClient((*ClientSideConnection).LoadSession)}),
		gate(gatedMethod{wire: AgentMethodSessionResume, params: decoded[ResumeSessionRequest](t, `{"sessionId":"s","cwd":`+dir+`,`+prompt+`}`),
			send: fromClient((*ClientSideConnection).ResumeSession)}),
	} {
		t.Run(g.wire, func(t *testing.T) {
			t.Parallel()
			checkValueGate(t, g)
		})
	}

	calls := make(chan any, 1)
	asc := NewAgentSideConnection(recordingAgent{calls: calls, reply: validReply}, &countingWriter{}, idleReader(t))
	negotiate(t, asc, allAdvertised())
	if _, rerr := asc.handle(t.Context(), AgentMethodSessionNew, []byte(jsonOf(t, newSessionWith(t, prompt)))); rerr != nil {
		t.Fatalf("session/new with an advertised prompt: %v", rerr)
	}
	got, ok := within(t, calls, "the handler's call").(NewSessionRequest)
	if !ok || got.SystemPrompt == nil || got.SystemPrompt.Mode != SystemPromptModeAppend || len(got.SystemPrompt.Content) != 1 ||
		got.SystemPrompt.Content[0].Text == nil || got.SystemPrompt.Content[0].Text.Text != "Be brief." {
		t.Errorf("the handler received %+v, want the append prompt with its text block", got.SystemPrompt)
	}
	// A request without a prompt needs no capability.
	if rerr, ran := receive(t, newSession(newSessionWith(t, "")), versionOnly()); rerr != nil || !ran {
		t.Errorf("session/new without a prompt, nothing advertised: %v, handler ran %v, want it handled", rerr, ran)
	}
}

// TestSystemPromptOverrideGated requires the override mode to need the
// agent's sessionCapabilities.systemPrompt.mode.override as well: with the
// prompt advertised in append mode only, an override prompt is refused both
// ways, naming the mode, and an append prompt passes.
func TestSystemPromptOverrideGated(t *testing.T) {
	t.Parallel()
	override := newSessionWith(t, systemPromptJSON(`"override"`, textBlockJSON))
	checkValueGate(t, valueGate{
		gatedMethod: gatedMethod{wire: AgentMethodSessionNew, capability: "sessionCapabilities.systemPrompt.mode.override", toAgent: true,
			params: override, send: fromClient((*ClientSideConnection).NewSession),
			advertise: func(n *Negotiation, on bool) {
				n.Response.AgentCapabilities.SessionCapabilities.SystemPrompt = &SessionSystemPromptCapabilities{
					Mode: when(on, &SessionSystemPromptModeCapabilities{Override: &SessionSystemPromptOverrideCapabilities{}}),
				}
			}},
		field: "mode", variant: "override",
	})
	// A mode object that advertises no override is append only too.
	n := appendOnly()
	n.Response.AgentCapabilities.SessionCapabilities.SystemPrompt.Mode = &SessionSystemPromptModeCapabilities{}
	refusedBothWays(t, newSession(override), n, "sessionCapabilities.systemPrompt.mode.override", "mode", "override")

	appended := newSession(newSessionWith(t, systemPromptJSON(`"append"`, textBlockJSON)))
	if rerr, ran := receive(t, appended, appendOnly()); rerr != nil || !ran {
		t.Errorf("an append prompt, append advertised: %v, handler ran %v, want it handled", rerr, ran)
	}
}

// TestSystemPromptUndeclaredModes requires a mode the schema does not list,
// one beginning with an underscore and the empty string included, to be
// refused both ways with every capability advertised, since no capability
// advertises one; and a null mode, which decodes as the empty string, to be
// answered -32602.
func TestSystemPromptUndeclaredModes(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"_vendor", "later", ""} {
		t.Run("mode "+mode, func(t *testing.T) {
			t.Parallel()
			req := newSessionWith(t, systemPromptJSON(jsonOf(t, mode), textBlockJSON))
			if got := req.SystemPrompt.Mode; string(got) != mode {
				t.Fatalf("decoded mode %q, want %q kept", got, mode)
			}
			refusedBothWays(t, newSession(req), allAdvertised(), "sessionCapabilities.systemPrompt.mode."+mode, "mode", mode)
		})
	}
	null := `{"cwd":` + jsonOf(t, testAbsPath()) + `,"mcpServers":[],` + systemPromptJSON(`null`, textBlockJSON) + `}`
	if rerr, ran := receive(t, newSession(json.RawMessage(null)), allAdvertised()); rerr == nil || rerr.Code != -32602 || ran {
		t.Errorf("a null mode received: %v, handler ran %v, want -32602 and no handler", rerr, ran)
	}
}

// TestSystemPromptTextOnly requires a content block other than text, each
// declared type and one the schema does not list, to be refused both ways
// with every capability advertised, naming the block and its type; and text
// blocks to pass.
func TestSystemPromptTextOnly(t *testing.T) {
	t.Parallel()
	for typ, block := range map[string]string{
		"image":         `{"type":"image","data":"x","mimeType":"image/png"}`,
		"audio":         `{"type":"audio","data":"x","mimeType":"audio/wav"}`,
		"resource_link": `{"type":"resource_link","uri":"file:///x","name":"x"}`,
		"resource":      `{"type":"resource","resource":{"uri":"file:///x","text":"t"}}`,
		"video":         `{"type":"video","uri":"file:///v"}`,
	} {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			req := newSessionWith(t, systemPromptJSON(`"append"`, textBlockJSON+","+block))
			refusedBothWays(t, newSession(req), allAdvertised(), "sessionCapabilities.systemPrompt.content."+typ, "content[1]", typ)
		})
	}
	two := newSession(newSessionWith(t, systemPromptJSON(`"append"`, textBlockJSON+","+textBlockJSON)))
	if rerr, ran := receive(t, two, allAdvertised()); rerr != nil || !ran {
		t.Errorf("two text blocks: %v, handler ran %v, want them handled", rerr, ran)
	}
}

// TestSystemPromptEmptyContent requires empty content to fail Validate with
// RuleMinItems, not to be sent, and to be answered -32602 when received,
// with every capability advertised.
func TestSystemPromptEmptyContent(t *testing.T) {
	t.Parallel()
	for name, content := range map[string][]ContentBlock{"empty": {}, "nil": nil} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			prompt := &SystemPrompt{Mode: SystemPromptModeAppend, Content: content}
			ve, ok := errors.AsType[*ValidationError](prompt.Validate())
			if !ok || ve.Rule != RuleMinItems || ve.Type != "SystemPrompt" || ve.Field != "content" {
				t.Errorf("Validate = %v, want *ValidationError{SystemPrompt, content, RuleMinItems}", prompt.Validate())
			}
			w := &countingWriter{}
			_, csc := sides(t, w, allAdvertised())
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			_, err := csc.NewSession(ctx, NewSessionRequest{Cwd: testAbsPath(), McpServers: []McpServer{}, SystemPrompt: prompt})
			cancel()
			if ve, ok := errors.AsType[*ValidationError](err); !ok || ve.Rule != RuleMinItems {
				t.Errorf("NewSession = %v, want a *ValidationError with RuleMinItems", err)
			}
			if got := w.count(); got != 0 {
				t.Errorf("%d bytes written, want none", got)
			}
		})
	}
	empty := `{"cwd":` + jsonOf(t, testAbsPath()) + `,"mcpServers":[],"systemPrompt":{"mode":"append","content":[]}}`
	if rerr, ran := receive(t, newSession(json.RawMessage(empty)), allAdvertised()); rerr == nil || rerr.Code != -32602 || ran {
		t.Errorf("empty content received: %v, handler ran %v, want -32602 and no handler", rerr, ran)
	}
}

// TestSystemPromptInboundValidationOptIn requires an agent's validation hook
// to be able to accept what the SDK refuses of a received prompt: a mode the
// schema does not list, which it is given as the gate's *CapabilityError, and
// empty content, which it is given as the *ValidationError. Each then reaches
// the handler.
func TestSystemPromptInboundValidationOptIn(t *testing.T) {
	t.Parallel()
	dir := jsonOf(t, testAbsPath())
	for name, tt := range map[string]struct {
		raw  string
		want func(error) bool
	}{
		"a vendor mode": {`{"cwd":` + dir + `,"mcpServers":[],` + systemPromptJSON(`"_vendor"`, textBlockJSON) + `}`, func(err error) bool {
			ce, ok := errors.AsType[*CapabilityError](err)
			return ok && ce.Capability == "sessionCapabilities.systemPrompt.mode._vendor" && ce.Variant == "_vendor"
		}},
		"empty content": {`{"cwd":` + dir + `,"mcpServers":[],"systemPrompt":{"mode":"append","content":[]}}`, func(err error) bool {
			ve, ok := errors.AsType[*ValidationError](err)
			return ok && ve.Rule == RuleMinItems
		}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var seen []error
			accept := WithInboundValidation(func(_ context.Context, _ string, _ any, err error) error {
				seen = append(seen, err)
				return nil
			})
			if rerr, ran := receive(t, newSession(json.RawMessage(tt.raw)), allAdvertised(), accept); rerr != nil || !ran {
				t.Errorf("received with a hook that accepts it: %v, handler ran %v, want it handled", rerr, ran)
			}
			found := false
			for _, err := range seen {
				found = found || tt.want(err)
			}
			if !found {
				t.Errorf("the hook saw %v, want the SDK's refusal among them", seen)
			}
		})
	}
}

// TestSystemPromptAllowsValue requires Negotiation.AllowsValue to report,
// without sending, what the gate would refuse of a prompt, and nil for what it
// would send.
func TestSystemPromptAllowsValue(t *testing.T) {
	t.Parallel()
	appended := newSessionWith(t, systemPromptJSON(`"append"`, textBlockJSON))
	override := newSessionWith(t, systemPromptJSON(`"override"`, textBlockJSON))
	vendor := newSessionWith(t, systemPromptJSON(`"_vendor"`, textBlockJSON))
	image := newSessionWith(t, systemPromptJSON(`"append"`, `{"type":"image","data":"x","mimeType":"image/png"}`))
	for _, tt := range []struct {
		name       string
		n          *Negotiation
		params     NewSessionRequest
		capability string
	}{
		{"a prompt, nothing advertised", versionOnly(), appended, "sessionCapabilities.systemPrompt"},
		{"override, append advertised", appendOnly(), override, "sessionCapabilities.systemPrompt.mode.override"},
		{"a vendor mode, everything advertised", allAdvertised(), vendor, "sessionCapabilities.systemPrompt.mode._vendor"},
		{"an image, everything advertised", allAdvertised(), image, "sessionCapabilities.systemPrompt.content.image"},
		{"append, append advertised", appendOnly(), appended, ""},
		{"override, everything advertised", allAdvertised(), override, ""},
		{"no prompt, nothing advertised", versionOnly(), newSessionWith(t, ""), ""},
	} {
		err := tt.n.AllowsValue(AgentMethodSessionNew, tt.params)
		ce, ok := errors.AsType[*CapabilityError](err)
		switch {
		case tt.capability == "" && err != nil:
			t.Errorf("%s: AllowsValue = %v, want nil", tt.name, err)
		case tt.capability != "" && (!ok || ce.Capability != tt.capability || ce.Method != AgentMethodSessionNew):
			t.Errorf("%s: AllowsValue = %v, want a *CapabilityError naming %s", tt.name, err, tt.capability)
		}
	}
}

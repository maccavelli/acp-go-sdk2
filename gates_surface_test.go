package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// notifyFromClient is the client's typed notification m as a sender.
func notifyFromClient[P any](m func(*ClientSideConnection, context.Context, P) error) sender {
	return func(ctx context.Context, _ *AgentSideConnection, csc *ClientSideConnection, params any) error {
		return m(csc, ctx, params.(P))
	}
}

// unstableGatedMethods returns the 13 method gates of the unstable
// capabilities: providers/*, nes/* and the five document events
// (0049-PLAN-surface-fixes.md, phase 4, S3).
func unstableGatedMethods() []gatedMethod {
	agentCaps := func(n *Negotiation) *AgentCapabilities { return &n.Response.AgentCapabilities }
	providers := func(n *Negotiation, on bool) { agentCaps(n).Providers = when(on, &ProvidersCapabilities{}) }
	nes := func(n *Negotiation, on bool) { agentCaps(n).Nes = when(on, &NesCapabilities{}) }
	// document is the agent's document events, made on the way.
	document := func(n *Negotiation) *NesDocumentEventCapabilities {
		caps := agentCaps(n)
		if caps.Nes == nil {
			caps.Nes = &NesCapabilities{}
		}
		if caps.Nes.Events == nil {
			caps.Nes.Events = &NesEventCapabilities{}
		}
		if caps.Nes.Events.Document == nil {
			caps.Nes.Events.Document = &NesDocumentEventCapabilities{}
		}
		return caps.Nes.Events.Document
	}
	const s, uri = SessionId("s"), "file:///x"
	return []gatedMethod{
		{wire: AgentMethodProvidersList, capability: "providers", toAgent: true,
			params: UnstableListProvidersRequest{}, send: fromClient((*ClientSideConnection).UnstableListProviders), advertise: providers},
		{wire: AgentMethodProvidersSet, capability: "providers", toAgent: true,
			params: UnstableSetProviderRequest{ProviderId: "p", ApiType: UnstableLlmProtocolAnthropic, BaseUrl: "https://example.invalid"},
			send:   fromClient((*ClientSideConnection).UnstableSetProvider), advertise: providers},
		{wire: AgentMethodProvidersDisable, capability: "providers", toAgent: true,
			params: UnstableDisableProviderRequest{ProviderId: "p"}, send: fromClient((*ClientSideConnection).UnstableDisableProvider),
			advertise: providers},
		{wire: AgentMethodNesStart, capability: "nes", toAgent: true,
			params: UnstableStartNesRequest{}, send: fromClient((*ClientSideConnection).UnstableStartNes), advertise: nes},
		{wire: AgentMethodNesSuggest, capability: "nes", toAgent: true,
			params: UnstableSuggestNesRequest{SessionId: s, Uri: uri, TriggerKind: UnstableNesTriggerKindManual},
			send:   fromClient((*ClientSideConnection).UnstableSuggestNes), advertise: nes},
		{wire: AgentMethodNesAccept, capability: "nes", toAgent: true, notifies: true,
			params: UnstableAcceptNesNotification{SessionId: s, Id: "n"}, send: notifyFromClient((*ClientSideConnection).UnstableAcceptNes),
			advertise: nes},
		{wire: AgentMethodNesReject, capability: "nes", toAgent: true, notifies: true,
			params: UnstableRejectNesNotification{SessionId: s, Id: "n"}, send: notifyFromClient((*ClientSideConnection).UnstableRejectNes),
			advertise: nes},
		{wire: AgentMethodNesClose, capability: "nes", toAgent: true,
			params: UnstableCloseNesRequest{SessionId: s}, send: fromClient((*ClientSideConnection).UnstableCloseNes), advertise: nes},
		{wire: AgentMethodDocumentDidOpen, capability: "nes.events.document.didOpen", toAgent: true, notifies: true,
			params: UnstableDidOpenDocumentNotification{SessionId: s, Uri: uri, LanguageId: "go", Text: "x"},
			send:   notifyFromClient((*ClientSideConnection).UnstableDidOpenDocument),
			advertise: func(n *Negotiation, on bool) {
				document(n).DidOpen = when(on, &NesDocumentDidOpenCapabilities{})
			}},
		{wire: AgentMethodDocumentDidChange, capability: "nes.events.document.didChange", toAgent: true, notifies: true,
			params: UnstableDidChangeDocumentNotification{SessionId: s, Uri: uri, ContentChanges: []UnstableTextDocumentContentChangeEvent{{Text: "x"}}},
			send:   notifyFromClient((*ClientSideConnection).UnstableDidChangeDocument),
			advertise: func(n *Negotiation, on bool) {
				document(n).DidChange = when(on, &NesDocumentDidChangeCapabilities{SyncKind: TextDocumentSyncKindFull})
			}},
		{wire: AgentMethodDocumentDidClose, capability: "nes.events.document.didClose", toAgent: true, notifies: true,
			params: UnstableDidCloseDocumentNotification{SessionId: s, Uri: uri},
			send:   notifyFromClient((*ClientSideConnection).UnstableDidCloseDocument),
			advertise: func(n *Negotiation, on bool) {
				document(n).DidClose = when(on, &NesDocumentDidCloseCapabilities{})
			}},
		{wire: AgentMethodDocumentDidSave, capability: "nes.events.document.didSave", toAgent: true, notifies: true,
			params: UnstableDidSaveDocumentNotification{SessionId: s, Uri: uri},
			send:   notifyFromClient((*ClientSideConnection).UnstableDidSaveDocument),
			advertise: func(n *Negotiation, on bool) {
				document(n).DidSave = when(on, &NesDocumentDidSaveCapabilities{})
			}},
		{wire: AgentMethodDocumentDidFocus, capability: "nes.events.document.didFocus", toAgent: true, notifies: true,
			params: UnstableDidFocusDocumentNotification{SessionId: s, Uri: uri},
			send:   notifyFromClient((*ClientSideConnection).UnstableDidFocusDocument),
			advertise: func(n *Negotiation, on bool) {
				document(n).DidFocus = when(on, &NesDocumentDidFocusCapabilities{})
			}},
	}
}

// TestBooleanSetConfigOptionGated requires a session/set_config_option of
// type boolean to need the client's session.configOptions.boolean, as the
// boolean options it answers do: sent without it, the typed method returns a
// *CapabilityError and writes nothing; with it, it is sent; received without
// it, it is answered -32602 (0049-PLAN-surface-fixes.md, phase 4, S5).
func TestBooleanSetConfigOptionGated(t *testing.T) {
	t.Parallel()
	checkValueGate(t, valueGate{gatedMethod: gatedMethod{wire: AgentMethodSessionSetConfigOption,
		capability: "session.configOptions.boolean", toAgent: true,
		params: decoded[SetSessionConfigOptionRequest](t, `{"sessionId":"s","configId":"c","type":"boolean","value":true}`),
		send:   fromClient((*ClientSideConnection).SetSessionConfigOption),
		advertise: func(n *Negotiation, on bool) {
			n.Request.ClientCapabilities.Session.ConfigOptions.Boolean = when(on, &BooleanConfigOptionCapabilities{})
		}}, variant: "boolean"})

	// A value-id request needs no capability.
	n := allAdvertised()
	n.Request.ClientCapabilities.Session.ConfigOptions.Boolean = nil
	m := gatedMethod{wire: AgentMethodSessionSetConfigOption, toAgent: true,
		params: SetSessionConfigOptionRequest{ValueId: &SetSessionConfigOptionValueId{SessionId: "s", ConfigId: "c", Value: "v"}}}
	if rerr, ran := receive(t, m, n); rerr != nil || !ran {
		t.Errorf("received a value-id request: %v, handler ran %v, want it handled", rerr, ran)
	}
}

// TestAuthenticateUnadvertisedMethod requires authenticate's methodId to name
// a method of the agent's initialize answer: a variant's id, or the "id" of
// an entry of a type this SDK does not know. Any other id is refused with
// ErrUnadvertisedAuthMethod before it is sent, writing nothing, and answered
// -32602 when received; a terminal method keeps ErrTerminalAuthMethod
// (0049-PLAN-surface-fixes.md, phase 4, S8).
func TestAuthenticateUnadvertisedMethod(t *testing.T) {
	t.Parallel()
	n := allAdvertised()
	n.Response.AuthMethods = []AuthMethod{
		NewAuthMethodAgent("a", "A"),
		NewAuthMethodTerminal("t", "T"),
		decoded[AuthMethod](t, `{"type":"future","ID":"f","name":"F"}`),
	}
	if n.Response.AuthMethods[2].Unknown == nil {
		t.Fatalf("the future method decoded as %+v, want Unknown", n.Response.AuthMethods[2])
	}
	authenticate := fromClient((*ClientSideConnection).Authenticate)
	m := gatedMethod{wire: AgentMethodAuthenticate, toAgent: true}

	w := &countingWriter{}
	_, csc := sides(t, w, n)
	// A send the gate lets through waits for an answer that never comes.
	refused, cancelRefused := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancelRefused()
	err := authenticate(refused, nil, csc, AuthenticateRequest{MethodId: "x"})
	if !errors.Is(err, ErrUnadvertisedAuthMethod) || err.Error() != `authenticate: methodId "x": `+ErrUnadvertisedAuthMethod.Error()+
		`; authenticate accepts "a", "f"` {
		t.Errorf("Authenticate(x) = %v, want ErrUnadvertisedAuthMethod", err)
	}
	if err := authenticate(refused, nil, csc, AuthenticateRequest{MethodId: "t"}); !errors.Is(err, ErrTerminalAuthMethod) {
		t.Errorf("Authenticate(t) = %v, want ErrTerminalAuthMethod", err)
	}
	if got := w.count(); got != 0 {
		t.Errorf("%d bytes written, want none", got)
	}
	if err := n.AllowsValue(AgentMethodAuthenticate, AuthenticateRequest{MethodId: "x"}); !errors.Is(err, ErrUnadvertisedAuthMethod) {
		t.Errorf("AllowsValue(x) = %v, want ErrUnadvertisedAuthMethod", err)
	}
	m.params = AuthenticateRequest{MethodId: "x"}
	if rerr, ran := receive(t, m, n); rerr == nil || rerr.Code != -32602 || ran ||
		!strings.Contains(fmt.Sprint(rerr.Data), `; authenticate accepts "a", "f"`) {
		t.Errorf("received x: %v, handler ran %v, want -32602 naming the accepted ids and no handler", rerr, ran)
	}
	for _, id := range []AuthMethodId{"a", "f"} {
		m.params = AuthenticateRequest{MethodId: id}
		if rerr, ran := receive(t, m, n); rerr != nil || !ran {
			t.Errorf("received %s: %v, handler ran %v, want it handled", id, rerr, ran)
		}
		if err := n.AllowsValue(AgentMethodAuthenticate, m.params); err != nil {
			t.Errorf("AllowsValue(%s) = %v, want nil", id, err)
		}
	}

	// An advertised method is sent.
	ctx, cancel := context.WithCancel(t.Context())
	sent := make(chan error, 1)
	go func() { sent <- authenticate(ctx, nil, csc, AuthenticateRequest{MethodId: "f"}) }()
	for deadline := time.Now().Add(5 * time.Second); w.count() == 0 && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := within(t, sent, "the send"); w.count() == 0 || errors.Is(err, ErrUnadvertisedAuthMethod) {
		t.Errorf("Authenticate(f) was not sent: %v", err)
	}

	// An answer that advertises no method refuses every id.
	none := allAdvertised()
	none.Response.AuthMethods = nil
	if err := none.AllowsValue(AgentMethodAuthenticate, AuthenticateRequest{MethodId: "a"}); !errors.Is(err, ErrUnadvertisedAuthMethod) {
		t.Errorf("AllowsValue(a) with no methods = %v, want ErrUnadvertisedAuthMethod", err)
	}
}

// TestAuthenticateUnknownEntryID requires an entry of a type this SDK does
// not know to advertise its "id" only when that is a JSON string, the empty
// string included and the key folded as its decoder folds it. Absent, null or
// not a string, it advertises nothing, so methodId "unknown", which it used
// to advertise, is refused on send, writing nothing, refused by AllowsValue,
// and answered -32602 when received
// (0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md, F5 and D1).
func TestAuthenticateUnknownEntryID(t *testing.T) {
	t.Parallel()
	answer := func(raw string) *Negotiation {
		n := allAdvertised()
		n.Response.AuthMethods = []AuthMethod{NewAuthMethodAgent("a", "A"), decoded[AuthMethod](t, raw)}
		if n.Response.AuthMethods[1].Unknown == nil {
			t.Fatalf("%s decoded as %+v, want Unknown", raw, n.Response.AuthMethods[1])
		}
		return n
	}
	m := gatedMethod{wire: AgentMethodAuthenticate, toAgent: true, params: AuthenticateRequest{MethodId: "unknown"}}
	for _, raw := range []string{`{"type":"future","name":"F"}`, `{"type":"future","id":"","name":"F"}`, `{"type":"future","id":null,"name":"F"}`,
		`{"type":"future","id":7,"name":"F"}`, `{"type":"future","id":{"x":1},"name":"F"}`} {
		n := answer(raw)
		if err := n.AllowsValue(AgentMethodAuthenticate, m.params); !errors.Is(err, ErrUnadvertisedAuthMethod) {
			t.Errorf("%s: AllowsValue(unknown) = %v, want ErrUnadvertisedAuthMethod", raw, err)
		}
		w := &countingWriter{}
		_, csc := sides(t, w, n)
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		_, err := csc.Authenticate(ctx, AuthenticateRequest{MethodId: "unknown"})
		cancel()
		if !errors.Is(err, ErrUnadvertisedAuthMethod) || w.count() != 0 {
			t.Errorf("%s: Authenticate(unknown) = %v, %d bytes written; want ErrUnadvertisedAuthMethod and none", raw, err, w.count())
		}
		if rerr, ran := receive(t, m, n); rerr == nil || rerr.Code != -32602 || ran {
			t.Errorf("%s: received unknown: %v, handler ran %v, want -32602 and no handler", raw, rerr, ran)
		}
	}
	for _, tc := range []struct {
		raw string
		id  AuthMethodId
	}{
		{`{"type":"future","id":"","name":"F"}`, ""},
		{`{"type":"future","ID":"f","name":"F"}`, "f"},
		{`{"type":"future","id":"unknown","name":"F"}`, "unknown"},
	} {
		n := answer(tc.raw)
		if err := n.AllowsValue(AgentMethodAuthenticate, AuthenticateRequest{MethodId: tc.id}); err != nil {
			t.Errorf("%s: AllowsValue(%q) = %v, want nil", tc.raw, tc.id, err)
		}
		mm := m
		mm.params = AuthenticateRequest{MethodId: tc.id}
		if rerr, ran := receive(t, mm, n); rerr != nil || !ran {
			t.Errorf("%s: received %q: %v, handler ran %v, want it handled", tc.raw, tc.id, rerr, ran)
		}
	}
}

// TestAuthenticateAcceptsNone requires the refusal of an unadvertised id to
// say that authenticate accepts none when every method the agent listed is
// terminal, and an id that is both terminal and agent to be refused as
// terminal (0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md,
// D2 and D4).
func TestAuthenticateAcceptsNone(t *testing.T) {
	t.Parallel()
	n := allAdvertised()
	n.Response.AuthMethods = []AuthMethod{NewAuthMethodTerminal("t", "T")}
	err := n.AllowsValue(AgentMethodAuthenticate, AuthenticateRequest{MethodId: "a"})
	if want := `authenticate: methodId "a": ` + ErrUnadvertisedAuthMethod.Error() + `; authenticate accepts none`; err == nil ||
		err.Error() != want || !errors.Is(err, ErrUnadvertisedAuthMethod) {
		t.Errorf("AllowsValue(a) against one terminal method = %v, want %q", err, want)
	}
	n.Response.AuthMethods = append(n.Response.AuthMethods, NewAuthMethodAgent("t", "T, agent"), NewAuthMethodAgent("b", "B"))
	if err := n.AllowsValue(AgentMethodAuthenticate, AuthenticateRequest{MethodId: "t"}); !errors.Is(err, ErrTerminalAuthMethod) {
		t.Errorf("AllowsValue(t), t terminal and agent = %v, want ErrTerminalAuthMethod", err)
	}
	if err := n.AllowsValue(AgentMethodAuthenticate, AuthenticateRequest{MethodId: "z"}); err == nil ||
		!strings.HasSuffix(err.Error(), `; authenticate accepts "b"`) {
		t.Errorf("AllowsValue(z) = %v, want the accepted ids to be b alone", err)
	}
}

// TestPromptUndeclaredBlockGated requires a prompt's content block of a type
// the schema does not list to be refused even with every prompt capability
// advertised, since "Other variants must be explicitly opted in to" and no
// capability opts one in: sent, the typed method returns a *CapabilityError
// naming the type and the block, writing nothing; received, it is answered
// -32602. Text and resource-link blocks pass
// (0049-PLAN-surface-fixes.md, phase 4, S9).
func TestPromptUndeclaredBlockGated(t *testing.T) {
	t.Parallel()
	n := allAdvertised()
	prompt := func(blocks string) PromptRequest {
		return decoded[PromptRequest](t, `{"sessionId":"s","prompt":[{"type":"text","text":"a"},`+blocks+`]}`)
	}
	m := gatedMethod{wire: AgentMethodSessionPrompt, toAgent: true, send: fromClient((*ClientSideConnection).Prompt),
		params: prompt(`{"type":"video","uri":"file:///v"}`)}
	if b := m.params.(PromptRequest).Prompt[1]; b.Unknown == nil {
		t.Fatalf("the video block decoded as %+v, want Unknown", b)
	}

	w := &countingWriter{}
	asc, csc := sides(t, w, n)
	// A send the gate lets through waits for an answer that never comes.
	refused, cancelRefused := context.WithTimeout(t.Context(), 2*time.Second)
	err := m.send(refused, asc, csc, m.params)
	cancelRefused()
	ce, ok := errors.AsType[*CapabilityError](err)
	if !ok || ce.Method != AgentMethodSessionPrompt || ce.Capability != "promptCapabilities.video" || ce.Field != "prompt[1]" || ce.Variant != "video" {
		t.Errorf("send = %v, want *CapabilityError{session/prompt, promptCapabilities.video, prompt[1], video}", err)
	}
	if got := w.count(); got != 0 {
		t.Errorf("%d bytes written, want none", got)
	}
	if rerr, ran := receive(t, m, n); rerr == nil || rerr.Code != -32602 || ran {
		t.Errorf("received: %v, handler ran %v, want -32602 and no handler", rerr, ran)
	}
	if err := n.AllowsValue(AgentMethodSessionPrompt, m.params); !isCapabilityError(err) {
		t.Errorf("AllowsValue = %v, want the *CapabilityError", err)
	}

	m.params = prompt(`{"type":"resource_link","uri":"file:///x","name":"x"}`)
	if rerr, ran := receive(t, m, versionOnly()); rerr != nil || !ran {
		t.Errorf("received text and a resource link advertising nothing: %v, handler ran %v, want it handled", rerr, ran)
	}
	ctx, cancel := context.WithCancel(t.Context())
	sent := make(chan error, 1)
	go func() { sent <- m.send(ctx, asc, csc, m.params) }()
	for deadline := time.Now().Add(5 * time.Second); w.count() == 0 && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := within(t, sent, "the send"); w.count() == 0 || isCapabilityError(err) {
		t.Errorf("text and a resource link were not sent: %v", err)
	}
}

// TestUnstableCapabilitiesGated requires the unstable capabilities to gate
// what they name, each refused without its capability and allowed with it:
// the 13 methods as TestMethodGates checks a method, a received one answered
// -32601; the plan_update and plan_removed session updates as TestValueGates
// checks a variant, a received one answered -32602; and the jump, rename and
// searchAndReplace suggestions of an nes/suggest result, which an agent
// answering one the client did not advertise answers -32603 and a client
// given one returns as a *CapabilityError, as TestResultGates has it. An edit
// suggestion, the baseline, needs none (0049-PLAN-surface-fixes.md, phase 4,
// S3).
func TestUnstableCapabilitiesGated(t *testing.T) {
	t.Parallel()
	methods := unstableGatedMethods()
	if len(methods) != 13 {
		t.Fatalf("%d unstable gated methods, want 13", len(methods))
	}
	for _, m := range methods {
		t.Run(m.wire, func(t *testing.T) {
			t.Parallel()
			checkMethodGate(t, m)
		})
	}

	plan := func(variant, raw string) valueGate {
		return valueGate{gatedMethod: gatedMethod{wire: ClientMethodSessionUpdate, capability: "plan", notifies: true,
			params: decoded[SessionNotification](t, `{"sessionId":"s","update":`+raw+`}`),
			send: func(ctx context.Context, asc *AgentSideConnection, _ *ClientSideConnection, params any) error {
				return asc.SessionUpdate(ctx, params.(SessionNotification))
			},
			advertise: func(n *Negotiation, on bool) { n.Request.ClientCapabilities.Plan = when(on, &PlanCapabilities{}) }}, variant: variant}
	}
	for _, g := range []valueGate{
		plan("plan_update", `{"sessionUpdate":"plan_update","plan":{"type":"markdown","planId":"p","content":"x"}}`),
		plan("plan_removed", `{"sessionUpdate":"plan_removed","planId":"p"}`),
	} {
		t.Run(g.variant, func(t *testing.T) {
			t.Parallel()
			checkValueGate(t, g)
		})
	}

	const at = `"uri":"file:///x","position":{"line":0,"character":0}`
	for _, k := range []struct {
		kind, capability, raw string
		take                  func(c *ClientNesCapabilities)
	}{
		{"jump", "nes.jump", `{"kind":"jump","id":"j",` + at + `}`, func(c *ClientNesCapabilities) { c.Jump = nil }},
		{"rename", "nes.rename", `{"kind":"rename","id":"r","newName":"n",` + at + `}`, func(c *ClientNesCapabilities) { c.Rename = nil }},
		{"searchAndReplace", "nes.searchAndReplace", `{"kind":"searchAndReplace","id":"s","uri":"file:///x","search":"a","replace":"b"}`,
			func(c *ClientNesCapabilities) { c.SearchAndReplace = nil }},
		{"edit", "", `{"kind":"edit","id":"e","uri":"file:///x","edits":[]}`, func(c *ClientNesCapabilities) { *c = ClientNesCapabilities{} }},
	} {
		t.Run("suggestion "+k.kind, func(t *testing.T) {
			t.Parallel()
			answer := `{"suggestions":[` + k.raw + `]}`
			result := decoded[UnstableSuggestNesResponse](t, answer)
			request := UnstableSuggestNesRequest{SessionId: "s", Uri: "file:///x", TriggerKind: UnstableNesTriggerKindManual}
			for _, advertised := range []bool{false, true} {
				n := allAdvertised()
				if !advertised {
					k.take(n.Request.ClientCapabilities.Nes)
				}
				refused := !advertised && k.capability != ""
				asc := NewAgentSideConnection(agentFuncs{UnstableSuggestNesFunc: func(context.Context, UnstableSuggestNesRequest) (UnstableSuggestNesResponse, error) {
					return result, nil
				}}, &countingWriter{}, idleReader(t))
				negotiate(t, asc, n)
				_, rerr := asc.handle(t.Context(), AgentMethodNesSuggest, json.RawMessage(jsonOf(t, request)))
				if refused != (rerr != nil) || (rerr != nil && rerr.Code != -32603) {
					t.Errorf("advertised %v: the agent answered %v", advertised, rerr)
				}
				peer := rawPeer(t, answer)
				negotiate(t, peer, n)
				_, err := peer.UnstableSuggestNes(t.Context(), request)
				ce, isCE := errors.AsType[*CapabilityError](err)
				if refused != (err != nil) || (err != nil && (!isCE || ce.Method != AgentMethodNesSuggest || ce.Capability != k.capability ||
					ce.Field != "suggestions[0]" || ce.Variant != k.kind)) {
					t.Errorf("advertised %v: the client's UnstableSuggestNes = %v", advertised, err)
				}
			}
		})
	}
}

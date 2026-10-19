package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// sender sends one typed message from the side that sends it.
type sender func(ctx context.Context, asc *AgentSideConnection, csc *ClientSideConnection, params any) error

// fromClient is the client's typed request m as a sender.
func fromClient[P, R any](m func(*ClientSideConnection, context.Context, P) (R, error)) sender {
	return func(ctx context.Context, _ *AgentSideConnection, csc *ClientSideConnection, params any) error {
		_, err := m(csc, ctx, params.(P))
		return err
	}
}

// fromAgent is the agent's typed request m as a sender.
func fromAgent[P, R any](m func(*AgentSideConnection, context.Context, P) (R, error)) sender {
	return func(ctx context.Context, asc *AgentSideConnection, _ *ClientSideConnection, params any) error {
		_, err := m(asc, ctx, params.(P))
		return err
	}
}

// when is v when on, and nil otherwise.
func when[T any](on bool, v *T) *T {
	if on {
		return v
	}
	return nil
}

// gatedMethod is one method gate of the protocol table: the method, its
// capability, whether the agent receives it, whether it is a notification,
// valid parameters, its typed send, and how to advertise the capability in a
// record or take it away.
type gatedMethod struct {
	wire, capability  string
	toAgent, notifies bool
	params            any
	send              sender
	advertise         func(n *Negotiation, on bool)
}

// gatedMethods returns the 15 method gates of the protocol table
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 9).
func gatedMethods() []gatedMethod {
	agentCaps := func(n *Negotiation) *AgentCapabilities { return &n.Response.AgentCapabilities }
	sessionCaps := func(n *Negotiation) *SessionCapabilities { return &agentCaps(n).SessionCapabilities }
	clientCaps := func(n *Negotiation) *ClientCapabilities { return &n.Request.ClientCapabilities }
	terminal := func(n *Negotiation, on bool) { clientCaps(n).Terminal = on }
	const s, term = SessionId("s"), TerminalId("t")
	return []gatedMethod{
		{wire: AgentMethodSessionList, capability: "sessionCapabilities.list", toAgent: true,
			params: ListSessionsRequest{}, send: fromClient((*ClientSideConnection).ListSessions),
			advertise: func(n *Negotiation, on bool) { sessionCaps(n).List = when(on, &SessionListCapabilities{}) }},
		{wire: AgentMethodSessionDelete, capability: "sessionCapabilities.delete", toAgent: true,
			params: DeleteSessionRequest{SessionId: s}, send: fromClient((*ClientSideConnection).DeleteSession),
			advertise: func(n *Negotiation, on bool) { sessionCaps(n).Delete = when(on, &SessionDeleteCapabilities{}) }},
		{wire: AgentMethodSessionResume, capability: "sessionCapabilities.resume", toAgent: true,
			params: ResumeSessionRequest{SessionId: s, Cwd: testAbsPath()}, send: fromClient((*ClientSideConnection).ResumeSession),
			advertise: func(n *Negotiation, on bool) { sessionCaps(n).Resume = when(on, &SessionResumeCapabilities{}) }},
		{wire: AgentMethodSessionClose, capability: "sessionCapabilities.close", toAgent: true,
			params: CloseSessionRequest{SessionId: s}, send: fromClient((*ClientSideConnection).CloseSession),
			advertise: func(n *Negotiation, on bool) { sessionCaps(n).Close = when(on, &SessionCloseCapabilities{}) }},
		{wire: AgentMethodSessionFork, capability: "sessionCapabilities.fork", toAgent: true,
			params: UnstableForkSessionRequest{SessionId: s, Cwd: testAbsPath()}, send: fromClient((*ClientSideConnection).UnstableForkSession),
			advertise: func(n *Negotiation, on bool) { sessionCaps(n).Fork = when(on, &SessionForkCapabilities{}) }},
		{wire: AgentMethodLogout, capability: "auth.logout", toAgent: true,
			params: LogoutRequest{}, send: fromClient((*ClientSideConnection).Logout),
			advertise: func(n *Negotiation, on bool) { agentCaps(n).Auth.Logout = when(on, &LogoutCapabilities{}) }},
		{wire: AgentMethodSessionLoad, capability: "loadSession", toAgent: true,
			params: LoadSessionRequest{SessionId: s, Cwd: testAbsPath(), McpServers: []McpServer{}}, send: fromClient((*ClientSideConnection).LoadSession),
			advertise: func(n *Negotiation, on bool) { agentCaps(n).LoadSession = on }},
		{wire: ClientMethodFsReadTextFile, capability: "fs.readTextFile",
			params: ReadTextFileRequest{SessionId: s, Path: testAbsPath("f")}, send: fromAgent((*AgentSideConnection).ReadTextFile),
			advertise: func(n *Negotiation, on bool) { clientCaps(n).Fs.ReadTextFile = on }},
		{wire: ClientMethodFsWriteTextFile, capability: "fs.writeTextFile",
			params: WriteTextFileRequest{SessionId: s, Path: testAbsPath("f"), Content: "x"}, send: fromAgent((*AgentSideConnection).WriteTextFile),
			advertise: func(n *Negotiation, on bool) { clientCaps(n).Fs.WriteTextFile = on }},
		{wire: ClientMethodTerminalCreate, capability: "terminal",
			params: CreateTerminalRequest{SessionId: s, Command: "ls"}, send: fromAgent((*AgentSideConnection).CreateTerminal), advertise: terminal},
		{wire: ClientMethodTerminalOutput, capability: "terminal",
			params: TerminalOutputRequest{SessionId: s, TerminalId: term}, send: fromAgent((*AgentSideConnection).TerminalOutput), advertise: terminal},
		{wire: ClientMethodTerminalRelease, capability: "terminal",
			params: ReleaseTerminalRequest{SessionId: s, TerminalId: term}, send: fromAgent((*AgentSideConnection).ReleaseTerminal), advertise: terminal},
		{wire: ClientMethodTerminalWaitForExit, capability: "terminal",
			params: WaitForTerminalExitRequest{SessionId: s, TerminalId: term}, send: fromAgent((*AgentSideConnection).WaitForTerminalExit), advertise: terminal},
		{wire: ClientMethodTerminalKill, capability: "terminal",
			params: KillTerminalRequest{SessionId: s, TerminalId: term}, send: fromAgent((*AgentSideConnection).KillTerminal), advertise: terminal},
		{wire: ClientMethodElicitationComplete, capability: "elicitation.url", notifies: true,
			params: CompleteElicitationNotification{ElicitationId: "e"},
			send: func(ctx context.Context, asc *AgentSideConnection, _ *ClientSideConnection, params any) error {
				return asc.CompleteElicitation(ctx, params.(CompleteElicitationNotification))
			},
			advertise: func(n *Negotiation, on bool) {
				if clientCaps(n).Elicitation == nil {
					clientCaps(n).Elicitation = &ElicitationCapabilities{}
				}
				clientCaps(n).Elicitation.Url = when(on, &ElicitationUrlCapabilities{})
			}},
	}
}

// versionOnly is a record that advertises nothing.
func versionOnly() *Negotiation {
	return &Negotiation{
		Request:  InitializeRequest{ProtocolVersion: ProtocolVersionNumber},
		Response: InitializeResponse{ProtocolVersion: ProtocolVersionNumber},
	}
}

// records returns, for m, a record that advertises everything but m's
// capability, and one that advertises only m's capability.
func records(m gatedMethod) (without, only *Negotiation) {
	without, only = allAdvertised(), versionOnly()
	m.advertise(without, false)
	m.advertise(only, true)
	return without, only
}

// sides returns an agent and a client connection that both write to w and
// never receive, each with record n.
func sides(t *testing.T, w *countingWriter, n *Negotiation, opts ...ConnectionOption) (*AgentSideConnection, *ClientSideConnection) {
	t.Helper()
	calls := make(chan any, 1)
	asc := NewAgentSideConnection(recordingAgent{calls: calls}, w, idleReader(t), opts...)
	csc := NewClientSideConnection(recordingClient{calls: calls}, w, idleReader(t), opts...)
	if n != nil {
		negotiate(t, asc, n)
		negotiate(t, csc, n)
	}
	return asc, csc
}

// validReply fills a result the zero value of which does not validate.
func validReply(result any) error {
	if r, ok := result.(*CreateElicitationResponse); ok {
		*r = NewCreateElicitationResponseCancel()
	}
	return nil
}

// receive hands m to the side that receives it, returning its answer and
// whether the handler ran.
func receive(t *testing.T, m gatedMethod, n *Negotiation, opts ...ConnectionOption) (*RequestError, bool) {
	t.Helper()
	calls := make(chan any, 1)
	raw, err := json.Marshal(m.params)
	if err != nil {
		t.Fatal(err)
	}
	var rerr *RequestError
	if m.toAgent {
		asc := NewAgentSideConnection(recordingAgent{calls: calls}, &countingWriter{}, idleReader(t), opts...)
		if n != nil {
			negotiate(t, asc, n)
		}
		_, rerr = asc.handle(t.Context(), m.wire, raw)
	} else {
		csc := NewClientSideConnection(recordingClient{calls: calls, reply: validReply}, &countingWriter{}, idleReader(t), opts...)
		if n != nil {
			negotiate(t, csc, n)
		}
		_, rerr = csc.handle(t.Context(), m.wire, raw)
	}
	return rerr, len(calls) == 1
}

// wantMethodGate fails unless err is the *CapabilityError of m's method gate.
func wantMethodGate(t *testing.T, what string, m gatedMethod, err error) {
	t.Helper()
	ce, ok := errors.AsType[*CapabilityError](err)
	if !ok || ce.Method != m.wire || ce.Capability != m.capability || ce.Field != "" || ce.Variant != "" {
		t.Errorf("%s: %v, want *CapabilityError{Method: %q, Capability: %q}", what, err, m.wire, m.capability)
	}
}

// TestMethodGates requires each of the 15 method gates of the protocol table
// both ways: a typed send the record does not allow returns a
// *CapabilityError and writes nothing, and one it allows is written; a
// received message it does not allow is answered -32601 without reaching the
// handler, one it allows reaches it, and a validation hook that accepts the
// *CapabilityError lets it through (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 9).
func TestMethodGates(t *testing.T) {
	t.Parallel()
	methods := gatedMethods()
	if len(methods) != 15 {
		t.Fatalf("%d gated methods, want the table's 15", len(methods))
	}
	for _, m := range methods {
		t.Run(m.wire, func(t *testing.T) {
			t.Parallel()
			without, only := records(m)
			for name, n := range map[string]*Negotiation{"advertising everything else": without, "advertising nothing": versionOnly()} {
				w := &countingWriter{}
				asc, csc := sides(t, w, n)
				// A send the gate lets through waits for an answer that never comes.
				ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
				wantMethodGate(t, "send "+name, m, m.send(ctx, asc, csc, m.params))
				cancel()
				if got := w.count(); got != 0 {
					t.Errorf("send %s: %d bytes written, want none", name, got)
				}
				rerr, ran := receive(t, m, n)
				if rerr == nil || rerr.Code != -32601 || ran {
					t.Errorf("receive %s: answered %v, handler ran %v, want -32601 and no handler", name, rerr, ran)
				}
			}

			w := &countingWriter{}
			asc, csc := sides(t, w, only)
			ctx, cancel := context.WithCancel(t.Context())
			sent := make(chan error, 1)
			go func() { sent <- m.send(ctx, asc, csc, m.params) }()
			deadline := time.Now().Add(5 * time.Second)
			for w.count() == 0 && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if w.count() == 0 {
				t.Errorf("send advertising only %s: nothing written", m.capability)
			}
			cancel()
			if err := within(t, sent, "the send"); isCapabilityError(err) || (m.notifies && err != nil) {
				t.Errorf("send advertising only %s: %v", m.capability, err)
			}
			if rerr, ran := receive(t, m, only); rerr != nil || !ran {
				t.Errorf("receive advertising only %s: answered %v, handler ran %v, want it handled", m.capability, rerr, ran)
			}

			accept := WithInboundValidation(func(_ context.Context, _ string, _ any, err error) error {
				if isCapabilityError(err) {
					return nil
				}
				return err
			})
			if rerr, ran := receive(t, m, without, accept); rerr != nil || !ran {
				t.Errorf("receive with a hook accepting the gate: answered %v, handler ran %v, want it handled", rerr, ran)
			}
		})
	}
}

// TestHookRulesOnEachError requires a validation hook that accepts a
// message's *ValidationError not to accept the method gate with it: the hook
// is asked about each, and the method is still refused -32601
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 9 deviation).
func TestHookRulesOnEachError(t *testing.T) {
	t.Parallel()
	var seen []error
	asc := NewAgentSideConnection(recordingAgent{calls: make(chan any, 1)}, &countingWriter{}, idleReader(t),
		WithInboundValidation(func(_ context.Context, _ string, _ any, err error) error {
			seen = append(seen, err)
			if _, ok := errors.AsType[*ValidationError](err); ok {
				return nil
			}
			return err
		}))
	negotiate(t, asc, versionOnly())
	_, rerr := asc.handle(t.Context(), AgentMethodSessionList, json.RawMessage(`{"cwd":"relative"}`))
	if rerr == nil || rerr.Code != -32601 {
		t.Errorf("handle = %v, want -32601", rerr)
	}
	if len(seen) != 2 || !isValidationError(seen[0]) || !isCapabilityError(seen[1]) {
		t.Errorf("the hook saw %v, want the *ValidationError, then the *CapabilityError", seen)
	}
}

// TestNotInitialized requires every typed method but Initialize to return an
// error wrapping ErrNotInitialized before initialize, writing nothing, and a
// received request to be answered -32601, while initialize itself is handled
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 9).
func TestNotInitialized(t *testing.T) {
	t.Parallel()
	w := &countingWriter{}
	asc, csc := sides(t, w, nil)
	ctxType := reflect.TypeFor[context.Context]()
	checked := 0
	for _, side := range []any{asc, csc} {
		v := reflect.ValueOf(side)
		for i := range v.NumMethod() {
			name, mt := v.Type().Method(i).Name, v.Method(i).Type()
			if name == "Initialize" || mt.NumIn() != 2 || mt.In(0) != ctxType || mt.In(1).Kind() != reflect.Struct {
				continue
			}
			// A method the gate lets through waits for an answer that never comes.
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			out := v.Method(i).Call([]reflect.Value{reflect.ValueOf(ctx), reflect.New(mt.In(1)).Elem()})
			cancel()
			err, _ := reflect.TypeAssert[error](out[len(out)-1])
			if !errors.Is(err, ErrNotInitialized) {
				t.Errorf("%T.%s = %v, want an error wrapping ErrNotInitialized", side, name, err)
			}
			checked++
		}
	}
	if checked < 30 {
		t.Errorf("%d typed methods checked, want every one", checked)
	}
	if got := w.count(); got != 0 {
		t.Errorf("%d bytes written, want none", got)
	}
	raw, _ := json.Marshal(NewSessionRequest{Cwd: testAbsPath(), McpServers: []McpServer{}})
	if _, rerr := asc.handle(t.Context(), AgentMethodSessionNew, raw); rerr == nil || rerr.Code != -32601 {
		t.Errorf("session/new before initialize: %v, want -32601", rerr)
	}
	raw, _ = json.Marshal(ReadTextFileRequest{SessionId: "s", Path: testAbsPath("f")})
	if _, rerr := csc.handle(t.Context(), ClientMethodFsReadTextFile, raw); rerr == nil || rerr.Code != -32601 {
		t.Errorf("fs/read_text_file before initialize: %v, want -32601", rerr)
	}
	if _, rerr := asc.handle(t.Context(), AgentMethodInitialize, json.RawMessage(`{"protocolVersion":1}`)); rerr != nil {
		t.Errorf("initialize: %v, want it handled", rerr)
	}
}

// TestRawPathsUngated requires the connection's own requests and
// notifications and the extension methods to send before initialize, a gated
// method included (0017-MADR-remediate-acp-v1-conformance-violations.md, G1).
func TestRawPathsUngated(t *testing.T) {
	t.Parallel()
	csc := rawPeer(t, `{"sessions":[]}`)
	if _, err := SendRequest[ListSessionsResponse](csc.Connection(), t.Context(), AgentMethodSessionList, ListSessionsRequest{}); err != nil {
		t.Errorf("SendRequest: %v", err)
	}
	if err := csc.Connection().SendNotification(t.Context(), AgentMethodSessionCancel, CancelNotification{SessionId: "s"}); err != nil {
		t.Errorf("SendNotification: %v", err)
	}
	if _, err := csc.CallExtension(t.Context(), "_x/ping", map[string]any{}); err != nil {
		t.Errorf("CallExtension: %v", err)
	}
	if _, ok := csc.Negotiated(); ok {
		t.Error("a record exists, want none")
	}
}

// TestAllows requires Negotiation.Allows to agree with the gate: nil for a
// method the table does not gate, for initialize and for a gated method whose
// capability is advertised, and the gate's *CapabilityError otherwise
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 9).
func TestAllows(t *testing.T) {
	t.Parallel()
	none := versionOnly()
	for _, method := range []string{AgentMethodInitialize, AgentMethodSessionNew, AgentMethodSessionPrompt, ClientMethodSessionUpdate, ClientMethodSessionRequestPermission} {
		if err := none.Allows(method); err != nil {
			t.Errorf("Allows(%s) = %v, want nil", method, err)
		}
	}
	if err := gateMessage(nil, AgentMethodInitialize, nil); err != nil {
		t.Errorf("initialize without a record: %v, want nil", err)
	}
	for _, m := range gatedMethods() {
		without, only := records(m)
		wantMethodGate(t, "Allows("+m.wire+") without its capability", m, without.Allows(m.wire))
		if err := only.Allows(m.wire); err != nil {
			t.Errorf("Allows(%s) with only %s = %v, want nil", m.wire, m.capability, err)
		}
		for _, n := range []*Negotiation{without, only, none} {
			if a, g := n.Allows(m.wire), gateMessage(n, m.wire, m.params); !reflect.DeepEqual(a, g) {
				t.Errorf("%s: Allows = %v, gate = %v", m.wire, a, g)
			}
		}
	}
}

// valueGate is one capability rule of the protocol table on a message that
// carries its property or variant: the gatedMethod fields say how it is sent
// and received and how to advertise the capability, and field and variant
// what the *CapabilityError names.
type valueGate struct {
	gatedMethod
	field, variant string
}

// decoded is raw decoded into a T, as a peer's message would be.
func decoded[T any](t *testing.T, raw string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
	return v
}

// jsonOf is v as JSON text.
func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// valueGates returns a message for each of the 17 capability rules that a
// typed method sends, and the three MCP rules again on session/fork, whose
// servers are the unstable copy of McpServer (0019-PLAN-behaviour-fixes.md,
// B6); the terminal-auth rule, the undeclared elicitation mode
// (TestUndeclaredElicitationMode) and the result-only gates have tests of
// their own.
func valueGates(t *testing.T) []valueGate {
	t.Helper()
	dir, other := jsonOf(t, testAbsPath()), jsonOf(t, testAbsPath("more"))
	roots := `"additionalDirectories":[` + other + `]`
	agentCaps := func(n *Negotiation) *AgentCapabilities { return &n.Response.AgentCapabilities }
	clientCaps := func(n *Negotiation) *ClientCapabilities { return &n.Request.ClientCapabilities }
	rootsCap := func(n *Negotiation, on bool) {
		agentCaps(n).SessionCapabilities.AdditionalDirectories = when(on, &SessionAdditionalDirectoriesCapabilities{})
	}
	session := func(n *Negotiation) *ClientSessionCapabilities {
		if clientCaps(n).Session == nil {
			clientCaps(n).Session = &ClientSessionCapabilities{}
		}
		return clientCaps(n).Session
	}
	elicitation := func(n *Negotiation) *ElicitationCapabilities {
		if clientCaps(n).Elicitation == nil {
			clientCaps(n).Elicitation = &ElicitationCapabilities{}
		}
		return clientCaps(n).Elicitation
	}
	newSession := func(extra string) any {
		return decoded[NewSessionRequest](t, `{"cwd":`+dir+`,"mcpServers":[`+extra+`]}`)
	}
	withRoots := func(m gatedMethod) valueGate {
		m.capability, m.toAgent, m.advertise = "sessionCapabilities.additionalDirectories", true, rootsCap
		return valueGate{gatedMethod: m, field: "additionalDirectories"}
	}
	mcp := func(variant, server string, advertise func(n *Negotiation, on bool)) valueGate {
		return valueGate{wire: AgentMethodSessionNew, capability: "mcpCapabilities." + variant, toAgent: true,
			params: newSession(server), send: fromClient((*ClientSideConnection).NewSession), advertise: advertise, variant: variant}
	}
	forkMcp := func(variant, server string, advertise func(n *Negotiation, on bool)) valueGate {
		return valueGate{wire: AgentMethodSessionFork, capability: "mcpCapabilities." + variant, toAgent: true,
			params: decoded[UnstableForkSessionRequest](t, `{"sessionId":"s","cwd":`+dir+`,"mcpServers":[`+server+`]}`),
			send:   fromClient((*ClientSideConnection).UnstableForkSession), advertise: advertise, variant: variant}
	}
	block := func(variant, capability, raw string, advertise func(n *Negotiation, on bool)) valueGate {
		return valueGate{wire: AgentMethodSessionPrompt, capability: "promptCapabilities." + capability, toAgent: true,
			params: decoded[PromptRequest](t, `{"sessionId":"s","prompt":[{"type":"text","text":"a"},`+raw+`]}`),
			send:   fromClient((*ClientSideConnection).Prompt), advertise: advertise, field: "prompt[1]", variant: variant}
	}
	update := func(variant, capability, raw string, advertise func(n *Negotiation, on bool)) valueGate {
		return valueGate{wire: ClientMethodSessionUpdate, capability: capability, notifies: true,
			params: decoded[SessionNotification](t, `{"sessionId":"s","update":`+raw+`}`),
			send: func(ctx context.Context, asc *AgentSideConnection, _ *ClientSideConnection, params any) error {
				return asc.SessionUpdate(ctx, params.(SessionNotification))
			}, advertise: advertise, variant: variant}
	}
	elicit := func(variant, raw string, advertise func(n *Negotiation, on bool)) valueGate {
		return valueGate{wire: ClientMethodElicitationCreate, capability: "elicitation." + variant,
			params: decoded[CreateElicitationRequest](t, raw), send: fromAgent((*AgentSideConnection).CreateElicitation),
			advertise: advertise, variant: variant}
	}
	return []valueGate{
		withRoots(gatedMethod{wire: AgentMethodSessionNew, params: decoded[NewSessionRequest](t, `{"cwd":`+dir+`,"mcpServers":[],`+roots+`}`),
			send: fromClient((*ClientSideConnection).NewSession)}),
		withRoots(gatedMethod{wire: AgentMethodSessionLoad,
			params: decoded[LoadSessionRequest](t, `{"sessionId":"s","cwd":`+dir+`,"mcpServers":[],`+roots+`}`),
			send:   fromClient((*ClientSideConnection).LoadSession)}),
		withRoots(gatedMethod{wire: AgentMethodSessionResume, params: decoded[ResumeSessionRequest](t, `{"sessionId":"s","cwd":`+dir+`,`+roots+`}`),
			send: fromClient((*ClientSideConnection).ResumeSession)}),
		withRoots(gatedMethod{wire: AgentMethodSessionFork, params: decoded[UnstableForkSessionRequest](t, `{"sessionId":"s","cwd":`+dir+`,`+roots+`}`),
			send: fromClient((*ClientSideConnection).UnstableForkSession)}),
		mcp("http", `{"type":"http","name":"m","url":"https://example.invalid","headers":[]}`,
			func(n *Negotiation, on bool) { agentCaps(n).McpCapabilities.Http = on }),
		mcp("sse", `{"type":"sse","name":"m","url":"https://example.invalid","headers":[]}`,
			func(n *Negotiation, on bool) { agentCaps(n).McpCapabilities.Sse = on }),
		mcp("acp", `{"type":"acp","name":"m","serverId":"x"}`, func(n *Negotiation, on bool) { agentCaps(n).McpCapabilities.Acp = on }),
		forkMcp("http", `{"type":"http","name":"m","url":"https://example.invalid","headers":[]}`,
			func(n *Negotiation, on bool) { agentCaps(n).McpCapabilities.Http = on }),
		forkMcp("sse", `{"type":"sse","name":"m","url":"https://example.invalid","headers":[]}`,
			func(n *Negotiation, on bool) { agentCaps(n).McpCapabilities.Sse = on }),
		forkMcp("acp", `{"type":"acp","name":"m","serverId":"x"}`, func(n *Negotiation, on bool) { agentCaps(n).McpCapabilities.Acp = on }),
		block("image", "image", `{"type":"image","data":"x","mimeType":"image/png"}`,
			func(n *Negotiation, on bool) { agentCaps(n).PromptCapabilities.Image = on }),
		block("audio", "audio", `{"type":"audio","data":"x","mimeType":"audio/wav"}`,
			func(n *Negotiation, on bool) { agentCaps(n).PromptCapabilities.Audio = on }),
		block("resource", "embeddedContext", `{"type":"resource","resource":{"uri":"file:///x","text":"t"}}`,
			func(n *Negotiation, on bool) { agentCaps(n).PromptCapabilities.EmbeddedContext = on }),
		elicit("form", `{"mode":"form","message":"m","requestedSchema":{"type":"object","properties":{}},"sessionId":"s"}`,
			func(n *Negotiation, on bool) { elicitation(n).Form = when(on, &ElicitationFormCapabilities{}) }),
		elicit("url", `{"mode":"url","message":"m","elicitationId":"e","url":"https://example.invalid","sessionId":"s"}`,
			func(n *Negotiation, on bool) { elicitation(n).Url = when(on, &ElicitationUrlCapabilities{}) }),
		update("notice", "session.notices", `{"sessionUpdate":"notice","severity":"info","title":"t"}`,
			func(n *Negotiation, on bool) { session(n).Notices = when(on, &NoticeCapabilities{}) }),
		update("compaction_update", "session.compaction", `{"sessionUpdate":"compaction_update","compactionId":"c","status":"completed"}`,
			func(n *Negotiation, on bool) { session(n).Compaction = when(on, &CompactionCapabilities{}) }),
		update("compaction_summary_chunk", "session.compaction",
			`{"sessionUpdate":"compaction_summary_chunk","compactionId":"c","content":{"type":"text","text":"x"}}`,
			func(n *Negotiation, on bool) { session(n).Compaction = when(on, &CompactionCapabilities{}) }),
		// The subagent updates (0019-PLAN-issue-fixes.md, B24).
		update("subagent_update", "subagents", `{"sessionUpdate":"subagent_update","sessionId":"child"}`,
			func(n *Negotiation, on bool) {
				n.Request.ClientCapabilities.Subagents = when(on, &SubagentCapabilities{})
			}),
		update("session_message", "subagents", `{"sessionUpdate":"session_message","messageId":"m1"}`,
			func(n *Negotiation, on bool) {
				n.Request.ClientCapabilities.Subagents = when(on, &SubagentCapabilities{})
			}),
		update("session_message_chunk", "subagents",
			`{"sessionUpdate":"session_message_chunk","messageId":"m1","content":{"type":"text","text":"x"}}`,
			func(n *Negotiation, on bool) {
				n.Request.ClientCapabilities.Subagents = when(on, &SubagentCapabilities{})
			}),
		update("boolean", "session.configOptions.boolean",
			`{"sessionUpdate":"config_option_update","configOptions":[{"type":"boolean","id":"b","name":"B","currentValue":true}]}`,
			func(n *Negotiation, on bool) {
				if on {
					session(n).ConfigOptions = &SessionConfigOptionsCapabilities{Boolean: &BooleanConfigOptionCapabilities{}}
				} else if session(n).ConfigOptions != nil {
					session(n).ConfigOptions.Boolean = nil
				}
			}),
	}
}

// TestValueGates requires each capability rule both ways: sent with a record
// that does not advertise its capability, the typed method returns a
// *CapabilityError naming the property or variant and writes nothing; with one
// that does, it is written; received without it, it is answered -32602
// (a notification's answer is dropped) without reaching the handler, and with
// it, it is handled (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 10).
func TestValueGates(t *testing.T) {
	t.Parallel()
	gates := valueGates(t)
	// 22 with the three subagent updates (0019-PLAN-issue-fixes.md, B24).
	if len(gates) != 22 {
		t.Fatalf("%d value gates, want 22", len(gates))
	}
	for _, g := range gates {
		t.Run(g.capability+" "+g.variant+g.field, func(t *testing.T) {
			t.Parallel()
			without, with := allAdvertised(), allAdvertised()
			g.advertise(without, false)
			w := &countingWriter{}
			asc, csc := sides(t, w, without)
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			err := g.send(ctx, asc, csc, g.params)
			cancel()
			ce, ok := errors.AsType[*CapabilityError](err)
			if !ok || ce.Method != g.wire || ce.Capability != g.capability || ce.Field != g.field || ce.Variant != g.variant {
				t.Errorf("send = %v, want *CapabilityError{%s, %s, Field %q, Variant %q}", err, g.wire, g.capability, g.field, g.variant)
			}
			if got := w.count(); got != 0 {
				t.Errorf("%d bytes written, want none", got)
			}
			if rerr, ran := receive(t, g.gatedMethod, without); rerr == nil || rerr.Code != -32602 || ran {
				t.Errorf("received without the capability: %v, handler ran %v, want -32602 and no handler", rerr, ran)
			}

			w = &countingWriter{}
			asc, csc = sides(t, w, with)
			ctx, cancel = context.WithCancel(t.Context())
			sent := make(chan error, 1)
			go func() { sent <- g.send(ctx, asc, csc, g.params) }()
			// A notification's send returns once written, so it is awaited
			// before the cancel; a request's waits for an answer that never
			// comes, so it is cancelled once its bytes are seen
			// (0019-PLAN-api-decisions.md, step 7's deviation).
			err = nil
			if g.notifies {
				err = within(t, sent, "the send")
			}
			for deadline := time.Now().Add(5 * time.Second); w.count() == 0 && time.Now().Before(deadline); {
				time.Sleep(time.Millisecond)
			}
			if w.count() == 0 {
				t.Error("with the capability, nothing written")
			}
			cancel()
			if !g.notifies {
				err = within(t, sent, "the send")
			}
			if isCapabilityError(err) || (g.notifies && err != nil) {
				t.Errorf("send with the capability: %v", err)
			}
			if rerr, ran := receive(t, g.gatedMethod, with); rerr != nil || !ran {
				t.Errorf("received with the capability: %v, handler ran %v, want it handled", rerr, ran)
			}
		})
	}
}

// TestResultGates requires a result to be checked against the gates too: an
// agent whose session/new result holds a boolean config option for a client
// that did not advertise them answers -32603, and a client given one returns
// the *CapabilityError (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 10).
func TestResultGates(t *testing.T) {
	t.Parallel()
	option := `{"sessionId":"s","configOptions":[{"type":"boolean","id":"b","name":"B","currentValue":true}]}`
	result := decoded[NewSessionResponse](t, option)
	params := jsonOf(t, NewSessionRequest{Cwd: testAbsPath(), McpServers: []McpServer{}})
	for _, advertised := range []bool{false, true} {
		n := allAdvertised()
		if !advertised {
			n.Request.ClientCapabilities.Session.ConfigOptions.Boolean = nil
		}
		asc := NewAgentSideConnection(agentFuncs{NewSessionFunc: func(context.Context, NewSessionRequest) (NewSessionResponse, error) {
			return result, nil
		}}, &countingWriter{}, idleReader(t))
		negotiate(t, asc, n)
		_, rerr := asc.handle(t.Context(), AgentMethodSessionNew, json.RawMessage(params))
		if advertised != (rerr == nil) || (rerr != nil && rerr.Code != -32603) {
			t.Errorf("advertised %v: the agent answered %v", advertised, rerr)
		}
		peer := rawPeer(t, option)
		negotiate(t, peer, n)
		_, err := peer.NewSession(t.Context(), NewSessionRequest{Cwd: testAbsPath(), McpServers: []McpServer{}})
		ce, isCE := errors.AsType[*CapabilityError](err)
		if advertised != (err == nil) || (err != nil && (!isCE || ce.Variant != "boolean")) {
			t.Errorf("advertised %v: the client's NewSession = %v", advertised, err)
		}
	}
}

// TestInitializeGates requires initialize's answer to be checked against the
// record it makes: an agent listing a terminal auth method to a client
// without auth.terminal answers -32603 and records nothing, and a client
// given one returns the *CapabilityError and records nothing unless a
// validation hook accepts it (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 10).
func TestInitializeGates(t *testing.T) {
	t.Parallel()
	terminal := InitializeResponse{ProtocolVersion: ProtocolVersionNumber, AuthMethods: []AuthMethod{NewAuthMethodTerminal("t", "T")}}
	for _, advertised := range []bool{false, true} {
		asc := NewAgentSideConnection(agentFuncs{InitializeFunc: func(context.Context, InitializeRequest) (InitializeResponse, error) {
			return terminal, nil
		}}, &countingWriter{}, idleReader(t))
		req := jsonOf(t, InitializeRequest{ProtocolVersion: ProtocolVersionNumber, ClientCapabilities: ClientCapabilities{Auth: AuthCapabilities{Terminal: advertised}}})
		_, rerr := asc.handle(t.Context(), AgentMethodInitialize, json.RawMessage(req))
		if _, ok := asc.Negotiated(); advertised != (rerr == nil) || advertised != ok || (rerr != nil && rerr.Code != -32603) {
			t.Errorf("auth.terminal %v: the agent answered %v, recorded %v", advertised, rerr, ok)
		}
	}
	answer := jsonOf(t, terminal)
	c := rawPeer(t, answer)
	_, err := c.Initialize(t.Context(), InitializeRequest{})
	ce, isCE := errors.AsType[*CapabilityError](err)
	if _, ok := c.Negotiated(); !isCE || ce.Field != "authMethods[0]" || ce.Variant != "terminal" || ok {
		t.Errorf("Initialize = %v, recorded %v; want the *CapabilityError and no record", err, ok)
	}
	accept := WithInboundValidation(func(_ context.Context, _ string, _ any, err error) error {
		if isCapabilityError(err) {
			return nil
		}
		return err
	})
	c = rawPeer(t, answer, accept)
	if _, err := c.Initialize(t.Context(), InitializeRequest{}); err != nil {
		t.Errorf("Initialize with a hook accepting the gate: %v", err)
	}
	if _, ok := c.Negotiated(); !ok {
		t.Error("with a hook accepting the gate, no record")
	}
}

// TestTerminalAuthMethod requires authenticate naming a terminal method of
// the record to be refused with ErrTerminalAuthMethod before it is sent, and
// answered -32602 when received; another method is sent
// (0017-MADR-remediate-acp-v1-conformance-violations.md, G1).
func TestTerminalAuthMethod(t *testing.T) {
	t.Parallel()
	n := allAdvertised()
	n.Response.AuthMethods = []AuthMethod{NewAuthMethodAgent("a", "A"), NewAuthMethodTerminal("t", "T")}
	w := &countingWriter{}
	_, csc := sides(t, w, n)
	err := fromClient((*ClientSideConnection).Authenticate)(t.Context(), nil, csc, AuthenticateRequest{MethodId: "t"})
	if !errors.Is(err, ErrTerminalAuthMethod) || err.Error() != `authenticate: methodId "t": `+ErrTerminalAuthMethod.Error() {
		t.Errorf("Authenticate(t) = %v, want ErrTerminalAuthMethod", err)
	}
	if got := w.count(); got != 0 {
		t.Errorf("%d bytes written, want none", got)
	}
	m := gatedMethod{wire: AgentMethodAuthenticate, toAgent: true, params: AuthenticateRequest{MethodId: "t"}}
	if rerr, ran := receive(t, m, n); rerr == nil || rerr.Code != -32602 || ran {
		t.Errorf("received: %v, handler ran %v, want -32602 and no handler", rerr, ran)
	}
	m.params = AuthenticateRequest{MethodId: "a"}
	if rerr, ran := receive(t, m, n); rerr != nil || !ran {
		t.Errorf("received the agent method: %v, handler ran %v", rerr, ran)
	}
	ctx, cancel := context.WithCancel(t.Context())
	sent := make(chan error, 1)
	go func() {
		sent <- fromClient((*ClientSideConnection).Authenticate)(ctx, nil, csc, AuthenticateRequest{MethodId: "a"})
	}()
	for deadline := time.Now().Add(5 * time.Second); w.count() == 0 && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}
	cancel()
	if w.count() == 0 || errors.Is(within(t, sent, "the send"), ErrTerminalAuthMethod) {
		t.Error("Authenticate(a) was not sent")
	}
}

// TestExtensionMessagesHookedAndGated requires a received extension message to
// pass the inbound hooks and the initialize gate as a typed one does: before
// initialize a request is answered -32601 and a notification dropped, neither
// reaching the handler; after it, both hooks see the method with
// *json.RawMessage params, a rewrite reaches the handler, and a refusal by the
// validation hook is answered -32602 (0019-PLAN-api-decisions.md, B14).
func TestExtensionMessagesHookedAndGated(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var hooked []string
	note := func(s string) {
		mu.Lock()
		defer mu.Unlock()
		hooked = append(hooked, s)
	}
	saw := func(s string) bool {
		mu.Lock()
		defer mu.Unlock()
		return slices.Contains(hooked, s)
	}
	// kind names params' type by assertion: %T prints the alias target of
	// json.RawMessage, which depends on the json build.
	kind := func(params any) string {
		if _, ok := params.(*json.RawMessage); ok {
			return "*json.RawMessage"
		}
		return fmt.Sprintf("%T", params)
	}
	handled := make(chan string, 8)
	var asc *AgentSideConnection
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		asc = NewAgentSideConnection(agentFuncs{HandleExtensionMethodFunc: func(_ context.Context, method string, params json.RawMessage) (any, error) {
			handled <- method + " " + string(params)
			return params, nil
		}}, w, r,
			WithInboundRewrite(func(_ context.Context, method string, params any) error {
				note("rewrite " + method + " " + kind(params))
				if raw, ok := params.(*json.RawMessage); ok && method == "_x/echo" {
					*raw = json.RawMessage(`{"v":2}`)
				}
				return nil
			}),
			WithInboundValidation(func(_ context.Context, method string, params any, err error) error {
				note("validate " + method + " " + kind(params))
				if method == "_x/refuse" {
					return errors.New("refused by the hook")
				}
				return err
			}))
	})

	p.send(`{"jsonrpc":"2.0","id":1,"method":"_x/echo","params":{"v":1}}`)
	if a := p.next("the early request's answer"); string(a.ID) != "1" || a.Error == nil || a.Error.Code != -32601 {
		t.Errorf("a request before initialize: id %s, %+v; want -32601", a.ID, a.Error)
	}
	// The validation hook runs after the gate, so once it has seen the early
	// notification, the gate has refused it.
	p.send(`{"jsonrpc":"2.0","method":"_x/early","params":{}}`)
	for deadline := time.Now().Add(5 * time.Second); !saw("validate _x/early *json.RawMessage"); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the hooks never saw the notification sent before initialize")
		}
	}

	negotiate(t, asc, allAdvertised())
	p.send(`{"jsonrpc":"2.0","id":2,"method":"_x/echo","params":{"v":1}}`)
	if a := p.next("the echo's answer"); string(a.ID) != "2" || a.Error != nil || string(a.Result) != `{"v":2}` {
		t.Errorf("the echo answered id %s, result %s, error %+v; want the rewritten {\"v\":2}", a.ID, a.Result, a.Error)
	}
	p.send(`{"jsonrpc":"2.0","id":3,"method":"_x/refuse","params":{}}`)
	if a := p.next("the refused request's answer"); string(a.ID) != "3" || a.Error == nil || a.Error.Code != -32602 {
		t.Errorf("a request the validation hook refused: id %s, %+v; want -32602", a.ID, a.Error)
	}
	p.send(`{"jsonrpc":"2.0","method":"_x/late","params":{}}`)

	// Notifications are handled in order, so the late one comes after any
	// early one that got through.
	got := []string{within(t, handled, "the echo's handler"), within(t, handled, "the late notification's handler")}
	if want := []string{`_x/echo {"v":2}`, `_x/late {}`}; !slices.Equal(got, want) {
		t.Errorf("the handler ran for %q, want %q", got, want)
	}
	for _, h := range []string{"rewrite", "validate"} {
		for _, m := range []string{"_x/echo", "_x/refuse", "_x/late"} {
			if want := h + " " + m + " *json.RawMessage"; !saw(want) {
				t.Errorf("the hooks never saw %q", want)
			}
		}
	}
}

// TestUndeclaredElicitationMode requires an elicitation in a mode the schema
// does not list to be refused both ways, with every elicitation capability
// advertised: sent, the typed method returns a *CapabilityError naming the
// mode and writes nothing; received, it is answered -32602 without reaching
// the handler, unless the validation hook accepts it
// (0019-PLAN-api-decisions.md, B20).
func TestUndeclaredElicitationMode(t *testing.T) {
	t.Parallel()
	req := decoded[CreateElicitationRequest](t, `{"mode":"telepathy","message":"m"}`)
	if req.Other == nil || req.Other.Mode != "telepathy" {
		t.Fatalf("decoded as %+v, want the catch-all", req)
	}

	w := &countingWriter{}
	asc, _ := sides(t, w, allAdvertised())
	// The peer never answers, so a send the gate let through ends at the
	// deadline instead of hanging the test.
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	_, err := asc.CreateElicitation(ctx, req)
	cancel()
	ce, ok := errors.AsType[*CapabilityError](err)
	if !ok || ce.Method != ClientMethodElicitationCreate || ce.Capability != "elicitation.telepathy" ||
		ce.Variant != "telepathy" || ce.Field != "" {
		t.Errorf("send = %v, want *CapabilityError{%s, elicitation.telepathy, Variant telepathy}", err, ClientMethodElicitationCreate)
	} else if want := "capability elicitation.telepathy not advertised: variant telepathy"; !strings.HasSuffix(err.Error(), want) {
		t.Errorf("send = %q, want it to end %q", err, want)
	}
	if got := w.count(); got != 0 {
		t.Errorf("%d bytes written, want none", got)
	}

	m := gatedMethod{wire: ClientMethodElicitationCreate, params: req}
	if rerr, ran := receive(t, m, allAdvertised()); rerr == nil || rerr.Code != -32602 || ran {
		t.Errorf("received: %v, handler ran %v, want -32602 and no handler", rerr, ran)
	}
	accept := WithInboundValidation(func(context.Context, string, any, error) error { return nil })
	if rerr, ran := receive(t, m, allAdvertised(), accept); rerr != nil || !ran {
		t.Errorf("received with a validation hook that accepts it: %v, handler ran %v, want it handled", rerr, ran)
	}
}

// isCapabilityError reports whether err is, or wraps, a *CapabilityError.
func isCapabilityError(err error) bool {
	_, ok := errors.AsType[*CapabilityError](err)
	return ok
}

// isValidationError reports whether err is, or wraps, a *ValidationError.
func isValidationError(err error) bool {
	_, ok := errors.AsType[*ValidationError](err)
	return ok
}

// TestTypedGate requires typedGate to pass its gate a result of type *T, and to
// refuse a result of another type with an error naming both types, without
// calling the gate (0019-PLAN-idioms-performance.md, D12).
func TestTypedGate(t *testing.T) {
	t.Parallel()
	refused := errors.New("refused")
	var got []*InitializeResponse
	gate := typedGate(func(method string, result *InitializeResponse) error {
		got = append(got, result)
		return refused
	})
	want := &InitializeResponse{ProtocolVersion: ProtocolVersionNumber}
	if err := gate(AgentMethodInitialize, want); !errors.Is(err, refused) || len(got) != 1 || got[0] != want {
		t.Errorf("gate(*InitializeResponse) = %v, gate given %v; want the gate's error, given only %p", err, got, want)
	}
	got = nil
	err := gate(AgentMethodInitialize, InitializeResponse{})
	if err == nil || err.Error() != "initialize: result of type acp.InitializeResponse, not *acp.InitializeResponse" {
		t.Errorf("gate(InitializeResponse) = %v, want the error naming both types", err)
	}
	if len(got) != 0 {
		t.Errorf("the gate was called %d times with a result of another type", len(got))
	}
}

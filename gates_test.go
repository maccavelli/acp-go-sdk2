package acp

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
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
		csc := NewClientSideConnection(recordingClient{calls: calls}, &countingWriter{}, idleReader(t), opts...)
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
	var ce *CapabilityError
	if !errors.As(err, &ce) || ce.Method != m.wire || ce.Capability != m.capability || ce.Field != "" || ce.Variant != "" {
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
			var ce *CapabilityError
			if err := within(t, sent, "the send"); errors.As(err, &ce) || (m.notifies && err != nil) {
				t.Errorf("send advertising only %s: %v", m.capability, err)
			}
			if rerr, ran := receive(t, m, only); rerr != nil || !ran {
				t.Errorf("receive advertising only %s: answered %v, handler ran %v, want it handled", m.capability, rerr, ran)
			}

			accept := WithInboundValidation(func(_ context.Context, _ string, _ any, err error) error {
				if errors.As(err, &ce) {
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
			var ve *ValidationError
			if errors.As(err, &ve) {
				return nil
			}
			return err
		}))
	negotiate(t, asc, versionOnly())
	_, rerr := asc.handle(t.Context(), AgentMethodSessionList, json.RawMessage(`{"cwd":"relative"}`))
	if rerr == nil || rerr.Code != -32601 {
		t.Errorf("handle = %v, want -32601", rerr)
	}
	var ve *ValidationError
	var ce *CapabilityError
	if len(seen) != 2 || !errors.As(seen[0], &ve) || !errors.As(seen[1], &ce) {
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
			err, _ := out[len(out)-1].Interface().(error)
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

package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

// initPair connects an agent whose Initialize is handle and a client over
// pipes, and returns both connections.
func initPair(t *testing.T, handle func(context.Context, InitializeRequest) (InitializeResponse, error)) (*AgentSideConnection, *ClientSideConnection) {
	t.Helper()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aW.Close()
		_ = a2cW.Close()
	})
	asc := NewAgentSideConnection(agentFuncs{InitializeFunc: handle}, a2cW, c2aR)
	return asc, NewClientSideConnection(&clientFuncs{}, c2aW, a2cR)
}

// negotiate stores n as what initialize agreed on side, an
// *AgentSideConnection or a *ClientSideConnection, for a test whose subject
// is not initialize (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 9).
func negotiate(t *testing.T, side any, n *Negotiation) {
	t.Helper()
	switch s := side.(type) {
	case *AgentSideConnection:
		s.negotiation.Store(n)
	case *ClientSideConnection:
		s.negotiation.Store(n)
	default:
		t.Fatalf("negotiate: %T is not a side connection", side)
	}
}

// allAdvertised returns a record in which each side advertises every
// capability the protocol table and its gates name.
func allAdvertised() *Negotiation {
	return &Negotiation{
		Request: InitializeRequest{ProtocolVersion: ProtocolVersionNumber, ClientCapabilities: ClientCapabilities{
			Auth:        AuthCapabilities{Terminal: true},
			Elicitation: &ElicitationCapabilities{Form: &ElicitationFormCapabilities{}, Url: &ElicitationUrlCapabilities{}},
			Fs:          FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true},
			Session: &ClientSessionCapabilities{
				Compaction:    &CompactionCapabilities{},
				ConfigOptions: &SessionConfigOptionsCapabilities{Boolean: &BooleanConfigOptionCapabilities{}},
				Notices:       &NoticeCapabilities{},
			},
			Terminal: true,
		}},
		Response: InitializeResponse{ProtocolVersion: ProtocolVersionNumber, AgentCapabilities: AgentCapabilities{
			Auth:               AgentAuthCapabilities{Logout: &LogoutCapabilities{}},
			LoadSession:        true,
			McpCapabilities:    McpCapabilities{Acp: true, Http: true, Sse: true},
			PromptCapabilities: PromptCapabilities{Audio: true, EmbeddedContext: true, Image: true},
			SessionCapabilities: SessionCapabilities{
				AdditionalDirectories: &SessionAdditionalDirectoriesCapabilities{},
				Close:                 &SessionCloseCapabilities{},
				Delete:                &SessionDeleteCapabilities{},
				Fork:                  &SessionForkCapabilities{},
				List:                  &SessionListCapabilities{},
				Resume:                &SessionResumeCapabilities{},
			},
		}},
	}
}

// encodesAlike reports whether a and b encode the same JSON value.
func encodesAlike(t *testing.T, a, b any) bool {
	t.Helper()
	x, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	y, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return sameJSON(t, x, y)
}

// TestNegotiationRecorded requires both sides to record the same request and
// response after initialize, nothing before it, and a later initialize to
// replace the record (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 8).
func TestNegotiationRecorded(t *testing.T) {
	t.Parallel()
	asc, c := initPair(t, func(_ context.Context, p InitializeRequest) (InitializeResponse, error) {
		return InitializeResponse{ProtocolVersion: ProtocolVersionNumber, AgentInfo: &Implementation{Name: p.ClientInfo.Name + "-agent"}}, nil
	})
	if _, ok := c.Negotiated(); ok {
		t.Error("the client has a record before initialize")
	}
	if _, ok := asc.Negotiated(); ok {
		t.Error("the agent has a record before initialize")
	}
	for _, name := range []string{"first", "second"} {
		resp, err := c.Initialize(t.Context(), InitializeRequest{ProtocolVersion: ProtocolVersionNumber, ClientInfo: &Implementation{Name: name}})
		if err != nil {
			t.Fatal(err)
		}
		cn, cok := c.Negotiated()
		an, aok := asc.Negotiated()
		if !cok || !aok || !encodesAlike(t, cn, an) || cn.Request.ClientInfo.Name != name || !encodesAlike(t, cn.Response, resp) {
			t.Errorf("%s: client %+v %v, agent %+v %v, want the same record of this initialize", name, cn, cok, an, aok)
		}
	}
}

// TestProtocolVersion requires the version rules of N1: a version left zero
// is the SDK's on both sides; an agent's handler answering another is
// answered -32603; a client does not send another; an agent answering another
// makes Initialize fail with a *ProtocolVersionError and records nothing
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 8).
func TestProtocolVersion(t *testing.T) {
	t.Parallel()
	t.Run("zero filled on both sides", func(t *testing.T) {
		t.Parallel()
		got := make(chan ProtocolVersion, 1)
		_, c := initPair(t, func(_ context.Context, p InitializeRequest) (InitializeResponse, error) {
			got <- p.ProtocolVersion
			return InitializeResponse{}, nil
		})
		resp, err := c.Initialize(t.Context(), InitializeRequest{})
		if err != nil || resp.ProtocolVersion != ProtocolVersionNumber {
			t.Errorf("Initialize = %+v, %v, want version %d", resp, err, ProtocolVersionNumber)
		}
		if v := within(t, got, "the request"); v != ProtocolVersionNumber {
			t.Errorf("the agent received version %d, want %d", v, ProtocolVersionNumber)
		}
	})
	t.Run("handler answering 2", func(t *testing.T) {
		t.Parallel()
		asc, c := initPair(t, func(context.Context, InitializeRequest) (InitializeResponse, error) {
			return InitializeResponse{ProtocolVersion: 2}, nil
		})
		_, err := c.Initialize(t.Context(), InitializeRequest{})
		var re *RequestError
		if !errors.As(err, &re) || re.Code != -32603 {
			t.Errorf("Initialize = %v, want -32603", err)
		}
		if _, ok := asc.Negotiated(); ok {
			t.Error("the agent recorded a refused answer")
		}
	})
	t.Run("client sending 2", func(t *testing.T) {
		t.Parallel()
		w := &countingWriter{}
		c := NewClientSideConnection(&clientFuncs{}, w, idleReader(t))
		_, err := c.Initialize(t.Context(), InitializeRequest{ProtocolVersion: 2})
		var pe *ProtocolVersionError
		if !errors.As(err, &pe) || pe.Requested != 2 || pe.Answered != 0 {
			t.Errorf("Initialize = %v, want *ProtocolVersionError{Requested: 2}", err)
		}
		w.mu.Lock()
		n := w.n
		w.mu.Unlock()
		if n != 0 {
			t.Errorf("%d bytes written, want none", n)
		}
	})
	t.Run("agent answering 2", func(t *testing.T) {
		t.Parallel()
		c := rawPeer(t, `{"protocolVersion":2,"agentCapabilities":{},"authMethods":[]}`)
		_, err := c.Initialize(t.Context(), InitializeRequest{})
		var pe *ProtocolVersionError
		if !errors.As(err, &pe) || pe.Requested != ProtocolVersionNumber || pe.Answered != 2 {
			t.Errorf("Initialize = %v, want *ProtocolVersionError{Requested: 1, Answered: 2}", err)
		}
		if _, ok := c.Negotiated(); ok {
			t.Error("the client recorded an answer it refused")
		}
	})
}

// TestNegotiatedIsACopy requires the record of initialize to share no memory
// with the values it was made from or handed out as: writing through the
// record Negotiated returns, through the request the agent's handler was
// given, or through the params a client passed changes no gate
// (0019-PLAN-behaviour-fixes.md, B13).
func TestNegotiatedIsACopy(t *testing.T) {
	t.Parallel()
	var kept *ElicitationCapabilities
	asc, csc := initPair(t, func(_ context.Context, p InitializeRequest) (InitializeResponse, error) {
		kept = p.ClientCapabilities.Elicitation
		return InitializeResponse{}, nil
	})
	params := InitializeRequest{ClientCapabilities: ClientCapabilities{
		Elicitation: &ElicitationCapabilities{Form: &ElicitationFormCapabilities{}},
	}}
	if _, err := csc.Initialize(t.Context(), params); err != nil {
		t.Fatal(err)
	}
	form := decoded[CreateElicitationRequest](t,
		`{"mode":"form","message":"m","requestedSchema":{"type":"object","properties":{}},"sessionId":"s"}`)
	gatesPass := func(when string) {
		t.Helper()
		if err := asc.gate(ClientMethodElicitationCreate, &form); err != nil {
			t.Errorf("agent, %s: %v", when, err)
		}
		if err := csc.gate(ClientMethodElicitationCreate, &form); err != nil {
			t.Errorf("client, %s: %v", when, err)
		}
	}
	gatesPass("before any write")

	for _, side := range []interface{ Negotiated() (Negotiation, bool) }{asc, csc} {
		rec, ok := side.Negotiated()
		if !ok || rec.Request.ClientCapabilities.Elicitation == nil || rec.Request.ClientCapabilities.Elicitation.Form == nil {
			t.Fatalf("%T.Negotiated() = %+v, %v", side, rec, ok)
		}
		rec.Request.ClientCapabilities.Elicitation.Form = nil
	}
	gatesPass("after writing through the records Negotiated returned")

	kept.Form = nil
	gatesPass("after the agent's handler wrote through its request")

	params.ClientCapabilities.Elicitation.Form = nil
	gatesPass("after the client's caller wrote through its params")
}

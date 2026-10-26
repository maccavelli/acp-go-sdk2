package acptest

import (
	"context"
	"fmt"
	"sync"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// Turn is one scripted prompt turn: the updates the agent sends, in order,
// and the stop reason it answers with.
type Turn struct {
	Updates    []acp.SessionUpdate
	StopReason acp.StopReason
}

// ScriptAgent is an acp.Agent that plays a script. Initialize answers
// acp.ProtocolVersionNumber with Capabilities; NewSession answers a new
// session id, "session-1", "session-2" and so on; each Prompt plays the
// next of Turns, on its session, and a Prompt past the last is answered with
// an internal error. Authenticate, Cancel, SetSessionMode and
// SetSessionConfigOption answer as a method that does nothing; the five
// methods acp.AgentRefusals covers answer method-not-found. Set the fields
// before connecting it; it receives its connection through
// acp.AgentConnAware (0039-MADR-five-adopter-additions-on-schema-1-25-0.md,
// item 5).
type ScriptAgent struct {
	acp.AgentRefusals

	Capabilities acp.AgentCapabilities
	Turns        []Turn

	mu       sync.Mutex
	conn     *acp.AgentSideConnection
	sessions int
	played   int
}

var (
	_ acp.Agent          = (*ScriptAgent)(nil)
	_ acp.AgentConnAware = (*ScriptAgent)(nil)
)

// SetAgentConnection implements acp.AgentConnAware.
func (a *ScriptAgent) SetAgentConnection(conn *acp.AgentSideConnection) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.conn = conn
}

// Initialize answers acp.ProtocolVersionNumber and a.Capabilities.
func (a *ScriptAgent) Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{ProtocolVersion: acp.ProtocolVersionNumber, AgentCapabilities: a.Capabilities}, nil
}

// Authenticate accepts every method.
func (a *ScriptAgent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

// NewSession answers the next session id.
func (a *ScriptAgent) NewSession(context.Context, acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions++
	return acp.NewSessionResponse{SessionId: acp.SessionId(fmt.Sprintf("session-%d", a.sessions))}, nil
}

// Prompt sends the next turn's updates on the prompt's session, then answers
// its stop reason.
func (a *ScriptAgent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	a.mu.Lock()
	conn, i := a.conn, a.played
	if i < len(a.Turns) {
		a.played++
	}
	a.mu.Unlock()
	if i >= len(a.Turns) {
		return acp.PromptResponse{}, acp.NewInternalError(fmt.Sprintf("acptest: the script has %d turns; this is prompt %d", len(a.Turns), i+1))
	}
	turn := a.Turns[i]
	for _, u := range turn.Updates {
		if err := conn.SessionUpdate(ctx, acp.SessionNotification{SessionId: params.SessionId, Update: u}); err != nil {
			return acp.PromptResponse{}, err
		}
	}
	return acp.PromptResponse{StopReason: turn.StopReason}, nil
}

// Cancel does nothing: a scripted turn is not cancellable.
func (a *ScriptAgent) Cancel(context.Context, acp.CancelNotification) error { return nil }

// SetSessionMode accepts every mode.
func (a *ScriptAgent) SetSessionMode(context.Context, acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

// SetSessionConfigOption accepts every option and reports none.
func (a *ScriptAgent) SetSessionConfigOption(context.Context, acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, nil
}

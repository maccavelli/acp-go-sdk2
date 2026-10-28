package acptest

import (
	"context"
	"fmt"
	"sync"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// Turn is one scripted prompt turn: the updates the agent sends, in order,
// and the stop reason it answers with
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 8, for Err, Usage
// and WaitCancel).
type Turn struct {
	Updates    []acp.SessionUpdate
	StopReason acp.StopReason

	// Err, when set, is the turn's answer after its updates: the prompt
	// fails with it as the connection answers a handler's error, an
	// *acp.RequestError as it is, and any other -32603, or -32800 when it
	// wraps context.Canceled.
	Err error

	// Usage is the token usage the turn's answer reports.
	Usage *acp.Usage //nolint:staticcheck // SA1019: the unstable schema's usage, which PromptResponse carries.

	// WaitCancel has the turn, after its updates, wait for the client's
	// session/cancel of the prompt's session, and then answer the stop
	// reason cancelled, as the specification requires of a cancelled turn;
	// or fail with the prompt's context's cause, when the connection ends
	// first. A cancel received for the session while no turn waits for one
	// counts for the next that does.
	WaitCancel bool
}

// ScriptAgent is an acp.Agent that plays a script. Initialize answers
// acp.ProtocolVersionNumber with Capabilities; NewSession answers a new
// session id, "session-1", "session-2" and so on; each Prompt plays the
// next of Turns, on its session, and a Prompt past the last is answered with
// an internal error. Cancel ends a turn that waits for it. Authenticate,
// SetSessionMode and SetSessionConfigOption answer as a method that does
// nothing; the five methods acp.AgentRefusals covers answer method-not-found.
// Set the fields before connecting it; it receives its connection through
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
	cancels  map[acp.SessionId]chan struct{} // a session's cancel, which holds one
}

// cancelled is the channel a session's cancel is sent on, holding one.
func (a *ScriptAgent) cancelled(id acp.SessionId) chan struct{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancels == nil {
		a.cancels = map[acp.SessionId]chan struct{}{}
	}
	ch, ok := a.cancels[id]
	if !ok {
		ch = make(chan struct{}, 1)
		a.cancels[id] = ch
	}
	return ch
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
// its stop reason, its error, or, for a turn that waits, cancelled once the
// session is cancelled.
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
	switch {
	case turn.Err != nil:
		return acp.PromptResponse{}, turn.Err
	case turn.WaitCancel:
		select {
		case <-a.cancelled(params.SessionId):
			return acp.PromptResponse{StopReason: acp.StopReasonCancelled, Usage: turn.Usage}, nil //nolint:staticcheck // SA1019: the turn's usage.
		case <-ctx.Done():
			return acp.PromptResponse{}, context.Cause(ctx)
		}
	}
	return acp.PromptResponse{StopReason: turn.StopReason, Usage: turn.Usage}, nil //nolint:staticcheck // SA1019: the turn's usage.
}

// Cancel ends the session's turn that waits for it, or the next that will.
func (a *ScriptAgent) Cancel(_ context.Context, params acp.CancelNotification) error {
	select {
	case a.cancelled(params.SessionId) <- struct{}{}:
	default:
	}
	return nil
}

// SetSessionMode accepts every mode.
func (a *ScriptAgent) SetSessionMode(context.Context, acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

// SetSessionConfigOption accepts every option and reports none.
func (a *ScriptAgent) SetSessionConfigOption(context.Context, acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, nil
}

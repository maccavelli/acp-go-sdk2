package acp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// demoAgent streams a message, announces a tool call, asks the client's
// permission for it and, when allowed, completes it. It keeps each session's
// working directory, so the paths it sends are absolute on the client's host.
type demoAgent struct {
	conn *acp.AgentSideConnection
	mu   sync.Mutex
	cwds map[acp.SessionId]string
}

var _ acp.Agent = (*demoAgent)(nil)

// SetAgentConnection is called by NewAgentSideConnection before it reads
// anything, so every handler can reach the connection.
func (a *demoAgent) SetAgentConnection(c *acp.AgentSideConnection) { a.conn = c }

func (a *demoAgent) Initialize(ctx context.Context, _ acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{ProtocolVersion: acp.ProtocolVersionNumber}, nil
}

func (a *demoAgent) NewSession(ctx context.Context, p acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	id := acp.SessionId("sess_demo")
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cwds == nil {
		a.cwds = map[acp.SessionId]string{}
	}
	a.cwds[id] = p.Cwd
	return acp.NewSessionResponse{SessionId: id}, nil
}

// Prompt runs under a context the SDK cancels for session/cancel; the SDK
// then answers the turn cancelled, whatever Prompt returns.
func (a *demoAgent) Prompt(ctx context.Context, p acp.PromptRequest) (acp.PromptResponse, error) {
	a.mu.Lock()
	config := filepath.Join(a.cwds[p.SessionId], "config.json")
	a.mu.Unlock()
	input, err := json.Marshal(map[string]string{"path": config})
	if err != nil {
		return acp.PromptResponse{}, err
	}

	if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
		SessionId: p.SessionId,
		Update:    acp.UpdateAgentMessageText("I'll update the configuration."),
	}); err != nil {
		return acp.PromptResponse{}, err
	}
	if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
		SessionId: p.SessionId,
		Update: acp.StartToolCall("call_1", "Modifying configuration",
			acp.WithStartKind(acp.ToolKindEdit),
			acp.WithStartStatus(acp.ToolCallStatusPending),
			acp.WithStartLocations([]acp.ToolCallLocation{{Path: config}}),
			acp.WithStartRawInput(input),
		),
	}); err != nil {
		return acp.PromptResponse{}, err
	}

	resp, err := a.conn.RequestPermission(ctx, acp.RequestPermissionRequest{
		SessionId: p.SessionId,
		ToolCall: acp.ToolCallUpdate{
			ToolCallId: "call_1",
			Title:      new("Modifying configuration"),
			Kind:       new(acp.ToolKindEdit),
			Status:     new(acp.ToolCallStatusPending),
			Locations:  []acp.ToolCallLocation{{Path: config}},
			RawInput:   input,
		},
		Options: []acp.PermissionOption{
			{Kind: acp.PermissionOptionKindAllowOnce, Name: "Allow", OptionId: "allow"},
			{Kind: acp.PermissionOptionKindRejectOnce, Name: "Reject", OptionId: "reject"},
		},
	})
	if err != nil {
		return acp.PromptResponse{}, err
	}
	if resp.Outcome.Selected == nil {
		return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
	}
	status := acp.ToolCallStatusFailed
	if resp.Outcome.Selected.OptionId == "allow" {
		status = acp.ToolCallStatusCompleted
	}
	if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
		SessionId: p.SessionId,
		Update:    acp.UpdateToolCall("call_1", acp.WithUpdateStatus(status)),
	}); err != nil {
		return acp.PromptResponse{}, err
	}
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

// Cancel needs no work: the SDK has already cancelled the turn's context.
func (a *demoAgent) Cancel(ctx context.Context, _ acp.CancelNotification) error { return nil }

func (a *demoAgent) Authenticate(ctx context.Context, _ acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

// The methods the agent does not offer answer method-not-found.

func (a *demoAgent) SetSessionMode(ctx context.Context, _ acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionSetMode)
}

func (a *demoAgent) ListSessions(ctx context.Context, _ acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	return acp.ListSessionsResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionList)
}

func (a *demoAgent) ResumeSession(ctx context.Context, _ acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	return acp.ResumeSessionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionResume)
}

func (a *demoAgent) DeleteSession(ctx context.Context, _ acp.DeleteSessionRequest) (acp.DeleteSessionResponse, error) {
	return acp.DeleteSessionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionDelete)
}

func (a *demoAgent) CloseSession(ctx context.Context, _ acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	return acp.CloseSessionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionClose)
}

func (a *demoAgent) SetSessionConfigOption(ctx context.Context, _ acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionSetConfigOption)
}

func (a *demoAgent) Logout(ctx context.Context, _ acp.LogoutRequest) (acp.LogoutResponse, error) {
	return acp.LogoutResponse{}, acp.NewMethodNotFound(acp.AgentMethodLogout)
}

// Example_agent serves an Agent and drives it with a client, in one process
// over two pipes. A real agent serves os.Stdin and os.Stdout instead:
//
//	asc := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
//	<-asc.Done()
func Example_agent() {
	ctx := context.Background()
	toAgent, fromClient := io.Pipe()
	toClient, fromAgent := io.Pipe()

	asc := acp.NewAgentSideConnection(&demoAgent{}, fromAgent, toAgent)
	csc := acp.NewClientSideConnection(&demoClient{}, fromClient, toClient)

	if _, err := csc.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		fmt.Println("initialize:", err)
		return
	}
	cwd, err := filepath.Abs(os.TempDir())
	if err != nil {
		fmt.Println("cwd:", err)
		return
	}
	sess, err := csc.NewSession(ctx, acp.NewSessionRequest{Cwd: cwd, McpServers: []acp.McpServer{}})
	if err != nil {
		fmt.Println("new session:", err)
		return
	}
	resp, err := csc.Prompt(ctx, acp.PromptRequest{SessionId: sess.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("Update the config")}})
	if err != nil {
		fmt.Println("prompt:", err)
		return
	}
	fmt.Println("stop reason:", resp.StopReason)

	// Closing the client's connection closes its pipes, which ends the agent's.
	if err := csc.Close(); err != nil {
		fmt.Println("close:", err)
	}
	<-asc.Done()
	// Output:
	// I'll update the configuration.
	// [tool] Modifying configuration (pending)
	// [tool] call_1: completed
	// stop reason: end_turn
}

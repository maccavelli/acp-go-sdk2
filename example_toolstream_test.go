package acp_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// streamingAgent runs one tool in its prompt turn and streams the tool's
// output to the client as it comes.
type streamingAgent struct {
	acp.AgentRefusals
	conn *acp.AgentSideConnection
}

func (a *streamingAgent) SetAgentConnection(c *acp.AgentSideConnection) { a.conn = c }

func (a *streamingAgent) Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{ProtocolVersion: acp.ProtocolVersionNumber}, nil
}

func (a *streamingAgent) Prompt(ctx context.Context, p acp.PromptRequest) (acp.PromptResponse, error) {
	// Started on Prompt's context, the stream belongs to this turn: should
	// Prompt return before Finish, its last state is still written before the
	// turn is answered.
	stream, err := acp.StartToolCallStream(ctx, a.conn, p.SessionId,
		acp.StartToolCall("call_1", "List files", acp.WithStartKind(acp.ToolKindExecute)))
	if err != nil {
		return acp.PromptResponse{}, err
	}
	if err := stream.SetStatus(acp.ToolCallStatusInProgress); err != nil {
		return acp.PromptResponse{}, err
	}
	// A command's output; with os/exec, cmd.Stdout = stream does the same.
	output := strings.NewReader("README.md\ngo.mod\n")
	status := acp.ToolCallStatusCompleted
	if _, err := io.Copy(stream, output); err != nil {
		status = acp.ToolCallStatusFailed
	}
	// Finish writes the final status with the whole output, also after
	// session/cancel has cancelled ctx.
	if err := stream.Finish(ctx, status); err != nil {
		return acp.PromptResponse{}, err
	}
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

func (a *streamingAgent) Cancel(context.Context, acp.CancelNotification) error { return nil }

func (a *streamingAgent) NewSession(context.Context, acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: "sess_1"}, nil
}

func (a *streamingAgent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (a *streamingAgent) SetSessionMode(context.Context, acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionSetMode)
}

func (a *streamingAgent) SetSessionConfigOption(context.Context, acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionSetConfigOption)
}

// toolViewer is a client that keeps the tool calls it is sent: each update
// is a whole state, which ToolCallTimeline applies.
type toolViewer struct {
	acp.ClientRefusals
	mu    sync.Mutex
	calls acp.ToolCallTimeline
}

func (c *toolViewer) SessionUpdate(_ context.Context, n acp.SessionNotification) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls.Apply(n.Update)
	return nil
}

func (c *toolViewer) RequestPermission(context.Context, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
}

// ExampleStartToolCallStream streams a tool's output from an agent to a
// client, which applies each whole state with a ToolCallTimeline.
func ExampleStartToolCallStream() {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aW.Close()
	defer a2cW.Close()
	acp.NewAgentSideConnection(&streamingAgent{}, a2cW, c2aR)
	viewer := &toolViewer{}
	client := acp.NewClientSideConnection(viewer, c2aW, a2cR)

	ctx := context.Background()
	if _, err := client.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		fmt.Println(err)
		return
	}
	if _, err := client.Prompt(ctx, acp.PromptRequest{SessionId: "sess_1", Prompt: []acp.ContentBlock{acp.TextBlock("list")}}); err != nil {
		fmt.Println(err)
		return
	}
	// Prompt returns once the client has handled every update the agent
	// sent before its answer.
	viewer.mu.Lock()
	call, _ := viewer.calls.Get("call_1")
	viewer.mu.Unlock()
	fmt.Println(call.Status)
	fmt.Print(call.Content[len(call.Content)-1].Content.Content.Text.Text)
	// Output:
	// completed
	// README.md
	// go.mod
}

package acp_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// demoClient prints the turn's text and tool calls and allows every
// permission request with its first option. It advertises no file system or
// terminal, so those methods answer method-not-found.
type demoClient struct{}

var _ acp.Client = (*demoClient)(nil)

func (demoClient) SessionUpdate(ctx context.Context, n acp.SessionNotification) error {
	switch u := n.Update; {
	case u.AgentMessageChunk != nil && u.AgentMessageChunk.Content.Text != nil:
		fmt.Println(u.AgentMessageChunk.Content.Text.Text)
	case u.ToolCall != nil:
		fmt.Printf("[tool] %s (%s)\n", u.ToolCall.Title, u.ToolCall.Status)
	case u.ToolCallUpdate != nil && u.ToolCallUpdate.Status != nil:
		fmt.Printf("[tool] %s: %s\n", u.ToolCallUpdate.ToolCallId, *u.ToolCallUpdate.Status)
	}
	return nil
}

// RequestPermission prints nothing: a request runs on its own goroutine, so
// its output could interleave with the notifications'.
func (demoClient) RequestPermission(ctx context.Context, p acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	if len(p.Options) == 0 {
		return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
	}
	return acp.RequestPermissionResponse{Outcome: acp.RequestPermissionOutcome{
		Selected: &acp.RequestPermissionOutcomeSelected{OptionId: p.Options[0].OptionId},
	}}, nil
}

func (demoClient) ReadTextFile(ctx context.Context, _ acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, acp.NewMethodNotFound(acp.ClientMethodFsReadTextFile)
}

func (demoClient) WriteTextFile(ctx context.Context, _ acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, acp.NewMethodNotFound(acp.ClientMethodFsWriteTextFile)
}

func (demoClient) CreateTerminal(ctx context.Context, _ acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalCreate)
}

func (demoClient) TerminalOutput(ctx context.Context, _ acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalOutput)
}

func (demoClient) ReleaseTerminal(ctx context.Context, _ acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalRelease)
}

func (demoClient) WaitForTerminalExit(ctx context.Context, _ acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalWaitForExit)
}

func (demoClient) KillTerminal(ctx context.Context, _ acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalKill)
}

func (demoClient) CreateElicitation(ctx context.Context, _ acp.CreateElicitationRequest) (acp.CreateElicitationResponse, error) {
	return acp.CreateElicitationResponse{}, acp.NewMethodNotFound(acp.ClientMethodElicitationCreate)
}

func (demoClient) CompleteElicitation(ctx context.Context, _ acp.CompleteElicitationNotification) error {
	return nil
}

// Example_client connects a Client to an agent, opens a session in a working
// directory, sends a prompt and prints the streamed turn and its stop reason,
// in one process over two pipes. A real client starts the agent and connects
// to its stdio instead:
//
//	cmd := exec.Command("my-agent")
//	stdin, _ := cmd.StdinPipe()
//	stdout, _ := cmd.StdoutPipe()
//	csc := acp.NewClientSideConnection(client, stdin, stdout)
func Example_client() {
	ctx := context.Background()
	toAgent, fromClient := io.Pipe()
	toClient, fromAgent := io.Pipe()

	asc := acp.NewAgentSideConnection(&demoAgent{}, fromAgent, toAgent)
	csc := acp.NewClientSideConnection(&demoClient{}, fromClient, toClient)

	ir, err := csc.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	if err != nil {
		fmt.Println("initialize:", err)
		return
	}
	fmt.Println("protocol version:", ir.ProtocolVersion)

	// The working directory must be absolute on the host.
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
	resp, err := csc.Prompt(ctx, acp.PromptRequest{SessionId: sess.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("Hello, agent!")}})
	if err != nil {
		fmt.Println("prompt:", err)
		return
	}
	fmt.Println("stop reason:", resp.StopReason)

	if err := csc.Close(); err != nil {
		fmt.Println("close:", err)
	}
	<-asc.Done()
	// Output:
	// protocol version: 1
	// I'll update the configuration.
	// [tool] Modifying configuration (pending)
	// [tool] call_1: completed
	// stop reason: end_turn
}

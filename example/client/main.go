// Command client is a minimal ACP client for trying the SDK: it starts an
// agent, opens a session in the current directory, sends one prompt, prints
// the streamed turn and its stop reason, and asks on stdin for each
// permission. With arguments, it starts that command as its agent; without,
// it runs the example agent from a source checkout with `go run`.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/maccavelli/acp-go-sdk2"
)

type exampleClient struct {
	// lines carries stdin, a line at a time; it is closed at the end of input.
	lines <-chan string
	// cwd is the working directory of the session this client opens; the
	// client writes files only inside it.
	cwd string
}

var _ acp.Client = (*exampleClient)(nil)

func (e *exampleClient) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	title := ""
	if params.ToolCall.Title != nil {
		title = *params.ToolCall.Title
	}
	fmt.Printf("\n🔐 Permission requested: %s\n", title)
	fmt.Println("\nOptions:")
	for i, opt := range params.Options {
		fmt.Printf("   %d. %s (%s)\n", i+1, opt.Name, opt.Kind)
	}
	cancelled := acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}
	for {
		fmt.Printf("\nChoose an option: ")
		var line string
		select {
		case <-ctx.Done():
			// The turn was cancelled; the SDK has answered for us.
			fmt.Println("\nThe permission request was cancelled.")
			return cancelled, nil
		case l, ok := <-e.lines:
			if !ok {
				fmt.Println("\nNo more input: the permission request is answered cancelled.")
				return cancelled, nil
			}
			line = strings.TrimSpace(l)
		}
		if line == "" {
			continue
		}
		if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(params.Options) {
			return acp.RequestPermissionResponse{Outcome: acp.RequestPermissionOutcome{Selected: &acp.RequestPermissionOutcomeSelected{OptionId: params.Options[n-1].OptionId}}}, nil
		}
		fmt.Println("Invalid option. Please try again.")
	}
}

func (e *exampleClient) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	u := params.Update
	switch {
	case u.AgentMessageChunk != nil:
		c := u.AgentMessageChunk.Content
		if c.Text != nil {
			fmt.Println(c.Text.Text)
		}
	case u.ToolCall != nil:
		fmt.Printf("\n🔧 %s (%s)\n", u.ToolCall.Title, u.ToolCall.Status)
	case u.ToolCallUpdate != nil:
		fmt.Printf("\n🔧 Tool call `%s` updated: %v\n\n", u.ToolCallUpdate.ToolCallId, u.ToolCallUpdate.Status)
	case u.Plan != nil || u.AgentThoughtChunk != nil || u.UserMessageChunk != nil:
		// Keep output compact for other updates
		fmt.Println("[", displayUpdateKind(u), "]")
	}
	return nil
}

func displayUpdateKind(u acp.SessionUpdate) string {
	switch {
	case u.UserMessageChunk != nil:
		return "user_message_chunk"
	case u.AgentMessageChunk != nil:
		return "agent_message_chunk"
	case u.AgentThoughtChunk != nil:
		return "agent_thought_chunk"
	case u.ToolCall != nil:
		return "tool_call"
	case u.ToolCallUpdate != nil:
		return "tool_call_update"
	case u.Plan != nil:
		return "plan"
	default:
		return "unknown"
	}
}

// WriteTextFile writes only inside the session's working directory, through
// an os.Root so that a symbolic link cannot lead out of it, and refuses any
// other path.
func (e *exampleClient) WriteTextFile(ctx context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	// The SDK has refused a path that is not absolute before this runs.
	rel, err := filepath.Rel(e.cwd, params.Path)
	if err != nil || !filepath.IsLocal(rel) {
		return acp.WriteTextFileResponse{}, acp.NewInvalidParams(map[string]string{
			"path":  params.Path,
			"error": "outside the session's working directory",
		})
	}
	if err := writeInside(e.cwd, rel, []byte(params.Content)); err != nil {
		return acp.WriteTextFileResponse{}, fmt.Errorf("write %s: %w", params.Path, err)
	}
	fmt.Printf("[Client] Wrote %d bytes to %s\n", len(params.Content), params.Path)
	return acp.WriteTextFileResponse{}, nil
}

// writeInside writes data to rel, a local path, inside dir, making its parent
// directories.
func writeInside(dir, rel string, data []byte) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	err = root.MkdirAll(filepath.Dir(rel), 0o750)
	if err == nil {
		err = root.WriteFile(rel, data, 0o600)
	}
	return errors.Join(err, root.Close())
}

func (e *exampleClient) ReadTextFile(ctx context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	// The SDK has refused a path that is not absolute before this runs.
	b, err := os.ReadFile(params.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, fmt.Errorf("read %s: %w", params.Path, err)
	}
	content := string(b)
	// Apply optional line/limit (1-based line index)
	if params.Line != nil || params.Limit != nil {
		lines := strings.Split(content, "\n")
		if params.Line != nil && *params.Line > 1 {
			lines = lines[min(uint64(*params.Line)-1, uint64(len(lines))):]
		}
		if params.Limit != nil && *params.Limit > 0 && uint64(*params.Limit) < uint64(len(lines)) {
			lines = lines[:*params.Limit]
		}
		content = strings.Join(lines, "\n")
	}
	fmt.Printf("[Client] ReadTextFile: %s (%d bytes)\n", params.Path, len(content))
	return acp.ReadTextFileResponse{Content: content}, nil
}

// The client does not advertise the terminal capability, so the terminal
// methods answer method-not-found.

func (e *exampleClient) CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalCreate)
}

func (e *exampleClient) TerminalOutput(ctx context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalOutput)
}

func (e *exampleClient) ReleaseTerminal(ctx context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalRelease)
}

func (e *exampleClient) WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalWaitForExit)
}

// KillTerminal implements acp.Client.
func (e *exampleClient) KillTerminal(ctx context.Context, params acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalKill)
}

// CompleteElicitation implements acp.Client.
func (e *exampleClient) CompleteElicitation(ctx context.Context, params acp.CompleteElicitationNotification) error {
	fmt.Printf("[Client] CompleteElicitation: %v\n", params)
	return nil
}

// CreateElicitation implements acp.Client.
func (e *exampleClient) CreateElicitation(ctx context.Context, params acp.CreateElicitationRequest) (acp.CreateElicitationResponse, error) {
	fmt.Printf("[Client] CreateElicitation: %v\n", params)
	return acp.CreateElicitationResponse{}, nil
}

func main() {
	// If args provided, treat them as agent program + args. Otherwise run the Go agent example.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cmd *exec.Cmd
	if len(os.Args) > 1 {
		cmd = exec.CommandContext(ctx, os.Args[1], os.Args[2:]...) //nolint:gosec // G204: the agent command the user names is what this mode runs.
	} else {
		// Default: run the Go example agent. Detect relative to this client's location.
		_, filename, _, ok := runtime.Caller(0)
		if !ok {
			fmt.Fprintf(os.Stderr, "failed to determine current file location\n")
			os.Exit(1)
		}

		// Get directory of this client file and find sibling agent directory
		clientDir := filepath.Dir(filename)
		agentPath := filepath.Join(clientDir, "..", "agent")

		if _, err := os.Stat(agentPath); err != nil {
			fmt.Fprintf(os.Stderr, "failed to find agent directory at %s: %v\n", agentPath, err)
			os.Exit(1)
		}

		cmd = exec.CommandContext(ctx, "go", "run", agentPath) //nolint:gosec // G204: the example agent's directory in this source checkout.
	}
	cmd.Stderr = os.Stderr
	// Set up pipes for stdio
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stdin pipe error: %v\n", err)
		os.Exit(1)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stdout pipe error: %v\n", err)
		os.Exit(1)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start agent: %v\n", err)
		os.Exit(1)
	}

	client := &exampleClient{lines: readLines(os.Stdin), cwd: mustCwd()}
	conn := acp.NewClientSideConnection(client, stdin, stdout)
	conn.SetLogger(slog.Default())

	// Initialize
	initResp, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true},
		},
	})
	if err != nil {
		printRequestError("initialize", err)
		stopAgent(cmd, stdin, cancel)
		os.Exit(1)
	}
	fmt.Printf("✅ Connected to agent (protocol v%v)\n", initResp.ProtocolVersion)

	// New session
	newSess, err := conn.NewSession(ctx, acp.NewSessionRequest{Cwd: client.cwd, McpServers: []acp.McpServer{}})
	if err != nil {
		printRequestError("newSession", err)
		stopAgent(cmd, stdin, cancel)
		os.Exit(1)
	}
	fmt.Printf("📝 Created session: %s\n", newSess.SessionId)
	fmt.Printf("💬 User: Hello, agent!\n\n")
	fmt.Print(" ")

	// Send prompt
	resp, err := conn.Prompt(ctx, acp.PromptRequest{
		SessionId: newSess.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock("Hello, agent!")},
	})
	if err != nil {
		printRequestError("prompt", err)
		stopAgent(cmd, stdin, cancel)
		os.Exit(1)
	}
	fmt.Printf("\n\n✅ Turn ended: stop reason %s\n", resp.StopReason)
	stopAgent(cmd, stdin, cancel)
}

// printRequestError surfaces a failed call, with a JSON-RPC error's detail.
func printRequestError(op string, err error) {
	re, ok := errors.AsType[*acp.RequestError](err)
	if !ok {
		fmt.Fprintf(os.Stderr, "%s error: %v\n", op, err)
		return
	}
	if b, mErr := json.MarshalIndent(re, "", "  "); mErr == nil {
		fmt.Fprintf(os.Stderr, "[Client] Error: %s\n", string(b))
	} else {
		fmt.Fprintf(os.Stderr, "%s error (%d): %s\n", op, re.Code, re.Message)
	}
}

// stopAgent closes the agent's input, which ends an ACP agent, and waits for
// it to exit; after five seconds it cancels the agent's context, which kills
// it.
func stopAgent(cmd *exec.Cmd, stdin io.Closer, cancel context.CancelFunc) {
	if err := stdin.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "close agent input: %v\n", err)
	}
	t := time.AfterFunc(5*time.Second, cancel)
	defer t.Stop()
	if err := cmd.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "agent exited: %v\n", err)
	}
}

// readLines reads r on its own goroutine and sends each line on the channel it
// returns, which it closes at the end of input. The whole program reads stdin
// through it, so no two readers split its lines (0019-PLAN-behaviour-fixes.md,
// E1 and E2).
func readLines(r io.Reader) <-chan string {
	lines := make(chan string)
	go func() {
		defer close(lines)
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	return lines
}

func mustCwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

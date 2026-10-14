package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/maccavelli/acp-go-sdk2"
)

// ClaudeCodeREPL demonstrates connecting to the Claude Code CLI running in ACP mode
// and providing a simple REPL to send prompts and print streamed updates.

type replClient struct {
	autoApprove bool
	// answers carries the lines typed while a turn runs, for its permission
	// prompt; the REPL loop owns stdin (0019-PLAN-behaviour-fixes.md, E2).
	answers chan string
}

// drainAnswers drops lines typed during a turn that no prompt took.
func (c *replClient) drainAnswers() {
	for {
		select {
		case <-c.answers:
		default:
			return
		}
	}
}

var _ acp.Client = (*replClient)(nil)

func (c *replClient) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	if c.autoApprove {
		// Prefer an allow option if present; otherwise choose the first option.
		for _, o := range params.Options {
			if o.Kind == acp.PermissionOptionKindAllowOnce || o.Kind == acp.PermissionOptionKindAllowAlways {
				return acp.RequestPermissionResponse{Outcome: acp.RequestPermissionOutcome{Selected: &acp.RequestPermissionOutcomeSelected{OptionId: o.OptionId}}}, nil
			}
		}
		if len(params.Options) > 0 {
			return acp.RequestPermissionResponse{Outcome: acp.RequestPermissionOutcome{Selected: &acp.RequestPermissionOutcomeSelected{OptionId: params.Options[0].OptionId}}}, nil
		}
		return acp.RequestPermissionResponse{Outcome: acp.RequestPermissionOutcome{Cancelled: &acp.RequestPermissionOutcomeCancelled{}}}, nil
	}

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
		case l, ok := <-c.answers:
			if !ok {
				fmt.Println("\nNo more input: the permission request is answered cancelled.")
				return cancelled, nil
			}
			line = strings.TrimSpace(l)
		}
		if line == "" {
			continue
		}
		idx := -1
		_, _ = fmt.Sscanf(line, "%d", &idx)
		idx = idx - 1
		if idx >= 0 && idx < len(params.Options) {
			return acp.RequestPermissionResponse{Outcome: acp.RequestPermissionOutcome{Selected: &acp.RequestPermissionOutcomeSelected{OptionId: params.Options[idx].OptionId}}}, nil
		}
		fmt.Println("Invalid option. Please try again.")
	}
}

func (c *replClient) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	u := params.Update
	switch {
	case u.AgentMessageChunk != nil:
		content := u.AgentMessageChunk.Content
		if content.Text != nil {
			fmt.Printf("[agent] \n%s\n", content.Text.Text)
		}
	case u.ToolCall != nil:
		fmt.Printf("\n🔧 %s (%s)\n", u.ToolCall.Title, u.ToolCall.Status)
	case u.ToolCallUpdate != nil:
		fmt.Printf("\n🔧 Tool call `%s` updated: %v\n\n", u.ToolCallUpdate.ToolCallId, u.ToolCallUpdate.Status)
	case u.Plan != nil:
		fmt.Println("[plan update]")
	case u.AgentThoughtChunk != nil:
		thought := u.AgentThoughtChunk.Content
		if thought.Text != nil {
			fmt.Printf("[agent_thought_chunk] \n%s\n", thought.Text.Text)
		}
	case u.UserMessageChunk != nil:
		fmt.Println("[user_message_chunk]")
	}
	return nil
}

func (c *replClient) WriteTextFile(ctx context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	// The SDK has refused a path that is not absolute before this runs.
	dir := filepath.Dir(params.Path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return acp.WriteTextFileResponse{}, fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(params.Path, []byte(params.Content), 0o644); err != nil {
		return acp.WriteTextFileResponse{}, fmt.Errorf("write %s: %w", params.Path, err)
	}
	fmt.Printf("[Client] Wrote %d bytes to %s\n", len(params.Content), params.Path)
	return acp.WriteTextFileResponse{}, nil
}

func (c *replClient) ReadTextFile(ctx context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	// The SDK has refused a path that is not absolute before this runs.
	b, err := os.ReadFile(params.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, fmt.Errorf("read %s: %w", params.Path, err)
	}
	content := string(b)
	if params.Line != nil || params.Limit != nil {
		lines := strings.Split(content, "\n")
		start := 0
		if params.Line != nil && *params.Line > 0 {
			start = int(min(uint64(*params.Line-1), uint64(len(lines))))
		}
		end := len(lines)
		if params.Limit != nil && *params.Limit > 0 && uint64(*params.Limit) < uint64(end-start) {
			end = start + int(*params.Limit)
		}
		content = strings.Join(lines[start:end], "\n")
	}
	fmt.Printf("[Client] ReadTextFile: %s (%d bytes)\n", params.Path, len(content))
	return acp.ReadTextFileResponse{Content: content}, nil
}

// Optional/UNSTABLE terminal methods: implement as no-ops for example
func (c *replClient) CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	fmt.Printf("[Client] CreateTerminal: %v\n", params)
	return acp.CreateTerminalResponse{TerminalId: "term-1"}, nil
}

func (c *replClient) TerminalOutput(ctx context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	fmt.Printf("[Client] TerminalOutput: %v\n", params)
	return acp.TerminalOutputResponse{Output: "", Truncated: false}, nil
}

func (c *replClient) ReleaseTerminal(ctx context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	fmt.Printf("[Client] ReleaseTerminal: %v\n", params)
	return acp.ReleaseTerminalResponse{}, nil
}

func (c *replClient) WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	fmt.Printf("[Client] WaitForTerminalExit: %v\n", params)
	return acp.WaitForTerminalExitResponse{}, nil
}

// CompleteElicitation implements acp.Client.
func (c *replClient) CompleteElicitation(ctx context.Context, params acp.CompleteElicitationNotification) error {
	fmt.Printf("[Client] CompleteElicitation: %v\n", params)
	return nil
}

// CreateElicitation implements acp.Client.
func (c *replClient) CreateElicitation(ctx context.Context, params acp.CreateElicitationRequest) (acp.CreateElicitationResponse, error) {
	fmt.Printf("[Client] CreateElicitation: %v\n", params)
	return acp.CreateElicitationResponse{}, nil
}

// KillTerminal implements acp.Client.
func (c *replClient) KillTerminal(ctx context.Context, params acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	fmt.Printf("[Client] KillTerminal: %v\n", params)
	return acp.KillTerminalResponse{}, nil
}

func main() {
	yolo := flag.Bool("yolo", false, "Auto-approve permission prompts")
	flag.Parse()

	// Invoke Claude Code via npx
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "npx", "-y", "@zed-industries/claude-code-acp@latest")
	cmd.Stderr = os.Stderr
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
		fmt.Fprintf(os.Stderr, "failed to start Claude Code: %v\n", err)
		os.Exit(1)
	}

	client := &replClient{autoApprove: *yolo, answers: make(chan string, 16)}
	conn := acp.NewClientSideConnection(client, stdin, stdout)
	conn.SetLogger(slog.Default())

	// Initialize
	initResp, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{Fs: acp.FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true}},
	})
	if err != nil {
		if re, ok := err.(*acp.RequestError); ok {
			if b, mErr := json.MarshalIndent(re, "", "  "); mErr == nil {
				fmt.Fprintf(os.Stderr, "[Client] Error: %s\n", string(b))
			} else {
				fmt.Fprintf(os.Stderr, "initialize error (%d): %s\n", re.Code, re.Message)
			}
		} else {
			fmt.Fprintf(os.Stderr, "initialize error: %v\n", err)
		}
		_ = cmd.Process.Kill()
		os.Exit(1)
	}
	fmt.Printf("✅ Connected to Claude Code (protocol v%v)\n", initResp.ProtocolVersion)

	// New session
	newSess, err := conn.NewSession(ctx, acp.NewSessionRequest{Cwd: mustCwd(), McpServers: []acp.McpServer{}})
	if err != nil {
		if re, ok := err.(*acp.RequestError); ok {
			if b, mErr := json.MarshalIndent(re, "", "  "); mErr == nil {
				fmt.Fprintf(os.Stderr, "[Client] Error: %s\n", string(b))
			} else {
				fmt.Fprintf(os.Stderr, "newSession error (%d): %s\n", re.Code, re.Message)
			}
		} else {
			fmt.Fprintf(os.Stderr, "newSession error: %v\n", err)
		}
		_ = cmd.Process.Kill()
		os.Exit(1)
	}
	fmt.Printf("📝 Created session: %s\n", newSess.SessionId)

	fmt.Println("Type a message and press Enter to send. Commands: :cancel, :exit")
	lines := readLines(os.Stdin)
	for {
		fmt.Print("> ")
		line, ok := <-lines
		if !ok {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch line {
		case ":exit", ":quit":
			cancel()
			return
		case ":cancel":
			fmt.Println("No turn is running.")
			continue
		}
		if !runTurn(ctx, conn, client, lines, newSess.SessionId, line) {
			break
		}
	}

	_ = cmd.Process.Kill()
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

// runTurn sends one prompt and reads stdin until the turn ends: ":cancel"
// cancels the turn's context, which sends session/cancel; ":exit" does the
// same and ends the REPL, as the end of input does; any other line goes to the
// turn's permission prompt. It returns false when the REPL should end
// (0019-PLAN-behaviour-fixes.md, E2).
func runTurn(ctx context.Context, conn *acp.ClientSideConnection, client *replClient, lines <-chan string, sid acp.SessionId, text string) bool {
	turnCtx, cancelTurn := context.WithCancel(ctx)
	defer cancelTurn()
	done := make(chan error, 1)
	go func() {
		_, err := conn.Prompt(turnCtx, acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock(text)}})
		done <- err
	}()
	keepGoing := true
	for {
		select {
		case err := <-done:
			switch {
			case turnCtx.Err() != nil:
				fmt.Println("\n[turn cancelled]")
			case err != nil:
				printPromptError(err)
			}
			client.drainAnswers()
			return keepGoing
		case line, ok := <-lines:
			switch l := strings.TrimSpace(line); {
			case !ok:
				lines, keepGoing = nil, false
				cancelTurn()
			case l == ":cancel":
				cancelTurn()
			case l == ":exit" || l == ":quit":
				keepGoing = false
				cancelTurn()
			case l != "":
				select {
				case client.answers <- l:
				default:
					fmt.Println("(input ignored while a turn runs; type :cancel to cancel it)")
				}
			}
		}
	}
}

// printPromptError surfaces a prompt's error, with a JSON-RPC error's detail.
func printPromptError(err error) {
	if re, ok := err.(*acp.RequestError); ok {
		if b, mErr := json.MarshalIndent(re, "", "  "); mErr == nil {
			fmt.Fprintf(os.Stderr, "[Client] Error: %s\n", string(b))
		} else {
			fmt.Fprintf(os.Stderr, "prompt error (%d): %s\n", re.Code, re.Message)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "prompt error: %v\n", err)
}

func mustCwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

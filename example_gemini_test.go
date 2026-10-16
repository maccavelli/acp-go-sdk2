package acp_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// Example_gemini starts the Gemini CLI in ACP mode, `gemini --acp`, and
// connects to it over its stdio as the client: it initializes, opens a
// session in the current directory, sends a prompt and prints the streamed
// turn. It needs the gemini binary, so it has no checked output and does not
// run under go test.
func Example_gemini() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "gemini", "--acp")
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Println("stdin:", err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("stdout:", err)
		return
	}
	if err := cmd.Start(); err != nil {
		fmt.Println("start gemini:", err)
		return
	}
	// Closing gemini's input ends it; waiting reaps it.
	defer func() {
		if err := stdin.Close(); err != nil {
			fmt.Println("close:", err)
		}
		if err := cmd.Wait(); err != nil {
			fmt.Println("gemini:", err)
		}
	}()

	csc := acp.NewClientSideConnection(&demoClient{}, stdin, stdout)
	if _, err := csc.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		fmt.Println("initialize:", err)
		return
	}
	cwd, err := filepath.Abs(".")
	if err != nil {
		fmt.Println("cwd:", err)
		return
	}
	sess, err := csc.NewSession(ctx, acp.NewSessionRequest{Cwd: cwd, McpServers: []acp.McpServer{}})
	if err != nil {
		fmt.Println("new session:", err)
		return
	}
	resp, err := csc.Prompt(ctx, acp.PromptRequest{SessionId: sess.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("List the files here.")}})
	if err != nil {
		fmt.Println("prompt:", err)
		return
	}
	fmt.Println("stop reason:", resp.StopReason)
}

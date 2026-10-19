package acp_test

import (
	"context"
	"fmt"
	"io"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// ExampleNewClientSideConnection connects a client and an agent in one
// process over two pipes, initializes, and closes both. demoAgent and
// demoClient implement Agent and Client (example_agent_test.go and
// example_client_test.go). The README's quickstart is this function's body
// (0024-PLAN-documentation.md, A8).
func ExampleNewClientSideConnection() {
	// Each side writes to one pipe and reads the other.
	toAgent, fromClient := io.Pipe()
	toClient, fromAgent := io.Pipe()
	agent := acp.NewAgentSideConnection(&demoAgent{}, fromAgent, toAgent)
	client := acp.NewClientSideConnection(&demoClient{}, fromClient, toClient)

	resp, err := client.Initialize(context.Background(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	if err != nil {
		fmt.Println("initialize:", err)
		return
	}
	fmt.Println("protocol version:", resp.ProtocolVersion)

	// Closing the client ends the agent's input, so the agent ends too.
	if err := client.Close(); err != nil {
		fmt.Println("close:", err)
	}
	<-agent.Done()
	if err := agent.Close(); err != nil {
		fmt.Println("close:", err)
	}
	// Output:
	// protocol version: 1
}

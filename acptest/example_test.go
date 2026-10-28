package acptest_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// In a test, Connect joins a scripted agent and an in-memory client; the test
// drives the client side, and the pair is closed and waited for when the test
// ends. An example has no *testing.T of its own, so this one is not run;
// TestConnectRoundTrip runs the same calls.
func ExampleConnect() {
	exampleTest := func(t *testing.T) {
		t.Helper()
		agent := &acptest.ScriptAgent{Turns: []acptest.Turn{{
			Updates:    []acp.SessionUpdate{acp.UpdateAgentMessageText("hello")},
			StopReason: acp.StopReasonEndTurn,
		}}}
		client := &acptest.MemoryClient{}
		pair := acptest.Connect(t, agent, client)
		ctx := t.Context()
		if _, err := pair.Client.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
			t.Fatal(err)
		}
		sess, err := pair.Client.NewSession(ctx, acp.NewSessionRequest{Cwd: acptest.Abs("/work")})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pair.Client.Prompt(ctx, acp.PromptRequest{SessionId: sess.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("hi")}}); err != nil {
			t.Fatal(err)
		}
		if n := len(client.Updates()); n != 1 {
			t.Errorf("%d updates, want 1", n)
		}
	}
	_ = exampleTest
}

// A Recorder wraps what each side writes, and Frames returns the frames in
// canonical form, for a golden.
func ExampleRecorder() {
	var rec acptest.Recorder
	agentReads, clientWrites := io.Pipe()
	clientReads, agentWrites := io.Pipe()
	pair := acptest.Pair{
		Agent:  acp.NewAgentSideConnection(&acptest.ScriptAgent{}, rec.Writer(agentWrites), agentReads),
		Client: acp.NewClientSideConnection(&acptest.MemoryClient{}, rec.Writer(clientWrites), clientReads),
	}
	if _, err := pair.Client.Initialize(context.Background(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		fmt.Println(err)
	}
	if err := pair.Close(); err != nil {
		fmt.Println(err)
	}
	fmt.Println("closed:", errors.Is(pair.Wait(), acp.ErrConnectionClosed))
	for _, f := range rec.Frames() {
		fmt.Println(string(f))
	}
	// Output:
	// closed: true
	// {"id":1,"jsonrpc":"2.0","method":"initialize","params":{"clientCapabilities":{"auth":{},"fs":{}},"protocolVersion":1}}
	// {"id":1,"jsonrpc":"2.0","result":{"agentCapabilities":{"auth":{},"mcpCapabilities":{},"promptCapabilities":{},"sessionCapabilities":{}},"authMethods":[],"protocolVersion":1}}
}

// A ReplayAgent plays a recorded session's agent side to a client: here a
// recorded initialize, answered under the client's own id, 7 in the
// recording. Serve returns nil when the client repeated the recording. In a
// test, ConnectReplay does the wiring, and fails the test on any departure.
func ExampleReplayAgent() {
	records := []acptest.Record{
		{Direction: acptest.Sent, Frame: json.RawMessage(`{"jsonrpc":"2.0","id":7,"method":"initialize","params":{"protocolVersion":1,"clientCapabilities":{}}}`)},
		{Direction: acptest.Received, Frame: json.RawMessage(`{"jsonrpc":"2.0","id":7,"result":{"protocolVersion":1,"agentInfo":{"name":"recorded-agent","version":"1.0"}}}`)},
	}
	agentReads, clientWrites := io.Pipe()
	clientReads, agentWrites := io.Pipe()
	served := make(chan error, 1)
	go func() {
		err := (&acptest.ReplayAgent{Records: records}).Serve(context.Background(), agentReads, agentWrites)
		_ = agentWrites.Close()
		served <- err
	}()
	client := acp.NewClientSideConnection(&acptest.MemoryClient{}, clientWrites, clientReads)
	resp, err := client.Initialize(context.Background(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("the agent:", resp.AgentInfo.Name)
	_ = client.Close()
	fmt.Println("replay:", <-served)
	// Output:
	// the agent: recorded-agent
	// replay: <nil>
}

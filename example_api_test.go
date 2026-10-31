package acp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// ExampleStartToolCall builds a tool_call update with two of its options and
// prints it as it is sent (0024-PLAN-documentation.md, A8).
func ExampleStartToolCall() {
	update := acp.StartToolCall("call_1", "Read notes.txt",
		acp.WithStartKind(acp.ToolKindRead),
		acp.WithStartStatus(acp.ToolCallStatusPending))
	b, err := json.Marshal(update)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(b))
	// Output:
	// {"kind":"read","sessionUpdate":"tool_call","status":"pending","title":"Read notes.txt","toolCallId":"call_1"}
}

// ExampleUpdateSessionTitle sets a session's title and clears it, and prints
// each update as it is sent
// (0036-PLAN-session-update-constructors-kind-and-client-refusals.md, step 2).
func ExampleUpdateSessionTitle() {
	for _, title := range []acp.Patch[string]{acp.PatchValue("Refactor"), acp.PatchNull[string]()} {
		b, err := json.Marshal(acp.UpdateSessionTitle(title))
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(string(b))
	}
	// Output:
	// {"sessionUpdate":"session_info_update","title":"Refactor"}
	// {"sessionUpdate":"session_info_update","title":null}
}

// ExampleSessionUpdate_Kind decodes an update this SDK knows and one it does
// not, and prints the kind of each
// (0036-PLAN-session-update-constructors-kind-and-client-refusals.md, step 3).
func ExampleSessionUpdate_Kind() {
	for _, payload := range []string{
		`{"sessionUpdate":"current_mode_update","currentModeId":"code"}`,
		`{"sessionUpdate":"future_update","detail":1}`,
	} {
		var u acp.SessionUpdate
		if err := json.Unmarshal([]byte(payload), &u); err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(u.Kind())
	}
	// Output:
	// current_mode_update
	// future_update
}

// refusingClient offers no file-system, terminal or elicitation methods: it
// embeds ClientRefusals for them and implements the two it must.
type refusingClient struct{ acp.ClientRefusals }

func (refusingClient) SessionUpdate(context.Context, acp.SessionNotification) error { return nil }

func (refusingClient) RequestPermission(context.Context, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
}

// pagingAgent is demoAgent with session/list advertised and answered in two
// pages, the first naming the second by its cursor. Each session's cwd is
// os.TempDir(), absolute on every host, since the client checks it with
// filepath.IsAbs.
type pagingAgent struct{ *demoAgent }

func (pagingAgent) Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion:   acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{SessionCapabilities: acp.SessionCapabilities{List: &acp.SessionListCapabilities{}}},
	}, nil
}

func (pagingAgent) ListSessions(_ context.Context, req acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	if req.Cursor == nil {
		next := "page-2"
		return acp.ListSessionsResponse{Sessions: []acp.SessionInfo{{SessionId: "s1", Cwd: os.TempDir()}}, NextCursor: &next}, nil
	}
	return acp.ListSessionsResponse{Sessions: []acp.SessionInfo{{SessionId: "s2", Cwd: os.TempDir()}}}, nil
}

// ExampleAllSessions lists every session an agent has across its pages
// (0038-PLAN-consumer-helpers-kind-and-agent-refusals.md, step 2).
func ExampleAllSessions() {
	toAgent, fromClient := io.Pipe()
	toClient, fromAgent := io.Pipe()
	agent := acp.NewAgentSideConnection(pagingAgent{&demoAgent{}}, fromAgent, toAgent)
	client := acp.NewClientSideConnection(refusingClient{}, fromClient, toClient)
	defer func() { _ = client.Close(); _ = agent.Close() }()

	ctx := context.Background()
	if _, err := client.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		fmt.Println(err)
		return
	}
	for s, err := range acp.AllSessions(ctx, client, acp.ListSessionsRequest{}) {
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(s.SessionId)
	}
	// Output:
	// s1
	// s2
}

// ExamplePickPermissionOption declines a permission request by kind, never
// by an option's name (0038-PLAN-consumer-helpers-kind-and-agent-refusals.md,
// step 2).
func ExamplePickPermissionOption() {
	opts := []acp.PermissionOption{
		{OptionId: "allow", Name: "Allow", Kind: acp.PermissionOptionKindAllowOnce},
		{OptionId: "deny", Name: "Don't allow", Kind: acp.PermissionOptionKindRejectOnce},
	}
	if o, ok := acp.PickPermissionOption(opts, acp.PermissionOptionKindRejectOnce, acp.PermissionOptionKindRejectAlways); ok {
		fmt.Println(o.OptionId)
	}
	// Output:
	// deny
}

// ExampleResolveStdioCommands resolves a client's bare MCP server command
// before the SDK checks that it is absolute. An agent installs the hook with
// WithInboundRewrite(acp.ResolveStdioCommands(exec.LookPath)); here a lookup
// in the temporary directory stands in for exec.LookPath, so the output is
// the same on every host (0038-PLAN-consumer-helpers-kind-and-agent-refusals.md,
// step 4).
func ExampleResolveStdioCommands() {
	lookPath := func(name string) (string, error) { return filepath.Join(os.TempDir(), "bin", name), nil }
	hook := acp.ResolveStdioCommands(lookPath)
	req := acp.NewSessionRequest{Cwd: os.TempDir(), McpServers: []acp.McpServer{acp.NewMcpServerStdio("tools", "npx", nil, nil)}}
	if err := hook(context.Background(), acp.AgentMethodSessionNew, &req); err != nil {
		fmt.Println(err)
		return
	}
	command := req.McpServers[0].Stdio.Command
	fmt.Println(filepath.IsAbs(command), filepath.Base(command))
	// Output:
	// true npx
}

// promptOnlyAgent offers no session listing, resuming, closing, deleting or
// logout: it embeds AgentRefusals for them and implements the seven it must.
type promptOnlyAgent struct{ acp.AgentRefusals }

func (promptOnlyAgent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (promptOnlyAgent) Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{ProtocolVersion: acp.ProtocolVersionNumber}, nil
}

func (promptOnlyAgent) Cancel(context.Context, acp.CancelNotification) error { return nil }

func (promptOnlyAgent) NewSession(context.Context, acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: "s"}, nil
}

func (promptOnlyAgent) Prompt(context.Context, acp.PromptRequest) (acp.PromptResponse, error) {
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

func (promptOnlyAgent) SetSessionConfigOption(context.Context, acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, nil
}

func (promptOnlyAgent) SetSessionMode(context.Context, acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

// ExampleAgentRefusals is an agent that embeds AgentRefusals, and the answer
// it gives session/list
// (0038-PLAN-consumer-helpers-kind-and-agent-refusals.md, step 4).
func ExampleAgentRefusals() {
	var a acp.Agent = promptOnlyAgent{}
	_, err := a.ListSessions(context.Background(), acp.ListSessionsRequest{})
	fmt.Println(err)
	// Output:
	// {"code":-32601,"message":"Method not found","data":{"method":"session/list"}}
}

// ExampleClientRefusals is a client that embeds ClientRefusals, and the answer
// it gives a file read
// (0036-PLAN-session-update-constructors-kind-and-client-refusals.md, step 4).
func ExampleClientRefusals() {
	var c acp.Client = refusingClient{}
	_, err := c.ReadTextFile(context.Background(), acp.ReadTextFileRequest{})
	fmt.Println(err)
	// Output:
	// {"code":-32601,"message":"Method not found","data":{"method":"fs/read_text_file"}}
}

// ExampleRequestError reads the code and message of the error a peer's
// handler answered with (0024-PLAN-documentation.md, A8).
func ExampleRequestError() {
	toA, fromB := io.Pipe()
	toB, fromA := io.Pipe()
	a := acp.NewConnection(func(context.Context, string, json.RawMessage) (any, *acp.RequestError) {
		return nil, acp.NewInvalidParams(map[string]any{"error": "path must be absolute"})
	}, fromA, toA)
	b := acp.NewConnection(nil, fromB, toB)

	_, err := acp.SendRequest[json.RawMessage](b, context.Background(), "_example/read", map[string]any{"path": "notes.txt"})
	if re, ok := errors.AsType[*acp.RequestError](err); ok {
		fmt.Println(re.Code, re.Message)
	}

	if err := b.Close(); err != nil {
		fmt.Println("close:", err)
	}
	<-a.Done()
	// Output:
	// -32602 Invalid params
}

// ExampleConnection_Close shows that a send after Close fails with an error
// that wraps ErrConnectionClosed (0024-PLAN-documentation.md, A8).
func ExampleConnection_Close() {
	r, w := io.Pipe()
	defer w.Close()
	c := acp.NewConnection(nil, io.Discard, r)
	if err := c.Close(); err != nil {
		fmt.Println("close:", err)
	}
	err := c.SendNotification(context.Background(), "_example/ping", map[string]any{})
	fmt.Println(errors.Is(err, acp.ErrConnectionClosed))
	// Output:
	// true
}

// ExampleWithNotificationOverflowPolicy names OverflowBlock, the default, with
// a queue of one notification. While the first handler is slow, the connection
// stops reading, so the peer's writes below wait, and nothing is dropped or
// closed. WithNotificationOverflowPolicy(acp.OverflowCloseConnection) would
// close the connection instead (0044-MADR-notification-queue-back-pressure.md).
func ExampleWithNotificationOverflowPolicy() {
	r, w := io.Pipe()
	c := acp.NewConnection(func(_ context.Context, _ string, params json.RawMessage) (any, *acp.RequestError) {
		var p struct{ N int }
		_ = json.Unmarshal(params, &p)
		if p.N == 1 {
			time.Sleep(10 * time.Millisecond) // a slow handler: the queue fills
		}
		fmt.Println("handled", p.N)
		return nil, nil
	}, io.Discard, r,
		acp.WithMaxQueuedNotifications(1),
		acp.WithNotificationOverflowPolicy(acp.OverflowBlock))

	for n := 1; n <= 4; n++ {
		if _, err := fmt.Fprintf(w, `{"jsonrpc":"2.0","method":"_example/progress","params":{"n":%d}}`+"\n", n); err != nil {
			fmt.Println("write:", err)
		}
	}
	_ = w.Close() // the end of the peer's output
	_ = c.Wait()  // the connection ends at the end of input, its handlers done
	fmt.Println("dropped:", c.DroppedNotifications())
	// Output:
	// handled 1
	// handled 2
	// handled 3
	// handled 4
	// dropped: 0
}

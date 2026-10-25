package acp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

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

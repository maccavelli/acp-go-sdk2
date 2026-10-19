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

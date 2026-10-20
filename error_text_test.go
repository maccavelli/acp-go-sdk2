package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// TestHandlerErrorReachesPeer requires an error a request's method returns to
// reach the peer as the docs of Agent and Client say: a *RequestError as it
// is; any other error -32603, or -32800 when it wraps context.Canceled, with
// its text in data's "error". It checks both sides: an Agent's NewSession, and
// a Client's ReadTextFile (0024-PLAN-behaviour-fixes.md, A2).
func TestHandlerErrorReachesPeer(t *testing.T) {
	t.Parallel()
	const text = "open /srv/app/token-7f3: permission denied"
	for _, tc := range []struct {
		name    string
		err     error
		code    int
		message string
		data    string
	}{
		{"another error", errors.New(text), -32603, "", text},
		{"one that wraps context.Canceled", fmt.Errorf("%s: %w", text, context.Canceled), -32800, "", text + ": context canceled"},
		{"a *RequestError", &RequestError{Code: -32042, Message: "private"}, -32042, "private", ""},
	} {
		fail := tc.err
		agent := agentFuncs{NewSessionFunc: func(context.Context, NewSessionRequest) (NewSessionResponse, error) {
			return NewSessionResponse{}, fail
		}}
		client := &clientFuncs{ReadTextFileFunc: func(context.Context, ReadTextFileRequest) (ReadTextFileResponse, error) {
			return ReadTextFileResponse{}, fail
		}}
		toAgent, fromClient := io.Pipe()
		toClient, fromAgent := io.Pipe()
		t.Cleanup(func() {
			_ = fromClient.Close()
			_ = fromAgent.Close()
		})
		asc := NewAgentSideConnection(agent, fromAgent, toAgent)
		csc := NewClientSideConnection(client, fromClient, toClient)
		negotiate(t, asc, allAdvertised())
		negotiate(t, csc, allAdvertised())

		_, toClientErr := asc.ReadTextFile(t.Context(), ReadTextFileRequest{SessionId: "s", Path: testAbsPath()})
		_, toAgentErr := csc.NewSession(t.Context(), NewSessionRequest{Cwd: testAbsPath(), McpServers: []McpServer{}})
		for side, err := range map[string]error{"the client's ReadTextFile": toClientErr, "the agent's NewSession": toAgentErr} {
			re, ok := errors.AsType[*RequestError](err)
			if !ok || re.Code != tc.code || (tc.message != "" && re.Message != tc.message) {
				t.Errorf("%s, %s: the peer got %v, want code %d", tc.name, side, err, tc.code)
				continue
			}
			got := ""
			if m, ok := re.Data.(map[string]any); ok {
				got, _ = m["error"].(string)
			}
			if got != tc.data {
				t.Errorf("%s, %s: data's error %q, want %q", tc.name, side, got, tc.data)
			}
		}
	}
}

// TestValidationMessagesQuoteValue requires what ValidationError's doc says:
// an absolute-path or excluded-value check's message quotes the value it
// refuses, and a one-variant check's has no value to quote; the value reaches
// a refused request's answer and a refused notification's log as it was
// (0024-PLAN-behaviour-fixes.md, A14).
func TestValidationMessagesQuoteValue(t *testing.T) {
	t.Parallel()
	const dir, loc = "relative/dir-7f3", "relative/loc-7f3"
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"absolute path", (&NewSessionRequest{Cwd: dir, McpServers: []McpServer{}}).validate(isPortableAbs), fmt.Sprintf("%q", dir)},
		{"excluded value", (&CreateElicitationRequest{Other: &CreateElicitationOther{Mode: "form"}}).validate(isPortableAbs), `"form"`},
	} {
		if tc.err == nil || !strings.Contains(tc.err.Error(), tc.want) {
			t.Errorf("%s: message %v, want it to hold %s", tc.name, tc.err, tc.want)
		}
	}
	if err := (&CreateElicitationRequest{}).validate(isPortableAbs); err == nil ||
		err.Error() != "CreateElicitationRequest must have exactly one variant set" {
		t.Errorf("one variant: message %v", err)
	}

	agent := rawWire(t, func(w io.Writer, r io.Reader) {
		negotiate(t, NewAgentSideConnection(agentFuncs{}, w, r), allAdvertised())
	})
	agent.send(`{"jsonrpc":"2.0","id":1,"method":"session/new","params":{"cwd":"` + dir + `","mcpServers":[]}}`)
	if a := agent.next("the answer"); a.Error == nil || a.Error.Code != -32602 || !strings.Contains(fmt.Sprint(a.Error.Data), fmt.Sprintf("%q", dir)) {
		t.Errorf("the answer %+v, want -32602 quoting %q", a.Error, dir)
	}

	var logs lockedBuffer
	client := rawWire(t, func(w io.Writer, r io.Reader) {
		negotiate(t, NewClientSideConnection(&clientFuncs{}, w, r, WithLogger(slog.New(slog.NewTextHandler(&logs, nil)))), allAdvertised())
	})
	update, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{
		"sessionId": "s", "update": map[string]any{"sessionUpdate": "tool_call", "toolCallId": "c", "title": "t",
			"locations": []any{map[string]any{"path": loc}}}}})
	if err != nil {
		t.Fatal(err)
	}
	client.send(string(update))
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(logs.String(), loc) {
		if time.Now().After(deadline) {
			t.Fatalf("the refused notification's log lacks %q: %s", loc, logs.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

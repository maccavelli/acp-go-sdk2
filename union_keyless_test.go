package acp

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync/atomic"
	"testing"
)

// The key-less variants (0049-PLAN-surface-fixes.md, phase 1): a union without
// a catch-all decodes its key-less variant's own name into it, strictly; and
// SetSessionConfigOptionRequest, as its schema says, decodes an unknown type
// whose payload fits the value-id variant into it. Every other unknown type is
// kept in Unknown (0011-MADR-harden-code-generator-engine.md, F1).

// keyLessPair connects a client to an agent whose set_config_option handler
// sends what it receives on got, both initialized.
func keyLessPair(t *testing.T, got chan<- SetSessionConfigOptionRequest) *ClientSideConnection {
	t.Helper()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aW.Close()
		_ = a2cW.Close()
	})
	ag := agentFuncs{SetSessionConfigOptionFunc: func(_ context.Context, p SetSessionConfigOptionRequest) (SetSessionConfigOptionResponse, error) {
		got <- p
		return SetSessionConfigOptionResponse{}, nil
	}}
	NewAgentSideConnection(ag, a2cW, c2aR)
	csc := NewClientSideConnection(&clientFuncs{}, c2aW, a2cR)
	if _, err := csc.Initialize(t.Context(), InitializeRequest{ProtocolVersion: ProtocolVersionNumber}); err != nil {
		t.Fatal(err)
	}
	return csc
}

// TestSetConfigOptionUnknownTypeIsValueId requires an agent's handler, given
// a set_config_option whose type is unknown and whose value is a string, to
// read it as the value-id variant, with its session, config and value, as the
// schema says: "Unknown `type` values with string payloads also gracefully
// deserialize into this variant." A non-string value with an unknown type is
// kept in Unknown.
func TestSetConfigOptionUnknownTypeIsValueId(t *testing.T) {
	t.Parallel()
	got := make(chan SetSessionConfigOptionRequest, 1)
	csc := keyLessPair(t, got)
	params := json.RawMessage(`{"sessionId":"s","configId":"model","type":"select","value":"fast"}`)
	if _, err := SendRequest[json.RawMessage](csc.Connection(), t.Context(), AgentMethodSessionSetConfigOption, params); err != nil {
		t.Fatal(err)
	}
	p := within(t, got, "the handler")
	if p.ValueId == nil || p.ValueId.SessionId != "s" || p.ValueId.ConfigId != "model" || p.ValueId.Value != "fast" {
		t.Errorf("handler got %+v, Kind() %q; want the value-id variant {s model fast}", p, p.Kind())
	}

	var u SetSessionConfigOptionRequest
	if err := json.Unmarshal([]byte(`{"sessionId":"s","configId":"c","type":"future","value":true}`), &u); err != nil {
		t.Fatal(err)
	}
	if u.Unknown == nil || u.ValueId != nil || u.Kind() != "future" {
		t.Errorf("an unknown type with a boolean value decoded as %+v, Kind() %q; want Unknown", u, u.Kind())
	}
}

// TestKeylessVariantNamedExplicitly requires McpServer's "type":"stdio" and
// AuthMethod's "type":"agent", their key-less variants' own names, to decode
// into Stdio and Agent, as an absent type does; ResolveStdioCommands then
// resolves the stdio server it now sees. A payload the own name does not
// decode as is an error naming it; an unknown type is still Unknown.
func TestKeylessVariantNamedExplicitly(t *testing.T) {
	t.Parallel()
	var m McpServer
	if err := json.Unmarshal([]byte(`{"type":"stdio","name":"fs","command":"npx","args":[],"env":[]}`), &m); err != nil {
		t.Fatal(err)
	}
	if m.Stdio == nil || m.Stdio.Name != "fs" || m.Kind() != "stdio" {
		t.Fatalf(`McpServer {"type":"stdio",...} decoded as %+v`, m)
	}
	req := &NewSessionRequest{Cwd: testAbsPath("w"), McpServers: []McpServer{m}}
	if err := ResolveStdioCommands(fakeLookPath(new(atomic.Int32)))(t.Context(), AgentMethodSessionNew, req); err != nil {
		t.Fatal(err)
	}
	if got, want := req.McpServers[0].Stdio.Command, testAbsPath("bin", "npx"); got != want {
		t.Errorf("ResolveStdioCommands left command %q, want %q", got, want)
	}

	var a AuthMethod
	if err := json.Unmarshal([]byte(`{"type":"agent","id":"login","name":"Log in"}`), &a); err != nil {
		t.Fatal(err)
	}
	if a.Agent == nil || a.Agent.Id != "login" || a.Kind() != "agent" {
		t.Errorf(`AuthMethod {"type":"agent",...} decoded as %+v`, a)
	}

	var bad McpServer
	if err := json.Unmarshal([]byte(`{"type":"stdio","name":"fs"}`), &bad); err == nil || !strings.Contains(err.Error(), `McpServer: type "stdio"`) {
		t.Errorf(`{"type":"stdio"} without command = %v, decoded as %+v; want an error naming McpServer: type "stdio"`, err, bad)
	}
	var future McpServer
	if err := json.Unmarshal([]byte(`{"type":"ws","name":"fs","command":"npx","args":[],"env":[]}`), &future); err != nil {
		t.Fatal(err)
	}
	if future.Unknown == nil || future.Stdio != nil {
		t.Errorf("an unknown type with stdio's keys decoded as %+v, want Unknown", future)
	}
}

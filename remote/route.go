package remote

import (
	"bytes"
	"encoding/json"
	"strings"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// kind is what a line of the seam holds.
type kind int

const (
	kindOther        kind = iota // not a JSON-RPC object: a batch, a value of another type, or not JSON
	kindRequest                  // a method and an id
	kindNotification             // a method and no id
	kindResponse                 // no method; a result or an error
)

// message is what the transports route a line by.
type message struct {
	kind kind
	// idKey is the id's bytes, or "" when there is none. The connection
	// writes an answer's id as the bytes it received (connection.go's
	// anyMessage.ID is a *json.RawMessage), so a request and its answer key
	// the same.
	idKey string
	// method is the method, when it is a string.
	method string
	// sessionID is params.sessionId, when params is an object and it a string.
	sessionID string
	// resultSessionID is result.sessionId, when result is an object and it a
	// string.
	resultSessionID string
}

// classify reads line, one message, for routing. A line it cannot read is
// kindOther; routing sends it where it sends a connection's own messages.
func classify(line []byte) message {
	var env struct {
		ID     json.RawMessage `json:"id"`
		Method json.RawMessage `json:"method"`
		Params json.RawMessage `json:"params"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 || trimmed[0] != '{' || json.Unmarshal(trimmed, &env) != nil {
		return message{}
	}
	// A json.RawMessage decoded from a member holds the value's bytes with no
	// space around them, and a scalar has none inside, so the bytes are the key.
	m := message{idKey: string(env.ID)}
	switch {
	case env.Method != nil:
		if json.Unmarshal(env.Method, &m.method) != nil || m.method == "" {
			return message{}
		}
		m.kind = kindNotification
		if env.ID != nil {
			m.kind = kindRequest
		}
		m.sessionID = sessionIDOf(env.Params)
	case env.Result != nil || env.Error != nil:
		m.kind = kindResponse
		m.resultSessionID = sessionIDOf(env.Result)
	default:
		return message{}
	}
	return m
}

// sessionIDOf returns raw's sessionId member when raw is an object and the
// member a string, and "" otherwise.
func sessionIDOf(raw json.RawMessage) string {
	if len(raw) == 0 || raw[0] != '{' {
		return ""
	}
	var v struct {
		SessionID json.RawMessage `json:"sessionId"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return ""
	}
	var s string
	if json.Unmarshal(v.SessionID, &s) != nil {
		return ""
	}
	return s
}

// connectionScoped reports whether method is a protocol method, "$/…", which
// belongs to the connection whatever its params name, as Rust's protocol.rs
// keeps it (docs/decisions/0043-MADR-remote-http-and-websocket-transport.md,
// K3).
func connectionScoped(method string) bool {
	return strings.HasPrefix(method, "$/")
}

// sessionScopedMethods are the methods that act on one session, the list of
// Rust's protocol.rs and TypeScript's protocol.ts without session/set_model,
// which the pinned schema does not have. "session/fork" is spelled out:
// acp.AgentMethodSessionFork is deprecated, and staticcheck's SA1019 reports
// its use outside acp (0043-PLAN-remote-http-and-websocket-transport.md,
// phase 3).
var sessionScopedMethods = map[string]bool{
	acp.AgentMethodSessionPrompt:          true,
	acp.AgentMethodSessionCancel:          true,
	acp.AgentMethodSessionClose:           true,
	acp.AgentMethodSessionDelete:          true,
	"session/fork":                        true,
	acp.AgentMethodSessionLoad:            true,
	acp.AgentMethodSessionResume:          true,
	acp.AgentMethodSessionSetConfigOption: true,
	acp.AgentMethodSessionSetMode:         true,
}

// sessionScoped reports whether method acts on one session.
func sessionScoped(method string) bool {
	return sessionScopedMethods[method]
}

// session is the session m belongs to for routing: its params' sessionId,
// unless its method is connection-scoped.
func (m message) session() string {
	if connectionScoped(m.method) {
		return ""
	}
	return m.sessionID
}

package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ExtensionMethodHandler can be implemented by either an Agent or a Client.
//
// ACP extension methods are JSON-RPC methods whose names begin with "_".
// They provide a stable namespace for custom functionality that is not part
// of the core ACP spec.
//
// If the method is unrecognized, implementations should return NewMethodNotFound(method).
//
// A received extension message passes the inbound hooks and the initialize
// gate before it reaches HandleExtensionMethod, as a typed one does: before
// initialize, a request is answered -32601 and a notification dropped; the
// hooks see params as a *json.RawMessage, which WithInboundRewrite may
// replace, and the method gets what they leave
// (0019-MADR-remediate-debugging-pass-findings.md, decision 7).
//
// See: https://agentclientprotocol.com/protocol/extensibility#extension-methods
type ExtensionMethodHandler interface {
	HandleExtensionMethod(ctx context.Context, method string, params json.RawMessage) (any, error)
}

func validateExtensionMethodName(method string) error {
	if method == "" {
		return errors.New("extension method name must be non-empty")
	}
	if !strings.HasPrefix(method, "_") {
		return fmt.Errorf("extension method name must start with '_' (got %q)", method)
	}
	return nil
}

func isExtensionMethodName(method string) bool {
	return strings.HasPrefix(method, "_")
}

func (a *AgentSideConnection) handleWithExtensions(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
	if isExtensionMethodName(method) {
		return handleExtension(ctx, a.conn, a.inboundGate(), a.agent, method, params)
	}

	return a.handle(ctx, method, params)
}

func (c *ClientSideConnection) handleWithExtensions(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
	if isExtensionMethodName(method) {
		return handleExtension(ctx, c.conn, c.inboundGate(), c.client, method, params)
	}

	return c.handle(ctx, method, params)
}

// handleExtension checks a received extension message with the inbound hooks
// and gate, as the generated dispatch checks a typed one, then hands it to
// value when value implements ExtensionMethodHandler
// (0019-PLAN-api-decisions.md, B14).
func handleExtension(ctx context.Context, conn *Connection, gate gateFunc, value any, method string, params json.RawMessage) (any, *RequestError) {
	raw := params
	if err := conn.checkInbound(ctx, method, &raw, gate); err != nil {
		return nil, inboundError(err)
	}
	h, ok := value.(ExtensionMethodHandler)
	if !ok {
		return nil, NewMethodNotFound(method)
	}
	resp, err := h.HandleExtensionMethod(ctx, method, raw)
	if err != nil {
		return nil, toReqErr(err)
	}
	return resp, nil
}

// CallExtension sends an ACP extension-method request (method names starting with "_")
// from an agent to its client. It does not check what initialize negotiated.
//
// Before anything is sent, it returns an error when method does not start
// with "_"; a *ValidationError when params are of a type of this package and
// break the schema; and a *RequestError when params do not encode, or encode
// as neither null nor a JSON object or array. It returns a *RequestError
// wrapping context.Cause(ctx) when ctx ends, ErrPeerDisconnected or
// ErrConnectionClosed when the connection ends, or the write's error when
// writing fails; and the peer's error answer as a *RequestError
// (0024-PLAN-documentation.md, A6).
func (a *AgentSideConnection) CallExtension(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if err := validateExtensionMethodName(method); err != nil {
		return nil, err
	}
	return SendRequest[json.RawMessage](a.conn, ctx, method, params)
}

// NotifyExtension sends an ACP extension-method notification (method names starting with "_")
// from an agent to its client. It does not check what initialize negotiated.
//
// Before anything is sent, it returns an error when method does not start
// with "_"; a *ValidationError when params are of a type of this package and
// break the schema; and a *RequestError when params do not encode, or encode
// as neither null nor a JSON object or array. It returns a *RequestError
// wrapping context.Cause(ctx) when ctx ends, ErrPeerDisconnected or
// ErrConnectionClosed when the connection ends, or the write's error when
// writing fails (0024-PLAN-documentation.md, A6).
func (a *AgentSideConnection) NotifyExtension(ctx context.Context, method string, params any) error {
	if err := validateExtensionMethodName(method); err != nil {
		return err
	}
	return a.conn.SendNotification(ctx, method, params)
}

// CallExtension sends an ACP extension-method request (method names starting with "_")
// from a client to its agent. It does not check what initialize negotiated.
//
// Before anything is sent, it returns an error when method does not start
// with "_"; a *ValidationError when params are of a type of this package and
// break the schema; and a *RequestError when params do not encode, or encode
// as neither null nor a JSON object or array. It returns a *RequestError
// wrapping context.Cause(ctx) when ctx ends, ErrPeerDisconnected or
// ErrConnectionClosed when the connection ends, or the write's error when
// writing fails; and the peer's error answer as a *RequestError
// (0024-PLAN-documentation.md, A6).
func (c *ClientSideConnection) CallExtension(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if err := validateExtensionMethodName(method); err != nil {
		return nil, err
	}
	return SendRequest[json.RawMessage](c.conn, ctx, method, params)
}

// NotifyExtension sends an ACP extension-method notification (method names starting with "_")
// from a client to its agent. It does not check what initialize negotiated.
//
// Before anything is sent, it returns an error when method does not start
// with "_"; a *ValidationError when params are of a type of this package and
// break the schema; and a *RequestError when params do not encode, or encode
// as neither null nor a JSON object or array. It returns a *RequestError
// wrapping context.Cause(ctx) when ctx ends, ErrPeerDisconnected or
// ErrConnectionClosed when the connection ends, or the write's error when
// writing fails (0024-PLAN-documentation.md, A6).
func (c *ClientSideConnection) NotifyExtension(ctx context.Context, method string, params any) error {
	if err := validateExtensionMethodName(method); err != nil {
		return err
	}
	return c.conn.SendNotification(ctx, method, params)
}

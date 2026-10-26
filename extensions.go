package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
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

// ExtensionMux is an ExtensionMethodHandler that routes each extension
// method to the function registered for its name, as http.ServeMux routes a
// request to the handler registered for its pattern. Handle registers a
// request method and Notify a notification method. A request for a name
// registered by neither, or by Notify, is answered NewMethodNotFound; a
// notification for a name registered by neither, or by Handle, is ignored,
// as the specification says an unrecognized notification should be. It
// tells the two apart with IsNotification, so a call it did not receive
// through a connection is taken for a request. The zero ExtensionMux is
// ready; it is safe for concurrent use
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 4).
//
// An Agent or a Client serves it by returning its HandleExtensionMethod from
// its own, or by embedding it.
type ExtensionMux struct {
	mu       sync.RWMutex
	handlers map[string]extensionHandler
}

// extensionHandler is one registration of an ExtensionMux: run decodes the
// params and calls the registered function.
type extensionHandler struct {
	notification bool
	run          func(ctx context.Context, params json.RawMessage) (any, error)
}

// Handle registers fn for requests of method. A request's params are decoded
// into P with encoding/json, absent ones as {}, and a decode failure is
// answered NewInvalidParams; otherwise fn's result is the answer, and its
// error is answered as a handler's is, a *RequestError as it is. Handle
// panics when method does not start with "_", when it is already
// registered, or when fn is nil.
func Handle[P, R any](m *ExtensionMux, method string, fn func(context.Context, P) (R, error)) {
	if fn == nil {
		panic(fmt.Sprintf("acp: nil handler for extension method %q", method))
	}
	m.register(method, extensionHandler{run: func(ctx context.Context, params json.RawMessage) (any, error) {
		var p P
		if err := decodeParams(params, &p); err != nil {
			return nil, NewInvalidParams(map[string]any{jsonErrorField: err.Error()})
		}
		r, err := fn(ctx, p)
		if err != nil {
			return nil, err
		}
		return r, nil
	}})
}

// Notify registers fn for notifications of method. A notification's params
// are decoded as Handle decodes a request's. Nothing is answered: a decode
// failure or fn's error is logged by the connection, as a notification
// handler's is. Notify panics as Handle does.
func Notify[P any](m *ExtensionMux, method string, fn func(context.Context, P) error) {
	if fn == nil {
		panic(fmt.Sprintf("acp: nil handler for extension method %q", method))
	}
	m.register(method, extensionHandler{notification: true, run: func(ctx context.Context, params json.RawMessage) (any, error) {
		var p P
		if err := decodeParams(params, &p); err != nil {
			return nil, NewInvalidParams(map[string]any{jsonErrorField: err.Error()})
		}
		return nil, fn(ctx, p)
	}})
}

func (m *ExtensionMux) register(method string, h extensionHandler) {
	if err := validateExtensionMethodName(method); err != nil {
		panic("acp: " + err.Error())
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.handlers[method]; ok {
		panic(fmt.Sprintf("acp: extension method %q is already registered", method))
	}
	if m.handlers == nil {
		m.handlers = make(map[string]extensionHandler)
	}
	m.handlers[method] = h
}

// HandleExtensionMethod runs the function registered for method, as
// ExtensionMux describes.
func (m *ExtensionMux) HandleExtensionMethod(ctx context.Context, method string, params json.RawMessage) (any, error) {
	m.mu.RLock()
	h, ok := m.handlers[method]
	m.mu.RUnlock()
	notification := IsNotification(ctx)
	if !ok || h.notification != notification {
		if notification {
			return nil, nil
		}
		return nil, NewMethodNotFound(method)
	}
	return h.run(ctx, params)
}

// Methods returns the names registered with Handle and Notify, sorted.
func (m *ExtensionMux) Methods() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return slices.Sorted(maps.Keys(m.handlers))
}

// TraceMeta returns a _meta object holding the root keys the specification
// reserves for trace context: traceparent and tracestate, of W3C Trace
// Context, and baggage, of W3C Baggage, each set to its argument. A key whose
// argument is empty is left out. The values' syntax is not checked.
func TraceMeta(traceparent, tracestate, baggage string) map[string]any {
	meta := make(map[string]any, 3)
	for key, value := range map[string]string{"traceparent": traceparent, "tracestate": tracestate, "baggage": baggage} {
		if value != "" {
			meta[key] = value
		}
	}
	return meta
}

// AdvertiseMeta returns a copy of meta with features set under namespace, as
// the specification advertises a custom capability in a capability object's
// _meta; for its example, AdvertiseMeta(caps.Meta, "zed.dev",
// map[string]any{"workspace": true, "fileNotifications": true}). An empty
// namespace sets nothing. meta is not changed, and the copy is shallow.
func AdvertiseMeta(meta map[string]any, namespace string, features map[string]any) map[string]any {
	out := make(map[string]any, len(meta)+1)
	maps.Copy(out, meta)
	if namespace != "" {
		out[namespace] = features
	}
	return out
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

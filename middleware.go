package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
)

// Middleware wraps a MethodHandler: it returns next, or a handler that calls
// next at most once, or one that answers in its place. A connection calls each
// Middleware once, when it is built, not once per message
// (0046-MADR-sending-and-receiving-middleware.md, D1 and D2).
//
// The handler a Middleware returns must be safe for concurrent use: requests
// run concurrently. It calls next, if at all, before it returns, and with a
// context derived from the one it was given. It must not change params in
// place or keep them after it returns; to change them it passes next a new
// slice. A handler that panics ends the process, as a MethodHandler's panic
// does (D7).
type Middleware func(next MethodHandler) MethodHandler

// WithReceivingMiddleware wraps the connection's handler in mw. Each
// WithReceivingMiddleware appends to those given before it, the one exception
// to ConnectionOption's rule that the last of a repeated option wins, so a
// library that adds its own middleware keeps the application's. The first
// registered is outermost: WithReceivingMiddleware(a, b) and then
// WithReceivingMiddleware(c) run a, then b, then c, then the handler. A nil
// middleware, or one that returns a nil handler, panics when the connection
// is built (0046-MADR-sending-and-receiving-middleware.md, D2).
//
// The chain sees every request and notification that reaches the handler,
// where the handler runs: on the request's goroutine, counted against
// WithMaxConcurrentRequests and WithMaxQueuedBytes until it returns, or on the
// notification worker, where under OverflowBlock it must not wait for anything
// the peer sends later. It runs outside the side connections' dispatch: it
// sees the method and the raw params, nil when the peer left them out or sent
// null, before they are decoded, before WithInboundRewrite and
// WithInboundValidation and before the check of what initialize agreed, and
// it sees the result as the dispatch returns it, checked and not yet encoded.
// It may pass next other params or another method. A middleware that returns
// without calling next answers in the handler's place, and the dispatch's
// validation and its check of the result do not run for that message; a
// result it returns is encoded as a handler's is. IsNotification tells a
// notification from a request, and RequestIDFromContext gives a request's id
// (D3).
//
// The chain does not see what the reader handles itself: invalid messages,
// $/cancel_request, the requests refused over WithMaxConcurrentRequests, past
// WithMaxQueuedBytes or during Shutdown, the notifications an overflow policy
// drops, and responses (D6).
func WithReceivingMiddleware(mw ...Middleware) ConnectionOption {
	return func(cfg *connectionConfig) {
		cfg.receiving = append(cfg.receiving, mw...)
	}
}

// chainMiddleware returns inner wrapped in mw, mw[0] outermost, calling each
// middleware once. It panics, naming option and the middleware's position
// across the option's calls, for a nil middleware or a nil handler one returns
// (0046-MADR-sending-and-receiving-middleware.md, D2).
func chainMiddleware(option string, mw []Middleware, inner MethodHandler) MethodHandler {
	h := inner
	for i, m := range slices.Backward(mw) {
		if m == nil {
			panic(fmt.Sprintf("acp: %s: middleware %d is nil", option, i))
		}
		if h = m(h); h == nil {
			panic(fmt.Sprintf("acp: %s: middleware %d returned a nil handler", option, i))
		}
	}
	return h
}

// notFound is a nil handler's behaviour under a receiving chain, as
// handleInbound has it without one: a request is answered -32601, and a
// notification is dropped without a log
// (0046-MADR-sending-and-receiving-middleware.md, D3).
func notFound(ctx context.Context, method string, _ json.RawMessage) (any, *RequestError) {
	if IsNotification(ctx) {
		return nil, nil
	}
	return nil, NewMethodNotFound(method)
}

// RequestIDFromContext returns the id of the request the connection made ctx
// for, or the one ctx derives from, as a copy, and true: a number or a string
// as the peer sent it, or null for a null id. It returns false for a
// notification's context and for one the connection did not make. Receiving
// middleware and handlers can read it
// (0046-MADR-sending-and-receiving-middleware.md, D5).
func RequestIDFromContext(ctx context.Context) (json.RawMessage, bool) {
	if r, ok := ctx.Value(receiptKey{}).(requestReceipt); ok && r.id != nil {
		return slices.Clone(*r.id), true
	}
	return nil, false
}

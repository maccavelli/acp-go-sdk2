package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync/atomic"
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
//
// In a sending middleware it returns the id of the request being sent, which
// the connection allocates before the chain runs, and false for a
// notification being sent, even when ctx derives from a received request's.
func RequestIDFromContext(ctx context.Context) (json.RawMessage, bool) {
	if call, ok := ctx.Value(outboundKey{}).(*outboundCall); ok {
		if call.id == nil {
			return nil, false
		}
		return slices.Clone(*call.id), true
	}
	if r, ok := ctx.Value(receiptKey{}).(requestReceipt); ok && r.id != nil {
		return slices.Clone(*r.id), true
	}
	return nil, false
}

// WithSendingMiddleware wraps every request and notification the application
// sends in mw: SendRequest, SendRequestNoResult, SendNotification,
// CallExtension, NotifyExtension, and each typed method of the side
// connections, Initialize included. It appends and orders as
// WithReceivingMiddleware does, the first registered outermost, and panics
// the same way when the connection is built
// (0046-MADR-sending-and-receiving-middleware.md, D2 and D4).
//
// Before the chain, the connection does what it does without one: the typed
// methods check what initialize agreed, a context that has ended or an input
// that has ended fails the call, the request's id is allocated, and the
// params are validated and encoded; a message the connection would refuse
// never reaches the chain. The chain sees the method and the encoded params,
// and for a request returns the result as the peer sent it, a
// json.RawMessage, or the peer's error. Inside its innermost handler the
// connection writes the message and, for a request, waits for the answer
// and for the notifications received before it, so a middleware's timing
// includes the round trip. After the chain, the result is decoded and
// checked as a peer's result is, so a middleware cannot make the connection
// accept a result it would refuse. RequestIDFromContext gives a request's id.
//
// A middleware may answer without calling next. It may return another
// result: a json.RawMessage is used as it is, nil is no result, and any other
// value is encoded with encoding/json. It may pass next other params, which
// are written as given and must be nil, a JSON object or a JSON array.
// Initialize's result is recorded as the reader reads it, before the chain
// sees it: a middleware may fail the call, and the record stays, but a result
// other than the one next returned fails the call. The connection's own
// $/cancel_request, sent when a caller's context ends, does not pass the
// chain (D4).
//
// next must be called at most once, with the method it was given and a
// context derived from the one the middleware was given, which carries the
// call; otherwise it writes nothing and returns an error wrapping
// ErrSendingMiddleware (D10).
func WithSendingMiddleware(mw ...Middleware) ConnectionOption {
	return func(cfg *connectionConfig) {
		cfg.sending = append(cfg.sending, mw...)
	}
}

// outboundKey is the context key of the outboundCall a sending chain carries.
type outboundKey struct{}

// outboundCall is one message the application sends through the sending
// chain: what was prepared before the chain, for the innermost step, and what
// that step met, for the caller after it
// (0046-PLAN-sending-and-receiving-middleware.md, step 3.2).
type outboundCall struct {
	method string
	// id, idKey and pr are nil, empty and nil for a notification.
	id     *json.RawMessage
	idKey  string
	pr     *pendingResponse
	params json.RawMessage // as encoded, to tell a replacement

	entered atomic.Bool // next was called

	// The innermost step's outcome, read after the chain has returned.
	sent     bool             // the request was handed to the writer
	answered bool             // an answer was read, and pr's commit ran
	resp     responseEnvelope // that answer
	werr     error            // the wait for the notifications before it
	raw      json.RawMessage  // the result it carried, without an error
	innerErr *RequestError    // the error the innermost step returned
}

// misuse is the error the innermost step returns when a sending middleware
// breaks rule (0046-MADR-sending-and-receiving-middleware.md, D10).
func misuse(rule string) *RequestError {
	return NewInternalError(map[string]any{jsonErrorField: rule}).withCause(ErrSendingMiddleware)
}

// sameBytes reports whether a and b are the same slice of memory, so a
// middleware passed on the params it was given.
func sameBytes(a, b json.RawMessage) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}

// structuredParams reports whether p may be written as a message's params:
// none, a JSON object or a JSON array, as encodeParams leaves them.
func structuredParams(p json.RawMessage) bool {
	t := bytes.TrimLeft(p, " \t\r\n")
	return len(t) == 0 || t[0] == '{' || t[0] == '['
}

// sendInnermost is the sending chain's innermost handler. It checks the call
// against D10's rules, then writes the message and, for a request, waits for
// its answer and for the notifications received before it, recording what it
// met on the call (0046-MADR-sending-and-receiving-middleware.md, D4 and D10).
func (c *Connection) sendInnermost(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
	call, ok := ctx.Value(outboundKey{}).(*outboundCall)
	switch {
	case !ok:
		return nil, misuse("next called with a context that does not carry the call")
	case !call.entered.CompareAndSwap(false, true):
		return nil, misuse("next called twice")
	case method != call.method:
		return nil, misuse("next called with another method")
	case !sameBytes(params, call.params) && !structuredParams(params):
		return nil, misuse("params must be a JSON object or array")
	}
	if call.pr == nil {
		if ctx.Err() != nil {
			return nil, NewInternalError(map[string]any{jsonErrorField: ctx.Err().Error()}).withCause(context.Cause(ctx))
		}
		if err := c.sendMessage(ctx, anyMessage{Method: method, Params: params}); err != nil {
			return nil, toReqErr(err)
		}
		return nil, nil
	}
	if ctx.Err() != nil {
		return nil, toReqErr(context.Cause(ctx))
	}
	resp, sent, err := c.exchangePrepared(ctx, anyMessage{ID: call.id, Method: method, Params: params}, call.idKey, call.pr)
	call.sent = sent
	if err != nil {
		call.innerErr = toReqErr(err)
		return nil, call.innerErr
	}
	call.answered, call.resp = true, resp
	if resp.msg.Error == nil {
		call.raw = resp.msg.Result
	}
	werr := c.waitNotificationsUpTo(ctx, resp.notificationWatermark)
	if call.pr.commit == nil {
		switch {
		case werr != nil:
			call.innerErr = toReqErr(werr)
		case resp.msg.Error != nil:
			call.innerErr = resp.msg.Error
		default:
			return resp.msg.Result, nil
		}
		return nil, call.innerErr
	}
	// sendRequestCommit's order: a committed result stands even when the
	// wait then ends.
	call.werr = werr
	switch {
	case resp.msg.Error == nil && resp.commitErr == nil:
		return resp.msg.Result, nil
	case werr != nil:
		call.innerErr = toReqErr(werr)
	case resp.msg.Error != nil:
		call.innerErr = resp.msg.Error
	default:
		call.innerErr = toReqErr(resp.commitErr)
	}
	return nil, call.innerErr
}

// startCall does what exchange does before it registers a request, and
// returns the call the sending chain carries, registered as pr
// (0046-PLAN-sending-and-receiving-middleware.md, step 3.4).
func (c *Connection) startCall(ctx context.Context, method string, params any, pr *pendingResponse) (*outboundCall, error) {
	if ctx.Err() != nil {
		return nil, toReqErr(context.Cause(ctx))
	}
	if c.inputEnded() {
		return nil, inputEndedError()
	}
	msg, idKey, err := c.prepareRequest(method, params)
	if err != nil {
		return nil, err
	}
	return &outboundCall{method: method, id: msg.ID, idKey: idKey, pr: pr, params: msg.Params}, nil
}

// runChain runs the sending chain for call, under a context that carries it.
func (c *Connection) runChain(ctx context.Context, call *outboundCall) (any, *RequestError) {
	return c.sending(context.WithValue(ctx, outboundKey{}, call), call.method, call.params)
}

// resultBytes is what a sending chain returned, as the bytes the caller
// decodes: a json.RawMessage as it is, nil as none, and any other value
// encoded (0046-MADR-sending-and-receiving-middleware.md, D4).
func resultBytes(r any) (json.RawMessage, *RequestError) {
	switch v := r.(type) {
	case nil:
		return nil, nil
	case json.RawMessage:
		return v, nil
	}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, NewInternalError(map[string]any{jsonErrorField: err.Error()})
	}
	return b, nil
}

// sendThroughChain is sendRequestSent with a sending chain
// (0046-PLAN-sending-and-receiving-middleware.md, step 3.4).
func sendThroughChain[T any](c *Connection, ctx context.Context, method string, params any, gate gateFunc) (result T, sent bool, err error) { //nolint:revive // context-as-argument: the connection value stays the first parameter.
	call, err := c.startCall(ctx, method, params, &pendingResponse{ch: make(chan responseEnvelope, 1)})
	if err != nil {
		return result, false, err
	}
	r, rerr := c.runChain(ctx, call)
	if rerr != nil {
		return result, call.sent, rerr
	}
	raw, rerr := resultBytes(r)
	if rerr != nil {
		return result, call.sent, rerr
	}
	result, err = decodeResult[T](c, ctx, method, params, gate, raw)
	return result, call.sent, err
}

// commitThroughChain is sendRequestCommit with a sending chain, after its
// peer wait has begun. settled runs when no answer was judged, as the path
// without a chain runs it; a result other than the one the innermost step
// returned fails the call, since the record follows the answer the reader read
// (0046-MADR-sending-and-receiving-middleware.md, D4).
func commitThroughChain[T any](c *Connection, ctx context.Context, method string, params any, pr *pendingResponse, settled func()) (T, error) { //nolint:revive // context-as-argument: the connection value stays the first parameter.
	var zero T
	call, err := c.startCall(ctx, method, params, pr)
	if err != nil {
		settled()
		return zero, err
	}
	r, rerr := c.runChain(ctx, call)
	if !call.answered {
		settled()
	}
	switch {
	case rerr != nil && (rerr != call.innerErr || !call.answered):
		return zero, rerr
	case !call.answered:
		return zero, misuse("a sending middleware replaced the result of " + method)
	case rerr == nil:
		raw, berr := resultBytes(r)
		if berr != nil || !bytes.Equal(raw, call.raw) {
			return zero, misuse("a sending middleware replaced the result of " + method)
		}
	}
	return committedResult[T](call.resp, call.werr)
}

package acp

import (
	"encoding/json"
	"slices"
	"strconv"
)

// ObservationKind is what an Observation reports: a message the connection
// handled itself, which no middleware sees
// (0046-MADR-sending-and-receiving-middleware.md, D9).
type ObservationKind int

const (
	// ObservedFault is a received line the connection answered as invalid:
	// over WithMaxMessageSize, not JSON, not a JSON-RPC object, or not a valid
	// request. Err is the answer, ID the id it carries when that is valid,
	// and Method is empty: the line is not trusted.
	ObservedFault ObservationKind = iota + 1
	// ObservedCancelReceived is a $/cancel_request the peer sent. ID is the
	// requestId it names, or nil when its params do not decode or name none.
	ObservedCancelReceived
	// ObservedCancelSent is a $/cancel_request the connection sent when a
	// caller's context ended. ID is the requestId it names, and Err the
	// write's error, if any.
	ObservedCancelSent
	// ObservedRefused is a received request answered without its handler:
	// over WithMaxConcurrentRequests, past WithMaxQueuedBytes, or during
	// Shutdown. Err is the answer. RefusedMessages does not count it: it
	// counts the refusals of the hooks and the validation, which the
	// receiving chain sees.
	ObservedRefused
	// ObservedUnmatchedResponse is a response no pending request took: a late
	// answer to a call whose caller gave up, one with an unknown id, or one
	// without a usable id. Err is the error it carried, if any. A response a
	// call takes is that call's result, which sending middleware sees.
	ObservedUnmatchedResponse
	// ObservedDropped is a notification the overflow policy dropped under
	// OverflowDropNewest, as the drop handler is called for it, or, with Err
	// ErrNotificationQueueOverflow, the one whose overflow closed the
	// connection.
	ObservedDropped
)

// String returns k's name, as a log or a metric's label would carry it, or
// ObservationKind(n) for a value that is not a kind.
func (k ObservationKind) String() string {
	switch k {
	case ObservedFault:
		return "fault"
	case ObservedCancelReceived:
		return "cancel_received"
	case ObservedCancelSent:
		return "cancel_sent"
	case ObservedRefused:
		return "refused"
	case ObservedUnmatchedResponse:
		return "unmatched_response"
	case ObservedDropped:
		return "dropped"
	}
	return "ObservationKind(" + strconv.Itoa(int(k)) + ")"
}

// Observation is one message the connection handled itself, as WithObserver
// reports it. Its fields are plain values: ID is the observer's own copy
// (0046-MADR-sending-and-receiving-middleware.md, D9).
type Observation struct {
	Kind ObservationKind
	// Method is the message's method, when it names one and was decoded.
	Method string
	// ID is the message's id, or for a cancel the requestId it names; nil
	// when there is none.
	ID json.RawMessage
	// Size is the received line's length in bytes, counted for a line over
	// WithMaxMessageSize too, as it is discarded; 0 for a message sent.
	Size int
	// Err is the *RequestError the connection answered, the error a response
	// carried, the write's error, or ErrNotificationQueueOverflow; nil
	// otherwise.
	Err error
}

// WithObserver reports to fn every message the connection handles itself,
// which neither WithReceivingMiddleware nor WithSendingMiddleware sees:
// faults, $/cancel_request in both directions, requests refused by a limit
// or by Shutdown, unmatched responses, and dropped notifications. With both
// chains and an observer, every message is seen once. The last WithObserver
// given wins, and nil removes it
// (0046-MADR-sending-and-receiving-middleware.md, D6 and D9).
//
// fn runs synchronously on the goroutine that met the message: the reader,
// for every kind but ObservedCancelSent, which runs on the goroutine that
// sends cancels. It must be safe for concurrent use and must not block: while
// it runs, the connection reads nothing. It is never called with a lock of
// the connection held, so it may call the connection's methods, such as
// QueuedBytes. It cannot change or stop a message. A panic in it ends the
// process, as a handler's does.
//
// fn runs after the connection has acted on the message: after the answer to
// a fault or a refusal is written, after the request a $/cancel_request names
// is cancelled, after the connection's own $/cancel_request is written, after
// the drop handler, and after a response is found to match no call; for the
// notification whose overflow closes the connection, before the connection
// closes. So a peer can read an answer before fn has run for it: a program
// that counts refusals waits for fn, not for the answer
// (0046-MADR-sending-and-receiving-middleware.md, "Amendment, 2026-11-01").
func WithObserver(fn func(Observation)) ConnectionOption {
	return func(cfg *connectionConfig) {
		cfg.observer = fn
	}
}

// observe reports a message to the observer, when one is set, with its own
// copy of id. With none it returns at once, copying nothing.
func (c *Connection) observe(kind ObservationKind, method string, id json.RawMessage, size int, err error) {
	fn := c.observer
	if fn == nil {
		return
	}
	fn(Observation{Kind: kind, Method: method, ID: slices.Clone(id), Size: size, Err: err})
}

// answerObserved answers id with e, then reports the message, of size bytes,
// to the observer as kind.
func (c *Connection) answerObserved(kind ObservationKind, method string, id json.RawMessage, size int64, e *RequestError) {
	c.answer(id, e)
	c.observe(kind, method, id, int(size), e)
}

// requestErr is e as an error: nil for a nil *RequestError, never a typed
// nil.
func requestErr(e *RequestError) error {
	if e == nil {
		return nil
	}
	return e
}

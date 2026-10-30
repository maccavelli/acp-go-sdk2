package remote

import "errors"

// The causes a serve's context ends with, which context.Cause reports, and
// the error a handler's close returns for a serve that outlived the ending's
// last step (docs/decisions/0043-MADR-remote-http-and-websocket-transport.md,
// "Amended by the PLAN, 2026-10-29", 1).
var (
	// ErrDeleted is the cause when the client ended its Streamable HTTP
	// connection with DELETE.
	ErrDeleted = errors.New("remote: connection deleted by the client")

	// ErrDisconnected is the cause when the client went away: it closed its
	// socket, stopped answering pings, or left its Streamable HTTP connection
	// with no connection-scoped stream for 60 s.
	ErrDisconnected = errors.New("remote: client disconnected")

	// ErrHandlerClosed is the cause when the handler itself was closed.
	ErrHandlerClosed = errors.New("remote: handler closed")

	// ErrOverflow is the cause when the messages held for a client that was not
	// receiving them grew past the handler's limit.
	ErrOverflow = errors.New("remote: messages held for the client exceed the limit")

	// ErrAbandoned is returned by a handler's close when a serve had not
	// returned a grace after its context was cancelled and its pipes closed.
	// Its goroutine is left running, since Go cannot stop a goroutine.
	ErrAbandoned = errors.New("remote: serve abandoned")
)

package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	notificationQueueDrainTimeout = 5 * time.Second
	defaultMaxQueuedNotifications = 1024
	// defaultMaxMessageSize is the largest line receive accepts unless
	// WithMaxMessageSize sets another.
	defaultMaxMessageSize = 10 * 1024 * 1024
	// readBufferSize is the size of receive's bufio.Reader: a line up to it is
	// read in one piece (0030-MADR-record-initialize-on-arrival.md, the
	// amendment of 2026-10-21, B1 to B7).
	readBufferSize = 128 << 10
	jsonRPCVersion = "2.0"
	jsonNull       = "null"
	jsonErrorField = "error"
)

type anyMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *RequestError    `json:"error,omitempty"`
}

type queuedNotification struct {
	seq     uint64
	receipt uint64
	msg     *anyMessage
}

// receiptKey is the context key of a received request's or notification's
// receipt number: its place, from 1, in the order receive read the requests
// and notifications of the connection.
type receiptKey struct{}

// receiptOf returns ctx's receipt number, or 0 for a context receive did not
// make, such as a test's call of a handler. Generated code orders a prompt
// turn and a session/cancel by it
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 5 deviation).
func receiptOf(ctx context.Context) uint64 {
	if r, ok := ctx.Value(receiptKey{}).(uint64); ok {
		return r
	}
	return 0
}

// messageKey is the context key of the kind of message a handler's context
// was made for: a request, or a notification with its sequence number in the
// notification queue (0019-PLAN-behaviour-fixes.md, B1).
type messageKey struct{}

// messageMark is what messageKey holds.
type messageMark struct {
	notification bool
	seq          uint64
}

// markOf returns ctx's mark, and false for a context receive did not make,
// such as a test's call of a handler.
func markOf(ctx context.Context) (messageMark, bool) {
	m, ok := ctx.Value(messageKey{}).(messageMark)
	return m, ok
}

// sentAsNotification and sentAsRequest report whether receive made ctx for a
// notification or for a request; a context receive did not make is neither.
// The generated dispatch refuses a method sent as the other kind
// (0019-PLAN-behaviour-fixes.md, B12).
func sentAsNotification(ctx context.Context) bool {
	m, ok := markOf(ctx)
	return ok && m.notification
}

// IsNotification reports whether the connection made ctx, or the context ctx
// derives from, for a received notification, so a handler that receives both
// kinds, such as an ExtensionMethodHandler, can tell a notification from a
// request. It is false for a request's context and for one the connection
// did not make (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 4).
func IsNotification(ctx context.Context) bool { return sentAsNotification(ctx) }

func sentAsRequest(ctx context.Context) bool {
	m, ok := markOf(ctx)
	return ok && !m.notification
}

// decodeParams decodes a received message's params into p. Params left out,
// which JSON-RPC 2.0 allows and ACP's schema does not require, decode as an
// empty object, so a type with required keys refuses them by name
// (0019-PLAN-behaviour-fixes.md, B3).
func decodeParams(params json.RawMessage, p any) error {
	if len(params) == 0 {
		params = json.RawMessage("{}")
	}
	return json.Unmarshal(params, p)
}

// jsonCopy returns v encoded and decoded again: its JSON form, what a peer
// sees of it, sharing no memory with v. The side connections keep the record
// of initialize this way, so a handler's or a caller's values and the record
// never alias (0019-PLAN-behaviour-fixes.md, B13).
func jsonCopy[T any](v T) (T, error) {
	var out T
	b, err := json.Marshal(v)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(b, &out)
	return out, err
}

// inflightRequest is one received request whose handler is running, held by
// pointer so that a request removes only its own entry
// (0019-PLAN-behaviour-fixes.md, B11).
type inflightRequest struct{ cancel context.CancelCauseFunc }

type responseEnvelope struct {
	msg                   anyMessage
	notificationWatermark uint64
	// committed and commitErr are what the pending request's commit made of
	// its result, on the reader, before the envelope was handed over
	// (0030-MADR-record-initialize-on-arrival.md, O1).
	committed any
	commitErr error
}

type pendingResponse struct {
	ch chan responseEnvelope
	// commit, when set, checks and records a result as the reader reads it,
	// before it reads the next message (sendRequestCommit).
	commit func(result json.RawMessage) (any, error)
	// settle, when set, runs on the reader once it has judged the answer,
	// after commit for a result, before it reads the next message
	// (sendRequestCommit).
	settle func()
}

type cancelRequestParams struct {
	RequestID json.RawMessage `json:"requestId"`
}

// isPortableAbs reports whether p is absolute in POSIX or Windows form: it
// begins with "/", with a drive letter, a colon and a separator, or with two
// backslashes. It is the rule for a path the SDK sends, whose receiver's
// operating system the SDK cannot know; a path the SDK receives is checked
// with the host's filepath.IsAbs (0015-MADR-validate-absolute-protocol-paths.md, A4).
func isPortableAbs(p string) bool {
	switch {
	case strings.HasPrefix(p, "/"), strings.HasPrefix(p, `\\`):
		return true
	case len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/'):
		c := p[0] | 0x20
		return c >= 'a' && c <= 'z'
	}
	return false
}

// validator is a generated type's validate, which takes the rule for an
// absolute path.
type validator interface {
	validate(abs func(string) bool) error
}

// validateOutbound validates params of a generated type, a pointer or a
// value, with the rule for a path the SDK sends; params of any other type, an
// extension's raw JSON among them, pass (0015-MADR-validate-absolute-protocol-paths.md, E1).
func validateOutbound(params any) error {
	rv := reflect.ValueOf(params)
	switch {
	case !rv.IsValid():
		return nil
	case rv.Kind() == reflect.Pointer:
		if v, ok := params.(validator); ok && !rv.IsNil() {
			return v.validate(isPortableAbs)
		}
		return nil
	}
	ptr := reflect.New(rv.Type())
	ptr.Elem().Set(rv)
	if v, ok := reflect.TypeAssert[validator](ptr); ok {
		return v.validate(isPortableAbs)
	}
	return nil
}

// pendingRequest is one request a side answers itself when it cancels the
// prompt turn of the request's session.
type pendingRequest struct{ cancel context.CancelCauseFunc }

// pendingRequests holds, by session, the contexts of the requests a side
// answers itself when it cancels that session's prompt turn
// (0017-MADR-remediate-acp-v1-conformance-violations.md, Q1). The zero value
// is ready to use.
type pendingRequests struct {
	mu sync.Mutex
	m  map[string]map[*pendingRequest]struct{}
}

// track registers a request of session and returns its context and the
// function that removes it, which the caller defers.
func (p *pendingRequests) track(ctx context.Context, session string) (context.Context, func()) {
	rctx, cancel := context.WithCancelCause(ctx)
	r := &pendingRequest{cancel: cancel}
	p.mu.Lock()
	if p.m == nil {
		p.m = map[string]map[*pendingRequest]struct{}{}
	}
	if p.m[session] == nil {
		p.m[session] = map[*pendingRequest]struct{}{}
	}
	p.m[session][r] = struct{}{}
	p.mu.Unlock()
	return rctx, func() {
		p.mu.Lock()
		delete(p.m[session], r)
		if len(p.m[session]) == 0 {
			delete(p.m, session)
		}
		p.mu.Unlock()
		cancel(nil)
	}
}

// cancel cancels the context of each pending request of session with
// ErrPromptCancelled.
func (p *pendingRequests) cancel(session string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for r := range p.m[session] {
		r.cancel(ErrPromptCancelled)
	}
}

// answerUnlessCancelled runs call on its own goroutine and returns its result,
// unless ctx ends with a cause that is ErrPromptCancelled, before or after
// call returns: then it returns cancelled at once, and a result call returns
// later is discarded and logged at Debug. Another end of ctx, such as
// $/cancel_request, waits for call, as a handler is otherwise awaited
// (0017-MADR-remediate-acp-v1-conformance-violations.md, Q1).
func answerUnlessCancelled[T any](c *Connection, ctx context.Context, method string, call func(context.Context) (T, error), cancelled T) (T, error) { //nolint:revive // context-as-argument: the connection value stays the first parameter.
	type result struct {
		v   T
		err error
	}
	done := make(chan result, 1)
	c.goroutines.Go(func() {
		v, err := call(ctx)
		done <- result{v, err}
	})
	select {
	case r := <-done:
		if errors.Is(context.Cause(ctx), ErrPromptCancelled) {
			return cancelled, nil
		}
		return r.v, r.err
	case <-ctx.Done():
		if errors.Is(context.Cause(ctx), ErrPromptCancelled) {
			c.goroutines.Go(func() {
				r := <-done
				c.loggerOrDefault().Debug("handler returned after its request was answered cancelled; result discarded", "method", method, "err", r.err)
			})
			return cancelled, nil
		}
		r := <-done
		return r.v, r.err
	}
}

// addressable returns v when it is a pointer, and a pointer to a copy of it
// otherwise, so that methods with pointer receivers can be reached.
func addressable(v any) any {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || rv.Kind() == reflect.Pointer {
		return v
	}
	ptr := reflect.New(rv.Type())
	ptr.Elem().Set(rv)
	return ptr.Interface()
}

// MethodHandler handles one inbound JSON-RPC request or notification.
//
// Notifications are handled one at a time, in the order received, and a
// response is returned to its caller only after the notifications received
// before it are handled. A notification handler that sends a request on the
// same connection must send it under the handler's context, or one derived
// from it: such a request does not wait for the handler that sent it. Sent
// under an unrelated context, such as context.Background(), it waits for that
// handler, which is waiting for it (0019-PLAN-behaviour-fixes.md, B1).
//
// A handler that panics ends the process, as an unrecovered panic in any
// goroutine does. The connection does not recover it
// (0024-PLAN-behaviour-fixes.md, A1).
type MethodHandler func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError)

// Connection is a simple JSON-RPC 2.0 connection over line-delimited JSON.
//
// Its methods are safe for concurrent use. Messages sent from several
// goroutines are written whole, one at a time, in the order they reach the
// connection's writer (0024-PLAN-documentation.md, A5).
type Connection struct {
	w       io.Writer
	r       io.Reader
	handler MethodHandler

	mu sync.Mutex
	// promptOverlap is the agent side's prompt-overlap policy, which the
	// generated prompt-turn code reads (0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md, step 3).
	promptOverlap PromptOverlapPolicy
	// requestSlots holds a value for each inbound request whose handler is
	// running; it is nil when requests are unbounded, and its capacity is the
	// limit (0019-PLAN-api-decisions.md, B18).
	requestSlots chan struct{}
	// writes hands each message to the writer goroutine, which writes them one
	// at a time in the order it receives them (0019-PLAN-api-decisions.md, B16).
	writes chan writeRequest
	// closeOnce guards Close, and closedLocally records that this side ended
	// the connection (B17).
	closeOnce     sync.Once
	closedLocally atomic.Bool
	nextID        atomic.Uint64
	pending       map[string]*pendingResponse
	inflight      map[string]*inflightRequest
	// inflightReceipts holds the receipt of every received request whose
	// handler is running, a null id's included (0019-PLAN-behaviour-fixes.md,
	// B7).
	inflightReceipts     map[uint64]struct{}
	pendingCancelRequest []string
	cancelRequestSignal  chan struct{}

	// goroutines counts every goroutine the connection starts, each through
	// its Go, so Wait can wait for them all. The context.AfterFunc callbacks
	// are not counted: one on a caller's context can start after the count has
	// reached zero (0026-MADR-connection-end-cause-and-wait.md).
	goroutines sync.WaitGroup

	// endErr is why the connection ended, recorded once, by the first of
	// closeWith and shutdownReceive, before the context ends; Err returns it
	// (0026-MADR-connection-end-cause-and-wait.md).
	endErr atomic.Pointer[error]

	// ctx/cancel govern connection lifetime and are used for Done() and for canceling
	// callers waiting on responses when the peer disconnects.
	ctx    context.Context
	cancel context.CancelCauseFunc

	// inboundCtx/inboundCancel are used when invoking the inbound MethodHandler.
	// This ctx is intentionally kept alive long enough to process notifications
	// that were successfully received and queued just before a peer disconnect.
	// Otherwise, handlers that respect context cancellation may drop end-of-connection
	// messages that we already read off the wire.
	inboundCtx    context.Context
	inboundCancel context.CancelCauseFunc

	// inputDone is closed when the input ends cleanly, at end of file; the
	// requests received before it, which inboundRequests counts, are then
	// answered before the connection ends (0019-PLAN-issue-fixes.md, B23).
	inputDone       chan struct{}
	inboundRequests sync.WaitGroup

	// logger is read by the connection's goroutines, which start starts,
	// so it is set atomically: by WithLogger before they start, or by SetLogger
	// at any time.
	logger atomic.Pointer[slog.Logger]

	notifyMu sync.Mutex
	// notifyCond coordinates response-scoped waits for sequential notification processing.
	notifyCond *sync.Cond
	// invariant: completedNotificationSeq <= lastEnqueuedNotificationSeq.
	lastEnqueuedNotificationSeq uint64
	completedNotificationSeq    uint64

	// notificationQueue serializes notification processing to maintain order.
	// It is bounded to keep memory usage predictable.
	notificationQueue chan queuedNotification

	// maxMessageSize is the largest line receive accepts (WithMaxMessageSize).
	maxMessageSize int

	// notificationOverflow is what receive does with a notification that arrives
	// when notificationQueue is full.
	notificationOverflow NotificationOverflowPolicy
	// onNotificationDropped, if non-nil, is called for each notification dropped
	// under OverflowDropNewest. It runs on the receive goroutine.
	onNotificationDropped func(method string, totalDropped uint64)
	// droppedNotifications counts notifications dropped under OverflowDropNewest.
	droppedNotifications atomic.Uint64
	// refusedMessages counts received messages checkInbound refused.
	refusedMessages atomic.Uint64
	// rewrite and inbound are the consumer's hooks on received messages
	// (WithInboundRewrite, WithInboundValidation).
	rewrite func(ctx context.Context, method string, params any) error
	inbound func(ctx context.Context, method string, params any, err error) error
}

// ConnectionOption configures optional parameters for a Connection.
//
// Options apply in the order given, so the last of a repeated option wins. A
// value n <= 0 keeps the value already set for WithMaxQueuedNotifications and
// WithMaxMessageSize, the default unless an earlier option set one, but sets
// WithMaxConcurrentRequests back to no limit. A nil option panics
// (0024-PLAN-documentation.md, A10).
type ConnectionOption func(*connectionConfig)

type connectionConfig struct {
	maxQueuedNotifications int
	maxMessageSize         int
	notificationOverflow   NotificationOverflowPolicy
	onNotificationDropped  func(method string, totalDropped uint64)
	logger                 *slog.Logger
	rewrite                func(ctx context.Context, method string, params any) error
	inbound                func(ctx context.Context, method string, params any, err error) error
	maxConcurrentRequests  int
	promptOverlap          PromptOverlapPolicy
}

// WithMaxQueuedNotifications sets the maximum number of queued notifications
// before the connection is closed with a notification queue overflow error.
// The default is 1024. Increase this value when multiplexing many concurrent
// sessions over a single connection, as the combined notification rate may
// exceed the processing speed of the single-threaded notification consumer.
func WithMaxQueuedNotifications(n int) ConnectionOption {
	return func(cfg *connectionConfig) {
		if n > 0 {
			cfg.maxQueuedNotifications = n
		}
	}
}

// PromptOverlapPolicy selects what an AgentSideConnection does with a
// session/prompt for a session whose prompt turn is still running. ACP does
// not define it (0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md, issue #4).
type PromptOverlapPolicy int

const (
	// PromptOverlapSupersede cancels the running turn, with a cause that wraps
	// ErrPromptCancelled, and runs the new prompt. A prompt received before
	// the running turn's is cancelled at once instead. This is the default.
	PromptOverlapSupersede PromptOverlapPolicy = iota

	// PromptOverlapRefuse answers the new prompt at once, -32603 with data
	// {"error": "a prompt turn is already running for this session",
	// "sessionId": <id>}, and does not call Prompt; the running turn goes on.
	// A turn that session/cancel or session/close has cancelled no longer
	// counts, so a prompt sent after a cancel runs.
	PromptOverlapRefuse

	// PromptOverlapConcurrent calls Prompt for every prompt, so the agent
	// queues, steers or refuses in its own handler. session/cancel and
	// session/close cancel every running turn of the session received before
	// them.
	PromptOverlapConcurrent
)

// WithPromptOverlapPolicy sets what an AgentSideConnection does with a
// session/prompt for a session whose turn is running. The default is
// PromptOverlapSupersede, and any value this package does not define is
// treated as the default. Other connections ignore it.
func WithPromptOverlapPolicy(p PromptOverlapPolicy) ConnectionOption {
	return func(cfg *connectionConfig) {
		cfg.promptOverlap = p
	}
}

// WithMaxConcurrentRequests bounds the inbound requests whose handlers run at
// once. When n are running, a further request is answered at once with -32603
// and data {"error": "too many concurrent requests", "limit": n}, its handler
// does not run, and the refusal is logged at Debug. A request stops counting
// when its handler returns, before its answer is written. The reader never
// waits for a handler to end: a running handler may be waiting for a response
// only the reader can deliver. Notifications and $/cancel_request are not
// counted. n <= 0, the default, leaves requests unbounded
// (0019-PLAN-api-decisions.md, B18).
func WithMaxConcurrentRequests(n int) ConnectionOption {
	return func(cfg *connectionConfig) {
		cfg.maxConcurrentRequests = n
	}
}

// WithMaxMessageSize sets the largest message, one line in bytes without its
// newline, the connection reads. A longer line is read through its newline,
// discarded, logged, and answered -32600 with "id": null, and reading goes on.
// The default is 10 MiB; n <= 0 keeps it
// (0017-MADR-remediate-acp-v1-conformance-violations.md, J1).
func WithMaxMessageSize(n int) ConnectionOption {
	return func(cfg *connectionConfig) {
		if n > 0 {
			cfg.maxMessageSize = n
		}
	}
}

// NotificationOverflowPolicy selects what a Connection does with an inbound
// notification that arrives when its notification queue is full.
// More policies may be added in the future.
type NotificationOverflowPolicy int

const (
	// OverflowCloseConnection closes the connection when a notification cannot
	// be queued, as Close does, its transport included, and in-flight requests
	// fail. This is the default.
	OverflowCloseConnection NotificationOverflowPolicy = iota

	// OverflowDropNewest discards the notification that could not be queued and
	// keeps the connection open. Notifications that are delivered keep their
	// relative order; the dropped one is never delivered and is not retried.
	//
	// Choosing it means accepting loss, so observe it: install a handler with
	// WithNotificationDropHandler, or read DroppedNotifications.
	//
	// Two consequences to weigh first:
	//   - A request waits for the notifications queued before its response
	//     arrived. A dropped notification was never queued, so a request can
	//     return although a notification that preceded its response on the
	//     wire was dropped.
	//   - The policy applies to every inbound notification. On an agent-side
	//     connection that includes session/cancel; dropping one leaves the
	//     agent working on a turn its client has abandoned.
	OverflowDropNewest
)

// WithNotificationOverflowPolicy sets what the connection does with a
// notification that arrives when its notification queue is full. The default
// is OverflowCloseConnection, and any value this package does not define is
// treated as the default.
func WithNotificationOverflowPolicy(p NotificationOverflowPolicy) ConnectionOption {
	return func(cfg *connectionConfig) {
		cfg.notificationOverflow = p
	}
}

// WithNotificationDropHandler registers fn to be called once for each
// notification dropped under OverflowDropNewest, with the notification's method
// and the number dropped on this connection so far, including this one.
//
// fn runs on the goroutine that reads from the peer, so no further messages are
// read until it returns: it must return quickly and must not block. In
// particular it must not send a request on the connection, whose response could
// only be read by the goroutine fn is holding.
func WithNotificationDropHandler(fn func(method string, totalDropped uint64)) ConnectionOption {
	return func(cfg *connectionConfig) {
		cfg.onNotificationDropped = fn
	}
}

// WithLogger directs the connection's internal diagnostics to l from the
// start, before any message is read. It is the race-free way to set a logger:
// a connection's goroutines begin reading the peer inside the constructor, so
// a diagnostic can be logged before a later SetLogger call returns. A nil l
// keeps the default logger.
func WithLogger(l *slog.Logger) ConnectionOption {
	return func(cfg *connectionConfig) {
		cfg.logger = l
	}
}

// WithInboundRewrite installs a hook that sees every request, notification
// and response the connection receives, after it is decoded and before it is
// validated: params is a pointer to the decoded value, such as
// *NewSessionRequest, which the hook may change, for example to resolve a
// bare MCP server command with exec.LookPath or to translate a path from
// another host. The SDK then validates what the hook leaves, so a handler sees
// the rewritten value, not what the peer sent. A non-nil error refuses the
// message as WithInboundValidation's does. A message that does not decode
// reaches no hook (0015-MADR-validate-absolute-protocol-paths.md, H1). An
// extension message's params is a *json.RawMessage
// (0019-PLAN-api-decisions.md, B14).
func WithInboundRewrite(fn func(ctx context.Context, method string, params any) error) ConnectionOption {
	return func(cfg *connectionConfig) {
		cfg.rewrite = fn
	}
}

// ChainRewrites returns one rewrite hook that runs fns in order and stops at
// the first error, which it returns. A connection keeps one
// WithInboundRewrite hook, the last given, so hooks that should all run are
// chained into one (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func ChainRewrites(fns ...func(ctx context.Context, method string, params any) error) func(ctx context.Context, method string, params any) error {
	return func(ctx context.Context, method string, params any) error {
		for _, fn := range fns {
			if err := fn(ctx, method, params); err != nil {
				return err
			}
		}
		return nil
	}
}

// WithInboundValidation installs a hook that decides the outcome of every
// request, notification and response the connection receives, after the SDK
// validates it: err is the SDK's result, nil when the message is valid, and
// a *ValidationError, wrapped, when it is not. The hook returns the error to
// act on, or nil to accept the message; returning nil for a violation accepts
// a message the ACP standard forbids, which the application then answers
// for. It may also refuse a valid message with its own error. A request is
// answered with a *RequestError as it is, and with any other error as
// invalid params (-32602); a notification with an error is dropped and
// logged; a response's error is returned to the caller. params is the same
// pointer the rewrite hook saw, and this hook must not change it
// (0015-MADR-validate-absolute-protocol-paths.md, H1).
func WithInboundValidation(fn func(ctx context.Context, method string, params any, err error) error) ConnectionOption {
	return func(cfg *connectionConfig) {
		cfg.inbound = fn
	}
}

// NewConnection starts a line-delimited JSON-RPC connection.
// peerInput receives bytes written to the peer. peerOutput supplies bytes read from the peer.
//
// Reading starts before NewConnection returns, so handler may be called
// before then: it must not read a variable assigned from the result.
//
// A nil handler answers every request -32601 and drops every notification.
// NewConnection panics, naming the argument, when peerInput or peerOutput is
// nil, as http.Handle does for a nil handler
// (0024-PLAN-behaviour-fixes.md, A1).
func NewConnection(handler MethodHandler, peerInput io.Writer, peerOutput io.Reader, opts ...ConnectionOption) *Connection {
	if peerInput == nil {
		panic("acp: NewConnection: nil peerInput")
	}
	if peerOutput == nil {
		panic("acp: NewConnection: nil peerOutput")
	}
	c := buildConnection(handler, peerInput, peerOutput, opts...)
	c.start()
	return c
}

// AgentConnAware is implemented by an Agent that keeps its connection, to send
// the client requests and notifications. NewAgentSideConnection calls
// SetAgentConnection once, after building the connection and before reading
// starts, so a handler always finds it set. The method must only store the
// connection: it is not running yet, and a send from the method would block
// the constructor (0019-MADR-remediate-debugging-pass-findings.md,
// decision 1).
type AgentConnAware interface {
	SetAgentConnection(conn *AgentSideConnection)
}

// ClientConnAware is implemented by a Client that keeps its connection, as
// AgentConnAware is by an Agent: NewClientSideConnection calls
// SetClientConnection once, before reading starts, and the method must only
// store the connection.
type ClientConnAware interface {
	SetClientConnection(conn *ClientSideConnection)
}

// buildConnection builds a connection and starts none of its goroutines. The
// side connections assign the result before calling start, so their handlers
// never see it unset (0018-MADR-start-side-connections-after-assignment.md).
func buildConnection(handler MethodHandler, peerInput io.Writer, peerOutput io.Reader, opts ...ConnectionOption) *Connection {
	cfg := connectionConfig{
		maxQueuedNotifications: defaultMaxQueuedNotifications,
		maxMessageSize:         defaultMaxMessageSize,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	inboundCtx, inboundCancel := context.WithCancelCause(context.Background())
	c := &Connection{
		w:                   peerInput,
		r:                   peerOutput,
		handler:             handler,
		writes:              make(chan writeRequest),
		pending:             make(map[string]*pendingResponse),
		inflight:            make(map[string]*inflightRequest),
		inflightReceipts:    make(map[uint64]struct{}),
		cancelRequestSignal: make(chan struct{}, 1),
		ctx:                 ctx,
		cancel:              cancel,
		inboundCtx:          inboundCtx,
		inboundCancel:       inboundCancel,
		inputDone:           make(chan struct{}),
		notificationQueue:   make(chan queuedNotification, cfg.maxQueuedNotifications),
		maxMessageSize:      cfg.maxMessageSize,

		notificationOverflow:  cfg.notificationOverflow,
		onNotificationDropped: cfg.onNotificationDropped,
		rewrite:               cfg.rewrite,
		inbound:               cfg.inbound,
	}
	if cfg.maxConcurrentRequests > 0 {
		c.requestSlots = make(chan struct{}, cfg.maxConcurrentRequests)
	}
	c.promptOverlap = cfg.promptOverlap
	if cfg.logger != nil {
		c.logger.Store(cfg.logger)
	}
	c.notifyCond = sync.NewCond(&c.notifyMu)
	return c
}

// start starts the connection's goroutines, reading included. It is called
// once, after the connection is built.
func (c *Connection) start() {
	// The end of the connection wakes every notification wait; the
	// registration lasts the connection's life
	// (0019-PLAN-idioms-performance.md, D8).
	context.AfterFunc(c.ctx, c.wakeNotificationWaits)
	c.goroutines.Go(c.writeLoop)
	c.goroutines.Go(c.sendCancelRequests)
	c.goroutines.Go(c.receive)
	c.goroutines.Go(c.processNotifications)
}

// wakeNotificationWaits wakes every wait on the notification sequence, so each
// checks again whether its context or the connection has ended.
func (c *Connection) wakeNotificationWaits() {
	c.notifyMu.Lock()
	c.notifyCond.Broadcast()
	c.notifyMu.Unlock()
}

// writeRequest is one message for the writer goroutine and where it reports
// the write's result.
type writeRequest struct {
	b    []byte
	done chan error
}

// writeLoop writes each message sendMessage hands it, until the connection
// ends. A write that has begun runs to its end, so its sender may already
// have returned on its context (0019-PLAN-api-decisions.md, B16).
func (c *Connection) writeLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case req := <-c.writes:
			_, err := c.w.Write(req.b)
			req.done <- err
		}
	}
}

// endError is the error a send or a call meets once the connection has
// ended: ErrConnectionClosed when this side closed it, wrapping the overflow
// that made it close; otherwise ErrPeerDisconnected, wrapping how the peer
// ended it (0019-PLAN-api-decisions.md, B17).
func (c *Connection) endError() error {
	if c.closedLocally.Load() {
		if cause := context.Cause(c.ctx); cause != nil && !errors.Is(cause, ErrConnectionClosed) {
			return fmt.Errorf("%w: %w", ErrConnectionClosed, cause)
		}
		return ErrConnectionClosed
	}
	if cause := context.Cause(c.ctx); cause != nil && !errors.Is(cause, ErrPeerDisconnected) {
		return fmt.Errorf("%w: %w", ErrPeerDisconnected, cause)
	}
	return ErrPeerDisconnected
}

// recordEnd records why the connection ended, kind wrapping cause unless cause
// already is kind, unless an end is recorded already: the first end wins, so
// a Close after the peer's end leaves the peer's
// (0026-MADR-connection-end-cause-and-wait.md, E4).
func (c *Connection) recordEnd(kind, cause error) {
	err := kind
	if cause != nil && !errors.Is(cause, kind) {
		err = fmt.Errorf("%w: %w", kind, cause)
	}
	c.endErr.CompareAndSwap(nil, &err)
}

// Err returns nil until the connection ends, and then why it ended, the
// same each time: ErrConnectionClosed when this side ended it, by Close or
// by a notification overflow, wrapping ErrNotificationQueueOverflow for the
// latter; otherwise ErrPeerDisconnected, wrapping how the peer ended it,
// io.EOF at the end of its output or the read's error. It is non-nil exactly
// when Done is closed. A call made after the end returns its own error:
// after the peer's end of output, a request's wraps ErrPeerDisconnected,
// a late Close or not, since the input has ended, while a notification's
// wraps ErrConnectionClosed once a late Close has run
// (0026-MADR-connection-end-cause-and-wait.md).
func (c *Connection) Err() error {
	if c.ctx.Err() == nil {
		return nil
	}
	if err := c.endErr.Load(); err != nil {
		return *err
	}
	return c.endError()
}

// Wait blocks until the connection has ended and every goroutine it
// started has returned, its handlers' included, and then returns Err. It
// does not end the connection: call Close first to stop it. A reader that
// Close cannot close, one that does not implement io.Closer, keeps Wait
// waiting until its Read returns; so does a handler that ignores its
// context (0026-MADR-connection-end-cause-and-wait.md).
func (c *Connection) Wait() error {
	// The end comes first, as the contract says. Today the wait group alone
	// implies it, since writeLoop and sendCancelRequests return only once the
	// connection's context ends; this wait keeps the contract should that
	// change (0026-PLAN-connection-end-cause-and-wait.md, step 3).
	<-c.Done()
	c.goroutines.Wait()
	return c.Err()
}

// Close ends the connection: its context ends with ErrConnectionClosed, so
// Done is closed, every call waiting for a response returns, and every later
// send fails without writing; every handler's context ends, a request's and a
// notification's alike (0023-MADR-connection-lifetime-and-test-leaks.md, C1);
// and its reader and writer are closed when they implement io.Closer, which
// also ends a read in progress. It returns their errors joined, and nil when
// it was called before (0019-PLAN-api-decisions.md, B17).
func (c *Connection) Close() error {
	return c.closeWith(ErrConnectionClosed)
}

// closeWith is Close, ending the context with cause. It marks the connection
// closed locally before the context ends, so every waiter reads endError
// after the mark. It ends the notification handlers' context too, which the
// input's drain would otherwise end only once the reader stops
// (0023-MADR-connection-lifetime-and-test-leaks.md, C1).
func (c *Connection) closeWith(cause error) error {
	var err error
	c.closeOnce.Do(func() {
		c.closedLocally.Store(true)
		c.recordEnd(ErrConnectionClosed, cause)
		c.cancel(cause)
		c.inboundCancel(cause)
		var closers []io.Closer
		for _, x := range []any{c.r, c.w} {
			cl, ok := x.(io.Closer)
			if !ok || slices.ContainsFunc(closers, func(have io.Closer) bool { return sameValue(have, cl) }) {
				continue
			}
			closers = append(closers, cl)
		}
		var errs []error
		for _, cl := range closers {
			errs = append(errs, cl.Close())
		}
		err = errors.Join(errs...)
	})
	return err
}

// sameValue reports whether a and b are the same pointer, such as one
// *net.TCPConn passed as both the reader and the writer. Only pointers are
// compared, which never panics.
func sameValue(a, b any) bool {
	ta := reflect.TypeOf(a)
	return ta != nil && ta == reflect.TypeOf(b) && ta.Kind() == reflect.Pointer && a == b
}

// DroppedNotifications reports how many inbound notifications this connection
// has dropped under OverflowDropNewest. It is always zero under the default
// policy.
func (c *Connection) DroppedNotifications() uint64 {
	return c.droppedNotifications.Load()
}

// RefusedMessages reports how many received messages this connection has
// refused after decoding them: a request, notification, extension message or
// response that the WithInboundRewrite hook refused, that broke the schema
// and the WithInboundValidation hook did not accept, or that what initialize
// agreed does not allow. A message held until initialize ends is counted
// once, if it is then refused. A line that does not decode, or a method the
// side does not handle, is answered as before and not counted
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md, question 3).
func (c *Connection) RefusedMessages() uint64 {
	return c.refusedMessages.Load()
}

// dropNotification records a notification discarded under OverflowDropNewest.
// It runs on the receive goroutine, after notifyMu has been released.
func (c *Connection) dropNotification(method string) {
	total := c.droppedNotifications.Add(1)
	c.loggerOrDefault().Debug("failed to queue notification; dropping it", "method", method, "capacity", cap(c.notificationQueue), "queued", len(c.notificationQueue), "dropped_total", total)
	if fn := c.onNotificationDropped; fn != nil {
		fn(method, total)
	}
}

// SetLogger installs a logger used for internal connection diagnostics.
// If unset, logs are written via the default logger. It is safe to call while
// the connection runs, but diagnostics logged before it returns go to the
// previous logger; use WithLogger to set one from the start.
func (c *Connection) SetLogger(l *slog.Logger) { c.logger.Store(l) }

func (c *Connection) loggerOrDefault() *slog.Logger {
	if l := c.logger.Load(); l != nil {
		return l
	}
	return slog.Default()
}

const (
	maxCanonicalJSONRPCIDKeyLen   = 4096
	maxCanonicalJSONRPCIDAbsExp10 = 4096
	maxPendingCancelRequests      = 1024
)

var (
	errInvalidJSONRPCNumericID  = errors.New("invalid json-rpc numeric id")
	errJSONRPCNumericIDTooLarge = errors.New("json-rpc numeric id too large")
)

func canonicalJSONRPCIDKey(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	// A plain integer is its own key, the one the decoder gives it
	// (0019-PLAN-idioms-performance.md, D5).
	if isPlainIDInteger(trimmed) {
		return string(trimmed), nil
	}
	return decodeJSONRPCIDKey(trimmed)
}

// isPlainIDInteger reports whether b is 0, or 1 to 19 ASCII digits whose
// first is not 0: an integer the decoder keeps as it is written.
func isPlainIDInteger(b []byte) bool {
	if len(b) == 0 || len(b) > 19 || (b[0] == '0' && len(b) > 1) {
		return false
	}
	for _, d := range b {
		if !isASCIIDigit(d) {
			return false
		}
	}
	return true
}

// decodeJSONRPCIDKey is the key of trimmed, an id without surrounding space,
// read by the JSON decoder: canonicalJSONRPCIDKey without its fast path.
func decodeJSONRPCIDKey(trimmed []byte) (string, error) {
	if len(trimmed) == 0 {
		return "", errors.New("empty json-rpc id")
	}

	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()

	var id any
	if err := dec.Decode(&id); err != nil {
		return "", err
	}

	// Ensure the id contains a single JSON value.
	var trailing any
	if err := dec.Decode(&trailing); err == nil {
		return "", errors.New("invalid json-rpc id: trailing data")
	} else if !errors.Is(err, io.EOF) {
		return "", err
	}

	switch v := id.(type) {
	case nil:
		return jsonNull, nil
	case string:
		canon, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(canon), nil
	case json.Number:
		return canonicalJSONRPCNumericIDKey(v)
	default:
		return "", errors.New("json-rpc id must be string, number, or null")
	}
}

func canonicalJSONRPCNumericIDKey(v json.Number) (string, error) {
	raw := strings.TrimSpace(v.String())
	if raw == "" {
		return "", errInvalidJSONRPCNumericID
	}

	negative, digits, exp10, err := parseJSONRPCNumericID(raw)
	if err != nil {
		return "", err
	}

	return formatCanonicalJSONRPCNumericID(negative, digits, exp10)
}

func parseJSONRPCNumericID(raw string) (negative bool, digits string, exp10 int, err error) {
	i := 0
	if raw[i] == '-' {
		negative = true
		i++
		if i >= len(raw) {
			return false, "", 0, errInvalidJSONRPCNumericID
		}
	}

	intStart := i
	switch {
	case raw[i] == '0':
		i++
		if i < len(raw) && isASCIIDigit(raw[i]) {
			return false, "", 0, errInvalidJSONRPCNumericID
		}
	case raw[i] >= '1' && raw[i] <= '9':
		for i < len(raw) && isASCIIDigit(raw[i]) {
			i++
		}
	default:
		return false, "", 0, errInvalidJSONRPCNumericID
	}
	intDigits := raw[intStart:i]

	fracDigits := ""
	if i < len(raw) && raw[i] == '.' {
		i++
		fracStart := i
		for i < len(raw) && isASCIIDigit(raw[i]) {
			i++
		}
		if fracStart == i {
			return false, "", 0, errInvalidJSONRPCNumericID
		}
		fracDigits = raw[fracStart:i]
	}

	exponent := 0
	if i < len(raw) && (raw[i] == 'e' || raw[i] == 'E') {
		i++
		if i >= len(raw) {
			return false, "", 0, errInvalidJSONRPCNumericID
		}

		exponentSign := 1
		if raw[i] == '+' || raw[i] == '-' {
			if raw[i] == '-' {
				exponentSign = -1
			}
			i++
			if i >= len(raw) {
				return false, "", 0, errInvalidJSONRPCNumericID
			}
		}

		exponentStart := i
		for i < len(raw) && isASCIIDigit(raw[i]) {
			i++
		}
		if exponentStart == i {
			return false, "", 0, errInvalidJSONRPCNumericID
		}

		exponentMagnitude, parseErr := parseBoundedInt(raw[exponentStart:i], maxCanonicalJSONRPCIDAbsExp10)
		if parseErr != nil {
			return false, "", 0, parseErr
		}
		exponent = exponentSign * exponentMagnitude
	}

	if i != len(raw) {
		return false, "", 0, errInvalidJSONRPCNumericID
	}

	digits = strings.TrimLeft(intDigits+fracDigits, "0")
	if digits == "" {
		return false, "", 0, nil
	}
	if len(digits) > maxCanonicalJSONRPCIDKeyLen {
		return false, "", 0, errJSONRPCNumericIDTooLarge
	}

	exp10 = exponent - len(fracDigits)
	return negative, digits, exp10, nil
}

func parseBoundedInt(raw string, upper int) (int, error) {
	if raw == "" {
		return 0, errInvalidJSONRPCNumericID
	}

	value := 0
	for i := range len(raw) {
		if !isASCIIDigit(raw[i]) {
			return 0, errInvalidJSONRPCNumericID
		}
		digit := int(raw[i] - '0')
		if value > (upper-digit)/10 {
			return 0, errJSONRPCNumericIDTooLarge
		}
		value = value*10 + digit
	}
	return value, nil
}

func isASCIIDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func formatCanonicalJSONRPCNumericID(negative bool, digits string, exp10 int) (string, error) {
	for digits != "" && digits[len(digits)-1] == '0' {
		digits = digits[:len(digits)-1]
		exp10++
	}

	if digits == "" {
		return "0", nil
	}

	sign := ""
	if negative {
		sign = "-"
	}

	if exp10 >= 0 {
		if exp10 > maxCanonicalJSONRPCIDKeyLen-len(digits) {
			return "", errJSONRPCNumericIDTooLarge
		}
		result := digits + strings.Repeat("0", exp10)
		if sign != "" {
			result = sign + result
		}
		if len(result) > maxCanonicalJSONRPCIDKeyLen+len(sign) {
			return "", errJSONRPCNumericIDTooLarge
		}
		return result, nil
	}

	scale := -exp10
	if scale > maxCanonicalJSONRPCIDKeyLen {
		return "", errJSONRPCNumericIDTooLarge
	}

	if len(digits) > scale {
		intPart := digits[:len(digits)-scale]
		fracPart := digits[len(digits)-scale:]
		if len(intPart)+1+len(fracPart) > maxCanonicalJSONRPCIDKeyLen {
			return "", errJSONRPCNumericIDTooLarge
		}
		result := intPart + "." + fracPart
		if sign != "" {
			result = sign + result
		}
		return result, nil
	}

	leadingZeros := scale - len(digits)
	if leadingZeros > maxCanonicalJSONRPCIDKeyLen-len(digits)-2 {
		return "", errJSONRPCNumericIDTooLarge
	}
	result := "0." + strings.Repeat("0", leadingZeros) + digits
	if sign != "" {
		result = sign + result
	}
	return result, nil
}

// readLine reads one line from r, without its newline. A line longer than max
// bytes is read through its newline and discarded, and tooLong is true. err is
// r's error, io.EOF at the end; a last line without a newline comes with it.
func readLine(r *bufio.Reader, maxBytes int) (line []byte, tooLong bool, err error) {
	for {
		chunk, err := r.ReadSlice('\n')
		if !tooLong {
			line = append(line, chunk...)
			if len(bytes.TrimSuffix(line, []byte("\n"))) > maxBytes {
				line, tooLong = nil, true
			}
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if tooLong {
			return nil, true, err
		}
		return bytes.TrimRight(line, "\r\n"), false, err
	}
}

func (c *Connection) receive() {
	r := bufio.NewReaderSize(c.r, readBufferSize)
	var received uint64
	cause := errPeerEOF
	for {
		line, tooLong, err := readLine(r, c.maxMessageSize)
		switch {
		case tooLong:
			c.loggerOrDefault().Warn("incoming message exceeds the size limit; answered and skipped", "limit", c.maxMessageSize)
			c.answer(nil, NewInvalidRequest(map[string]any{jsonErrorField: fmt.Sprintf("message exceeds %d bytes", c.maxMessageSize)}))
		case len(bytes.TrimSpace(line)) > 0:
			if !c.receiveLine(line, &received) {
				return
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				c.drainInput()
			} else {
				cause = err
			}
			break
		}
	}
	c.shutdownReceive(cause)
}

// drainInput ends the connection's input cleanly: the requests it is
// waiting on fail at once, as do any it sends later, since no answer can
// arrive, while the requests already received run, under contexts that stay
// live, and their answers are written. It returns when they have, after
// notificationQueueDrainTimeout, or when Close ends the connection; the
// handlers still running are then cancelled as the connection ends
// (0019-PLAN-issue-fixes.md, B23).
func (c *Connection) drainInput() {
	close(c.inputDone)
	answered := make(chan struct{})
	c.goroutines.Go(func() {
		c.inboundRequests.Wait()
		close(answered)
	})
	bound := time.NewTimer(notificationQueueDrainTimeout)
	defer bound.Stop()
	select {
	case <-answered:
	case <-bound.C:
		c.loggerOrDefault().Debug("requests still running when the input's drain ended", "timeout", notificationQueueDrainTimeout)
	case <-c.ctx.Done():
	}
}

// errInputEnded is the cause of the error a request meets when the
// connection's input has ended, so no answer can arrive
// (0019-PLAN-issue-fixes.md, B23).
var errInputEnded = fmt.Errorf("%w: end of input", ErrPeerDisconnected)

// inputEndedError is the error such a request returns.
func inputEndedError() *RequestError {
	return NewInternalError(map[string]any{jsonErrorField: "input ended before a response"}).withCause(errInputEnded)
}

// inputEnded reports whether the connection's input has ended cleanly.
func (c *Connection) inputEnded() bool {
	select {
	case <-c.inputDone:
		return true
	default:
		return false
	}
}

// envelope is a received JSON-RPC object's members, raw: an absent member is
// nil, and a null one the bytes null.
type envelope struct {
	JSONRPC json.RawMessage `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  json.RawMessage `json:"method"`
	Params  json.RawMessage `json:"params"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

// isVersion reports whether raw is the JSON string "2.0".
func isVersion(raw json.RawMessage) bool {
	var v string
	return raw != nil && json.Unmarshal(raw, &v) == nil && v == jsonRPCVersion
}

// validID reports whether raw is a JSON-RPC id: a string, a number or null.
func validID(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	switch c := raw[0]; {
	case c == '"', c == '-', c >= '0' && c <= '9':
		return true
	}
	return string(raw) == jsonNull
}

// requestFault returns why env is not a valid Request object, and the id to
// answer it with: its own when the id is valid, nil (null) otherwise.
func requestFault(env *envelope) (json.RawMessage, string) {
	var id json.RawMessage
	switch {
	case env.ID != nil && !validID(env.ID):
		return nil, "id must be a string, a number or null"
	case env.ID != nil:
		id = env.ID
	}
	switch {
	case !isVersion(env.JSONRPC):
		return id, `jsonrpc must be "2.0"`
	case env.Method == nil:
		return id, "method is missing"
	case env.Method[0] != '"':
		return id, "method must be a string"
	case env.Params != nil && string(env.Params) != jsonNull && env.Params[0] != '{' && env.Params[0] != '[':
		return id, "params must be an object, an array or null"
	}
	return id, ""
}

// receiveLine handles one non-blank line as JSON-RPC 2.0 requires: invalid
// JSON is answered -32700 and anything else that is not a valid Request object
// -32600, with the request's id when it has a valid one and null otherwise; a
// response is never answered; a valid notification is handled and never
// answered (0017-MADR-remediate-acp-v1-conformance-violations.md, J1). It
// returns false when the connection has shut down.
func (c *Connection) receiveLine(line []byte, received *uint64) bool {
	// A message's content is never logged: it may hold prompt text or file
	// contents. Its size, the fault and a valid id are
	// (0019-PLAN-behaviour-fixes.md, B15).
	if !json.Valid(line) {
		c.loggerOrDefault().Warn("failed to parse incoming message", "size", len(line))
		c.answer(nil, NewParseError(nil))
		return true
	}
	line = bytes.TrimSpace(line)
	var env envelope
	if line[0] != '{' || json.Unmarshal(line, &env) != nil {
		c.loggerOrDefault().Warn("incoming message is not a JSON-RPC object", "size", len(line))
		c.answer(nil, NewInvalidRequest(map[string]any{jsonErrorField: "not a JSON-RPC object"}))
		return true
	}
	if env.Method == nil && (env.Result != nil || env.Error != nil || c.isPending(env.ID)) {
		c.receiveResponse(&env, line)
		return true
	}
	id, fault := requestFault(&env)
	if fault != "" {
		c.loggerOrDefault().Warn("invalid request", "fault", fault, "size", len(line), "id", string(id))
		c.answer(id, NewInvalidRequest(map[string]any{jsonErrorField: fault}))
		return true
	}
	msg := anyMessage{JSONRPC: jsonRPCVersion, Params: env.Params}
	// ACP's schema lets an envelope carry "params": null, which is read as
	// params left out (0019-MADR-remediate-debugging-pass-findings.md,
	// decision 13).
	if string(msg.Params) == jsonNull {
		msg.Params = nil
	}
	if err := json.Unmarshal(env.Method, &msg.Method); err != nil {
		c.answer(id, NewInvalidRequest(map[string]any{jsonErrorField: "method must be a string"}))
		return true
	}
	if env.ID != nil {
		msg.ID = &id
	}

	// Handle $/cancel_request notifications synchronously so cancellations take effect
	// immediately and do not participate in notification ordering.
	if msg.ID == nil && msg.Method == ProtocolMethodCancelRequest {
		c.handleCancelRequest(&msg)
		return true
	}

	// A request over the limit is answered at once and its handler never
	// runs; the reader does not wait for a slot, since a running handler may
	// be waiting for a response only the reader delivers
	// (0019-PLAN-api-decisions.md, B18).
	if msg.ID != nil && c.requestSlots != nil {
		select {
		case c.requestSlots <- struct{}{}:
		default:
			limit := cap(c.requestSlots)
			c.loggerOrDefault().Debug("refused a request over the concurrency limit", "method", msg.Method, "limit", limit)
			c.answer(id, NewInternalError(map[string]any{jsonErrorField: "too many concurrent requests", "limit": limit}))
			return true
		}
	}

	*received++
	if msg.ID != nil {
		base := context.WithValue(context.WithValue(c.ctx, receiptKey{}, *received), messageKey{}, messageMark{})
		reqCtx, cancel := context.WithCancelCause(base)
		// A null id names no single request, so $/cancel_request cannot
		// reach it, and it is not registered. A request that reuses the id
		// of one still running takes its entry, and each removes only its
		// own (0019-PLAN-behaviour-fixes.md, B11).
		idKey := ""
		own := &inflightRequest{cancel: cancel}
		receipt := *received
		c.mu.Lock()
		c.inflightReceipts[receipt] = struct{}{}
		c.mu.Unlock()
		if string(*msg.ID) != jsonNull {
			var err error
			if idKey, err = canonicalJSONRPCIDKey(*msg.ID); err != nil {
				c.loggerOrDefault().Warn("failed to canonicalize inbound request id", "err", err, "size", len(*msg.ID))
				idKey = string(*msg.ID)
			}
			c.mu.Lock()
			_, reused := c.inflight[idKey]
			c.inflight[idKey] = own
			c.mu.Unlock()
			if reused {
				c.loggerOrDefault().Debug("received a request whose id is still in flight", "id", string(*msg.ID))
			}
		}

		m := msg
		c.inboundRequests.Add(1)
		c.goroutines.Go(func() {
			defer c.inboundRequests.Done()
			release := func() {}
			if c.requestSlots != nil {
				release = sync.OnceFunc(func() { <-c.requestSlots })
			}
			defer func() {
				release()
				c.mu.Lock()
				if idKey != "" && c.inflight[idKey] == own {
					delete(c.inflight, idKey)
				}
				delete(c.inflightReceipts, receipt)
				c.mu.Unlock()
				own.cancel(nil)
			}()
			c.handleInbound(reqCtx, &m, release)
		})
		return true
	}

	// Queue the notification for sequential processing. The sequence number marks
	// the response-scoped barrier boundary for requests that observe later responses.
	m := msg
	c.notifyMu.Lock()
	c.lastEnqueuedNotificationSeq++
	seq := c.lastEnqueuedNotificationSeq
	select {
	case c.notificationQueue <- queuedNotification{seq: seq, receipt: *received, msg: &m}:
		c.notifyMu.Unlock()
	default:
		if c.lastEnqueuedNotificationSeq != seq {
			c.notifyMu.Unlock()
			panic("notification sequence advanced while receive goroutine was queueing")
		}
		c.lastEnqueuedNotificationSeq--
		// invariant: completedNotificationSeq never exceeds the highest accepted enqueue.
		if c.completedNotificationSeq > c.lastEnqueuedNotificationSeq {
			c.notifyMu.Unlock()
			panic("completed notification sequence exceeded enqueued notification sequence")
		}
		c.notifyMu.Unlock()
		if c.notificationOverflow == OverflowDropNewest {
			// The rollback above released this notification's sequence
			// number, so the next one reuses it and ordering stays
			// contiguous. The reader must not block here: responses are
			// dispatched on this goroutine too.
			c.dropNotification(m.Method)
			return true
		}
		c.loggerOrDefault().Error("failed to queue notification; closing connection", "err", ErrNotificationQueueOverflow, "capacity", cap(c.notificationQueue), "queued", len(c.notificationQueue))
		// The policy closes the connection, its transport included
		// (0019-PLAN-api-decisions.md, B17).
		if err := c.closeWith(ErrNotificationQueueOverflow); err != nil {
			c.loggerOrDefault().Debug("closing the transport after a notification overflow", "err", err)
		}
		c.shutdownReceive(ErrNotificationQueueOverflow)
		return false
	}
	return true
}

// keepCancel keeps receipt, a session/cancel's or a session close's, as the
// session's kept cancel, for a prompt received before it whose turn is not
// stored yet. Such a prompt is a request still in flight, so a kept receipt
// that no in-flight request is older than can no longer apply: each keep
// first removes those (0019-PLAN-behaviour-fixes.md, B5 and B7). A receipt of
// 0, from a context receive did not make, is not kept. The caller holds the
// lock that guards kept.
func (c *Connection) keepCancel(kept map[string]uint64, session string, receipt uint64) {
	if receipt == 0 {
		return
	}
	oldest, inFlight := c.oldestInflight()
	for k, r := range kept {
		if !inFlight || r <= oldest {
			delete(kept, k)
		}
	}
	if receipt > kept[session] {
		kept[session] = receipt
	}
}

// oldestInflight returns the smallest receipt of a received request whose
// handler is running, and false when there is none.
func (c *Connection) oldestInflight() (uint64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var oldest uint64
	for r := range c.inflightReceipts {
		if oldest == 0 || r < oldest {
			oldest = r
		}
	}
	return oldest, oldest != 0
}

// isPending reports whether raw is the id of a request this connection waits
// on, so that a message carrying it and no method is a response.
func (c *Connection) isPending(raw json.RawMessage) bool {
	if raw == nil || !validID(raw) {
		return false
	}
	key, err := canonicalJSONRPCIDKey(raw)
	if err != nil {
		key = string(raw)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pending[key] != nil
}

// receiveResponse delivers a response to the call it answers. A response that
// breaks JSON-RPC, with jsonrpc other than "2.0", both or neither of result and
// error, or an error that is not an error object, fails the call. A response is
// never answered, so two peers cannot answer each other's errors.
func (c *Connection) receiveResponse(env *envelope, line []byte) {
	if env.ID == nil || !validID(env.ID) || string(env.ID) == jsonNull {
		c.loggerOrDefault().Warn("received a response without a request id", "size", len(line))
		return
	}
	id := env.ID
	msg := anyMessage{JSONRPC: jsonRPCVersion, ID: &id}
	var fault string
	switch {
	case !isVersion(env.JSONRPC):
		fault = `jsonrpc must be "2.0"`
	case env.Result != nil && env.Error != nil:
		fault = "both result and error"
	case env.Result == nil && env.Error == nil:
		fault = "neither result nor error"
	case env.Error != nil:
		var re RequestError
		if env.Error[0] != '{' || json.Unmarshal(env.Error, &re) != nil {
			fault = "error is not an error object"
		} else {
			msg.Error = &re
		}
	default:
		msg.Result = env.Result
	}
	if fault != "" {
		c.loggerOrDefault().Warn("malformed response", "fault", fault, "size", len(line), "id", string(id))
		msg.Error = NewInternalError(map[string]any{jsonErrorField: "malformed response: " + fault})
	}
	c.handleResponse(&msg)
}

// answer writes an error response with id, or with "id": null when id is nil.
func (c *Connection) answer(id json.RawMessage, e *RequestError) {
	if id == nil {
		id = json.RawMessage(jsonNull)
	}
	if err := c.sendMessage(context.Background(), anyMessage{ID: &id, Error: e}); err != nil {
		c.loggerOrDefault().Debug("failed to answer an invalid message", "err", err)
	}
}

func (c *Connection) shutdownReceive(cause error) {
	if cause == nil {
		cause = errors.New("connection closed")
	}

	// First, signal disconnect to callers waiting on responses.
	c.recordEnd(ErrPeerDisconnected, cause)
	c.cancel(cause)

	// Then close the notification queue so already-received messages can drain.
	// IMPORTANT: Do not block this receive goroutine waiting for the drain to complete;
	// notification handlers may legitimately block until their context is canceled.
	close(c.notificationQueue)

	c.notifyMu.Lock()
	finalEnqueuedSeq := c.lastEnqueuedNotificationSeq
	if c.completedNotificationSeq > finalEnqueuedSeq {
		c.notifyMu.Unlock()
		panic("completed notification sequence exceeded final enqueued sequence during shutdown")
	}
	c.notifyMu.Unlock()

	// Cancel inboundCtx after notifications finish, but ensure we don't leak forever if a
	// handler blocks waiting for cancellation.
	c.goroutines.Go(func() {
		c.waitForNotificationDrain(finalEnqueuedSeq, notificationQueueDrainTimeout)
		c.inboundCancel(cause)
	})

	c.loggerOrDefault().Debug("connection closed", "cause", cause.Error())
}

// processNotifications processes notifications sequentially to maintain order.
// It terminates when notificationQueue is closed (e.g. on disconnect in receive()).
func (c *Connection) processNotifications() {
	for queued := range c.notificationQueue {
		ctx := context.WithValue(context.WithValue(c.inboundCtx, receiptKey{}, queued.receipt),
			messageKey{}, messageMark{notification: true, seq: queued.seq})
		c.handleInbound(ctx, queued.msg, nil)

		c.notifyMu.Lock()
		expectedSeq := c.completedNotificationSeq + 1
		if queued.seq != expectedSeq {
			c.notifyMu.Unlock()
			panic("notification sequence completed out of order")
		}
		c.completedNotificationSeq = queued.seq
		if c.completedNotificationSeq > c.lastEnqueuedNotificationSeq {
			c.notifyMu.Unlock()
			panic("completed notification sequence exceeded enqueued notification sequence")
		}
		c.notifyCond.Broadcast()
		c.notifyMu.Unlock()
	}
}

func (c *Connection) handleResponse(msg *anyMessage) {
	idStr, err := canonicalJSONRPCIDKey(*msg.ID)
	if err != nil {
		c.loggerOrDefault().Warn("failed to canonicalize response id", "err", err, "size", len(*msg.ID))
		idStr = string(*msg.ID)
	}

	c.mu.Lock()
	pr := c.pending[idStr]
	if pr != nil {
		delete(c.pending, idStr)
	}
	c.mu.Unlock()
	if pr == nil {
		// A late answer to a call its caller gave up on is expected.
		c.loggerOrDefault().Debug("received a response to no pending request", "id", string(*msg.ID))
	}

	if pr != nil {
		c.notifyMu.Lock()
		watermark := c.lastEnqueuedNotificationSeq
		if c.completedNotificationSeq > watermark {
			c.notifyMu.Unlock()
			panic("completed notification sequence exceeded response watermark")
		}
		c.notifyMu.Unlock()
		env := responseEnvelope{msg: *msg, notificationWatermark: watermark}
		// The commit runs before the reader reads on, so every later message
		// is judged by what it recorded (0030-MADR-record-initialize-on-arrival.md, O1).
		if pr.commit != nil && msg.Error == nil {
			env.committed, env.commitErr = pr.commit(msg.Result)
		}
		if pr.settle != nil {
			pr.settle()
		}
		pr.ch <- env
	}
}

func (c *Connection) handleCancelRequest(msg *anyMessage) {
	var p cancelRequestParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		c.loggerOrDefault().Warn("failed to parse "+ProtocolMethodCancelRequest+" params", "err", err)
		return
	}
	if len(bytes.TrimSpace(p.RequestID)) == 0 {
		c.loggerOrDefault().Warn("received " + ProtocolMethodCancelRequest + " without requestId")
		return
	}

	idKey, err := canonicalJSONRPCIDKey(p.RequestID)
	if err != nil {
		c.loggerOrDefault().Warn("failed to canonicalize "+ProtocolMethodCancelRequest+" requestId", "err", err, "size", len(p.RequestID))
		idKey = string(p.RequestID)
	}

	c.mu.Lock()
	r := c.inflight[idKey]
	c.mu.Unlock()
	if r == nil {
		return
	}

	r.cancel(context.Canceled)
}

// handleInbound runs the handler for req and answers it when it is a request.
// handled, when not nil, is called as the handler returns, before the answer
// is written.
func (c *Connection) handleInbound(ctx context.Context, req *anyMessage, handled func()) {
	res := anyMessage{JSONRPC: jsonRPCVersion}

	// copy ID if present
	if req.ID != nil {
		res.ID = req.ID
	}
	if c.handler == nil {
		if req.ID != nil {
			res.Error = NewMethodNotFound(req.Method)
			if err := c.sendMessage(context.Background(), res); err != nil {
				return
			}
		}
		return
	}

	result, err := c.handler(ctx, req.Method, req.Params)
	if handled != nil {
		handled()
	}
	if req.ID == nil {
		// Notification: no response is sent; log handler errors to surface decode failures.
		if err != nil {
			// Per ACP, unknown extension notifications should be ignored.
			if err.Code == -32601 && strings.HasPrefix(req.Method, "_") {
				return
			}
			// The dispatch refuses a notification the peer got wrong -32601 or
			// -32602: the peer's fault, at Warn; any other error is this side's
			// (0024-PLAN-behaviour-fixes.md, A4).
			level := slog.LevelError
			if err.Code == -32601 || err.Code == -32602 {
				level = slog.LevelWarn
			}
			c.loggerOrDefault().Log(ctx, level, "failed to handle notification", "method", req.Method, "err", err)
		}
		return
	}
	if err != nil {
		res.Error = err
	} else {
		// marshal result
		b, mErr := json.Marshal(result)
		if mErr != nil {
			res.Error = NewInternalError(map[string]any{jsonErrorField: mErr.Error()})
		} else {
			res.Result = b
		}
	}
	if err := c.sendMessage(context.Background(), res); err != nil {
		return
	}
}

// sendMessage hands msg to the writer goroutine and waits for its write. Once
// the connection has ended it returns endError and writes nothing; when ctx
// ends first it returns the context's cause, and a write that has begun runs
// to its end, so the message may still reach the peer
// (0019-PLAN-api-decisions.md, B16 and B17).
func (c *Connection) sendMessage(ctx context.Context, msg anyMessage) error {
	_, err := c.sendMessageSent(ctx, msg)
	return err
}

// sendMessageSent is sendMessage, and also reports whether msg was handed to
// the writer goroutine: when it was not, nothing was written; when it was,
// the write runs to its end, so msg may have reached the peer whatever the
// error (0019-PLAN-issue-fixes.md, B22).
func (c *Connection) sendMessageSent(ctx context.Context, msg anyMessage) (sent bool, err error) {
	select {
	case <-c.ctx.Done():
		return false, c.endError()
	default:
	}
	msg.JSONRPC = jsonRPCVersion
	b, err := json.Marshal(msg)
	if err != nil {
		return false, err
	}
	req := writeRequest{b: append(b, '\n'), done: make(chan error, 1)}
	select {
	case c.writes <- req:
	case <-ctx.Done():
		return false, context.Cause(ctx)
	case <-c.ctx.Done():
		return false, c.endError()
	}
	select {
	case err := <-req.done:
		return true, err
	case <-ctx.Done():
		// A write that finished as the context ended is reported, not the
		// context's cause, which select may pick when both are ready.
		select {
		case err := <-req.done:
			return true, err
		default:
			return true, context.Cause(ctx)
		}
	}
}

// SendRequest sends a JSON-RPC request and returns a typed result.
// For methods that do not return a result, use SendRequestNoResult instead.
// Unlike the typed methods of the side connections, it does not check what
// initialize negotiated: it sends any method, before initialize too
// (0017-MADR-remediate-acp-v1-conformance-violations.md, G1).
func SendRequest[T any](c *Connection, ctx context.Context, method string, params any) (T, error) { //nolint:revive // context-as-argument: the connection value stays the first parameter.
	return sendRequest[T](c, ctx, method, params, nil)
}

// gateFunc checks a typed message against what initialize agreed; the
// generated side connections pass theirs, and the raw transport passes nil
// (0017-MADR-remediate-acp-v1-conformance-violations.md, G1).
type gateFunc func(method string, params any) error

// typedGate is gate as a gateFunc for a result of type *T, which sendRequest
// gives its gate. A result of another type is an error naming both types
// (0019-PLAN-idioms-performance.md, D12).
func typedGate[T any](gate func(method string, result *T) error) gateFunc {
	return func(method string, result any) error {
		r, ok := result.(*T)
		if !ok {
			return fmt.Errorf("%s: result of type %T, not %T", method, result, r)
		}
		return gate(method, r)
	}
}

// sendRequest is SendRequest with the gate its result is checked by, nil for
// none.
func sendRequest[T any](c *Connection, ctx context.Context, method string, params any, gate gateFunc) (T, error) { //nolint:revive // context-as-argument: the connection value stays the first parameter.
	result, _, err := sendRequestSent[T](c, ctx, method, params, gate)
	return result, err
}

// sendRequestSent is sendRequest, and also reports whether the request was
// handed to the writer goroutine, so that it may have reached the peer
// (0019-PLAN-issue-fixes.md, B22).
func sendRequestSent[T any](c *Connection, ctx context.Context, method string, params any, gate gateFunc) (result T, sent bool, err error) { //nolint:revive // context-as-argument: the connection value stays the first parameter.
	resp, sent, err := c.exchange(ctx, method, params, &pendingResponse{ch: make(chan responseEnvelope, 1)})
	if err != nil {
		return result, sent, err
	}
	if err := c.waitNotificationsUpTo(ctx, resp.notificationWatermark); err != nil {
		return result, true, err
	}

	if resp.msg.Error != nil {
		return result, true, resp.msg.Error
	}

	if len(resp.msg.Result) > 0 {
		if err := json.Unmarshal(resp.msg.Result, &result); err != nil {
			return result, true, NewInternalError(map[string]any{jsonErrorField: err.Error()})
		}
		if err := c.checkInbound(ctx, method, &result, gate); err != nil {
			var zero T
			return zero, true, fmt.Errorf("%s response: %w", method, err)
		}
	}
	return result, true, nil
}

// sendRequestCommit is sendRequest for a request whose result the reader
// checks and records as it reads it, before it reads the next message: on
// the reader, the result is decoded and checked as sendRequestSent checks it,
// with ctx and gate, and then given to commit, whose error is the call's. It
// returns the value the reader checked, decoding nothing itself, and returns
// a committed result even when the wait for earlier notifications then ends,
// so that what it returns and what commit recorded agree
// (0030-MADR-record-initialize-on-arrival.md, O1;
// 0030-PLAN-record-initialize-on-arrival.md, the deviation during step 2).
// settled runs once the answer has been judged, on the reader, after the
// commit for a result and at once for an error answer, or, when no answer
// will come, on the caller before it returns; it ends what waits for the
// answer, and so it runs before the wait for earlier notifications, which
// may be among what waits (0030-MADR-record-initialize-on-arrival.md,
// question 2).
func sendRequestCommit[T any](c *Connection, ctx context.Context, method string, params any, gate gateFunc, commit func(result *T) error, settled func()) (T, error) { //nolint:revive // context-as-argument: the connection value stays the first parameter.
	pr := &pendingResponse{ch: make(chan responseEnvelope, 1), settle: settled, commit: func(raw json.RawMessage) (any, error) {
		var result T
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &result); err != nil {
				return result, NewInternalError(map[string]any{jsonErrorField: err.Error()})
			}
			if err := c.checkInbound(ctx, method, &result, gate); err != nil {
				var zero T
				return zero, fmt.Errorf("%s response: %w", method, err)
			}
		}
		err := commit(&result)
		return result, err
	}}
	var zero T
	resp, _, err := c.exchange(ctx, method, params, pr)
	if err != nil {
		// No answer was judged, and none will be.
		settled()
		return zero, err
	}
	werr := c.waitNotificationsUpTo(ctx, resp.notificationWatermark)
	// committed is nil only for an error response, which runs no commit.
	var result T
	if v, ok := resp.committed.(T); ok {
		result = v
	}
	switch {
	case resp.msg.Error == nil && resp.commitErr == nil:
		return result, nil
	case werr != nil:
		return zero, werr
	case resp.msg.Error != nil:
		return zero, resp.msg.Error
	default:
		return result, resp.commitErr
	}
}

// initializeAttempt is a client's initialize in flight: done is closed once,
// when the attempt's answer has been judged or no answer will come. A
// received message held for want of a record waits for it
// (0030-MADR-record-initialize-on-arrival.md, question 2).
type initializeAttempt struct {
	done chan struct{}
	once sync.Once
}

// newInitializeAttempt returns an attempt in flight.
func newInitializeAttempt() *initializeAttempt {
	return &initializeAttempt{done: make(chan struct{})}
}

// end marks the attempt over; a later call does nothing.
func (a *initializeAttempt) end() { a.once.Do(func() { close(a.done) }) }

// holdUntilInitialized judges a received message with gate. When gate refuses
// it only for want of a record and attempt is in flight, it waits until the
// attempt ends, and judges it again, so that a message the peer sends before
// its initialize answer is judged by that answer; with no attempt, or when
// the connection ends first, done, the refusal stands
// (0030-MADR-record-initialize-on-arrival.md, question 2).
func holdUntilInitialized(gate gateFunc, attempt *initializeAttempt, done <-chan struct{}, method string, params any) error {
	err := gate(method, params)
	if attempt == nil || !errors.Is(err, ErrNotInitialized) {
		return err
	}
	select {
	case <-attempt.done:
		return gate(method, params)
	case <-done:
		return err
	}
}

// exchange sends the request for method, registered as pr, and returns its
// answer, and whether the request was handed to the writer goroutine, so
// that it may have reached the peer (0019-PLAN-issue-fixes.md, B22).
func (c *Connection) exchange(ctx context.Context, method string, params any, pr *pendingResponse) (responseEnvelope, bool, error) {
	// A request whose context has already ended is not written
	// (0019-PLAN-behaviour-fixes.md, B9).
	if ctx.Err() != nil {
		return responseEnvelope{}, false, toReqErr(context.Cause(ctx))
	}
	// No answer can arrive once the input has ended.
	if c.inputEnded() {
		return responseEnvelope{}, false, inputEndedError()
	}

	msg, idKey, err := c.prepareRequest(method, params)
	if err != nil {
		return responseEnvelope{}, false, err
	}

	c.mu.Lock()
	c.pending[idKey] = pr
	c.mu.Unlock()

	sent, err := c.sendMessageSent(ctx, msg)
	if err != nil {
		// A write that has begun runs to its end (writeLoop), so the answer
		// may already have been read.
		if resp, ok := c.claimedAnswer(pr, idKey); ok {
			return resp, true, nil
		}
		c.cleanupPending(idKey)
		if ctx.Err() != nil {
			c.sendCancelRequest(idKey)
		}
		return responseEnvelope{}, sent, toReqErr(err)
	}

	resp, err := c.waitForResponse(ctx, pr, idKey)
	return resp, true, err
}

// claimedAnswer is, for a request with a commit, the answer the reader has
// already taken from the pending requests, waited for while its commit runs,
// so that a caller that stops waiting after that returns what was recorded;
// a request still pending is removed, and nothing is committed for it. It
// reports false for a request without a commit, leaving it pending
// (0030-PLAN-record-initialize-on-arrival.md, the deviation during step 2).
// Only the reader and the caller remove a pending request, and the reader
// always hands over what it took.
func (c *Connection) claimedAnswer(pr *pendingResponse, idKey string) (responseEnvelope, bool) {
	if pr.commit == nil {
		return responseEnvelope{}, false
	}
	c.mu.Lock()
	pending := c.pending[idKey] == pr
	if pending {
		delete(c.pending, idKey)
	}
	c.mu.Unlock()
	if pending {
		return responseEnvelope{}, false
	}
	return <-pr.ch, true
}

func (c *Connection) prepareRequest(method string, params any) (anyMessage, string, error) {
	id := c.nextID.Add(1)
	idRaw, err := json.Marshal(id)
	if err != nil {
		return anyMessage{}, "", NewInternalError(map[string]any{jsonErrorField: err.Error()})
	}

	msg := anyMessage{
		JSONRPC: jsonRPCVersion,
		ID:      (*json.RawMessage)(&idRaw),
		Method:  method,
	}
	if msg.Params, err = encodeParams(params); err != nil {
		return msg, "", err
	}
	return msg, string(idRaw), nil
}

// encodeParams validates and encodes a message's params: nil for none, and
// for params that encode as null, which are left out; anything that is not an
// object or an array is refused, as JSON-RPC 2.0 requires a structured value
// (0019-PLAN-behaviour-fixes.md, B10).
func encodeParams(params any) (json.RawMessage, error) {
	if params == nil {
		return nil, nil
	}
	if err := validateOutbound(params); err != nil {
		return nil, err
	}
	b, err := json.Marshal(params)
	switch {
	case err != nil:
		return nil, NewInvalidParams(map[string]any{jsonErrorField: err.Error()})
	case string(b) == jsonNull:
		return nil, nil
	case b[0] != '{' && b[0] != '[':
		return nil, NewInvalidParams(map[string]any{jsonErrorField: "params must encode as a JSON object or array"})
	}
	return b, nil
}

func (c *Connection) sendCancelRequests() {
	for {
		select {
		case <-c.Done():
			return
		case <-c.cancelRequestSignal:
			for {
				c.mu.Lock()
				if len(c.pendingCancelRequest) == 0 {
					c.mu.Unlock()
					break
				}
				idKey := c.pendingCancelRequest[0]
				c.pendingCancelRequest = c.pendingCancelRequest[1:]
				c.mu.Unlock()

				requestID := json.RawMessage(append([]byte(nil), idKey...))
				if err := c.SendNotification(context.Background(), ProtocolMethodCancelRequest, cancelRequestParams{RequestID: requestID}); err != nil {
					c.loggerOrDefault().Debug("failed to send "+ProtocolMethodCancelRequest, "err", err)
				}
			}
		}
	}
}

func (c *Connection) sendCancelRequest(idKey string) {
	if strings.TrimSpace(idKey) == "" {
		return
	}

	select {
	case <-c.Done():
		return
	default:
	}

	queueFull := false
	c.mu.Lock()
	if len(c.pendingCancelRequest) >= maxPendingCancelRequests {
		queueFull = true
	} else {
		c.pendingCancelRequest = append(c.pendingCancelRequest, idKey)
	}
	c.mu.Unlock()

	if queueFull {
		c.loggerOrDefault().Debug("dropping "+ProtocolMethodCancelRequest+" due to full queue", "queue_len", maxPendingCancelRequests)
		return
	}

	select {
	case c.cancelRequestSignal <- struct{}{}:
	default:
	}
}

func (c *Connection) waitForResponse(ctx context.Context, pr *pendingResponse, idKey string) (responseEnvelope, error) {
	ended := func() *RequestError {
		// A connection that ends after its input ended reports the end of
		// input, which select may otherwise lose to Done when both are
		// ready (0019-PLAN-issue-fixes.md, deviation after the close).
		if c.inputEnded() {
			return inputEndedError()
		}
		return NewInternalError(map[string]any{jsonErrorField: "connection ended before response"}).withCause(c.endError())
	}

	// answered returns an answer already read, which is delivered in
	// preference to the end of input or of the connection that select may
	// pick when both are ready (0019-PLAN-issue-fixes.md, B23).
	answered := func() (responseEnvelope, bool) {
		select {
		case resp := <-pr.ch:
			return resp, true
		default:
			return responseEnvelope{}, false
		}
	}

	claimed := func() (responseEnvelope, bool) { return c.claimedAnswer(pr, idKey) }

	select {
	case resp := <-pr.ch:
		return resp, nil
	case <-c.inputDone:
		if resp, ok := answered(); ok {
			return resp, nil
		}
		if resp, ok := claimed(); ok {
			return resp, nil
		}
		c.cleanupPending(idKey)
		return responseEnvelope{}, inputEndedError()
	case <-ctx.Done():
		if resp, ok := claimed(); ok {
			return resp, nil
		}
		// If the connection dropped at the same time, prefer reporting peer disconnect
		// and avoid queueing a best-effort cancel notification to a dead peer.
		select {
		case <-c.Done():
			c.cleanupPending(idKey)
			return responseEnvelope{}, ended()
		default:
		}

		c.sendCancelRequest(idKey)
		c.cleanupPending(idKey)

		// An ended context has a cause: context.Cause gives its Err
		// when none was set, and Err is non-nil once Done is closed.
		return responseEnvelope{}, toReqErr(context.Cause(ctx))
	case <-c.Done():
		if resp, ok := answered(); ok {
			return resp, nil
		}
		if resp, ok := claimed(); ok {
			return resp, nil
		}
		c.cleanupPending(idKey)
		return responseEnvelope{}, ended()
	}
}

func (c *Connection) waitNotificationsUpTo(ctx context.Context, target uint64) error {
	if target == 0 {
		return nil
	}

	ended := func() *RequestError {
		return NewInternalError(map[string]any{jsonErrorField: "connection ended while waiting for pre-response notifications"}).withCause(c.endError())
	}

	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	if target > c.lastEnqueuedNotificationSeq {
		panic("response watermark exceeded last enqueued notification sequence")
	}
	// A request sent under the context of a notification whose handler is
	// still running would wait for that handler, which may be the caller:
	// the wait stops before it. Notifications before it have completed, the
	// queue running one at a time (0019-PLAN-behaviour-fixes.md, B1).
	if m, ok := markOf(ctx); ok && m.notification && c.completedNotificationSeq < m.seq && target >= m.seq {
		target = m.seq - 1
	}

	// The end of ctx wakes this wait (0019-PLAN-idioms-performance.md, D8).
	stop := context.AfterFunc(ctx, c.wakeNotificationWaits)
	defer stop()

	for c.completedNotificationSeq < target {
		if c.completedNotificationSeq > c.lastEnqueuedNotificationSeq {
			panic("completed notification sequence exceeded enqueued notification sequence while waiting")
		}

		select {
		case <-c.Done():
			return ended()
		default:
		}
		select {
		case <-ctx.Done():
			select {
			case <-c.Done():
				return ended()
			default:
			}
			return toReqErr(context.Cause(ctx))
		default:
		}

		c.notifyCond.Wait()
	}
	return nil
}

func (c *Connection) waitForNotificationDrain(target uint64, timeout time.Duration) {
	if target == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	if target > c.lastEnqueuedNotificationSeq {
		panic("drain target exceeded last enqueued notification sequence")
	}

	// The timeout wakes this wait (0019-PLAN-idioms-performance.md, D8).
	stop := context.AfterFunc(ctx, c.wakeNotificationWaits)
	defer stop()

	for c.completedNotificationSeq < target {
		if c.completedNotificationSeq > c.lastEnqueuedNotificationSeq {
			panic("completed notification sequence exceeded enqueued notification sequence during drain")
		}
		if ctx.Err() != nil {
			return
		}
		c.notifyCond.Wait()
	}
}

func (c *Connection) cleanupPending(idKey string) {
	c.mu.Lock()
	delete(c.pending, idKey)
	c.mu.Unlock()
}

// SendRequestNoResult sends a JSON-RPC request that returns no result payload.
// Like SendRequest, it does not check what initialize negotiated.
func (c *Connection) SendRequestNoResult(ctx context.Context, method string, params any) error {
	if ctx.Err() != nil {
		return toReqErr(context.Cause(ctx))
	}
	if c.inputEnded() {
		return inputEndedError()
	}
	msg, idKey, err := c.prepareRequest(method, params)
	if err != nil {
		return err
	}

	pr := &pendingResponse{ch: make(chan responseEnvelope, 1)}
	c.mu.Lock()
	c.pending[idKey] = pr
	c.mu.Unlock()

	if err := c.sendMessage(ctx, msg); err != nil {
		c.cleanupPending(idKey)
		if ctx.Err() != nil {
			c.sendCancelRequest(idKey)
		}
		return toReqErr(err)
	}

	resp, err := c.waitForResponse(ctx, pr, idKey)
	if err != nil {
		return err
	}
	if err := c.waitNotificationsUpTo(ctx, resp.notificationWatermark); err != nil {
		return err
	}

	if resp.msg.Error != nil {
		return resp.msg.Error
	}
	return nil
}

// SendNotification sends a JSON-RPC notification. It does not wait for a response.
// Like SendRequest, it does not check what initialize negotiated.
func (c *Connection) SendNotification(ctx context.Context, method string, params any) error {
	select {
	case <-ctx.Done():
		return NewInternalError(map[string]any{jsonErrorField: ctx.Err().Error()}).withCause(context.Cause(ctx))
	default:
	}

	msg, err := c.prepareNotification(method, params)
	if err != nil {
		return err
	}

	if err := c.sendMessage(ctx, msg); err != nil {
		return toReqErr(err)
	}
	return nil
}

// checkResult returns a handler's result to send, after validating it as
// sent: a result that fails is not sent, the peer gets -32603 naming the type
// and property, and the connection logs it (0015-MADR-validate-absolute-protocol-paths.md, E1).
func (c *Connection) checkResult(method string, result any, gate gateFunc) (any, *RequestError) {
	err := validateOutbound(result)
	if err == nil && gate != nil {
		err = gate(method, addressable(result))
	}
	if err != nil {
		c.loggerOrDefault().Error("handler result fails validation; not sent", "method", method, "err", err)
		return nil, NewInternalError(map[string]any{jsonErrorField: err.Error()})
	}
	return result, nil
}

// checkInbound runs the receiving order on a decoded message: the rewrite hook,
// the SDK's validation with the host's rule for an absolute path, then the
// validation hook with that result. It returns the error to act on, nil to
// accept (0015-MADR-validate-absolute-protocol-paths.md, item 5). A non-nil
// gate checks the message against what initialize agreed; the hook rules on
// the validation error first and, only if it accepts that, on the gate's, so
// accepting one never accepts the other
// (0017-MADR-remediate-acp-v1-conformance-violations.md, G1). Each refusal
// adds one to RefusedMessages; every received message is checked once here
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md, question 3).
func (c *Connection) checkInbound(ctx context.Context, method string, params any, gate gateFunc) error {
	err := c.judgeInbound(ctx, method, params, gate)
	if err != nil {
		c.refusedMessages.Add(1)
	}
	return err
}

// judgeInbound is checkInbound's judgement, without its count: the rewrite
// hook, validation and the gate, then the validation hook.
func (c *Connection) judgeInbound(ctx context.Context, method string, params any, gate gateFunc) error {
	if c.rewrite != nil {
		if err := c.rewrite(ctx, method, params); err != nil {
			return err
		}
	}
	var err, gateErr error
	if v, ok := params.(validator); ok {
		err = v.validate(filepath.IsAbs)
	}
	if gate != nil {
		gateErr = gate(method, params)
	}
	if c.inbound == nil {
		if err != nil {
			return err
		}
		return gateErr
	}
	if err != nil {
		if err = c.inbound(ctx, method, params, err); err != nil || gateErr == nil {
			return err
		}
	}
	return c.inbound(ctx, method, params, gateErr)
}

// inboundError is the answer to a received request that checkInbound refused: a
// *RequestError as it is, any other error as invalid params.
func inboundError(err error) *RequestError {
	if re, ok := errors.AsType[*RequestError](err); ok {
		return re
	}
	// A method the record does not allow, or any before initialize, is not
	// available: -32601. A property or variant it does not allow is a bad
	// parameter: -32602 (0017-MADR-remediate-acp-v1-conformance-violations.md, G1).
	ce, isCE := errors.AsType[*CapabilityError](err)
	if errors.Is(err, ErrNotInitialized) || (isCE && ce.Field == "" && ce.Variant == "") {
		return &RequestError{Code: -32601, Message: "Method not found", Data: map[string]any{jsonErrorField: err.Error()}}
	}
	return NewInvalidParams(map[string]any{jsonErrorField: err.Error()})
}

func (c *Connection) prepareNotification(method string, params any) (anyMessage, error) {
	msg := anyMessage{
		JSONRPC: jsonRPCVersion,
		Method:  method,
	}
	var err error
	msg.Params, err = encodeParams(params)
	return msg, err
}

// Done returns a channel that is closed when the connection ends: when the
// peer disconnects or the input stream is closed, when Close is called, or
// when OverflowCloseConnection closes it. Once it is closed every send fails
// without writing.
//
// When the input ends cleanly, at end of file, the requests already received
// are answered first: the requests this side is waiting on fail at once, as
// do any it sends later, wrapping ErrPeerDisconnected; the handlers already
// running keep their contexts and their answers are written, for at most
// five seconds, after which they are cancelled; then Done is closed. A read
// that fails otherwise ends the connection at once
// (0019-PLAN-issue-fixes.md, B23). Err then says why it ended, and Wait
// returns once its goroutines have.
func (c *Connection) Done() <-chan struct{} {
	return c.ctx.Done()
}

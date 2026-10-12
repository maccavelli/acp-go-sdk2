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
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	notificationQueueDrainTimeout = 5 * time.Second
	defaultMaxQueuedNotifications = 1024
	jsonRPCVersion                = "2.0"
	jsonNull                      = "null"
	jsonErrorField                = "error"
)

var errNotificationQueueOverflow = errors.New("notification queue overflow")

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

type responseEnvelope struct {
	msg                   anyMessage
	notificationWatermark uint64
}

type pendingResponse struct {
	ch chan responseEnvelope
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
	if v, ok := ptr.Interface().(validator); ok {
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
	go func() {
		v, err := call(ctx)
		done <- result{v, err}
	}()
	select {
	case r := <-done:
		if errors.Is(context.Cause(ctx), ErrPromptCancelled) {
			return cancelled, nil
		}
		return r.v, r.err
	case <-ctx.Done():
		if errors.Is(context.Cause(ctx), ErrPromptCancelled) {
			go func() {
				r := <-done
				c.loggerOrDefault().Debug("handler returned after its request was answered cancelled; result discarded", "method", method, "err", r.err)
			}()
			return cancelled, nil
		}
		r := <-done
		return r.v, r.err
	}
}

// MethodHandler handles one inbound JSON-RPC request or notification.
type MethodHandler func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError)

// Connection is a simple JSON-RPC 2.0 connection over line-delimited JSON.
type Connection struct {
	w       io.Writer
	r       io.Reader
	handler MethodHandler

	mu                   sync.Mutex
	writeMu              sync.Mutex
	nextID               atomic.Uint64
	pending              map[string]*pendingResponse
	inflight             map[string]context.CancelCauseFunc
	pendingCancelRequest []string
	cancelRequestSignal  chan struct{}

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

	// logger is read by the connection's goroutines, which NewConnection starts,
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

	// notificationOverflow is what receive does with a notification that arrives
	// when notificationQueue is full.
	notificationOverflow NotificationOverflowPolicy
	// onNotificationDropped, if non-nil, is called for each notification dropped
	// under OverflowDropNewest. It runs on the receive goroutine.
	onNotificationDropped func(method string, totalDropped uint64)
	// droppedNotifications counts notifications dropped under OverflowDropNewest.
	droppedNotifications atomic.Uint64
	// rewrite and inbound are the consumer's hooks on received messages
	// (WithInboundRewrite, WithInboundValidation).
	rewrite func(ctx context.Context, method string, params any) error
	inbound func(ctx context.Context, method string, params any, err error) error
}

// ConnectionOption configures optional parameters for a Connection.
type ConnectionOption func(*connectionConfig)

type connectionConfig struct {
	maxQueuedNotifications int
	notificationOverflow   NotificationOverflowPolicy
	onNotificationDropped  func(method string, totalDropped uint64)
	logger                 *slog.Logger
	rewrite                func(ctx context.Context, method string, params any) error
	inbound                func(ctx context.Context, method string, params any, err error) error
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

// NotificationOverflowPolicy selects what a Connection does with an inbound
// notification that arrives when its notification queue is full.
// More policies may be added in the future.
type NotificationOverflowPolicy int

const (
	// OverflowCloseConnection closes the connection when a notification cannot
	// be queued, and in-flight requests fail. This is the default, and the
	// behaviour of every release before this option existed.
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
// reaches no hook (0015-MADR-validate-absolute-protocol-paths.md, H1).
func WithInboundRewrite(fn func(ctx context.Context, method string, params any) error) ConnectionOption {
	return func(cfg *connectionConfig) {
		cfg.rewrite = fn
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
func NewConnection(handler MethodHandler, peerInput io.Writer, peerOutput io.Reader, opts ...ConnectionOption) *Connection {
	cfg := connectionConfig{
		maxQueuedNotifications: defaultMaxQueuedNotifications,
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
		pending:             make(map[string]*pendingResponse),
		inflight:            make(map[string]context.CancelCauseFunc),
		cancelRequestSignal: make(chan struct{}, 1),
		ctx:                 ctx,
		cancel:              cancel,
		inboundCtx:          inboundCtx,
		inboundCancel:       inboundCancel,
		notificationQueue:   make(chan queuedNotification, cfg.maxQueuedNotifications),

		notificationOverflow:  cfg.notificationOverflow,
		onNotificationDropped: cfg.onNotificationDropped,
		rewrite:               cfg.rewrite,
		inbound:               cfg.inbound,
	}
	if cfg.logger != nil {
		c.logger.Store(cfg.logger)
	}
	c.notifyCond = sync.NewCond(&c.notifyMu)
	go func() {
		<-c.ctx.Done()
		c.notifyMu.Lock()
		c.notifyCond.Broadcast()
		c.notifyMu.Unlock()
	}()
	go c.sendCancelRequests()
	go c.receive()
	go c.processNotifications()
	return c
}

// DroppedNotifications reports how many inbound notifications this connection
// has dropped under OverflowDropNewest. It is always zero under the default
// policy.
func (c *Connection) DroppedNotifications() uint64 {
	return c.droppedNotifications.Load()
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
	for i := 0; i < len(raw); i++ {
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

func (c *Connection) receive() {
	const (
		initialBufSize = 1024 * 1024
		maxBufSize     = 10 * 1024 * 1024
	)

	var received uint64
	scanner := bufio.NewScanner(c.r)
	buf := make([]byte, 0, initialBufSize)
	scanner.Buffer(buf, maxBufSize)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var msg anyMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			c.loggerOrDefault().Error("failed to parse incoming message", "err", err, "raw", string(line))
			continue
		}

		// Handle $/cancel_request notifications synchronously so cancellations take effect
		// immediately and do not participate in notification ordering.
		if msg.ID == nil && msg.Method == ProtocolMethodCancelRequest {
			c.handleCancelRequest(&msg)
			continue
		}

		switch {
		case msg.ID != nil && msg.Method == "":
			c.handleResponse(&msg)
		case msg.Method != "":
			received++
			if msg.ID != nil {
				idKey, err := canonicalJSONRPCIDKey(*msg.ID)
				if err != nil {
					c.loggerOrDefault().Error("failed to canonicalize inbound request id", "err", err, "id", string(*msg.ID))
					idKey = string(*msg.ID)
				}
				reqCtx, cancel := context.WithCancelCause(context.WithValue(c.ctx, receiptKey{}, received))

				c.mu.Lock()
				c.inflight[idKey] = cancel
				c.mu.Unlock()

				m := msg
				go func(m *anyMessage, idKey string, reqCtx context.Context, cancel context.CancelCauseFunc) {
					defer func() {
						c.mu.Lock()
						delete(c.inflight, idKey)
						c.mu.Unlock()

						cancel(nil)
					}()
					c.handleInbound(reqCtx, m)
				}(&m, idKey, reqCtx, cancel)
				continue
			}

			// Queue the notification for sequential processing. The sequence number marks
			// the response-scoped barrier boundary for requests that observe later responses.
			m := msg
			c.notifyMu.Lock()
			c.lastEnqueuedNotificationSeq++
			seq := c.lastEnqueuedNotificationSeq
			select {
			case c.notificationQueue <- queuedNotification{seq: seq, receipt: received, msg: &m}:
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
					continue
				}
				c.loggerOrDefault().Error("failed to queue notification; closing connection", "err", errNotificationQueueOverflow, "capacity", cap(c.notificationQueue), "queued", len(c.notificationQueue))
				c.shutdownReceive(errNotificationQueueOverflow)
				return
			}
		default:
			c.loggerOrDefault().Error("received message with neither id nor method", "raw", string(line))
		}
	}

	cause := errors.New("peer connection closed")
	if err := scanner.Err(); err != nil {
		cause = err
	}
	c.shutdownReceive(cause)
}

func (c *Connection) shutdownReceive(cause error) {
	if cause == nil {
		cause = errors.New("connection closed")
	}

	// First, signal disconnect to callers waiting on responses.
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
	go func(finalEnqueuedSeq uint64) {
		c.waitForNotificationDrain(finalEnqueuedSeq, notificationQueueDrainTimeout)
		c.inboundCancel(cause)
	}(finalEnqueuedSeq)

	c.loggerOrDefault().Info("connection closed", "cause", cause.Error())
}

// processNotifications processes notifications sequentially to maintain order.
// It terminates when notificationQueue is closed (e.g. on disconnect in receive()).
func (c *Connection) processNotifications() {
	for queued := range c.notificationQueue {
		c.handleInbound(context.WithValue(c.inboundCtx, receiptKey{}, queued.receipt), queued.msg)

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
		c.loggerOrDefault().Error("failed to canonicalize response id", "err", err, "id", string(*msg.ID))
		idStr = string(*msg.ID)
	}

	c.mu.Lock()
	pr := c.pending[idStr]
	if pr != nil {
		delete(c.pending, idStr)
	}
	c.mu.Unlock()

	if pr != nil {
		c.notifyMu.Lock()
		watermark := c.lastEnqueuedNotificationSeq
		if c.completedNotificationSeq > watermark {
			c.notifyMu.Unlock()
			panic("completed notification sequence exceeded response watermark")
		}
		c.notifyMu.Unlock()
		pr.ch <- responseEnvelope{msg: *msg, notificationWatermark: watermark}
	}
}

func (c *Connection) handleCancelRequest(msg *anyMessage) {
	var p cancelRequestParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		c.loggerOrDefault().Error("failed to parse "+ProtocolMethodCancelRequest+" params", "err", err)
		return
	}
	if len(bytes.TrimSpace(p.RequestID)) == 0 {
		c.loggerOrDefault().Error("received " + ProtocolMethodCancelRequest + " without requestId")
		return
	}

	idKey, err := canonicalJSONRPCIDKey(p.RequestID)
	if err != nil {
		c.loggerOrDefault().Error("failed to canonicalize "+ProtocolMethodCancelRequest+" requestId", "err", err, "requestId", string(p.RequestID))
		idKey = string(p.RequestID)
	}

	c.mu.Lock()
	cancel := c.inflight[idKey]
	c.mu.Unlock()
	if cancel == nil {
		return
	}

	cancel(context.Canceled)
}

func (c *Connection) handleInbound(ctx context.Context, req *anyMessage) {
	res := anyMessage{JSONRPC: jsonRPCVersion}

	// copy ID if present
	if req.ID != nil {
		res.ID = req.ID
	}
	if c.handler == nil {
		if req.ID != nil {
			res.Error = NewMethodNotFound(req.Method)
			if err := c.sendMessage(res); err != nil {
				return
			}
		}
		return
	}

	result, err := c.handler(ctx, req.Method, req.Params)
	if req.ID == nil {
		// Notification: no response is sent; log handler errors to surface decode failures.
		if err != nil {
			// Per ACP, unknown extension notifications should be ignored.
			if err.Code == -32601 && strings.HasPrefix(req.Method, "_") {
				return
			}
			c.loggerOrDefault().Error("failed to handle notification", "method", req.Method, "err", err)
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
	if err := c.sendMessage(res); err != nil {
		return
	}
}

func (c *Connection) sendMessage(msg anyMessage) error {
	msg.JSONRPC = jsonRPCVersion
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	b = append(b, '\n')

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.w.Write(b)
	return err
}

// SendRequest sends a JSON-RPC request and returns a typed result.
// For methods that do not return a result, use SendRequestNoResult instead.
func SendRequest[T any](c *Connection, ctx context.Context, method string, params any) (T, error) { //nolint:revive // context-as-argument: the connection value stays the first parameter.
	var result T

	msg, idKey, err := c.prepareRequest(method, params)
	if err != nil {
		return result, err
	}

	pr := &pendingResponse{ch: make(chan responseEnvelope, 1)}
	c.mu.Lock()
	c.pending[idKey] = pr
	c.mu.Unlock()

	if err := c.sendMessage(msg); err != nil {
		c.cleanupPending(idKey)
		return result, NewInternalError(map[string]any{jsonErrorField: err.Error()})
	}

	resp, err := c.waitForResponse(ctx, pr, idKey)
	if err != nil {
		return result, err
	}
	if err := c.waitNotificationsUpTo(ctx, resp.notificationWatermark); err != nil {
		return result, err
	}

	if resp.msg.Error != nil {
		return result, resp.msg.Error
	}

	if len(resp.msg.Result) > 0 {
		if err := json.Unmarshal(resp.msg.Result, &result); err != nil {
			return result, NewInternalError(map[string]any{jsonErrorField: err.Error()})
		}
		if err := c.checkInbound(ctx, method, &result); err != nil {
			var zero T
			return zero, fmt.Errorf("%s response: %w", method, err)
		}
	}
	return result, nil
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

	if params != nil {
		if err := validateOutbound(params); err != nil {
			return msg, "", err
		}
		b, err := json.Marshal(params)
		if err != nil {
			return msg, "", NewInvalidParams(map[string]any{jsonErrorField: err.Error()})
		}
		msg.Params = b
	}

	return msg, string(idRaw), nil
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
	peerDisconnectedErr := NewInternalError(map[string]any{jsonErrorField: "peer disconnected before response"})

	select {
	case resp := <-pr.ch:
		return resp, nil
	case <-ctx.Done():
		// If the connection dropped at the same time, prefer reporting peer disconnect
		// and avoid queueing a best-effort cancel notification to a dead peer.
		select {
		case <-c.Done():
			c.cleanupPending(idKey)
			return responseEnvelope{}, peerDisconnectedErr
		default:
		}

		c.sendCancelRequest(idKey)
		c.cleanupPending(idKey)

		cause := context.Cause(ctx)
		if cause == nil {
			cause = ctx.Err()
		}
		if cause != nil {
			return responseEnvelope{}, toReqErr(cause)
		}
		return responseEnvelope{}, NewInternalError(map[string]any{jsonErrorField: "request context ended without cause"})
	case <-c.Done():
		c.cleanupPending(idKey)
		return responseEnvelope{}, peerDisconnectedErr
	}
}

func (c *Connection) waitNotificationsUpTo(ctx context.Context, target uint64) error {
	if target == 0 {
		return nil
	}

	peerDisconnectedErr := NewInternalError(map[string]any{jsonErrorField: "peer disconnected while waiting for pre-response notifications"})
	stopWake := make(chan struct{})
	defer close(stopWake)

	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	if target > c.lastEnqueuedNotificationSeq {
		panic("response watermark exceeded last enqueued notification sequence")
	}

	go func() {
		select {
		case <-ctx.Done():
		case <-stopWake:
			return
		}
		c.notifyMu.Lock()
		c.notifyCond.Broadcast()
		c.notifyMu.Unlock()
	}()

	for c.completedNotificationSeq < target {
		if c.completedNotificationSeq > c.lastEnqueuedNotificationSeq {
			panic("completed notification sequence exceeded enqueued notification sequence while waiting")
		}

		select {
		case <-c.Done():
			return peerDisconnectedErr
		default:
		}
		select {
		case <-ctx.Done():
			select {
			case <-c.Done():
				return peerDisconnectedErr
			default:
			}
			cause := context.Cause(ctx)
			if cause == nil {
				cause = ctx.Err()
			}
			if cause != nil {
				return toReqErr(cause)
			}
			return NewInternalError(map[string]any{jsonErrorField: "request context ended without cause while waiting for notifications"})
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

	stopWake := make(chan struct{})
	defer close(stopWake)

	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	if target > c.lastEnqueuedNotificationSeq {
		panic("drain target exceeded last enqueued notification sequence")
	}

	go func() {
		select {
		case <-ctx.Done():
		case <-stopWake:
			return
		}
		c.notifyMu.Lock()
		c.notifyCond.Broadcast()
		c.notifyMu.Unlock()
	}()

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
func (c *Connection) SendRequestNoResult(ctx context.Context, method string, params any) error {
	msg, idKey, err := c.prepareRequest(method, params)
	if err != nil {
		return err
	}

	pr := &pendingResponse{ch: make(chan responseEnvelope, 1)}
	c.mu.Lock()
	c.pending[idKey] = pr
	c.mu.Unlock()

	if err := c.sendMessage(msg); err != nil {
		c.cleanupPending(idKey)
		return NewInternalError(map[string]any{jsonErrorField: err.Error()})
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
func (c *Connection) SendNotification(ctx context.Context, method string, params any) error {
	select {
	case <-ctx.Done():
		return NewInternalError(map[string]any{jsonErrorField: ctx.Err().Error()})
	default:
	}

	msg, err := c.prepareNotification(method, params)
	if err != nil {
		return err
	}

	if err := c.sendMessage(msg); err != nil {
		return NewInternalError(map[string]any{jsonErrorField: err.Error()})
	}
	return nil
}

// checkResult returns a handler's result to send, after validating it as
// sent: a result that fails is not sent, the peer gets -32603 naming the type
// and property, and the connection logs it (0015-MADR-validate-absolute-protocol-paths.md, E1).
func (c *Connection) checkResult(method string, result any) (any, *RequestError) {
	if err := validateOutbound(result); err != nil {
		c.loggerOrDefault().Error("handler result fails validation; not sent", "method", method, "err", err)
		return nil, NewInternalError(map[string]any{jsonErrorField: err.Error()})
	}
	return result, nil
}

// checkInbound runs the receiving order on a decoded message: the rewrite hook,
// the SDK's validation with the host's rule for an absolute path, then the
// validation hook with that result. It returns the error to act on, nil to
// accept (0015-MADR-validate-absolute-protocol-paths.md, item 5).
func (c *Connection) checkInbound(ctx context.Context, method string, params any) error {
	if c.rewrite != nil {
		if err := c.rewrite(ctx, method, params); err != nil {
			return err
		}
	}
	var err error
	if v, ok := params.(validator); ok {
		err = v.validate(filepath.IsAbs)
	}
	if c.inbound != nil {
		return c.inbound(ctx, method, params, err)
	}
	return err
}

// inboundError is the answer to a received request that checkInbound refused: a
// *RequestError as it is, any other error as invalid params.
func inboundError(err error) *RequestError {
	var re *RequestError
	if errors.As(err, &re) {
		return re
	}
	return NewInvalidParams(map[string]any{jsonErrorField: err.Error()})
}

func (c *Connection) prepareNotification(method string, params any) (anyMessage, error) {
	msg := anyMessage{
		JSONRPC: jsonRPCVersion,
		Method:  method,
	}

	if params != nil {
		if err := validateOutbound(params); err != nil {
			return msg, err
		}
		b, err := json.Marshal(params)
		if err != nil {
			return msg, NewInvalidParams(map[string]any{jsonErrorField: err.Error()})
		}
		msg.Params = b
	}

	return msg, nil
}

// Done returns a channel that is closed when the underlying reader loop exits
// (typically when the peer disconnects or the input stream is closed).
func (c *Connection) Done() <-chan struct{} {
	return c.ctx.Done()
}

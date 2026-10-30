package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// RequestError represents a JSON-RPC error response.
//
// An error the connection makes itself for a local failure keeps that
// failure as its cause, which Unwrap returns: the context.Canceled or
// context.DeadlineExceeded of a context that ended, ErrPeerDisconnected, or
// the error that failed a write. An error a peer answered has none
// (0019-MADR-remediate-debugging-pass-findings.md, decision 3).
type RequestError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`

	cause error
}

// Unwrap returns the local failure e stands for, or nil for an error a peer
// answered.
func (e *RequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// withCause sets e's cause and returns e.
func (e *RequestError) withCause(cause error) *RequestError {
	e.cause = cause
	return e
}

// Error returns e as compact JSON, its code, message and any data, as the
// error object is written on the wire; when the data does not encode, it
// returns "code <code>: <message>" with the data printed by fmt. A nil e
// returns "<nil>".
func (e *RequestError) Error() string {
	// Prefer a structured, JSON-style string so callers get details by default
	// similar to the TypeScript client.
	// Example: {"code":-32603,"message":"Internal error","data":{"details":"..."}}
	if e == nil {
		return "<nil>"
	}
	// Compact JSON of the exported fields, as the wire has them; the cause is
	// unexported and not encoded.
	b, err := json.Marshal(e)
	if err == nil {
		return string(b)
	}
	// Fallback if marshal fails.
	if e.Data != nil {
		return fmt.Sprintf("code %d: %s (data: %v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("code %d: %s", e.Code, e.Message)
}

// ValidationRule names the rule a generated Validate check, or an elicitation
// check, enforces.
type ValidationRule string

// The rules of the generated Validate checks
// (0015-MADR-validate-absolute-protocol-paths.md, X1).
const (
	// RuleAbsolutePath: a path the protocol requires to be absolute is not.
	RuleAbsolutePath ValidationRule = "absolutePath"
	// RuleMinLength: a string is shorter, in characters, than its minLength.
	RuleMinLength ValidationRule = "minLength"
	// RuleMinimum: a number is less than its minimum.
	RuleMinimum ValidationRule = "minimum"
	// RuleMaximum: a number is greater than its maximum.
	RuleMaximum ValidationRule = "maximum"
	// RuleOneVariant: a union does not have exactly one variant set.
	RuleOneVariant ValidationRule = "oneVariant"
	// RuleOneMember: an object with flattened members does not have exactly
	// one member set.
	RuleOneMember ValidationRule = "oneMember"
	// RuleExcludedValue: a union's catch-all holds a discriminator value that
	// names another variant.
	RuleExcludedValue ValidationRule = "excludedValue"
)

// The rules of the elicitation checks, ElicitationSchema's CheckSchema and
// Check (0040-MADR-check-elicitation-forms-answers-and-urls.md). RuleMinLength,
// RuleMinimum and RuleMaximum serve them too.
const (
	// RuleRequired: a field the form requires is absent from the answer.
	RuleRequired ValidationRule = "required"
	// RuleUndeclared: a name the form does not declare: a key of the answer,
	// or a name its required list gives.
	RuleUndeclared ValidationRule = "undeclared"
	// RuleType: a value is not of the type its field takes.
	RuleType ValidationRule = "type"
	// RuleMaxLength: a string is longer, in characters, than its maxLength.
	RuleMaxLength ValidationRule = "maxLength"
	// RulePattern: a string does not match its pattern.
	RulePattern ValidationRule = "pattern"
	// RuleFormat: a string is not of its format, or a format or URL is not
	// one the protocol allows.
	RuleFormat ValidationRule = "format"
	// RuleEnum: a value is not one of its choices, or a list of choices is
	// empty or given twice.
	RuleEnum ValidationRule = "enum"
	// RuleMinItems: a list has fewer items than its minItems.
	RuleMinItems ValidationRule = "minItems"
	// RuleMaxItems: a list has more items than its maxItems.
	RuleMaxItems ValidationRule = "maxItems"
	// RuleUnsupported: a rule this package cannot check, such as a pattern
	// Go's RE2 cannot compile; the value is not taken to pass it.
	RuleUnsupported ValidationRule = "unsupported"
	// RuleBounds: a lower bound is greater than its upper bound.
	RuleBounds ValidationRule = "bounds"
	// RuleDuplicate: a choice is listed twice.
	RuleDuplicate ValidationRule = "duplicate"
	// RuleDefault: a default its own field's rules refuse; the error also
	// holds the rule it breaks.
	RuleDefault ValidationRule = "default"
)

// ValidationError is the error a generated Validate check returns, and each
// problem an elicitation check reports. Type is the Go type checked; Field is
// the JSON key, with an index for an element such as
// "additionalDirectories[1]", and is empty for a check of the whole value;
// Value is the offending value when it is a single value; Rule is the rule
// broken. A Validate error that wraps a nested type's error keeps this one
// reachable with errors.As.
//
// The message of an absolute-path or excluded-value check quotes the value it
// refuses, so a value the peer sent reaches the answer's data and the logs as
// it was (0024-PLAN-behaviour-fixes.md, A14). The message of an elicitation
// check of an answer, a text input or a URL names the field and the rule and
// never the value, which may be personal data or a credential: Value holds an
// answer's value, and is nil for a text input or a URL. CheckSchema's messages
// may quote the schema, such as the part of a pattern RE2 cannot compile
// (0040-MADR-check-elicitation-forms-answers-and-urls.md, items 2 to 5).
type ValidationError struct {
	Type  string
	Field string
	Value any
	Rule  ValidationRule
	msg   string
}

// Error returns the check's message.
func (e *ValidationError) Error() string {
	if e.msg != "" {
		return e.msg
	}
	if e.Field == "" {
		return fmt.Sprintf("%s: %s", e.Type, e.Rule)
	}
	return fmt.Sprintf("%s: %s: %s", e.Type, e.Field, e.Rule)
}

// ErrPromptCancelled is the cause of a prompt turn's cancellation by
// session/cancel, read with context.Cause; a turn superseded by a later prompt
// for its session is cancelled with an error that wraps it
// (0017-MADR-remediate-acp-v1-conformance-violations.md, P1 and Q1).
var ErrPromptCancelled = errors.New("acp: prompt turn cancelled")

// ErrNotInitialized is the error a typed method returns, wrapped with the
// method's name, when initialize has not completed on its connection
// (0017-MADR-remediate-acp-v1-conformance-violations.md, G1).
var ErrNotInitialized = errors.New("acp: initialize has not completed")

// ErrTerminalAuthMethod is the error, wrapped with the method, the property
// and the id, that an authenticate naming a terminal auth method of the
// agent's initialize answer meets: a terminal method is run, not
// authenticated, and the client must not send it
// (0017-MADR-remediate-acp-v1-conformance-violations.md, G1).
var ErrTerminalAuthMethod = errors.New("acp: a terminal auth method is not authenticated")

// ErrPeerDisconnected is the cause of the error a call returns when its
// connection ended because the peer closed it, read with errors.Is
// (0019-MADR-remediate-debugging-pass-findings.md, decision 3).
var ErrPeerDisconnected = errors.New("acp: peer disconnected")

// ErrConnectionClosed is the cause of the error a call or a send returns once
// Close has ended its connection, read with errors.Is
// (0019-MADR-remediate-debugging-pass-findings.md, decision 4).
var ErrConnectionClosed = errors.New("acp: connection closed")

// ErrShutdown is the cause every handler's context ends with under
// Connection.Shutdown, so that a handler can tell its connection's shutdown
// from a client's cancel, and the cause Err wraps, beside ErrConnectionClosed,
// once Shutdown has closed the connection
// (0042-MADR-in-process-agent-runner.md, item 2).
var ErrShutdown = errors.New("acp: connection shut down")

// ErrHandlersRunning is why a wait for a connection's handlers ended before
// they had returned: a handler that ignores its context, or one still at
// work, holds them. ServeAgent wraps it when its bound passes with handlers
// still running, and agentproc's CloseContext and KillContext join it when
// their context ends with the connection's handlers still running; read
// with errors.Is (0042-MADR-in-process-agent-runner.md, the amendment of
// 2026-10-30, X2).
var ErrHandlersRunning = errors.New("acp: the connection's handlers still run")

// ErrNotificationQueueOverflow is why a connection closed itself under
// OverflowCloseConnection: an inbound notification found its queue full.
// Err and the errors of later calls wrap it, beside ErrConnectionClosed, read
// with errors.Is (0026-MADR-connection-end-cause-and-wait.md).
var ErrNotificationQueueOverflow = errors.New("notification queue overflow")

// peerEOF is why a connection ended at the end of the peer's output. Its text
// is the one the connection has always given; it wraps io.EOF, so Err and a
// later call's error can be read with errors.Is(err, io.EOF)
// (0026-MADR-connection-end-cause-and-wait.md).
type peerEOF struct{}

func (peerEOF) Error() string { return "peer connection closed" }

func (peerEOF) Unwrap() error { return io.EOF }

// errPeerEOF is the peer's end of output.
var errPeerEOF error = peerEOF{}

// CapabilityError reports a method, property or variant that the capabilities
// negotiated by initialize do not allow. Method is the wire method;
// Capability is the dotted capability path; Field is the JSON key, with an
// index for an element, when the gate is on a property; Variant is the
// discriminator value when the gate is on a union variant
// (0017-MADR-remediate-acp-v1-conformance-violations.md, G1).
type CapabilityError struct {
	Method     string
	Capability string
	Field      string
	Variant    string
}

// Error names the method and capability, then the property or variant.
func (e *CapabilityError) Error() string {
	msg := fmt.Sprintf("%s: capability %s not advertised", e.Method, e.Capability)
	switch {
	case e.Field != "":
		msg += ": " + e.Field
	case e.Variant != "":
		msg += ": variant " + e.Variant
	}
	return msg
}

// ProtocolVersionError reports a protocol version this SDK does not speak:
// one an application asked to send, with Answered 0, or one an agent answered
// to the version requested. The application closes the connection and
// informs the user, as the specification recommends
// (0017-MADR-remediate-acp-v1-conformance-violations.md, N1).
type ProtocolVersionError struct {
	Requested int
	Answered  int
}

// Error names the versions.
func (e *ProtocolVersionError) Error() string {
	if e.Answered == 0 {
		return fmt.Sprintf("acp: protocol version %d is not supported", e.Requested)
	}
	return fmt.Sprintf("acp: agent answered protocol version %d to a request for %d", e.Answered, e.Requested)
}

// NewParseError returns a JSON-RPC parse error (-32700).
func NewParseError(data any) *RequestError {
	return &RequestError{Code: -32700, Message: "Parse error", Data: data}
}

// NewInvalidRequest returns a JSON-RPC invalid-request error (-32600).
func NewInvalidRequest(data any) *RequestError {
	return &RequestError{Code: -32600, Message: "Invalid request", Data: data}
}

// NewMethodNotFound returns a JSON-RPC method-not-found error (-32601).
func NewMethodNotFound(method string) *RequestError {
	return &RequestError{Code: -32601, Message: "Method not found", Data: map[string]any{"method": method}}
}

// NewInvalidParams returns a JSON-RPC invalid-params error (-32602).
func NewInvalidParams(data any) *RequestError {
	return &RequestError{Code: -32602, Message: "Invalid params", Data: data}
}

// NewInternalError returns a JSON-RPC internal error (-32603).
func NewInternalError(data any) *RequestError {
	return &RequestError{Code: -32603, Message: "Internal error", Data: data}
}

// NewRequestCancelled returns a JSON-RPC request-cancelled error (-32800).
func NewRequestCancelled(data any) *RequestError {
	return &RequestError{Code: -32800, Message: "Request cancelled", Data: data}
}

// NewAuthRequired returns an ACP authentication-required error (-32000).
func NewAuthRequired(data any) *RequestError {
	return &RequestError{Code: -32000, Message: "Authentication required", Data: data}
}

// NewResourceNotFound returns an ACP resource-not-found error (-32002), naming
// the resource, such as a file, that was not found
// (0017-MADR-remediate-acp-v1-conformance-violations.md, K1).
func NewResourceNotFound(uri string) *RequestError {
	return &RequestError{Code: -32002, Message: "Resource not found", Data: map[string]any{"uri": uri}}
}

// The codes the error helpers compare. Each is the ErrorCode constant named
// beside it, written as a number, since this file is compiled into the
// generator's synthetic cases, which have no ErrorCode; TestErrorCodesPinned
// keeps them equal.
const (
	codeMethodNotFound         = -32601 // ErrorCodeMethodNotFound
	codeAuthenticationRequired = -32000 // ErrorCodeAuthenticationRequired
	codeRequestCancelled       = -32800 // ErrorCodeRequestCancelled
	codeResourceNotFound       = -32002 // ErrorCodeResourceNotFound
)

// IsMethodNotFound reports whether err, or an error it wraps, is a
// method-not-found answer (-32601), the peer's or this side's, or the
// *CapabilityError of a method the peer did not advertise, which this side
// refuses before sending it. A *CapabilityError that names a field or a
// variant is not: the method exists, and one of its values is refused
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func IsMethodNotFound(err error) bool {
	if re, ok := errors.AsType[*RequestError](err); ok && re != nil && re.Code == codeMethodNotFound {
		return true
	}
	ce, ok := errors.AsType[*CapabilityError](err)
	return ok && ce != nil && ce.Field == "" && ce.Variant == ""
}

// IsAuthRequired reports whether err, or an error it wraps, is an
// authentication-required answer (-32000)
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func IsAuthRequired(err error) bool {
	re, ok := errors.AsType[*RequestError](err)
	return ok && re != nil && re.Code == codeAuthenticationRequired
}

// IsResourceNotFound reports whether err, or an error it wraps, is a
// resource-not-found answer (-32002), such as a client's for a file that
// does not exist, or a terminal id it does not know
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 3).
func IsResourceNotFound(err error) bool {
	re, ok := errors.AsType[*RequestError](err)
	return ok && re != nil && re.Code == codeResourceNotFound
}

// IsCancelled reports whether err, or an error it wraps, is a cancellation:
// context.Canceled, ErrPromptCancelled, or a request-cancelled answer
// (-32800). A context's deadline is not a cancellation
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func IsCancelled(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, ErrPromptCancelled) {
		return true
	}
	re, ok := errors.AsType[*RequestError](err)
	return ok && re != nil && re.Code == codeRequestCancelled
}

// Reason returns why e was answered: the "error" of its data when that is a
// non-empty string, as this SDK writes a refusal's reason, and its message
// otherwise. A nil e returns ""
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
func (e *RequestError) Reason() string {
	if e == nil {
		return ""
	}
	if m, ok := e.Data.(map[string]any); ok {
		if s, ok := m[jsonErrorField].(string); ok && s != "" {
			return s
		}
	}
	return e.Message
}

// toReqErr coerces arbitrary errors into JSON-RPC RequestError.
func toReqErr(err error) *RequestError {
	if err == nil {
		return nil
	}
	if re, ok := errors.AsType[*RequestError](err); ok {
		// A nil *RequestError in a non-nil error is an error, not success
		// (0019-PLAN-behaviour-fixes.md, B8).
		if re == nil {
			return NewInternalError(map[string]any{jsonErrorField: "handler returned a nil *RequestError"})
		}
		return re
	}
	if errors.Is(err, context.Canceled) {
		return NewRequestCancelled(map[string]any{jsonErrorField: err.Error()}).withCause(err)
	}
	return NewInternalError(map[string]any{jsonErrorField: err.Error()}).withCause(err)
}

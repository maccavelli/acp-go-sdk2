package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// RequestError represents a JSON-RPC error response.
type RequestError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RequestError) Error() string {
	// Prefer a structured, JSON-style string so callers get details by default
	// similar to the TypeScript client.
	// Example: {"code":-32603,"message":"Internal error","data":{"details":"..."}}
	if e == nil {
		return "<nil>"
	}
	// Try to pretty-print compact JSON for stability in logs.
	type view struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    any    `json:"data,omitempty"`
	}
	v := view{Code: e.Code, Message: e.Message, Data: e.Data}
	b, err := json.Marshal(v)
	if err == nil {
		return string(b)
	}
	// Fallback if marshal fails.
	if e.Data != nil {
		return fmt.Sprintf("code %d: %s (data: %v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("code %d: %s", e.Code, e.Message)
}

// ValidationRule names the rule a generated Validate check enforces.
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

// ValidationError is the error a generated Validate check returns. Type is
// the Go type checked; Field is the JSON key, with an index for an element
// such as "additionalDirectories[1]", and is empty for a check of the whole
// value; Value is the offending value when it is a single value; Rule is the
// rule broken. A Validate error that wraps a nested type's error keeps this
// one reachable with errors.As.
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

// toReqErr coerces arbitrary errors into JSON-RPC RequestError.
func toReqErr(err error) *RequestError {
	if err == nil {
		return nil
	}
	if re, ok := errors.AsType[*RequestError](err); ok {
		return re
	}
	if errors.Is(err, context.Canceled) {
		return NewRequestCancelled(map[string]any{jsonErrorField: err.Error()})
	}
	return NewInternalError(map[string]any{jsonErrorField: err.Error()})
}

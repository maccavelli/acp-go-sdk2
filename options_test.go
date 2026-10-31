package acp

import (
	"io"
	"strings"
	"testing"
)

// TestOptionSemantics requires what ConnectionOption's doc says: options
// apply in order, the last of a repeated one winning; n <= 0 keeps the value
// already set for the notification queue and the message size, the default or
// an earlier option's, and sets the request limit back to none; a nil option
// panics. The tool-call options apply in order too
// (0024-PLAN-documentation.md, A10).
func TestOptionSemantics(t *testing.T) {
	t.Parallel()
	build := func(opts ...ConnectionOption) *Connection {
		return buildConnection(nil, io.Discard, strings.NewReader(""), opts...)
	}
	check := func(what string, c *Connection, queue, size, limit int) {
		t.Helper()
		if got := c.queueLimit; got != queue {
			t.Errorf("%s: notification queue %d, want %d", what, got, queue)
		}
		if c.maxMessageSize != size {
			t.Errorf("%s: message size %d, want %d", what, c.maxMessageSize, size)
		}
		if got := cap(c.requestSlots); got != limit || (limit == 0) != (c.requestSlots == nil) {
			t.Errorf("%s: request limit %d (nil %v), want %d", what, got, c.requestSlots == nil, limit)
		}
	}
	check("the last wins",
		build(WithMaxQueuedNotifications(5), WithMaxQueuedNotifications(7), WithMaxMessageSize(100), WithMaxMessageSize(200),
			WithMaxConcurrentRequests(2), WithMaxConcurrentRequests(3)),
		7, 200, 3)
	check("n <= 0 after a value",
		build(WithMaxQueuedNotifications(5), WithMaxQueuedNotifications(0), WithMaxMessageSize(100), WithMaxMessageSize(-1),
			WithMaxConcurrentRequests(2), WithMaxConcurrentRequests(0)),
		5, 100, 0)
	check("n <= 0 alone", build(WithMaxQueuedNotifications(0), WithMaxMessageSize(0), WithMaxConcurrentRequests(-1)),
		defaultMaxQueuedNotifications, defaultMaxMessageSize, 0)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("a nil option did not panic")
			}
		}()
		build(nil)
	}()

	start := StartToolCall("c", "t", WithStartStatus(ToolCallStatusPending), WithStartStatus(ToolCallStatusCompleted))
	if got := start.ToolCall.Status; got != ToolCallStatusCompleted {
		t.Errorf("StartToolCall status %q, want the last option's, completed", got)
	}
	update := UpdateToolCall("c", WithUpdateStatus(ToolCallStatusPending), WithUpdateStatus(ToolCallStatusFailed))
	if got := update.ToolCallUpdate.Status; got == nil || *got != ToolCallStatusFailed {
		t.Errorf("UpdateToolCall status %v, want the last option's, failed", got)
	}
}

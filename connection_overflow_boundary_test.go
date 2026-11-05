package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

// The tests of the queue's limit at its boundary
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, R11). Each holds the
// notification loop with withBeforeDequeue before it takes anything, so that
// the notification next to run is still in the channel: the order in which
// the loop runs, which decided the outcome when the channel's room did, is
// the test's.

// heldLoop returns an option that holds the notification loop before each
// receive until release is called, and release, which the test's end calls
// too, so that a failed test does not leave the loop held in its bubble.
func heldLoop(t *testing.T) (opt ConnectionOption, release func()) {
	t.Helper()
	hold := make(chan struct{})
	release = sync.OnceFunc(func() { close(hold) })
	t.Cleanup(release)
	return withBeforeDequeue(func() { <-hold }), release
}

// TestOverflowBoundaryCounted requires, after initialize, with a queue of 1
// and the loop held before it takes anything, two notifications, one to run
// and one to wait, to leave the connection open and both to be handled once
// the loop runs, under each policy; and a third to meet the policy: a close,
// a drop, or a pause after which all three are handled.
func TestOverflowBoundaryCounted(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		policy  NotificationOverflowPolicy
		closes  bool
		handled int32
		dropped uint64
	}{
		{"close", OverflowCloseConnection, true, -1, 0},
		{"drop", OverflowDropNewest, false, 2, 1},
		{"block", OverflowBlock, false, 3, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				hold, release := heldLoop(t)
				var handled atomic.Int32
				var c *ClientSideConnection
				p := rawWire(t, func(w io.Writer, r io.Reader) {
					c = NewClientSideConnection(&clientFuncs{HandleExtensionMethodFunc: func(context.Context, string, json.RawMessage) (any, error) {
						handled.Add(1)
						return map[string]any{}, nil
					}}, w, r, WithLogger(slog.New(slog.DiscardHandler)), WithMaxQueuedNotifications(1),
						WithNotificationOverflowPolicy(tc.policy), hold)
				})
				t.Cleanup(func() { _ = c.conn.Close() })
				if _, err := initializeAgainst(t.Context(), t, c, p, ""); err != nil {
					t.Fatal(err)
				}
				p.send(extNotification)
				p.send(extNotification)
				synctest.Wait()
				if isEnded(c.conn) || c.DroppedNotifications() != 0 {
					t.Fatalf("two notifications with a queue of 1, the loop behind: ended %v (%v), dropped %d; want open and none",
						isEnded(c.conn), c.Err(), c.DroppedNotifications())
				}
				go func() { _ = p.trySend(extNotification) }()
				synctest.Wait()
				if isEnded(c.conn) != tc.closes {
					t.Fatalf("a third: ended %v (%v), want %v", isEnded(c.conn), c.Err(), tc.closes)
				}
				release()
				synctest.Wait()
				if tc.closes {
					if err := c.Err(); !errors.Is(err, ErrNotificationQueueOverflow) {
						t.Errorf("the connection ended with %v, want ErrNotificationQueueOverflow", err)
					}
					return
				}
				if got := handled.Load(); got != tc.handled || c.DroppedNotifications() != tc.dropped {
					t.Errorf("handled %d, dropped %d; want %d and %d", got, c.DroppedNotifications(), tc.handled, tc.dropped)
				}
			})
		})
	}
}

// TestOverflowBlockHeldInitializeLoopBehind is
// TestOverflowBlockHeldInitialize's input with the loop held before its first
// receive, the order in which that test failed: three notifications held for
// initialize's answer, with a queue of 1, leave the connection open, and once
// the loop runs Initialize returns its result and the three are handled; a
// fourth, past twice the queue's size and the one next to run, closes as
// OverflowCloseConnection does (0044-MADR-notification-queue-back-pressure.md,
// D4 as amended).
func TestOverflowBlockHeldInitializeLoopBehind(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		notes int
	}{{"at the limit", 3}, {"past it", 4}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				hold, release := heldLoop(t)
				c, p, ext := initializeClient(t, WithMaxQueuedNotifications(1), WithNotificationOverflowPolicy(OverflowBlock), hold)
				t.Cleanup(func() { _ = c.conn.Close() })
				done := make(chan error, 1)
				go func() {
					_, err := c.Initialize(t.Context(), InitializeRequest{})
					done <- err
				}()
				req := p.next("the initialize request")
				go func() {
					for range tc.notes {
						if p.trySend(extNotification) != nil {
							return
						}
					}
					_ = p.trySend(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, initializeAnswer))
				}()
				synctest.Wait()
				if tc.notes == 4 {
					if !isEnded(c.conn) || !errors.Is(c.Err(), ErrNotificationQueueOverflow) {
						t.Fatalf("four held notifications: ended %v (%v), want ErrNotificationQueueOverflow", isEnded(c.conn), c.Err())
					}
					release()
					if err := within(t, done, "Initialize"); err == nil {
						t.Error("Initialize = nil, want an error: the connection ended before the answer")
					}
					return
				}
				if isEnded(c.conn) {
					t.Fatalf("three held notifications, the loop behind: the connection ended: %v", c.Err())
				}
				release()
				synctest.Wait()
				select {
				case err := <-done:
					if err != nil {
						t.Fatalf("Initialize = %v, want its result", err)
					}
				default:
					t.Fatal("Initialize did not return once the loop ran")
				}
				for i := range 3 {
					select {
					case <-ext:
					default:
						t.Fatalf("%d of the 3 held notifications were handled", i)
					}
				}
			})
		})
	}
}

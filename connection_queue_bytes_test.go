package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
)

// The tests of the byte budget (0045-MADR-notification-queue-byte-budget.md).
// Each sizes its lines exactly, under a small WithMaxMessageSize and a small
// budget, with the count far from binding, so only the bytes decide. They run
// in synctest bubbles where a reader or a write waits, as the OverflowBlock
// tests do.

// bytesOpts are a budget of 1000 bytes, lines of up to 400, and a count of
// 100 that never binds, with policy p.
func bytesOpts(p NotificationOverflowPolicy) []ConnectionOption {
	return []ConnectionOption{
		WithMaxMessageSize(400), WithMaxQueuedBytes(1000), WithMaxQueuedNotifications(100), WithNotificationOverflowPolicy(p),
	}
}

// padded is the _x/n notification numbered n, padded to size bytes.
func padded(t *testing.T, n, size int) string {
	t.Helper()
	const format = `{"jsonrpc":"2.0","method":"_x/n","params":{"n":%d,"pad":"%s"}}`
	bare := fmt.Sprintf(format, n, "")
	if size < len(bare) {
		t.Fatalf("a line of %d bytes cannot hold notification %d", size, n)
	}
	return fmt.Sprintf(format, n, strings.Repeat("x", size-len(bare)))
}

// paddedNotes is the _x/n notifications numbered from to to, inclusive, each
// size bytes.
func paddedNotes(t *testing.T, from, to, size int) []string {
	t.Helper()
	var lines []string
	for n := from; n <= to; n++ {
		lines = append(lines, padded(t, n, size))
	}
	return lines
}

// paddedRequest is a request for method with id id, padded to size bytes.
func paddedRequest(t *testing.T, id, method string, size int) string {
	t.Helper()
	const format = `{"jsonrpc":"2.0","id":%q,"method":%q,"params":{"pad":"%s"}}`
	bare := fmt.Sprintf(format, id, method, "")
	if size < len(bare) {
		t.Fatalf("a line of %d bytes cannot hold request %s", size, id)
	}
	return fmt.Sprintf(format, id, method, strings.Repeat("x", size-len(bare)))
}

// TestQueuedBytesPausesReading requires notifications whose lines fill the
// byte budget, while the count has room, to pause the reader as the count
// does, and then every one to be delivered in order, none dropped, the
// connection open, and no byte left counted (Confirmation 1 and 5).
func TestQueuedBytesPausesReading(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		b := newBlockPeer(t, false, bytesOpts(OverflowBlock)...)
		// 300 bytes each: 0 running, 1 and 2 queued, 900 counted; 3 would make
		// 1200, so the reader holds it and waits.
		b.feed(paddedNotes(t, 0, 9, 300)...)
		synctest.Wait()
		checkPaused(t, b)
		if got := b.c.QueuedBytes(); got != 900 {
			t.Errorf("QueuedBytes = %d while paused, want 900", got)
		}
		b.releaseOnce()
		synctest.Wait()
		if got := b.deliveredSoFar(); !slices.Equal(got, count(10)) {
			t.Errorf("delivered %v, want 0 to 9 in order", got)
		}
		if isEnded(b.c) {
			t.Errorf("the connection ended: %v", b.c.Err())
		}
		if got := b.c.QueuedBytes(); got != 0 {
			t.Errorf("QueuedBytes = %d with every handler returned, want 0", got)
		}
	})
}

// TestQueuedBytesReadOnLimit requires the reader, while a notification
// handler waits on the connection, to read on to twice the byte budget and
// then close as OverflowCloseConnection does (Confirmation 2).
func TestQueuedBytesReadOnLimit(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		b := newBlockPeer(t, true, bytesOpts(OverflowBlock)...)
		b.feed(padded(t, 0, 300))
		b.p.next("the handler's request")
		// 1 to 5 make 1800 with 0's 300; 6 would make 2100, past 2000.
		b.feed(paddedNotes(t, 1, 5, 300)...)
		synctest.Wait()
		if got := b.read(); got != 6 || isEnded(b.c) {
			t.Fatalf("%d notifications queued, ended %v (%v); want 6 and open: 0 running, 1 to 5 queued", got, isEnded(b.c), b.c.Err())
		}
		b.feed(padded(t, 6, 300))
		synctest.Wait()
		if !isEnded(b.c) {
			t.Fatal("the connection is open after a notification past twice the byte budget")
		}
		if err := b.c.Err(); !errors.Is(err, ErrNotificationQueueOverflow) {
			t.Errorf("Err = %v, want it to wrap ErrNotificationQueueOverflow", err)
		}
		if err := <-b.innerErr; err == nil {
			t.Error("the handler's request returned nil after the connection closed")
		}
	})
}

// TestQueuedBytesCloseAndDrop requires the other two policies to act at the
// byte budget as at the count: close, or drop and count (Confirmation 3).
func TestQueuedBytesCloseAndDrop(t *testing.T) {
	t.Parallel()
	t.Run("OverflowCloseConnection", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			b := newBlockPeer(t, false, bytesOpts(OverflowCloseConnection)...)
			b.feed(paddedNotes(t, 0, 5, 300)...)
			synctest.Wait()
			if !isEnded(b.c) {
				t.Fatal("the connection is open after a notification past the byte budget")
			}
			if err := b.c.Err(); !errors.Is(err, ErrNotificationQueueOverflow) {
				t.Errorf("Err = %v, want it to wrap ErrNotificationQueueOverflow", err)
			}
			if got := b.read(); got != 3 {
				t.Errorf("%d notifications queued, want 3: 0 running, 1 and 2 queued", got)
			}
		})
	})
	t.Run("OverflowDropNewest", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			b := newBlockPeer(t, false, bytesOpts(OverflowDropNewest)...)
			b.feed(paddedNotes(t, 0, 9, 300)...)
			synctest.Wait()
			if got := b.c.DroppedNotifications(); got != 7 {
				t.Errorf("DroppedNotifications = %d, want 7: 3 to 9", got)
			}
			b.releaseOnce()
			synctest.Wait()
			if got := b.deliveredSoFar(); !slices.Equal(got, count(3)) {
				t.Errorf("delivered %v, want 0 to 2", got)
			}
			if isEnded(b.c) {
				t.Errorf("the connection ended: %v", b.c.Err())
			}
		})
	})
}

// TestQueuedBytesClampAndSingleLine requires a budget below the message size
// to be raised to it, and a single line of the message size to be accepted
// with nothing counted (Confirmation 4).
func TestQueuedBytesClampAndSingleLine(t *testing.T) {
	t.Parallel()
	t.Run("raised to the message size", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			b := newBlockPeer(t, false, WithMaxMessageSize(400), WithMaxQueuedBytes(10), WithNotificationOverflowPolicy(OverflowCloseConnection))
			// 0 running and 1 queued make 300: within 400, past 10.
			b.feed(paddedNotes(t, 0, 1, 150)...)
			synctest.Wait()
			if got := b.read(); got != 2 || isEnded(b.c) {
				t.Errorf("%d notifications queued, ended %v (%v); want 2 and open: the budget is raised to 400", got, isEnded(b.c), b.c.Err())
			}
		})
	})
	t.Run("one line of the message size", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			b := newBlockPeer(t, false, WithMaxMessageSize(400), WithMaxQueuedBytes(400), WithNotificationOverflowPolicy(OverflowCloseConnection))
			b.feed(padded(t, 0, 400))
			synctest.Wait()
			if got := b.read(); got != 1 || isEnded(b.c) {
				t.Errorf("%d notifications queued, ended %v (%v); want 1 and open", got, isEnded(b.c), b.c.Err())
			}
		})
	})
}

// TestQueuedBytesRefusesRequest requires a request whose line would take the
// counted bytes past the budget to be answered at once, -32603 with the
// limit, its handler never run, and nothing more counted (Confirmation 6).
func TestQueuedBytesRefusesRequest(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		b := newBlockPeer(t, false, bytesOpts(OverflowBlock)...)
		b.feed(paddedNotes(t, 0, 2, 300)...)
		synctest.Wait()
		b.feed(paddedRequest(t, "big", "_x/req", 300))
		a := b.p.next("the refusal")
		if a.Error == nil || a.Error.Code != -32603 || string(a.ID) != `"big"` {
			t.Fatalf("answer %+v, want -32603 for id big", a)
		}
		data, _ := json.Marshal(a.Error.Data)
		if want := `{"error":"too many queued bytes","limit":1000}`; string(data) != want {
			t.Errorf("data %s, want %s", data, want)
		}
		if got := b.c.QueuedBytes(); got != 900 {
			t.Errorf("QueuedBytes = %d after the refusal, want 900", got)
		}
	})
}

// TestQueuedBytesDefault requires a connection given no option to have a
// 64 MiB budget, and one whose message size is larger to have that size.
func TestQueuedBytesDefault(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		opts []ConnectionOption
		want int64
	}{
		{"no option", nil, 64 << 20},
		{"a larger message size", []ConnectionOption{WithMaxMessageSize(100 << 20)}, 100 << 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, w := io.Pipe()
			t.Cleanup(func() { _ = w.Close() })
			c := NewConnection(nil, io.Discard, r, tc.opts...)
			t.Cleanup(func() { _ = c.Close() })
			if c.byteLimit != tc.want {
				t.Errorf("byte budget %d, want %d", c.byteLimit, tc.want)
			}
		})
	}
}

// TestQueuedBytesRequestsDoNotPause requires a request's bytes never to pause
// the reader on a notification: a request's handler holds most of the budget
// and waits for the peer's answer to its own request, and notifications
// arrive before that answer, which must still be read (Confirmation 7; the
// PLAN's deviation 1).
func TestQueuedBytesRequestsDoNotPause(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		var c *Connection
		asked := make(chan error, 1)
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			c = NewConnection(func(ctx context.Context, method string, _ json.RawMessage) (any, *RequestError) {
				if method == "_x/big" {
					_, err := SendRequest[json.RawMessage](c, ctx, "_x/ask", map[string]any{})
					asked <- err
					return map[string]any{}, nil
				}
				return nil, nil
			}, w, r, WithLogger(slog.New(slog.DiscardHandler)), WithMaxMessageSize(1000), WithMaxQueuedBytes(1000),
				WithMaxQueuedNotifications(100), WithNotificationOverflowPolicy(OverflowBlock))
		})
		t.Cleanup(func() { _ = c.Close() })
		go func() { _ = p.trySend(paddedRequest(t, "big", "_x/big", 900)) }()
		ask := p.next("the handler's own request")
		go func() {
			for _, l := range paddedNotes(t, 0, 2, 300) {
				if p.trySend(l) != nil {
					return
				}
			}
			_ = p.trySend(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{}}`, ask.ID))
		}()
		synctest.Wait()
		select {
		case err := <-asked:
			if err != nil {
				t.Errorf("the handler's request: %v", err)
			}
		default:
			t.Fatal("the handler's request was not answered: the reader paused on the request's bytes")
		}
		if a := p.next("the big request's answer"); a.Error != nil || string(a.ID) != `"big"` {
			t.Errorf("answer %+v, want a result for id big", a)
		}
		synctest.Wait()
		if got := c.QueuedBytes(); got != 0 {
			t.Errorf("QueuedBytes = %d with every handler returned, want 0: the request's bytes were not let go", got)
		}
	})
}

// TestByteBudgetAtMaxInt requires a budget whose double does not fit an
// int64, such as math.MaxInt for no practical limit, to keep the connection
// open under OverflowBlock while notifications wait: twice it is
// math.MaxInt64, not a negative number
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, R5). Each runs in a
// synctest bubble.
func TestByteBudgetAtMaxInt(t *testing.T) {
	t.Parallel()
	const note = `{"jsonrpc":"2.0","method":"_x/n"}`
	for _, budget := range []int{math.MaxInt, math.MaxInt/2 + 1} {
		t.Run(strconv.Itoa(budget), func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				entered, release := make(chan struct{}, 4), make(chan struct{})
				t.Cleanup(sync.OnceFunc(func() { close(release) }))
				var c *Connection
				p := rawWire(t, func(w io.Writer, r io.Reader) {
					c = NewConnection(holding(entered, release), w, r, WithMaxQueuedBytes(budget), quietLogs())
				})
				p.send(note)
				within(t, entered, "the running notification")
				// A connection that closes stops reading, so these may fail;
				// the check below says why.
				_ = p.trySend(note)
				_ = p.trySend(note)
				synctest.Wait()
				if isEnded(c) {
					t.Fatalf("WithMaxQueuedBytes(%d): the connection closed with notifications waiting: %v", budget, c.Err())
				}
				if got, want := c.QueuedBytes(), int64(3*len(note)); got != want {
					t.Errorf("QueuedBytes = %d, want %d", got, want)
				}
			})
		})
	}
}

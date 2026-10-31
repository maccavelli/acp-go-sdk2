package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// The tests of OverflowBlock (0044-MADR-notification-queue-back-pressure.md).
// Each runs in a synctest bubble, so a reader paused on a full queue and a
// peer's write blocked on the unread pipe are durable blocks synctest.Wait
// settles on, and the 5 s pause log passes in no wall time.

// blockPeer is a raw peer of a connection whose handler records each _x/n
// notification's n and holds notification 0 until release closes or, unless
// ignoreCtx is set, its context ends. When inner is set, notification 0's
// handler first sends the request _x/inner under its own context and reports
// its error on innerErr.
type blockPeer struct {
	c         *Connection
	p         wire
	release   chan struct{}
	innerErr  chan error
	written   atomic.Int64 // lines feed has finished writing
	ignoreCtx atomic.Bool
	once      sync.Once

	mu        sync.Mutex
	delivered []int
}

func newBlockPeer(t *testing.T, inner bool, opts ...ConnectionOption) *blockPeer {
	t.Helper()
	b := &blockPeer{release: make(chan struct{}), innerErr: make(chan error, 1)}
	b.p = rawWire(t, func(w io.Writer, r io.Reader) {
		b.c = NewConnection(func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
			if method != "_x/n" {
				return map[string]any{}, nil
			}
			var v struct{ N int }
			_ = json.Unmarshal(params, &v)
			b.mu.Lock()
			b.delivered = append(b.delivered, v.N)
			b.mu.Unlock()
			if v.N != 0 {
				return nil, nil
			}
			if inner {
				_, err := SendRequest[json.RawMessage](b.c, ctx, "_x/inner", map[string]any{})
				b.innerErr <- err
			}
			if b.ignoreCtx.Load() {
				<-b.release
				return nil, nil
			}
			select {
			case <-b.release:
			case <-ctx.Done():
			}
			return nil, nil
		}, w, r, append([]ConnectionOption{WithLogger(slog.New(slog.DiscardHandler))}, opts...)...)
	})
	t.Cleanup(func() { _ = b.c.Close() })
	// Runs first, so a test that stops early still lets a handler that
	// ignores its context return before its bubble ends.
	t.Cleanup(b.releaseOnce)
	return b
}

// releaseOnce lets notification 0's handler return; a later call does
// nothing.
func (b *blockPeer) releaseOnce() {
	b.once.Do(func() { close(b.release) })
}

// note is the _x/n notification numbered n.
func note(n int) string { return fmt.Sprintf(`{"jsonrpc":"2.0","method":"_x/n","params":{"n":%d}}`, n) }

// notes is the _x/n notifications numbered from to to, inclusive.
func notes(from, to int) []string {
	var lines []string
	for n := from; n <= to; n++ {
		lines = append(lines, note(n))
	}
	return lines
}

// feed writes lines in order on a goroutine of its own, since a paused reader
// blocks the pipe, counting each write that returns.
func (b *blockPeer) feed(lines ...string) {
	go func() {
		for _, l := range lines {
			if b.p.trySend(l) != nil {
				return
			}
			b.written.Add(1)
		}
	}()
}

// read is how many notifications the connection has queued.
func (b *blockPeer) read() uint64 {
	b.c.notifyMu.Lock()
	defer b.c.notifyMu.Unlock()
	return b.c.lastEnqueuedNotificationSeq
}

func (b *blockPeer) deliveredSoFar() []int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.delivered)
}

// isEnded reports whether c's Done is closed.
func isEnded(c *Connection) bool {
	select {
	case <-c.Done():
		return true
	default:
		return false
	}
}

// count returns 0, 1, ..., n-1.
func count(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}

// checkPaused requires b's reader to be paused with a queue of two while
// notification 0's handler is held: 1 and 2 queued, 3 read and waiting, the
// write of 4 not returned, nothing dropped, the connection open.
func checkPaused(t *testing.T, b *blockPeer) {
	t.Helper()
	if got := b.read(); got != 3 {
		t.Errorf("%d notifications queued while the handler is held, want 3: 0 running, 1 and 2 queued", got)
	}
	if got := b.written.Load(); got != 4 {
		t.Errorf("%d writes returned, want 4: the write of 4 waits for the paused reader", got)
	}
	if got := b.c.DroppedNotifications(); got != 0 {
		t.Errorf("DroppedNotifications = %d, want 0", got)
	}
	if isEnded(b.c) {
		t.Errorf("the connection ended while paused: %v", b.c.Err())
	}
}

// TestOverflowBlockPausesReading requires a full queue to stop the reader
// until the held handler returns, and then every notification to be
// delivered in order, none dropped, the connection open (D2).
func TestOverflowBlockPausesReading(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		b := newBlockPeer(t, false, WithMaxQueuedNotifications(2), WithNotificationOverflowPolicy(OverflowBlock))
		b.feed(notes(0, 6)...)
		synctest.Wait() // the reader waits for room, the feed for the reader
		checkPaused(t, b)
		b.releaseOnce()
		synctest.Wait()
		if got := b.deliveredSoFar(); !slices.Equal(got, count(7)) {
			t.Errorf("delivered %v, want 0 to 6 in order", got)
		}
		if isEnded(b.c) {
			t.Errorf("the connection ended: %v", b.c.Err())
		}
	})
}

// TestOverflowBlockIsTheDefault requires a connection given no policy, or an
// undefined one, to pause as under OverflowBlock, and one given
// OverflowCloseConnection to close on the same input (D9).
func TestOverflowBlockIsTheDefault(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		opts   []ConnectionOption
		closes bool
	}{
		{"no option", nil, false},
		{"undefined policy", []ConnectionOption{WithNotificationOverflowPolicy(NotificationOverflowPolicy(99))}, false},
		{"OverflowCloseConnection named", []ConnectionOption{WithNotificationOverflowPolicy(OverflowCloseConnection)}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				b := newBlockPeer(t, false, append([]ConnectionOption{WithMaxQueuedNotifications(2)}, tc.opts...)...)
				b.feed(notes(0, 6)...)
				synctest.Wait()
				if !tc.closes {
					checkPaused(t, b)
					b.releaseOnce()
					return
				}
				if !isEnded(b.c) || !errors.Is(b.c.Err(), ErrNotificationQueueOverflow) {
					t.Errorf("under OverflowCloseConnection the connection ended %v with %v, want closed with ErrNotificationQueueOverflow", isEnded(b.c), b.c.Err())
				}
				b.releaseOnce()
			})
		})
	}
}

// TestOverflowBlockResponseAsWithRoom requires a request whose response
// follows four notifications, the first held, to return only once they are
// handled, under OverflowBlock with a queue of two as with room for all: a
// paused reader delays no request past the response barrier (F3, F9).
func TestOverflowBlockResponseAsWithRoom(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		opts []ConnectionOption
	}{
		{"room for all", []ConnectionOption{WithMaxQueuedNotifications(100), WithNotificationOverflowPolicy(OverflowCloseConnection)}},
		{"OverflowBlock, a queue of two", []ConnectionOption{WithMaxQueuedNotifications(2), WithNotificationOverflowPolicy(OverflowBlock)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				b := newBlockPeer(t, false, tc.opts...)
				done := make(chan error, 1)
				go func() {
					_, err := SendRequest[json.RawMessage](b.c, t.Context(), "_x/req", map[string]any{})
					done <- err
				}()
				req := b.p.next("the request")
				b.feed(append(notes(0, 3), fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{}}`, req.ID))...)
				synctest.Wait()
				select {
				case err := <-done:
					t.Fatalf("the request returned %v while notification 0 was held", err)
				default:
				}
				b.releaseOnce()
				synctest.Wait()
				select {
				case err := <-done:
					if err != nil {
						t.Errorf("the request returned %v, want its result", err)
					}
				default:
					t.Fatal("the request did not return once the notifications before its response were handled")
				}
				if got := b.deliveredSoFar(); !slices.Equal(got, count(4)) {
					t.Errorf("delivered %v, want 0 to 3", got)
				}
				if isEnded(b.c) {
					t.Errorf("the connection ended: %v", b.c.Err())
				}
			})
		})
	}
}

// TestOverflowBlockRequestFromNotificationHandler requires a request that a
// notification handler sends under its context to be answered although the
// answer follows a full queue, and the reader to pause again once it is: the
// guard's count ends with the request (D3).
func TestOverflowBlockRequestFromNotificationHandler(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		b := newBlockPeer(t, true, WithMaxQueuedNotifications(2), WithNotificationOverflowPolicy(OverflowBlock))
		b.feed(note(0))
		req := b.p.next("the handler's request")
		b.feed(append(notes(1, 3), fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{}}`, req.ID))...)
		synctest.Wait()
		select {
		case err := <-b.innerErr:
			if err != nil {
				t.Fatalf("the handler's request returned %v, want its result", err)
			}
		default:
			t.Fatal("the handler's request is not answered: the reader paused while the handler waited on it")
		}
		// 1 to 3 are queued past the limit of two; the next waits.
		b.feed(notes(4, 6)...)
		synctest.Wait()
		if got := b.read(); got != 4 {
			t.Errorf("%d notifications queued after the request returned, want 4: the reader pauses again", got)
		}
		b.releaseOnce()
		synctest.Wait()
		if got := b.deliveredSoFar(); !slices.Equal(got, count(7)) {
			t.Errorf("delivered %v, want 0 to 6 in order", got)
		}
	})
}

// TestOverflowBlockHeldInitialize requires notifications held for
// initialize's answer past the queue's size not to stop Initialize: the
// reader reads on to the answer that ends the hold (D3).
// TestHeldNotificationsFillQueue requires the same input to close the
// connection under OverflowCloseConnection.
func TestOverflowBlockHeldInitialize(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		c, p, ext := initializeClient(t, WithMaxQueuedNotifications(1), WithNotificationOverflowPolicy(OverflowBlock))
		t.Cleanup(func() { _ = c.conn.Close() })
		done := make(chan error, 1)
		go func() {
			_, err := c.Initialize(t.Context(), InitializeRequest{})
			done <- err
		}()
		req := p.next("the initialize request")
		go func() {
			for _, line := range []string{extNotification, extNotification, extNotification,
				fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, initializeAnswer)} {
				if p.trySend(line) != nil {
					return
				}
			}
		}()
		synctest.Wait()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Initialize = %v, want its result", err)
			}
		default:
			t.Fatal("Initialize did not return: the reader paused on the held notifications")
		}
		for i := range 3 {
			select {
			case <-ext:
			default:
				t.Fatalf("%d of the 3 held notifications were handled", i)
			}
		}
		if isEnded(c.conn) {
			t.Errorf("the connection ended: %v", c.Err())
		}
	})
}

// TestOverflowBlockReadOnLimit requires the reader, while a notification
// handler waits on the connection, to read on to twice the queue's size and
// then close as OverflowCloseConnection does (D4).
func TestOverflowBlockReadOnLimit(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		b := newBlockPeer(t, true, WithMaxQueuedNotifications(2), WithNotificationOverflowPolicy(OverflowBlock))
		b.feed(note(0))
		b.p.next("the handler's request")
		b.feed(notes(1, 4)...)
		synctest.Wait()
		if got := b.read(); got != 5 || isEnded(b.c) {
			t.Fatalf("%d notifications queued, ended %v (%v); want 5 and open: 0 running, 1 to 4 queued", got, isEnded(b.c), b.c.Err())
		}
		b.feed(note(5))
		synctest.Wait()
		if !isEnded(b.c) {
			t.Fatal("the connection is open after a notification past twice the queue's size")
		}
		err := b.c.Err()
		if !errors.Is(err, ErrNotificationQueueOverflow) || !errors.Is(err, ErrConnectionClosed) {
			t.Errorf("Err = %v, want ErrConnectionClosed wrapping ErrNotificationQueueOverflow", err)
		}
		if err := <-b.innerErr; err == nil {
			t.Error("the handler's request returned nil after the connection closed")
		}
	})
}

// TestOverflowBlockEndsOnClose requires Close, and Shutdown, to end a paused
// reader: Done closes, Wait returns, and Err says how (D2). With a handler
// that ignores its context, so the queue never drains, the end itself must
// wake the reader: no goroutine is left in waitForRoom once Close returns.
// That case reads every goroutine's stack, so the test is not parallel: the
// package's parallel tests, paused readers among them, run after it.
func TestOverflowBlockEndsOnClose(t *testing.T) {
	for _, tc := range []struct {
		name      string
		end       func(*Connection)
		want      error
		ignoreCtx bool
	}{
		{"Close", func(c *Connection) { _ = c.Close() }, ErrConnectionClosed, false},
		{"Shutdown", func(c *Connection) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = c.Shutdown(ctx)
		}, ErrShutdown, false},
		{"Close, the handler ignoring its context", func(c *Connection) { _ = c.Close() }, ErrConnectionClosed, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				b := newBlockPeer(t, false, WithMaxQueuedNotifications(1), WithNotificationOverflowPolicy(OverflowBlock))
				b.ignoreCtx.Store(tc.ignoreCtx)
				b.feed(notes(0, 4)...)
				synctest.Wait()
				if got := b.read(); got != 2 {
					t.Fatalf("%d notifications queued, want 2: the reader paused", got)
				}
				tc.end(b.c)
				if tc.ignoreCtx {
					synctest.Wait() // the reader ends, the handler still holds
					if stacksNaming("(*Connection).waitForRoom") {
						t.Error("the reader still waits for room after Close")
					}
					if !isEnded(b.c) {
						t.Error("Done is open after Close")
					}
					b.releaseOnce()
				}
				waited := make(chan error, 1)
				go func() { waited <- b.c.Wait() }()
				synctest.Wait()
				select {
				case err := <-waited:
					if !errors.Is(err, tc.want) || !errors.Is(err, ErrConnectionClosed) {
						t.Errorf("Wait = %v, want ErrConnectionClosed and %v", err, tc.want)
					}
				default:
					t.Fatal("Wait did not return after the paused connection ended")
				}
			})
		})
	}
}

// TestOverflowBlockSlowsAgent requires a client that pauses to slow its agent
// at SessionUpdate, and session/cancel still to end the agent's turn, its
// blocked SessionUpdate returning its context's error (the MADR's F5).
func TestOverflowBlockSlowsAgent(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		c2aR, c2aW := io.Pipe()
		a2cR, a2cW := io.Pipe()
		t.Cleanup(func() {
			_ = c2aW.Close()
			_ = a2cW.Close()
		})
		release := make(chan struct{})
		var sent atomic.Int64
		updateErr := make(chan error, 1)
		var asc *AgentSideConnection
		asc = NewAgentSideConnection(agentFuncs{PromptFunc: func(ctx context.Context, p PromptRequest) (PromptResponse, error) {
			for i := 0; ; i++ {
				if err := asc.SessionUpdate(ctx, testSessionUpdate(p.SessionId, i)); err != nil {
					updateErr <- err
					return PromptResponse{}, ctx.Err()
				}
				sent.Add(1)
			}
		}}, a2cW, c2aR)
		negotiate(t, asc, allAdvertised())
		var held sync.Once
		csc := NewClientSideConnection(&clientFuncs{SessionUpdateFunc: func(context.Context, SessionNotification) error {
			held.Do(func() { <-release })
			return nil
		}}, c2aW, a2cR, WithMaxQueuedNotifications(2), WithNotificationOverflowPolicy(OverflowBlock))
		negotiate(t, csc, allAdvertised())
		t.Cleanup(func() {
			_ = csc.Close()
			_ = asc.Close()
		})

		answer := make(chan PromptResponse, 1)
		go func() {
			resp, err := csc.Prompt(t.Context(), PromptRequest{SessionId: "s"})
			if err != nil {
				t.Errorf("Prompt = %v", err)
			}
			answer <- resp
		}()
		synctest.Wait()
		if got := sent.Load(); got > 4 {
			t.Errorf("%d updates returned while the client was held, want at most 4: one held, two queued, one read and waiting", got)
		}
		stalled := sent.Load()
		if err := csc.Cancel(t.Context(), CancelNotification{SessionId: "s"}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		select {
		case err := <-updateErr:
			if !errors.Is(err, ErrPromptCancelled) && !errors.Is(err, context.Canceled) {
				t.Errorf("the blocked SessionUpdate returned %v, want the turn's cancel", err)
			}
		default:
			t.Fatal("session/cancel did not end the agent's blocked SessionUpdate")
		}
		if got := sent.Load(); got != stalled {
			t.Errorf("%d updates returned after the cancel, want none", got-stalled)
		}
		close(release)
		synctest.Wait()
		select {
		case resp := <-answer:
			if resp.StopReason != StopReasonCancelled {
				t.Errorf("stopReason %q, want cancelled", resp.StopReason)
			}
		default:
			t.Fatal("Prompt did not return once the client handled its updates")
		}
	})
}

// warnings records the Warn records a connection logs.
type warnings struct {
	mu   sync.Mutex
	recs []slog.Record
}

func (w *warnings) Enabled(context.Context, slog.Level) bool { return true }

func (w *warnings) Handle(_ context.Context, r slog.Record) error {
	if r.Level == slog.LevelWarn {
		w.mu.Lock()
		w.recs = append(w.recs, r.Clone())
		w.mu.Unlock()
	}
	return nil
}

func (w *warnings) WithAttrs([]slog.Attr) slog.Handler { return w }

func (w *warnings) WithGroup(string) slog.Handler { return w }

func (w *warnings) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.recs)
}

// TestOverflowBlockLogsLongPause requires a pause of 5 s to log one Warn with
// the queue's capacity, and a pause of 4 s to log none (D5).
func TestOverflowBlockLogsLongPause(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		w := &warnings{}
		b := newBlockPeer(t, false, WithMaxQueuedNotifications(1), WithNotificationOverflowPolicy(OverflowBlock), WithLogger(slog.New(w)))
		b.feed(notes(0, 3)...)
		synctest.Wait()
		time.Sleep(4 * time.Second)
		synctest.Wait()
		if n := w.count(); n != 0 {
			t.Fatalf("%d warnings after a pause of 4 s, want none", n)
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if n := w.count(); n != 1 {
			t.Fatalf("%d warnings after a pause of 5 s, want 1", n)
		}
		w.mu.Lock()
		r := w.recs[0]
		w.mu.Unlock()
		var capacity int64
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "capacity" {
				capacity = a.Value.Int64()
			}
			return true
		})
		if r.Message != "notification queue full; reading paused" || capacity != 1 {
			t.Errorf("logged %q with capacity %d, want the pause with capacity 1", r.Message, capacity)
		}
		b.releaseOnce()
		synctest.Wait()
		time.Sleep(10 * time.Second)
		synctest.Wait()
		if n := w.count(); n != 1 {
			t.Errorf("%d warnings after the pause ended, want still 1", n)
		}
	})
}

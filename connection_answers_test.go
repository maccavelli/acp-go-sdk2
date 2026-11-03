package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// The reader's answers (0049-PLAN-runtime-fixes.md, phase 1): the reader queues
// its answers to faults and refusals for the answering goroutine and never
// writes them itself, so it never waits on its peer
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, R1). Each runs in a
// synctest bubble.

// unreadPeer is a peer of a connection that writes to it and never reads
// what it writes back.
type unreadPeer struct {
	in  *io.PipeWriter
	out *io.PipeReader
}

// newUnreadPeer starts a connection from start whose peer never reads.
func newUnreadPeer(t *testing.T, start func(w io.Writer, r io.Reader)) unreadPeer {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	t.Cleanup(func() {
		_ = inW.Close()
		_ = outR.Close()
	})
	start(outW, inR)
	return unreadPeer{in: inW, out: outR}
}

func (p unreadPeer) send(t *testing.T, s string) {
	t.Helper()
	if _, err := io.WriteString(p.in, s+"\n"); err != nil {
		t.Fatal(err)
	}
}

// TestPeersRefusingEachOtherDoNotDeadlock connects two connections, each
// limited to one running request with that request held, and has each send
// the other eight. Each refuses seven, and its refusals are written while
// the other's are: the two readers never wait on each other, and once the
// held requests are released every call returns. Without a limit, the
// control, nothing is refused.
func TestPeersRefusingEachOtherDoNotDeadlock(t *testing.T) {
	t.Parallel()
	for _, limit := range []int{0, 1} {
		t.Run(fmt.Sprint("limit=", limit), func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				const n = 8
				entered, release := make(chan struct{}, 2*n), make(chan struct{})
				releaseOnce := sync.OnceFunc(func() { close(release) })
				t.Cleanup(releaseOnce)
				h := func(ctx context.Context, _ string, _ json.RawMessage) (any, *RequestError) {
					entered <- struct{}{}
					select {
					case <-release:
					case <-ctx.Done():
					}
					return map[string]any{}, nil
				}
				aR, bW := io.Pipe()
				bR, aW := io.Pipe()
				a := NewConnection(h, aW, aR, WithMaxConcurrentRequests(limit), quietLogs())
				b := NewConnection(h, bW, bR, WithMaxConcurrentRequests(limit), quietLogs())
				t.Cleanup(func() {
					_ = a.Close()
					_ = b.Close()
				})
				results := make(chan error, 2*n)
				for range n {
					for _, c := range []*Connection{a, b} {
						go func() {
							ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
							defer cancel()
							_, err := SendRequest[json.RawMessage](c, ctx, "_x/hold", nil)
							results <- err
						}()
					}
				}
				admitted := 2 * n
				if limit > 0 {
					admitted = 2 * limit
				}
				for range admitted {
					within(t, entered, "an admitted request")
				}
				synctest.Wait()
				releaseOnce()
				ok, refused := 0, 0
				for range 2 * n {
					err := within(t, results, "a call")
					re, isRE := errors.AsType[*RequestError](err)
					switch {
					case err == nil:
						ok++
					case errors.Is(err, context.DeadlineExceeded):
						t.Errorf("a call returned %v after the held requests were released: the readers waited on each other", err)
					case isRE && re.Code == -32603:
						refused++
					default:
						t.Errorf("a call returned %v, want its result or a refusal", err)
					}
				}
				if ok != admitted || refused != 2*n-admitted {
					t.Errorf("%d answered and %d refused, want %d and %d", ok, refused, admitted, 2*n-admitted)
				}
			})
		})
	}
}

// TestFaultAnswersDoNotBlockTheReader sends a connection invalid lines from
// a peer that does not read, then a notification: the notification reaches
// its handler while every answer is unwritten, and once the peer reads, the
// answers arrive, one per line, in order.
func TestFaultAnswersDoNotBlockTheReader(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const faults = 10
		handled := make(chan string, 1)
		var observed atomic.Int64
		p := newUnreadPeer(t, func(w io.Writer, r io.Reader) {
			c := NewConnection(func(_ context.Context, method string, _ json.RawMessage) (any, *RequestError) {
				handled <- method
				return nil, nil
			}, w, r, quietLogs(), WithObserver(func(o Observation) {
				if o.Kind == ObservedFault {
					observed.Add(1)
				}
			}))
			t.Cleanup(func() { _ = c.Close() })
		})
		// The lines are sent from their own goroutine: a reader that waits
		// on its answers stops reading, and the test then reports it.
		go func() {
			for range faults {
				_, _ = io.WriteString(p.in, "{oops\n")
			}
			_, _ = io.WriteString(p.in, `{"jsonrpc":"2.0","method":"_x/n"}`+"\n")
		}()
		if got := within(t, handled, "the notification after the faults"); got != "_x/n" {
			t.Errorf("handled %q, want _x/n", got)
		}
		sc := bufio.NewScanner(p.out)
		for i := range faults {
			if !sc.Scan() {
				t.Fatalf("answer %d: %v", i, sc.Err())
			}
			var a answer
			if err := json.Unmarshal(sc.Bytes(), &a); err != nil || a.Error == nil || a.Error.Code != -32700 || string(a.ID) != jsonNull {
				t.Errorf("answer %d: %s, want -32700 with id null", i, sc.Bytes())
			}
		}
		synctest.Wait()
		if n := observed.Load(); n != faults {
			t.Errorf("observed %d faults, want %d, each once its answer was written", n, faults)
		}
	})
}

// TestFaultAnswersWrittenAtEndOfInput sends a connection invalid lines and
// ends its input at once: the end of input waits for the queued answers, as
// it does for the requests received before it, and the peer reads every one
// (0019-PLAN-issue-fixes.md, B23).
func TestFaultAnswersWrittenAtEndOfInput(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const faults = 50
		var c *Connection
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			c = NewConnection(nil, w, r, quietLogs())
		})
		for range faults {
			p.send(`{oops`)
		}
		_ = p.w.(io.Closer).Close()
		for i := range faults {
			if a := p.next(fmt.Sprint("answer ", i)); a.Error == nil || a.Error.Code != -32700 {
				t.Errorf("answer %d: %+v, want -32700", i, a)
			}
		}
		within(t, c.Done(), "the end of the connection")
		if err := c.Err(); !EndedCleanly(err) {
			t.Errorf("Err() = %v, want the peer's clean end of output", err)
		}
	})
}

// TestUnreadAnswersCloseTheConnection sends a connection, from a peer that
// does not read, as many invalid lines as it holds answers for, and then
// one more: the connection stays open through the first maxPendingAnswers,
// and the next closes it, its Err naming the peer that does not read, not
// ended cleanly. Every line is observed once, and Wait returns.
func TestUnreadAnswersCloseTheConnection(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		var c *Connection
		var observed atomic.Int64
		p := newUnreadPeer(t, func(w io.Writer, r io.Reader) {
			c = NewConnection(nil, w, r, quietLogs(), WithObserver(func(o Observation) {
				if o.Kind == ObservedFault {
					observed.Add(1)
				}
			}))
		})
		for range maxPendingAnswers {
			p.send(t, `{oops`)
		}
		synctest.Wait()
		if err := c.Err(); err != nil {
			t.Fatalf("the connection ended with %d answers unwritten: %v", maxPendingAnswers, err)
		}
		p.send(t, `{oops`)
		within(t, c.Done(), "the end of the connection")
		err := c.Err()
		if !errors.Is(err, ErrConnectionClosed) || !errors.Is(err, errUnreadAnswers) {
			t.Errorf("Err() = %v, want ErrConnectionClosed wrapping errUnreadAnswers", err)
		}
		if EndedCleanly(err) {
			t.Errorf("EndedCleanly(%v) = true, want false", err)
		}
		waited := make(chan error, 1)
		go func() { waited <- c.Wait() }()
		within(t, waited, "Wait")
		if n := observed.Load(); n != maxPendingAnswers+1 {
			t.Errorf("observed %d faults, want %d, each once", n, maxPendingAnswers+1)
		}
	})
}

package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// trackedWriter is a peer's input that counts the writes it receives and
// records whether it was closed.
type trackedWriter struct {
	io.Writer
	writes atomic.Int64
	closed atomic.Bool
}

func (w *trackedWriter) Write(p []byte) (int, error) {
	w.writes.Add(1)
	return w.Writer.Write(p)
}

func (w *trackedWriter) Close() error {
	w.closed.Store(true)
	if c, ok := w.Writer.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// readLines sends each line read from r to the returned channel, and the
// scanner's error to the second when r ends.
func readLines(r io.Reader) (<-chan string, <-chan error) {
	lines, end := make(chan string, 16), make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			lines <- sc.Text()
		}
		end <- sc.Err()
	}()
	return lines, end
}

// TestSendHonoursContextWhileWriteBlocks requires a send to return when its
// context ends although the peer does not read, both the send whose write
// has begun and one waiting behind it, and the connection to work once the
// peer reads (0019-PLAN-api-decisions.md, B16).
func TestSendHonoursContextWhileWriteBlocks(t *testing.T) {
	t.Parallel()
	outR, outW := io.Pipe()
	t.Cleanup(func() { _ = outR.Close() })
	c := NewConnection(nil, outW, idleReader(t))

	for _, what := range []string{"a send whose write blocks", "a send behind it"} {
		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		returned := make(chan error, 1)
		go func() { returned <- c.SendNotification(ctx, "_test/n", map[string]any{"what": what}) }()
		select {
		case err := <-returned:
			checkLocal(t, what, err, context.DeadlineExceeded, -32603)
		case <-time.After(time.Second):
			cancel()
			t.Fatalf("%s did not return within 1s of its 200ms deadline", what)
		}
		cancel()
	}

	// The first write had begun, so its message reaches the peer; the second
	// was never handed to the writer.
	lines, _ := readLines(outR)
	if err := c.SendNotification(t.Context(), "_test/n", map[string]any{"what": "after"}); err != nil {
		t.Fatalf("a send once the peer reads: %v", err)
	}
	for _, want := range []string{"a send whose write blocks", "after"} {
		if line := within(t, lines, want); !strings.Contains(line, want) {
			t.Errorf("the peer read %s, want the message %q", line, want)
		}
	}
}

// TestCloseEndsConnection requires Close to end the connection: Done closes,
// a pending call returns ErrConnectionClosed, a later send fails without
// writing, the reader and the writer are closed, and a second Close returns
// nil (0019-PLAN-api-decisions.md, B17).
func TestCloseEndsConnection(t *testing.T) {
	t.Parallel()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	t.Cleanup(func() {
		_ = inW.Close()
		_ = outR.Close()
	})
	out := &trackedWriter{Writer: outW}
	c := NewConnection(nil, out, inR)
	lines, end := readLines(outR)

	done := make(chan error, 1)
	go func() {
		_, err := SendRequest[json.RawMessage](c, t.Context(), "_test/r", map[string]any{})
		done <- err
	}()
	within(t, lines, "the request")
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	within(t, c.Done(), "Done")
	checkLocal(t, "a call pending at Close", within(t, done, "the pending call"), ErrConnectionClosed, -32603)

	before := out.writes.Load()
	err := c.SendNotification(t.Context(), "_test/n", map[string]any{})
	checkLocal(t, "a send after Close", err, ErrConnectionClosed, -32603)
	if n := out.writes.Load(); n != before {
		t.Errorf("a send after Close wrote %d times", n-before)
	}

	if !out.closed.Load() {
		t.Error("Close left the writer open")
	}
	if err := within(t, end, "the end of the output"); err != nil {
		t.Errorf("the output ended with %v", err)
	}
	if _, err := inW.Write([]byte("{}\n")); !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("Close left the reader open: a peer's write returned %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("a second Close: %v, want nil", err)
	}
}

// failingCloser is a writer whose Close fails.
type failingCloser struct{ io.Writer }

var errCloseFailed = errors.New("close failed")

func (failingCloser) Close() error { return errCloseFailed }

// TestCloseReturnsErrorOnce requires the Close that closes the connection to
// return its transport's error, and a later Close to return nil
// (0019-PLAN-api-decisions.md, step 9's second deviation).
func TestCloseReturnsErrorOnce(t *testing.T) {
	t.Parallel()
	c := NewConnection(nil, failingCloser{io.Discard}, idleReader(t))
	if err := c.Close(); !errors.Is(err, errCloseFailed) {
		t.Errorf("the first Close: %v, want the writer's error", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("a second Close: %v, want nil", err)
	}
}

// TestSendAfterPeerEndFails requires a send after the peer ended the
// connection to fail with ErrPeerDisconnected and write nothing
// (0019-PLAN-api-decisions.md, B17).
func TestSendAfterPeerEndFails(t *testing.T) {
	t.Parallel()
	inR, inW := io.Pipe()
	out := &trackedWriter{Writer: io.Discard}
	c := NewConnection(nil, out, inR)
	_ = inW.Close()
	within(t, c.Done(), "the end of the connection")

	err := c.SendNotification(t.Context(), "_test/n", map[string]any{})
	checkLocal(t, "a notification after the peer ended", err, ErrPeerDisconnected, -32603)
	_, err = SendRequest[json.RawMessage](c, t.Context(), "_test/r", map[string]any{})
	checkLocal(t, "a request after the peer ended", err, ErrPeerDisconnected, -32603)
	if errors.Is(err, ErrConnectionClosed) {
		t.Errorf("a request after the peer ended: %v wraps ErrConnectionClosed", err)
	}
	if n := out.writes.Load(); n != 0 {
		t.Errorf("%d writes after the peer ended, want 0", n)
	}
}

// TestOverflowCloseClosesTransport requires OverflowCloseConnection to close
// the connection as Close does: its reader is closed, and a later send fails
// with ErrConnectionClosed, wrapping the overflow that closed it
// (0019-PLAN-api-decisions.md, B17). The policy is named, since it is no
// longer the default (0044-MADR-notification-queue-back-pressure.md, D9).
func TestOverflowCloseClosesTransport(t *testing.T) {
	t.Parallel()
	f := newOverflowFixture(t, 1, WithNotificationOverflowPolicy(OverflowCloseConnection))
	for n := 1; n <= 2; n++ { // 1 fills the queue, 2 overflows
		f.notify(t, n)
	}
	within(t, f.c.Done(), "the overflow's close")

	// A write to a pipe nobody reads blocks until the reader is closed.
	written := make(chan error, 1)
	go func() {
		_, err := io.WriteString(f.incoming, "{}\n")
		written <- err
	}()
	if err := within(t, written, "the reader's close"); !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("the reader is open after the overflow: a peer's write returned %v", err)
	}

	err := f.c.SendNotification(t.Context(), "_test/n", map[string]any{})
	checkLocal(t, "a send after the overflow", err, ErrConnectionClosed, -32603)
	if !errors.Is(err, ErrNotificationQueueOverflow) {
		t.Errorf("a send after the overflow: %v does not wrap the overflow", err)
	}
	close(f.release)
	waitForNotificationBarrierDrain(t, f.c, overflowWait)
}

// readOnly is a reader with no Close, such as a bufio.Reader or an
// application's own type: Close cannot end a read on it.
type readOnly struct{ r io.Reader }

func (o readOnly) Read(p []byte) (int, error) { return o.r.Read(p) }

// TestCloseEndsNotificationHandlers requires Close to end the context of every
// running handler at once, a notification's as well as a request's, with the
// cause ErrConnectionClosed, whether or not Close can close the reader
// (0023-MADR-connection-lifetime-and-test-leaks.md, C1). It runs in a
// synctest bubble, so "at once" is no time.
func TestCloseEndsNotificationHandlers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		reader func(*io.PipeReader) io.Reader
	}{
		{"a reader Close closes", func(r *io.PipeReader) io.Reader { return r }},
		{"a reader Close cannot close", func(r *io.PipeReader) io.Reader { return readOnly{r} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				inR, inW := io.Pipe()
				// Ends the input, so that the reader Close cannot close ends
				// with the bubble.
				t.Cleanup(func() { _ = inW.Close() })
				var mu sync.Mutex
				causes := map[string]error{}
				c := NewConnection(func(ctx context.Context, method string, _ json.RawMessage) (any, *RequestError) {
					<-ctx.Done()
					mu.Lock()
					causes[method] = context.Cause(ctx)
					mu.Unlock()
					return nil, nil
				}, io.Discard, tc.reader(inR))
				for _, line := range []string{
					`{"jsonrpc":"2.0","method":"_test/notification","params":{}}`,
					`{"jsonrpc":"2.0","id":1,"method":"_test/request","params":{}}`,
				} {
					if _, err := io.WriteString(inW, line+"\n"); err != nil {
						t.Fatal(err)
					}
				}
				synctest.Wait() // both handlers wait on their contexts
				start := time.Now()
				if err := c.Close(); err != nil {
					t.Fatal(err)
				}
				synctest.Wait() // every handler whose context Close ended has returned
				if waited := time.Since(start); waited != 0 {
					t.Errorf("the handlers returned %v after Close, want at once", waited)
				}
				mu.Lock()
				defer mu.Unlock()
				for _, method := range []string{"_test/notification", "_test/request"} {
					cause, ok := causes[method]
					if !ok {
						t.Errorf("the %s handler's context was still live after Close", method)
						continue
					}
					if !errors.Is(cause, ErrConnectionClosed) {
						t.Errorf("the %s handler's context ended with %v, want ErrConnectionClosed", method, cause)
					}
				}
			})
		})
	}
}

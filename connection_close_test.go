package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
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
// (0019-PLAN-api-decisions.md, B17).
func TestOverflowCloseClosesTransport(t *testing.T) {
	t.Parallel()
	f := newOverflowFixture(t, 1)
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
	if !errors.Is(err, errNotificationQueueOverflow) {
		t.Errorf("a send after the overflow: %v does not wrap the overflow", err)
	}
	close(f.release)
	waitForNotificationBarrierDrain(t, f.c, overflowWait)
}

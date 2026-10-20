package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"testing"
	"time"
)

// errOnRead is a reader whose every read fails with err.
type errOnRead struct{ err error }

func (r errOnRead) Read([]byte) (int, error) { return 0, r.err }

// ended waits for c's end and returns Err, read twice to require the same
// answer each time.
func ended(t *testing.T, c *Connection) error {
	t.Helper()
	within(t, c.Done(), "the connection's end")
	err := c.Err()
	if again := c.Err(); !errors.Is(again, err) {
		t.Errorf("Err returned %v, then %v", err, again)
	}
	return err
}

// TestErr requires Err to be nil until the connection ends and then to say
// why it ended, the same each time, with the cause each end gives readable by
// errors.Is: Close, the peer's end of output (io.EOF, which a later send's
// error wraps too), a read's error, and a notification overflow; and, after
// the peer's end, a later Close not to change it while a call then still
// returns ErrConnectionClosed
// (0026-MADR-connection-end-cause-and-wait.md, E1 to E6).
func TestErr(t *testing.T) {
	t.Parallel()
	is := func(t *testing.T, err error, want []error, not []error) {
		t.Helper()
		for _, w := range want {
			if !errors.Is(err, w) {
				t.Errorf("Err = %v, want it to wrap %v", err, w)
			}
		}
		for _, n := range not {
			if errors.Is(err, n) {
				t.Errorf("Err = %v, want it not to wrap %v", err, n)
			}
		}
	}
	t.Run("before the end", func(t *testing.T) {
		t.Parallel()
		c := NewConnection(nil, io.Discard, idleReader(t))
		if err := c.Err(); err != nil {
			t.Errorf("Err before the end = %v, want nil", err)
		}
		_ = c.Close()
	})
	t.Run("Close", func(t *testing.T) {
		t.Parallel()
		c := NewConnection(nil, io.Discard, idleReader(t))
		_ = c.Close()
		err := ended(t, c)
		if err == nil || err.Error() != "acp: connection closed" {
			t.Errorf("Err = %v, want acp: connection closed", err)
		}
		is(t, err, []error{ErrConnectionClosed}, []error{ErrPeerDisconnected, io.EOF})
	})
	t.Run("the peer's end of output", func(t *testing.T) {
		t.Parallel()
		pr, pw := io.Pipe()
		c := NewConnection(nil, io.Discard, pr)
		_ = pw.Close()
		err := ended(t, c)
		if err == nil || err.Error() != "acp: peer disconnected: peer connection closed" {
			t.Errorf("Err = %v, want acp: peer disconnected: peer connection closed", err)
		}
		is(t, err, []error{ErrPeerDisconnected, io.EOF}, []error{ErrConnectionClosed})
		send := c.SendNotification(context.Background(), "_x/n", map[string]any{})
		if !errors.Is(send, ErrPeerDisconnected) || !errors.Is(send, io.EOF) {
			t.Errorf("a send after the peer's end = %v, want ErrPeerDisconnected and io.EOF", send)
		}
	})
	t.Run("a read error", func(t *testing.T) {
		t.Parallel()
		reset := errors.New("read: connection reset")
		c := NewConnection(nil, io.Discard, errOnRead{reset})
		err := ended(t, c)
		is(t, err, []error{ErrPeerDisconnected, reset}, []error{ErrConnectionClosed, io.EOF})
	})
	t.Run("notification overflow", func(t *testing.T) {
		t.Parallel()
		pr, pw := io.Pipe()
		t.Cleanup(func() { _ = pw.Close() })
		h := func(ctx context.Context, _ string, _ json.RawMessage) (any, *RequestError) {
			<-ctx.Done()
			return nil, nil
		}
		c := NewConnection(h, io.Discard, pr, WithMaxQueuedNotifications(1))
		go func() {
			for range 4 {
				if _, err := pw.Write([]byte(`{"jsonrpc":"2.0","method":"_x/n","params":{}}` + "\n")); err != nil {
					return
				}
			}
		}()
		err := ended(t, c)
		if err == nil || err.Error() != "acp: connection closed: notification queue overflow" {
			t.Errorf("Err = %v, want acp: connection closed: notification queue overflow", err)
		}
		is(t, err, []error{ErrConnectionClosed, ErrNotificationQueueOverflow}, []error{ErrPeerDisconnected})
	})
	t.Run("the peer's end, then Close", func(t *testing.T) {
		t.Parallel()
		pr, pw := io.Pipe()
		c := NewConnection(nil, io.Discard, pr)
		_ = pw.Close()
		before := ended(t, c)
		_ = c.Close()
		if after := c.Err(); !errors.Is(after, before) {
			t.Errorf("Err after a late Close = %v, want the peer's end, %v", after, before)
		}
		is(t, c.Err(), []error{ErrPeerDisconnected, io.EOF}, []error{ErrConnectionClosed})
		if err := c.SendNotification(context.Background(), "_x/n", map[string]any{}); !errors.Is(err, ErrConnectionClosed) {
			t.Errorf("a send after the late Close = %v, want ErrConnectionClosed", err)
		}
	})
}

// waitPipe is a reader Close can close, a type no other test uses, so a
// goroutine blocked in its Read is this test's.
type waitPipe struct{ *io.PipeReader }

// waitStuck is a reader with no Close whose reads block until release closes.
type waitStuck struct{ release chan struct{} }

func (r waitStuck) Read([]byte) (int, error) { <-r.release; return 0, io.EOF }

// waitIgnoringHandler is a handler that ignores its context until release
// closes.
func waitIgnoringHandler(started, release chan struct{}) MethodHandler {
	return func(context.Context, string, json.RawMessage) (any, *RequestError) {
		close(started)
		<-release
		return map[string]any{}, nil
	}
}

// stacksNaming reports whether a goroutine's stack names fn.
func stacksNaming(fn string) bool {
	var b bytes.Buffer
	_ = pprof.Lookup("goroutine").WriteTo(&b, 2)
	return strings.Contains(b.String(), fn)
}

// waiting starts c.Wait on a goroutine and returns where its result arrives.
func waiting(c *Connection) <-chan error {
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	return done
}

// stillWaiting fails the test when done delivers within 200 ms.
func stillWaiting(t *testing.T, done <-chan error, why string) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("Wait returned %v while %s", err, why)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestWait requires Wait to block until the connection has ended and every
// goroutine it started has returned: before the end; with a reader Close
// closes, none left once it returns; with a reader Close cannot close, until
// its Read returns; with a handler that ignores its context, until it
// returns. It returns Err (0026-MADR-connection-end-cause-and-wait.md).
func TestWait(t *testing.T) {
	t.Run("before the end", func(t *testing.T) {
		c := NewConnection(nil, io.Discard, idleReader(t))
		done := waiting(c)
		stillWaiting(t, done, "the connection runs")
		_ = c.Close()
		if err := within(t, done, "Wait after Close"); !errors.Is(err, ErrConnectionClosed) {
			t.Errorf("Wait = %v, want ErrConnectionClosed", err)
		}
	})
	t.Run("a reader Close closes", func(t *testing.T) {
		pr, pw := io.Pipe()
		t.Cleanup(func() { _ = pw.Close() })
		c := NewConnection(nil, io.Discard, waitPipe{pr})
		_ = c.Close()
		err := within(t, waiting(c), "Wait after Close")
		if !errors.Is(err, c.Err()) {
			t.Errorf("Wait = %v, want Err, %v", err, c.Err())
		}
		if stacksNaming("waitPipe") {
			t.Error("a goroutine still reads the closed reader after Wait")
		}
	})
	t.Run("a reader Close cannot close", func(t *testing.T) {
		stuck := waitStuck{release: make(chan struct{})}
		c := NewConnection(nil, io.Discard, stuck)
		_ = c.Close()
		done := waiting(c)
		stillWaiting(t, done, "the reader's Read blocks")
		close(stuck.release)
		within(t, done, "Wait once the Read returns")
		if stacksNaming("waitStuck") {
			t.Error("a goroutine still reads after Wait")
		}
	})
	t.Run("a handler that ignores its context", func(t *testing.T) {
		pr, pw := io.Pipe()
		t.Cleanup(func() { _ = pw.Close() })
		started, release := make(chan struct{}), make(chan struct{})
		c := NewConnection(waitIgnoringHandler(started, release), io.Discard, pr)
		go func() { _, _ = pw.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"_x/r","params":{}}` + "\n")) }()
		within(t, started, "the handler's start")
		_ = c.Close()
		done := waiting(c)
		stillWaiting(t, done, "the handler runs")
		close(release)
		within(t, done, "Wait once the handler returns")
		if stacksNaming("waitIgnoringHandler") {
			t.Error("the handler's goroutine is left after Wait")
		}
	})
}

// TestGoStatementsCounted fails on a go statement in the package's non-test
// files: every goroutine the connection starts goes through its wait group's
// Go, so Wait can wait for it (0026-MADR-connection-end-cause-and-wait.md).
func TestGoStatementsCounted(t *testing.T) {
	t.Parallel()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if g, ok := n.(*ast.GoStmt); ok {
				t.Errorf("%s: a go statement; start it with the connection's goroutines.Go", fset.Position(g.Go))
			}
			return true
		})
	}
}

package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"regexp"
	"runtime/pprof"
	"strconv"
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
// errors.Is: Close, the peer's end of output (io.EOF, which a later
// notification's error wraps too, and a later request's does not), a read's
// error, and a notification overflow; and, after the peer's end, a later
// Close not to change it, while a notification then returns
// ErrConnectionClosed and a request still ErrPeerDisconnected
// (0026-MADR-connection-end-cause-and-wait.md, E1 to E6, and its amendment
// during the PLAN's step 6).
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
		req := c.SendRequestNoResult(context.Background(), "_x/r", map[string]any{})
		if !errors.Is(req, ErrPeerDisconnected) || errors.Is(req, io.EOF) {
			t.Errorf("a request after the peer's end = %v, want ErrPeerDisconnected, not io.EOF", req)
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
		req := c.SendRequestNoResult(context.Background(), "_x/r", map[string]any{})
		if !errors.Is(req, ErrPeerDisconnected) || errors.Is(req, ErrConnectionClosed) {
			t.Errorf("a request after the late Close = %v, want ErrPeerDisconnected, not ErrConnectionClosed", req)
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

// TestEndedCleanly requires EndedCleanly to be true for the Err of each clean
// end, Close and the peer's end of output, wrapped or not, and false for each
// other end, a notification overflow's close and a read's error; for a
// request's error after the peer's end, which names the peer but not its EOF;
// for an error that is no connection's end; and for nil
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 7).
func TestEndedCleanly(t *testing.T) {
	t.Parallel()
	closed := NewConnection(nil, io.Discard, idleReader(t))
	_ = closed.Close()
	pr, pw := io.Pipe()
	eof := NewConnection(nil, io.Discard, pr)
	_ = pw.Close()
	reset := NewConnection(nil, io.Discard, errOnRead{errors.New("read: connection reset")})
	opr, opw := io.Pipe()
	t.Cleanup(func() { _ = opw.Close() })
	block := func(ctx context.Context, _ string, _ json.RawMessage) (any, *RequestError) {
		<-ctx.Done()
		return nil, nil
	}
	overflow := NewConnection(block, io.Discard, opr, WithMaxQueuedNotifications(1))
	go func() {
		for range 4 {
			if _, err := opw.Write([]byte(`{"jsonrpc":"2.0","method":"_x/n","params":{}}` + "\n")); err != nil {
				return
			}
		}
	}()
	eofErr := ended(t, eof)
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"Close", ended(t, closed), true},
		{"Close, wrapped", fmt.Errorf("session: %w", ended(t, closed)), true},
		{"the peer's end of output", eofErr, true},
		{"Wait after the peer's end", eof.Wait(), true},
		{"a read's error", ended(t, reset), false},
		{"a notification overflow", ended(t, overflow), false},
		{"a request after the peer's end", eof.SendRequestNoResult(context.Background(), "_x/r", map[string]any{}), false},
		{"io.EOF alone", io.EOF, false},
		{"another error", errors.New("acp: something else"), false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		if got := EndedCleanly(c.err); got != c.want {
			t.Errorf("EndedCleanly(%v), %s, = %v, want %v", c.err, c.name, got, c.want)
		}
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

// TestSideConnectionsErrAndWait requires each side connection's Err and Wait
// to return its connection's: nil before the end, then the peer's end of
// output (0026-MADR-connection-end-cause-and-wait.md, question 6).
func TestSideConnectionsErrAndWait(t *testing.T) {
	t.Parallel()
	type side interface {
		Err() error
		Wait() error
		Done() <-chan struct{}
		Connection() *Connection
	}
	for name, open := range map[string]func(io.Reader) side{
		"agent":  func(r io.Reader) side { return NewAgentSideConnection(&agentFuncs{}, io.Discard, r) },
		"client": func(r io.Reader) side { return NewClientSideConnection(&clientFuncs{}, io.Discard, r) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			pr, pw := io.Pipe()
			s := open(pr)
			if err := s.Err(); err != nil {
				t.Errorf("Err before the end = %v, want nil", err)
			}
			_ = pw.Close()
			within(t, s.Done(), "the connection's end")
			want := s.Connection().Err()
			if want == nil || !errors.Is(want, io.EOF) {
				t.Fatalf("the connection's Err = %v, want the peer's end", want)
			}
			if err := s.Err(); !errors.Is(err, want) {
				t.Errorf("Err = %v, want the connection's, %v", err, want)
			}
			if err := within(t, waitingFor(s.Wait), "Wait"); !errors.Is(err, want) {
				t.Errorf("Wait = %v, want the connection's, %v", err, want)
			}
		})
	}
}

// waitingFor runs wait on a goroutine and returns where its result arrives.
func waitingFor(wait func() error) <-chan error {
	done := make(chan error, 1)
	go func() { done <- wait() }()
	return done
}

// TestWaitContext requires WaitContext to return context.Cause(ctx) when ctx
// ends before the connection's goroutines have returned, leaving the
// connection running; Wait's error once they have, on each side connection's
// Connection too; and its calls, however many, to share one goroutine, which is gone
// once Wait has returned (0041-MADR-close-the-gaps-the-consumers-wait-on.md,
// item 6).
func TestWaitContext(t *testing.T) {
	t.Parallel()
	t.Run("ctx ends first", func(t *testing.T) {
		t.Parallel()
		c := NewConnection(nil, io.Discard, idleReader(t))
		why := errors.New("the test's bound")
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(why)
		if err := c.WaitContext(ctx); !errors.Is(err, why) {
			t.Errorf("WaitContext with an ended ctx = %v, want its cause, %v", err, why)
		}
		if err := c.Err(); err != nil {
			t.Errorf("Err after WaitContext = %v, want nil: the connection is left running", err)
		}
		_ = c.Close()
		if err := c.WaitContext(context.Background()); !errors.Is(err, ErrConnectionClosed) {
			t.Errorf("WaitContext after Close = %v, want ErrConnectionClosed", err)
		}
	})
	for name, open := range map[string]func(io.Reader) *Connection{
		"the connection": func(r io.Reader) *Connection { return NewConnection(nil, io.Discard, r) },
		"the agent side's": func(r io.Reader) *Connection {
			return NewAgentSideConnection(&agentFuncs{}, io.Discard, r).Connection()
		},
		"the client side's": func(r io.Reader) *Connection {
			return NewClientSideConnection(&clientFuncs{}, io.Discard, r).Connection()
		},
	} {
		t.Run(name+" ends first", func(t *testing.T) {
			t.Parallel()
			pr, pw := io.Pipe()
			s := open(pr)
			_ = pw.Close()
			err := s.WaitContext(t.Context())
			if !errors.Is(err, ErrPeerDisconnected) || !errors.Is(err, io.EOF) {
				t.Errorf("WaitContext = %v, want the peer's end", err)
			}
			if want := s.Wait(); !errors.Is(err, want) {
				t.Errorf("WaitContext = %v, want Wait's error, %v", err, want)
			}
		})
	}
	t.Run("one goroutine", func(t *testing.T) {
		t.Parallel()
		c := NewConnection(nil, io.Discard, idleReader(t))
		const label = "TestWaitContext/one goroutine"
		pprof.Do(context.Background(), pprof.Labels("test", label), func(context.Context) {
			ended, cancel := context.WithCancel(context.Background())
			cancel()
			for range 100 {
				_ = c.WaitContext(ended)
			}
		})
		if n := waiters(label); n != 1 {
			t.Errorf("100 calls of WaitContext left %d waiting goroutines, want 1", n)
		}
		_ = c.Close()
		_ = c.WaitContext(context.Background())
		deadline := time.Now().Add(2 * time.Second)
		for waiters(label) != 0 {
			if time.Now().After(deadline) {
				t.Fatal("WaitContext's goroutine still runs 2s after Wait returned")
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
}

// waiters counts the goroutines in waitDone that carry the label "test" with
// value, which a goroutine inherits from the one that starts it.
func waiters(value string) int {
	var b bytes.Buffer
	_ = pprof.Lookup("goroutine").WriteTo(&b, 1)
	n := 0
	for group := range strings.SplitSeq(b.String(), "\n\n") {
		if !strings.Contains(group, `"test":"`+value+`"`) || !strings.Contains(group, "waitDone") {
			continue
		}
		// The first group follows the profile's header line.
		if m := profileCount.FindStringSubmatch(group); m != nil {
			count, _ := strconv.Atoi(m[1])
			n += count
		}
	}
	return n
}

// profileCount is the count line of a group in a goroutine profile of
// debug 1.
var profileCount = regexp.MustCompile(`(?m)^(\d+) @`)

// TestGoStatementsCounted fails on a go statement in the package's non-test
// files: every goroutine the connection starts goes through its wait group's
// Go, so Wait can wait for it (0026-MADR-connection-end-cause-and-wait.md).
// The one exception is WaitContext's waiter, which runs Wait and so cannot be
// one of the goroutines Wait waits for: exactly one go statement, in
// connection.go's waitDone method
// (0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phase 6).
func TestGoStatementsCounted(t *testing.T) {
	t.Parallel()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	waiters := 0
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			waiter := ok && name == "connection.go" && fn.Recv != nil && fn.Name.Name == "waitDone"
			ast.Inspect(d, func(n ast.Node) bool {
				g, ok := n.(*ast.GoStmt)
				switch {
				case !ok:
				case waiter:
					waiters++
				default:
					t.Errorf("%s: a go statement; start it with the connection's goroutines.Go", fset.Position(g.Go))
				}
				return true
			})
		}
	}
	if waiters != 1 {
		t.Errorf("%d go statements in connection.go's waitDone, want 1: WaitContext's waiter", waiters)
	}
}

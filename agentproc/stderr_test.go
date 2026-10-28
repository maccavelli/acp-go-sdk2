package agentproc

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
)

// TestTailWriterCaps requires the tail to keep at most its byte cap and its
// line cap, from a line's beginning, a line longer than the byte cap keeping
// its end, whatever the writes' boundaries
// (0042-MADR-in-process-agent-runner.md, item 3).
func TestTailWriterCaps(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		bytes, lines int
		writes       []string
		want         string
	}{
		{"the byte cap at a line's beginning", 10, 0, []string{"aaaa\nbbbb\ncccc\n"}, "bbbb\ncccc\n"},
		{"the byte cap mid-line, moved to the next line", 12, 0, []string{"aaaa\nbbbb\ncccc\n"}, "bbbb\ncccc\n"},
		{"a line longer than the byte cap keeps its end", 4, 0, []string{"0123456789\n"}, "789\n"},
		{"a long unended line keeps its end", 4, 0, []string{"x\n", "0123456789"}, "6789"},
		{"the line cap", 0, 2, []string{"a\nb\nc\n"}, "b\nc\n"},
		{"the line cap counts an unended line", 0, 2, []string{"a\nb\nc"}, "b\nc"},
		{"both caps, the byte cap the tighter", 6, 3, []string{"aa\nbb\ncc\ndd\n"}, "cc\ndd\n"},
		{"both caps, the line cap the tighter", 100, 1, []string{"aa\nbb\ncc\n"}, "cc\n"},
		{"a line split across writes", 0, 1, []string{"aa", "a\nbb", "b\n"}, "bbb\n"},
		{"under both caps, everything", 100, 10, []string{"one\n", "two\n"}, "one\ntwo\n"},
	} {
		w := &tailWriter{bytes: tc.bytes, lines: tc.lines}
		for _, s := range tc.writes {
			if n, err := w.Write([]byte(s)); n != len(s) || err != nil {
				t.Fatalf("%s: Write = %d, %v", tc.name, n, err)
			}
		}
		if got := w.tail(); got != tc.want {
			t.Errorf("%s: the tail is %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestTailWriterForwards requires every byte to reach the caller's writer,
// and its result to be the Write's.
func TestTailWriterForwards(t *testing.T) {
	t.Parallel()
	next := &capture{}
	w := &tailWriter{next: next, bytes: 4}
	for _, s := range []string{"first line\n", "second\n"} {
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	if got := string(next.bytes()); got != "first line\nsecond\n" {
		t.Errorf("the caller's writer received %q, want every byte", got)
	}
	failing := &tailWriter{next: failing{}, bytes: 4}
	if _, err := failing.Write([]byte("x\n")); err == nil {
		t.Error("Write = nil error, want the caller's writer's")
	}
}

// startStderrFail starts the "stderr-fail" child with the caps and the join,
// the caller's Stderr set to next, and returns it once it has exited.
func startStderrFail(t *testing.T, bytes, lines int, join bool, next io.Writer) (*Process, error) {
	t.Helper()
	cmd := agentCmd("stderr-fail")
	cmd.Stderr = next
	p, err := (&Command{Cmd: cmd, Grace: testGrace, StderrTail: bytes, StderrTailLines: lines, JoinStderrTail: join}).Start(&testClient{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), readyWait)
	defer cancel()
	return p, p.WaitContext(ctx)
}

// TestStderrTailOfFailedAgent requires an agent that writes to its stderr and
// exits 1 to leave its last lines in StderrTail, the caller's Stderr to
// receive every line, and, under JoinStderrTail, Wait's error to hold the
// tail beside the exit status.
func TestStderrTailOfFailedAgent(t *testing.T) {
	t.Parallel()
	next := &capture{}
	p, err := startStderrFail(t, 1<<10, 3, true, next)
	const want = "line 98\nline 99\nline 100\n"
	if got := p.StderrTail(); got != want {
		t.Errorf("StderrTail = %q, want %q", got, want)
	}
	if got := strings.Count(string(next.bytes()), "\n"); got != 100 {
		t.Errorf("the caller's Stderr received %d lines, want all 100", got)
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Errorf("Wait = %v, want the exit status 1", err)
	}
	var se *StderrError
	if !errors.As(err, &se) || se.Tail != want {
		t.Errorf("Wait = %v, want a *StderrError holding %q", err, want)
	}
}

// TestStderrTailNotJoined requires Wait's error to hold no tail without
// JoinStderrTail, while StderrTail still returns it.
func TestStderrTailNotJoined(t *testing.T) {
	t.Parallel()
	p, err := startStderrFail(t, 64, 0, false, nil)
	if _, ok := errors.AsType[*StderrError](err); ok {
		t.Errorf("Wait = %v, want no *StderrError without JoinStderrTail", err)
	}
	if got := p.StderrTail(); !strings.HasSuffix(got, "line 100\n") || len(got) > 64 {
		t.Errorf("StderrTail = %q, want the stream's last 64 bytes or fewer, from a line", got)
	}
}

// TestStderrTailOfKilledAgent requires a killed agent's Wait error, under
// JoinStderrTail, still to be ErrKilled, beside the tail.
func TestStderrTailOfKilledAgent(t *testing.T) {
	t.Parallel()
	cmd := agentCmd("stubborn")
	w := &readyWriter{ready: make(chan struct{})}
	cmd.Stderr = w
	p, err := (&Command{Cmd: cmd, Grace: testGrace, StderrTailLines: 5, JoinStderrTail: true}).Start(&testClient{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	awaitReady(t, "stubborn", w.ready)
	_, err = within(t, slack, "Kill", p.Kill)
	var se *StderrError
	if !errors.Is(err, ErrKilled) || !errors.As(err, &se) || se.Tail != "ready\n" {
		t.Errorf("Kill = %v, want ErrKilled beside a *StderrError holding %q", err, "ready\n")
	}
}

// TestInProcessHasNoStderrTail requires an InProcess agent's StderrTail to be
// empty.
func TestInProcessHasNoStderrTail(t *testing.T) {
	t.Parallel()
	p := startInProcess(t, &InProcess{Serve: func(context.Context, io.Reader, io.Writer) error { return errServe }})
	if _, err := within(t, testGrace+slack, "Wait", p.Wait); !errors.Is(err, errServe) {
		t.Errorf("Wait = %v, want Serve's error", err)
	}
	if got := p.StderrTail(); got != "" {
		t.Errorf("StderrTail = %q, want empty", got)
	}
}

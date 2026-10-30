package agentproc

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// testGrace keeps the suite fast; the default stays 5 seconds.
const testGrace = 300 * time.Millisecond

// slack bounds how long past its expected time a step may take on a busy
// runner.
const slack = 3 * time.Second

// descendantGrace is TestStderrHeldByDescendant's grace. Start sets WaitDelay
// to half of it, and the agent is seen to exit only once WaitDelay has passed
// after its exit, so its exit must be seen within the other half of Close's
// first step: slack, where testGrace would leave 150 ms, which a loaded runner
// under -race has exceeded (0027-PLAN-agent-process-helper.md, step 9).
const descendantGrace = 2 * slack

// readyWait bounds how long a child may take to start and set itself up. It
// bounds the test's setup, not the helper: a loaded machine has taken over
// 3 s to start the test binary.
const readyWait = 30 * time.Second

// testClient counts session updates. No test calls another client method.
type testClient struct {
	acp.Client
	updates atomic.Int64
}

func (c *testClient) SessionUpdate(context.Context, acp.SessionNotification) error {
	c.updates.Add(1)
	return nil
}

// agentCmd is the test binary run as a child in mode (child_test.go). Under
// -race a child would otherwise wait the race runtime's atexit_sleep_ms,
// 1000 ms by default, before it exits, longer than testGrace; os/exec keeps
// the last value of a repeated variable.
func agentCmd(mode string) *exec.Cmd {
	cmd := exec.Command(testBinary(), "-test.run=^$")
	cmd.Env = append(os.Environ(), childEnv+"="+mode, "GORACE=atexit_sleep_ms=0")
	return cmd
}

// readyWriter stands for a caller's Cmd.Stderr that is not a file, so
// os/exec copies to it on a goroutine; it closes ready when the child writes
// "ready" (child_test.go), and drops everything.
type readyWriter struct {
	once  sync.Once
	ready chan struct{}
}

func (w *readyWriter) Write(b []byte) (int, error) {
	if strings.Contains(string(b), "ready") {
		w.once.Do(func() { close(w.ready) })
	}
	return len(b), nil
}

// start starts the test binary as an agent in mode under testGrace, waits
// until the child says it is ready unless mode is "burst", and closes the
// agent when the test ends.
func start(t *testing.T, mode string, client acp.Client) *Process {
	t.Helper()
	return startCmd(t, agentCmd(mode), mode, client)
}

// startCmd is start for a command the test has adjusted.
func startCmd(t *testing.T, cmd *exec.Cmd, mode string, client acp.Client) *Process {
	t.Helper()
	w := &readyWriter{ready: make(chan struct{})}
	cmd.Stderr = w
	p, err := (&Command{Cmd: cmd, Grace: testGrace}).Start(client)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	if mode != "burst" {
		select {
		case <-w.ready:
		case <-time.After(readyWait):
			t.Fatalf("the %s agent did not say it was ready within %v", mode, readyWait)
		}
	}
	return p
}

// within returns how long f took and its result, failing the test if it
// takes longer than d.
func within(t *testing.T, d time.Duration, what string, f func() error) (time.Duration, error) {
	t.Helper()
	begin := time.Now()
	done := make(chan error, 1)
	go func() { done <- f() }()
	select {
	case err := <-done:
		return time.Since(begin), err
	case <-time.After(d):
		t.Fatalf("%s took longer than %v", what, d)
		return 0, nil
	}
}

// TestStartRefuses requires Start to refuse, starting nothing, a nil Cmd or
// client, a Cmd already started, and one whose Stdin or Stdout is set
// (0027-PLAN-agent-process-helper.md, "Start").
func TestStartRefuses(t *testing.T) {
	t.Parallel()
	started := agentCmd("eof")
	if err := started.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = started.Wait() })
	withStdin, withStdout := agentCmd("eof"), agentCmd("eof")
	withStdin.Stdin = strings.NewReader("")
	withStdout.Stdout = io.Discard
	for _, tc := range []struct {
		name   string
		cmd    *exec.Cmd
		client acp.Client
		want   string
	}{
		{"nil Cmd", nil, &testClient{}, "Command.Cmd is nil"},
		{"nil client", agentCmd("eof"), nil, "the client is nil"},
		{"already started", started, &testClient{}, "has already been started"},
		{"Stdin set", withStdin, &testClient{}, "Stdin is set"},
		{"Stdout set", withStdout, &testClient{}, "Stdout is set"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var before *os.Process
			if tc.cmd != nil {
				before = tc.cmd.Process
			}
			p, err := (&Command{Cmd: tc.cmd}).Start(tc.client)
			if err == nil || !strings.Contains(err.Error(), tc.want) || p != nil {
				t.Fatalf("Start = %v, %v; want an error containing %q", p, err, tc.want)
			}
			if tc.cmd != nil && tc.cmd.Process != before {
				t.Errorf("Start started the agent")
			}
		})
	}
}

// TestNoLostOutput requires every message the agent writes before it exits
// to reach the client: the agent writes burstCount session updates and exits
// at once (0027-MADR-agent-process-helper.md, P1, P2).
func TestNoLostOutput(t *testing.T) {
	t.Parallel()
	client := &testClient{}
	p := start(t, "burst", client)
	if _, err := p.Conn().Initialize(t.Context(), acp.InitializeRequest{}); err != nil {
		t.Fatalf("Initialize = %v", err)
	}
	// The agent writes its updates once this arrives, after initialize has
	// completed here: the client refuses a session update before then.
	if err := p.Conn().Connection().SendNotification(t.Context(), "_test/go", map[string]any{}); err != nil {
		t.Fatalf("SendNotification = %v", err)
	}
	_, err := within(t, slack, "Wait", p.Wait)
	if err != nil {
		t.Fatalf("Wait = %v, want nil", err)
	}
	if n := client.updates.Load(); n != burstCount {
		t.Errorf("the client received %d of the %d session updates the agent wrote", n, burstCount)
	}
}

// TestCloseLadder requires Close's steps: an agent that exits at its input's
// end is not signalled; one that exits on SIGTERM is terminated after one
// grace, where there is a process group; one that ignores both is killed,
// and each says so in Close's error (0027-MADR-agent-process-helper.md,
// "Close, in order").
func TestCloseLadder(t *testing.T) {
	t.Parallel()
	t.Run("exits at its input's end", func(t *testing.T) {
		t.Parallel()
		p := start(t, "eof", &testClient{})
		took, err := within(t, slack, "Close", p.Close)
		if err != nil || took >= testGrace {
			t.Errorf("Close = %v after %v, want nil within the grace, %v", err, took, testGrace)
		}
	})
	t.Run("exits on SIGTERM", func(t *testing.T) {
		t.Parallel()
		p := start(t, "term", &testClient{})
		took, err := within(t, testGrace*2+slack, "Close", p.Close)
		if !hasGroups {
			if !errors.Is(err, ErrKilled) || took < testGrace {
				t.Errorf("Close = %v after %v, want ErrKilled after the grace", err, took)
			}
			return
		}
		var ee *exec.ExitError
		if !errors.Is(err, ErrTerminated) || !errors.As(err, &ee) || ee.ExitCode() != 3 || took < testGrace || took >= 2*testGrace {
			t.Errorf("Close = %v after %v, want ErrTerminated with exit status 3 between one and two graces", err, took)
		}
	})
	t.Run("ignores both", func(t *testing.T) {
		t.Parallel()
		p := start(t, "stubborn", &testClient{})
		took, err := within(t, testGrace*2+slack, "Close", p.Close)
		floor := testGrace
		if hasGroups {
			floor = 2 * testGrace
		}
		if !errors.Is(err, ErrKilled) || errors.Is(err, ErrTerminated) || took < floor {
			t.Errorf("Close = %v after %v, want ErrKilled after %v", err, took, floor)
		}
	})
}

// TestStderrHeldByDescendant requires a Cmd.Stderr writer that a process the
// agent started still holds neither to keep Close waiting nor to make it
// escalate: WaitDelay, which Start sets to half the grace, closes it, so the
// agent's exit is seen within Close's first step. It runs under
// descendantGrace, which leaves slack for the agent's exit to be seen
// (0027-MADR-agent-process-helper.md, P10, P11;
// 0027-PLAN-agent-process-helper.md, the deviation during step 4, and step 9).
func TestStderrHeldByDescendant(t *testing.T) {
	t.Parallel()
	// Its Stderr is a readyWriter, not a file; it has no taps.
	p := startTapped(t, "parent-stderr", nil, descendantGrace, nil, nil)
	// The agent exits at its input's end; the heartbeat it started ignores
	// SIGTERM and holds the writer. Without WaitDelay, Wait would hold until
	// a kill ended the heartbeat, or for its minute where nothing can.
	_, err := within(t, descendantGrace*3+slack, "Close", p.Close)
	if !errors.Is(err, exec.ErrWaitDelay) || errors.Is(err, ErrTerminated) || errors.Is(err, ErrKilled) {
		t.Errorf("Close = %v, want exec.ErrWaitDelay, and no signal", err)
	}
}

// TestDefaultGrace requires the grace the docs name: 5 seconds when
// Command.Grace is zero or less, and a zero WaitDelay set to half of it,
// a WaitDelay the caller set kept (0027-MADR-agent-process-helper.md).
func TestDefaultGrace(t *testing.T) {
	t.Parallel()
	for _, grace := range []time.Duration{0, -time.Second} {
		cmd := agentCmd("eof")
		p, err := (&Command{Cmd: cmd, Grace: grace}).Start(&testClient{})
		if err != nil {
			t.Fatal(err)
		}
		if p.grace != 5*time.Second || cmd.WaitDelay != 2500*time.Millisecond {
			t.Errorf("Grace %v: the grace is %v and WaitDelay %v, want 5s and 2.5s", grace, p.grace, cmd.WaitDelay)
		}
		if err := p.Close(); err != nil {
			t.Errorf("Close = %v", err)
		}
	}
	cmd := agentCmd("eof")
	cmd.WaitDelay = time.Second
	p, err := (&Command{Cmd: cmd}).Start(&testClient{})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.WaitDelay != time.Second {
		t.Errorf("a WaitDelay the caller set became %v, want 1s kept", cmd.WaitDelay)
	}
	if err := p.Close(); err != nil {
		t.Errorf("Close = %v", err)
	}
}

// TestCloseIdempotent requires concurrent and repeated calls of Close to
// return the same error once Close's steps have run.
func TestCloseIdempotent(t *testing.T) {
	t.Parallel()
	p := start(t, "stubborn", &testClient{})
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Go(func() { errs[i] = p.Close() })
	}
	wg.Wait()
	for _, err := range slices.Concat(errs, []error{p.Close(), p.Wait()}) {
		if !errors.Is(err, ErrKilled) || err.Error() != errs[0].Error() {
			t.Errorf("Close and Wait returned %v and %v, want the same ErrKilled", errs[0], err)
		}
	}
}

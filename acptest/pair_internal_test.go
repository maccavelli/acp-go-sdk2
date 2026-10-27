package acptest

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// recordingTB is a testing.TB that keeps its cleanups, for the test to run,
// and its errors, for the test to read.
type recordingTB struct {
	testing.TB
	mu       sync.Mutex
	errs     []string
	cleanups []func()
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Cleanup(f func()) { r.cleanups = append(r.cleanups, f) }

func (r *recordingTB) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

// runCleanups runs the cleanups last first, as testing does, and returns how
// long they took.
func (r *recordingTB) runCleanups() time.Duration {
	begin := time.Now()
	for _, f := range slices.Backward(r.cleanups) {
		f()
	}
	return time.Since(begin)
}

// stuckAgent is a ScriptAgent whose NewSession ignores its context and runs
// until released.
type stuckAgent struct {
	ScriptAgent
	started, release chan struct{}
}

func (a *stuckAgent) NewSession(context.Context, acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	close(a.started)
	<-a.release
	return acp.NewSessionResponse{}, nil
}

// TestConnectCleanupBounded requires Connect's cleanup to stop waiting once
// its bound has passed, while a handler that ignores its context still runs,
// and to fail the test naming the side and the handler; and, with no such
// handler, to report nothing (0041-PLAN-close-the-gaps-the-consumers-wait-on.md,
// phase 6).
func TestConnectCleanupBounded(t *testing.T) {
	t.Parallel()
	const bound = 300 * time.Millisecond
	t.Run("a stuck handler", func(t *testing.T) {
		t.Parallel()
		tb := &recordingTB{TB: t}
		agent := &stuckAgent{started: make(chan struct{}), release: make(chan struct{})}
		pair := connectWithin(tb, bound, agent, &MemoryClient{}, nil)
		if _, err := pair.Client.Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = pair.Client.NewSession(context.Background(), acp.NewSessionRequest{Cwd: Abs("/work")})
		}()
		<-agent.started
		took := tb.runCleanups()
		close(agent.release)
		if took < bound || took > bound+5*time.Second {
			t.Errorf("the cleanup took %v, want the bound, %v, and not much more", took, bound)
		}
		got := strings.Join(tb.errs, "\n")
		if !strings.Contains(got, "the agent side's goroutines still run") || !strings.Contains(got, "stuckAgent).NewSession") {
			t.Errorf("the cleanup reported %q, want the agent side named, and the stuck handler among what runs", got)
		}
		if err := pair.Agent.Connection().WaitContext(t.Context()); err == nil {
			t.Error("the agent side's Wait = nil once released, want why it ended")
		}
		<-done
	})
	t.Run("no stuck handler", func(t *testing.T) {
		t.Parallel()
		tb := &recordingTB{TB: t}
		pair := connectWithin(tb, bound, &ScriptAgent{}, &MemoryClient{}, nil)
		if _, err := pair.Client.Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
			t.Fatal(err)
		}
		tb.runCleanups()
		if len(tb.errs) != 0 {
			t.Errorf("the cleanup reported %q, want nothing", tb.errs)
		}
	})
}

//go:build windows

package agentproc

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// stderrText is an agent's Stderr that keeps what the agent writes and
// closes ready when it writes "ready".
type stderrText struct {
	mu    sync.Mutex
	b     strings.Builder
	once  sync.Once
	ready chan struct{}
}

func (w *stderrText) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.b.Write(p)
	if strings.Contains(w.b.String(), "ready") {
		w.once.Do(func() { close(w.ready) })
	}
	return len(p), nil
}

func (w *stderrText) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

// TestBreakawayLeavesJob requires the agent's job to allow breakaway: a
// process the agent starts with CREATE_BREAKAWAY_FROM_JOB starts, is outside
// the job and still runs after Close, which ends the job once the agent exits
// at its input's end, and after Kill, which terminates it, while one it starts
// without it ends with the job (0030-MADR-record-initialize-on-arrival.md, the
// amendment of 2026-10-21, W3c, W3d and question 8).
func TestBreakawayLeavesJob(t *testing.T) {
	t.Parallel()
	t.Run("Close", func(t *testing.T) {
		t.Parallel()
		breakaway(t, (*Process).Close, func(err error) bool { return err == nil }, "nil")
	})
	t.Run("Kill", func(t *testing.T) {
		t.Parallel()
		breakaway(t, (*Process).Kill, func(err error) bool { return errors.Is(err, ErrKilled) }, "ErrKilled")
	})
}

// breakaway runs TestBreakawayLeavesJob's agent and stops it with stop, whose
// error must be as ok says.
func breakaway(t *testing.T, stop func(*Process) error, ok func(error) bool, want string) {
	t.Helper()
	beat := filepath.Join(t.TempDir(), "beat")
	cmd := agentCmd("parent-breakaway")
	cmd.Env = append(cmd.Env, beatEnv+"="+beat)
	w := &stderrText{ready: make(chan struct{})}
	cmd.Stderr = w
	p, err := (&Command{Cmd: cmd, Grace: testGrace}).Start(&testClient{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { endBeat(beat + ".away"); endBeat(beat) })
	t.Cleanup(func() { _ = p.Close() })
	select {
	case <-w.ready:
	case <-p.Done():
		t.Fatalf("the parent-breakaway agent exited before it was ready: %v; it wrote %q", p.Wait(), w.String())
	case <-time.After(readyWait):
		t.Fatalf("the parent-breakaway agent did not say it was ready within %v", readyWait)
	}
	if _, err := within(t, testGrace+slack, "stop", func() error { return stop(p) }); !ok(err) {
		t.Errorf("stop = %v, want %s", err, want)
	}
	if !beating(beat + ".away") {
		t.Error("the grandchild started with CREATE_BREAKAWAY_FROM_JOB does not run after the agent was stopped")
	}
	if beating(beat) {
		t.Error("the grandchild started without CREATE_BREAKAWAY_FROM_JOB still runs after the agent was stopped")
	}
}

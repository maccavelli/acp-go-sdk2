package agentproc

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// TestKill requires Kill to end, at once, an agent that ignores its input's
// end and whose grandchild holds its output, at the default grace of 5 s,
// with ErrKilled, and, where there is a tree, the grandchild with it
// (0030-MADR-record-initialize-on-arrival.md, the amendment of 2026-10-21,
// K2). The slack is shorter than the grace, so that Close's first wait would
// fail it.
func TestKill(t *testing.T) {
	t.Parallel()
	beat := filepath.Join(t.TempDir(), "beat")
	p := startTapped(t, "parent-wait", []string{beatEnv + "=" + beat}, 0, nil, nil)
	if !beating(beat) {
		t.Fatal("the parent-wait agent's beat grandchild is not writing")
	}
	_, err := within(t, slack, "Kill", p.Kill)
	if !errors.Is(err, ErrKilled) || errors.Is(err, ErrTerminated) {
		t.Errorf("Kill = %v, want ErrKilled alone", err)
	}
	if hasTree && beating(beat) {
		t.Error("the agent's grandchild still runs after Kill")
	}
}

// TestKillCutsCloseShort requires Kill, called while Close waits out a grace
// of 5 s for a stubborn agent, to make Close stop waiting and kill: both
// return within the slack, with the same error, ErrKilled joined.
func TestKillCutsCloseShort(t *testing.T) {
	t.Parallel()
	p := startTapped(t, "stubborn", nil, 5*time.Second, nil, nil)
	closed := make(chan error, 1)
	go func() { closed <- p.Close() }()
	time.Sleep(100 * time.Millisecond)
	_, err := within(t, slack, "Kill", p.Kill)
	select {
	case cerr := <-closed:
		if !errors.Is(err, ErrKilled) || errors.Is(err, ErrTerminated) || cerr == nil || cerr.Error() != err.Error() {
			t.Errorf("Close = %v and Kill = %v, want the same ErrKilled", cerr, err)
		}
	case <-time.After(slack):
		t.Fatalf("Close had not returned %v after Kill", slack)
	}
}

// TestKillAfterExit requires Kill, once the agent has exited of itself, to
// send nothing and to return Wait's error, without ErrKilled.
func TestKillAfterExit(t *testing.T) {
	t.Parallel()
	beat := filepath.Join(t.TempDir(), "beat")
	p := startTapped(t, "parent-exit", []string{beatEnv + "=" + beat}, testGrace, nil, nil)
	select {
	case <-p.Done():
	case <-time.After(readyWait):
		t.Fatalf("the parent-exit agent had not exited %v after it was ready", readyWait)
	}
	_, err := within(t, testGrace+slack, "Kill", p.Kill)
	if err != nil {
		t.Errorf("Kill = %v, want nil, the agent's exit of status 0", err)
	}
	if werr := p.Wait(); werr != nil {
		t.Errorf("Wait = %v after Kill, want nil", werr)
	}
}

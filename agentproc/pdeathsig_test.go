//go:build linux || freebsd

package agentproc

import (
	"errors"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
)

// TestStartFromExitingLockedGoroutine requires an agent with a Pdeathsig to
// live while its client does, even when the goroutine that called Start was
// locked to its thread and has returned, which ends that thread: Start
// starts the agent from a thread of its own
// (0029-MADR-unix-client-death-pdeathsig.md, D14).
func TestStartFromExitingLockedGoroutine(t *testing.T) {
	t.Parallel()
	w := &readyWriter{ready: make(chan struct{})}
	type result struct {
		p   *Process
		err error
	}
	got := make(chan result, 1)
	go func() {
		runtime.LockOSThread() // never unlocked: the thread ends when this goroutine returns
		cmd := agentCmd("stubborn")
		cmd.Stderr = w
		cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
		p, err := (&Command{Cmd: cmd, Grace: testGrace}).Start(&testClient{})
		got <- result{p, err}
	}()
	r := <-got
	if r.err != nil {
		t.Fatal(r.err)
	}
	var once sync.Once
	stop := func() (err error) { once.Do(func() { err = r.p.Close() }); return err }
	t.Cleanup(func() { _ = stop() })
	select {
	case <-w.ready:
	case <-r.p.Done():
		t.Fatalf("the agent ended, %v, when the thread that called Start did, before it said it was ready", r.p.Wait())
	case <-time.After(readyWait):
		t.Fatalf("the agent did not say it was ready within %v", readyWait)
	}
	select {
	case <-r.p.Done():
		t.Fatalf("the agent ended, %v, when the thread that called Start did", r.p.Wait())
	case <-time.After(time.Second):
	}
	if err := stop(); !errors.Is(err, ErrKilled) {
		t.Errorf("Close = %v, want ErrKilled: the stubborn agent ran until Close", err)
	}
}

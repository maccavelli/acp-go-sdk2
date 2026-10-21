//go:build linux || freebsd

package agentproc

import (
	"errors"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
)

// setInertDeathSignal sets cmd's Pdeathsig to SIGURG, which the Go runtime of
// a test child handles and survives, so that a test can tell the agent's
// input's end from its Pdeathsig at its client's death
// (0029-PLAN-unix-client-death-pdeathsig.md, the deviation during step 4).
func setInertDeathSignal(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGURG}
}

// TestDefaultPdeathsig requires Start to set SIGKILL as the agent's Pdeathsig
// when the caller set none (0029-MADR-unix-client-death-pdeathsig.md,
// question 1).
func TestDefaultPdeathsig(t *testing.T) {
	t.Parallel()
	cmd := agentCmd("eof")
	p := startCmd(t, cmd, "eof", &testClient{})
	if got := cmd.SysProcAttr.Pdeathsig; got != syscall.SIGKILL {
		t.Errorf("Pdeathsig = %v, want SIGKILL", got)
	}
	if err := p.Close(); err != nil {
		t.Errorf("Close = %v", err)
	}
}

// TestCallersPdeathsigKept requires Start to keep a Pdeathsig the caller set.
func TestCallersPdeathsigKept(t *testing.T) {
	t.Parallel()
	cmd := agentCmd("eof")
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
	p := startCmd(t, cmd, "eof", &testClient{})
	if got := cmd.SysProcAttr.Pdeathsig; got != syscall.SIGTERM {
		t.Errorf("Pdeathsig = %v, want the caller's SIGTERM", got)
	}
	if err := p.Close(); err != nil {
		t.Errorf("Close = %v", err)
	}
}

// TestClientDeathEndsAgent requires an agent that ignores its input's end
// and SIGTERM to end when its client is killed without Close: the "owner"
// child starts it through Start, and the test sends the owner SIGKILL
// (0029-MADR-unix-client-death-pdeathsig.md, D7).
func TestClientDeathEndsAgent(t *testing.T) {
	t.Parallel()
	requireGone(t, killOwner(t))
}

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

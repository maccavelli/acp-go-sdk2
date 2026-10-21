//go:build linux || freebsd

package agentproc

import (
	"bufio"
	"errors"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

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
	o := agentCmd("owner")
	stderr, err := o.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := o.Start(); err != nil {
		t.Fatal(err)
	}
	pid, ready := 0, false
	found := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			line := sc.Text()
			if n, ok := strings.CutPrefix(line, "pid "); ok {
				pid, _ = strconv.Atoi(n)
			}
			ready = ready || line == "ready"
			if pid > 0 && ready {
				close(found)
				break
			}
		}
		for sc.Scan() { // drain until the owner and the agent have closed it
		}
	}()
	select {
	case <-found:
	case <-time.After(readyWait):
		_ = o.Process.Kill()
		_ = o.Wait()
		t.Fatalf("the owner did not report a ready agent within %v", readyWait)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	if err := o.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = o.Wait()
	deadline := time.Now().Add(2 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			t.Fatalf("the agent, pid %d, still runs 2s after its client was killed", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
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

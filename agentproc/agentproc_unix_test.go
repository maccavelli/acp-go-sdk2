//go:build unix

package agentproc

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestClientDeathEndsAgentAtInputEnd requires an agent that exits at its
// input's end to end when its client is killed without Close: the client
// held the only write end of the agent's input. On Linux and FreeBSD the
// agent's Pdeathsig is one it survives, so that only its input's end can end
// it (0029-MADR-unix-client-death-pdeathsig.md, D1;
// 0029-PLAN-unix-client-death-pdeathsig.md, the deviation during step 4).
func TestClientDeathEndsAgentAtInputEnd(t *testing.T) {
	t.Parallel()
	requireGone(t, killOwner(t, ownedEnv+"=eof", inertDeathEnv+"=1"))
}

// killOwner starts the "owner" child, with env added to its environment,
// waits for it to report its agent's process ID and the agent ready, kills
// the owner with SIGKILL, and returns the agent's process ID. The agent is
// killed when the test ends.
func killOwner(t *testing.T, env ...string) int {
	t.Helper()
	o := agentCmd("owner")
	o.Env = append(o.Env, env...)
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
	return pid
}

// requireGone fails the test unless the agent, process pid, has ended
// within 2 seconds.
func requireGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			t.Fatalf("the agent, pid %d, still runs 2s after its client was killed", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestEscapedDescendantDoesNotHoldWait requires Wait to return when a
// process the agent started has left its process group and still holds the
// agent's output: the connection is closed after the grace
// (0027-MADR-agent-process-helper.md, "The agent's own end").
func TestEscapedDescendantDoesNotHoldWait(t *testing.T) {
	t.Parallel()
	p, beat := startBeating(t, "parent-escape")
	t.Cleanup(func() {
		if b, err := os.ReadFile(beat + ".pid"); err == nil {
			if pid, err := strconv.Atoi(string(b)); err == nil {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	if _, err := within(t, testGrace+slack, "Wait", p.Wait); err != nil {
		t.Errorf("Wait = %v, want nil: the agent exits of itself", err)
	}
	select {
	case <-p.Conn().Done():
	default:
		t.Error("the connection is open after Wait")
	}
	if !beating(beat) {
		t.Error("the escaped grandchild is not running: the test proves nothing")
	}
}

// TestTapKeepsCloseEnding requires Wait to return, an output tap set, when a
// process the agent started has left its process group and still holds the
// agent's output: the tap's reader closes the pipe when the connection closes
// it (0030-MADR-record-initialize-on-arrival.md, the amendment of 2026-10-21,
// T2 and T3).
func TestTapKeepsCloseEnding(t *testing.T) {
	t.Parallel()
	beat := filepath.Join(t.TempDir(), "beat")
	p := startTapped(t, "parent-escape", []string{beatEnv + "=" + beat}, testGrace, &capture{}, nil)
	t.Cleanup(func() {
		if b, err := os.ReadFile(beat + ".pid"); err == nil {
			if pid, err := strconv.Atoi(string(b)); err == nil {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	if !beating(beat) {
		t.Fatal("the escaped grandchild is not running: the test proves nothing")
	}
	if _, err := within(t, testGrace+slack, "Wait", p.Wait); err != nil {
		t.Errorf("Wait = %v, want nil: the agent exits of itself", err)
	}
}

// TestKillClosesAtOnce requires Kill to close the connection at once when a
// process the agent started has left its process group and holds the agent's
// output, at the default grace of 5 s: the wait for the output's end stops at
// Kill (0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phase 3). The
// slack is shorter than the grace, so that the wait would fail it.
func TestKillClosesAtOnce(t *testing.T) {
	t.Parallel()
	beat := filepath.Join(t.TempDir(), "beat")
	p := startTapped(t, "parent-escape", []string{beatEnv + "=" + beat}, 0, nil, nil)
	t.Cleanup(func() {
		if b, err := os.ReadFile(beat + ".pid"); err == nil {
			if pid, err := strconv.Atoi(string(b)); err == nil {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	if !beating(beat) {
		t.Fatal("the escaped grandchild is not running: the test proves nothing")
	}
	// The agent may exit of itself before Kill, so Kill's error is not checked.
	_, _ = within(t, slack, "Kill", p.Kill)
	select {
	case <-p.Conn().Done():
	default:
		t.Error("the connection is open after Kill")
	}
}

// TestStartRefusesSharedGroup requires Start to refuse, starting nothing, a
// Cmd that would join a process group other processes share
// (0027-PLAN-agent-process-helper.md, "Start").
func TestStartRefusesSharedGroup(t *testing.T) {
	t.Parallel()
	cmd := agentCmd("eof")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 1}
	p, err := (&Command{Cmd: cmd}).Start(&testClient{})
	if err == nil || !strings.Contains(err.Error(), "Pgid is set") || p != nil || cmd.Process != nil {
		t.Fatalf("Start = %v, %v, started %v; want a refusal naming Pgid", p, err, cmd.Process != nil)
	}
}

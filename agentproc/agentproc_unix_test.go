//go:build unix

package agentproc

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

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

//go:build unix

package agentproc

import (
	"strings"
	"syscall"
	"testing"
)

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

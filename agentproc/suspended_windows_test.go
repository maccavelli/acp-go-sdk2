//go:build windows

package agentproc

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestDescendantInJobDespiteSlowAttach requires a process the agent starts
// at once to be in the agent's job though the assignment comes 200 ms after
// the agent's start, as a slow client's would: the agent runs nothing until
// attach has assigned and resumed it, so Close, ending the job, ends the
// grandchild (0030-MADR-record-initialize-on-arrival.md, "Amendment
// 2026-10-21, Windows: a process the agent starts before its job
// assignment", J4). The "parent" agent stays alive, so that without the
// suspended start it is assigned, late, and its grandchild is not. It is not
// parallel: the delay is the package's, and no parallel test runs beside it.
func TestDescendantInJobDespiteSlowAttach(t *testing.T) {
	attachDelay.Store(int64(200 * time.Millisecond))
	t.Cleanup(func() { attachDelay.Store(0) })
	p, beat := startBeating(t, "parent")
	t.Cleanup(func() { endBeat(beat) })
	if _, err := within(t, testGrace+slack, "Close", p.Close); err != nil {
		t.Errorf("Close = %v, want nil", err)
	}
	if beating(beat) {
		t.Error("the grandchild the agent started at once, before a late assignment, still runs after Close")
	}
}

// endBeat ends the beat grandchild whose process ID startBeat recorded, should
// a failing test leave it running.
func endBeat(beat string) {
	b, err := os.ReadFile(beat + ".gpid")
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return
	}
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return
	}
	_ = windows.TerminateProcess(h, 1)
	_ = windows.CloseHandle(h)
}

// TestStartRefusesSuspended requires Start to refuse, starting nothing, a Cmd
// the caller would start suspended itself: Start resumes the agent
// (question 12 of the amendment above).
func TestStartRefusesSuspended(t *testing.T) {
	t.Parallel()
	cmd := agentCmd("eof")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	p, err := (&Command{Cmd: cmd}).Start(&testClient{})
	if err == nil || !strings.Contains(err.Error(), "CREATE_SUSPENDED") || p != nil || cmd.Process != nil {
		t.Fatalf("Start = %v, %v, started %v; want a refusal naming CREATE_SUSPENDED", p, err, cmd.Process != nil)
	}
}

// TestCallersCreationFlagsKept requires the caller's creation flags to be
// kept beside CREATE_SUSPENDED, and the agent, resumed, to run and to stop.
func TestCallersCreationFlagsKept(t *testing.T) {
	t.Parallel()
	cmd := agentCmd("eof")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	p := startCmd(t, cmd, "eof", &testClient{})
	if f := cmd.SysProcAttr.CreationFlags; f&windows.CREATE_NEW_PROCESS_GROUP == 0 || f&windows.CREATE_SUSPENDED == 0 {
		t.Errorf("CreationFlags = %#x, want the caller's CREATE_NEW_PROCESS_GROUP and CREATE_SUSPENDED", f)
	}
	if err := p.Close(); err != nil {
		t.Errorf("Close = %v, want nil: the agent runs, and exits at its input's end", err)
	}
}

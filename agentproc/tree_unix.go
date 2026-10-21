//go:build unix

package agentproc

import (
	"errors"
	"os/exec"
	"syscall"
)

// prepare puts the agent in a process group of its own, whose ID is its
// process ID, so that a signal to the group reaches the processes it starts
// (0027-MADR-agent-process-helper.md, P14). Setsid does as much.
func prepare(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if cmd.SysProcAttr.Pgid != 0 {
		return errors.New("agentproc: Command.Cmd.SysProcAttr.Pgid is set; the agent needs a process group of its own")
	}
	if !cmd.SysProcAttr.Setsid {
		cmd.SysProcAttr.Setpgid = true
	}
	return nil
}

// hasGroups reports that the agent runs in a process group Close can signal.
const hasGroups = true

// terminate sends SIGTERM to the agent's process group.
func terminate(cmd *exec.Cmd) error { return signalGroup(cmd, syscall.SIGTERM) }

// kill sends SIGKILL to the agent's process group, or kills the agent alone
// when the group cannot be signalled.
func kill(cmd *exec.Cmd) error {
	if err := signalGroup(cmd, syscall.SIGKILL); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}

// signalGroup sends sig to the process group whose ID is the agent's process
// ID. A group that has emptied is not an error.
func signalGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if err := syscall.Kill(-cmd.Process.Pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

//go:build unix

package agentproc

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// hasGroups reports that the agent runs in a process group Close can signal.
const hasGroups = true

// hasTree reports that stopping the agent ends the processes it started.
const hasTree = true

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
	setDeathSignal(cmd.SysProcAttr)
	return nil
}

// tree is the agent's process group, whose ID is the agent's process ID.
type tree struct{ cmd *exec.Cmd }

// attach has nothing to set up once the agent has started: prepare made the
// group.
func attach(cmd *exec.Cmd) (*tree, error) { return &tree{cmd: cmd}, nil }

// terminate sends SIGTERM to the agent's process group.
func (t *tree) terminate() error { return signalGroup(t.cmd, syscall.SIGTERM) }

// kill sends SIGKILL to the agent's process group, and kills the agent
// itself too, as the Rust SDK does, so the agent ends even where the group
// cannot be signalled.
func (t *tree) kill() error {
	gerr := signalGroup(t.cmd, syscall.SIGKILL)
	if err := t.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return errors.Join(gerr, err)
	}
	return nil
}

// end kills what is left of the agent's process group once the agent has
// exited. The group's ID is not reused while the group has members.
func (t *tree) end() { _ = signalGroup(t.cmd, syscall.SIGKILL) } //nolint:errcheck // an emptied group is the common case.

// signalGroup sends sig to the process group whose ID is the agent's process
// ID. A group that has emptied is not an error.
func signalGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if err := syscall.Kill(-cmd.Process.Pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

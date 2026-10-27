//go:build unix

package proctree

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// HasGroups reports that the process runs in a process group that can be
// signalled.
const HasGroups = true

// HasTree reports that ending the process ends the processes it started.
const HasTree = true

// Prepare puts the process in a process group of its own, whose ID is its
// process ID, so that a signal to the group reaches the processes it starts
// (0027-MADR-agent-process-helper.md, P14). Setsid does as much. It refuses,
// with ErrPgidSet, a Cmd whose Pgid is set.
func Prepare(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if cmd.SysProcAttr.Pgid != 0 {
		return ErrPgidSet
	}
	if !cmd.SysProcAttr.Setsid {
		cmd.SysProcAttr.Setpgid = true
	}
	return nil
}

// SetDeathSignal has the kernel end the process should this one die, where
// there is a Pdeathsig, unless the caller chose one
// (0029-MADR-unix-client-death-pdeathsig.md). Linux sends it when the thread
// that started the process ends, so the caller starts the process from a
// thread locked for the process's life, as agentproc does.
func SetDeathSignal(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	defaultDeathSignal(cmd.SysProcAttr)
}

// Tree is the process's process group, whose ID is the process's ID.
type Tree struct{ cmd *exec.Cmd }

// Attach has nothing to set up once the process has started: Prepare made
// the group.
func Attach(cmd *exec.Cmd) (*Tree, error) { return &Tree{cmd: cmd}, nil }

// Terminate sends SIGTERM to the process group.
func (t *Tree) Terminate() error { return signalGroup(t.cmd, syscall.SIGTERM) }

// Kill sends SIGKILL to the process group, and kills the process itself too,
// as the Rust SDK does, so the process ends even where the group cannot be
// signalled.
func (t *Tree) Kill() error {
	gerr := signalGroup(t.cmd, syscall.SIGKILL)
	if err := t.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return errors.Join(gerr, err)
	}
	return nil
}

// End kills what is left of the process group once the process has exited.
// The group's ID is not reused while the group has members.
func (t *Tree) End() { _ = signalGroup(t.cmd, syscall.SIGKILL) } //nolint:errcheck // an emptied group is the common case.

// signalGroup sends sig to the process group whose ID is the process's ID. A
// group that has emptied is not an error.
func signalGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if err := syscall.Kill(-cmd.Process.Pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

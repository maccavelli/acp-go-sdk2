//go:build !unix && !windows

package proctree

import (
	"errors"
	"os/exec"
)

// HasGroups reports that there is no process group here.
const HasGroups = false

// HasTree reports that there is no tree of processes to end here.
const HasTree = false

// Prepare has no group to set up here.
func Prepare(*exec.Cmd) error { return nil }

// SetDeathSignal has nothing to set here: there is no Pdeathsig.
func SetDeathSignal(*exec.Cmd) {}

// Tree is the process alone: there is no group or job here.
type Tree struct{ cmd *exec.Cmd }

// Attach has nothing to set up.
func Attach(cmd *exec.Cmd) (*Tree, error) { return &Tree{cmd: cmd}, nil }

// Terminate has no signal to send here, so a caller goes on to Kill.
func (*Tree) Terminate() error { return errors.ErrUnsupported }

// Kill kills the process alone.
func (t *Tree) Kill() error { return t.cmd.Process.Kill() }

// End has no tree to end here.
func (*Tree) End() {}

//go:build !unix && !windows

package agentproc

import (
	"errors"
	"os/exec"
)

// hasGroups reports that there is no process group here.
const hasGroups = false

// hasTree reports that there is no tree of processes to end here.
const hasTree = false

// prepare has no group to set up here.
func prepare(*exec.Cmd) error { return nil }

// tree is the agent alone: there is no group or job here.
type tree struct{ cmd *exec.Cmd }

// attach has nothing to set up.
func attach(cmd *exec.Cmd) (*tree, error) { return &tree{cmd: cmd}, nil }

// terminate has no signal to send here, so Close goes from the agent's
// input to the kill.
func (*tree) terminate() error { return errors.ErrUnsupported }

// kill kills the agent alone.
func (t *tree) kill() error { return t.cmd.Process.Kill() }

// end has no tree to end here.
func (*tree) end() {}

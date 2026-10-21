//go:build !unix

package agentproc

import (
	"errors"
	"os/exec"
)

// prepare has no group to set up here.
func prepare(*exec.Cmd) error { return nil }

// hasGroups reports that there is no process group here.
const hasGroups = false

// hasTree reports that there is no tree of processes to end here.
const hasTree = false

// endTree has no tree to end here.
func endTree(*exec.Cmd) {}

// terminate has no signal to send here: Windows does not implement sending
// an interrupt (0027-MADR-agent-process-helper.md, S3), so Close goes from
// the agent's input to the kill.
func terminate(*exec.Cmd) error { return errors.ErrUnsupported }

// kill kills the agent alone.
func kill(cmd *exec.Cmd) error { return cmd.Process.Kill() }

package agentproc

import (
	"errors"
	"os/exec"

	"github.com/maccavelli/acp-go-sdk2/internal/proctree"
)

// prepare readies the agent's Cmd for its process group or job, which
// internal/proctree holds, shared with clienthost
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, question 1), and gives
// the agent its death signal, where there is one
// (0029-MADR-unix-client-death-pdeathsig.md). Its refusals name the agent's
// Command.
func prepare(cmd *exec.Cmd) error {
	switch err := proctree.Prepare(cmd); {
	case errors.Is(err, proctree.ErrPgidSet):
		return errors.New("agentproc: Command.Cmd.SysProcAttr.Pgid is set; the agent needs a process group of its own")
	case errors.Is(err, proctree.ErrSuspended):
		return errors.New("agentproc: Command.Cmd.SysProcAttr.CreationFlags has CREATE_SUSPENDED; Start starts the agent suspended itself")
	case err != nil:
		return err
	}
	proctree.SetDeathSignal(cmd)
	return nil
}

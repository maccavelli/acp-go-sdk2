//go:build linux || freebsd

package agentproc

import "syscall"

// setDeathSignal has the kernel SIGKILL the agent should its client die
// without Close, unless the caller chose a Pdeathsig of its own: an agent
// that ignores its input's end would otherwise outlive the client. Run
// starts the agent from a thread that lives as long as the agent, which
// Linux's Pdeathsig follows (0029-MADR-unix-client-death-pdeathsig.md).
func setDeathSignal(a *syscall.SysProcAttr) {
	if a.Pdeathsig == 0 {
		a.Pdeathsig = syscall.SIGKILL
	}
}

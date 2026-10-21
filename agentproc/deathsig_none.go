//go:build unix && !linux && !freebsd

package agentproc

import "syscall"

// setDeathSignal has nothing to set here: no Pdeathsig exists, and an agent
// ends with its client by its input's end alone
// (0029-MADR-unix-client-death-pdeathsig.md).
func setDeathSignal(*syscall.SysProcAttr) {}

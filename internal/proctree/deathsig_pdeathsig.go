//go:build linux || freebsd

package proctree

import "syscall"

// defaultDeathSignal has the kernel SIGKILL the process should this one die
// without ending it, unless the caller chose a Pdeathsig of its own: a
// process that ignores its input's end would otherwise outlive this one
// (0029-MADR-unix-client-death-pdeathsig.md).
func defaultDeathSignal(a *syscall.SysProcAttr) {
	if a.Pdeathsig == 0 {
		a.Pdeathsig = syscall.SIGKILL
	}
}

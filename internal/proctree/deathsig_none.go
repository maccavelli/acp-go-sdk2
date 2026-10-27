//go:build unix && !linux && !freebsd

package proctree

import "syscall"

// defaultDeathSignal has nothing to set here: no Pdeathsig exists, and a
// process ends with this one only by its own means, as an agent does at its
// input's end (0029-MADR-unix-client-death-pdeathsig.md).
func defaultDeathSignal(*syscall.SysProcAttr) {}

//go:build unix

package clienthost

import (
	"os"
	"syscall"

	acp "github.com/maccavelli/acp-go-sdk2"
	"golang.org/x/sys/unix"
)

// exitStatus is how the command ended: the signal that ended it, by its
// name, such as "SIGKILL", or else its exit code.
func exitStatus(ps *os.ProcessState) acp.TerminalExitStatus {
	var s acp.TerminalExitStatus
	if ps == nil {
		return s
	}
	if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		name := unix.SignalName(ws.Signal())
		if name == "" {
			name = ws.Signal().String()
		}
		s.Signal = &name
		return s
	}
	if code := int64(ps.ExitCode()); code >= 0 && code <= int64(^uint32(0)) {
		c := uint32(code)
		s.ExitCode = &c
	}
	return s
}

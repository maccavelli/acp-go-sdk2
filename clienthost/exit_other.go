//go:build !unix

package clienthost

import (
	"os"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// exitStatus is the command's exit code: there are no signals here, and a
// killed command on Windows exits with status 1, which its job's end gives.
func exitStatus(ps *os.ProcessState) acp.TerminalExitStatus {
	var s acp.TerminalExitStatus
	if ps == nil {
		return s
	}
	if code := int64(ps.ExitCode()); code >= 0 && code <= int64(^uint32(0)) {
		c := uint32(code)
		s.ExitCode = &c
	}
	return s
}

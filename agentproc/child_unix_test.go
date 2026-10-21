//go:build unix

package agentproc

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// platformChild runs the modes only Unix has.
func platformChild(mode string) (int, bool) {
	if mode != "parent-escape" {
		return 0, false
	}
	// Starts a beat in a session of its own, outside the agent's process
	// group, which still holds the agent's output; records its process ID;
	// then exits of itself.
	g, err := startBeat(func(g *exec.Cmd) { g.SysProcAttr = &syscall.SysProcAttr{Setsid: true} })
	if err != nil {
		return 1, true
	}
	if err := os.WriteFile(os.Getenv(beatEnv)+".pid", []byte(strconv.Itoa(g.Process.Pid)), 0o600); err != nil {
		return 1, true
	}
	ready()
	return 0, true
}

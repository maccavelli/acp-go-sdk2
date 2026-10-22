//go:build windows

package agentproc

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// platformChild runs the modes only Windows has.
func platformChild(mode string) (int, bool) {
	switch mode {
	case "owner":
		// Starts a parent-wait agent through the helper, whose beat grandchild
		// shares this child's beat file, and exits once the beat has written,
		// without Close: the job's last handle closes as this process ends.
		p, err := (&Command{Cmd: agentCmd("parent-wait")}).Start(&testClient{})
		if err != nil {
			return 1, true
		}
		for range 400 {
			if fi, err := os.Stat(os.Getenv(beatEnv)); err == nil && fi.Size() > 0 {
				ready()
				os.Exit(0)
			}
			time.Sleep(25 * time.Millisecond)
		}
		_ = p.Close()
		return 1, true
	case "parent-breakaway":
		// Starts a beat with CREATE_BREAKAWAY_FROM_JOB, on the file beatEnv
		// names with ".away" added and not on this child's output, as a
		// detached updater runs, and a beat without it; then exits at its
		// input's end.
		away := func(g *exec.Cmd) {
			g.Stdout = nil
			g.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_BREAKAWAY_FROM_JOB}
		}
		if _, err := startBeatAt(os.Getenv(beatEnv)+".away", away); err != nil {
			fmt.Fprintf(os.Stderr, "starting the beat that breaks away: %v\n", err)
			return 1, true
		}
		if _, err := startBeat(nil); err != nil {
			return 1, true
		}
		ready()
		_, _ = io.Copy(io.Discard, os.Stdin)
		return 0, true
	}
	return 0, false
}

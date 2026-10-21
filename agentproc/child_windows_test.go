//go:build windows

package agentproc

import (
	"os"
	"time"
)

// platformChild runs the modes only Windows has.
func platformChild(mode string) (int, bool) {
	if mode != "owner" {
		return 0, false
	}
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
}

//go:build windows

package agentproc

import (
	"path/filepath"
	"testing"
	"time"
)

// TestClientDeathEndsTree requires the agent's processes to end when the
// process that started them dies without Close: the job object ends them as
// its last handle closes (0027-MADR-agent-process-helper.md, P18).
func TestClientDeathEndsTree(t *testing.T) {
	t.Parallel()
	beat := filepath.Join(t.TempDir(), "beat")
	owner := agentCmd("owner")
	owner.Env = append(owner.Env, beatEnv+"="+beat)
	w := &readyWriter{ready: make(chan struct{})}
	owner.Stderr = w
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Wait(); err != nil {
		t.Fatalf("the owner exited %v, want 0 once its agent's grandchild wrote", err)
	}
	select {
	case <-w.ready:
	case <-time.After(readyWait):
		t.Fatal("the owner never said its agent's grandchild wrote")
	}
	if beating(beat) {
		t.Error("the agent's grandchild still runs after the process that started it died")
	}
}

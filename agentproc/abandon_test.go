package agentproc

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/maccavelli/acp-go-sdk2/internal/proctree"
)

// TestStartAbandonsUnattachedAgent requires Start, when it cannot attach the
// agent it started to a process tree, to kill and reap it, close the
// parent's pipe ends, and return the reason, with no Process
// (0049-PLAN-tooling-fixes.md, phase 5, F-C: abandon was never run by a
// test). The agent is a "beat" child, which appends to its file every 50 ms
// while it lives, so a file that stops growing is an agent that has ended.
//
// It does not run in parallel: it replaces attachTree, which every Start
// calls.
func TestStartAbandonsUnattachedAgent(t *testing.T) {
	refused := errors.New("attach refused by the test")
	old := attachTree
	t.Cleanup(func() { attachTree = old })
	attachTree = func(*exec.Cmd) (*proctree.Tree, error) { return nil, refused }

	beat := filepath.Join(t.TempDir(), "beat")
	cmd := agentCmd("beat")
	cmd.Env = append(cmd.Env, beatEnv+"="+beat)
	p, err := (&Command{Cmd: cmd, Grace: testGrace}).Start(&testClient{})
	if p != nil || !errors.Is(err, refused) {
		t.Fatalf("Start = %v, %v; want no Process and an error wrapping the attach's", p, err)
	}
	size := func() int64 {
		fi, err := os.Stat(beat)
		if err != nil {
			return -1
		}
		return fi.Size()
	}
	before := size()
	time.Sleep(300 * time.Millisecond)
	if after := size(); after != before {
		t.Errorf("the agent still runs after Start abandoned it: its file grew from %d to %d bytes in 300 ms", before, after)
	}
}

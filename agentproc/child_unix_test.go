//go:build unix

package agentproc

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// platformChild runs the modes only Unix has.
func platformChild(mode string) (int, bool) {
	if mode == "owner" {
		return owner(), true
	}
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

// ownedEnv names the mode of the agent the "owner" child starts, "stubborn"
// when it is unset. inertDeathEnv, when set, has the owner give the agent a
// Pdeathsig it survives, where there is one
// (0029-PLAN-unix-client-death-pdeathsig.md, the deviation during step 4).
const (
	ownedEnv      = "AGENTPROC_TEST_OWNED"
	inertDeathEnv = "AGENTPROC_TEST_INERT_DEATHSIG"
)

// owner is a client: it starts an agent through Start, its Cmd otherwise as
// the caller left it, the agent's stderr its own, writes "pid <n>" to stderr,
// and waits to be killed (0029-PLAN-unix-client-death-pdeathsig.md, step 3).
func owner() int {
	cmd := exec.Command(testBinary(), "-test.run=^$")
	cmd.Env = append(os.Environ(), childEnv+"="+cmp.Or(os.Getenv(ownedEnv), "stubborn"), "GORACE=atexit_sleep_ms=0")
	cmd.Stderr = os.Stderr
	if os.Getenv(inertDeathEnv) != "" {
		setInertDeathSignal(cmd)
	}
	if _, err := (&Command{Cmd: cmd}).Start(&testClient{}); err != nil {
		return 1
	}
	fmt.Fprintf(os.Stderr, "pid %d\n", cmd.Process.Pid)
	time.Sleep(time.Hour)
	return 0
}

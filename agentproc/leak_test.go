package agentproc

import (
	"bytes"
	"fmt"
	"os"
	"runtime/pprof"
	"testing"
	"time"
)

// childEnv names the child mode the test binary runs instead of its tests
// (child_test.go).
const childEnv = "AGENTPROC_TEST_CHILD"

// TestMain runs a child mode when childEnv names one; otherwise it runs the
// package's tests and then fails the run if they left a goroutine leaked, as
// the root package's guard does
// (0023-MADR-connection-lifetime-and-test-leaks.md, T2).
func TestMain(m *testing.M) {
	if mode := os.Getenv(childEnv); mode != "" {
		os.Exit(child(mode))
	}
	code := m.Run()
	// Go 1.26 has the profile only as an experiment, which Go 1.27 rejects
	// (0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md, question 2).
	if pprof.Lookup("goroutineleak") == nil {
		fmt.Fprint(os.Stderr, "the goroutineleak profile is not available: on Go 1.26, set GOEXPERIMENT=goroutineleakprofile\n")
		os.Exit(1)
	}
	if leaked := waitForLeaks(5 * time.Second); leaked != "" {
		fmt.Fprintf(os.Stderr, "goroutines leaked by the tests (0023-MADR-connection-lifetime-and-test-leaks.md); "+
			"run tests alone to find the one that leaked them:\n%s", leaked)
		code = 1
	}
	os.Exit(code)
}

// settleWindow is how long the goroutineleak profile must count none before
// the guard reports clean, as the root package's guard waits.
const settleWindow = 500 * time.Millisecond

// waitForLeaks polls the goroutineleak profile, and returns "" once every
// read for settleWindow has counted none; otherwise, after bound, the
// profile.
func waitForLeaks(bound time.Duration) string {
	p := pprof.Lookup("goroutineleak")
	start := time.Now()
	var clean time.Time
	for {
		var buf bytes.Buffer
		if err := p.WriteTo(&buf, 1); err != nil {
			return fmt.Sprintf("reading the goroutineleak profile: %v\n", err)
		}
		now := time.Now()
		switch {
		case p.Count() != 0:
			clean = time.Time{}
			if now.Sub(start) >= bound {
				return buf.String()
			}
		case clean.IsZero():
			clean = now
		case now.Sub(clean) >= settleWindow:
			return ""
		}
		time.Sleep(50 * time.Millisecond)
	}
}

package acp

import (
	"bytes"
	"fmt"
	"os"
	"runtime/pprof"
	"testing"
	"time"
)

// TestMain runs the package's tests and then fails the run if they left a
// goroutine leaked, such as a connection's that no test closed
// (0023-MADR-connection-lifetime-and-test-leaks.md, T2).
func TestMain(m *testing.M) {
	code := m.Run()
	if leaked := waitForLeaks(notificationQueueDrainTimeout + 2*time.Second); leaked != "" {
		fmt.Fprintf(os.Stderr, "goroutines leaked by the tests (0023-MADR-connection-lifetime-and-test-leaks.md); "+
			"run tests alone to find the one that leaked them:\n%s", leaked)
		code = 1
	}
	os.Exit(code)
}

// settleWindow is how long the goroutineleak profile must count none before
// the guard reports clean. The profile counts a goroutine only once it has
// blocked, and a goroutine the last test leaked may not have blocked yet when
// the tests return: measured, it was counted within 11 ms
// (0023-PLAN-connection-lifetime-and-test-leaks.md, step 6).
const settleWindow = 500 * time.Millisecond

// waitForLeaks polls the goroutineleak profile, and returns "" once every
// read for settleWindow has counted none. While it counts some, it polls
// until bound has passed, and then returns the profile, its identical stacks
// grouped with their counts.
func waitForLeaks(bound time.Duration) string {
	p := pprof.Lookup("goroutineleak")
	start := time.Now()
	var clean time.Time // when the reads began to count none; zero while they count some
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

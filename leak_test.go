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

// waitForLeaks polls the goroutineleak profile until it counts none, and
// returns "". A connection whose input has ended may still be draining, for
// at most notificationQueueDrainTimeout, so the profile is read until bound
// has passed; then it returns the profile, its identical stacks grouped with
// their counts.
func waitForLeaks(bound time.Duration) string {
	p := pprof.Lookup("goroutineleak")
	deadline := time.Now().Add(bound)
	for {
		var buf bytes.Buffer
		if err := p.WriteTo(&buf, 1); err != nil {
			return fmt.Sprintf("reading the goroutineleak profile: %v\n", err)
		}
		if p.Count() == 0 {
			return ""
		}
		if time.Now().After(deadline) {
			return buf.String()
		}
		time.Sleep(50 * time.Millisecond)
	}
}

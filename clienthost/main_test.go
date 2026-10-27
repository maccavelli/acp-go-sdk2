package clienthost

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime/pprof"
	"strconv"
	"strings"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// childEnv names the mode the test binary runs instead of its tests.
const childEnv = "CLIENTHOST_TEST_CHILD"

// Environment of the child modes.
const (
	outEnv   = "CLIENTHOST_TEST_OUT"   // what "write" writes to standard output
	errEnv   = "CLIENTHOST_TEST_ERR"   // what "write" writes to standard error
	exitEnv  = "CLIENTHOST_TEST_EXIT"  // the status "write" exits with
	charEnv  = "CLIENTHOST_TEST_CHAR"  // the character "repeat" writes
	countEnv = "CLIENTHOST_TEST_COUNT" // how many times "repeat" writes it
	beatEnv  = "CLIENTHOST_TEST_BEAT"  // the file "parent"'s grandchild appends to
)

// TestMain runs a child mode when childEnv names one; otherwise it runs the
// package's tests and then fails the run if they left a goroutine leaked, as
// the root package's guard does
// (0023-MADR-connection-lifetime-and-test-leaks.md, T2).
func TestMain(m *testing.M) {
	if mode := os.Getenv(childEnv); mode != "" {
		os.Exit(child(mode))
	}
	code := m.Run()
	if pprof.Lookup("goroutineleak") == nil {
		fmt.Fprint(os.Stderr, "the goroutineleak profile is not available: on Go 1.26, set GOEXPERIMENT=goroutineleakprofile\n")
		os.Exit(1)
	}
	if leaked := waitForLeaks(5 * time.Second); leaked != "" {
		fmt.Fprintf(os.Stderr, "goroutines leaked by the tests (0023-MADR-connection-lifetime-and-test-leaks.md):\n%s", leaked)
		code = 1
	}
	os.Exit(code)
}

// child runs one behaviour of a terminal's command and returns its exit
// status.
func child(mode string) int {
	switch mode {
	case "write": // writes outEnv and errEnv, and exits with exitEnv
		_, _ = os.Stdout.WriteString(os.Getenv(outEnv))
		_, _ = os.Stderr.WriteString(os.Getenv(errEnv))
		code, _ := strconv.Atoi(os.Getenv(exitEnv))
		return code
	case "repeat": // writes charEnv countEnv times, in writes of 7 bytes, which split the character
		n, _ := strconv.Atoi(os.Getenv(countEnv))
		b := []byte(strings.Repeat(os.Getenv(charEnv), n))
		for len(b) > 0 {
			k := min(7, len(b))
			if _, err := os.Stdout.Write(b[:k]); err != nil {
				return 1
			}
			b = b[k:]
		}
		return 0
	case "sleep": // writes "ready", and sleeps
		_, _ = os.Stdout.WriteString("ready\n")
		time.Sleep(time.Hour)
		return 0
	case "parent": // starts a beat grandchild on its output, writes "ready" once the beat has begun, and sleeps
		g := exec.Command(testBinary(), "-test.run=^$")
		g.Env = append(os.Environ(), childEnv+"=beat")
		g.Stdout = os.Stdout
		if err := g.Start(); err != nil {
			return 1
		}
		for range 400 {
			if fi, err := os.Stat(os.Getenv(beatEnv)); err == nil && fi.Size() > 0 {
				_, _ = os.Stdout.WriteString("ready\n")
				time.Sleep(time.Hour)
				return 0
			}
			time.Sleep(25 * time.Millisecond)
		}
		return 1
	case "beat": // appends a byte to beatEnv every 50 ms for a minute
		f, err := os.OpenFile(os.Getenv(beatEnv), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return 1
		}
		for range 1200 {
			if _, err := f.Write([]byte{'.'}); err != nil {
				return 1
			}
			time.Sleep(50 * time.Millisecond)
		}
		return 0
	}
	fmt.Fprintf(os.Stderr, "unknown child mode %q\n", mode)
	return 2
}

// testBinary is the test binary's absolute path.
func testBinary() string {
	exe, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return exe
}

// testCommand is a Terminals.Command that runs the test binary in the
// request's command as its mode, with the request's env added; it is
// Command's test too.
func testCommand(_ context.Context, req acp.CreateTerminalRequest) (*exec.Cmd, error) {
	cmd := exec.Command(testBinary(), "-test.run=^$")
	cmd.Env = append(os.Environ(), childEnv+"="+req.Command, "GORACE=atexit_sleep_ms=0")
	for _, e := range req.Env {
		cmd.Env = append(cmd.Env, e.Name+"="+e.Value)
	}
	return cmd, nil
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

package agentproc

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"runtime/pprof"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestProcessWaitContext requires Process.WaitContext to return
// context.Cause(ctx) when ctx ends before the agent has been waited for,
// leaving the agent running; Wait's error once it has; and its calls,
// however many, to share one goroutine, which is gone once Wait has returned
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 6).
func TestProcessWaitContext(t *testing.T) {
	t.Parallel()
	t.Run("ctx ends first", func(t *testing.T) {
		t.Parallel()
		p := start(t, "eof", &testClient{})
		why := errors.New("the test's bound")
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(why)
		if err := p.WaitContext(ctx); !errors.Is(err, why) {
			t.Errorf("WaitContext with an ended ctx = %v, want its cause, %v", err, why)
		}
		select {
		case <-p.Done():
			t.Error("the agent exited after WaitContext: it is left running")
		default:
		}
		if err := p.Close(); err != nil {
			t.Fatalf("Close = %v", err)
		}
		if err := p.WaitContext(context.Background()); err != nil {
			t.Errorf("WaitContext after Close = %v, want Close's, nil", err)
		}
	})
	t.Run("the agent ends first", func(t *testing.T) {
		t.Parallel()
		cmd := agentCmd("parent-exit")
		cmd.Env = append(cmd.Env, beatEnv+"="+filepath.Join(t.TempDir(), "beat"))
		p := startCmd(t, cmd, "parent-exit", &testClient{})
		if err := p.WaitContext(t.Context()); err != nil {
			t.Errorf("WaitContext = %v, want Wait's, nil: the agent exits 0 of itself", err)
		}
		if !p.ExitedFirst() {
			t.Error("ExitedFirst after WaitContext = false, want true: WaitContext returns once Wait has")
		}
	})
	t.Run("one goroutine", func(t *testing.T) {
		t.Parallel()
		p := start(t, "eof", &testClient{})
		const label = "TestProcessWaitContext/one goroutine"
		pprof.Do(context.Background(), pprof.Labels("test", label), func(context.Context) {
			ended, cancel := context.WithCancel(context.Background())
			cancel()
			for range 100 {
				_ = p.WaitContext(ended)
			}
		})
		if n := processWaiters(label); n != 1 {
			t.Errorf("100 calls of WaitContext left %d waiting goroutines, want 1", n)
		}
		if err := p.Close(); err != nil {
			t.Fatalf("Close = %v", err)
		}
		deadline := time.Now().Add(slack)
		for processWaiters(label) != 0 {
			if time.Now().After(deadline) {
				t.Fatalf("WaitContext's goroutine still runs %v after Close returned", slack)
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
}

// processWaiters counts the goroutines in WaitContext's waiter that carry the
// label "test" with value, which a goroutine inherits from the one that
// starts it.
func processWaiters(value string) int {
	var b bytes.Buffer
	_ = pprof.Lookup("goroutine").WriteTo(&b, 1)
	n := 0
	for group := range strings.SplitSeq(b.String(), "\n\n") {
		if !strings.Contains(group, `"test":"`+value+`"`) || !strings.Contains(group, "(*Process).WaitContext.func") {
			continue
		}
		// The first group follows the profile's header line.
		if m := profileCount.FindStringSubmatch(group); m != nil {
			count, _ := strconv.Atoi(m[1])
			n += count
		}
	}
	return n
}

// profileCount is the count line of a group in a goroutine profile of
// debug 1.
var profileCount = regexp.MustCompile(`(?m)^(\d+) @`)

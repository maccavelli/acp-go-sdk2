package agentproc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// beating reports whether the beat file grows within 400 ms.
func beating(path string) bool {
	size := func() int64 {
		fi, err := os.Stat(path)
		if err != nil {
			return -1
		}
		return fi.Size()
	}
	before := size()
	time.Sleep(400 * time.Millisecond)
	return size() > before
}

// startBeating starts mode, whose beat grandchild appends to a file in the
// test's directory, and returns once the child is ready and the file grows.
func startBeating(t *testing.T, mode string) (*Process, string) {
	t.Helper()
	beat := filepath.Join(t.TempDir(), "beat")
	cmd := agentCmd(mode)
	cmd.Env = append(cmd.Env, beatEnv+"="+beat)
	p := startCmd(t, cmd, mode, &testClient{})
	if !beating(beat) {
		t.Fatalf("the %s agent's beat grandchild is not writing", mode)
	}
	return p, beat
}

// TestDescendantsEnded requires the processes the agent started to end with
// it: after Close, whether the agent exits at its input's end or is
// terminated, and when the agent exits of itself, the beat grandchild it
// started stops (0027-MADR-agent-process-helper.md, P8, P9, P14).
func TestDescendantsEnded(t *testing.T) {
	t.Parallel()
	if !hasTree {
		t.Skip("no tree of processes to end on this platform")
	}
	t.Run("the agent exits at its input's end", func(t *testing.T) {
		t.Parallel()
		p, beat := startBeating(t, "parent")
		if _, err := within(t, testGrace+slack, "Close", p.Close); err != nil {
			t.Errorf("Close = %v, want nil", err)
		}
		if beating(beat) {
			t.Error("the agent's grandchild still runs after Close")
		}
	})
	t.Run("the agent is terminated", func(t *testing.T) {
		t.Parallel()
		p, beat := startBeating(t, "parent-wait")
		if _, err := within(t, 2*testGrace+slack, "Close", p.Close); !errors.Is(err, ErrTerminated) && !errors.Is(err, ErrKilled) {
			t.Errorf("Close = %v, want ErrTerminated or ErrKilled", err)
		}
		if beating(beat) {
			t.Error("the agent's grandchild still runs after Close")
		}
	})
	t.Run("the agent exits of itself", func(t *testing.T) {
		t.Parallel()
		// The agent says it is ready once its grandchild has written, and
		// then exits at once, so the grandchild is not checked running here.
		beat := filepath.Join(t.TempDir(), "beat")
		cmd := agentCmd("parent-exit")
		cmd.Env = append(cmd.Env, beatEnv+"="+beat)
		p := startCmd(t, cmd, "parent-exit", &testClient{})
		if fi, err := os.Stat(beat); err != nil || fi.Size() == 0 {
			t.Fatalf("the agent's grandchild never wrote: %v", err)
		}
		if _, err := within(t, testGrace+slack, "Wait", p.Wait); err != nil {
			t.Errorf("Wait = %v, want nil", err)
		}
		select {
		case <-p.Conn().Done():
		default:
			t.Error("the connection is open after Wait")
		}
		if beating(beat) {
			t.Error("the agent's grandchild still runs after the agent exited")
		}
	})
}

// TestCallersCommandKept requires the agent to run with the Env and Dir the
// caller set on its Cmd (0027-MADR-agent-process-helper.md, question 8, and
// the README's "Closing and limits").
func TestCallersCommandKept(t *testing.T) {
	t.Parallel()
	dir, out := t.TempDir(), filepath.Join(t.TempDir(), "where")
	cmd := agentCmd("where")
	cmd.Env = append(cmd.Env, beatEnv+"="+out, "AGENTPROC_TEST_VALUE=the caller's")
	cmd.Dir = dir
	p := startCmd(t, cmd, "where", &testClient{})
	if err := p.Close(); err != nil {
		t.Fatalf("Close = %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	wd, value, _ := strings.Cut(string(b), "\n")
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := filepath.EvalSymlinks(wd); got != want || value != "the caller's" {
		t.Errorf("the agent ran in %q with AGENTPROC_TEST_VALUE %q, want %q and %q", wd, value, want, "the caller's")
	}
}

// blockingClient blocks every session update until release closes.
type blockingClient struct {
	acp.Client
	release chan struct{}
}

func (c *blockingClient) SessionUpdate(ctx context.Context, _ acp.SessionNotification) error {
	select {
	case <-c.release:
	case <-ctx.Done():
	}
	return nil
}

// TestConnectionEndStopsAgent requires the agent to be stopped when its
// connection ends while it runs: closed by the caller, and closed by a
// notification overflow (0027-MADR-agent-process-helper.md, question 6).
func TestConnectionEndStopsAgent(t *testing.T) {
	t.Parallel()
	t.Run("the caller closes the connection", func(t *testing.T) {
		t.Parallel()
		p := start(t, "stubborn", &testClient{})
		if err := p.Conn().Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := within(t, 2*testGrace+slack, "Wait", p.Wait); !errors.Is(err, ErrKilled) {
			t.Errorf("Wait = %v, want ErrKilled: the stubborn agent stopped", err)
		}
	})
	t.Run("a notification overflow", func(t *testing.T) {
		t.Parallel()
		client := &blockingClient{release: make(chan struct{})}
		t.Cleanup(func() { close(client.release) })
		cmd := agentCmd("flood")
		p, err := (&Command{Cmd: cmd, Grace: testGrace, ConnectionOptions: []acp.ConnectionOption{acp.WithMaxQueuedNotifications(1)}}).Start(client)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = p.Close() })
		if _, err := p.Conn().Initialize(t.Context(), acp.InitializeRequest{}); err != nil {
			t.Fatalf("Initialize = %v", err)
		}
		if err := p.Conn().Connection().SendNotification(t.Context(), "_test/go", map[string]any{}); err != nil {
			t.Fatalf("SendNotification = %v", err)
		}
		select {
		case <-p.Done():
		case <-time.After(testGrace + slack):
			t.Fatal("the agent still runs after its connection overflowed")
		}
		if err := p.Conn().Err(); !errors.Is(err, acp.ErrNotificationQueueOverflow) {
			t.Errorf("the connection's Err = %v, want the overflow", err)
		}
		if err := p.Wait(); err != nil {
			t.Errorf("Wait = %v, want nil: the flood agent exits at its input's end", err)
		}
	})
}

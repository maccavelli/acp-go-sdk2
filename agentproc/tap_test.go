package agentproc

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// Environment of the "bulk" child: the MiB it writes before it says it is
// ready, and the MiB it writes after a line of input.
const (
	bulkEnv     = "AGENTPROC_TEST_BULK"
	bulkMoreEnv = "AGENTPROC_TEST_BULK_MORE"
)

// framesStream is what the "frames" child writes: a line of 2 MiB, too long
// for a 1 MiB limit, a line that is not JSON, and a notification.
func framesStream() []byte {
	return []byte(strings.Repeat("x", 2<<20) + "\nnot json\n" + `{"jsonrpc":"2.0","method":"_x/n","params":{}}` + "\n")
}

// bulkStream is n lines of 1 MiB each, notifications of an extension method
// whose padding counts on across the stream, so that any byte left out or
// moved changes what follows it. A shorter stream is a longer one's
// beginning.
func bulkStream(n int) []byte {
	const prefix, suffix = `{"jsonrpc":"2.0","method":"_x/bulk","params":{"pad":"`, `"}}` + "\n"
	b := make([]byte, 0, n<<20+16)
	count := 0
	for range n {
		end := len(b) + 1<<20 - len(suffix)
		b = append(b, prefix...)
		for len(b) < end {
			// count as eight digits and a dot
			var tok [9]byte
			tok[8] = '.'
			for i, x := 7, count; i >= 0; i, x = i-1, x/10 {
				tok[i] = byte('0' + x%10)
			}
			b = append(b, tok[:]...)
			count++
		}
		b = append(b[:end], suffix...)
	}
	return b
}

// bulk64 is bulkStream(64), built once for the tests that compare with it.
var bulk64 = sync.OnceValue(func() []byte { return bulkStream(64) })

// bulk is the "bulk" child: it writes the first bulkEnv MiB of a bulkStream,
// says it is ready, waits for a line of input, writes the bulkMoreEnv MiB
// that follow, and exits at its input's end.
func bulk() int {
	first, err1 := strconv.Atoi(os.Getenv(bulkEnv))
	more, err2 := strconv.Atoi(os.Getenv(bulkMoreEnv))
	if err1 != nil || err2 != nil {
		return 1
	}
	s := bulkStream(first + more)
	if _, err := os.Stdout.Write(s[:first<<20]); err != nil {
		return 1
	}
	ready()
	in := bufio.NewReader(os.Stdin)
	if _, err := in.ReadString('\n'); err != nil {
		return 1
	}
	if _, err := os.Stdout.Write(s[first<<20:]); err != nil {
		return 1
	}
	_, _ = io.Copy(io.Discard, in)
	return 0
}

// lastMiB is how many MiB of a bulkStream the "last" child writes: more than
// a pipe holds.
const lastMiB = 2

// last is the "last" child: it says it is ready, waits for a line of input,
// writes lastMiB of a bulkStream, and exits at once, without reading the rest
// of its input.
func last() int {
	ready()
	if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
		return 1
	}
	if _, err := os.Stdout.Write(bulkStream(lastMiB)); err != nil {
		return 1
	}
	return 0
}

// capture is a tap that keeps what it receives, each Write after delay; and,
// when gate is set, whose first Write waits until gate is closed.
type capture struct {
	mu    sync.Mutex
	b     bytes.Buffer
	delay time.Duration
	gate  chan struct{}
	once  sync.Once
}

func (c *capture) Write(p []byte) (int, error) {
	if c.gate != nil {
		c.once.Do(func() { <-c.gate })
	}
	time.Sleep(c.delay)
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.b.Write(p)
}

func (c *capture) bytes() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return bytes.Clone(c.b.Bytes())
}

// failing is a tap whose every Write fails.
type failing struct{}

func (failing) Write([]byte) (int, error) { return 0, errors.New("the tap fails") }

// startTapped starts the test binary as an agent in mode with env, grace and
// the taps, waits until it says it is ready, and closes it when the test
// ends.
func startTapped(t *testing.T, mode string, env []string, grace time.Duration, outTap, inTap io.Writer, opts ...acp.ConnectionOption) *Process {
	t.Helper()
	p, ready := launchTapped(t, mode, env, grace, outTap, inTap, opts...)
	awaitReady(t, mode, ready)
	return p
}

// launchTapped is startTapped without the wait, for a test that must
// register a cleanup to run before the agent's Close. The connection's log is
// discarded: these agents write lines it refuses before initialize, each
// logged.
func launchTapped(t *testing.T, mode string, env []string, grace time.Duration, outTap, inTap io.Writer, opts ...acp.ConnectionOption) (*Process, <-chan struct{}) {
	t.Helper()
	cmd := agentCmd(mode)
	cmd.Env = append(cmd.Env, env...)
	w := &readyWriter{ready: make(chan struct{})}
	cmd.Stderr = w
	opts = append([]acp.ConnectionOption{acp.WithLogger(slog.New(slog.DiscardHandler))}, opts...)
	p, err := (&Command{Cmd: cmd, Grace: grace, OutputTap: outTap, InputTap: inTap, ConnectionOptions: opts}).Start(&testClient{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p, w.ready
}

// awaitReady waits until the agent in mode says it is ready.
func awaitReady(t *testing.T, mode string, ready <-chan struct{}) {
	t.Helper()
	select {
	case <-ready:
	case <-time.After(readyWait):
		t.Fatalf("the %s agent did not say it was ready within %v", mode, readyWait)
	}
}

// blockedTap is a capture whose first Write blocks until the test releases
// it, which it does at the latest when it ends, before the agent's Close,
// which the blocked Write would otherwise hold when the tap is not one of
// its own.
func blockedTap(t *testing.T) (*capture, func()) {
	t.Helper()
	gate := make(chan struct{})
	release := sync.OnceFunc(func() { close(gate) })
	return &capture{gate: gate}, release
}

// tappedFrames runs the "frames" agent with both taps, a 1 MiB limit, and an
// output tap that takes 1 ms a write, so that Wait must wait for it to drain,
// and returns what each tap received once Close has returned. Its grace is
// slack, not testGrace: Wait gives the taps up to the grace to drain, and a
// loaded runner under -race has held this tap past 300 ms
// (0030-PLAN-record-initialize-on-arrival.md, step 12).
func tappedFrames(t *testing.T) (output, input []byte, p *Process) {
	t.Helper()
	out, in := &capture{delay: time.Millisecond}, &capture{}
	p = startTapped(t, "frames", nil, slack, out, in, acp.WithMaxMessageSize(1<<20))
	// The connection has read and answered the two lines it refuses.
	deadline := time.Now().Add(readyWait)
	for strings.Count(string(in.bytes()), "\n") < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("the input tap holds %q, not the connection's two answers, after %v", in.bytes(), readyWait)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close = %v", err)
	}
	return out.bytes(), in.bytes(), p
}

// TestOutputTapVerbatim requires the output tap to receive every byte the
// agent wrote, in order, the line too long and the line that is not JSON
// included, all of it by the time Close returns
// (0030-MADR-record-initialize-on-arrival.md, the amendment of 2026-10-21,
// T1).
func TestOutputTapVerbatim(t *testing.T) {
	t.Parallel()
	got, _, p := tappedFrames(t)
	if want := framesStream(); !bytes.Equal(got, want) {
		t.Errorf("the output tap received %d bytes, want the agent's %d, equal", len(got), len(want))
	}
	if out, in := p.DroppedTapBytes(); out != 0 || in != 0 {
		t.Errorf("DroppedTapBytes = %d, %d; want 0, 0", out, in)
	}
}

// TestInputTapSeesWrites requires the input tap to receive what the
// connection wrote to the agent: its answers to the line too long, -32600,
// and to the line that is not JSON, -32700, each a whole line (T1).
func TestInputTapSeesWrites(t *testing.T) {
	t.Parallel()
	_, got, _ := tappedFrames(t)
	lines := strings.Split(strings.TrimSuffix(string(got), "\n"), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"code":-32600`) || !strings.Contains(lines[0], "message exceeds 1048576 bytes") ||
		!strings.Contains(lines[1], `"code":-32700`) {
		t.Errorf("the input tap received %q, want the -32600 and -32700 answers, a line each", got)
	}
}

// TestTapErrorIgnored requires taps whose every Write fails to leave the
// connection whole: the burst agent's updates all reach the client, and Wait
// returns nil, as without them.
func TestTapErrorIgnored(t *testing.T) {
	t.Parallel()
	client := &testClient{}
	p, err := (&Command{Cmd: agentCmd("burst"), Grace: testGrace, OutputTap: failing{}, InputTap: failing{}}).Start(client)
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
	if _, err := within(t, slack, "Wait", p.Wait); err != nil {
		t.Fatalf("Wait = %v, want nil", err)
	}
	if n := client.updates.Load(); n != burstCount {
		t.Errorf("the client received %d of the %d session updates, its taps failing", n, burstCount)
	}
}

// TestTapsUnset requires the connection to read and write the agent's pipes
// themselves when no tap is set, with no tap and so no tap goroutine.
func TestTapsUnset(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close(); _ = w.Close() })
	stdin := &sharedClose{File: w}
	in, out, inTap, outTap := streams(stdin, r, nil, nil, nil)
	if in != io.Writer(stdin) || out != io.Reader(r) || inTap != nil || outTap != nil {
		t.Errorf("streams with no tap = %T, %T, %v, %v; want the input and the output themselves, and no tap", in, out, inTap, outTap)
	}
}

// TestBlockedTapDetached requires an output tap whose Write blocks to hold
// neither the connection nor Close, and to be detached once 32 MiB behind:
// the agent writes 48 MiB, which the connection reads while the tap is
// blocked; DroppedTapBytes then reports about the last 16 MiB dropped,
// before Close; Close returns within the grace and the slack; and, released,
// the tap has received the stream's beginning
// (0030-MADR-record-initialize-on-arrival.md, the amendment of 2026-10-21,
// A1f and questions 9 and 10).
func TestBlockedTapDetached(t *testing.T) {
	t.Parallel()
	tap, release := blockedTap(t)
	// The agent says it is ready once it has written its 48 MiB.
	p, ready := launchTapped(t, "bulk", []string{bulkEnv + "=48", bulkMoreEnv + "=0"}, testGrace, tap, nil)
	t.Cleanup(release)
	awaitReady(t, "bulk", ready)
	// At most the pipe's capacity, and a read's, are still unread.
	const want, margin = 16 << 20, 256 << 10
	deadline := time.Now().Add(readyWait)
	out, _ := p.DroppedTapBytes()
	for ; out < want-margin; out, _ = p.DroppedTapBytes() {
		if time.Now().After(deadline) {
			t.Fatalf("a tap blocked through 48 MiB dropped %d bytes in %v, want about the last 16 MiB", out, readyWait)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if out > want+margin {
		t.Errorf("a tap blocked through 48 MiB dropped %d bytes, want about the last 16 MiB: detached before 32 MiB", out)
	}
	if err := p.Conn().Connection().SendNotification(t.Context(), "_test/go", map[string]any{}); err != nil {
		t.Fatalf("SendNotification = %v", err)
	}
	if _, err := within(t, testGrace+slack, "Close", p.Close); err != nil {
		t.Fatalf("Close = %v", err)
	}
	release()
	deadline = time.Now().Add(slack)
	for len(tap.bytes()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	got := tap.bytes()
	if want := bulk64()[:48<<20]; len(got) == 0 || !bytes.HasPrefix(want, got) {
		t.Errorf("the released tap received %d bytes, not the beginning of the stream", len(got))
	}
}

// TestDetachedTapIsPrefix requires a detached tap to receive nothing after
// the first byte it lost: the tap blocks until it is detached, is released
// while the agent still has 8 MiB to write, and what it received is the
// stream's beginning (0030-MADR-record-initialize-on-arrival.md, the
// amendment of 2026-10-21, question 9).
func TestDetachedTapIsPrefix(t *testing.T) {
	t.Parallel()
	tap, release := blockedTap(t)
	p, ready := launchTapped(t, "bulk", []string{bulkEnv + "=56", bulkMoreEnv + "=8"}, 5*time.Second, tap, nil)
	t.Cleanup(release)
	awaitReady(t, "bulk", ready)
	deadline := time.Now().Add(readyWait)
	for out, _ := p.DroppedTapBytes(); out == 0; out, _ = p.DroppedTapBytes() {
		if time.Now().After(deadline) {
			t.Fatalf("the tap, blocked through 56 MiB, was not detached within %v", readyWait)
		}
		time.Sleep(10 * time.Millisecond)
	}
	release()
	// The agent writes its last 8 MiB once this arrives.
	if err := p.Conn().Connection().SendNotification(t.Context(), "_test/go", map[string]any{}); err != nil {
		t.Fatalf("SendNotification = %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close = %v", err)
	}
	got, want := tap.bytes(), bulk64()
	if !bytes.HasPrefix(want, got) {
		t.Errorf("the detached tap received %d bytes that are not the stream's beginning", len(got))
	}
	out, _ := p.DroppedTapBytes()
	if total := uint64(len(got)) + out; total != uint64(len(want)) {
		t.Errorf("the tap received %d bytes and dropped %d, %d in all; the agent wrote %d", len(got), out, total, len(want))
	}
}

// TestCloseReadsTheAgentsLastOutput requires Close, called as the agent
// writes its last output and exits, to leave the connection to read that
// output to its end: the output tap receives all of it, and nothing is
// dropped. The connection's reader is held back, 5 ms before each read from
// the agent's last write on, so that it is behind the agent when the agent
// exits, as under the race detector in CI
// (0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phases 2 and 3).
func TestCloseReadsTheAgentsLastOutput(t *testing.T) {
	t.Parallel()
	var slow atomic.Bool
	cmd := agentCmd("last")
	w := &readyWriter{ready: make(chan struct{})}
	cmd.Stderr = w
	tap := &capture{}
	c := &Command{
		Cmd:               cmd,
		Grace:             5 * time.Second,
		OutputTap:         tap,
		ConnectionOptions: []acp.ConnectionOption{acp.WithLogger(slog.New(slog.DiscardHandler))},
		readHook: func() {
			if slow.Load() {
				time.Sleep(5 * time.Millisecond)
			}
		},
	}
	p, err := c.Start(&testClient{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	awaitReady(t, "last", w.ready)
	slow.Store(true)
	// The agent writes its last output once this arrives, and exits.
	if err := p.Conn().Connection().SendNotification(t.Context(), "_test/go", map[string]any{}); err != nil {
		t.Fatalf("SendNotification = %v", err)
	}
	if _, err := within(t, c.Grace+slack, "Close", p.Close); err != nil {
		t.Fatalf("Close = %v, want nil: the agent exits of itself", err)
	}
	got, want := tap.bytes(), bulkStream(lastMiB)
	if out, _ := p.DroppedTapBytes(); !bytes.Equal(got, want) || out != 0 {
		t.Errorf("the tap received %d bytes and dropped %d of the agent's %d: the connection was closed before it read the agent's last output",
			len(got), out, len(want))
	}
}

// TestTapDrainBoundedByGrace requires Wait to wait for a tap no longer than
// the grace: a tap blocked in its first Write until the test ends still holds
// what the agent's 24 MiB filled its queue with when the grace ends, and Wait
// returns, counting the rest as dropped
// (0030-MADR-record-initialize-on-arrival.md, the amendment of 2026-10-21, A1e
// and question 11). A tap that was merely slow could keep pace with an agent
// read slowly, as under the race detector on Windows, and leave nothing queued
// (0034-MADR-race-detector-on-windows-ci.md, R6 to R8).
func TestTapDrainBoundedByGrace(t *testing.T) {
	t.Parallel()
	tap, release := blockedTap(t)
	p := startTapped(t, "bulk", []string{bulkEnv + "=24", bulkMoreEnv + "=0"}, testGrace, tap, nil)
	// Registered after startTapped's Close, so that it runs first.
	t.Cleanup(release)
	if err := p.Conn().Connection().SendNotification(t.Context(), "_test/go", map[string]any{}); err != nil {
		t.Fatalf("SendNotification = %v", err)
	}
	if _, err := within(t, testGrace+slack, "Close", p.Close); err != nil {
		t.Fatalf("Close = %v", err)
	}
	if out, _ := p.DroppedTapBytes(); out == 0 {
		t.Error("DroppedTapBytes reports nothing dropped by a tap still writing when the grace ended")
	}
}

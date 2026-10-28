package agentproc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// burstCount is how many session updates the "burst" agent writes before it
// exits.
const burstCount = 1000

// ready tells the test, on stderr, that the child has set itself up.
func ready() { fmt.Fprintln(os.Stderr, "ready") }

// child runs one behaviour of an agent process and returns its exit status
// (0027-MADR-agent-process-helper.md, "How a child process behaves").
func child(mode string) int {
	switch mode {
	case "frames": // writes framesStream, then exits at its input's end (tap_test.go)
		if _, err := os.Stdout.Write(framesStream()); err != nil {
			return 1
		}
		ready()
		_, _ = io.Copy(io.Discard, os.Stdin)
		return 0
	case "bulk": // writes a bulkStream in two parts, the second after a line of input (tap_test.go)
		return bulk()
	case "last": // writes a bulkStream after a line of input, and exits at once (tap_test.go)
		return last()
	case "eof": // exits 0 at its input's end
		ready()
		_, _ = io.Copy(io.Discard, os.Stdin)
		return 0
	case "term": // ignores its input's end; exits 3 on SIGTERM
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGTERM)
		go func() { _, _ = io.Copy(io.Discard, os.Stdin) }()
		ready()
		<-c
		return 3
	case "stubborn": // ignores its input's end and SIGTERM
		signal.Ignore(syscall.SIGTERM, os.Interrupt)
		go func() { _, _ = io.Copy(io.Discard, os.Stdin) }()
		ready()
		time.Sleep(time.Hour)
		return 0
	case "burst": // answers initialize, writes burstCount session updates, and exits at once
		return burst()
	case "stderr-fail": // writes "line 1" to "line 100" to stderr, and exits 1 (stderr_test.go)
		for i := 1; i <= 100; i++ {
			fmt.Fprintf(os.Stderr, "line %d\n", i)
		}
		return 1
	case "parent-stderr": // starts a heartbeat on its stderr, then exits at its input's end
		g := exec.Command(testBinary(), "-test.run=^$")
		g.Env = append(os.Environ(), childEnv+"=heartbeat-stderr")
		g.Stderr = os.Stderr
		if err := g.Start(); err != nil {
			return 1
		}
		ready()
		_, _ = io.Copy(io.Discard, os.Stdin)
		return 0
	case "heartbeat-stderr": // ignores SIGTERM; writes a line to stderr every 50 ms for a minute, until a write fails
		signal.Ignore(syscall.SIGTERM)
		for range 1200 {
			if _, err := fmt.Fprintln(os.Stderr, "beat"); err != nil {
				return 0
			}
			time.Sleep(50 * time.Millisecond)
		}
		return 0
	}
	if status, ok := tieChild(mode); ok {
		return status
	}
	if status, ok := platformChild(mode); ok {
		return status
	}
	fmt.Fprintf(os.Stderr, "unknown child mode %q\n", mode)
	return 2
}

// beatEnv names the file the "beat" grandchild appends to while it lives.
const beatEnv = "AGENTPROC_TEST_BEAT"

// startBeat starts the "beat" grandchild on this child's standard output,
// which it holds open as a launcher's agent does, and returns it once its
// file has grown. setup, when not nil, adjusts its command first.
func startBeat(setup func(*exec.Cmd)) (*exec.Cmd, error) {
	return startBeatAt(os.Getenv(beatEnv), setup)
}

// startBeatAt is startBeat for a beat that appends to file.
func startBeatAt(file string, setup func(*exec.Cmd)) (*exec.Cmd, error) {
	g := exec.Command(testBinary(), "-test.run=^$")
	g.Env = append(os.Environ(), childEnv+"=beat", beatEnv+"="+file)
	g.Stdout = os.Stdout
	if setup != nil {
		setup(g)
	}
	if err := g.Start(); err != nil {
		return nil, err
	}
	// A test that finds the beat running when it should not ends it by this.
	if err := os.WriteFile(file+".gpid", []byte(strconv.Itoa(g.Process.Pid)), 0o600); err != nil {
		return g, err
	}
	for range 400 {
		if fi, err := os.Stat(file); err == nil && fi.Size() > 0 {
			return g, nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return g, errors.New("the beat grandchild wrote nothing in 10 s")
}

// tieChild runs the modes of step 3's tests, which tie the agent's process
// to its connection (0027-PLAN-agent-process-helper.md, step 3).
func tieChild(mode string) (int, bool) {
	switch mode {
	case "beat": // appends a byte to its file every 50 ms for a minute
		f, err := os.OpenFile(os.Getenv(beatEnv), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return 1, true
		}
		for range 1200 {
			if _, err := f.WriteString("b"); err != nil {
				return 1, true
			}
			time.Sleep(50 * time.Millisecond)
		}
		return 0, true
	case "parent": // starts a beat, then exits at its input's end
		if _, err := startBeat(nil); err != nil {
			return 1, true
		}
		ready()
		_, _ = io.Copy(io.Discard, os.Stdin)
		return 0, true
	case "parent-wait": // starts a beat and waits for it, ignoring its input's end
		g, err := startBeat(nil)
		if err != nil {
			return 1, true
		}
		go func() { _, _ = io.Copy(io.Discard, os.Stdin) }()
		ready()
		_ = g.Wait()
		return 0, true
	case "parent-exit": // starts a beat, then exits of itself
		if _, err := startBeat(nil); err != nil {
			return 1, true
		}
		ready()
		return 0, true
	case "closeout": // closes its output, then exits at its input's end
		if err := os.Stdout.Close(); err != nil {
			return 1, true
		}
		ready()
		_, _ = io.Copy(io.Discard, os.Stdin)
		return 0, true
	case "where": // writes its working directory and AGENTPROC_TEST_VALUE to the file beatEnv names, then exits at its input's end
		wd, err := os.Getwd()
		if err != nil {
			return 1, true
		}
		if err := os.WriteFile(os.Getenv(beatEnv), []byte(wd+"\n"+os.Getenv("AGENTPROC_TEST_VALUE")), 0o600); err != nil {
			return 1, true
		}
		ready()
		_, _ = io.Copy(io.Discard, os.Stdin)
		return 0, true
	case "flood": // answers initialize; after the client's next message, writes 10 updates; exits at its input's end
		in := bufio.NewReader(os.Stdin)
		if !answerInitialize(in) {
			return 1, true
		}
		var b strings.Builder
		for i := range 10 {
			fmt.Fprintf(&b, `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s1","update":`+
				`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"%d"}}}}`+"\n", i)
		}
		if _, err := os.Stdout.WriteString(b.String()); err != nil {
			return 1, true
		}
		_, _ = io.Copy(io.Discard, in)
		return 0, true
	}
	return 0, false
}

// answerInitialize answers the client's initialize request read from in, and
// then reads the client's next message, which it sends once initialize has
// completed on its side.
func answerInitialize(in *bufio.Reader) bool {
	line, err := in.ReadBytes('\n')
	if err != nil {
		return false
	}
	var req struct {
		ID json.RawMessage `json:"id"`
	}
	if json.Unmarshal(line, &req) != nil {
		return false
	}
	if _, err := fmt.Fprintf(os.Stdout, `{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":1,"agentCapabilities":{},"authMethods":[]}}`+"\n",
		req.ID); err != nil {
		return false
	}
	_, err = in.ReadBytes('\n')
	return err == nil
}

// burst answers the client's initialize request; once the client's next
// message says initialize has completed on its side, it writes burstCount
// session updates in one write and exits without waiting for them to be read.
func burst() int {
	if !answerInitialize(bufio.NewReader(os.Stdin)) {
		return 1
	}
	var b strings.Builder
	for i := range burstCount {
		fmt.Fprintf(&b, `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s1","update":`+
			`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"%d"}}}}`+"\n", i)
	}
	if _, err := os.Stdout.WriteString(b.String()); err != nil {
		return 1
	}
	return 0
}

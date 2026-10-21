package agentproc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
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
	case "parent-stderr": // starts a heartbeat on its stderr, then exits at its input's end
		g := exec.Command(os.Args[0], "-test.run=^$")
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
	fmt.Fprintf(os.Stderr, "unknown child mode %q\n", mode)
	return 2
}

// burst answers the client's initialize request; once the client's next
// message says initialize has completed on its side, it writes burstCount
// session updates in one write and exits without waiting for them to be read.
func burst() int {
	in := bufio.NewReader(os.Stdin)
	line, err := in.ReadBytes('\n')
	if err != nil {
		return 1
	}
	var req struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(line, &req); err != nil {
		return 1
	}
	if _, err := fmt.Fprintf(os.Stdout, `{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":1,"agentCapabilities":{},"authMethods":[]}}`+"\n",
		req.ID); err != nil {
		return 1
	}
	if _, err := in.ReadBytes('\n'); err != nil {
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

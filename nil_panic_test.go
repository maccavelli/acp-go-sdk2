package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestConstructorsPanicOnNil requires each constructor to panic on each nil
// argument with a message naming it, before it hands the agent or client the
// connection. The reader is a pipe whose writer is dropped, so a connection
// started before the panic would leak a goroutine the leak guard reports
// (0024-PLAN-behaviour-fixes.md, A1).
func TestConstructorsPanicOnNil(t *testing.T) {
	t.Parallel()
	agent, client := &awareAgent{}, &awareClient{}
	for _, tc := range []struct {
		want  string
		build func(w io.Writer, r io.Reader)
	}{
		{"acp: NewConnection: nil peerInput", func(_ io.Writer, r io.Reader) { NewConnection(nil, nil, r) }},
		{"acp: NewConnection: nil peerOutput", func(w io.Writer, _ io.Reader) { NewConnection(nil, w, nil) }},
		{"acp: NewAgentSideConnection: nil agent", func(w io.Writer, r io.Reader) { NewAgentSideConnection(nil, w, r) }},
		{"acp: NewAgentSideConnection: nil peerInput", func(_ io.Writer, r io.Reader) { NewAgentSideConnection(agent, nil, r) }},
		{"acp: NewAgentSideConnection: nil peerOutput", func(w io.Writer, _ io.Reader) { NewAgentSideConnection(agent, w, nil) }},
		{"acp: NewClientSideConnection: nil client", func(w io.Writer, r io.Reader) { NewClientSideConnection(nil, w, r) }},
		{"acp: NewClientSideConnection: nil peerInput", func(_ io.Writer, r io.Reader) { NewClientSideConnection(client, nil, r) }},
		{"acp: NewClientSideConnection: nil peerOutput", func(w io.Writer, _ io.Reader) { NewClientSideConnection(client, w, nil) }},
	} {
		r, _ := io.Pipe()
		got := func() (v any) {
			defer func() { v = recover() }()
			tc.build(io.Discard, r)
			return nil
		}()
		if got != tc.want {
			t.Errorf("panic %v, want %q", got, tc.want)
		}
		if agent.handed.Load() || client.handed.Load() {
			t.Fatalf("%s: the agent or client was handed the connection before the panic", tc.want)
		}
	}
}

// handlerPanicEnv names the child process's case in
// TestHandlerPanicEndsProcess.
const handlerPanicEnv = "ACP_TEST_HANDLER_PANIC"

// TestHandlerPanicEndsProcess requires a handler that panics to end the
// process with its panic, as the docs of MethodHandler, Agent and Client say:
// the connection does not recover it. Each case runs this test again as a
// child process, in which a raw handler, or an Agent's Initialize, panics
// (0024-PLAN-behaviour-fixes.md, A1).
func TestHandlerPanicEndsProcess(t *testing.T) {
	const request = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}` + "\n"
	switch os.Getenv(handlerPanicEnv) {
	case "raw":
		NewConnection(func(context.Context, string, json.RawMessage) (any, *RequestError) {
			panic("handler bug: raw")
		}, io.Discard, strings.NewReader(request))
		time.Sleep(10 * time.Second) // the panic ends the process first
		return
	case "agent":
		NewAgentSideConnection(agentFuncs{InitializeFunc: func(context.Context, InitializeRequest) (InitializeResponse, error) {
			panic("handler bug: agent")
		}}, io.Discard, strings.NewReader(request))
		time.Sleep(10 * time.Second)
		return
	}
	t.Parallel()
	for _, c := range []string{"raw", "agent"} {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHandlerPanicEndsProcess$", "-test.count=1")
		cmd.Env = append(os.Environ(), handlerPanicEnv+"="+c)
		out, err := cmd.CombinedOutput()
		if ee, ok := errors.AsType[*exec.ExitError](err); !ok || ee.ExitCode() == 0 {
			t.Errorf("%s: the child exited %v, want a non-zero exit\n%s", c, err, out)
		}
		if want := "panic: handler bug: " + c; !strings.Contains(string(out), want) {
			t.Errorf("%s: the child's output lacks %q:\n%s", c, want, out)
		}
	}
}

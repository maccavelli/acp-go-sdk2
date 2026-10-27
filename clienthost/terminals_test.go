package clienthost

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
	"github.com/maccavelli/acp-go-sdk2/internal/proctree"
)

// readyWait bounds how long a child may take to start and say it is ready.
const readyWait = 30 * time.Second

// newTerminals is a Terminals running testCommand, closed when the test ends.
func newTerminals(t *testing.T) *Terminals {
	t.Helper()
	terms := &Terminals{Command: testCommand}
	t.Cleanup(func() { _ = terms.Close() })
	return terms
}

// create creates a terminal running mode with env, failing the test on an
// error.
func create(t *testing.T, terms *Terminals, mode string, limit *uint64, env ...acp.EnvVariable) acp.TerminalId {
	t.Helper()
	resp, err := terms.CreateTerminal(t.Context(), acp.CreateTerminalRequest{SessionId: "s", Command: mode, Env: env, OutputByteLimit: limit})
	if err != nil {
		t.Fatalf("CreateTerminal(%s) = %v", mode, err)
	}
	return resp.TerminalId
}

// waitExit waits for id's exit, failing the test on an error or after
// readyWait.
func waitExit(t *testing.T, terms *Terminals, id acp.TerminalId) acp.WaitForTerminalExitResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), readyWait)
	defer cancel()
	resp, err := terms.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{SessionId: "s", TerminalId: id})
	if err != nil {
		t.Fatalf("WaitForTerminalExit(%s) = %v", id, err)
	}
	return resp
}

// awaitReady waits until id's output says "ready".
func awaitReady(t *testing.T, terms *Terminals, id acp.TerminalId) {
	t.Helper()
	deadline := time.Now().Add(readyWait)
	for {
		resp, err := terms.TerminalOutput(t.Context(), acp.TerminalOutputRequest{SessionId: "s", TerminalId: id})
		if err != nil {
			t.Fatalf("TerminalOutput(%s) = %v", id, err)
		}
		if strings.Contains(resp.Output, "ready") {
			return
		}
		if resp.ExitStatus != nil || time.Now().After(deadline) {
			t.Fatalf("%s did not say it was ready: %+v", id, resp)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// code is the JSON-RPC code of err, or 0.
func code(err error) int {
	if re, ok := errors.AsType[*acp.RequestError](err); ok {
		return re.Code
	}
	return 0
}

// TestTerminalOutputAndExit requires a terminal to keep its command's
// standard output and standard error, to report its exit code by
// terminal/wait_for_exit and by terminal/output once it has exited, and no
// exit status before (0041-PLAN-close-the-gaps-the-consumers-wait-on.md,
// phase 8).
func TestTerminalOutputAndExit(t *testing.T) {
	t.Parallel()
	terms := newTerminals(t)
	id := create(t, terms, "write", nil, acp.EnvVariable{Name: outEnv, Value: "out\n"}, acp.EnvVariable{Name: errEnv, Value: "err\n"},
		acp.EnvVariable{Name: exitEnv, Value: "3"})
	exit := waitExit(t, terms, id)
	if exit.ExitCode == nil || *exit.ExitCode != 3 || exit.Signal != nil {
		t.Errorf("WaitForTerminalExit = %+v, want exit code 3 and no signal", exit)
	}
	resp, err := terms.TerminalOutput(t.Context(), acp.TerminalOutputRequest{SessionId: "s", TerminalId: id})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Output, "out\n") || !strings.Contains(resp.Output, "err\n") || resp.Truncated {
		t.Errorf("TerminalOutput = %q, truncated %v; want both streams, not truncated", resp.Output, resp.Truncated)
	}
	if resp.ExitStatus == nil || resp.ExitStatus.ExitCode == nil || *resp.ExitStatus.ExitCode != 3 {
		t.Errorf("TerminalOutput's exit status = %+v, want exit code 3", resp.ExitStatus)
	}
	running := create(t, terms, "sleep", nil)
	awaitReady(t, terms, running)
	if resp, err := terms.TerminalOutput(t.Context(), acp.TerminalOutputRequest{SessionId: "s", TerminalId: running}); err != nil ||
		resp.ExitStatus != nil {
		t.Errorf("TerminalOutput of a running command = %+v, %v; want no exit status", resp, err)
	}
}

// TestTerminalOutputByteLimit requires a terminal whose command writes past
// its outputByteLimit, in writes that split its characters, to keep valid
// UTF-8 of at most the limit, the output's end, and to say it truncated, for
// characters of 2, 3 and 4 bytes; and MaxOutputBytes to bound a larger
// limit.
func TestTerminalOutputByteLimit(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		char     string
		limit    uint64
		maxBytes int
	}{{"é", 101, 0}, {"€", 101, 0}, {"😀", 101, 0}, {"😀", 1 << 20, 102}} {
		terms := newTerminals(t)
		terms.MaxOutputBytes = c.maxBytes
		limit := c.limit
		id := create(t, terms, "repeat", &limit, acp.EnvVariable{Name: charEnv, Value: c.char}, acp.EnvVariable{Name: countEnv, Value: "5000"})
		waitExit(t, terms, id)
		resp, err := terms.TerminalOutput(t.Context(), acp.TerminalOutputRequest{SessionId: "s", TerminalId: id})
		if err != nil {
			t.Fatal(err)
		}
		bound := int(c.limit)
		if c.maxBytes > 0 && c.maxBytes < bound {
			bound = c.maxBytes
		}
		switch {
		case !utf8.ValidString(resp.Output):
			t.Errorf("%q: the output is not valid UTF-8", c.char)
		case len(resp.Output) > bound || len(resp.Output) < bound-(utf8.UTFMax-1):
			t.Errorf("%q: kept %d bytes, want at most %d and at least %d", c.char, len(resp.Output), bound, bound-(utf8.UTFMax-1))
		case resp.Output != strings.Repeat(c.char, len(resp.Output)/len(c.char)):
			t.Errorf("%q: the output is not whole characters of the stream", c.char)
		case !resp.Truncated:
			t.Errorf("%q: truncated = false", c.char)
		}
	}
}

// TestTerminalKill requires terminal/kill to end a running command and keep
// the terminal: its exit is SIGKILL on Unix and status 1 on Windows, its
// output stays readable, a second kill does nothing, and release then
// forgets it.
func TestTerminalKill(t *testing.T) {
	t.Parallel()
	terms := newTerminals(t)
	id := create(t, terms, "sleep", nil)
	awaitReady(t, terms, id)
	if _, err := terms.KillTerminal(t.Context(), acp.KillTerminalRequest{SessionId: "s", TerminalId: id}); err != nil {
		t.Fatalf("KillTerminal = %v", err)
	}
	exit := waitExit(t, terms, id)
	switch {
	case runtime.GOOS == "windows":
		if exit.ExitCode == nil || *exit.ExitCode != 1 {
			t.Errorf("WaitForTerminalExit after kill = %+v, want exit code 1", exit)
		}
	case exit.Signal == nil || *exit.Signal != "SIGKILL" || exit.ExitCode != nil:
		t.Errorf("WaitForTerminalExit after kill = %+v, want signal SIGKILL", exit)
	}
	if resp, err := terms.TerminalOutput(t.Context(), acp.TerminalOutputRequest{SessionId: "s", TerminalId: id}); err != nil ||
		!strings.Contains(resp.Output, "ready") || resp.ExitStatus == nil {
		t.Errorf("TerminalOutput after kill = %+v, %v; want the output and the exit status", resp, err)
	}
	if _, err := terms.KillTerminal(t.Context(), acp.KillTerminalRequest{SessionId: "s", TerminalId: id}); err != nil {
		t.Errorf("a second KillTerminal = %v, want nil", err)
	}
	if _, err := terms.ReleaseTerminal(t.Context(), acp.ReleaseTerminalRequest{SessionId: "s", TerminalId: id}); err != nil {
		t.Errorf("ReleaseTerminal = %v", err)
	}
}

// TestTerminalEndsTree requires kill and release to end the processes the
// command started: the "parent" command's beat grandchild stops.
func TestTerminalEndsTree(t *testing.T) {
	t.Parallel()
	if !proctree.HasTree {
		t.Skip("no tree of processes to end on this platform")
	}
	for name, end := range map[string]func(*Terminals, acp.TerminalId) error{
		"kill": func(terms *Terminals, id acp.TerminalId) error {
			_, err := terms.KillTerminal(context.Background(), acp.KillTerminalRequest{SessionId: "s", TerminalId: id})
			return err
		},
		"release": func(terms *Terminals, id acp.TerminalId) error {
			_, err := terms.ReleaseTerminal(context.Background(), acp.ReleaseTerminalRequest{SessionId: "s", TerminalId: id})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			terms := newTerminals(t)
			beat := filepath.Join(t.TempDir(), "beat")
			id := create(t, terms, "parent", nil, acp.EnvVariable{Name: beatEnv, Value: beat})
			awaitReady(t, terms, id)
			if !beating(beat) {
				t.Fatal("the beat grandchild is not writing: the test proves nothing")
			}
			if err := end(terms, id); err != nil {
				t.Fatalf("%s = %v", name, err)
			}
			if beating(beat) {
				t.Errorf("the command's grandchild still runs after %s", name)
			}
		})
	}
}

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

// TestTerminalWaitContext requires terminal/wait_for_exit to return its
// context's cause when the context ends before the command exits, leaving
// the command running.
func TestTerminalWaitContext(t *testing.T) {
	t.Parallel()
	terms := newTerminals(t)
	id := create(t, terms, "sleep", nil)
	why := errors.New("the agent cancelled")
	ctx, cancel := context.WithCancelCause(t.Context())
	time.AfterFunc(100*time.Millisecond, func() { cancel(why) })
	if _, err := terms.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{SessionId: "s", TerminalId: id}); !errors.Is(err, why) {
		t.Errorf("WaitForTerminalExit = %v, want its context's cause", err)
	}
	if resp, err := terms.TerminalOutput(t.Context(), acp.TerminalOutputRequest{SessionId: "s", TerminalId: id}); err != nil ||
		resp.ExitStatus != nil {
		t.Errorf("TerminalOutput after the wait ended = %+v, %v; want the command still running", resp, err)
	}
}

// TestTerminalUnknownID requires every method to answer an id never created,
// and one released, with resource-not-found (-32002).
func TestTerminalUnknownID(t *testing.T) {
	t.Parallel()
	terms := newTerminals(t)
	released := create(t, terms, "write", nil)
	if _, err := terms.ReleaseTerminal(t.Context(), acp.ReleaseTerminalRequest{SessionId: "s", TerminalId: released}); err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	for _, id := range []acp.TerminalId{"terminal-none", released} {
		errs := map[string]error{}
		_, errs["output"] = terms.TerminalOutput(ctx, acp.TerminalOutputRequest{TerminalId: id})
		_, errs["wait_for_exit"] = terms.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{TerminalId: id})
		_, errs["kill"] = terms.KillTerminal(ctx, acp.KillTerminalRequest{TerminalId: id})
		_, errs["release"] = terms.ReleaseTerminal(ctx, acp.ReleaseTerminalRequest{TerminalId: id})
		for method, err := range errs {
			if code(err) != -32002 {
				t.Errorf("terminal/%s of %s = %v, want resource-not-found (-32002)", method, id, err)
			}
		}
	}
}

// TestTerminalsClose requires Close to end every terminal's command before it
// returns, and to refuse terminal/create from then on.
func TestTerminalsClose(t *testing.T) {
	t.Parallel()
	terms := &Terminals{Command: testCommand}
	var running []*terminal
	for range 3 {
		id := create(t, terms, "sleep", nil)
		awaitReady(t, terms, id)
		term, err := terms.get(id)
		if err != nil {
			t.Fatal(err)
		}
		running = append(running, term)
	}
	if err := terms.Close(); err != nil {
		t.Fatalf("Close = %v", err)
	}
	for i, term := range running {
		if !term.exited() {
			t.Errorf("terminal %d's command runs after Close", i)
		}
	}
	if _, err := terms.CreateTerminal(t.Context(), acp.CreateTerminalRequest{SessionId: "s", Command: "write"}); err == nil {
		t.Error("CreateTerminal after Close = nil, want a refusal")
	}
	if err := terms.Close(); err != nil {
		t.Errorf("a second Close = %v", err)
	}
}

// TestTerminalCommandRefused requires CreateTerminal to refuse, starting
// nothing, a Command's error and a command it cannot run as a terminal's.
func TestTerminalCommandRefused(t *testing.T) {
	t.Parallel()
	why := errors.New("the policy refuses it")
	for name, build := range map[string]func(context.Context, acp.CreateTerminalRequest) (*exec.Cmd, error){
		"an error":   func(context.Context, acp.CreateTerminalRequest) (*exec.Cmd, error) { return nil, why },
		"no command": func(context.Context, acp.CreateTerminalRequest) (*exec.Cmd, error) { return nil, nil },
		"Stdout set": func(ctx context.Context, req acp.CreateTerminalRequest) (*exec.Cmd, error) {
			cmd, err := testCommand(ctx, req)
			cmd.Stdout = os.Stdout
			return cmd, err
		},
	} {
		terms := &Terminals{Command: build}
		if _, err := terms.CreateTerminal(t.Context(), acp.CreateTerminalRequest{SessionId: "s", Command: "write"}); err == nil ||
			(name == "an error" && !errors.Is(err, why)) {
			t.Errorf("%s: CreateTerminal = %v, want a refusal", name, err)
		}
		_ = terms.Close()
	}
}

// TestDirectCommand requires DirectCommand to run the request's command with
// its args, with this process's environment and then the request's env,
// whose entries win, and to refuse a cwd that is not absolute. It is not
// parallel: it sets this process's environment.
func TestDirectCommand(t *testing.T) {
	terms := &Terminals{}
	t.Cleanup(func() { _ = terms.Close() })
	t.Setenv(outEnv, "the process's")
	resp, err := terms.CreateTerminal(t.Context(), acp.CreateTerminalRequest{SessionId: "s", Command: testBinary(), Args: []string{"-test.run=^$"},
		Env: []acp.EnvVariable{{Name: childEnv, Value: "write"}, {Name: outEnv, Value: "the request's"}}})
	if err != nil {
		t.Fatal(err)
	}
	waitExit(t, terms, resp.TerminalId)
	out, err := terms.TerminalOutput(t.Context(), acp.TerminalOutputRequest{SessionId: "s", TerminalId: resp.TerminalId})
	if err != nil || out.Output != "the request's" {
		t.Errorf("TerminalOutput = %q, %v; want the request's env to win", out.Output, err)
	}
	relative := "work"
	if _, err := DirectCommand(t.Context(), acp.CreateTerminalRequest{Command: "x", Cwd: &relative}); code(err) != -32602 {
		t.Errorf("DirectCommand with a relative cwd = %v, want invalid params (-32602)", err)
	}
}

// hostClient is a client that serves the terminal methods with Terminals and
// refuses the other optional methods with Refusals, as the package doc shows.
type hostClient struct {
	Terminals
	Refusals
}

func (*hostClient) SessionUpdate(context.Context, acp.SessionNotification) error { return nil }

func (*hostClient) RequestPermission(context.Context, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{}, nil
}

// TestTerminalsOverConnection requires a client embedding Terminals and
// Refusals to serve the terminal methods over a connection, and to refuse
// another optional method with method-not-found.
func TestTerminalsOverConnection(t *testing.T) {
	t.Parallel()
	client := &hostClient{Terminals: Terminals{Command: testCommand}}
	t.Cleanup(func() { _ = client.Close() })
	pair := acptest.Connect(t, &acptest.ScriptAgent{}, client)
	ctx := t.Context()
	if _, err := pair.Client.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{Terminal: true, Fs: acp.FileSystemCapabilities{ReadTextFile: true}}}); err != nil {
		t.Fatal(err)
	}
	created, err := pair.Agent.CreateTerminal(ctx, acp.CreateTerminalRequest{SessionId: "s", Command: "write",
		Env: []acp.EnvVariable{{Name: outEnv, Value: "over the wire"}, {Name: exitEnv, Value: "4"}}})
	if err != nil {
		t.Fatalf("terminal/create = %v", err)
	}
	exit, err := pair.Agent.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{SessionId: "s", TerminalId: created.TerminalId})
	if err != nil || exit.ExitCode == nil || *exit.ExitCode != 4 {
		t.Errorf("terminal/wait_for_exit = %+v, %v; want exit code 4", exit, err)
	}
	out, err := pair.Agent.TerminalOutput(ctx, acp.TerminalOutputRequest{SessionId: "s", TerminalId: created.TerminalId})
	if err != nil || out.Output != "over the wire" {
		t.Errorf("terminal/output = %+v, %v", out, err)
	}
	if _, err := pair.Agent.ReleaseTerminal(ctx, acp.ReleaseTerminalRequest{SessionId: "s", TerminalId: created.TerminalId}); err != nil {
		t.Errorf("terminal/release = %v", err)
	}
	if _, err := pair.Agent.TerminalOutput(ctx, acp.TerminalOutputRequest{SessionId: "s", TerminalId: created.TerminalId}); code(err) != -32002 {
		t.Errorf("terminal/output after release = %v, want resource-not-found (-32002)", err)
	}
	if _, err := pair.Agent.ReadTextFile(ctx, acp.ReadTextFileRequest{SessionId: "s", Path: acptest.Abs("/a.txt")}); !acp.IsMethodNotFound(err) {
		t.Errorf("fs/read_text_file = %v, want method-not-found from Refusals", err)
	}
}

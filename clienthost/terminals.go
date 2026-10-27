package clienthost

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/internal/proctree"
)

// DefaultMaxOutputBytes is the most output a terminal keeps when
// Terminals.MaxOutputBytes is zero or less.
const DefaultMaxOutputBytes = 16 << 20

// exitDrain bounds how long a terminal waits, once its command has exited
// and its process tree has been ended, for the rest of the command's output:
// a process that left the tree can hold the output open.
const exitDrain = time.Second

// Terminals runs the commands of terminal/create and answers the other four
// terminal methods for them, as the ACP specification describes: a command
// runs in the background from its create, terminal/output returns what it
// has written so far, terminal/wait_for_exit its exit status, terminal/kill
// ends it and keeps the terminal, and terminal/release ends it and forgets
// the terminal. Each command runs in a process group of its own on Unix, and
// in a job object on Windows, so that kill and release end the processes it
// started too. When the command exits of itself, what is left of its group
// or job is ended then, as agentproc ends an agent's
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 2).
//
// A terminal keeps its command's output, standard output and standard error
// together, to the request's outputByteLimit and MaxOutputBytes, the smaller,
// dropping from the front at a character boundary, and reports truncated
// once it has dropped any. While the command runs, terminal/output holds back
// a character the command has not finished writing. A terminal id it does
// not know, or has released, is answered with acp.NewResourceNotFound.
//
// The zero Terminals is ready to use, and must not be copied after its first
// use. Its methods are safe for concurrent use. Close ends every terminal;
// a command is not otherwise ended when this process exits, except on
// Windows, where its job ends with this process.
type Terminals struct {
	// Command builds the command a terminal/create request runs. It is where
	// a client applies its own policy: the specification leaves checking the
	// command, its args, its env and its cwd to the client, which a
	// permission request from the agent does not replace, and recommends a
	// cwd inside the session's roots. An error refuses the request, and is
	// the agent's answer. The Cmd must not have been started, and its Stdout
	// and Stderr must be nil, since the terminal collects both; a nil Stdin
	// gives the command no input. It must not be bound to ctx, which ends
	// when terminal/create returns. Nil means DirectCommand.
	Command func(ctx context.Context, req acp.CreateTerminalRequest) (*exec.Cmd, error)

	// MaxOutputBytes bounds the output a terminal keeps, whatever the
	// request's outputByteLimit, which an agent chooses. Zero or less means
	// DefaultMaxOutputBytes.
	MaxOutputBytes int

	mu     sync.Mutex
	terms  map[acp.TerminalId]*terminal
	next   uint64
	closed bool
}

// DirectCommand is the command a nil Terminals.Command builds: req.Command
// run directly, without a shell, with req.Args, in req.Cwd when it is set,
// with this process's environment followed by req.Env, whose entries win. It
// refuses a cwd that is not absolute, since the schema requires one. It
// applies no other policy.
func DirectCommand(_ context.Context, req acp.CreateTerminalRequest) (*exec.Cmd, error) {
	cmd := exec.Command(req.Command, req.Args...) //nolint:gosec // G204: running the agent's command is the method; the caller's Command applies the policy.
	if req.Cwd != nil {
		if !filepath.IsAbs(*req.Cwd) {
			return nil, acp.NewInvalidParams(map[string]any{"cwd": "not an absolute path: " + *req.Cwd})
		}
		cmd.Dir = *req.Cwd
	}
	if len(req.Env) > 0 {
		cmd.Env = os.Environ()
		for _, e := range req.Env {
			cmd.Env = append(cmd.Env, e.Name+"="+e.Value)
		}
	}
	return cmd, nil
}

// CreateTerminal starts the request's command, as Command builds it, and
// returns its terminal's id at once.
func (t *Terminals) CreateTerminal(ctx context.Context, req acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	build := t.Command
	if build == nil {
		build = DirectCommand
	}
	cmd, err := build(ctx, req)
	switch {
	case err != nil:
		return acp.CreateTerminalResponse{}, err
	case cmd == nil:
		return acp.CreateTerminalResponse{}, errors.New("clienthost: Terminals.Command returned no command")
	case cmd.Process != nil:
		return acp.CreateTerminalResponse{}, errors.New("clienthost: Terminals.Command returned a command already started")
	case cmd.Stdout != nil || cmd.Stderr != nil:
		return acp.CreateTerminalResponse{}, errors.New("clienthost: Terminals.Command set Stdout or Stderr; the terminal collects both")
	}
	limit := t.MaxOutputBytes
	if limit <= 0 {
		limit = DefaultMaxOutputBytes
	}
	if n := req.OutputByteLimit; n != nil && *n < uint64(limit) {
		limit = int(*n) //nolint:gosec // G115: below limit, an int.
	}
	if err := proctree.Prepare(cmd); err != nil {
		return acp.CreateTerminalResponse{}, fmt.Errorf("clienthost: %w", err)
	}
	term, err := start(cmd, limit)
	if err != nil {
		return acp.CreateTerminalResponse{}, err
	}
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		term.release()
		return acp.CreateTerminalResponse{}, errors.New("clienthost: Terminals is closed")
	}
	t.next++
	id := acp.TerminalId(fmt.Sprintf("terminal-%d", t.next))
	if t.terms == nil {
		t.terms = make(map[acp.TerminalId]*terminal)
	}
	t.terms[id] = term
	t.mu.Unlock()
	return acp.CreateTerminalResponse{TerminalId: id}, nil
}

// TerminalOutput returns the terminal's output so far, whether any was
// dropped, and its exit status once its command has exited.
func (t *Terminals) TerminalOutput(_ context.Context, req acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	term, err := t.get(req.TerminalId)
	if err != nil {
		return acp.TerminalOutputResponse{}, err
	}
	exited := term.exited()
	var resp acp.TerminalOutputResponse
	resp.Output, resp.Truncated = term.out.snapshot(exited)
	if exited {
		status := term.status
		resp.ExitStatus = &status
	}
	return resp, nil
}

// WaitForTerminalExit waits for the terminal's command to exit and returns
// its exit status, or ctx's cause when ctx ends first.
func (t *Terminals) WaitForTerminalExit(ctx context.Context, req acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	term, err := t.get(req.TerminalId)
	if err != nil {
		return acp.WaitForTerminalExitResponse{}, err
	}
	select {
	case <-term.done:
		return acp.WaitForTerminalExitResponse{ExitCode: term.status.ExitCode, Signal: term.status.Signal}, nil
	case <-ctx.Done():
		return acp.WaitForTerminalExitResponse{}, context.Cause(ctx)
	}
}

// KillTerminal ends the terminal's command and the processes it started,
// and keeps the terminal, whose output and exit status stay readable. A
// command that has exited is sent nothing.
func (t *Terminals) KillTerminal(_ context.Context, req acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	term, err := t.get(req.TerminalId)
	if err != nil {
		return acp.KillTerminalResponse{}, err
	}
	if err := term.kill(); err != nil {
		return acp.KillTerminalResponse{}, fmt.Errorf("clienthost: killing the terminal's command: %w", err)
	}
	return acp.KillTerminalResponse{}, nil
}

// ReleaseTerminal ends the terminal's command, if it still runs, and the
// processes it started, and forgets the terminal: its id is not found from
// then on.
func (t *Terminals) ReleaseTerminal(_ context.Context, req acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	t.mu.Lock()
	term, ok := t.terms[req.TerminalId]
	delete(t.terms, req.TerminalId)
	t.mu.Unlock()
	if !ok {
		return acp.ReleaseTerminalResponse{}, acp.NewResourceNotFound(string(req.TerminalId))
	}
	term.release()
	return acp.ReleaseTerminalResponse{}, nil
}

// Close releases every terminal, and refuses terminal/create from then on.
// It returns once every terminal's command has ended. A second call does
// nothing.
func (t *Terminals) Close() error {
	t.mu.Lock()
	t.closed = true
	terms := t.terms
	t.terms = nil
	t.mu.Unlock()
	var wg sync.WaitGroup
	for _, term := range terms {
		wg.Go(term.release)
	}
	wg.Wait()
	return nil
}

// get returns the terminal of id, or acp.NewResourceNotFound.
func (t *Terminals) get(id acp.TerminalId) (*terminal, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if term, ok := t.terms[id]; ok {
		return term, nil
	}
	return nil, acp.NewResourceNotFound(string(id))
}

// terminal is one command, its process tree, and its output.
type terminal struct {
	cmd  *exec.Cmd
	tree *proctree.Tree
	pipe *os.File // the read end of the command's output
	out  *output

	mu    sync.Mutex
	ended bool // the command has exited: nothing is sent to its tree from then on

	read   chan struct{} // closed once the output has been read to its end, or its pipe closed
	done   chan struct{} // closed once the command has exited, after status is set
	status acp.TerminalExitStatus

	releaseOnce sync.Once
}

// start starts cmd, which proctree.Prepare has readied, with its output to a
// pipe the terminal reads, and attaches its process tree.
func start(cmd *exec.Cmd, limit int) (*terminal, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("clienthost: %w", err)
	}
	cmd.Stdout, cmd.Stderr = w, w
	if err := cmd.Start(); err != nil {
		return nil, errors.Join(fmt.Errorf("clienthost: starting the terminal's command: %w", err), r.Close(), w.Close())
	}
	// The command holds its own end now.
	werr := w.Close()
	tree, err := proctree.Attach(cmd)
	if err == nil {
		err = werr
	}
	if err != nil {
		_ = cmd.Process.Kill() //nolint:errcheck // it may have exited; Wait reaps it either way.
		_ = cmd.Wait()         //nolint:errcheck // its exit is not why the create failed.
		return nil, errors.Join(fmt.Errorf("clienthost: %w", err), r.Close())
	}
	term := &terminal{cmd: cmd, tree: tree, pipe: r, out: &output{limit: limit}, read: make(chan struct{}),
		done: make(chan struct{})}
	go term.readOutput()
	go term.wait()
	return term, nil
}

// readOutput reads the command's output into out until its end.
func (term *terminal) readOutput() {
	defer close(term.read)
	_, _ = io.Copy(term.out, term.pipe) //nolint:errcheck // the end of the output, or its pipe closed by release.
}

// wait reaps the command, ends what is left of its process tree, and waits
// up to exitDrain for the rest of its output before it records the exit.
func (term *terminal) wait() {
	err := term.cmd.Wait()
	term.mu.Lock()
	term.ended = true
	term.tree.End()
	term.mu.Unlock()
	t := time.NewTimer(exitDrain)
	defer t.Stop()
	select {
	case <-term.read:
	case <-t.C:
	}
	var ee *exec.ExitError
	if err == nil || errors.As(err, &ee) {
		term.status = exitStatus(term.cmd.ProcessState)
	}
	close(term.done)
}

// exited reports whether the command has exited and its status is set.
func (term *terminal) exited() bool {
	select {
	case <-term.done:
		return true
	default:
		return false
	}
}

// kill ends the command and its process tree, unless the command has
// exited.
func (term *terminal) kill() error {
	term.mu.Lock()
	defer term.mu.Unlock()
	if term.ended {
		return nil
	}
	return term.tree.Kill()
}

// release kills the command, waits for its exit, and closes its output's
// pipe, which ends a read held open by a process outside its tree.
func (term *terminal) release() {
	term.releaseOnce.Do(func() {
		_ = term.kill() //nolint:errcheck // the command ends either way: Wait below returns once it has.
		<-term.done
		_ = term.pipe.Close() //nolint:errcheck // the read end: nothing is lost.
		<-term.read
	})
}

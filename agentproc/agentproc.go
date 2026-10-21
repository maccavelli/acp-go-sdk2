// Package agentproc starts an ACP agent as a child process, connects to it
// over the child's standard input and output, and stops it, with the
// processes it started, in bounded steps
// (0027-MADR-agent-process-helper.md).
package agentproc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// defaultGrace is how long each step of Close waits for the agent when
// Command.Grace is zero or less.
const defaultGrace = 5 * time.Second

// ErrTerminated is joined to Wait's error when the agent exited after Close
// sent SIGTERM to its process group.
var ErrTerminated = errors.New("agentproc: agent terminated")

// ErrKilled is joined to Wait's error when Close killed the agent's process
// group or ended its job, or killed the agent itself where there is neither.
var ErrKilled = errors.New("agentproc: agent killed")

// Command is an agent to start as a child process.
type Command struct {
	// Cmd is the agent's command. It must not have been started, and its
	// Stdin and Stdout must be nil: Start connects them to pipes it owns.
	// Its Path, Args, Env, Dir and Stderr are the caller's; a nil Stderr
	// discards the agent's log, as os/exec does.
	Cmd *exec.Cmd

	// Grace is how long each step of Close waits for the agent to exit.
	// Zero or less means 5 seconds.
	Grace time.Duration

	// ConnectionOptions configure the connection to the agent.
	ConnectionOptions []acp.ConnectionOption
}

// Process is a running agent and the connection to it. Its methods are safe
// for concurrent use.
type Process struct {
	cmd   *exec.Cmd
	grace time.Duration
	conn  *acp.ClientSideConnection
	stdin *sharedClose // the write end of the agent's standard input
	tree  *tree        // the agent's process group or job, per platform

	// done is closed once Cmd.Wait has returned, which the watcher alone
	// calls (os/exec: "Wait must not be called concurrently").
	done chan struct{}

	mu      sync.Mutex
	exited  bool  // Cmd.Wait has returned: no signal is sent from then on
	waitErr error // what Cmd.Wait returned
	ended   error // ErrTerminated or ErrKilled, when Close ended the agent so

	// goroutines counts the watcher and the monitor, which Wait waits for.
	goroutines sync.WaitGroup

	// closeOnce runs Close's steps once, so concurrent callers share one
	// sequence of graces and send no signal twice. Each step is idempotent by
	// itself, so no test can tell its absence
	// (0027-PLAN-agent-process-helper.md, the deviation during step 2).
	closeOnce sync.Once
}

// Start starts the agent and connects client to it over the agent's
// standard input and output. On Unix the agent runs in a process group of
// its own, and on Windows in a job object, so that stopping it stops the
// processes it started; the job also ends them should this process die. When
// Cmd.WaitDelay is zero, Start sets it to half the grace, so that an agent
// that exits within it is seen to exit even while a process it started
// holds a Stderr writer that is not a file. Start refuses, starting
// nothing, a nil Cmd or client, a Cmd already started, or one whose Stdin or
// Stdout is set.
//
// The process and the connection are tied. When the agent exits, what is
// left of its process group or job is killed, and the connection is closed once it
// has read to the end of the agent's output, or after the grace. When the
// connection ends first, the agent is stopped as Close stops it.
func (c *Command) Start(client acp.Client) (*Process, error) {
	cmd := c.Cmd
	switch {
	case cmd == nil:
		return nil, errors.New("agentproc: Command.Cmd is nil")
	case client == nil:
		return nil, errors.New("agentproc: the client is nil")
	case cmd.Process != nil:
		return nil, errors.New("agentproc: Command.Cmd has already been started")
	case cmd.Stdin != nil:
		return nil, errors.New("agentproc: Command.Cmd.Stdin is set; Start connects it")
	case cmd.Stdout != nil:
		return nil, errors.New("agentproc: Command.Cmd.Stdout is set; Start connects it")
	}
	if err := prepare(cmd); err != nil {
		return nil, err
	}
	grace := c.Grace
	if grace <= 0 {
		grace = defaultGrace
	}
	// Pipes the helper owns, not StdoutPipe: Wait closes a StdoutPipe once
	// the agent exits, which can lose the agent's last output
	// (0027-MADR-agent-process-helper.md, P1, P2).
	inR, inW, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("agentproc: %w", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("agentproc: %w", err), inR.Close(), inW.Close())
	}
	cmd.Stdin, cmd.Stdout = inR, outW
	// Half the grace: an agent seen to exit only after WaitDelay is seen
	// within Close's first step, not reported as killed
	// (0027-PLAN-agent-process-helper.md, the deviation during step 4).
	if cmd.WaitDelay == 0 {
		cmd.WaitDelay = grace / 2
	}
	p := &Process{cmd: cmd, grace: grace, done: make(chan struct{})}
	started, proceed := make(chan error, 1), make(chan bool, 1)
	p.goroutines.Go(func() { p.run(started, proceed) })
	if err := <-started; err != nil {
		p.goroutines.Wait()
		return nil, errors.Join(fmt.Errorf("agentproc: starting the agent: %w", err), inR.Close(), inW.Close(),
			outR.Close(), outW.Close())
	}
	// The agent holds its own ends now; with the parent's closed, its output
	// ends when it and the processes it started have closed theirs.
	if err := errors.Join(inR.Close(), outW.Close()); err != nil {
		return nil, p.abandon(proceed, err, inW, outR)
	}
	t, err := attach(cmd)
	if err != nil {
		return nil, p.abandon(proceed, err, inW, outR)
	}
	p.stdin = &sharedClose{File: inW}
	p.tree = t
	p.conn = acp.NewClientSideConnection(client, p.stdin, outR, c.ConnectionOptions...)
	proceed <- true
	p.goroutines.Go(p.monitor)
	return p, nil
}

// run starts the agent and then waits for it, on one goroutine locked to
// its OS thread for the agent's life: on Linux the kernel sends a Pdeathsig
// when the thread that started the child ends, which can be long before the
// process ends (0029-MADR-unix-client-death-pdeathsig.md, D12 to D14). It
// reports Cmd.Start's error on started; then, as proceed says, it watches
// the agent, or kills it and reaps it.
func (p *Process) run(started chan<- error, proceed <-chan bool) {
	runtime.LockOSThread()
	// Unlocked only once Cmd.Wait has returned, so the thread is the agent's
	// until it is reaped. An earlier unlock would leave it free while run
	// waits for Start's word, which no test can catch
	// (0029-PLAN-unix-client-death-pdeathsig.md, the deviation during step 2).
	defer runtime.UnlockOSThread()
	if err := p.cmd.Start(); err != nil {
		started <- err
		return
	}
	started <- nil
	if !<-proceed {
		_ = p.cmd.Process.Kill() //nolint:errcheck // the agent may have exited; Wait reaps it either way.
		_ = p.cmd.Wait()         //nolint:errcheck // its exit is not why Start failed.
		return
	}
	p.watch()
}

// abandon has run kill and reap an agent Start has started but cannot
// connect, waits for it, closes the parent's ends of its pipes, and returns
// why.
func (p *Process) abandon(proceed chan<- bool, why error, files ...*os.File) error {
	proceed <- false
	p.goroutines.Wait()
	errs := []error{fmt.Errorf("agentproc: %w", why)}
	for _, f := range files {
		errs = append(errs, f.Close())
	}
	return errors.Join(errs...)
}

// sharedClose is the agent's input, which both the connection's Close and
// Close's first step close, at times at once when the connection's end
// stops the agent: it is closed once, and every caller gets that result.
type sharedClose struct {
	*os.File
	once sync.Once
	err  error
}

// Close closes the file the first time, and returns that result each time.
func (f *sharedClose) Close() error {
	f.once.Do(func() { f.err = f.File.Close() })
	return f.err
}

// watch owns Cmd.Wait, on run's locked thread. When it returns, watch kills what is left of the
// agent's process group or job, since a launcher's descendants outlive it
// (0027-MADR-agent-process-helper.md, P8), records the result, and closes
// done; then it gives the connection up to the grace to read to the end of
// the agent's output, and closes it.
func (p *Process) watch() {
	err := p.cmd.Wait()
	p.mu.Lock()
	p.tree.end()
	p.exited, p.waitErr = true, err
	p.mu.Unlock()
	close(p.done)
	t := time.NewTimer(p.grace)
	defer t.Stop()
	select {
	case <-p.conn.Done():
	case <-t.C:
	}
	_ = p.conn.Close() //nolint:errcheck // Close of a connection reports nothing Wait does not.
}

// monitor stops the agent when the connection ends while the agent runs: an
// agent without its connection cannot be reached.
func (p *Process) monitor() {
	select {
	case <-p.done:
	case <-p.conn.Done():
		p.closeOnce.Do(p.stop)
	}
}

// Conn returns the connection to the agent.
func (p *Process) Conn() *acp.ClientSideConnection { return p.conn }

// Done returns a channel that is closed once the agent process has exited
// and what was left of its process group or job has been killed.
func (p *Process) Done() <-chan struct{} { return p.done }

// Wait blocks until the agent process has exited and the connection's
// goroutines have returned, and then returns the agent's error, the same
// each time: nil for an exit of status 0 of its own, otherwise the error
// exec.Cmd.Wait returned, joined with ErrTerminated or ErrKilled when Close
// ended the agent so. It does not stop the agent.
func (p *Process) Wait() error {
	<-p.done
	p.goroutines.Wait()
	_ = p.conn.Wait() //nolint:errcheck // the connection's end is not the agent's; Conn().Err() reports it.
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case p.ended == nil:
		return p.waitErr
	case p.waitErr == nil:
		return p.ended
	default:
		return fmt.Errorf("%w: %w", p.ended, p.waitErr)
	}
}

// Close stops the agent and returns Wait's error. It closes the agent's
// standard input and waits up to the grace for the agent to exit. On Unix it
// then sends SIGTERM to the agent's process group and waits up to the grace
// again. It then kills the group, or on Windows ends the job, or kills the
// agent where there is neither, and closes the connection. A second call returns the same error.
func (p *Process) Close() error {
	p.closeOnce.Do(p.stop)
	return p.Wait()
}

// stop runs Close's steps once.
func (p *Process) stop() {
	// The agent's input only: the connection keeps reading what the agent
	// writes before it exits (0027-MADR-agent-process-helper.md, A6).
	_ = p.stdin.Close() //nolint:errcheck // a failed close leaves the agent to the next steps.
	switch {
	case p.exitedWithin(p.grace):
	case p.signal(p.tree.terminate, ErrTerminated) && p.exitedWithin(p.grace):
	default:
		p.signal(p.tree.kill, ErrKilled)
		<-p.done // WaitDelay bounds it once the agent has exited
	}
	_ = p.conn.Close() //nolint:errcheck // Close of a connection reports nothing Wait does not.
}

// exitedWithin reports whether the agent has exited within d.
func (p *Process) exitedWithin(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-p.done:
		return true
	case <-t.C:
		return false
	}
}

// signal sends what send sends, unless the agent has exited, and records
// why the agent ended. It reports whether it sent anything.
func (p *Process) signal(send func() error, why error) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.exited {
		return false
	}
	if err := send(); err != nil {
		return false
	}
	p.ended = why
	return true
}

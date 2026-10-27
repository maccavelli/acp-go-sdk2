// Package agentproc starts an ACP agent as a child process, connects to it
// over the child's standard input and output, and stops it, with the
// processes it started, in bounded steps
// (0027-MADR-agent-process-helper.md).
package agentproc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/internal/proctree"
)

// defaultGrace is how long each step of Close waits for the agent when
// Command.Grace is zero or less.
const defaultGrace = 5 * time.Second

// ErrTerminated is joined to Wait's error when the agent exited after Close
// sent SIGTERM to its process group.
var ErrTerminated = errors.New("agentproc: agent terminated")

// ErrKilled is joined to Wait's error when Close or Kill killed the agent's
// process group or ended its job, or killed the agent itself where there is
// neither.
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

	// OutputTap, when set, receives a copy of every byte the agent writes to
	// its standard output, in order, as the connection reads it and before it
	// is decoded: a line too long or not JSON included, through the end of
	// the output. A goroutine of the tap's own calls Write, so the connection
	// never waits for it, and Write's error is ignored. Should the tap fall
	// 32 MiB behind, it is detached: what it received is the stream's
	// beginning, and DroppedTapBytes counts the rest. Wait waits up to the
	// grace for the tap to write what it was given. Write must return, and
	// must not retain p (0030-MADR-record-initialize-on-arrival.md, the
	// amendment of 2026-10-21).
	OutputTap io.Writer

	// InputTap, when set, receives a copy of what the connection writes to
	// the agent's standard input, as OutputTap does of its output. A writer
	// set as both taps is called from two goroutines, and must be safe for
	// concurrent use; the two streams have no order between them.
	InputTap io.Writer

	// readHook, when set with OutputTap, is called before each read of the
	// agent's output. Only this package's tests set it, to hold the
	// connection's reader back behind the agent
	// (0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phase 3).
	readHook func()
}

// Process is a running agent and the connection to it. Its methods are safe
// for concurrent use.
type Process struct {
	cmd   *exec.Cmd
	grace time.Duration
	conn  *acp.ClientSideConnection
	stdin *sharedClose   // the write end of the agent's standard input
	tree  *proctree.Tree // the agent's process group or job, per platform

	// done is closed once Cmd.Wait has returned, which the watcher alone
	// calls (os/exec: "Wait must not be called concurrently").
	done chan struct{}

	mu          sync.Mutex
	exited      bool  // Cmd.Wait has returned: no signal is sent from then on
	waitErr     error // what Cmd.Wait returned
	ended       error // ErrTerminated or ErrKilled, when Close ended the agent so
	exitedFirst bool  // the connection was still open when the agent's exit was seen
	returned    bool  // Wait has returned once

	// goroutines counts the watcher and the monitor, which Wait waits for.
	goroutines sync.WaitGroup

	// closeOnce runs Close's steps once, so concurrent callers share one
	// sequence of graces and send no signal twice. Each step is idempotent by
	// itself, so no test can tell its absence
	// (0027-PLAN-agent-process-helper.md, the deviation during step 2).
	closeOnce sync.Once

	// hurry is closed, once (hurryOnce), by Kill: Close's steps then wait no
	// more, and send no SIGTERM, but kill.
	hurry     chan struct{}
	hurryOnce sync.Once

	// outTap and inTap are the taps of the agent's output and input, nil
	// when unset; their goroutines are not in goroutines, and Wait drains
	// them once, up to the grace (drainOnce).
	outTap, inTap *tap
	drainOnce     sync.Once

	// waitOnce starts, once, the goroutine WaitContext waits through, which
	// runs Wait and then closes waited.
	waitOnce sync.Once
	waited   chan struct{}
}

// Start starts the agent and connects client to it over the agent's
// standard input and output. On Unix the agent runs in a process group of
// its own, and on Windows in a job object, so that stopping it stops the
// processes it started.
//
// Should this process die without Close, the agent's input ends, which ends
// an agent that exits at its input's end. Beyond that, on Windows the job
// ends the agent and every process it started; on Linux and FreeBSD the
// kernel sends the agent the SIGKILL that Start sets as its Pdeathsig, unless
// the caller set one, and a process it started ends only at its input's end;
// elsewhere nothing more ends them. On Windows, a process the agent's tree
// starts with CREATE_BREAKAWAY_FROM_JOB is outside the job, as one that
// starts a session of its own is outside the process group on Unix: what
// ends the job or the group does not end it, and it is the agent's to end.
// Start starts the agent from a goroutine
// locked to its OS thread until the agent is reaped, since Linux sends a
// Pdeathsig when the thread that started the child ends
// (0029-MADR-unix-client-death-pdeathsig.md). When
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
	p := &Process{cmd: cmd, grace: grace, done: make(chan struct{}), hurry: make(chan struct{})}
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
	t, err := proctree.Attach(cmd)
	if err != nil {
		return nil, p.abandon(proceed, err, inW, outR)
	}
	p.stdin = &sharedClose{File: inW}
	p.tree = t
	in, out, inTap, outTap := streams(p.stdin, outR, c.InputTap, c.OutputTap, c.readHook)
	p.inTap, p.outTap = inTap, outTap
	p.conn = acp.NewClientSideConnection(client, in, out, c.ConnectionOptions...)
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
// the agent's output, or until Kill, and closes it. It alone closes the
// connection once the agent has run, whether or not Close was called
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 1).
func (p *Process) watch() {
	err := p.cmd.Wait()
	p.mu.Lock()
	// Seen before the tree's end, which can end what a process the agent
	// started holds of its output.
	select {
	case <-p.conn.Done():
	default:
		p.exitedFirst = true
	}
	p.tree.End()
	p.exited, p.waitErr = true, err
	p.mu.Unlock()
	close(p.done)
	t := time.NewTimer(p.grace)
	defer t.Stop()
	select {
	case <-p.conn.Done():
	case <-p.hurry:
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
// goroutines have returned, and the taps have written what they were given,
// or the grace has passed, and then returns the agent's error, the same each
// time: nil for an exit of status 0 of its own, otherwise the error
// exec.Cmd.Wait returned, joined with ErrTerminated or ErrKilled when Close
// or Kill ended the agent so. It does not stop the agent.
func (p *Process) Wait() error {
	<-p.done
	p.goroutines.Wait()
	_ = p.conn.Wait() //nolint:errcheck // the connection's end is not the agent's; Conn().Err() reports it.
	p.drainOnce.Do(p.drainTaps)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.returned = true
	switch {
	case p.ended == nil:
		return p.waitErr
	case p.waitErr == nil:
		return p.ended
	default:
		return fmt.Errorf("%w: %w", p.ended, p.waitErr)
	}
}

// WaitContext is Wait bounded by ctx: it returns Wait's error once Wait
// would, or context.Cause(ctx) when ctx ends first, leaving the agent as it
// was; it does not stop the agent. The first call starts one goroutine, which
// runs Wait and ends when Wait returns; every later call shares it, so a
// caller may wait again (0041-MADR-close-the-gaps-the-consumers-wait-on.md,
// item 6).
func (p *Process) WaitContext(ctx context.Context) error {
	p.waitOnce.Do(func() {
		p.waited = make(chan struct{})
		go func() {
			_ = p.Wait() //nolint:errcheck // WaitContext calls Wait again, which then returns at once.
			close(p.waited)
		}()
	})
	select {
	case <-p.waited:
		return p.Wait()
	case <-ctx.Done():
		select {
		case <-p.waited:
			return p.Wait()
		default:
			return context.Cause(ctx)
		}
	}
}

// ExitedFirst reports whether the agent's exit was seen while the connection
// was still open: the agent exited, of itself or stopped by Close or Kill,
// before the connection ended. It is false when the connection ended first,
// as when the agent closed its output, or the client the connection, while
// the agent ran; then the agent was stopped as Close stops it, and its exit
// status says how it took that. An agent's output ends as it exits, unless a
// process it started holds it, so the two ends can come together, and either
// can be seen first. ExitedFirst is false until Wait has returned, and final
// from then on (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 7).
func (p *Process) ExitedFirst() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.returned && p.exitedFirst
}

// drainTaps ends the taps' streams and waits, from one timer of the grace,
// for each tap to write what it was given; a tap still writing when the grace
// has passed is abandoned, and what it has not written is counted as dropped
// (0030-MADR-record-initialize-on-arrival.md, the amendment of 2026-10-21,
// question 11).
func (p *Process) drainTaps() {
	t := time.NewTimer(p.grace)
	defer t.Stop()
	expired := false
	for _, tp := range []*tap{p.outTap, p.inTap} {
		if tp == nil {
			continue
		}
		tp.end()
		if !expired {
			select {
			case <-tp.done:
				continue
			case <-t.C:
				expired = true
			}
		}
		tp.abandon()
	}
}

// DroppedTapBytes reports how many bytes of the agent's output and of its
// input OutputTap and InputTap did not receive: zero for a complete copy, or
// a tap unset. It is final once Wait has returned. A tap that falls 32 MiB
// behind, or is still writing when Wait's grace has passed, receives the
// stream's beginning, and the rest is counted here.
func (p *Process) DroppedTapBytes() (output, input uint64) {
	if p.outTap != nil {
		output = p.outTap.droppedBytes()
	}
	if p.inTap != nil {
		input = p.inTap.droppedBytes()
	}
	return output, input
}

// Close stops the agent and returns Wait's error. It closes the agent's
// standard input and waits up to the grace for the agent to exit. On Unix it
// then sends SIGTERM to the agent's process group and waits up to the grace
// again. It then kills the group, or on Windows ends the job, or kills the
// agent where there is neither. Once the agent has exited, the connection
// reads to the end of the agent's output, up to the grace from the exit, and
// is closed. A second call returns the same error. Kill cuts a Close in
// progress short.
func (p *Process) Close() error {
	p.closeOnce.Do(p.stop)
	return p.Wait()
}

// Kill stops the agent at once and returns Wait's error. It runs Close's
// steps without their waits: it closes the agent's standard input, kills the
// agent's process group, or on Windows ends its job, or kills the agent where
// there is neither, so that the processes the agent started end with it, and
// closes the connection; Wait's error then has ErrKilled joined. An agent that
// has already exited is sent nothing. A Close in progress stops waiting and
// kills, and Close and Kill, called during or after it, return the same error
// (0030-MADR-record-initialize-on-arrival.md, the amendment of 2026-10-21,
// question 7).
func (p *Process) Kill() error {
	p.hurryOnce.Do(func() { close(p.hurry) })
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
	case !p.hurried() && p.signal(p.tree.Terminate, ErrTerminated) && p.exitedWithin(p.grace):
	default:
		p.signal(p.tree.Kill, ErrKilled)
		<-p.done // WaitDelay bounds it once the agent has exited
	}
	// The watcher closes the connection: once it has read to the end of the
	// agent's output, up to the grace from the agent's exit, or at once after
	// Kill. Closing it here would lose what the agent wrote last and the
	// connection had not yet read
	// (0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phase 2).
}

// exitedWithin reports whether the agent has exited within d, and false at
// once when Kill hurries Close's steps.
func (p *Process) exitedWithin(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-p.done:
		return true
	case <-p.hurry:
		return false
	case <-t.C:
		return false
	}
}

// hurried reports whether Kill has been called.
func (p *Process) hurried() bool {
	select {
	case <-p.hurry:
		return true
	default:
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

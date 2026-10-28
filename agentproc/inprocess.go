package agentproc

import (
	"context"
	"errors"
	"io"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// ErrAbandoned is joined to Wait's error, with ErrKilled, when an InProcess
// agent's Serve has not returned within the grace after its context was
// cancelled and its pipes closed. Its goroutine is left running, since Go
// cannot stop a goroutine; the agent's code holds it
// (0042-MADR-in-process-agent-runner.md, item 1, A4).
var ErrAbandoned = errors.New("agentproc: in-process agent abandoned")

// InProcess is an agent written in Go to run in this process: its Serve runs
// on a goroutine, joined to the client by two pipes, and Start returns the
// same Process a Command's child gets, with its lifecycle mapped onto a
// goroutine (0042-MADR-in-process-agent-runner.md, item 1):
//
//   - when Serve returns, its output and input are closed, so that the client
//     reads the end of the output, as at a child's exit, and a later write of
//     the client's fails rather than waits; Serve's error is Wait's;
//   - Close's first step closes the agent's input, so that Serve's reads
//     return io.EOF, and waits up to the grace for Serve to return;
//   - its second cancels Serve's context with the cause ErrTerminated, and
//     waits up to the grace again;
//   - its third cancels the context with ErrKilled and closes the agent's ends
//     of both pipes, so that its blocked reads and writes return
//     io.ErrClosedPipe; a Serve that still has not returned within the grace
//     is abandoned, and Wait's error has ErrKilled and ErrAbandoned joined;
//   - Kill runs the steps without their waits, the abandonment's grace
//     excepted.
//
// A panic in Serve is not recovered: it ends the program, as a panic on any
// goroutine does. A client that needs to survive its agent's faults runs the
// agent as a child, through Command. An in-process agent has no process
// group, job, death signal or standard error; it ends with this process.
type InProcess struct {
	// Serve runs the agent over in and out until in ends, as a program's main
	// serves os.Stdin and os.Stdout, and returns why it stopped. ctx is the
	// runner's own, derived from no caller's context, and is cancelled by
	// Close's second step with ErrTerminated or by its third, or Kill, with
	// ErrKilled, which context.Cause reports.
	Serve func(ctx context.Context, in io.Reader, out io.Writer) error

	// Grace is how long each step of Close waits for Serve to return. Zero or
	// less means 5 seconds.
	Grace time.Duration

	// ConnectionOptions configure the connection to the agent.
	ConnectionOptions []acp.ConnectionOption

	// OutputTap and InputTap are as Command's: a copy of what the agent writes,
	// and of what the connection writes to it.
	OutputTap io.Writer
	InputTap  io.Writer

	// readHook is as Command's: only this package's tests set it.
	readHook func()
}

// Start runs Serve on a goroutine of its own and connects client to it. It
// refuses, starting nothing, a nil Serve or client.
func (a *InProcess) Start(client acp.Client) (*Process, error) {
	switch {
	case a.Serve == nil:
		return nil, errors.New("agentproc: InProcess.Serve is nil")
	case client == nil:
		return nil, errors.New("agentproc: the client is nil")
	}
	grace := a.Grace
	if grace <= 0 {
		grace = defaultGrace
	}
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancelCause(context.Background())
	ag := &inProcessAgent{
		cancel: cancel, inR: inR, inW: inW, outW: outW, grace: grace,
		returned: make(chan struct{}), killed: make(chan struct{}),
	}
	p := &Process{agent: ag, grace: grace, done: make(chan struct{}), hurry: make(chan struct{})}
	in, out, inTap, outTap := streams(inW, outR, a.InputTap, a.OutputTap, a.readHook)
	p.inTap, p.outTap = inTap, outTap
	p.conn = acp.NewClientSideConnection(client, in, out, a.ConnectionOptions...)
	// Not in p.goroutines: Wait would wait for an abandoned Serve for ever.
	go ag.run(ctx, a.Serve)
	p.goroutines.Go(p.watch)
	p.goroutines.Go(p.monitor)
	return p, nil
}

// inProcessAgent is an InProcess's agent: Serve's goroutine and its ends of
// the two pipes.
type inProcessAgent struct {
	cancel context.CancelCauseFunc // ends Serve's context
	inR    *io.PipeReader          // the agent's input, which Serve reads
	inW    *io.PipeWriter          // the connection's end of it
	outW   *io.PipeWriter          // the agent's output, which Serve writes
	grace  time.Duration

	returned chan struct{} // closed when Serve has returned, err set
	err      error         // what Serve returned
	killed   chan struct{} // closed by kill, which starts the abandonment's grace
}

// run runs serve and records what it returned.
func (g *inProcessAgent) run(ctx context.Context, serve func(context.Context, io.Reader, io.Writer) error) {
	g.err = serve(ctx, g.inR, g.outW)
	close(g.returned)
}

// closeInput ends the agent's input: Serve's reads return io.EOF. A pipe's
// writer closes any number of times.
func (g *inProcessAgent) closeInput() error { return g.inW.Close() }

// terminate cancels Serve's context with ErrTerminated.
func (g *inProcessAgent) terminate() error {
	g.cancel(ErrTerminated)
	return nil
}

// kill cancels Serve's context with ErrKilled, and closes the agent's ends of
// both pipes: its blocked reads and writes return io.ErrClosedPipe, the
// client's writes fail with ErrKilled, and the client reads the end of the
// output (0042-MADR-in-process-agent-runner.md, "Amended by the PLAN's phase
// 2"). Process.signal calls it at most once.
func (g *inProcessAgent) kill() error {
	g.cancel(ErrKilled)
	_ = g.inR.CloseWithError(ErrKilled)
	_ = g.outW.Close() //nolint:errcheck // Close returns nil.
	close(g.killed)
	return nil
}

// wait blocks until Serve has returned, and returns its error; after kill, it
// waits up to the grace more, and returns ErrAbandoned if Serve has not
// returned by then.
func (g *inProcessAgent) wait() error {
	select {
	case <-g.returned:
		return g.err
	case <-g.killed:
	}
	t := time.NewTimer(g.grace)
	defer t.Stop()
	select {
	case <-g.returned:
		return g.err
	case <-t.C:
		return ErrAbandoned
	}
}

// end closes the agent's output, so that the client reads its end as at a
// child's exit, and its input, so that a write of the client's fails rather
// than waits for a reader that will not come; and it ends Serve's context.
// After an abandonment the pipes are closed already.
func (g *inProcessAgent) end() {
	_ = g.outW.Close() //nolint:errcheck // Close returns nil.
	_ = g.inR.Close()  //nolint:errcheck // Close returns nil.
	g.cancel(context.Canceled)
}

package acp

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// ServeAgent serves agent over in and out, the agent's input and output,
// until in ends or ctx ends, as a program's main serves os.Stdin and
// os.Stdout, and returns why it stopped. It builds the side connection with
// opts, and waits on the caller's goroutine:
//
//   - at in's end, the connection lets the requests it received be answered,
//     as it always does, and ends; ServeAgent then waits up to 5 s for the
//     connection's handlers, and returns nil when the connection ended
//     cleanly, as EndedCleanly says, its error otherwise, or, when the 5 s
//     pass with handlers still running, an error wrapping ErrHandlersRunning
//     that says so;
//   - at ctx's end, it shuts the connection down with Shutdown, within the
//     same 5 s: every handler's context ends with ErrShutdown, and a handler
//     that honours it is answered, an agent's prompt cancelled, before the
//     connection closes. It returns context.Cause(ctx), joined with
//     Shutdown's error, which wraps ErrHandlersRunning too when the 5 s pass
//     with requests unanswered, their handlers still running.
//
// The agent's own cleanup is the caller's, once ServeAgent has returned. Its
// bound is the one the connection gives its input's drain
// (0042-MADR-in-process-agent-runner.md, item 2, and the amendment of
// 2026-10-30, X2).
func ServeAgent(ctx context.Context, agent Agent, in io.Reader, out io.Writer, opts ...ConnectionOption) error {
	c := NewAgentSideConnection(agent, out, in, opts...).Connection()
	select {
	case <-c.Done():
		bound, cancel := context.WithTimeout(context.Background(), notificationQueueDrainTimeout)
		defer cancel()
		err := c.WaitContext(bound)
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return fmt.Errorf("%w %v after its end", ErrHandlersRunning, notificationQueueDrainTimeout)
		case EndedCleanly(err):
			return nil
		default:
			return err
		}
	case <-ctx.Done():
		bound, cancel := context.WithTimeout(context.Background(), notificationQueueDrainTimeout)
		defer cancel()
		err := c.Shutdown(bound)
		// Shutdown's own error, not the joined one: a caller's context whose
		// cause is context.DeadlineExceeded is not the bound.
		if errors.Is(err, context.DeadlineExceeded) {
			err = fmt.Errorf("%w %v after the context's end: %w", ErrHandlersRunning, notificationQueueDrainTimeout, err)
		}
		return errors.Join(context.Cause(ctx), err)
	}
}

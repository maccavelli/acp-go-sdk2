---
status: accepted
date: 2026-10-20
decision-makers: Repository owner
---

# Report why a connection ended, and wait for its goroutines to exit

## Context and Problem Statement

[0023-MADR-connection-lifetime-and-test-leaks.md](0023-MADR-connection-lifetime-and-test-leaks.md) left two accessors undecided,
"deliberately", each "a record of its own":

* "`Err()`, an accessor for why the connection ended. … It would add exported API, and no consumer has asked for it."
* "`Wait()`, which returns once the connection's goroutines have exited, as the MCP Go SDK's `Wait` does. `Close` cannot promise that
  for a reader it cannot close."

On 2026-10-20 the owner asked for both to be scoped and specified. This record holds what the code does today, what the two known
consumers need, the prior art, and the questions to decide. Every figure comes from a command or a probe run that day on `0afe75e`;
the probes ran in a scratch copy of the tree.

### What a connection's end looks like today

Its lifetime is one context, `c.ctx`, ended with a cause (`connection.go:630`). `Done` returns its channel (`:2162`). An unexported
`endError` (`:712`) turns the cause into the error a call meets after the end: `ErrConnectionClosed` when this side closed it, wrapping
the cause; otherwise `ErrPeerDisconnected`, wrapping it.

The causes, by how the connection ends:

* **`Close`:** `ErrConnectionClosed` (`:733`).
* **Notification overflow,** under the default `OverflowCloseConnection`: `closeWith(errNotificationQueueOverflow)` (`:1357`), an
  unexported error, "notification queue overflow" (`:32`).
* **The peer's end of input,** at EOF: after the drain of B23, `shutdownReceive` ends the context with `cause :=
  errors.New("peer connection closed")` (`:1084`), a new error that does not wrap `io.EOF`.
* **Any other read error:** that error.

A probe, one connection per case, read `endError()` and `errors.Is` against the sentinels and `io.EOF`:

| Id | Case | `endError()` | Found |
| :--- | :--- | :--- | :--- |
| E1 | before the end | "acp: peer disconnected", `Is(ErrPeerDisconnected)` | `endError` is meaningful only after `Done`; an accessor must return nil until then |
| E2 | `Close` | "acp: connection closed" | as documented |
| E3 | the peer's EOF | "acp: peer disconnected: peer connection closed", `Is(io.EOF)` false | an EOF cannot be told from another cause |
| E4 | the peer's EOF, then `Close` | "acp: connection closed: peer connection closed" | **the answer changes after the end**: `Close` marks the connection closed locally even when the peer had already ended it (`:743`–`:744`) |
| E5 | a read error, "read: connection reset" | "acp: peer disconnected: read: connection reset" | the read error is wrapped, so `errors.Is` and `errors.As` reach it |
| E6 | notification overflow | "acp: connection closed: notification queue overflow" | only the unexported `errNotificationQueueOverflow` identifies it |

### Which goroutines a connection runs

`start` launches four, which live as long as the connection: `writeLoop`, `sendCancelRequests`, `receive` and
`processNotifications` (`:672`–`:675`). Others are started as the connection works:

* one per inbound request, running the handler (`:1304`);
* for a prompt turn answered cancelled, the handler's call and a goroutine that waits for its late result (`:244`, `:256`);
* the end-of-input drain's waiter (`:1118`) and the notification drain after the input ends (`:1486`);
* `context.AfterFunc` callbacks that wake notification waits (`:671`, `:1949`, `:1993`).

A probe counted the connection's goroutines by the function each is blocked in:

| Id | Case | Goroutines left |
| :--- | :--- | :--- |
| G1 | running, an `io.Pipe` reader | `receive`, `processNotifications`, `writeLoop`, `sendCancelRequests`: 1 each |
| G2 | 300 ms after `Close`, an `io.Pipe` reader | none: `Close` closes the reader, so `receive` returns, and `processNotifications` with it |
| G3 | 300 ms after `Close`, a reader with no `Close` that blocks | `receive` and `processNotifications`; both return once its `Read` returns |
| G4 | 300 ms after `Close`, a request whose handler ignores its context | that handler's goroutine (`handleInbound`); it returns when the handler does |

`processNotifications` ends when `shutdownReceive` closes the queue (`:1474`), which only `receive` does, so a stuck reader holds both
(G3). The leak guard of 0023 treats goroutines left this way as leaks.

### What the consumers do without them

* **`magic-cli-remote`,** `internal/provider/acpagent/session.go`: `watchConnClose` waits on `conn.Done()` and reports a fixed
  message, `fmt.Sprintf("%s agent connection lost", s.providerID)`. Its comment explains the gap: an inbound-queue overflow "tears the connection down WITHOUT killing the agent
  process", and "conn.Done() also fires on every normal teardown", so a latch in `signalDisconnected` stays silent then. It cannot say
  why the connection ended.
* **The second consumer,** a CLI named to the owner (this repository's identifier scan keeps its name out), `acpserver/serve.go`: on
  `conn.Done()` it returns `agent.Close()`, so an overflow or a read error ends its serve loop as cleanly as the peer's EOF. Its test
  helper `acpclient/acptest/pair.go` waits on both side connections' `Done` under a deadline; after `Done`, a handler may still run
  (G4).

### Prior art

| Library | Why it ended | Waiting for its goroutines |
| :--- | :--- | :--- |
| `golang.org/x/tools/internal/jsonrpc2` | `Err() error`: "Err returns an error if there was one from within the processing goroutine. If err returns non nil, the connection will be already closed or closing." | `Done()`: closed "when the processing goroutine has terminated" |
| `golang.org/x/tools/internal/jsonrpc2_v2` | the close error, returned by `Wait` | `Wait() error`: "Wait blocks until the connection is fully closed, but does not close it." It returns once in-flight requests are idle and the stream's closer has run |
| MCP Go SDK, `modelcontextprotocol/go-sdk` | returned by `Wait` | `ClientSession.Wait() error` and `ServerSession.Wait() error`, which return `conn.Wait()`: "Wait waits for the connection to be closed by the server" (by the client, for a server session) |
| ACP TypeScript SDK, `v1.9.0` | `signal`, an `AbortSignal` whose `reason` is the cause (`updates.fail(closeSignal.reason ?? new Error("ACP connection closed"))`) | `closed`, a promise that resolves on close |
| `coder/acp-go-sdk` | none | `Done()` only: closed "when the underlying reader loop exits" |
| the standard library | `context.Context.Err` with `context.Cause` | `sync.WaitGroup.Wait`, `exec.Cmd.Wait`; `http.Server.Shutdown(ctx)` bounds a wait with a context |

`sync.WaitGroup.Go`, which runs a function on a new goroutine and counts it, exists at the module's floor: `go doc sync.WaitGroup.Go` under
go1.26.9 prints it.

## Decision Drivers

* **Within `v1`, additive only** ([0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md)),
  and `make api` must show no incompatible change.
* **Go idiom and the prior art:** `Err` beside `Done`, as `context` and `jsonrpc2` pair them; `Wait` returning the end's error, as
  `jsonrpc2_v2` and the MCP Go SDK do.
* **A stable answer:** why a connection ended must not change after it ended (E4).
* **Causes a caller can test** with `errors.Is`, without matching text (E3, E6).
* **The reader belongs to the application.** `Close` closes it only when it implements `io.Closer` (0023), so no accessor can promise a
  bounded wait for a reader it cannot close (G3).
* **The side connections** expose `Done` and `Close` (`agent_gen.go:65`, `:71`; `client_gen.go:59`, `:65`), so callers hold them, not
  the `*Connection`.
* **No new dependency.**

## Considered Options

* **O1.** Both accessors, `Err` and `Wait`, on `Connection` and both side connections, with causes made testable and the end fixed at
  the first.
* **O2.** `Err` only; `Wait` stays undecided.
* **O3.** Neither: document that a call's error carries the cause, as 0023 left it.

Each of O1's parts is a question below, with a recommended answer.

## Decision Outcome

Chosen option: "O1", with the recommended answer to each of questions 1 to 7, decided by the owner on 2026-10-20 (see "Decided
with the owner, 2026-10-20"), because it closes both gaps the consumers show with additive API in the prior art's shapes.

* It closes both gaps the consumers show, the reason (`magic-cli-remote`, the second consumer's serve loop) and the full stop (the
  second consumer's test helper), with exported API only added.
* O2 leaves G3 and G4 to each caller, who cannot see the SDK's goroutines.
* O3 leaves E3, E4 and E6 as they are: a caller that never made a call has no error to read.

### Specification, as recommended

**`Err`.**

```go
// Err returns nil until the connection ends, and then why it ended, the
// same each time: ErrConnectionClosed when this side ended it, by Close or
// by a notification overflow, wrapping ErrNotificationQueueOverflow for the
// latter; otherwise ErrPeerDisconnected, wrapping how the peer ended it,
// io.EOF at the end of its output or the read's error.
func (c *Connection) Err() error
```

* **Fixed at the first end.** The cause is recorded once, by whichever of `closeWith` and `shutdownReceive` ends the context first, and
  `Err` returns it from then on (E4). It is safe for concurrent use, and non-nil exactly when `Done` is closed.
* **A call's error is unchanged.** A send or a call after a late `Close` still returns `ErrConnectionClosed` (question 7) (amended 2026-10-20, see "Amended during the PLAN's step 6").

**Testable causes.**

* **`ErrNotificationQueueOverflow`,** exported, the same value and text as today's `errNotificationQueueOverflow`, "notification queue
  overflow", so no message changes (E6).
* **The peer's EOF wraps `io.EOF`,** with its text unchanged, "peer connection closed": an error type whose `Error` returns that text
  and whose `Unwrap` returns `io.EOF` (E3).

**`Wait`.**

```go
// Wait blocks until the connection has ended and every goroutine it
// started has returned, its handlers' included, and then returns Err. It
// does not end the connection: call Close first to stop it. A reader that
// Close cannot close, one that does not implement io.Closer, keeps Wait
// waiting until its Read returns; so does a handler that ignores its
// context.
func (c *Connection) Wait() error
```

* **What it counts:** every goroutine the connection starts, through one `sync.WaitGroup`'s `Go`. That includes the four of `start`,
  the handlers, `answerUnlessCancelled`'s two, the drains' and the `context.AfterFunc` callbacks. A function counted that way must not
  panic, which matches the documented rule that a panicking handler ends the process (0024).

  Corrected 2026-10-20, while drafting the PLAN: the `context.AfterFunc` callbacks are not counted. `sync.WaitGroup`'s doc: "calls
  with a positive delta that occur when the counter is zero must happen before a Wait". The callbacks at `connection.go:1949` and
  `:1993` run on a caller's context, so one can start after the connection's goroutines have all returned. Each only takes
  `notifyMu`, broadcasts and returns (`wakeNotificationWaits`). Every other goroutine is started by `start`, before `Wait` can
  run, or by a goroutine already counted, so the counter is above zero when it starts.
* **Unbounded,** as `jsonrpc2_v2`'s and the MCP Go SDK's are. A caller that needs a bound runs it on a goroutine and selects (question
  4).

**The side connections** gain `Err` and `Wait`, delegating to their connection, emitted by the generator beside `Done` and `Close`.

**Docs.** `Done`'s doc names `Err` and `Wait`. The README's "Closing and limits" shows `Close`, then `Wait`, and reading `Err`. The
leak guard's docs in `docs/architecture.md` note that a test can end with `Wait`.

**Release.** Exported API added and `io.EOF` newly reachable from the peer's EOF, so a minor, `v1.18.0`, which the owner confirms at the
tag.

### Consequences

* Good, because a caller can say why a connection ended without having made a call, and tell an overflow or the peer's EOF from a
  `Close` with `errors.Is`.
* Good, because the answer no longer changes after the end (E4).
* Good, because a test, or a program that must not leave goroutines, can wait for the connection to stop entirely (G2, G4).
* Neutral, because `Wait` can block forever with a reader `Close` cannot close, or a handler that ignores its context; its doc says
  so, as 0023's `Close` doc does for the reader.
* Bad, because a call that tests `errors.Is(err, io.EOF)` on a failed call's error now matches after the peer's EOF, where it did not;
  the release notes name it (amended 2026-10-20, see "Amended during the PLAN's step 6").
* Bad, because every goroutine start goes through the wait group, so a future `go` statement that bypasses it is a gap only a test
  catches.

### Confirmation

The PLAN names its checks, each seen to fail on a planted break in a scratch copy, at least:

* `Err` before the end is nil; after each of E2, E3, E5 and E6 it is the error of the table, with `errors.Is` true for
  `ErrConnectionClosed`, `ErrPeerDisconnected`, `io.EOF` and `ErrNotificationQueueOverflow` where they apply;
* after the peer's EOF and a later `Close`, `Err` is still the peer's (E4), and a call returns `ErrConnectionClosed`;
* `Wait` returns after `Close` with an `io.Pipe` reader, with no goroutine of the connection left; it does not return while a reader
  without `Close` blocks or a handler ignores its context, and returns once they do (G3, G4);
* a `go` statement in the package's non-test files outside the wait group fails a test;
* the leak guard passes, `make lint` reports 0 issues, and `make api` shows only additions.

#### Questions for the owner

Each has a recommended answer.

1. **The accessor's name.**
   * Recommended: `Err() error`, as `context.Context` and `jsonrpc2` name it beside `Done`.
   * Alternatives: `Cause() error`, after `context.Cause`; or none, as O3.
2. **Fixed at the first end** (E4).
   * Recommended: yes, `Err` records the first end's cause.
   * Alternative: `Err` returns `endError` as it is, which changes after a late `Close`.
3. **Testable causes** (E3, E6).
   * Recommended: export `ErrNotificationQueueOverflow`, same text; the peer's EOF wraps `io.EOF`, same text.
   * Alternative: leave them as they are; a caller then matches text, or cannot tell.
4. **`Wait`'s form.**
   * Recommended: `Wait() error`, unbounded, as `jsonrpc2_v2` and the MCP Go SDK.
   * Alternatives:
     * `Wait(ctx context.Context) error`, bounded like `http.Server.Shutdown`, returning the context's error when it ends first;
     * a channel, `Stopped() <-chan struct{}`, which composes with `select` like `Done`.
5. **What `Wait` covers.**
   * Recommended: every goroutine the connection starts, handlers included.
   * Alternative: only the SDK's own; a handler is then the caller's to wait for. `receive` still waits on the reader either way.
6. **The side connections.**
   * Recommended: `Err` and `Wait` on `AgentSideConnection` and `ClientSideConnection` too, delegating.
   * Alternative: on `Connection` only, reached through `Connection()`.
7. **A call after a late `Close`.**
   * Recommended: unchanged, `ErrConnectionClosed`, as the sentinel's doc says for a send once `Close` has run.
   * Alternative: the first end's error, as `Err` returns.

#### Decided with the owner, 2026-10-20

| Question | Answer |
| :--- | :--- |
| 1. The accessor's name | As recommended: `Err() error`. |
| 2. Fixed at the first end | As recommended: `Err` records the first end's cause. |
| 3. Testable causes | As recommended: `ErrNotificationQueueOverflow` exported, same text; the peer's EOF wraps `io.EOF`, same text. |
| 4. `Wait`'s form | As recommended: `Wait() error`, unbounded. |
| 5. What `Wait` covers | As recommended: every goroutine the connection starts, handlers included. |
| 6. The side connections | As recommended: `Err` and `Wait` on both, delegating. |
| 7. A call after a late `Close` | As recommended: unchanged, `ErrConnectionClosed` (amended 2026-10-20, see "Amended during the PLAN's step 6"). |

#### Amended during the PLAN's step 6, 2026-10-20

Question 7's decision stands: a call's error is unchanged. Two statements of what that error is claimed more than the code does, and a
probe against `git archive` trees of `v1.17.1` and of the PLAN's step 5 commit found:

* **After the peer's end of output, a request does not return `ErrConnectionClosed` after a late `Close`.** A request, in flight or
  sent later, fails `-32603` with "input ended before a response", wrapping `ErrPeerDisconnected`, before and after a late `Close`,
  in both trees: the end of input refuses it before the connection's end is consulted. A notification sent after the late `Close`
  wraps `ErrConnectionClosed`, as this record said.
* **Only a notification's error newly matches `io.EOF`.** After the peer's EOF, a later notification's error wraps `io.EOF` in the
  step 5 tree and not in `v1.17.1`'s, after a late `Close` too. A request's error matches it in neither. Every error's text is the same
  in both trees.
* **After an overflow,** a request's and a notification's error both wrap `ErrConnectionClosed` and `ErrNotificationQueueOverflow`, as
  `ErrNotificationQueueOverflow`'s doc says.

The owner chose to correct the docs, not the behaviour: `Err`'s doc, the README and `TestErr` say and require the above. The
alternative, a request after the end reporting the recorded end, was a behaviour change this record's question 7 ruled out.

[0026-PLAN-connection-end-cause-and-wait.md](0026-PLAN-connection-end-cause-and-wait.md) implements it.

#### Amended by 0041, 2026-10-27: a bounded wait beside `Wait`

The owner reopened question 4 in
[0041-MADR-close-the-gaps-the-consumers-wait-on.md](0041-MADR-close-the-gaps-the-consumers-wait-on.md), question 2. `Wait()`
stays as decided, unbounded. `WaitContext(ctx) error` is added beside it, on `Connection` and `agentproc.Process`; a side
connection's is its `Connection()`'s (0041's "Amended by the PLAN's phase 6"). It returns `Wait`'s error, or
`context.Cause(ctx)` when ctx ends first. The pattern this record left to callers,
`Wait` on a goroutine and a select, had been written three times in one consumer and was needed by `acptest.Connect`'s cleanup.

## Pros and Cons of the Options

### O1. Both accessors

* Good, because it serves both consumers' gaps, with the prior art's shapes.
* Good, because it fixes E4 and makes E3 and E6 testable on the way.
* Neutral, because `Wait` cannot be bounded for a reader the SDK does not own.
* Bad, because it is the larger change: a wait group around every goroutine, and the generator's side methods.

### O2. `Err` only

* Good, because it is small: one accessor, the recorded cause, two testable causes.
* Bad, because a caller still cannot know when the connection's goroutines are gone (G3, G4).

### O3. Neither

* Good, because nothing changes.
* Bad, because a caller that made no call cannot learn why the connection ended, and E4 stays.

## More Information

* **Measurements outside the tree:** the two probes ran as a test file in a scratch copy of `0afe75e`'s tree; the consumers' trees were
  only read.
* **Related records:**
  * [0023-MADR-connection-lifetime-and-test-leaks.md](0023-MADR-connection-lifetime-and-test-leaks.md): `Close` ending every handler, the
    leak guard, and the two accessors it left to this record;
  * [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md): B17 (`Close` and
    `ErrConnectionClosed`), B23 (the end-of-input drain);
  * [0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md): the documented
    rule that a panicking handler ends the process, and the `apidiff` gate.
* **Not decided here:** an SDK helper that starts an agent process and escalates its shutdown, the third item 0023 left, which stays a
  record of its own.

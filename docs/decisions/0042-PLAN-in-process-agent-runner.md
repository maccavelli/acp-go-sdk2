---
status: in-progress
date: 2026-10-30
associated-madr: "0042-MADR-in-process-agent-runner.md"
---

# Implement 0042: an in-process agent runner, graceful serving, the agent's stderr tail, and transcripts

Associated MADR: [0042-MADR-in-process-agent-runner.md](0042-MADR-in-process-agent-runner.md)

## Goal

The MADR's four items ship with no new dependency, and `make api` lists only additions:

1. **`agentproc.InProcess`.** `InProcess{Serve, Grace, ConnectionOptions, OutputTap, InputTap}.Start(client)` runs a serve function on
   a goroutine, joined to the client by two `io.Pipe`s, and returns the same `*agentproc.Process` a child gets. Its lifecycle
   follows the MADR's mapping table:
   * step 1 closes the agent's input;
   * step 2 cancels `Serve`'s context with `ErrTerminated`;
   * step 3 closes its pipes with `ErrKilled`, then abandons a `Serve` that still runs, with `ErrAbandoned`.
2. **`Connection.Shutdown(ctx)`, `ErrShutdown` and `acp.ServeAgent`.**
   * `Shutdown` ends every handler's context with `ErrShutdown`, answers a later request `-32603` while still reading, writes the
     earlier requests' answers, then closes.
   * `ServeAgent` serves an agent until its input ends, or shuts it down within 5 s when its context ends.
3. **The agent's stderr tail.** `Command.StderrTail` (bytes), `Command.StderrTailLines` and `Command.JoinStderrTail`, with
   `Process.StderrTail()` and `*StderrError`.
4. **Transcripts.**
   * In `agentproc`: `NewTranscript`, `Transcript.Input`, `Transcript.Output` and `Transcript.Close`.
   * In `acptest`: `ReadRecords`, `WriteRecords`, `Golden` (updated under `ACPTEST_UPDATE=1`), `Decode[T]`, `ReplayAgent` and
     `ConnectReplay`.
   * `ValidateGolden` pairs a response with its request wherever the request is recorded.

## Prerequisites

* The MADR is `accepted`, with the owner's answers of 2026-10-28 to item 1's seven questions and items 2 to 4's eight.
* [0041-PLAN-close-the-gaps-the-consumers-wait-on.md](0041-PLAN-close-the-gaps-the-consumers-wait-on.md) is `complete` (`v1.25.0`):
  `WaitContext`, `ExitedFirst`, `Recorder`'s records, `ValidateGolden` and the read to the end of a child's output are the
  behaviours this PLAN extends.

## Scope

* **`agentproc/`:**
  * `agentproc.go`: `Process`'s child-specific parts behind an unexported seam (phase 3); the stderr tail (phase 6); the package
    doc;
  * `tap.go`: the tap reader and writer over an `io.ReadCloser` and an `io.WriteCloser`, not an `*os.File`;
  * new files: `inprocess.go` (phase 4), `stderr.go` (phase 6), `transcript.go` (phase 7);
  * their tests, and `example_test.go`.
* **The root package, hand-written:**
  * `connection.go`: `Shutdown` and `ErrShutdown`, and the shutting-down state;
  * a new `serve.go`: `ServeAgent`. `TestRuntimeFilesCoverHandWritten` (`cmd/generate/engine_test.go`) names it;
  * `example/agent/main.go`, which serves through `ServeAgent`.
* **`acptest/`:** `golden.go` (the pairing, `Golden`), a new `records.go` (`ReadRecords`, `WriteRecords`, `Decode`), a new
  `replay.go` (`ReplayAgent`, `ConnectReplay`), and their tests.
* **Generated:** none.
* **Docs:** `README.md`'s `agentproc`, connection and `acptest` sections; `docs/architecture.md`.
* **Records:** this pair and their rows; an amendment to
  [0041-MADR-close-the-gaps-the-consumers-wait-on.md](0041-MADR-close-the-gaps-the-consumers-wait-on.md)'s "Not chosen"; the
  release-notes report (phase 11).
* **Tools:** `tools/consumer.py` (phase 11).
* **(Added 2026-10-30, phases 12 to 17, issue #12.)** `agentproc/agentproc.go`, `agentproc/example_test.go`, a new
  `agentproc/closecontext_test.go` and `TestWaitErrorSaysTheCauseOnce`; `errors.go`, `serve.go` and `shutdown_test.go`; `README.md`
  and `docs/architecture.md`; `tools/consumer.py` and its row in `tools/README.md`; the release notes
  `0042-REPORT-release-notes-v1-28-0.md`; the 0027 MADR's annotation. Not in scope: `Connection.Wait` and `WaitContext`, unchanged
  (0026, 0041); `Process.Wait`'s bound, which the MADR rejects (I7); any change in `<cli-b>`.
* **Not in scope:**
  * `clienthost` and `internal/proctree`;
  * a handshake inside `InProcess.Start`;
  * recovering panics, and a `StartContext`;
  * a replay client;
  * redaction inside `Transcript`;
  * any change in either consumer's repository.

## Implementation Steps

Each phase ends with its checks, an entry in the execution record, and its files staged for the owner, who commits on `main`
(`AGENTS.md`, "Commits"). Every new check is seen to fail on a planted break, each in its own copy of the tree, and the record
keeps each break's failing line.

Every code phase's gates:

* `make pre-add-check`, and `scripts/go-precheck.sh` on each new file;
* `make check-records`, `make markdownlint`, `make disclosure-check` and `make api`;
* `tools/gates.py --all`, with `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` set; `tools/platforms.py` for every `agentproc` change;
* `tools/idscan.py --paths` over the staged files.

### 1. Records

This PLAN becomes `in-progress`, and its row follows it.
[0041-MADR-close-the-gaps-the-consumers-wait-on.md](0041-MADR-close-the-gaps-the-consumers-wait-on.md) gains "Amended by
0042-MADR-in-process-agent-runner.md, 2026-10-28". It says that the in-process runner in its "Not chosen" is decided by 0042, and
that the exported process-tree primitive stays declined.

### 2. Probes and the reproduction

In a scratch module or a scratch copy of the tree only, on macOS and on both hosts. Nothing here changes the tree.

* **Item 1's pipes.** Each probe confirms a behaviour the mapping table relies on:
  * **P1.** A connection whose writer is an `io.Pipe` with no reader blocks in its write. `CloseWithError(e)` on the read end
    unblocks it with `e`. This is the reason the agent's input is closed when `Serve` returns.
  * **P2.** A `Serve` written as a program's main returns once the agent's input is closed.
  * **P3.** Once the agent's output is closed, the client's connection ends with `ErrPeerDisconnected` wrapping `io.EOF`, and
    `acp.EndedCleanly` is true, as at a child's exit.
  * **P4.** A `Serve` that writes N bytes and returns at once loses none, since an `io.Pipe` write returns only once it has been
    read. A tap over the pipes receives them all.
  * **P5.** A `Read` or `Write` blocked on the agent's closed pipe ends returns.
* **Item 2, the lost answer reproduced.** A scratch test serves an agent whose prompt handler answers `cancelled` when its context
  ends, as S1's `Serve` does. It cancels the serve's context during the prompt, and shows the client's `Prompt` failing with the
  connection's end instead of returning `cancelled`, the MADR's S3. Nothing changes until it fails so.
* **Item 4, the order.** A scratch transcript from both taps of a real session is fed to today's `ValidateGolden`, to see whether
  a response is ever recorded before its request (R6). The answer is recorded either way; phase 7's relaxation is made in any
  case, since the order is not guaranteed.

A result that contradicts the MADR stops the phase: the MADR is amended, with the owner, before phase 3.

### 3. `agentproc`'s seam (item 1; no behaviour change)

`Process` keeps its exported API and its tests. Inside it:

* the child's parts, `exec.Cmd`, the `sharedClose` input, `proctree.Tree` and the locked-thread `run`, sit behind an unexported
  interface of the operations `stop` and `watch` use:
  * close the agent's input;
  * terminate;
  * kill;
  * wait for the agent's end;
  * end what is left;
* the tap reader and writer take an `io.ReadCloser` and an `io.WriteCloser`.

Checks:
* every existing `agentproc` test, unchanged;
* `TestDetachedTapIsPrefix`, `TestCloseReadsTheAgentsLastOutput` and `TestKillClosesAtOnce` 100 times on macOS under `-race` and
  on the Linux host, and the first two on the Windows host, as 0041's phase 3 ran them;
* `make api`: "0 incompatible, 0 compatible".

A refactor adds no check, so it has no planted break. The existing tests are its check.

### 4. `InProcess` (item 1)

`inprocess.go`:

* **`Start`:**
  * refuses a nil `Serve` or client, as `Command.Start` refuses a nil `Cmd`;
  * makes two `io.Pipe`s and `Serve`'s context, `context.WithCancelCause(context.Background())`;
  * starts `Serve`, the watcher and the monitor.
* **When `Serve` returns:**
  * the agent's output and input are closed;
  * `ExitedFirst` is recorded;
  * `Done` closes;
  * the connection is closed once it has read to the end, up to the grace, or at once after `Kill`, as `watch` does for a child.
* **The steps:**
  1. close the agent's input;
  2. cancel the context with `ErrTerminated`;
  3. cancel it with `ErrKilled`, and close the agent's pipe ends.
  * After step 3's grace, a `Serve` still running is abandoned: `Done` closes, and `Wait`'s error has `ErrKilled` and
    `ErrAbandoned` joined.
  * `ErrTerminated` and `ErrKilled` are joined only when their step found `Serve` running.
* **No panic is recovered,** and the doc says so, with the MADR's O5 pattern for isolation.
* **From phase 2's probes** (the MADR's "Amended by the PLAN's phase 2"):
  * the agent's output is closed with a plain `Close`, so the client reads `io.EOF`;
  * `ErrKilled` reaches the agent through its context's cause, and its blocked reads and writes return `io.ErrClosedPipe`;
  * the runner closes its pipe ends itself whenever the connection ends.

`inprocess_test.go`, a `Serve` of each kind:

| Test | `Serve` | Required |
| :--- | :--- | :--- |
| `TestInProcessEndsAtInputEnd` | returns at its input's end | `Close` returns in step 1; `Wait`'s error is `Serve`'s |
| `TestInProcessTerminate` | ignores its input's end, returns at its context's end | `context.Cause` is `ErrTerminated`; `Wait` has `ErrTerminated` |
| `TestInProcessKill` | ignores both, returns when its read fails | step 3 returns it; `Wait` has `ErrKilled`, not `ErrAbandoned` |
| `TestInProcessAbandon` | ignores all three until the test releases it | `Close` returns within three graces and a slack; `Wait` has `ErrAbandoned`; the test then releases it, so the leak guard passes |
| `TestInProcessKillAtOnce` | as `TestInProcessKill` | `Kill` returns within one grace and a slack |
| `TestInProcessReturnsFirst` | returns while the connection is open | `ExitedFirst` true; with the connection closed first, false |
| `TestInProcessClientWriteAfterReturn` | returns at once, while the client sends | the client's request fails, not blocks |
| `TestInProcessTaps` | writes 64 MiB and returns | the output tap receives it whole; with a slow reader hook, as `TestCloseReadsTheAgentsLastOutput` |
| `TestInProcessStartRefuses` | nil `Serve`, nil client | the errors, and nothing started |

Planted breaks, each in its own copy:
* `Serve`'s return not closing the agent's input;
* step 2 cancelling with no cause;
* step 3 not closing the pipes;
* the abandonment bound removed;
* `ExitedFirst` recorded after `Done`;
* the output tap left off the in-process reader.

### 5. `Shutdown` and `ServeAgent` (item 2)

* **`connection.go`:**
  * `Shutdown(ctx)` marks the connection shutting down, then ends the handlers' context with `ErrShutdown`. It waits for the
    requests already received to be answered, through the connection's own goroutine group as `drainInput` waits, so no `go`
    statement is added. Then it closes, or closes at once when `ctx` ends first, returning `context.Cause(ctx)`.
  * While shutting down, the receive loop answers a new request `-32603`, its data saying the agent is shutting down, and keeps
    delivering responses to the agent's own requests.
  * A second `Shutdown`, or a `Close` during one, returns at once with the same end.
* **`serve.go`:** `ServeAgent(ctx, agent, in, out, opts...)` builds the side connection with `opts` and waits on the caller's
  goroutine:
  * at the input's end, it waits for the connection's goroutines within 5 s, and returns nil when `EndedCleanly`, or the
    connection's error;
  * at `ctx`'s end, it calls `Shutdown` under a 5 s bound, and returns `context.Cause(ctx)` joined with `Shutdown`'s error.
* **`example/agent/main.go`** serves through `ServeAgent`.

Tests:

| Test | Required |
| :--- | :--- |
| `TestShutdownAnswersRunningPrompt` | a prompt whose handler honours its context returns `cancelled` to the client; the handler read `ErrShutdown` |
| `TestShutdownRefusesLateRequest` | a request sent after `Shutdown` began is answered `-32603`, and a response to the agent's own request still arrives |
| `TestShutdownBound` | a handler that ignores its context: `Shutdown` returns `context.Cause(ctx)` when its context ends, and the connection is closed |
| `TestServeAgentInputEnd` | `ServeAgent` returns nil at a clean end of input, its running request answered |
| `TestServeAgentContextEnd` | `ServeAgent` returns the context's cause, and its prompt was answered `cancelled` |
| `TestInProcessServeAgentStep2` | an `InProcess` agent built on `ServeAgent`, closed in step 2, answers its prompt `cancelled` |

Phase 2's reproduction, run against this phase, returns `cancelled`. `TestGoStatementsCounted` still allows only `WaitContext`'s
`go` statement, and `TestSideConnectionMethodsAreProtocolMethods` is unchanged, since `Shutdown` is `Connection`'s.

Planted breaks:
* `Shutdown` closing at once, as `Close` does;
* the late request left unanswered;
* the handlers' context ended with no cause;
* `ServeAgent` ignoring its context;
* a `go` statement added to wait for the requests.

### 6. The stderr tail (item 3)

`stderr.go`: a writer that keeps the last `StderrTail` bytes and the last `StderrTailLines` lines, a cap of zero being no cap,
starting at a line's beginning. A line longer than the byte cap keeps its end. It forwards every byte to `Cmd.Stderr` when that is
set.

* `Start` sets it as `Cmd.Stderr` when either cap is positive.
* `Process.StderrTail()` is final once `Wait` has returned.
* Under `JoinStderrTail`, `Wait`'s error, when not nil and with a tail, has `&StderrError{Tail}` joined, computed once.

Tests:
* each cap alone and both together, over a stream of short lines, one line longer than the byte cap, and an unended last line;
* `Cmd.Stderr` receiving every byte;
* a child mode that writes to stderr and exits 1, whose words reach `StderrTail()` and, under the join, `errors.As` on `Wait`'s
  error, with `errors.Is(err, ErrKilled)` still holding for a killed agent;
* no join without the field, and an `InProcess` agent's empty tail.

The child mode runs on macOS and both hosts through the gates.

Planted breaks:
* the byte cap not applied to a long line;
* the line cap ignored;
* `Cmd.Stderr` not forwarded;
* the join made without the field.

### 7. Transcripts, records and `Golden` (item 4)

* **`agentproc/transcript.go`:** `NewTranscript(w)`.
  * `Input()` and `Output()` are writers, for `InputTap` and `OutputTap`. They join lines across writes, as `LineTap` does, and
    write one `{"direction":…,"frame":…}` object a line under a mutex.
  * A line that is not JSON goes as a JSON string.
  * `Close` closes neither tap's stream, and returns the first write error.
* **`acptest/records.go`:** `ReadRecords`, `WriteRecords` in the same form, and `Decode[T](records, d, method)`.
* **`acptest/golden.go`:**
  * `Golden(tb, path, records)`: equal, it passes. Under `ACPTEST_UPDATE=1` it writes `path`. Otherwise it writes the actual
    records to `tb.ArtifactDir()` and reports the first record that differs.
  * `ValidateGolden` pairs in two passes: the requests first, then the responses.

Tests:
* a real session through `agentproc` with a `Transcript` as both taps, read back, validated, and its `session/update`s decoded;
* a response recorded before its request validating;
* `Golden`'s three paths;
* `Decode` refusing a record whose params its type refuses.

Planted breaks:
* the direction swapped in `Transcript`;
* a line split across writes recorded as two;
* `ValidateGolden` back to one pass;
* `Golden` updating without the variable;
* `Decode` skipping the direction.

### 8. `ReplayAgent` and `ConnectReplay` (item 4, question 4c)

`acptest/replay.go`, the MADR's rule:
* the received records are played in order;
* a notification, or an agent's request under a fresh id, is sent when its turn comes;
* a response waits for the client's matching request: the k-th request of a method meets the recorded answer to the k-th, under
  the live id;
* a request with no answer left is answered `-32603`;
* at the input's end, `Serve` returns nil, or an error listing the drift.

`ConnectReplay` joins it to a client over two pipes, with `Connect`'s cleanup.

Tests:
* a session recorded in phase 7's test, replayed to the same client, with `Serve` returning nil;
* a client that sends an extra request, met by `-32603`, and `Serve` naming it;
* a client that stops early, and `Serve` naming the frames not played;
* a transcript whose directions are interleaved out of order, replayed;
* an agent's recorded `session/request_permission`, answered by the client under its fresh id.

Planted breaks:
* the live id not substituted;
* the k-th matching taken as the first;
* drift not reported;
* the agent's request sent under its recorded id.

### 9. Docs

* `agentproc`'s package doc: it "starts an ACP agent as a child process, or runs one written in Go in this process".
* `README.md`:
  * the `agentproc` section: `InProcess`, the mapping in brief, `ErrAbandoned`, the panic rule, the stderr tail and
    `Transcript`;
  * the connection section: `Shutdown` beside `Close`, and `ServeAgent`;
  * the `acptest` section: records, `Golden`, `Decode` and `ReplayAgent`.
* `docs/architecture.md`: the seam, the new files, the shutting-down state, and the new tests.
* Examples: `ExampleInProcess`, `ExampleServeAgent`, `ExampleTranscript` and `ExampleReplayAgent`.

### 10. Close the implementation

* `make api` against the newest `v1` tag;
* `tools/gates.py --all`;
* `README.md`, `docs/architecture.md` and every new doc comment read in full against the code;
* the MADR's "Confirmation" walked item by item.

### 11. The release, only on the owner's ask in that turn

As 0041's phase 17 ran:
* the release-notes report;
* `tools/consumer.py` extended to run an in-process agent through `ServeAgent`, read its stderr tail, and write and read a
  transcript, with `make check-tools`;
* the push and its CI, before the report cites it;
* the tag on the owner's ask, the GitHub release, `tools/publication.py`, and `tools/consumer.py`.

### Phases 12 to 17, issue #12 (added 2026-10-30, proposed)

The MADR's "Amended 2026-10-30, issue #12" (X1 to X4). Nothing is changed until the owner approves; every code phase keeps the gates
above, and each new check is seen to fail on a planted break in its own copy of the tree.

#### 12. Records

On the owner's approval: this PLAN and its row `in-progress`; the MADR's amendment `accepted`, dated; its row and this PLAN's name
the amendment.

#### 13. `Wait`'s error said once (X3)

* `agentproc/agentproc.go`:
  * `agentError`, factored out of `Wait` and called under `p.mu`: the composition `Wait` makes today, with one case added, the agent's
    error returned as it is when it already wraps the step's sentinel (`errors.Is(p.waitErr, p.ended)`); the stderr tail joined
    after, as now. `Wait` calls it, so its result is unchanged but for that case.
  * `Wait`'s doc, as the MADR's specification gives it.
* Test, `TestWaitErrorSaysTheCauseOnce`: the MADR's I9 table, each row's `Wait` error by its exact text, with `errors.Is` true for
  the step's sentinel; the child's row unchanged, its text beginning `agentproc: agent killed: `; and a `Serve` that returns
  `ErrAbandoned`'s case unchanged, `agentproc: agent killed: agentproc: in-process agent abandoned`.
* Planted break: the added case removed. The test must fail with the repeated text.

#### 14. `acp.ErrHandlersRunning` and `ServeAgent` (X2)

* `errors.go`: `ErrHandlersRunning`, beside `ErrShutdown`, with the specification's doc.
* `serve.go`:
  * at the input's end, `fmt.Errorf("%w %v after its end", ErrHandlersRunning, notificationQueueDrainTimeout)`, the same text as
    today;
  * at the context's end, `Shutdown`'s own error, when it wraps `context.DeadlineExceeded`, the bound having passed with requests
    unanswered, wrapped as `fmt.Errorf("%w %v after the context's end: %w", ErrHandlersRunning, notificationQueueDrainTimeout, err)`,
    then joined after `context.Cause(ctx)` as now. Only `Shutdown`'s error is tested, so a caller's context whose own cause is
    `context.DeadlineExceeded` is not mistaken for the bound;
  * `ServeAgent`'s doc names the sentinel at both ends.
* Tests, in `shutdown_test.go`, each in a `synctest` bubble, so the 5 s bound passes on the bubble's clock:
  * `TestServeAgentHandlersStillRun`, "the input's end": a prompt handler that ignores its context until released; the input ends;
    `ServeAgent`'s error wraps `ErrHandlersRunning`, and its text is `acp: the connection's handlers still run 5s after its end`;
  * "the context's end": the same handler, the context cancelled with a cause; the error wraps both `ErrHandlersRunning` and the
    cause;
  * each releases its handler before the bubble ends.
* Planted breaks: `%w` made `%v` at the input's end; the context's-end wrapping removed. Each must fail its case.

#### 15. `CloseContext` and `KillContext` (X1)

* `agentproc/agentproc.go`:
  * `hurryUp`, the `hurry` close `Kill` makes today, now a method that `Kill`, `KillContext` and `CloseContext`'s
    `context.AfterFunc` call;
  * `waitDone`, factored out of `WaitContext`, as `Connection`'s is: the channel its one goroutine closes once `Wait` has returned;
  * two monotonic flags, `connWaitBegan` and `connWaitEnded` (`atomic.Bool`), set by `Wait` around its wait on the connection, so
    that "the handlers still run" is true exactly while `Wait` is past the watcher, whose close of the connection ends the handlers'
    context, and the connection's goroutines have not returned;
  * `CloseContext`: `context.AfterFunc(ctx, p.hurryUp)` for the steps alone, `p.closeOnce.Do(p.stop)`, the `AfterFunc` stopped,
    then the bounded wait; `KillContext`: `p.hurryUp()`, the steps, the bounded wait;
  * the bounded wait: `waitDone()` or `ctx.Done()`, the end winning when both are ready, as `WaitContext` does; at the context's end,
    `errors.Join` of `context.Cause(ctx)`, of `acp.ErrHandlersRunning` when the flags say so, and of `p.agentError()`, in that order;
  * the docs of `Close`, `Kill`, `CloseContext` and `KillContext`, as the specification gives them.
* `agentproc/example_test.go`: `ExampleProcess_CloseContext`, an in-process `ScriptAgent` closed under a 5 s bound, with
  `// Output:`; and a comment showing `context.WithTimeoutCause`.
* Tests, a new `agentproc/closecontext_test.go`:
  * `TestCloseContextBoundsHandlers`: for an `InProcess` agent and a child (the `burst` mode), and for `CloseContext` and
    `KillContext`, a client `SessionUpdate` that ignores its context until released. The call returns within its bound and the
    slack; `errors.Is` holds for `context.DeadlineExceeded` and `acp.ErrHandlersRunning`; `Done` is closed. Released, `Wait` returns
    what `Close`, or `Kill`, returns for that agent.
  * `TestCloseContextJoinsTheAgentsError`: a `Serve` that returns its own error after `ServeAgent`, and a hung handler; the error
    holds it.
  * `TestCloseContextWaitsForHandlers`: a handler that returns half a grace into a 5 s bound; `CloseContext` returns `Wait`'s
    error, `nil`, after the handler has returned, and joins nothing: `<cli-b>`'s `TestCloseWaitsForHandlers` in this repository.
  * `TestCloseContextHurriesTheSteps`: a `stubborn` child under a 5 s grace and a 300 ms context; `CloseContext` returns well within
    one grace, `errors.Is(err, ErrKilled)` and not `ErrTerminated`; `KillContext` likewise.
  * `TestCloseContextCause`: `context.WithTimeoutCause` with the test's own error; the error holds it.
  * `TestCloseContextNotHandlers`: an output tap held by its gate past a 100 ms context, under a 2 s grace, after a clean exit; the
    error holds the context's cause and not `acp.ErrHandlersRunning`: a drain is not a handler.
  * `TestCloseContextOneGoroutine`: 100 calls with an ended context leave one waiting goroutine, as `TestProcessWaitContext` counts
    them.
* Planted breaks, each in its own copy:
  * `CloseContext` calling `Close`: `TestCloseContextBoundsHandlers` by its bound;
  * the `AfterFunc` removed: `TestCloseContextHurriesTheSteps` by its duration;
  * `KillContext` without `hurryUp`: the same, its `KillContext` case;
  * `agentError` not joined: `TestCloseContextJoinsTheAgentsError`;
  * the sentinel joined at any context's end: `TestCloseContextNotHandlers`;
  * the sentinel never joined: `TestCloseContextBoundsHandlers`;
  * a goroutine per call, not `waitDone`: `TestCloseContextOneGoroutine`.
* Repeats: the new tests 20 times under `-race` on macOS, and 20 times on the Linux and Windows hosts.
* (Added 2026-10-30, during the phase: `agentproc/waitcontext_test.go`'s `processWaiters`, which matched the waiter by the function
  `waitDone` renames; see the execution record's deviation.)

#### 16. Docs (X4)

* `README.md`:
  * the `agentproc` paragraph's "every wait bounded" says what is bounded;
  * `Close`'s paragraph: `Close`, `Kill` and `Wait` also wait for the connection's handlers, without bound, and
    `CloseContext` and `KillContext` bound that wait, joining `acp.ErrHandlersRunning`;
  * `Wait`'s error said once;
  * `ServeAgent`'s paragraph: `acp.ErrHandlersRunning` at both ends.
* `docs/architecture.md`: `Wait`'s description, `agentError`, the bounded methods and their flags, and the new tests in the
  package's checks; `ServeAgent`'s sentinel.
* Each claim mapped to the test that makes it; every changed doc comment read in full against the code.

#### 17. Close, and the release on the owner's ask

* `make api` against `v1.27.0`: "0 incompatible, 3 compatible", `ErrHandlersRunning`, `(*Process).CloseContext` and
  `(*Process).KillContext`; `tools/gates.py --all`; the amendment's "Confirmation" walked item by item.
* On the owner's ask, as phase 11 ran: the release-notes report
  `0042-REPORT-release-notes-v1-28-0.md`, a minor; `tools/consumer.py` extended to close an in-process agent through
  `CloseContext` and to test `acp.ErrHandlersRunning` with `errors.Is`, with `make check-tools`; the push and its CI; the tag
  `v1.28.0`, the GitHub release, `tools/publication.py` and `tools/consumer.py v1.28.0`.
* Issue #12 answered and closed, on the owner's ask, with the release's link.

## Verification

* Every item of the MADR's "Confirmation" holds, each by its phase's tests.
* `make api` lists only additions:
  * the root package: `Connection.Shutdown`, `ErrShutdown`, `ServeAgent`;
  * `agentproc`: `InProcess` and its `Start`, `ErrAbandoned`, `Command.StderrTail`, `StderrTailLines` and `JoinStderrTail`,
    `Process.StderrTail`, `StderrError` and its `Error`, `NewTranscript`, `Transcript` and its three methods;
  * `acptest`: `ReadRecords`, `WriteRecords`, `Golden`, `Decode`, `ReplayAgent` and its `Serve`, `ConnectReplay`.
* No generated file changes.
* After phase 3, every existing `agentproc` test passes unchanged, with the 100-run suites on macOS and both hosts.
* `make pre-add-check`, `make lint`, `tools/platforms.py` and `tools/gates.py --all` at every code phase.
* `make disclosure-check` and `tools/idscan.py`: nothing found.
* Every planted break fails its check, each in its own copy of the tree.
* (Added 2026-10-30, phases 12 to 17.) The amendment's "Confirmation" holds, each item by its phase's tests; `make api` lists only
  `ErrHandlersRunning`, `CloseContext` and `KillContext`; every existing test passes unchanged but for the texts phase 13 corrects,
  which none pins; `TestNoLostOutput` among them, since `Wait` is unchanged.

## Rollout and Rollback

* **Rollout.** Phases 3 to 8 are separate commits, so the seam lands alone and is seen to change nothing. One minor carries
  them. Each consumer adopts under its own records:
  * `<cli-b>`: its `acpclient.Start` runs its agent through `InProcess`, and its `acpserver.Serve` through `ServeAgent`. Its
    goldens move to `Golden` and the record form, which its 0034 D3 would record.
  * magic-cli-remote: its ACP path sets the stderr tail and a `Transcript` as both taps. Its fixture tests use `ReadRecords` and
    `Decode`, and its 0214 G1 recording can replay through `ReplayAgent`.
* **What a consumer meets on the release:** new names, and one widened behaviour, `ValidateGolden` accepting a response recorded
  before its request. A `Command`, a connection that never calls `Shutdown`, and today's goldens behave as before.
* **Rollback before a tag:** revert the phase's commits. Phase 4 reverts without phase 3, and phases 5 to 8 revert independently.
* **Rollback after a tag:** a name cannot be removed under `v1`; a defect is fixed forward in a patch.
* **(Added 2026-10-30, phases 12 to 17.)** One minor, `v1.28.0`. Phases 13 and 14 are independent commits; phase 15 needs both.
  Before a tag, each reverts by its commit, phase 15 first. A consumer meets three names, and two changed texts: `Wait`'s error for an
  agent whose error wraps the step's sentinel, and `ServeAgent`'s at its context's end when `Shutdown`'s bound passes. `<cli-b>`
  runs its held P12 with `CloseContext` in place of `Close` then `WaitContext`, under its own records.

## Execution record

### Phase 1, the records (2026-10-28)

The owner committed the pair as `2a3ea34` and approved execution ("committed. proceed.").

* This PLAN is `in-progress`, and its row in `docs/README.md` says so. The MADR was already `accepted` in `2a3ea34`.
* [0041-MADR-close-the-gaps-the-consumers-wait-on.md](0041-MADR-close-the-gaps-the-consumers-wait-on.md):
  * it gains "Amended by 0042-MADR-in-process-agent-runner.md, 2026-10-28", which says that 0042 decides the in-process runner, that
    the exported process-tree primitive stays declined, and that 0042's item 4 widens `ValidateGolden`'s pairing;
  * its "Not chosen" entry is annotated, not rewritten, with a pointer to that amendment.
* Checks, each exit 0: `make check-records`, `make markdownlint`, `make disclosure-check`; `tools/idscan.py --paths` over the
  changed files, nothing found.

### Phase 2, the probes and the reproductions (2026-10-28)

The owner committed phase 1 as `5b09afc` and asked for phase 2 ("committed, proceed").

**How it ran.** One test package, `probe0042`, was added to a scratch copy of the tree made by `git archive` of `5b09afc`. Nothing
in the tree changed. It ran three ways, each exit 0:
* on macOS under `-race`;
* on the Linux host, through `tools/remote.py pkg --host linux --tree <the copy> ./probe0042`;
* on the Windows host, the same with `--host windows`.

Its agent and client are the SDK's own: an agent embedding `AgentRefusals` and a client embedding `ClientRefusals`.

**The pipes.** The same results on all three hosts:

| Probe | Result |
| :--- | :--- |
| P1 | A write to an `io.Pipe` with no reader blocked, and `CloseWithError(errKilled)` on the read end returned it with `errKilled`. A client whose writer had no reader failed its `initialize` at its context's deadline, `-32603` "context deadline exceeded"; after that writer's reader was closed, the connection was **still open 2 s later**. |
| P2 | A serve written as a program's main, `NewAgentSideConnection` then `<-Done()`, returned at once, "0s", when its input was closed; its connection's `Err` was "acp: peer disconnected: peer connection closed". |
| P3 | With the agent's output then closed, the client's `Err` was the same: `ErrPeerDisconnected` true, `io.EOF` true, `EndedCleanly` true. |
| P4 | A serve that answered `initialize`, wrote 1000 `session/update` frames and returned at once: 158946 bytes written, 158946 read through a tap, 1000 of 1000 updates handled. |
| P5 | Blocked reads and writes returned once a pipe end was closed: with `errKilled` when the **other** end was closed with it, and with `io: read/write on closed pipe` when their **own** end was. |

**Two findings beyond the probes' questions:**
* **P1:** a connection whose writer fails does not end. So when `Serve` returns, the runner closes the agent's output, which ends
  the client's connection (P3), and not only the agent's input.
* **P4's first run** wrote 2000 frames. The client's notification queue holds 1024, so it logged "notification queue overflow" and
  closed its connection, as `OverflowClose` does. Its reader was wrapped in an `io.TeeReader`, which does not close, so the
  serve's next write blocked for ever: `P4 serve stuck: client handled 1507, tapped 240226`. The probe was rerun with 1000 frames,
  under the queue. The finding: the runner closes its pipe ends itself whenever the connection ends.

**Item 2, the lost answer, reproduced** on all three hosts:
* `TestReproLostCancelled` serves as the MADR's S1 does, and cancels the serve's context during a prompt whose handler waits for its
  context and answers `cancelled`. The client's `Prompt` returned no stop reason and the error `{"code":-32603,"message":"Internal
  error","data":{"error":"input ended before a response"}}`. The handler's context cause was "acp: connection closed", and the
  agent's connection `Err` the same.
* `TestControlSessionCancel`, the same agent cancelled through `session/cancel`, returned stop reason `"cancelled"` with no error;
  the handler's cause was "acp: prompt turn cancelled". So the handler is right, and the connection's close loses its answer.

**Item 4, the order.** `TestTranscriptOrder` ran 50 sessions with a child agent through `agentproc`. The child was the test binary
itself, in an agent mode. Each session had five prompts of three updates and a permission request, with two `LineTap`s as the
taps, and the records were counted in one pass:

| Host | Responses recorded before their request |
| :--- | :--- |
| macOS | 3 of 600, in 1950 records (4 of 600 in the first run) |
| Linux | 4 of 600, in 1950 records |
| Windows | 0 of 600, in 1950 records |

So the taps' order between the directions does reorder a request and its response, and phase 7's two-pass pairing is needed.

**Against the MADR.** The probes confirmed every row of the mapping table and S3. They corrected one statement: question 3's
recommendation said `io.Pipe` "carries `ErrKilled` to a blocked read or write", which P5 shows holds only from the pipe's other
end. No decision changes:
* step 3 still unblocks the agent;
* `ErrKilled` reaches it through its context's cause;
* the client reads `io.EOF` from a plain `Close`.

The MADR gains "Amended by the PLAN's phase 2, 2026-10-28", and question 3's text is annotated, not rewritten. Phase 4 lists the
three consequences.

**Checks**, each exit 0: `make check-records`, `make markdownlint`, `make disclosure-check`; `tools/idscan.py --paths` over the
changed files, nothing found.

### Phase 3, `agentproc`'s seam (2026-10-28)

The owner committed phase 2 as `4d84311`, which also accepted the MADR's amendment, and asked for phase 3 ("committed, proceed").

**What changed**, in two files, with no exported change:
* **`agentproc/agentproc.go`:**
  * `Process`'s `cmd`, `stdin` and `tree` fields became one, `agent`, of an unexported interface, `agent`, of the five operations
    `stop` and `watch` use:
    * `closeInput`, Close's first step;
    * `terminate`, its second;
    * `kill`, its third;
    * `wait`, which the watcher alone calls;
    * `end`, which ends what is left once the agent has ended.
  * `childAgent` holds the child's `cmd`, its `sharedClose` input and its `proctree.Tree`, and implements the five by
    `sharedClose.Close`, `Tree.Terminate`, `Tree.Kill`, `Cmd.Wait` and `Tree.End`.
  * `run`, still on its locked thread, takes the `*childAgent` it starts.
  * `Start` sets the child's input and tree before it sends `proceed`, which `run` receives before it watches, as it set
    `Process`'s before.
  * The comments on `done`, `exited` and `waitErr` say "the agent's wait" for `Cmd.Wait`.
  * The type was first named `child`, which `child_test.go` already declares for its child modes. The build failed with "child
    redeclared in this block", so it is `childAgent`.
* **`agentproc/tap.go`:** `tapReader` reads an `io.ReadCloser` and `tapWriter` writes an `io.WriteCloser`, not an `*os.File` and a
  `*sharedClose`, and `streams` takes the same two. A child passes the same values as before, so `TestTapsUnset`'s call is
  unchanged.

**Checks:**
* `go test -race -count=1 ./agentproc/...`: ok, 11.4 s.
* **The 100-run suites**, built from the working tree:

  | Host | Tests | Result |
  | :--- | :--- | :--- |
  | macOS, `-race` | `TestDetachedTapIsPrefix`, `TestCloseReadsTheAgentsLastOutput`, `TestKillClosesAtOnce`, `-count=100 -timeout=60m` | 300 passed, 0 failed, 681.9 s |
  | the Linux host | the same three, through `tools/remote.py pkg --host linux` | 300 passed, 0 failed |
  | the Windows host | `TestDetachedTapIsPrefix`, `TestCloseReadsTheAgentsLastOutput` | 200 passed, 0 failed |

* `make api`: "against v1.25.0: 0 incompatible, 0 compatible".
* `make pre-add-check`: "240 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)".
* `tools/gates.py --all`, with both hosts set: all 18 passed, `platforms`, `windows` and `linux` among them.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0. `tools/idscan.py --paths` over the staged files:
  nothing found.

As the step says, a refactor adds no check, so there is no planted break; the existing tests above are its check.
`docs/architecture.md` still describes a child's steps truly, through `Cmd.Wait` and `sharedClose`. The seam is written into it
in phase 9, as planned.

### Phase 4, `InProcess` (2026-10-28)

The owner committed phase 3 as `5293572` and asked for phase 4 ("committed, proceed").

**What changed:**
* **`agentproc/inprocess.go`, new:**
  * `InProcess{Serve, Grace, ConnectionOptions, OutputTap, InputTap}`, with an unexported `readHook` as `Command` has;
  * `(*InProcess).Start` and `ErrAbandoned`;
  * `inProcessAgent`, the second implementation of phase 3's `agent`:
    * `closeInput` closes the client's end of the agent's input;
    * `terminate` cancels `Serve`'s context with `ErrTerminated`;
    * `kill` cancels it with `ErrKilled`, closes the agent's input end with `CloseWithError(ErrKilled)` and its output end with
      `Close`, and starts the abandonment's grace;
    * `wait` returns `Serve`'s error, or `ErrAbandoned` when the grace after `kill` passes first;
    * `end` closes the agent's output and input ends and ends the context.
  * `Serve` runs on a goroutine outside `Process`'s goroutine group, so that `Wait` does not wait for an abandoned one. The
    watcher and the monitor run as for a child, through `p.goroutines`.
  * It follows phase 2's three consequences, as this PLAN's phase 4 lists them.
* **`agentproc/agentproc.go`:** docs only.
  * The package doc gains the in-process agent.
  * `ErrTerminated`, `ErrKilled`, `Done`, `Wait`, `ExitedFirst`, `Close` and `Kill` each gain the in-process case.
  * `Process.agent`'s comment names both kinds.
* **`agentproc/inprocess_test.go`, new:** a `serveAgent` embedding `AgentRefusals`, `startInProcess`, `readUntilFailure`, and ten
  tests, the step's nine and one more (the deviation below). Each passed on its first run under `-race`.

**Deviation, 2026-10-28, a check the step's table did not name.**
* **Found:** the planted break "Serve's return not closing the agent's input" passed `TestInProcessClientWriteAfterReturn` and a
  first form of a direct check:
  * closing the agent's output ends the client's connection at once, through its end of input (phase 2, P3);
  * the watcher then closes the connection, which closes the client's end of the input too;
  * so a write of the client's fails either way, and cannot tell whether the agent's end was closed.
* **Done:** `TestInProcessEndClosesInput`, new. After `Wait`, a read of the agent's end returns `io.ErrClosedPipe` when that end was
  closed, and `io.EOF` when only the client's was. The break now fails it. The step's own list names the break this check must
  fail, so this is recorded as a deviation and was not asked about first, as 0041's phase 3 recorded its `TestKillClosesAtOnce`.
* The close stays, as the MADR's table and A3 state it. It is what makes a client's write fail at once in the moment between
  `Serve`'s return and the connection's end.

**Deviation, 2026-10-28, a test that could hang.** The planted break "the abandonment bound removed" first passed for the wrong
reason. `TestInProcessAbandon` failed with "Close took longer than", but its cleanup's `Close` then waited for ever, and the run
ended at `go test`'s 5-minute timeout without printing the failure. The test now releases its `Serve` in a cleanup registered after
`startInProcess`'s, so that it runs first, and the break fails with "Close took longer than 3.9s".

**Planted breaks**, each in its own copy through `tools/plants.py`, 6 of 6 as required, run again after the lint fix below:

| Break | Check | Failing line |
| :--- | :--- | :--- |
| `Serve`'s return not closing the agent's input | `TestInProcessEndClosesInput` | `the read = EOF, want io.ErrClosedPipe: the agent's end was not closed` |
| step 2 cancelling with no cause | `TestInProcessTerminate` | `Serve's context's cause = context canceled, want ErrTerminated` |
| step 3 not closing the pipes | `TestInProcessKill` | `Close = agentproc: agent killed: agentproc: in-process agent abandoned, want ErrKilled joined to Serve's io.ErrClosedPipe, and not ErrAbandoned` |
| the abandonment bound removed | `TestInProcessAbandon` | `Close took longer than 3.9s` |
| `ExitedFirst` recorded after the connection is closed | `TestInProcessReturnsFirst` | `ExitedFirst = false, want true: Serve returned while the connection was open` |
| the output tap left off the in-process reader | `TestInProcessTaps` | `the tap received 0 bytes and dropped 0 of the agent's 67108864` |

The step named the fifth break "`ExitedFirst` recorded after `Done`". It was planted as recorded after the connection's close,
which `Done` precedes, since a check moved only past `Done` races the client's reading of the output's end and would not fail every
time.

**Checks:**
* **Repeated runs**, `-run '^TestInProcess' -count=20`, built from the working tree:

  | Host | Result |
  | :--- | :--- |
  | macOS, `-race` | 240 passed, 0 failed |
  | the Linux host | 240 passed, 0 failed |
  | the Windows host | 240 passed, 0 failed |

* **`make pre-add-check`:** its first run found one lint issue, `inprocess.go:134:38: directive //nolint:errcheck // CloseWithError
  returns nil. is unused for linter "errcheck" (nolintlint)`. The directive was removed; then "240 file(s) clean".
  `scripts/go-precheck.sh` on the two new files: "2 file(s) clean".
* **`make api`:** "against v1.25.0: 0 incompatible, 2 compatible": `ErrAbandoned` and `InProcess`. `apidiff` reports a new type's
  methods with the type, so `(*InProcess).Start`, which this PLAN's Verification lists apart, is counted in `InProcess`.
* **`tools/gates.py --all`**, with both hosts set: all 18 passed, `newfiles`, `platforms`, `windows` and `linux` among them.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0. `tools/idscan.py --paths` over the staged files:
  nothing found.

### Phase 5, `Shutdown` and `ServeAgent` (2026-10-28)

The owner committed phase 4 as `a5ff41f` and asked for phase 5 ("committed proceed").

**What the code showed first.** A request handler's context derived from the connection's own context (`connection.go`, in
`receiveLine`). Ending it to stop the handlers would also end every write, which is why `Close` loses the answers (phase 2). And
`inboundRequests`, the wait group `drainInput` waits on, takes its `Add` outside any lock. A `Wait` on it while requests still
arrive would race that `Add`, so `Shutdown` does not use it.

**What changed:**
* **`errors.go`:** `ErrShutdown`, beside `ErrConnectionClosed`.
* **`connection.go`:**
  * `handlersCtx` and `handlersCancel`: a context between the connection's and each request's. Close still ends it, as its
    parent's child; `Shutdown` ends it alone, with `ErrShutdown`.
  * `shuttingDown`, `idle` and `idleOnce`:
    * `receiveLine` checks `shuttingDown` under `mu`, where it already registers each request's receipt in `inflightReceipts`,
      a null id's included;
    * a request received after it is refused there. Its context is released, its concurrency slot given back, and it is
      answered `-32603` with the data "the connection is shutting down";
    * the cleanup that removes a receipt after its answer is written calls `signalIdle`, which closes `idle` once shutting down
      and no receipt is left. So no `go` statement and no wait group are added.
  * `Shutdown(ctx)`:
    * it sets the flag, ends `handlersCtx` and the notification handlers' context with `ErrShutdown`, and waits for `idle`, the
      connection's end, or `ctx`;
    * then it closes through `closeWith(ErrShutdown)`, so `Err` wraps `ErrShutdown` beside `ErrConnectionClosed` and
      `EndedCleanly` is true;
    * it returns `context.Cause(ctx)` when `ctx` ended first, joined with the close's error.
* **`serve.go`, new:** `ServeAgent(ctx, agent, in, out, opts...)`, waiting on the caller's goroutine.
  * At the input's end it waits for the connection's goroutines through `WaitContext` within `notificationQueueDrainTimeout`,
    5 s. It returns nil when `EndedCleanly`, the error otherwise, or an error naming the handlers still running.
  * At `ctx`'s end it returns `context.Cause(ctx)` joined with `Shutdown`'s error, under the same bound.
* **`cmd/generate/engine_test.go`:** `TestRuntimeFilesCoverHandWritten` names `serve.go`, which uses ACP types, beside
  `helpers.go`.
* **`example/agent/main.go`:** its `NewAgentSideConnection`, `SetLogger` and `select` became one `ServeAgent` call, with the
  logger given at construction through `WithLogger`.
* **Tests:**
  * `shutdown_test.go`, new: the step's five, with a `shutdownAgent` and a `shutdownClient` embedding the refusals;
  * `agentproc/inprocess_test.go`: `TestInProcessServeAgentStep2`.

**Deviation, 2026-10-28, how phase 2's reproduction is rerun.** The step says phase 2's reproduction, "run against this phase,
returns `cancelled`". That probe's serve calls `Close`, which this phase leaves as it was by design, so run unchanged it still loses
the answer. Its scenario, served through `ServeAgent`, is `TestServeAgentContextEnd`, which returns `cancelled`. And the planted
break that makes `Shutdown` close at once fails with phase 2's own message, `input ended before a response`. The step's intent is
met, and is recorded here rather than asked about.

**Planted breaks**, each in its own copy through `tools/plants.py`, 5 of 5 as required:

| Break | Check | Failing line |
| :--- | :--- | :--- |
| `Shutdown` closing at once, as `Close` does | `TestShutdownAnswersRunningPrompt` | `Prompt = "", {"code":-32603,"message":"Internal error","data":{"error":"input ended before a response"}}; want cancelled, nil` |
| the late request left unanswered | `TestShutdownRefusesLateRequest` | `the late request = {"code":-32603,…"input ended before a response"}, want -32603 saying the connection is shutting down` |
| the handlers' context ended with no cause | `TestShutdownAnswersRunningPrompt` | `the handler's cause = context canceled, want ErrShutdown` |
| `ServeAgent` ignoring its context | `TestServeAgentContextEnd` | `the client's Prompt: nothing within 5 s` |
| a `go` statement added to wait for the requests | `TestGoStatementsCounted` | `connection.go:943:2: a go statement; start it with the connection's goroutines.Go` |

**Checks:**
* `go test -race` of the root, the examples, `agentproc` and `acptest`: ok. In `cmd/generate`, `TestRuntimeFilesCoverHandWritten`:
  ok. `TestGoStatementsCounted`, `TestSideConnectionMethodsAreProtocolMethods` and `TestExportedDeclarationsDocumented`: ok.
* **Repeated runs**, `-count=20`, built from the working tree. The root's pattern `^(TestShutdown|TestServeAgent)` also matches
  the existing `TestShutdownDrainsNotifications_WithBarrier`:

  | Host | The root's six | `TestInProcessServeAgentStep2` |
  | :--- | :--- | :--- |
  | macOS, `-race` | 120 passed, 0 failed | 20 passed, 0 failed |
  | the Linux host | 120 passed, 0 failed | 20 passed, 0 failed |
  | the Windows host | 120 passed, 0 failed | 20 passed, 0 failed |

* `make pre-add-check`: "242 file(s) clean". `scripts/go-precheck.sh serve.go shutdown_test.go`: "2 file(s) clean".
* `make api`: "against v1.25.0: 0 incompatible, 5 compatible": `(*Connection).Shutdown`, `ErrShutdown` and `ServeAgent`, with
  phase 4's `agentproc` `ErrAbandoned` and `InProcess`.
* `tools/gates.py --all`, with both hosts set: all 18 passed.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0. `tools/idscan.py --paths` over the staged files:
  nothing found.

### Phase 6, the stderr tail (2026-10-28)

The owner committed phase 5 as `5da0d0f` and asked for phase 6 ("committed, proceed").

**What changed:**
* **`agentproc/stderr.go`, new:**
  * `StderrError{Tail}`, whose `Error` reads "agentproc: the agent's standard error ended: " and the tail, without its last
    newline;
  * `tailWriter`, which holds the tail under a mutex:
    * `Write` keeps at most one byte more than the byte cap of each write, so that `trim` sees whether its cut falls mid-line;
      it forwards every byte to the caller's `Cmd.Stderr`, and returns that writer's result;
    * `trim` applies the line cap, counting an unended last line, then the byte cap, moving a mid-line cut to the next line
      unless the last line is all that is left; it lets go of the cut bytes once the buffer's capacity is twice what it holds.
* **`agentproc/agentproc.go`:**
  * `Command` gains `StderrTail`, `StderrTailLines` and `JoinStderrTail`, documented as the MADR's item 3 states them;
  * `Start`, with either cap set, wraps `Cmd.Stderr` in a `tailWriter` after its checks, beside its `Stdin` and `Stdout`;
  * `Process` gains `stderr` and `joinStderr`, and the exported `StderrTail()`;
  * `Wait` computes its error as before, then, under the join, a non-nil error with a non-empty tail has `&StderrError{tail}`
    joined. The tail is final there, since `Cmd.Wait` has returned and with it `os/exec`'s copy. `Wait`'s doc says so.
* **`agentproc/child_test.go`:** the child mode `stderr-fail` writes "line 1" to "line 100" to stderr and exits 1.
* **`agentproc/stderr_test.go`, new:** `TestTailWriterCaps`, ten cases; `TestTailWriterForwards`; `TestStderrTailOfFailedAgent`;
  `TestStderrTailNotJoined`; `TestStderrTailOfKilledAgent`, on the `stubborn` child; `TestInProcessHasNoStderrTail`.

**Deviation, 2026-10-28, a defect found by reading, and its break.** `Write` first trimmed each write to exactly the byte cap. The
tail of a write longer than the cap then began wherever the cut fell, since `trim` could no longer see the byte before it: a
mid-line start the MADR's design refuses. It was read in the code before the tests first ran, and fixed to keep one byte more. So
that the check is seen to fail on it, the old trimming is a fifth planted break below, beyond the step's four. The failing case is
`TestTailWriterCaps`' "the byte cap mid-line".

**Planted breaks**, each in its own copy through `tools/plants.py`, 5 of 5 as required, run again after the lint fix below:

| Break | Check | Failing line |
| :--- | :--- | :--- |
| the byte cap not applied to a long line | `TestTailWriterCaps` | `a line longer than the byte cap keeps its end: the tail is "0123456789\n", want "789\n"` |
| the line cap ignored | `TestTailWriterCaps` | `the line cap: the tail is "a\nb\nc\n", want "b\nc\n"` |
| `Cmd.Stderr` not forwarded | `TestTailWriterForwards` | `the caller's writer received "", want every byte` |
| the join made without the field | `TestStderrTailNotJoined` | `Wait = exit status 1` and the tail, `want no *StderrError without JoinStderrTail` |
| the write trimmed to the byte cap exactly | `TestTailWriterCaps` | `the byte cap mid-line, moved to the next line: the tail is "a\nbbbb\ncccc\n", want "bbbb\ncccc\n"` |

**Checks:**
* **Repeated runs**, `-run '^(TestTailWriter|TestStderrTail|TestInProcessHasNoStderrTail)' -count=20`, the child mode on every
  host:

  | Host | Result |
  | :--- | :--- |
  | macOS, `-race` | 120 passed, 0 failed |
  | the Linux host | 120 passed, 0 failed |
  | the Windows host | 120 passed, 0 failed |

  A first run of the two hosts wrote both logs to one file, a shell variable named wrong. Both passed, and they were run again
  into their own logs, with the results above.
* **`make pre-add-check`:** its first run found one lint issue, `stderr_test.go:114:5: errorsastype: errors.As can be simplified
  using AsType[*StderrError] (modernize)`. It was fixed; then "244 file(s) clean". `scripts/go-precheck.sh` on the two new files:
  "2 file(s) clean".
* **`make api`:** "against v1.25.0: 0 incompatible, 10 compatible". This phase's five are `(*Process).StderrTail`,
  `Command.JoinStderrTail`, `Command.StderrTail`, `Command.StderrTailLines` and `StderrError`, with its `Error` counted in the
  type.
* `tools/gates.py --all`, with both hosts set: all 18 passed.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0. `tools/idscan.py --paths` over the staged files:
  nothing found.

### Phase 7, transcripts, records and `Golden` (2026-10-28)

The owner committed phase 6 as `ad56dc2` and asked for phase 7 ("committed proceed").

**What changed:**
* **`agentproc/transcript.go`, new:** `NewTranscript(w)` and `Transcript`:
  * `Input` and `Output` are two `lineTap`s, the type `LineTap` returns, recording "sent" and "received";
  * each frame is written under a mutex as `{"direction":…,"frame":…}`, compact through `json.Marshal`, and a line that is not
    JSON as a JSON string;
  * `Close` drops either stream's unended line and returns the first write error, and does not close `w`.
* **`acptest/records.go`, new:**
  * `ReadRecords`, which skips blank lines and names the line of a bad record or an unknown direction;
  * `WriteRecords`, which writes a frame that is not JSON as a JSON string;
  * `Decode[T]`, the params of the records that went one way with one method, absent params read as `null`.
* **`acptest/golden.go`:**
  * `ValidateGolden` pairs in two passes. `requestsOf` first collects every request it could pass, by way and id, queued, so
    that a reused id pairs in order. Then each record is validated, and a response takes its request's method from that queue.
    The registrations inside `validateRecord` are gone.
  * `Golden(tb, path, records)` canonicalizes each frame with `canonical`, the form `Recorder` keeps, so that key order never
    changes a file. Equal, it passes. Under `ACPTEST_UPDATE=1` it writes `path`. Otherwise it writes the records to
    `tb.ArtifactDir()` and fails, naming the first differing record by `firstDiff`, or why `path` cannot be read. CRLF reads as
    LF.
* **Tests:**
  * `acptest/records_test.go`, new: `TestRecordsRoundTrip`, `TestReadRecordsRefuses`, `TestDecode`,
    `TestValidateGoldenResponseFirst`, `TestGolden`;
  * `agentproc/transcript_test.go`, new: `TestTranscriptLines`, `TestTranscriptWriteError`, and `TestTranscriptOfSession`. The
    last runs a real session: an `InProcess` agent served by `acp.ServeAgent`, which sends three updates and asks a permission,
    with a `Transcript` as both taps, read back, validated and decoded.

**Deviation, 2026-10-28, a hole found by reading, and its break.** The first pass at first kept every frame with a method and an id.
A notification sent as a request, or an unknown method, was then kept too. A response to it called that method's result check,
which a notification has none of, and panicked. `validateRecord` had refused such a request before registering it. `requestsOf`
now keeps only an extension method's request or one the way's table has with a result. `TestValidateGoldenResponseFirst` checks
it: such a pair is two errors, not a panic. A sixth planted break, beyond the step's five, removes the filter, and the test panics
with "invalid memory address or nil pointer dereference".

**Planted breaks**, each in its own copy through `tools/plants.py`, 6 of 6 as required, run again after the lint fix below:

| Break | Check | Failing line |
| :--- | :--- | :--- |
| the direction swapped in `Transcript` | `TestTranscriptLines` | `record 0 = received {"jsonrpc":"2.0","id":1,"method":"_x/a"}, want sent …` |
| a line split across writes recorded as two | `TestTranscriptLines` | `the transcript has 3 records, want 2` |
| `ValidateGolden` back to one pass | `TestValidateGoldenResponseFirst` | `ValidateGolden refused ["acptest: record 0, received: a response to no recorded request: id 1: …"], want a response recorded first, and a reused id, to pass` |
| `Golden` updating without the variable | `TestGolden` | `Golden = [], want record 1 named` |
| `Decode` skipping the direction | `TestDecode` | `Decode = [{… SessionId:a …} …], want the received updates of a and b` |
| `requestsOf` keeping a request `validateRecord` refuses | `TestValidateGoldenResponseFirst` | `panic: runtime error: invalid memory address or nil pointer dereference` |

**Checks:**
* **Repeated runs**, `-count=20`, built from the working tree. The `acptest` pattern also matches the existing
  `TestValidateGoldenRefuses`:

  | Host | `acptest`'s six | `agentproc`'s three |
  | :--- | :--- | :--- |
  | macOS, `-race` | 120 passed, 0 failed | 60 passed, 0 failed |
  | the Linux host | 120 passed, 0 failed | 60 passed, 0 failed |
  | the Windows host | 120 passed, 0 failed | 60 passed, 0 failed |

* **`make pre-add-check`:** its first run found two lint issues, `transcript.go:34:9` and `:35:10`, "Error return value is not
  checked (errcheck)": the type assertions of `LineTap`'s result to `io.WriteCloser`. `NewTranscript` now builds the two
  `*lineTap`s directly. The repeated runs above predate that change, which changes no behaviour; the gates' runs below cover the
  final code. Two planted breaks edited the changed lines, and all six were run again. Then "246 file(s) clean", and
  `scripts/go-precheck.sh` on the four new files, "4 file(s) clean".
* **The Transcript's doc** said it was safe for concurrent use. Each of `Input` and `Output` is for one stream, written from one
  goroutine, as a tap is, and the doc now says so.
* **`make api`:** "against v1.25.0: 0 incompatible, 16 compatible". This phase's six are `agentproc`'s `NewTranscript` and
  `Transcript`, its methods counted in the type, and `acptest`'s `Decode`, `Golden`, `ReadRecords` and `WriteRecords`.
* `tools/gates.py --all`, with both hosts set, on the final code: all 18 passed.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0. `tools/idscan.py --paths` over the staged files:
  nothing found.

### Phase 8, `ReplayAgent` and `ConnectReplay` (2026-10-28)

The owner committed phase 7 as `1bb1eeb` and asked for phase 8 ("committed proceed").

**What changed: `acptest/replay.go`, new.**
* **`planReplay`** reads the records once:
  * the Sent requests, by id, give each recorded client request its method and which of that method's requests it was;
  * the Received records become the steps: a notification, a request of the agent's own, or a response tagged with the method
    and position of the client request it answered;
  * a Received record that is not a JSON object, or a response to an id no Sent record asks, is an error, and `Serve` returns it.
* **`Serve`** starts one reader goroutine and plays the steps on its own:
  * The reader queues each live client request by method. A request beyond the recorded ones of its method is recorded as drift
    and answered `-32603`, "the replay has no recorded answer to" its method, written after the lock is released. The reader
    marks the agent's own requests answered, and notes an answer that is neither a result nor an error. It ends when its input
    does.
  * The player sends a notification at its turn. It sends an agent request under a fresh id, `"replay-1"` on, and waits for its
    answer. It sends a response once the client's request of the same method and position has arrived, under that request's
    live id.
  * `Serve` returns at the input's end: nil, or an error naming the extra requests, the bad answers, and the steps not played,
    each by kind and method. When its context ends first it returns the context's cause, and its reader ends when its input does,
    which the doc says.
  * The two goroutines' writes share a mutex.
* **`ConnectReplay`** runs `Serve` over two pipes, joins the client to it, and in the test's cleanup closes the connection and
  fails the test with `Serve`'s error, or when the replay has not ended within `cleanupWait`, 10 s.
* **`acptest/replay_test.go`, new:**
  * `TestReplayAgentPlays`, a real session recorded with `ConnectRecorded`: initialize, two sessions, a prompt of three updates
    and a permission request. It is replayed to a new client, which gets session-1 then session-2, the updates, and the request
    under `"replay-1"`, and `Serve` returns nil;
  * `TestReplayAgentLiveIDs`, a hand-written recording whose ids start at 100;
  * `TestReplayAgentExtraRequest`, `TestReplayAgentStopsEarly` and `TestReplayAgentUnordered`, the last with every Sent record
    after every Received one;
  * `TestConnectReplay`.

  Each passed on its first run under `-race`.

**A design point settled in the code, not a deviation.** The reader at first answered an extra request from a goroutine of its own,
to avoid writing under the replay's lock. It now decides under the lock, in `unrecorded`, and writes after releasing it, so the
replay starts no goroutine but its reader.

**Planted breaks**, each in its own copy through `tools/plants.py`, 4 of 4 as required, run again after the lint fix below:

| Break | Check | Failing line |
| :--- | :--- | :--- |
| the live id not substituted | `TestReplayAgentLiveIDs` | `Initialize = {"code":-32603,…"context deadline exceeded"}, want the recorded answer under the live id` |
| the k-th matching taken as the first | `TestReplayAgentPlays` | `NewSession = {"code":-32603,…"context deadline exceeded"}` |
| drift not reported | `TestReplayAgentExtraRequest` | `Serve = <nil>, want the extra request named` |
| the agent's request sent under its recorded id | `TestReplayAgentPlays` | `Prompt = {"code":-32603,…"context deadline exceeded"}` |

**Checks:**
* **Repeated runs**, `-run '^(TestReplayAgent|TestConnectReplay)' -count=20`:

  | Host | Result |
  | :--- | :--- |
  | macOS, `-race` | 120 passed, 0 failed |
  | the Linux host | 120 passed, 0 failed |
  | the Windows host | 120 passed, 0 failed |

* `go test -race -count=3 ./acptest`, which runs its leak guard: ok.
* **`make pre-add-check`:** its first run found one lint issue, `replay.go:330:3: mapsloop: Replace m[k]=v loop with maps.Copy
  (modernize)`. `withID` now uses `maps.Clone`. The planted breaks were run again, 4 of 4. Then "250 file(s) clean", and
  `scripts/go-precheck.sh` on the two new files, "2 file(s) clean".
* **`make api`:** "against v1.25.0: 0 incompatible, 18 compatible". This phase's two are `acptest`'s `ConnectReplay` and
  `ReplayAgent`, with `Serve` counted in the type.
* `tools/gates.py --all`, with both hosts set, on the final code: all 18 passed.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0. `tools/idscan.py --paths` over the staged files:
  nothing found.

### Phase 9, the docs (2026-10-28)

The owner committed phase 8 as `5828c46` and asked for phase 9 ("committed proceed").

**What changed:**
* **`README.md`:**
  * "Closing and limits" gains two paragraphs, one for `Connection.Shutdown`, beside `Close`, and one for `acp.ServeAgent`;
  * after `WaitContext`, three more: `InProcess` with its mapped steps, `ErrAbandoned` and the panic rule; the stderr tail; and
    `Transcript`;
  * "Testing with acptest" widens the `ValidateGolden` item to its two-pass pairing, and gains two items: records, `Decode` and
    `Golden`; and `ReplayAgent` with `ConnectReplay`.

  Each paragraph names its example and cites the MADR by item. No new code block is pinned to an example, since only the two
  `docs_test.go` pins today, and each needs a test of its own.
* **`docs/architecture.md`:**
  * `acptest`'s list: `ValidateGolden`'s two passes, `records.go` and `Golden` with the reason for its environment variable, and
    `replay.go`;
  * the root's hand-written files: `Shutdown` in `connection.go`, and `serve.go`;
  * the connection: a paragraph on `handlersCtx`, `shuttingDown`, `signalIdle` and `idle`, and why `Shutdown` does not wait on
    `inboundRequests`;
  * the agent process helper: the `agent` interface and its two implementations; the taps over any closer; `InProcess`; the
    stderr tail; and `Transcript`;
  * the test list and the tree.
* **The package doc** of `agentproc` already names the in-process agent, from phase 4.
* **Examples, each run by `go test`:**
  * `ExampleInProcess` and `ExampleTranscript` in `agentproc/example_test.go`;
  * `ExampleReplayAgent` in `acptest/example_test.go`;
  * `ExampleServeAgent`, new in `example_serve_test.go`. It has no output, since it serves the process's own standard input, so
    it is compiled, not run.

**A slip, put right before anything was checked.** The first write of `ExampleReplayAgent` went through a shell command whose
backquoted JSON the shell ran as commands. Two record lines were left garbled in `acptest/example_test.go`, which `go vet` refused at
`example_test.go:78:52: illegal character U+005C`. That file's only uncommitted change was the example, so its two lines were
written again with the Edit tool, and the file was not restored from git.

**Planted breaks**, one for each runnable example, each in its own copy through `tools/plants.py`, 3 of 3 as required:

| Break | Check | Failing line |
| :--- | :--- | :--- |
| `ServeAgent` failing a clean end | `ExampleInProcess` | `--- FAIL: ExampleInProcess`, its `close:` line an error |
| the `Transcript`'s directions swapped | `ExampleTranscript` | `index out of range`, no Sent `initialize` to decode |
| a replay reporting drift always | `ExampleReplayAgent` | `--- FAIL: ExampleReplayAgent`, its last line `replay: drift` |

**Checks:**
* `make markdownlint` and `make check-records`: exit 0.
* `go test` of the root, `agentproc` and `acptest`, `TestReadmeQuickstartIsTheExample` and `TestReadmeAgentprocIsTheExample`
  among them: ok.
* `make pre-add-check`: "252 file(s) clean". `scripts/go-precheck.sh example_serve_test.go`: "1 file(s) clean".
* `make api`: "against v1.25.0: 0 incompatible, 18 compatible", unchanged by this phase.
* `tools/gates.py --all`, with both hosts set: all 18 passed.
* `make disclosure-check`: exit 0. `tools/idscan.py --paths` over the staged files: nothing found.

### Phase 10, the implementation closed (2026-10-28)

The owner committed phase 9 as `2a053e9` and asked for phase 10 ("committed proceed").

**The docs, read in full against the code.** Every new name's doc was read as `go doc` renders it:
* the root's `Connection.Shutdown`, `ErrShutdown` and `ServeAgent`;
* `agentproc`'s `InProcess`, `ErrAbandoned`, `StderrError` and its `Error`, `Transcript` and its methods, `NewTranscript`,
  `Process.StderrTail`, `Command`, and the `Process`, `ErrTerminated` and `ErrKilled` docs phase 4 widened;
* `acptest`'s `ReadRecords`, `WriteRecords`, `Decode`, `Golden`, `ReplayAgent` and its `Serve`, `ConnectReplay` and
  `ValidateGolden`.

So was each passage phases 9 and 4 added to `README.md`, `docs/architecture.md` and the package doc. Two statements were wrong or
missing, and were corrected:

| Where | Was | Now |
| :--- | :--- | :--- |
| `StderrError.Error`'s doc | "Error returns the tail, its last newline left out", though it returns "agentproc: the agent's standard error ended: " and the tail | it says the agent's standard error ended so, and gives the tail |
| `ReplayAgent`'s doc | silent on records it cannot play | such records, a Received record that is not a JSON object or a response to an id no Sent record asks, make `Serve` return an error at once, before it reads or writes |

Everything else read matches the code.

**Deviation, 2026-10-28, two parts of the Confirmation not yet shown.** Walking the MADR's "Confirmation" found two parts of item 1
that no phase had shown. They are this phase's to close, since the step walks the Confirmation, so they were done here and not
asked about first:
* **"`TestDetachedTapIsPrefix` … repeated for an in-process agent."** Phase 4's `TestInProcessTaps` repeats
  `TestCloseReadsTheAgentsLastOutput`'s case, not a detached tap's. `TestInProcessDetachedTapIsPrefix`, new in
  `agentproc/inprocess_test.go`, does: `Serve` writes 56 MiB of `bulk64()` through a tap blocked until it is detached, waits for
  the test's word, and writes the last 8 MiB. The tap's bytes must be the stream's beginning, and with the dropped count the
  whole.
* **"The leak guard (F4) passing, and failing on a planted runner that leaves its watcher running."** It had passed in every phase,
  but no break had shown it failing for an in-process agent. The second break below does.

`docs/architecture.md`'s test list names the new test.

**Planted breaks**, each in its own copy through `tools/plants.py`, 2 of 2 as required:

| Break | Check | Failing line |
| :--- | :--- | :--- |
| a detached tap queuing again (`tap.offer` without its `detached` test) | `TestInProcessDetachedTapIsPrefix` | `the detached tap received 63963136 bytes that are not the stream's beginning` |
| the in-process watcher started outside `goroutines` and left blocked | the leak guard, under `TestInProcessEndsAtInputEnd` | `goroutines leaked by the tests …`, `goroutineleak profile: total 1`, in `(*InProcess).Start.func1` |

**The MADR's "Confirmation", item by item:**

| Item | Shown by |
| :--- | :--- |
| Item 1: each row of the mapping table, by a serve function of its kind | phase 4: `TestInProcessEndsAtInputEnd`, `TestInProcessTerminate`, `TestInProcessKill`, `TestInProcessAbandon`, `TestInProcessReturnsFirst`, with `TestInProcessKillAtOnce`, `TestInProcessClientWriteAfterReturn` and `TestInProcessEndClosesInput` |
| Item 1: `TestDetachedTapIsPrefix` and `TestCloseReadsTheAgentsLastOutput` repeated in-process | this phase's `TestInProcessDetachedTapIsPrefix`, and phase 4's `TestInProcessTaps` |
| Item 1: every existing `agentproc` test unchanged after A2, with the 100-run suites | phase 3: 300, 300 and 200 passing runs on macOS under `-race`, the Linux host and the Windows host |
| Item 1: the leak guard passing, and failing on a planted runner | every phase's runs; this phase's second break |
| Item 1: `make api`, `InProcess` with its `Start`, and `ErrAbandoned` | phase 4 |
| Item 1: `tools/consumer.py` running an in-process agent | not yet: phase 11, the release, extends `tools/consumer.py`, as this PLAN's step 11 says |
| Item 2: a prompt answered `cancelled` at `ServeAgent`'s context's end, its handler reading `ErrShutdown` | phase 5: `TestServeAgentContextEnd`, and `TestShutdownAnswersRunningPrompt` for the cause |
| Item 2: a late request answered `-32603`, the agent's own request still answered | phase 5: `TestShutdownRefusesLateRequest` |
| Item 2: `Shutdown` returning its context's cause past the bound | phase 5: `TestShutdownBound` |
| Item 2: `TestGoStatementsCounted` unchanged | phase 5, with a planted `go` statement |
| Item 2: an `InProcess` agent on `ServeAgent`, closed in step 2, answering `cancelled` | phase 5: `TestInProcessServeAgentStep2` |
| Item 2: the example agent on `ServeAgent` | phase 5 |
| Item 3: the caps, a long line's end, every byte forwarded | phase 6: `TestTailWriterCaps`, `TestTailWriterForwards` |
| Item 3: a failing agent's words in `StderrTail()` | phase 6: `TestStderrTailOfFailedAgent`, whose child writes 100 lines and exits 1 before any `initialize`, which an agent that fails it does in the same way |
| Item 3: the join through `errors.As`, `ErrKilled` kept, nothing joined without the field | phase 6: `TestStderrTailOfFailedAgent`, `TestStderrTailOfKilledAgent`, `TestStderrTailNotJoined` |
| Item 4: a real session's `Transcript` read back, validated and decoded | phase 7: `TestTranscriptOfSession` |
| Item 4: a response recorded before its request validating | phase 7: `TestValidateGoldenResponseFirst` |
| Item 4: `Golden`'s three paths | phase 7: `TestGolden` |
| Item 4: `ReplayAgent` playing a session, and reporting drift | phase 8: `TestReplayAgentPlays`, `TestReplayAgentExtraRequest`, `TestReplayAgentStopsEarly` |
| Item 4: an out-of-order transcript replaying | phase 8: `TestReplayAgentUnordered` |
| Every new check seen to fail on a planted break; `make api` only this record's names | 31 breaks across phases 4 to 10, each failing as required: 6, 5, 5, 6, 4, 3 and 2; `make api` below |

**`make api` against the newest `v1` tag, `v1.25.0`:** "0 incompatible, 18 compatible":
* **the root:** `(*Connection).Shutdown`, `ErrShutdown`, `ServeAgent`;
* **`agentproc`:** `(*Process).StderrTail`, `Command.JoinStderrTail`, `Command.StderrTail`, `Command.StderrTailLines`,
  `ErrAbandoned`, `InProcess`, `NewTranscript`, `StderrError`, `Transcript`;
* **`acptest`:** `ConnectReplay`, `Decode`, `Golden`, `ReadRecords`, `ReplayAgent`, `WriteRecords`.

A new type's methods are counted in it: `(*InProcess).Start`, `StderrError.Error`, `Transcript`'s three and `ReplayAgent.Serve`.
Nothing else in any package; `unstable`'s report: "0 incompatible".

**Checks:**
* `TestInProcessDetachedTapIsPrefix`, `-count=10`: 10 passed on macOS under `-race`, 10 on the Linux host, 10 on the Windows host.
* `make pre-add-check`: "253 file(s) clean".
* `tools/gates.py --all`, with both hosts set: all 18 passed.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0. `tools/idscan.py --paths` over the staged files:
  nothing found.

**The PLAN stays `in-progress`:** phase 11, the release, runs only on the owner's ask, and with it the Confirmation's last part,
`tools/consumer.py` running an in-process agent.

### Phase 11, the release `v1.26.0` (2026-10-28)

The owner committed phase 10 as `dd0d498` and asked for phase 11 ("committed proceed"). `make push-guard` over the outgoing
commits, `26a7242..dd0d498`, eleven: "the guard passed (exit 0)".

* **The release commit** changes records and `tools/` only:
  * [0042-REPORT-release-notes-v1-26-0.md](../reports/0042-REPORT-release-notes-v1-26-0.md), new, with its row in `docs/README.md`.
    It takes this record's number, as 0041's notes took theirs. Its lists were checked against the tree:
    * the twelve commits since `v1.25.0` and their files;
    * the 12 Go files changed outside the tests, and the unchanged areas;
    * the 18 additions `make api` lists, 3 in the root, 9 in `agentproc` and 6 in `acptest`.
    * Read against the `go doc` of the names it describes, three statements were corrected before it was staged: `Kill` keeps the
      abandonment's grace; `Transcript.Close` drops an unended line and does not close `w`; `TestGoStatementsCounted` is unchanged,
      not changed.
  * `tools/consumer.py`:
    * the program runs an `InProcess` agent whose `Serve` is `acp.ServeAgent`, a `Transcript` as both taps, initializes it, and
      prints the protocol version, `Close`'s error and `EndedCleanly`, then the transcript's `Close` and its line count;
    * it starts `sh -c "echo starting >&2; echo failed >&2; exit 2"` with `StderrTailLines: 1` and `JoinStderrTail`, and prints
      `StderrTail()` and whether `Wait`'s error holds a `*StderrError` with the same tail;
    * its new test, `TestTranscriptReplay`, records an in-process session in a `Transcript`, reads it with `ReadRecords`, checks it
      with `ValidateGolden`, decodes the `initialize` request with `Decode`, and replays it with `ConnectReplay`;
    * `VERSION` is `v1.26.0` or later, and `tools/README.md`'s row says so.
* **The consumer before the tag.** A scratch script ran the program and test against the tree, through a `replace`, at
  `go1.26.10`:
  * the output equals `WANT_RUN`, its new lines `in-process 1 <nil> <nil> true`, `transcript <nil> 2` and
    `stderr "failed\n" true`;
  * `go vet`, the test and the Windows build pass.
* **Planted breaks**, each in its own copy of the tree, through the same script, 3 of 3 as required:

  | Break | Broken |
  | :--- | :--- |
  | the stderr tail never joined (`Wait`'s join behind `false &&`) | `stderr "failed\n" false`; the output differs from `WANT_RUN` |
  | the transcript drops the client's frames (`Input`'s line tap writes nothing) | `transcript <nil> 1`; the output differs from `WANT_RUN`; `consumer_test.go:55: decoded 0 initialize requests from 1 records: <nil>` |
  | the test's expected count changed | `consumer_test.go:55: decoded 1 initialize requests from 2 records: <nil>` |

* **The Confirmation's last part.** Phase 10's row "`tools/consumer.py` running an in-process agent", "not yet", is shown by the
  program's `in-process` line and `TestTranscriptReplay` against the tree; after the tag, `tools/consumer.py v1.26.0` shows it from
  the proxy.
* **Checks**, each exit 0: `make check-tools`, its 31 tests "OK"; `make api` on `dd0d498`'s Go tree, "0 incompatible, 18
  compatible"; `make markdownlint`, "0 issues"; `make check-records`; `make disclosure-check`, "0 finding(s)"; `tools/idscan.py
  --paths` over the five changed files, "0 match(es)".

**Deviation, 2026-10-29, phase 10's new test failing in CI.** The owner committed the release commit as `d8badcb` and pushed it with
the eleven before it. Its CI run, `40812671975`, failed: `TestInProcessDetachedTapIsPrefix` failed in all four Linux and Windows
jobs, at Go `1.26.10` and `1.27.3`, at `inprocess_test.go:395`, with `Close = agentproc: agent terminated` in three and `Close =
agentproc: agent killed: io: read/write on closed pipe` in one. Both macOS jobs passed, and so did the Docs run, `40812671932`. The
test is new in phase 10, where it passed 10 times run alone on each of the three hosts; the failure is pre-existing in `dd0d498`.

* **Cause, measured in a scratch copy.** The test released its tap and closed `more` as soon as it saw the tap detached. That is
  when the connection has read about 33 of `Serve`'s first 56 MiB. So `Close`'s first step, 5 s, had to cover the rest of the
  first write and the last 8 MiB, about 31 MiB decoded under `-race`. On macOS that `Close` took 2.90 to 3.00 s in 5 runs of 5,
  60% of the grace; the CI runners are slower. The child test it repeats, `TestDetachedTapIsPrefix`, waits for its agent's ready
  after the 56 MiB first, so only 8 MiB remain. The library behaves as its docs say; the test's wait was missing.
* **Resolution, the owner's choice of two offered** (the other the same fix with 100 runs a host and a slowed reader): `Serve`
  closes a channel once its first `Write` returns, which an `io.Pipe` does once its reader has read all of it, and the test waits
  for it, within `readyWait`, before it waits for the detachment. With that, in the same scratch copy, `Close` took 0.83 to
  0.85 s in 5 runs of 5. Raising the test's grace, or removing the test, were not offered: each loosens what the test shows.
* **Files added to the phase's scope:** `agentproc/inprocess_test.go`, the test and its doc comment. No API, doc or MADR change: the
  MADR's "Confirmation" item stands, shown by the fixed test.
* **Checks, on the fix:**
  * `TestInProcessDetachedTapIsPrefix`, `-count=10`: 10 passed on macOS under `-race`, 10 on the Linux host, 10 on the Windows
    host;
  * `go test -race -shuffle=on -count=3 ./agentproc` with `GOMAXPROCS=2`: ok;
  * phase 10's planted break, a detached tap queuing again, through `tools/plants.py`, still fails as required:
    `inprocess_test.go:409: the detached tap received 41943040 bytes that are not the stream's beginning`;
  * `make pre-add-check`, "253 file(s) clean"; `tools/gates.py --all`, with both hosts set: all 18 passed.

Next: the owner commits and pushes the fix. On its CI, the release notes are updated to cite that run, to name the fix's commit as
the one whose API the release tags, and to count it among the commits; then the tag on the owner's ask.

### Phase 11, the fix's CI and the notes (2026-10-29)

The owner committed the fix as `8d9994a` and pushed it.
* **CI on `8d9994a`:** run `40884074796`, "completed success", its six jobs each "success", the four Linux and Windows jobs that
  failed on `d8badcb` among them; the Docs run `40884074976`, "completed success".
* **The release notes**, updated: observed also on `8d9994a`, the commit CI passed on, whose API is `dd0d498`'s; fourteen commits
  since `v1.25.0`, `d8badcb` and `8d9994a` among them; the 12 Go files outside the tests, as before; and a "CI" entry citing the
  two runs above and the failed run on `d8badcb`, with its cause and this deviation.
* **Checks**, each exit 0: `make api` on `8d9994a`, "0 incompatible, 18 compatible"; `make check-records`; `make markdownlint`;
  `make disclosure-check`; `tools/idscan.py --paths` over the two changed files, "0 match(es)".

Next: the annotated tag `v1.26.0` on the release-notes commit, and its push, on the owner's ask; CI on the tag; the GitHub release
from `tools/release_body.py`; `tools/publication.py`; then `tools/consumer.py v1.26.0`.

### Phase 11, the release published (2026-10-29)

The owner committed and pushed the notes as `59a43be`, then created and pushed the tag. Then:

* **CI:**
  * on `main` for `59a43be`, which changes docs-safe paths only: the Docs run `40885263186`, "completed success";
  * on the tag: run `40885851122`, six jobs, "completed success" on its first attempt.
* **The tag:** `v1.26.0` is annotated, its message `v1.26.0`, and names `59a43be` on `origin`.
* **`make api` after the tag:**
  * against `v1.26.0`, now the newest tag, "0 incompatible, 0 compatible";
  * against `v1.25.0`, through `scripts/check_api.py v1.25.0`, "0 incompatible, 18 compatible".
* **The body:** `tools/release_body.py v1.26.0 59a43be` extracted the report's "Release text". It is 103 lines with one link, which
  the tool points at the tag, and its identifier scan found 0 matches. The body was read in full.
* **The release:** the owner ran `gh release create v1.26.0 --verify-tag --title v1.26.0 --notes-file <the body>`.
* **`tools/publication.py v1.26.0 59a43be <the body>`:** "10 of 10 checks hold":
  * the release is published, marked latest, and its body is the extracted text;
  * CI holds on `main`'s head and on the tag;
  * the proxy serves the tag at `59a43be`;
  * `go list -m @latest` gives `v1.26.0`;
  * `go mod download` passes with the checksum database on.
* **`tools/consumer.py v1.26.0`:** "as required". At `go1.26.10`, from the proxy:
  * the program's output was as required, its last lines `in-process 1 <nil> <nil> true`, `transcript <nil> 2` and
    `stderr "failed\n" true`, the Confirmation's last part;
  * its `go test`, `TestTranscriptReplay` among it, passed;
  * the Windows build passed;
  * its `go.mod` kept `go 1.26.10` and gained no toolchain line. It requires `golang.org/x/sys v0.49.0` as indirect, as at
    `v1.25.0`, so the SDK gained no dependency.

The release is published and every item of the MADR's "Confirmation" is shown, so this PLAN and its row are `complete`. Each
consumer adopts `v1.26.0` under its own records, as "Rollout and Rollback" says.

### Amendment, 2026-10-30, after the publication of `v1.26.0`: phases 12 to 17 proposed, issue #12

**Found.** Issue #12: `Process.Close` waits without bound for a client handler that ignores its context, so `<cli-b>` cannot move its
runner to `InProcess`. The MADR's "Amended 2026-10-30, issue #12" holds the findings, I1 to I9, each read in the tree at `03b57e9`
or reproduced in a scratch copy:

* `Close`, and `Kill`, end in `Wait`, whose wait on the connection has no bound (I2); reproduced in process and with a child (I3);
* the README and two MADRs say "every wait bounded" (I4), and this record's C2 had found the bound `<cli-b>` keeps (I5);
* `ServeAgent`'s handler error is a text, and no test reaches it (I6);
* a prototype that bounds `Wait` broke `TestNoLostOutput` under load (I7); one that adds `CloseContext` and `KillContext` passed the
  whole package (I8);
* `Wait`'s error repeats its cause when the agent's error wraps it: three in-process cases, reproduced on the unmodified tree (I9).

**The owner's direction, 2026-10-30.** The issue's second option, a bounded close; this record amended; `errors.Is` for
`ServeAgent`'s handlers; the repeated message scoped and fixed. Phases 12 to 17 above carry it.

**Proposed.** Phases 12 to 17, their tests and planted breaks. Nothing is changed until the owner approves; this PLAN then returns to
`in-progress` until phase 17 closes.

**Approved (owner, 2026-10-30).** The owner committed the amendment as `9a3ff81` and answered "committed, proceed".

### Phase 12, the records (2026-10-30)

* The MADR's amendment `accepted`, its status line and the record's own status paragraph saying so.
* This PLAN `in-progress`; its row and the MADR's in `docs/README.md` updated.
* Its files ride with phase 13's, staged together, since this phase changes records alone.

Checks: `make check-records`, `make markdownlint`, `make disclosure-check` and `tools/idscan.py --paths`, run with phase 13's.

### Phase 13, `Wait`'s error said once (2026-10-30)

**Files:**
* `agentproc/agentproc.go`: `agentError`, factored out of `Wait`, which calls it under `mu` once it has set `returned`. One case is
  added, before the join: when `errors.Is(p.waitErr, p.ended)`, the agent's error is returned as it is. The stderr tail is joined
  after, as before. `Wait`'s doc says the sentinel is said once.
* `agentproc/inprocess_test.go`: `causeServe`, and `TestWaitErrorSaysTheCauseOnce`, six cases in parallel, each by its exact text and
  `errors.Is`:

  | Case | `Wait`'s error |
  | :--- | :--- |
  | `Kill`, `Serve` returning its cause | `agentproc: agent killed` |
  | step 2, `Serve` returning its cause | `agentproc: agent terminated` |
  | step 2, `Serve` wrapping its cause | `serve: agentproc: agent terminated` |
  | step 2, `Serve` returning its own error | `agentproc: agent terminated: serve's own error`, unchanged |
  | abandoned | `agentproc: agent killed: agentproc: in-process agent abandoned`, unchanged |
  | a child, `stubborn`, killed | begins with `agentproc: agent killed`, which it holds once, unchanged; the rest is the platform's |

**Run:** `go test -race -run '^TestWaitErrorSaysTheCauseOnce$' ./agentproc`, every case PASS.

**Planted break,** in a scratch copy: the added case made `false && errors.Is(…)`. The test failed as required, on the three cases
whose `Serve` wraps the sentinel and on no other:

```text
inprocess_test.go:236: Wait's error = agentproc: agent killed: agentproc: agent killed, want "agentproc: agent killed", and errors.Is agentproc: agent killed
inprocess_test.go:236: Wait's error = agentproc: agent terminated: agentproc: agent terminated, want "agentproc: agent terminated", and errors.Is agentproc: agent terminated
inprocess_test.go:236: Wait's error = agentproc: agent terminated: serve: agentproc: agent terminated, want "serve: agentproc: agent terminated", and errors.Is agentproc: agent terminated
```

**Gates,** `tools/gates.py --all`, the hosts named per command: "all gates passed", 18 of 18; among them `make pre-add-check`, "276
file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; `make api`, "against v1.27.0: 0 incompatible, 0 compatible";
`platforms`, "13 of 13 passed"; the floor gate at go1.26.10, passed; `windows` and `linux`, passed.

### Phase 14, `acp.ErrHandlersRunning` and `ServeAgent` (2026-10-30)

The owner committed phases 12 and 13 as `1457073` and answered "committed, proceed".

**Files:**
* `errors.go`: `ErrHandlersRunning`, "acp: the connection's handlers still run", beside `ErrShutdown`. Its doc names `ServeAgent`
  alone; phase 15 adds the `agentproc` methods to it once they exist.
* `serve.go`:
  * at the input's end, `fmt.Errorf("%w %v after its end", ErrHandlersRunning, notificationQueueDrainTimeout)`, whose text is the
    one it had;
  * at the context's end, `Shutdown`'s own error, when it wraps `context.DeadlineExceeded`, wrapped as
    `fmt.Errorf("%w %v after the context's end: %w", …)`, then joined after `context.Cause(ctx)` as before;
  * the doc names the sentinel at both ends.
* `shutdown_test.go`: `TestServeAgentHandlersStillRun`, two cases, each in a `synctest` bubble with a prompt handler that runs until
  released:
  * "the input's end": the error wraps `ErrHandlersRunning`, its text `acp: the connection's handlers still run 5s after its end`;
  * "the context's end": the error wraps the context's cause, `ErrHandlersRunning` and `context.DeadlineExceeded`, and says
    "acp: the connection's handlers still run 5s after the context's end".

  Each reads `ServeAgent`'s result from its channel without a timer, so a hang is the bubble's deadlock, and releases its handler
  before the bubble ends. Both run in about 10 ms, the 5 s bounds passing on the bubble's clock.

**Run:** `go test -race -run '^(TestServeAgentHandlersStillRun|TestServeAgentInputEnd|TestServeAgentContextEnd)$' .`, all PASS;
`go test -race -count=20 -run '^TestServeAgent' .`, exit 0.

**Planted breaks,** each in its own scratch copy; 2 of 2 failed as required:

| Break | The check that failed |
| :--- | :--- |
| `%w` made `%v` at the input's end | "the input's end": `ServeAgent = acp: the connection's handlers still run 5s after its end, want "acp: the connection's handlers still run 5s after its end", wrapping ErrHandlersRunning`: the same text, no longer testable |
| the context's-end wrapping disabled | "the context's end": `ServeAgent = the test's signal` and `Shutdown`'s deadline, without the sentinel |

**Gates,** `tools/gates.py --all`, the hosts named per command: "all gates passed", 18 of 18; among them `make pre-add-check`, "276
file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; `make api`, "compatible: ErrHandlersRunning: added", "against
v1.27.0: 0 incompatible, 1 compatible"; `platforms`, "13 of 13 passed"; the floor gate at go1.26.10, passed; `windows` and `linux`,
passed.

### Phase 15, `CloseContext` and `KillContext` (2026-10-30)

The owner committed phase 14 as `1b61f33` and answered "committed, proceed".

**Files:**
* `agentproc/agentproc.go`:
  * `hurryUp`, the close of `hurry` that `Kill` made inline, now a method that `Kill`, `KillContext` and `CloseContext`'s
    `context.AfterFunc` call;
  * `waitDone`, factored out of `WaitContext` as `Connection`'s is, and `endedFirst`, `WaitContext`'s select, which reports whether
    the context ended first, `Wait`'s return winning when both are ready. `WaitContext` is the two, its behaviour unchanged;
  * `connWaitBegan` and `connWaitEnded`, two `atomic.Bool`s that `Wait` sets around its `p.conn.Wait()`, after the watcher and the
    monitor have returned, so after the watcher's close of the connection;
  * `CloseContext`: `context.AfterFunc(ctx, p.hurryUp)`, `p.closeOnce.Do(p.stop)`, the `AfterFunc` stopped, then `waitBounded`.
    `KillContext`: `p.hurryUp()`, the steps, `waitBounded`;
  * `waitBounded`: `Wait`'s error when `endedFirst` says `Wait` returned; otherwise `errors.Join` of `context.Cause(ctx)`, of
    `acp.ErrHandlersRunning` when `connWaitBegan` is set and `connWaitEnded` is not, read in that order, and of `p.agentError()`
    under `mu`;
  * the docs: `Wait`'s gains the specification's paragraph, its unbounded wait for the connection's goroutines and the methods that
    bound it; `Close`'s and `Kill`'s each point to its bounded form; `CloseContext`'s and `KillContext`'s are the specification's,
    `CloseContext`'s citing the amendment; `WaitContext`'s says that the first call of the three starts the goroutine they share;
    the `hurry` and `waitOnce` fields name their new callers.
* `errors.go`: `ErrHandlersRunning`'s doc names `agentproc`'s `CloseContext` and `KillContext`, as the specification gives it.
* `agentproc/example_test.go`: `ExampleProcess_CloseContext`, a `ScriptAgent` served in process and closed under a 5 s bound, which
  tests `acp.ErrHandlersRunning` with `errors.Is`, and names `context.WithTimeoutCause` in a comment; `// Output: close: <nil>`.
* `agentproc/closecontext_test.go`, new: `hungClient`, whose `SessionUpdate` ignores its context until released; `hang`, which
  registers the release to run before the agent's `Close` and returns once the handler is blocked; `hungInProcess`, a `ScriptAgent`
  whose turn sends one update, prompted, as I3 reproduced it; `hungChild`, the `burst` mode. The seven tests:

  | Test | What it requires |
  | :--- | :--- |
  | `TestCloseContextBoundsHandlers` | in process and a child, by `CloseContext` and `KillContext`, a 300 ms bound: the call returns within the bound, a grace and the slack; `errors.Is` holds for `context.DeadlineExceeded` and `acp.ErrHandlersRunning`; `Done` is closed; released, `Wait` returns `nil`, or `ErrKilled` after `KillContext`, and the bounded error is exactly the cause, the sentinel and that error, joined in that order |
  | `TestCloseContextJoinsTheAgentsError` | a `Serve` that returns `errServe` after `ServeAgent`: the error is exactly the cause, the sentinel and `errServe` |
  | `TestCloseContextWaitsForHandlers` | a handler released half a grace into a 5 s bound: `nil`, and the handler had returned |
  | `TestCloseContextHurriesTheSteps` | a `stubborn` child under a 5 s grace, a 300 ms bound: `CloseContext` from 300 ms to 3.3 s, `KillContext` under 3.3 s, each `ErrKilled` and not `ErrTerminated` |
  | `TestCloseContextCause` | `context.WithTimeoutCause`: the test's cause, not `context.DeadlineExceeded`, and the sentinel |
  | `TestCloseContextNotHandlers` | a `burst` child that has exited 0, its connection ended cleanly and its goroutines returned, an output tap held by its gate, a 2 s grace, a 100 ms bound: the error is the cause alone, without the sentinel; released, `Wait` returns `nil` |
  | `TestCloseContextOneGoroutine` | 50 calls each of `CloseContext` and `KillContext` with an ended context leave one waiting goroutine, and none once the handler is released |

**Deviation, 2026-10-30: `agentproc/waitcontext_test.go` changed, a file the phase did not list.** `processWaiters`, which
`TestProcessWaitContext` and now `TestCloseContextOneGoroutine` count with, matched the waiter by its function's name,
`(*Process).WaitContext.func`. `waitDone`, which the phase factors out, renames that function `(*Process).waitDone.func`, and the
match would find none: `TestProcessWaitContext`'s "one goroutine" would fail, "left 0 waiting goroutines, want 1". The match now
names `waitDone`, and the doc says the waiter is shared. Nothing the test requires changed; it passed after the change, as before.

**Not done, and why:** the "a child, KillContext" case can log the connection's `WARN failed to parse incoming message`: the kill can
land while the `burst` child is still writing, and the connection reads a partial last line. That is a killed writer's output, as
`Kill` gives it today; `start`, which that case shares with `TestNoLostOutput`, keeps the connection's default log, and no check reads
it.

**Run:** `go test -race -count=1 ./agentproc`, the whole package with its leak guard: `ok`. `go test -race -count=20 -run
'^(TestCloseContext.*|TestProcessWaitContext|ExampleProcess_CloseContext)$' ./agentproc`: exit 0. The same pattern, `-test.count=20`,
through `tools/remote.py pkg` on the Linux host and the Windows host, its binary cross-compiled without `-race`: exit 0 on each.

**Planted breaks,** each in its own scratch copy; 7 of 7 failed as required:

| Break | The check that failed |
| :--- | :--- |
| `CloseContext` calling `Close` | `TestCloseContextBoundsHandlers`: "CloseContext took longer than 3.6s" |
| the `AfterFunc` removed | `TestCloseContextHurriesTheSteps`: "CloseContext took 10.004506875s, want from 300ms to the bound and the slack, 3.3s, well within one grace, 5s" |
| `KillContext` without `hurryUp` | `TestCloseContextHurriesTheSteps`: "KillContext took 10.003439375s, …" |
| `agentError` not joined | `TestCloseContextJoinsTheAgentsError`: "CloseContext = context deadline exceeded", the first line of the joined error, and the message's "Serve's own error joined" |
| the sentinel joined at any context's end | `TestCloseContextNotHandlers`: "want the bound's cause alone, without acp.ErrHandlersRunning" |
| the sentinel never joined | `TestCloseContextBoundsHandlers`, its four cases: "CloseContext = context deadline exceeded after 300.434083ms, want the bound's cause and acp.ErrHandlersRunning" |
| a goroutine per call, `waitDone` without its `sync.Once` | `TestCloseContextOneGoroutine`: "100 calls of CloseContext and KillContext left 100 waiting goroutines, want 1" |

**Gates,** `tools/gates.py --all`, the hosts named per command: "all gates passed", 18 of 18; among them `make pre-add-check`, "276
file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; `make api`, "compatible: ErrHandlersRunning: added",
"compatible: agentproc: (*Process).CloseContext: added", "compatible: agentproc: (*Process).KillContext: added", "against v1.27.0: 0
incompatible, 3 compatible", the count phase 17 expects; `platforms`, "13 of 13 passed"; the floor gate at go1.26.10, passed;
`windows` and `linux`, passed. `WaitContext`'s doc changed after the gates, a comment alone; `make pre-add-check` then, "276 file(s)
clean (gofmt, golangci-lint, go vet, go test, govulncheck)", and `scripts/go-precheck.sh agentproc/closecontext_test.go`, "1 file(s)
clean", each run with the active toolchain's `bin` first on `PATH`, since the host's `gofmt` link had come to name a toolchain no
longer installed.

### Phase 16, the docs (2026-10-30)

The owner committed phase 15 as `82f421b` and answered "committed, proceed".

**Files:**
* `README.md`:
  * `ServeAgent`'s paragraph: its error wraps `acp.ErrHandlersRunning` when the 5 seconds pass with handlers still running, at
    either end;
  * the `agentproc` paragraph's "every wait bounded" now reads "every wait for the agent bounded", and says that `CloseContext` and
    `KillContext` bound the wait for the client's own handlers;
  * a paragraph after `WaitContext`'s: `Close`, `Kill` and `Wait` wait for the connection's handlers without bound; what
    `CloseContext` and `KillContext` do at a context's end during the steps and after them; the agent ended by then, and `Wait`'s
    whole error once the handler has returned;
  * `InProcess`'s paragraph: a `Serve` that returns its context's cause has its error returned as it is, the sentinel not joined a
    second time.
* `docs/architecture.md`:
  * `serve.go`'s entry: `ServeAgent`'s error and `ErrHandlersRunning` at both ends;
  * the `agentproc` section: `hurryUp`; `Wait` returning `agentError`, the sentinel said once, final once `Done` is closed;
    `waitDone`; a new paragraph on the unbounded wait, `CloseContext`'s `AfterFunc`, `waitBounded`, `endedFirst`, the two flags, and
    why the output's read and the taps' drain join no sentinel;
  * the checks: `InProcess`'s count, eleven with `TestInProcessServeAgentStep2` beside them, is now thirteen, which names
    `TestWaitErrorSaysTheCauseOnce`, phase 13's, which the list had not yet named; and the seven tests of `closecontext_test.go`.
    The counts were taken from the files: 13 in `inprocess_test.go` (12 at `9a3ff81`), 6 in `stderr_test.go`, 3 in
    `transcript_test.go`, 7 in `closecontext_test.go`.

**Each claim and what makes it:**

| Claim | Made by |
| :--- | :--- |
| `ServeAgent`'s error wraps `acp.ErrHandlersRunning` at either end | `TestServeAgentHandlersStillRun`, its two cases (phase 14) |
| `Close`, `Kill` and `Wait` wait for a hung handler without bound | the MADR's I3, reproduced; and phase 15's planted break "`CloseContext` calling `Close`", which held past 3.6 s. No test waits for ever, so none pins it |
| a context that ends during the steps kills at once, without SIGTERM | `TestCloseContextHurriesTheSteps` |
| at the context's end: its cause, `acp.ErrHandlersRunning` while a handler runs, the agent's error | `TestCloseContextBoundsHandlers`, `TestCloseContextJoinsTheAgentsError`, `TestCloseContextCause` |
| no sentinel while the taps drain | `TestCloseContextNotHandlers` |
| the agent ended, `Done` closed, and `Wait`'s whole error once the handler returns | `TestCloseContextBoundsHandlers`, after the release |
| one goroutine shared by `WaitContext`, `CloseContext` and `KillContext` | `TestCloseContextOneGoroutine`, `TestProcessWaitContext` |
| a handler that returns within the bound is waited for | `TestCloseContextWaitsForHandlers` |
| a `Serve` returning its context's cause: its error as it is | `TestWaitErrorSaysTheCauseOnce` (phase 13) |
| the README's bounded-close example | `ExampleProcess_CloseContext`, by its `// Output:` |

Two claims in `docs/architecture.md` describe the code without a test of their own: `Wait`'s return winning when it and the
context's end are both ready, `WaitContext`'s rule since 0041, which no test can make simultaneous; and a context that ends after
`CloseContext`'s steps changing nothing in the process, since the `AfterFunc` is stopped by then and the agent has ended, so nothing
observable is left for it to change. Each was read against `agentproc/agentproc.go` at `82f421b`.

**Doc comments:** phase 15's, `Wait`'s, `WaitContext`'s, `Close`'s, `CloseContext`'s, `Kill`'s, `KillContext`'s, `hurryUp`'s,
`waitBounded`'s, `endedFirst`'s, `waitDone`'s and `ErrHandlersRunning`'s, read in full against the code; none changed in this phase.

**Checks:** `make check-records`, `make markdownlint` and `make disclosure-check`, each exit 0; `tools/idscan.py --paths README.md
docs/architecture.md`, "2 file(s), 17 pattern(s), 0 match(es)"; `go test -count=1 -run 'Readme|Doc|Example' . ./agentproc`, which
runs `TestReadmeAgentprocIsTheExample` and the examples, `ok` for both packages. No Go file changed, so `make pre-add-check` and the
gate set wait for phase 17, which runs them all.

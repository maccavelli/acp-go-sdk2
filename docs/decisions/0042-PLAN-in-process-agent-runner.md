---
status: in-progress
date: 2026-10-28
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

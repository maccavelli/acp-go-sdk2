---
status: in-progress
date: 2026-10-27
associated-madr: "0041-MADR-close-the-gaps-the-consumers-wait-on.md"
---

# Implement 0041: client hosts, end causes, a fuller acptest, a tool-call timeline, and agentproc's lost output

Associated MADR: [0041-MADR-close-the-gaps-the-consumers-wait-on.md](0041-MADR-close-the-gaps-the-consumers-wait-on.md)

## Goal

The MADR's ten items ship, on the schema `1.25.0` pin, with no new dependency:

1. `agentproc` reads the agent's output to its end before `Close` closes the connection;
2. `clienthost.Terminals`;
3. `clienthost.Files`, and `IsResourceNotFound`;
4. `Negotiation.AllowsValue`;
5. `agentproc.LineTap`;
6. `WaitContext` on the connections and on `agentproc.Process`;
7. `EndedCleanly`, and `Process.ExitedFirst`;
8. `acptest`'s additions;
9. `ToolCallTimeline`;
10. `Kind()` on the six stable unions without it and on `PlanUpdateContent`.

Then the adoption report, for both consumers. `make api` lists only compatible changes, and `clienthost` as a new package.

**Answers.** The owner chose the MADR's recommended answer to each of its questions 1 to 6 on 2026-10-27 (the MADR's "Decided
with the owner, 2026-10-27"). This PLAN was written for those answers. Before they were given, it named the phases each answer
would change:
* question 1 changes phases 7 to 9;
* question 2 changes phase 6;
* question 3 changes phase 5's second half;
* question 4 changes phase 11;
* question 5 changes phase 12;
* question 6 changes phase 17 and the rollout.

## Prerequisites

* [0040-PLAN-check-elicitation-forms-answers-and-urls.md](0040-PLAN-check-elicitation-forms-answers-and-urls.md) is `complete`
  (`v1.24.0`): phase 13's `MemoryClient` answers elicitations through its helpers.
* The MADR is `accepted`, with its questions answered.

## Scope

* **`agentproc/`:** `agentproc.go` (`stop`, `ExitedFirst`, `WaitContext`), `tap.go` or a new `linetap.go` (`LineTap`), and the
  tree files, `tree_unix.go`, `tree_windows.go`, `tree_other.go` and `deathsig_*.go`, whose process-tree code moves to
  `internal/proctree/` (phase 7); their tests.
* **`internal/proctree/`, new:** the process-tree code `agentproc` and `clienthost` share, with no exported change to `agentproc`.
* **`clienthost/`, new:** `doc.go`, `terminals.go`, `files.go`, their tests.
* **Root, hand-written:**
  * `connection.go` (`WaitContext`, `EndedCleanly`);
  * `errors.go` (`IsResourceNotFound`);
  * `helpers.go` (`AllowsValue`, the `Kind()` methods);
  * `tooltimeline.go`, new (`ToolCallTimeline`), which `TestRuntimeFilesCoverHandWritten` names.
* **Generated:** none. `WaitContext` on the side connections is hand-written beside them if the generator owns no part of it;
  otherwise phase 6 is amended first. (Phase 6: the side connections take no `WaitContext`; see its deviation.)
* **`acptest/`:** `pair.go`, `recorder.go`, `script.go`, `client.go`, a new `golden.go`.
* **Scripts:** `scripts/check_api.py`'s `PACKAGES` gains `clienthost` (phase 8), with `scripts/test_check_api.py`.
* **Records:** this pair and their rows; `docs/reports/0041-REPORT-consumer-adoption-of-v1-21-to-v1-25.md` (phase 15); the
  release-notes report.
* **Docs:** `README.md` (each item's section), `docs/architecture.md` (the packages, `internal/proctree`, the tree list).
* **Not in scope:** the MADR's "Not chosen"; any change in either consumer's repository; schema `1.26.0`.

## Implementation Steps

Each phase ends with its checks, an entry in the execution record, and its files staged for the owner, who commits on `main`
(`AGENTS.md`, "Commits"). Every new check is seen to fail on a planted break, each in its own copy of the tree, and the record keeps
each break's failing line.

Every code phase's gates:

* `make pre-add-check`, and `scripts/go-precheck.sh` on each new file, since `gofmt` there covers tracked files only;
* `make check-records`, `make markdownlint`, `make disclosure-check`, `make check-tools` and `make api`;
* `tools/gates.py --all`, with `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` set; `tools/platforms.py` for every `agentproc` and
  `internal/proctree` change;
* `tools/idscan.py --paths` over the staged files.

### 1. Records

* This pair and their rows in `docs/README.md`, and an "I want to…" row.
* The owner answers the MADR's questions. The MADR records them and becomes `accepted`; this PLAN becomes `in-progress` on approval.
* `0026-MADR-connection-end-cause-and-wait.md` and `0027-MADR-agent-process-helper.md` each gain an amendment that points to the
  MADR, for the decisions question 2 reopens and question 3 amends.

### 2. Reproduce the lost output

Read-only on the tree; the experiment runs in a scratch copy.

* In the copy, `agentproc`'s connection reader is held back, for example by a 50 ms pause before the `tapReader`'s first read after
  the agent's exit, and `TestDetachedTapIsPrefix` is run 20 times.
* **Expected:** it fails every time, with the bytes neither received nor counted. That confirms the MADR's cause: `stop` closing
  the connection before the reader reaches the end of the output. (Deviation, 2026-10-27: the single pause never failed; a pause
  before every read from `Close`'s start failed 37 of 40 runs, and the owner accepted that. See the execution record.)
* **If it does not fail so,** the MADR's cause is wrong. The PLAN stops, and the owner decides before phase 3.

The record keeps the scratch script and its failing lines.

### 3. `agentproc` reads the output to its end (MADR item 1)

* `agentproc.go`'s `stop`: in the case where the agent has exited within the grace, or after `SIGTERM`, it waits for
  `p.conn.Done()` or the rest of the grace, from the grace's start, before `p.conn.Close()`. After a kill it does the same, since
  the kill ends the agent's output too. With `hurry` closed, by `Kill`, it closes at once. (Deviation, 2026-10-27: the owner
  chose the grace from the agent's exit, the watcher's rule, over the rest of a step's grace. `stop` no longer closes the
  connection; the watcher does, and its wait also ends at `hurry`. See the execution record.)
* `stop`'s comment and `Close`'s doc say what it waits for.
* Tests:
  * `TestCloseReadsTheAgentsLastOutput`, new: a child mode that writes a known number of lines, more than a pipe buffer, and exits
    without reading its input. `Close` is called at once, and the connection has every line, read through `OutputTap`, with
    nothing dropped. (Deviation, 2026-10-27, from phase 2: the test holds the connection's reader back, 5 ms before each read
    from the start of `Close`'s steps, through an unexported `Command` field. Only the package's tests set it, and `tapReader`
    calls it. Without it, a reader that keeps up hides the loss.)
  * `TestDetachedTapIsPrefix`, unchanged, run 100 times on macOS and on the Linux and Windows hosts.
  * `TestKill`'s cases: `Kill` still does not wait.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| `stop` closes at once, as before | the new test, at `-count=20`; `TestDetachedTapIsPrefix` with phase 2's slow reader, at `-count=20` |
| the wait ignores `hurry` | ~~`TestKillCutsCloseShort`~~ `TestKillClosesAtOnce`, new, on Unix (deviation, 2026-10-27: `TestKillCutsCloseShort`'s agent leaves no process holding its output, so it cannot tell) |

### 4. `agentproc.LineTap` (MADR item 5)

* `LineTap(fn func(frame []byte)) io.Writer`, in a new `agentproc/linetap.go`:
  * calls `fn` with each line without its newline, a copy;
  * holds an unended line;
  * is safe for one writer, as a tap is;
  * its `Close`, through `io.Closer`, does not flush an unended line, which is not a frame.
* Tests: split and joined writes; a copy changed by `fn` leaves the next frame alone; an unended line held; `\r\n` kept as given.
  A tap used by `Command` delivers frames, the agent's lines.

Planted breaks: a frame aliasing the buffer; an unended line delivered.

### 5. End causes (MADR item 7)

* `connection.go`: `EndedCleanly(err error) bool`.
  * True: `ErrConnectionClosed` not wrapping `ErrNotificationQueueOverflow`; `ErrPeerDisconnected` wrapping `io.EOF`.
  * False: anything else, nil included.
* `agentproc.go`: `ExitedFirst() bool`. `watch` and the connection's end each record their order under `p.mu`. It is final once
  `Wait` has returned, and false before.
* Tests:
  * `EndedCleanly` over each end `connection_end_test.go` produces;
  * `ExitedFirst` true for an agent that exits and leaves its output open to a grandchild until after its exit, and false for one
    whose output ends before it exits; each run 50 times.

Planted breaks: overflow counted as clean; the order recorded the wrong way round.

### 6. `WaitContext` (MADR item 6)

* `connection.go`: `Connection.WaitContext(ctx) error`. The first call starts one goroutine that runs `Wait` and closes a channel;
  every call selects on that channel and `ctx.Done()`. The goroutine is counted by nothing the connection waits for, and ends when
  `Wait` returns, as `Wait` does once the connection has ended.
* ~~The side connections and~~ `agentproc.Process` gains the same, over its `Wait`. (Deviation, 2026-10-27: a side connection's is
  its `Connection()`'s, and `TestGoStatementsCounted` names the one waiter. See the execution record.)
* `acptest.Connect`'s cleanup uses it with a bound, 10 s, and fails the test with what is still running when it passes.
* Tests:
  * the cause returned when ctx ends before `Wait` would;
  * `Wait`'s error otherwise;
  * many calls start one goroutine;
  * the root's leak guard passes once the connection is closed.

Planted breaks: a goroutine per call; the cause lost.

### 7. `internal/proctree`, the tree code shared

* The process-tree code of `agentproc`'s tree and death-signal files moves, unchanged, into `internal/proctree`. `agentproc`
  calls it, and its exported API, behaviour and tests are unchanged.
* Checks: `git diff` shows the moved functions identical, by a scratch script comparing their token streams; `agentproc`'s tests on
  macOS and on the Linux and Windows hosts; `tools/platforms.py`, all 13.
* No planted break: the move adds no check. Its proof is the unchanged tests passing on every platform. (Deviation, 2026-10-27:
  `tools/platforms.py` is widened to `./internal/...`, which is a new check, and is planted. See the execution record.)

### 8. `clienthost.Terminals` (MADR item 2)

* `clienthost/doc.go`, `terminals.go`: `Terminals`, with a `Command func(ctx, acp.CreateTerminalRequest) (*exec.Cmd, error)`
  field, the five methods, and `Close`.
  * Output kept to `outputByteLimit`, dropped from the front at a character boundary, `truncated` set.
  * The exit status as `exitCode` or `signal`.
  * Kill and release end the tree through `internal/proctree`.
  * An unknown id is `NewResourceNotFound`.
* `scripts/check_api.py`'s `PACKAGES` gains `clienthost`; `scripts/test_check_api.py` asserts it.
* Tests:
  * truncation across a 2-, 3- and 4-byte character at the limit, the output valid UTF-8 every time;
  * exit codes, and a signal on Unix;
  * a child's own child ended by kill and by release;
  * `WaitForTerminalExit` under a context that ends;
  * an unknown id;
  * `Close` ending every terminal, under the leak guard;
  * a client embedding `Terminals` and ~~`ClientRefusals`~~ `clienthost.Refusals`, served over `acptest.Connect`. (Deviation,
    2026-10-27: the two at one depth do not compile. See the execution record.)

Planted breaks: truncation at a byte; the tree not ended; an unknown id `-32603`.

### 9. `clienthost.Files` and `IsResourceNotFound` (MADR item 3)

* `clienthost/files.go`: `Files`, with `ReadTextFile` and `WriteTextFile`.
  * `line` 1-based, and `limit` as the schema defines them; a `line` past the end gives no content.
  * A missing file is `NewResourceNotFound(path)`.
  * The `MkdirAll` option creates parent directories.
* `errors.go`: `IsResourceNotFound(err)`, a peer's `-32002`.
* Tests:
  * `line` and `limit` at 1, at the end and past it, `limit` 0, and absent;
  * CRLF files kept as they are;
  * a missing file `-32002`, and `IsResourceNotFound` of it over a pair;
  * a write, with and without the option, to a missing directory.

Planted breaks: `line` read as 0-based; a missing file `-32603`; parent directories always created.

### 10. `Negotiation.AllowsValue` (MADR item 4)

* `helpers.go`: `AllowsValue(method, params)`, over the generated gates of `types_gen.go`, as `Allows` is over the method gates.
* Tests: for each of the protocol table's value rules, `AllowsValue` returns the same `*CapabilityError` that sending the value
  over a pair returns, and nil once the capability is advertised. The cases come from the table, so a rule added later is covered.

Planted break: one rule left out.

### 11. `ToolCallTimeline` (MADR item 9)

* `tooltimeline.go`, new: `ToolCallEntity`, `ToolCallTimeline`, `Apply`, `Get` and `Open`, by the MADR's rules.
* `cmd/generate/engine_test.go`'s `TestRuntimeFilesCoverHandWritten` names `tooltimeline.go`.
* Tests:
  * a `tool_call`, then updates each carrying one field;
  * `content` and `locations` replaced, not appended;
  * an update for an unseen id starting the call `pending`;
  * entities as copies;
  * a fixture under `testdata/toolcalls/` applied live and as a replay, equal.

Planted breaks: `content` appended; an absent field cleared; an unseen id dropped.

### 12. `Kind()` on the remaining unions (MADR item 10)

* `helpers.go`: `Kind()` on `AuthMethod`, `CreateElicitationRequest`, `CreateElicitationResponse`, `McpServer`,
  `MultiSelectItems`, `SetSessionConfigOptionRequest` and `PlanUpdateContent`, each the discriminator of the set variant, an
  unknown or catch-all one's, or `""`.
* Tests: each variant of each union, decoded from JSON; a catch-all; an unknown value; the zero value. The script that counted
  the unions in the MADR is rerun, and finds every stable union with `Kind()`.

Planted break: one variant's name wrong.

### 13. `acptest`'s additions (MADR item 8)

* `pair.go`: `ConnectRecorded(tb, agent, client, rec, opts...)`.
* `recorder.go`: each frame's direction, `Frames` unchanged, and a `Records()` with the direction; `Wait(ctx, n)`.
* `golden.go`: `ValidateGolden(tb, frames)`.
* `script.go`: `Turn` gains `Err`, `Usage` and `WaitCancel`.
* `client.go`: `MemoryClient` gains `SetElicitation(func(acp.CreateElicitationRequest) acp.CreateElicitationResponse)`, its
  answers checked with `Check` before they are sent, and `Terminals`, a `*clienthost.Terminals` it serves when set.
* Tests: each addition, and `ValidateGolden` refusing a frame that breaks a type's `Validate`, an unknown method, and a response
  to no recorded request.

Planted breaks: a frame's direction swapped; `ValidateGolden` accepting a broken frame; `WaitCancel` not waiting.

### 14. Docs

`README.md`: the client hosts, end causes and `WaitContext`, `AllowsValue`, `LineTap`, the tool-call timeline, the `Kind()`
list, and `acptest`'s additions, each in the section its subject already has. `docs/architecture.md`: `clienthost` and
`internal/proctree`, the hand-written files, and the tree.

### 15. The adoption report

`docs/reports/0041-REPORT-consumer-adoption-of-v1-21-to-v1-25.md`. For each consumer, every site the MADR's investigation found,
read again at the consumer's head, with the API that replaces it, and the consumer records stale against this repository. It
decides nothing.

### 16. Close the implementation

`make api` against the newest `v1` tag; `tools/gates.py --all`; `README.md`, `docs/architecture.md` and the new packages' docs
read in full against the code; the MADR's "Confirmation" walked item by item.

### 17. The release, only on the owner's ask in that turn

As 0040's phase 8 ran:
* the release-notes report;
* `tools/consumer.py` extended to use `clienthost`, `EndedCleanly`, `WaitContext` and `ToolCallTimeline`, with
  `make check-tools`;
* the push and its CI, before the report cites it;
* the tag on the owner's ask, the GitHub release, `tools/publication.py`, and `tools/consumer.py`.

## Verification

* Every item of the MADR's "Confirmation" holds, each by its phase's tests.
* `make api` lists only compatible changes in `acp`, `agentproc` and `acptest`, and `clienthost` as new.
* No generated file changes, unless phase 6 is amended for it.
* `TestDetachedTapIsPrefix` and `TestCloseReadsTheAgentsLastOutput` pass 100 times on macOS and on both hosts.
* `make pre-add-check`, `make lint`, `tools/platforms.py` and `tools/gates.py --all` at every code phase.
* `make disclosure-check` and `tools/idscan.py`: nothing found.
* Every planted break fails its check, each in its own copy of the tree.

## Rollout and Rollback

* **Rollout.** Phases 3 to 13 are separate commits. Under question 6's answer, one minor carries them, or a patch for phase 3 first.
  Each consumer adopts under its own records, from the adoption report.
* **What a consumer meets on the release:** new names, a new package, and `agentproc.Close` waiting up to the grace for the
  agent's output to end after the agent exits. ~~That wait already bounded `Close`, so its longest time is unchanged.~~
  (Corrected 2026-10-27, phase 3: `Close` can take up to one grace longer than before, and only when the agent's output stays
  open after its exit, as when a process that left the agent's group or job holds it.)
* **Rollback before a tag:** revert the phase's commits.
* **Rollback after a tag:** a name cannot be removed under `v1`; a defect is fixed forward in a patch.

## Execution record

### Phase 1, the records (2026-10-27)

* The pair was written at `2200989` (`v1.24.0`) and indexed in `docs/README.md`.
* The owner answered the MADR's six questions on 2026-10-27, each with the recommended answer. The MADR records them in "Decided
  with the owner, 2026-10-27", and is `accepted`. This PLAN is `in-progress`.
* `0026-MADR-connection-end-cause-and-wait.md` gained "Amended by 0041, 2026-10-27: a bounded wait beside `Wait`".
  `0027-MADR-agent-process-helper.md` gained "Amended by 0041, 2026-10-27: the exit order, and the output read to its end".
* Checks: `make check-records`, `make markdownlint` and `make disclosure-check` exit 0. `tools/idscan.py --paths` over the
  changed records finds nothing. The second consumer's name appears nowhere in them.

### Phase 2, the lost output reproduced (2026-10-27)

Nothing in the tree changed. A scratch script exported `8d30c43` with `git archive` into a fresh directory per mode, edited the copy,
and ran `go test -v -run '^TestDetachedTapIsPrefix$' -count=20 ./agentproc/` on macOS. Each pause wrote `phase2-pause` to stderr
the first time it fired, so the record shows that it ran. The modes:

* **`baseline`:** the copy unchanged.
* **`once`:** `tapReader` gets a channel, `Process.done`. Its first `Read` after the channel closes sleeps 50 ms. This is the form
  the step names.
* **`slow`:** `Process` gains a `stopping` channel, closed as `stop` begins. `tapReader` sleeps 5 ms before every `Read` once that
  channel is closed.
* **`slowfix`:** `slow`, and `stop` waits after its switch, unless hurried, for `p.conn.Done()`, `p.hurry`, or the rest of the grace
  measured from `stop`'s start, before `p.conn.Close()`. This is the MADR's item 1.

| Mode | Runs | Failed | Pauses seen |
| :--- | ---: | ---: | ---: |
| `baseline` | 20 | 0 | — |
| `once` | 20 | 0 | 9 |
| `slow` | 20 | 18 | 20 |
| `slow`, again | 20 | 19 | 20 |
| `slowfix` | 20 | 0 | 20 |

Every one of the 37 failures has the same line, the CI run's:

```text
tap_test.go:335: the tap received 33554432 bytes and dropped 33488896, 67043328 in all; the agent wrote 67108864
```

**Deviation, 2026-10-27.**
* **Found:** the step expected a failure every time. The single pause it names never fails, since a reader that keeps up is
  already blocked in `Read` when the agent exits. The loss needs a reader that is behind when the agent exits, as under the race
  detector in CI. With one, the test failed in 37 of 40 runs. In the 3 that passed, the reader caught up first.
* **Decided:** the owner accepted the result as confirming the MADR's cause. Phase 3 proceeds.
* **Scope changes:** phase 3's new test holds the reader back through an unexported `Command` field, and its planted break runs at
  `-count=20`. The MADR carries "Amended by the PLAN's phase 2, 2026-10-27".

### Phase 3, `agentproc` reads the output to its end (2026-10-27)

**Deviation, 2026-10-27, the wait's bound.**
* **Found:** the step said "the rest of the grace, from the grace's start", and the rollout said `Close`'s longest time was
  unchanged. Those two do not fit the rule the MADR names, "as the watcher already does". The watcher counts its grace from the
  agent's exit, and `Start`'s doc already promised that rule.
* **Offered:** the grace from the exit; the rest of the step's grace, as phase 2's `slowfix` did, which leaves a late exit little
  time to read; or a fixed half grace.
* **Decided:** the owner chose the grace from the exit. The rollout line is corrected: `Close` can take up to one grace longer, and
  only when the agent's output stays open after its exit.

**Deviation, 2026-10-27, the `hurry` break's check.**
* **Found:** `TestKillCutsCloseShort` cannot catch a wait that ignores `hurry`. Its agent leaves no process holding the output, so
  the kill ends the output by itself.
* **Done:** the check is `TestKillClosesAtOnce`, new, on Unix. It uses the `parent-escape` agent, whose grandchild leaves the group
  and holds the output, at the default grace of 5 s. `Kill` must close the connection within the 3 s slack. The step's own words
  name the check this break must fail, so this is recorded as a deviation and not asked about first.

**What changed:**
* **`agentproc/agentproc.go`:**
  * `watch`'s wait for the connection's end also ends at `p.hurry`.
  * `stop` no longer closes the connection. The watcher alone closes it, up to the grace after the agent's exit, or at once after
    `Kill`.
  * `Close`'s and `watch`'s docs say so.
  * `Command` gains an unexported `readHook func()`, passed to `streams`.
* **`agentproc/tap.go`:** `tapReader` calls the hook, when set, before each read. `streams` takes it.
* **Tests:**
  * `TestCloseReadsTheAgentsLastOutput`, new in `tap_test.go`. The new `last` child, in `child_test.go`, writes 2 MiB of a
    `bulkStream` after a line of input and exits at once. The hook sleeps 5 ms before each read from just before that line is
    sent. The test calls `Close`, and requires the tap's bytes to equal the stream, with nothing dropped.
  * `TestKillClosesAtOnce`, new in `agentproc_unix_test.go`.
  * `TestTapsUnset`'s call to `streams` gains the argument.
* **Docs:**
  * `README.md`'s `agentproc` section: the connection reads to the end of the output after the exit, up to the grace.
  * `docs/architecture.md`: the watcher, not `Close`, closes the connection, and `hurry` ends the watcher's wait. The two new tests
    are added to the test list.
* **The API:** `make api` against `v1.24.0` reports `0 incompatible, 0 compatible`. The field is unexported.

Planted breaks. Each ran in its own copy of the working tree, made by `git archive` of `8d30c43` with the changed files copied
over, beside a control: the same copy and run with no break.

| Break | Check | Broken | Control |
| :--- | :--- | :--- | :--- |
| `stop` closes the connection at once, as before | `TestCloseReadsTheAgentsLastOutput`, `-count=20` | 20 of 20 failed | 0 of 20 failed |
| the same, with phase 2's slow reader | `TestDetachedTapIsPrefix`, `-count=20` | 19 of 20 failed | 0 of 20 failed |
| the watcher's wait ignores `hurry` | `TestKillClosesAtOnce` | failed | passed |

The broken runs' first failing lines:

```text
tap_test.go:399: the tap received 2031616 bytes and dropped 0 of the agent's 2097152: the connection was closed before it read the agent's last output
tap_test.go:354: the tap received 33554432 bytes and dropped 33488896, 67043328 in all; the agent wrote 67108864
agentproc_unix_test.go:159: Kill took longer than 3s
```

Both loss failures miss 65536 bytes, as the CI run did.

**100 runs:**

| Host | Tests | Result |
| :--- | :--- | :--- |
| macOS, `-race` | `TestDetachedTapIsPrefix`, `TestCloseReadsTheAgentsLastOutput`, `TestKillClosesAtOnce` | 300 passed, 0 failed, 711 s |
| a Linux host reached over SSH | the same three | 300 passed, 0 failed |
| a Windows host reached over SSH | `TestDetachedTapIsPrefix`, `TestCloseReadsTheAgentsLastOutput` | 200 passed, 0 failed |

The first macOS run of 200 stopped at `go test`'s default 10-minute timeout. Its panic named `TestDetachedTapIsPrefix`, 3 s into a
run and waiting for the agent's first 56 MiB. That is the time limit, not a hang. The run above used `-timeout=60m`.

**Gates:**
* `tools/gates.py --all`, with both hosts set: all 18 passed.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0.
* `tools/idscan.py --paths` over the staged files: nothing found.

### Phase 4, `agentproc.LineTap` (2026-10-27)

**What changed:**
* **`agentproc/linetap.go`, new:** `LineTap(fn func(frame []byte)) io.Writer`, the MADR's signature.
  * Each frame is a fresh copy, without its newline. A `\r` before the newline is kept.
  * A line split across writes is joined. An unended line is held, whole, and never given to `fn`.
  * The writer is also an `io.Closer`, whose `Close` drops the held line and returns nil.
  * `LineTap` panics on a nil `fn`, so the mistake surfaces at the call and not on the tap's goroutine. The step did not name
    this.
  * The doc says one `LineTap` per tap, since a tap is written from one goroutine.
* **`agentproc/linetap_test.go`, new:**
  * `TestLineTapFrames`: the stream whole, a byte at a time, and split at every pair of points.
  * `TestLineTapCopies`: `fn` changes each frame; the caller spoils its buffer after each write.
  * `TestLineTapHoldsUnended`: nothing before the newline or after `Close`, and a line after `Close` is its own.
  * `TestLineTapNilFn`.
  * `TestLineTapAsOutputTap`: the "frames" child's three lines through `Command.OutputTap`, the one too long for the 1 MiB limit
    and the one that is not JSON included.
* **`agentproc/example_test.go`:** `ExampleLineTap`.
* **The API:** `make api` against `v1.24.0` reports `compatible: agentproc: LineTap: added`, 0 incompatible.
* **Left to phase 14:** `README.md` and `docs/architecture.md`, as the step order says.

**Not bounded:** an unended line is held whole, however long, as the consumer's splitter holds it. The tap's own 32 MiB bound
applies before `LineTap` sees the bytes, but not to what `LineTap` holds. Neither record asks for a bound, so none was added.

Planted breaks, by `tools/plants.py`, each in its own copy of the working tree: 6 of 6 failed as required.

| Break | Check | Failing line |
| :--- | :--- | :--- |
| a frame aliases the caller's buffer | `TestLineTapCopies` | `linetap_test.go:103: frames kept after the caller reused its buffer = ["ZZZ" "two" "ZZZZZ"]` |
| a frame shares the held line's storage | `TestLineTapCopies` | `linetap_test.go:103: frames kept after the caller reused its buffer = ["thr" "thr" "three"]` |
| `Close` delivers the unended line | `TestLineTapHoldsUnended` | `linetap_test.go:125: frames after Close = ["partial"], want none` |
| a write delivers its unended end | `TestLineTapFrames` | `linetap_test.go:63: … gave frames ["a" "bb\r" "" "{\"x\":1}" "ccc"]` |
| the newline kept in the frame | `TestLineTapFrames` | `linetap_test.go:63: … gave frames ["a\n" "bb\r\n" "\n" "{\"x\":1}\n"]` |
| a nil `fn` accepted | `TestLineTapNilFn` | `linetap_test.go:139: LineTap(nil) did not panic` |

**Gates:**
* `tools/gates.py --all`, with both hosts set: all 18 passed. The `newfiles` gate ran `scripts/go-precheck.sh` on the two new
  files.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0.
* `tools/idscan.py --paths` over the staged files: nothing found.

### Phase 5, end causes (2026-10-27)

**What changed:**
* **`connection.go`:** `EndedCleanly(err error) bool`, after `Err`.
  * It is true for an error that wraps `ErrConnectionClosed` and not `ErrNotificationQueueOverflow`, or that wraps
    `ErrPeerDisconnected` and `io.EOF`. It is false otherwise, nil included.
  * Its doc names what it reads: a connection's `Err` or `Wait`'s error. A request made after the peer's end wraps
    `ErrPeerDisconnected` without `io.EOF`, so it is not a clean end.
* **`agentproc/agentproc.go`:**
  * `Process` gains `exitedFirst` and `returned`, under `p.mu`.
  * `watch` checks `p.conn.Done()` as it records the exit, before `p.tree.end()`. Ending the tree can end what a process the
    agent started holds of its output, so a check after it could see the connection end that the agent's exit caused.
  * `Wait` sets `returned`. `ExitedFirst` returns `returned && exitedFirst`.
  * Its doc says that an agent's own exit ends its output, so the two ends can come together and either can be seen first.
* **How the order is recorded, against the step:** the step said `watch` and the connection's end would each record their order.
  One check in `watch`, under the lock that marks the exit, decides the order alone. The monitor records nothing.
* **Tests:**
  * `TestEndedCleanly`, in `connection_end_test.go`, covers ten cases:
    * `Close`, and `Close` wrapped by `fmt.Errorf`;
    * the peer's end of output, and `Wait` after it;
    * a read's error, and a notification overflow;
    * a request after the peer's end;
    * `io.EOF` alone, another error, and nil.
  * `TestExitedFirst`, in `ties_test.go`, runs each case 50 times:
    * `parent-exit`, whose beat grandchild holds the output past the agent's exit, gives true;
    * the new `closeout` child closes its output and exits at its input's end, and gives false.

    Each run checks `ExitedFirst` false once `Done` is closed and before `Wait`, then the order after `Wait`.
* **The API:** `make api` against `v1.24.0` reports `EndedCleanly`, `(*Process).ExitedFirst` and `LineTap` added: 3 compatible, 0
  incompatible.
* **Left to phase 14:** `README.md` and `docs/architecture.md`.

**A check corrected during the phase.** The first form of `TestExitedFirst` read `ExitedFirst` as soon as the agent was ready.
The "final before Wait" break passed it, since the agent's exit had seldom been recorded by then. The check now runs once `Done`
is closed, and the break fails it.

Planted breaks, by `tools/plants.py`, each in its own copy of the working tree: 6 of 6 failed as required.

| Break | Check | Failing line |
| :--- | :--- | :--- |
| overflow counted as clean | `TestEndedCleanly` | `connection_end_test.go:238: EndedCleanly(acp: connection closed: notification queue overflow), a notification overflow, = true, want false` |
| any peer end counted as clean | `TestEndedCleanly` | `connection_end_test.go:238: EndedCleanly(acp: peer disconnected: read: connection reset), a read's error, = true, want false` |
| the order recorded the wrong way round | `TestExitedFirst` | `ties_test.go:134: run 0: ExitedFirst after Wait = true, want false` |
| the same, on the Windows host | `TestExitedFirst` | `ties_test.go:134: run 0: ExitedFirst after Wait = true, want false` |
| `ExitedFirst` final before `Wait` | `TestExitedFirst` | `ties_test.go:130: run 0: ExitedFirst before Wait = true, want false` |
| the order checked after the tree's end | `TestExitedFirst` | `ties_test.go:130: run 0: ExitedFirst after Wait = false, want true` |

`run` is a `t.Helper`, so line 130 is the exits-first case and line 134 the connection-first case.

**Gates:**
* `tools/gates.py --all`, with both hosts set: all 18 passed.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0.
* `tools/idscan.py --paths` over the staged files: nothing found.

### Phase 6, `WaitContext` (2026-10-27)

**Deviation, 2026-10-27, the one waiter.**
* **Found:** `TestGoStatementsCounted` (`connection_end_test.go`) failed on any `go` statement in the root package's non-test
  files. `WaitContext`'s goroutine runs `Wait`, so it cannot be one of the goroutines `Wait` counts. The step had said so, but not
  that this test forbids it.
* **Offered:** name the one exception in the test; replace the connection's `sync.WaitGroup` with a counter that closes a channel,
  so no goroutine is needed; or bound `acptest` alone, question 2's alternative.
* **Decided:** the owner chose to name the exception. The test now allows exactly one `go` statement, in `connection.go`'s
  `waitDone`, and fails on any other, and on none there.

**Deviation, 2026-10-27, the side connections.**
* **Found:** the first gate run failed `TestSideConnectionMethodsAreProtocolMethods` and `TestTypedMethodDocsNameErrors` on the two
  side-connection methods. Under 0038's rule, a side-connection method that takes a context is a protocol method. Generating
  them would not help: `TestNotInitialized` and the typed-method count would then treat them as wire calls.
* **Offered:** none on the side connections, the caller using `Connection().WaitContext`; a package-level function; or the methods
  kept, with the three tests exempting them.
* **Decided:** the owner chose `Connection().WaitContext`. The MADR carries "Amended by the PLAN's phase 6, 2026-10-27", and
  `0026-MADR-connection-end-cause-and-wait.md`'s amendment says the same.

**What changed:**
* **`connection.go`:**
  * `Connection` gains `waitOnce` and `waited`.
  * `WaitContext` selects on `waitDone()` and `ctx.Done()`. When both are ready, the end wins: it returns `Err`, or else
    `context.Cause(ctx)`.
  * `waitDone` starts, once, the goroutine that runs `Wait` and closes `waited`.
* **`agentproc/agentproc.go`:** `Process.WaitContext`, the same over `Process.Wait`. It returns `Wait()` again once `waited` is
  closed, which then returns at once.
* **`acptest/pair.go`:**
  * `Connect` calls `connectWithin(tb, cleanupWait, …)`, with `cleanupWait = 10 * time.Second`.
  * The cleanup closes the pair, then waits on each side's `Connection().WaitContext` under one 10 s deadline.
  * When the deadline passes, the test fails naming the side, with the stacks of this module's goroutines from the goroutine
    profile, `running()`.
* **Tests:**
  * `TestWaitContext`, root, covers four things:
    * an ended ctx's cause, the connection left running;
    * `ErrConnectionClosed` after `Close`;
    * the peer's end, equal to `Wait`'s error, for the connection and each side's `Connection()`;
    * 100 calls leaving one goroutine, found by its `pprof` label, and none after `Wait`.
  * `TestProcessWaitContext`, in a new `agentproc/waitcontext_test.go`, covers the same for a process, and `ExitedFirst` true
    once `WaitContext` has returned for `parent-exit`.
  * `TestConnectCleanupBounded`, in a new `acptest/pair_internal_test.go`, uses a `testing.TB` that keeps the cleanups and errors.
    A handler that ignores its context makes the cleanup return at the 300 ms bound, naming the agent side and
    `stuckAgent).NewSession`. A pair with no stuck handler reports nothing.
  * `TestGoStatementsCounted`, amended as above.
* **The API:** `make api` against `v1.24.0` reports `(*Connection).WaitContext` and `agentproc: (*Process).WaitContext` added, with
  phases 4 and 5's three: 5 compatible, 0 incompatible.
* **Left to phase 14:** `README.md` and `docs/architecture.md`.

**Checks corrected during the phase:**
* **The goroutine counters undercounted.** A goroutine profile of debug 1 begins with a header line, and the largest group follows
  it, so `Sscanf("%d @")` missed that group. The goroutine-per-call break reported 2 waiting goroutines, not 100. The counters now
  find the count line with a regexp, and the break reports 100.
* **Two plants were wrong as written,** a channel shared across calls and an unused variable, and were rewritten. The checks did
  not change.
* **The first gate run also failed `lint` and `go fix`:** `strings.SplitSeq`, `slices.Backward`, and revive's confusing-naming on
  `connect`, renamed `connectWithin`.

Planted breaks, by `tools/plants.py`, each in its own copy of the working tree: 8 of 8 failed as required.

| Break | Check | Failing line |
| :--- | :--- | :--- |
| a goroutine per call, the connection | `TestWaitContext` | `connection_end_test.go:408: 100 calls of WaitContext left 100 waiting goroutines, want 1` |
| the cause lost, the connection | `TestWaitContext` | `connection_end_test.go:363: WaitContext with an ended ctx = context canceled, want its cause, the test's bound` |
| a `go` statement outside `waitDone` | `TestGoStatementsCounted` | `connection_end_test.go:478: connection.go:829:2: a go statement; start it with the connection's goroutines.Go` |
| the waiter moved out of `waitDone` | `TestGoStatementsCounted` | `connection_end_test.go:478: connection.go:883:3: a go statement; …`, and `0 go statements in connection.go's waitDone, want 1` |
| a goroutine per call, the process | `TestProcessWaitContext` | `waitcontext_test.go:68: 100 calls of WaitContext left 100 waiting goroutines, want 1` |
| the cause lost, the process | `TestProcessWaitContext` | `waitcontext_test.go:30: WaitContext with an ended ctx = context canceled, want its cause, the test's bound` |
| the bound's report dropped | `TestConnectCleanupBounded` | `pair_internal_test.go:86: the cleanup reported "", want the agent side named, …` |
| the cleanup unbounded | `TestConnectCleanupBounded` | `panic: test timed out after 20s`, at the plant's `-timeout=20s` |

**Gates:**
* `tools/gates.py --all`, with both hosts set: the first run failed 10, on the side-connection rule and the lint above. The second
  run, after the fixes, passed all 18.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0.
* `tools/idscan.py --paths` over the staged files: nothing found.

### Phase 7, `internal/proctree` (2026-10-27)

**Deviation, 2026-10-27, `tools/platforms.py`.**
* **Found:** the tool linted and vetted `./agentproc` alone. The moved code in `internal/proctree`, and `clienthost` in phase 8,
  would have had no per-platform lint, though step 7 and the gate list rely on the tool. Its `go vet ./agentproc` type-checks the
  package as a dependency, but runs no analyzer on it.
* **Offered:** widen the tool now; check `internal/proctree` by a scratch script this phase alone; or widen it in a later change.
* **Decided:** the owner chose to widen it now.
  * `PKGS = ["./agentproc/...", "./internal/..."]` feeds both the lint and the vet commands. Phase 8 adds `clienthost`.
  * The tool's docstring, its line in `tools/README.md`, and the tools paragraph of `docs/architecture.md` say so.
  * `make check-tools` exits 0.

**What moved,** by `git mv`, so history follows:
`agentproc/{tree_unix,tree_windows,tree_other,deathsig_none,deathsig_pdeathsig}.go` to `internal/proctree/`.

**What changed in them, beyond the package:**
* **Exported names** for `agentproc`'s and `clienthost`'s use: `HasGroups`, `HasTree`, `Prepare`, `Attach`, `Tree` with
  `Terminate`, `Kill` and `End`, and on Windows `AttachDelay`. The unexported `setDeathSignal` is now `defaultDeathSignal`, after
  revive's confusing-naming finding beside the new `SetDeathSignal`, which the widened tool reported first.
* **`Prepare` no longer sets the death signal.** `SetDeathSignal(cmd)` does, on Unix, and is a no-op on Windows and elsewhere. A
  `Pdeathsig` follows the thread that started the process (0029), and `agentproc` keeps that thread for the agent's life. So the
  two are apart, for phase 8 to decide whether a terminal takes one. `agentproc`'s `prepare`, in a new `agentproc/tree.go`,
  calls both, in the old order, so the agent's start is as before.
* **The refusals are sentinels,** `ErrPgidSet` and `ErrSuspended` in a new `internal/proctree/proctree.go`, with the package doc.
  `agentproc`'s `prepare` returns its exact old texts for them. `TestStartRefusesSharedGroup` and the Windows suspended test pass
  unchanged.
* **Four Windows error texts** now say "the process" for "the agent", for example "creating the process's job object". These are
  failures of Windows calls, which `Start` wraps with `agentproc:`. No test reads them.
* **The tests' names stay in `agentproc`.** `hasGroups` and `hasTree` move to a new `agentproc/tree_test.go`, and `attachDelay`
  to `agentproc/tree_windows_test.go`, as `&proctree.AttachDelay`. No existing test changed.

**The token comparison.** A scratch script compares each moved file's Go tokens at `HEAD` with the new file's, comments
dropped and the renames above mapped. Ten differences are left, each one listed above:

```text
== tree_unix.go: 300 tokens at HEAD, 325 now, 3 difference(s)
  replace: 'errors . New ( "agentproc: Command.Cmd.SysProcAttr.Pgid is set; the agent needs a process group of its own" )' -> 'ErrPgidSet'
  insert: '' -> 'return nil } func SetDeathSignal ( cmd * exec . Cmd ) { if cmd . SysProcAttr = = nil { cmd . SysProcAttr = & syscall . SysProcAttr { } }'
  delete: 'return nil' -> ''
== tree_windows.go: 763 tokens at HEAD, 768 now, 6 difference(s)
  replace: 'errors . New ( "agentproc: Command.Cmd.SysProcAttr.CreationFlags has CREATE_SUSPENDED; Start starts the agent suspended itself" )' -> 'ErrSuspended'
  insert: '' -> '} func SetDeathSignal ( * exec . Cmd ) {'
  replace: '"creating the agent\'s job object: %w"' -> '"creating the process\'s job object: %w"'
  replace: '"limiting the agent\'s job object: %w"' -> '"limiting the process\'s job object: %w"'
  replace: '"assigning the agent to its job object: %w"' -> '"assigning the process to its job object: %w"'
  replace: '"resuming the agent: %w"' -> '"resuming the process: %w"'
== tree_other.go: 112 tokens at HEAD, 122 now, 1 difference(s)
  insert: '' -> '} func SetDeathSignal ( * exec . Cmd ) {'
== deathsig_none.go: 14 tokens at HEAD, 14 now, 0 difference(s)
== deathsig_pdeathsig.go: 31 tokens at HEAD, 31 now, 0 difference(s)
10 difference(s) in all
```

**The widened tool's planted breaks.** Each ran in a copy of the working tree, with the working tree's `platforms.py` and with
`HEAD`'s:

| Break in `internal/proctree/deathsig_pdeathsig.go` | The widened tool | `HEAD`'s tool |
| :--- | :--- | :--- |
| a variable declared and not used | `8 of 13 passed; failed: lint-linux, lint-freebsd, vet-linux-amd64, vet-linux-arm64, vet-freebsd-amd64` | the same: `agentproc`'s vet type-checks the package |
| an exported function without a doc comment | `11 of 13 passed; failed: lint-linux, lint-freebsd`, with `deathsig_pdeathsig.go:7:1: exported: exported function Planted should have comment or be unexported (revive)` | `13 of 13 passed` |

The second row is the widening's proof: a lint finding in a Linux-only file of the package, which the old tool passed.

**The API:** `make api` reports no change from phase 6, 5 compatible and 0 incompatible. `internal/proctree` is not part of the
API.

**Gates:**
* `tools/platforms.py`: the first run failed `lint-linux` and `lint-freebsd`, on the confusing-naming finding above. After the
  rename, `13 of 13 passed`.
* `make lint` failed once on `AttachDelay`'s doc form, which was corrected.
* `tools/gates.py --all`, with both hosts set: all 18 passed. The Windows host ran `agentproc` in 18.4 s, the Linux host in
  5.0 s.
* `make check-records`, `make markdownlint`, `make disclosure-check`, `make check-tools`: exit 0.
* `tools/idscan.py --paths` over the staged files: nothing found.

### Phase 8, `clienthost.Terminals` (2026-10-27)

**Deviation, 2026-10-27, `Refusals`.**
* **Found:** the step's last test, "a client embedding `Terminals` and `ClientRefusals`", cannot compile. Both declare the five
  terminal methods, and Go promotes neither of two methods of one name at one depth. A scratch program printed: `cannot use
  sameDepth{} … does not implement terminaler (ambiguous selector sameDepth.CreateTerminal)`.
* **Offered:** a `clienthost.Refusals` that puts `acp.ClientRefusals` one level deeper; a documented pattern with no new name; or
  forwarding methods instead of embedding.
* **Decided:** the owner chose `clienthost.Refusals`. The MADR carries "Amended by the PLAN's phase 8, 2026-10-27".

**Names and behaviours not in the step,** recorded in the same MADR amendment for the owner's review:
* `DirectCommand`, the default `Command`.
* `MaxOutputBytes` and `DefaultMaxOutputBytes`, 16 MiB, bounding an agent-chosen limit.
* While a command runs, an unfinished character is held back.
* A command's exit ends what is left of its tree, as `agentproc`'s does.
* A terminal takes no death signal.

**What changed:**
* **`clienthost/`, new:**
  * `doc.go`, the package doc, with the embedding of `Terminals` beside `Refusals`.
  * `refusals.go`, `Refusals`.
  * `output.go`, the output buffer:
    * it cuts back to its limit once it has grown 64 KiB past it;
    * `cut` moves a front cut past up to three continuation bytes;
    * `complete` drops a trailing character not yet finished.
  * `terminals.go`:
    * `Terminals`, its five methods and `Close`, and `DirectCommand`;
    * a `terminal` per command: its output to an `os.Pipe` the terminal reads, so `Cmd.Wait` returns at the exit;
    * `wait` ends the tree under the lock that marks the exit, then waits up to 1 s for the rest of the output;
    * `kill` sends nothing once the exit is marked;
    * `release` kills, waits for the exit, and closes the pipe;
    * ids are `terminal-1`, `terminal-2`, and so on.
  * `exit_unix.go` names a signal through `golang.org/x/sys/unix`'s `SignalName`, for example `SIGKILL`, a package of the module
    `go.mod` already requires. `exit_other.go` gives the exit code alone.
* **The tests:**
  * `main_test.go`: the test binary as the command, by `CLIENTHOST_TEST_CHILD`. Its modes are `write`, `repeat`, `sleep`,
    `parent` and `beat`, with the leak guard.
  * `output_test.go`: `TestOutputTruncatesAtCharacter` covers characters of 1 to 4 bytes, each limit offset into one, read and
    compacted. `TestOutputHoldsBackUnfinishedCharacter`.
  * `terminals_test.go`, ten tests:
    * `TestTerminalOutputAndExit`;
    * `TestTerminalOutputByteLimit`: 2-, 3- and 4-byte characters written in 7-byte writes, and `MaxOutputBytes`;
    * `TestTerminalKill`: `SIGKILL` on Unix, status 1 on Windows;
    * `TestTerminalEndsTree`, by kill and by release;
    * `TestTerminalWaitContext`;
    * `TestTerminalUnknownID`: four methods, a never-made id and a released one;
    * `TestTerminalsClose`;
    * `TestTerminalCommandRefused`;
    * `TestDirectCommand`, not parallel, since it sets the environment;
    * `TestTerminalsOverConnection`: a `hostClient` embedding `Terminals` and `Refusals`, over `acptest.Connect`.
  * `example_test.go`, `ExampleTerminals`: a `Command` that refuses a cwd outside the project.
* **`scripts/check_api.py`'s `PACKAGES` gains `clienthost`,** and `scripts/test_check_api.py` asserts it.
* **`tools/platforms.py`'s `PKGS` gains `./clienthost/...`,** as phase 7's entry said. Its line in `tools/README.md` and
  `docs/architecture.md` follows.
* **The API:** `make api` against `v1.24.0` reports 9 compatible, 0 incompatible. The new ones are `clienthost`'s
  `DefaultMaxOutputBytes`, `DirectCommand`, `Refusals` and `Terminals`.
* **Left to phase 14:** `README.md`, and `docs/architecture.md`'s package list.

Planted breaks, by `tools/plants.py`, each in its own copy of the working tree: 9 of 9 failed as required.

| Break | Check | Failing line |
| :--- | :--- | :--- |
| truncation at a byte | `TestOutputTruncatesAtCharacter` | `output_test.go:32: "é", limit 100, compacted false: the output is not valid UTF-8: "\xa9ééé\xc3"…` |
| an unfinished character shown while running | `TestOutputHoldsBackUnfinishedCharacter` | `output_test.go:57: "é" split at 1, running: "ab\xc3", false; want "ab", false` |
| the tree not ended by kill, nor at the exit | `TestTerminalEndsTree` | `terminals_test.go:215: the command's grandchild still runs after kill` |
| the tree not ended by release, nor at the exit | `TestTerminalEndsTree` | `terminals_test.go:215: the command's grandchild still runs after release` |
| the same, on the Windows host | `TestTerminalEndsTree` | `terminals_test.go:215: the command's grandchild still runs after release` |
| an unknown id answered with a plain error, `-32603` on the wire | `TestTerminalUnknownID` | `terminals_test.go:272: terminal/wait_for_exit of terminal-none = unknown terminal "terminal-none", want resource-not-found (-32002)` |
| the signal unnamed | `TestTerminalKill` | `terminals_test.go:171: WaitForTerminalExit after kill = {… ExitCode:<nil> Signal:0x…}, want signal SIGKILL` |
| `Close` not waiting | `TestTerminalsClose` | `terminals_test.go:298: terminal 0's command runs after Close` |
| `Refusals` at the same depth, an alias of `acp.ClientRefusals` | `TestTerminalsOverConnection` | `terminals_test.go:376:53: cannot use client (variable of type *hostClient) as acp.Client value …` |

**What the first plants showed:**
* **Kill alone is not the only path to the tree's end.** The first kill plant, which only made kill skip the tree, passed. The
  killed command's exit ends what is left of its tree anyway. The plant that removes both, kill's tree and the exit's, fails.
* **A plant was wrong as written:** the "`Close` not waiting" plant left a variable unused, and was rewritten.

**Gates:**
* `tools/gates.py --all`, with both hosts set: the first run failed `precheck`, `newfiles`, `lint` and `platforms` on one finding,
  an unneeded `//nolint:gosec` in `terminals_test.go`, since the linter does not run gosec on tests. After its removal, all 18
  passed. The Windows host ran `clienthost` in 1.9 s, the Linux host in 1.5 s.
* `make check-records`, `make markdownlint`, `make disclosure-check`, `make check-tools`: exit 0.
* `tools/idscan.py --paths` over the staged files: nothing found.

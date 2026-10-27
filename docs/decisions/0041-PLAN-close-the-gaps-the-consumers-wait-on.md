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
  otherwise phase 6 is amended first.
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
  the kill ends the agent's output too. With `hurry` closed, by `Kill`, it closes at once.
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
| the wait ignores `hurry` | `TestKillCutsCloseShort` |

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
* The side connections and `agentproc.Process` gain the same, over their `Wait`.
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
* No planted break: the move adds no check. Its proof is the unchanged tests passing on every platform.

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
  * a client embedding `Terminals` and `ClientRefusals`, served over `acptest.Connect`.

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
  agent's output to end after the agent exits. That wait already bounded `Close`, so its longest time is unchanged.
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

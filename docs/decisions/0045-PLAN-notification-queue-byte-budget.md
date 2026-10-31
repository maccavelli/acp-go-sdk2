---
status: in-progress
date: 2026-10-31
associated-madr: "0045-MADR-notification-queue-byte-budget.md"
---

# Implement 0045: a byte budget beside the notification count

Associated MADR: [0045-MADR-notification-queue-byte-budget.md](0045-MADR-notification-queue-byte-budget.md)

## Goal

A connection bounds in bytes what it keeps of its peer's unfinished messages, beside the count it already keeps. Each overflow policy
acts at the budget as it acts at the count, under the MADR's D1 to D7. The owner answered on 2026-10-31: a 64 MiB default (Q1 a)
and one budget for requests and notifications (Q2 a). The notes below on Q1 (c) and Q2 (b) record what those answers would have
changed.

## Scope

In scope:

* `connection.go`:
  * the option `WithMaxQueuedBytes`, its default and its clamp to `WithMaxMessageSize` (D1, D4);
  * the counter under `notifyMu`;
  * the size carried by `queuedNotification`;
  * `receiveLine`'s checks per policy (D3);
  * `waitForRoom`'s second condition;
  * `processNotifications`' release;
  * with Q2 (a), the request path's check and release;
  * the accessor `QueuedBytes` and the log fields (D7);
  * the godoc of `WithMaxQueuedNotifications`, the three policies, `WithMaxConcurrentRequests` and
    `ErrNotificationQueueOverflow`.
* A new test file, `connection_queue_bytes_test.go`. Existing tests change only if one is found to depend on an unbounded queue;
  each such change is recorded.
* `README.md`, "Closing and limits"; `docs/architecture.md`; `example_api_test.go`, an example of the option.
* `tools/consumer.py`, the release notes and the release, on the owner's ask, as 0044's phase 4 ran them.

Out of scope:

* `remote`'s `MaxBuffered`, a different bound (MADR, Consequences);
* `WithMaxMessageSize`'s default;
* a per-entry overhead;
* bytes held by a handler after it returns;
* queues per session;
* the outbound side.

## Implementation Steps

### Phase 1: the budget

1. **Grounding.** In a scratch copy, add a temporary test that queues notifications under each policy and reads the counters, to
   confirm D2's accounting point against `processNotifications`, and that `readLine`'s line length is what reaches
   `receiveLine`. Recorded, then deleted.
2. **The option and the counter.**
   * `connectionConfig.maxQueuedBytes`, default 64 MiB (Q1 a). With Q1 (c) it stays unset, and every check below is skipped
     when it is 0.
   * `WithMaxQueuedBytes(n)` ignores `n <= 0`.
   * `buildConnection` raises a budget below `maxMessageSize` to it (D4).
   * `Connection.queuedBytes int64` and `Connection.byteLimit int64`, under `notifyMu`.
3. **Notifications.**
   * `queuedNotification` gains `size int`. `receiveLine` passes `len(line)`; the line is in scope there.
   * ~~Under `OverflowBlock`, `waitForRoom(size)` also waits while `queuedBytes > 0 && queuedBytes+size > byteLimit`, with
     0044 D3's guard unchanged. Past the wait, a check closes with `ErrNotificationQueueOverflow` when
     `queuedBytes+size > 2*byteLimit`, as the channel's capacity does at 2n.~~ *Amended by deviation 1:* the byte test is
     `notifBytes > 0 && queuedBytes+size > limit`.
     * `notifBytes`, a second counter, holds the notifications' share of `queuedBytes`. `limit` is `byteLimit`, or twice it
       for the read-on check under `OverflowBlock`.
     * Under `OverflowBlock`, `waitForRoom(size)` also waits while the test holds at `byteLimit`, with 0044 D3's guard
       unchanged. Past the wait, the test at `2*byteLimit` closes with `ErrNotificationQueueOverflow`, as the channel's capacity
       does at 2n.
   * Under `OverflowCloseConnection` and `OverflowDropNewest`, the same test before the `select` takes the existing overflow
     path: close, or drop with the drop handler.
   * On queueing, `queuedBytes += size` and `notifBytes += size`. In `processNotifications`, after the handler returns, both
     drop by `queued.size`, beside `completedNotificationSeq`, before the `Broadcast`.
4. **Requests (Q2 a; skipped with Q2 b).**
   * Before a request's goroutine starts, under `notifyMu`: if `queuedBytes > 0 && queuedBytes+size > byteLimit`, answer
     `-32603` with `{"error":"too many queued bytes","limit":byteLimit}`, as the concurrency limit answers, and never wait.
   * Otherwise add `size`, and subtract it in the goroutine's deferred cleanup, with a `Broadcast` so a paused reader re-checks.
   * The check sits after the concurrency limit's, so a request refused for count is not counted.
5. **Observability (D7).** `QueuedBytes() int64`. The pause's `Warn` and the overflow's `Error` gain `bytes` and `byte_limit`.
6. **Godoc.** The option, and the sentences that state each policy's trigger: "n notifications, or the byte budget".
7. **Tests**, in `connection_queue_bytes_test.go`, each in a `synctest` bubble where it waits:
   * `TestQueuedBytesPausesReading`: count room, bytes full; the writer waits, all delivered in order, none dropped, the
     connection open.
   * `TestQueuedBytesReadOnLimit`: under 0044's guard the reader reads to twice the budget, then closes with
     `ErrNotificationQueueOverflow`.
   * `TestQueuedBytesClose` and `TestQueuedBytesDrop`: the other two policies at the budget.
   * `TestQueuedBytesClampAndSingleLine`: a budget below the message size is raised to it, and one maximum-size line is
     accepted with nothing counted.
   * `TestQueuedBytesReleased`: `QueuedBytes()` returns to 0 after every handler.
   * `TestQueuedBytesRefusesRequest` (Q2 a): the answer, the limit, and the handler never runs.
   * `TestQueuedBytesDefault`: a connection given no option has a 64 MiB budget (Q1 a).
   * `TestQueuedBytesRequestsDoNotPause` (deviation 1): a request whose handler holds the budget and awaits a response from
     the peer; notifications then arrive; the response is read, and the handler returns.
8. **Planted breaks**, each in a scratch copy, each failing its test:
   * the trigger on any counted bytes, D3 as first written: `TestQueuedBytesRequestsDoNotPause` hangs until its bubble's
     deadline;
   * no byte check in `waitForRoom`;
   * release at dequeue instead of completion;
   * no clamp;
   * no read-on limit on bytes;
   * no check under `OverflowCloseConnection`;
   * no drop under `OverflowDropNewest`;
   * no release in the request cleanup;
   * the default left unset.
9. **Checks.**
   * `go test -race ./...`, and `-count=3` on the new tests.
   * `tools/remote.py pkg` on the Linux and Windows hosts, with the new tests' `-test.run`.
   * `make pre-add-check`, `make api` (compatible additions only), `make check-records`, `make markdownlint`,
     `make disclosure-check`.
   * Then the execution record, and staging.

### Phase 2: documentation and an example

1. `README.md`, "Closing and limits": the budget, its default, what counts and when it is released, each policy at it, the
   request refusal (Q2 a), and `QueuedBytes`.
2. `docs/architecture.md`: the counter beside the sequence count, and the checks list.
3. `ExampleWithMaxQueuedBytes` in `example_api_test.go`, with fixed output, seen to fail with the option removed.
4. The phase 1 checks again, then the execution record and staging.

### Phase 3: close

1. `tools/gates.py --all`, with its output in the record.
2. After the owner pushes, `tools/watch_ci.py` on the commit.
3. A table of the MADR's ten Confirmation items, each with the test or command that shows it.
4. The MADR's status and the index rows.

### Phase 4: release, on the owner's ask

As [0044-PLAN-notification-queue-back-pressure.md](0044-PLAN-notification-queue-back-pressure.md)'s phase 4 ran it:

* `tools/consumer.py`, also setting `WithMaxQueuedBytes`, with `v1.30.0` in `WANT_RUN`;
* `docs/reports/0045-REPORT-release-notes-v1-30-0.md`, leading with the new default bound;
* the owner's tag, then `tools/publication.py` and `tools/consumer.py v1.30.0`.

## Verification

* Every test of step 1.7 passes under `-race` on macOS, and with `-count=3` on the Linux and Windows hosts, and was seen to fail on
  its planted break.
* The existing queue, barrier, overflow, hold and concurrency-limit tests pass unchanged, or each change is recorded with its
  reason.
* `make api`: compatible additions only, `WithMaxQueuedBytes` and `Connection.QueuedBytes`.
* `tools/gates.py --all` and CI pass.

## Rollout and Rollback

* **Rollout.** A minor, `v1.30.0`. With a default budget, a consumer on `OverflowCloseConnection` or `OverflowDropNewest` that
  keeps more than the budget queued now closes or drops. The release notes say so and name `WithMaxQueuedBytes` to raise it.
* **Rollback before the release.** Revert the phase commits.
* **Rollback after the release.** The option and accessor stay, under the compatibility promise. A patch can leave the budget
  unset by default (Q1 c's behaviour) while a problem is found.

## Execution record

*2026-10-31:* the owner accepted the MADR and approved this PLAN: "accept and approve, proceed with 0011 then 0045".

**Deviation 1, 2026-10-31, step 3, before any code: the trigger is gated on notifications.**

* *Found.* Tracing D3 against D6's request accounting before writing step 3 showed a deadlock under `OverflowBlock`:
  * A request's bytes are counted until its handler returns (`connection.go:1617-1636`). Its handler may await a response from
    the peer, such as an agent's `session/prompt` calling `fs/read_text_file`, which only the reader delivers (`:1529`).
  * A notification arriving with the budget full, "while anything is counted", would pause the reader (`waitForRoom`,
    `:1856-1871`).
  * 0044 D3's guard counts only requests sent from a notification handler's context, and `initialize`.
  * So the reader would wait for the handler and the handler for the reader, until the connection ended. The finding is new to
    this design, not a fault of the tree.
* *Options put to the owner:*
  * gate the trigger on a notification being counted (recommended);
  * separate accounts for requests and notifications, which loosens the bound to about twice the budget;
  * extend 0044's guard to request handlers, under which the budget would rarely bind.
* *Decided.* The owner chose the first. The MADR's D3 carries an annotated amendment, and Confirmation gains item 7.
* *Changed here.* Step 3's byte test is annotated, with a second counter, `notifBytes`. Step 7 gains
  `TestQueuedBytesRequestsDoNotPause`, and step 8 its planted break, the trigger as first written. No file is added to the
  phase's scope.

### Phase 1, 2026-10-31

**What changed.**

* `connection.go`:
  * `defaultMaxQueuedBytes`, 64 MiB.
  * `connectionConfig.maxQueuedBytes`, and `WithMaxQueuedBytes`, ignoring `n <= 0`.
  * `buildConnection` raises the budget to `maxMessageSize` with `max` (D4), and sets `byteLimit`.
  * `queuedBytes`, `notifBytes` and `byteLimit` on `Connection`, under `notifyMu`. `queuedNotification.size`.
  * In `receiveLine`, `size := len(line)`. A request passes `admitRequestBytes` after the concurrency limit's check, or is
    refused `-32603` with `{"error":"too many queued bytes","limit":n}`, its slot freed.
  * Its bytes are released by `releaseRequestBytes`, folded into the existing `release` callback, which `handleInbound` calls
    as the handler returns, before the answer is written (D2). The shutdown refusal releases them too.
  * A notification: `waitForRoom(size)` also waits on `overByteBudget(size, false)`. Past the wait, `overByteBudget(size,
    readOn)` sends it to the overflow path, at twice the budget under `OverflowBlock`. Queued, it adds its size to both
    counters, and `processNotifications` subtracts it beside `completedNotificationSeq`.
  * `QueuedBytes()` on `*Connection` only, as `RefusedMessages()` is (0038), so the generator and the engine cases'
    `want.api` are unchanged.
  * The pause's `Warn` and the overflow's `Error` gain `bytes`, `size` and `byte_limit` (D7). The `Warn` logs the count taken
    when the pause began, since it fires on a timer goroutine without `notifyMu`.
  * Godoc:
    * `WithMaxQueuedNotifications`;
    * `ConnectionOption`'s list of options that ignore `n <= 0`;
    * `WithMaxConcurrentRequests`;
    * the three policies;
    * the new option and accessor.
* `errors.go`: `ErrNotificationQueueOverflow`'s doc names the byte budget.
* `connection_queue_bytes_test.go`, new: seven tests, nine cases. `TestQueuedBytesClose` and `TestQueuedBytesDrop` are one
  test, `TestQueuedBytesCloseAndDrop`, with two subtests. `TestQueuedBytesReleased` became assertions in
  `TestQueuedBytesPausesReading` and `TestQueuedBytesRequestsDoNotPause`.
* *Refinement, not a deviation.* The first `make pre-add-check` failed gocognit: "cognitive complexity 53 of func
  `(*Connection).receiveLine` is high (> 50)". The notification tail of `receiveLine` moved, unchanged, into
  `queueNotification(m, receipt, size)`. The tests and the planted breaks below were rerun after the move.

**Step 1, grounding.** No temporary test was written. Reading `receive` (`c.receiveLine(line, &received)`), `readLine`
(a fresh line each time) and `processNotifications` gave the two facts it was to confirm. The new tests confirm them in place:
`TestQueuedBytesPausesReading` counts 900 bytes for three 300-byte lines, and the release-at-dequeue break fails it.

**The existing suite with the budget in place,** before any new test: `go test -race -count=1 ./...`, every package `ok`. No
existing test changed.

**Step 1.8, the planted breaks**, by a script that requires each break's text to match once, in a scratch copy, rerun after the
refactor:

| Break | Test | Failure seen |
| :--- | :--- | :--- |
| no byte check in `waitForRoom` | `TestQueuedBytesPausesReading` | "6 notifications queued while the handler is held, want 3"; "7 writes returned, want 4" |
| release at dequeue, not completion | `TestQueuedBytesPausesReading` | "4 notifications queued … want 3"; "5 writes returned, want 4" |
| no clamp | `TestQueuedBytesClampAndSingleLine/raised_to_the_message_size` | "1 notifications queued, ended true (acp: connection closed: notification queue overflow); want 2 and open" |
| no read-on limit on bytes | `TestQueuedBytesReadOnLimit` | "3 notifications queued, ended true (…); want 6 and open" |
| no byte check under `OverflowCloseConnection` | `TestQueuedBytesCloseAndDrop/OverflowCloseConnection` | the connection left open; "0 notifications queued, want 3" |
| no drop under `OverflowDropNewest` | `TestQueuedBytesCloseAndDrop/OverflowDropNewest` | "DroppedNotifications = 0, want 7: 3 to 9" |
| no release of a request's bytes | `TestQueuedBytesRequestsDoNotPause` | "QueuedBytes = 900 with every handler returned, want 0" |
| the default left unset | `TestQueuedBytesDefault/no_option` | "byte budget 10485760, want 67108864" |
| the trigger on any counted bytes, D3 as first written | `TestQueuedBytesRequestsDoNotPause` | "the handler's request was not answered: the reader paused on the request's bytes", deviation 1's deadlock reproduced |
| no refusal of a request past the budget | `TestQueuedBytesRefusesRequest` | "answer {ID:"big" Result:{} Error:<nil>}, want -32603 for id big" |

**Step 1.9, the checks.**

* `go test -race -count=1 ./...`: every package `ok`.
* `go test -race -count=3` over the byte, overflow, barrier, hold, queue-size, end, option and concurrency-limit tests: every
  package `ok`.
* `tools/remote.py pkg`, with `-test.run 'TestQueuedBytes|OverflowBlock|ConcurrentRequests' -test.count=3`: "PASS", exit 0,
  on each host, on the final code.
* `make pre-add-check`: "go-precheck: 278 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)." With the new
  file named: "go-precheck: 3 file(s) clean".
* `make api`: "compatible: (*Connection).QueuedBytes: added"; "compatible: WithMaxQueuedBytes: added"; "against v1.29.0: 0
  incompatible, 2 compatible".

The owner committed phase 1 as `7b87714`.

### Phase 2, 2026-10-31

**What changed.**

* `README.md`, "Closing and limits":
  * `WithMaxQueuedBytes` beside `WithMaxQueuedNotifications`, whichever limit is reached first counting.
  * The read-on limit and the `Warn` name the budget.
  * A new subsection, "The byte budget":
    * why it exists, about 10 GiB under the count alone;
    * what counts and until when;
    * the 64 MiB default and its raise to `WithMaxMessageSize`;
    * each policy at it, and that requests' bytes alone never pause the reader;
    * a request's refusal;
    * `QueuedBytes()`, through `Connection()` on a side connection;
    * a code block that raises the budget.
* `docs/architecture.md`:
  * a paragraph and list on the counters, `queueNotification`, `overByteBudget`, `admitRequestBytes` and
    `releaseRequestBytes`, and why requests' bytes never trigger a notification's policy;
  * the checks list names `connection_queue_bytes_test.go`.
* `example_api_test.go`: `ExampleWithMaxQueuedBytes`, with the `bufio` and `strings` imports.
  * It sets a connection to `WithMaxMessageSize(200)` and `WithMaxQueuedBytes(200)`.
  * It holds a 167-byte notification in its handler, prints `QueuedBytes()`, then sends a request that would pass the budget,
    and prints the refusal the connection writes.
  * It releases the handler and prints `QueuedBytes()` after `Wait`.
  * Its output is fixed, and it passed `-race -count=3`.

No step was skipped and nothing deviated.

**Seen to fail.** In a scratch copy, with `acp.WithMaxQueuedBytes(200)` removed, the example fails: the budget is the 64 MiB
default, and the request is answered `{"jsonrpc":"2.0","id":1,"result":{}}`, against the refusal it wants.

**Checks.**

* `go test -race -count=1 ./...`: every package `ok`.
* `go vet ./...`: clean.
* `make pre-add-check`: "go-precheck: 279 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)."
* `make check-records`, `make markdownlint` and `make disclosure-check`: exit 0.
* `tools/idscan.py --paths` over the three changed files: 0 matches.

The owner committed phase 2 as `1e4a198`.

### Phase 3, 2026-10-31

**Step 3.1, the gates.** `tools/gates.py --all` on `1e4a198`, the tree clean, exit 0:

```text
ok   regen (exit 0)
ok   precheck (exit 0)
  newfiles: no new .go files
ok   newfiles (exit 0)
ok   nojsonv2 (exit 0)
ok   winvet (exit 0)
ok   records (exit 0)
ok   mdlint (exit 0)
ok   idscan (exit 0)
ok   tools (exit 0)
ok   lint (exit 0)
ok   fix (exit 0)
ok   tidy (exit 0)
ok   api (exit 0)
ok   race3 (exit 0)
ok   floor (exit 0)
ok   platforms (exit 0)
ok   windows (exit 0)
ok   linux (exit 0)
all gates passed
```

* `precheck`: "go-precheck: 279 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)."
* `api`: "compatible: (*Connection).QueuedBytes: added"; "compatible: WithMaxQueuedBytes: added"; "against v1.29.0: 0
  incompatible, 2 compatible".
* `windows` and `linux`: the whole tree from source, `go1.27.3` on each host, every package `ok`, the root package included.

`make push-guard`, after a fetch: "4 outgoing commit(s), 9fcbb6f..1e4a198, main to origin"; "the guard passed (exit 0)".

**Step 3.2, CI.** The owner pushed `fd8e586`, `04d6673`, `7b87714` and `1e4a198`; `origin/main` is `1e4a198`.
`tools/watch_ci.py --branch main 1e4a198`, exit 0:

```text
watch_ci: 2 run(s) for 1e4a198: 41242795883, 41242795930
watch_ci: run 41242795883 (main, push): completed success; watch exit 0
  completed success: docs
watch_ci: run 41242795930 (main, push): completed success; watch exit 0
  completed success: validate (windows-2025, 1.27.3)
  completed success: validate (macos-15, 1.27.3)
  completed success: validate (macos-15, 1.26.10)
  completed success: validate (ubuntu-24.04, 1.26.10)
  completed success: validate (windows-2025, 1.26.10)
  completed success: validate (ubuntu-24.04, 1.27.3)
```

**Step 3.3, the MADR's Confirmation.**

| # | Item | Shown by |
| :--- | :--- | :--- |
| 1 | Under `OverflowBlock`, lines past the budget with count room pause the reader; all delivered in order, none dropped, open | `TestQueuedBytesPausesReading` |
| 2 | Under 0044's guard the reader reads on to twice the budget, then closes with `ErrNotificationQueueOverflow` | `TestQueuedBytesReadOnLimit` |
| 3 | `OverflowCloseConnection` and `OverflowDropNewest` close or drop at the budget | `TestQueuedBytesCloseAndDrop`, its two subtests |
| 4 | A budget below `WithMaxMessageSize` is raised to it; one maximum-size line is accepted with nothing counted | `TestQueuedBytesClampAndSingleLine`, its two subtests; `TestQueuedBytesDefault/a_larger_message_size` |
| 5 | The counted bytes return to 0 when every handler has returned, and `QueuedBytes()` reports them | `TestQueuedBytesPausesReading` (notifications), `TestQueuedBytesRequestsDoNotPause` (a request), `TestQueuedBytesRefusesRequest` (nothing counted for a refusal); `ExampleWithMaxQueuedBytes` |
| 6 | A request past the budget is answered `-32603` with the limit, and its handler does not run | `TestQueuedBytesRefusesRequest`; `ExampleWithMaxQueuedBytes` |
| 7 | Requests' bytes alone never pause or refuse a notification; a request handler awaiting the peer gets its response | `TestQueuedBytesRequestsDoNotPause` |
| 8 | The existing queue, barrier, overflow and hold tests pass unchanged | `git diff --stat 04d6673..1e4a198 -- '*_test.go'`: only `connection_queue_bytes_test.go`, new, and `example_api_test.go`, one example added; those tests pass in `race3` and on both hosts |
| 9 | `make api` reports only compatible additions | the `api` gate above |
| 10 | `tools/gates.py --all` and CI pass | step 3.1; step 3.2 |

*Refinement, not a deviation.* Building the table showed three cases that phase 1's breaks did not reach: one subtest, one
assertion, and one subtest that the no-clamp break reached but was run only against its sibling. Each was planted now, in a
scratch copy, on `1e4a198`:

| Break | Test | Failure seen |
| :--- | :--- | :--- |
| the trigger without its notification gate, at `>=` | `TestQueuedBytesClampAndSingleLine/one_line_of_the_message_size` | "0 notifications queued, ended true (acp: connection closed: notification queue overflow); want 1 and open" |
| a refused request's bytes counted | `TestQueuedBytesRefusesRequest` | "QueuedBytes = 1200 after the refusal, want 900" |
| no clamp | `TestQueuedBytesDefault/a_larger_message_size` | "byte budget 67108864, want 104857600" |

Every new test and subtest has now been seen to fail on a planted break, and the example was seen to fail in phase 2. No test or
code changed. With CI on `1e4a198`, every item of the Confirmation is shown.

**Step 3.4, the status lines.** The MADR stays `accepted`; its "Chosen option (proposed)" loses the leftover "(proposed)". The
index rows are unchanged: this PLAN stays `in-progress` until phase 4 ends, as 0044's did.

The owner committed phase 3's record and asked for phase 4: "committed, proceed with phase 4 include the 0011 fix in this
release".

**Deviation 2, 2026-10-31, phase 4: the release carries 0011's fix.**

* *Found.* `04d6673`, 0011's added phase, sits between `v1.29.0` and this PLAN's commits. Its own PLAN left a patch, `v1.29.1`,
  to the owner's ask, which never came.
* *Decided.* The owner chose to ship it in `v1.30.0`.
* *Changed here.* The release notes gain a "Fixed" section for `RequestId`, beside the byte budget. `tools/consumer.py` also
  decodes `{}` into an `acp.RequestId` and prints the error, so the published release is checked for the fix from the proxy.
  0011's PLAN carries the matching entry. No file is added to the phase's scope; the MADR is unchanged, since no decision of
  0045 moves.

### Phase 4, 2026-10-31: before the tag

**Step 4.1, `tools/consumer.py`.**

* The overflow connection also sets `acp.WithMaxQueuedBytes(256<<20)`. Once it is initialized, the program reads
  `oc.Connection().QueuedBytes()`, and after the overflow line prints `queued bytes 0`. It then decodes `{}` into an
  `acp.RequestId` and prints the error, `request id RequestId: no variant matches` (deviation 2).
* `WANT_RUN` gains those two lines; the docstring names this PLAN and says `VERSION` is `v1.30.0` or later.
* `tools/README.md`: `consumer.py`'s row names the budget, `QueuedBytes`, `RequestId`, and `v1.30.0`.
* *Before the tag.* A scratch script built the program and test against `70b947f`'s tree through a `replace`, at `go1.26.10`,
  three times: `go mod tidy`, the build, the run, `go test` and the Windows build, each exit 0; "output equals WANT_RUN: True".
* *Seen to fail, twice.*
  * `tools/consumer.py v1.29.0`, the published release without the budget: exit 1, `go build` "./main.go:241:7: undefined:
    acp.WithMaxQueuedBytes" and "oc.Connection().QueuedBytes undefined", "NOT AS REQUIRED".
  * A scratch export of `70b947f` with `fd8e586`'s `types_gen.go`, 0011's fix reverted: "first difference at line 29: got
    'request id <nil>', want 'request id RequestId: no variant matches'", "NOT AS REQUIRED". Lines 1 to 28 equal `WANT_RUN`.
* `make check-tools`, exit 0: "Ran 58 tests" and "Ran 32 tests", both "OK"; "check-ci-filters: 0 problem(s)".

**Step 4.2, the release notes.**
[0045-REPORT-release-notes-v1-30-0.md](../reports/0045-REPORT-release-notes-v1-30-0.md), and its row in `docs/README.md`. Its first
section is the new default bound: what counts, the default and its raise to the message size, each policy at it, requests' bytes
never pausing the reader, a request's refusal, the logs, `QueuedBytes`, and raising it in a code block. A "Fixed" section follows
for `RequestId` (deviation 2). Its sources: the eight commits since `v1.29.0`, the five Go files changed outside the tests, the
unchanged areas, `make api` on `70b947f` ("0 incompatible, 2 compatible"), CI on `1e4a198`, and the consumer before the tag.

**Left, each on the owner's ask:** the push of this commit and its CI; the tag `v1.30.0`, the GitHub release from
`tools/release_body.py`, `tools/publication.py`, and `tools/consumer.py v1.30.0`; then this PLAN and its row `complete`, and 0011's
added phase noted as released.

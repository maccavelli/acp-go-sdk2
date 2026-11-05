---
status: in-progress
date: 2026-11-05
associated-madr: "0049-MADR-remediate-the-2026-11-01-debugging-pass.md"
---

# Implement 0049, the runtime: R1 to R10

Associated MADR: [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md)

## Goal

Fix the connection runtime's ten findings, R1 to R10, as the MADR states them and as the owner answered on 2026-11-01: Q2 (a)
for R2, and the bound for R1 under "R1's and R2's designs". Each fix lands with its probe as a committed test, seen to fail on
`5c3344c`'s code and pass after. The first of the MADR's five PLANs (Q7 a).

Base: `5c3344c`, whose Go code is `v1.31.1`'s. Every file and line named below is at that commit.

*Amended 2026-11-05, for R11.* The MADR's R11, found by the tooling PLAN's close and traced on 2026-11-04, is the same
topic, so this PLAN carries it: phases 5 and 6 below, per Q20 (a), answered by the owner on 2026-11-05 with the severity
medium. Their base is `c69499d`, whose Go code is `v1.33.0`'s; every file and line in them is at that commit. Phases 1 to 4
stand as executed.

## Scope

In scope:

* `connection.go`, `middleware.go`, `observer.go`, `serve.go`: the fixes and their godoc.
* `cmd/generate/internal/emit/dispatch.go` (`:211`) and its test `dispatch_test.go` (`:221`), and the regenerated
  `agent_gen.go`: R2's one line in `NewAgentSideConnection`.
* New tests, one file per finding or area, in the root package: `connection_answers_test.go` (R1),
  `connection_initialize_guard_test.go` (R2), and cases added to the existing files named under each step.
* `README.md` ("What the chains do not see", "Closing and limits"), `docs/architecture.md`, and the amendment to
  [0046-MADR-sending-and-receiving-middleware.md](0046-MADR-sending-and-receiving-middleware.md) that R1 needs.
* The release, a patch `v1.31.2`, on the owner's ask: `make api` must report no change.

Out of scope: the other four areas' findings (their PLANs follow, Q7 a); any exported identifier; the 1.26.0 schema.

In scope for phases 5 and 6, R11 (amended 2026-11-05):

* `connection.go`: `queueNotification` (`:1875-1935`), `processNotifications` (`:2137-2161`), the queue's capacity in
  `buildConnection` (`:924-927`), `WithMaxQueuedNotifications`' godoc (`:592-605`), and one unexported option and field, the
  test's seam.
* Tests: a new `connection_overflow_boundary_test.go`; `connection_queue_size_test.go`'s capacity assertions (`:30-34`,
  `:169-171`, and its custom-size test's), which state the capacity this phase changes.
* `docs/architecture.md`'s overflow section (`:485-511`); `README.md` only where it names the channel's capacity; the
  amendment to [0044-MADR-notification-queue-back-pressure.md](0044-MADR-notification-queue-back-pressure.md)'s D4.
* The release, a patch `v1.33.1`, on the owner's ask: `make api` must report no change. Its notes name
  [0049-PLAN-tooling-fixes.md](0049-PLAN-tooling-fixes.md) too, which had no release of its own.
* Out of scope: the byte budget and `waitForRoom`'s pause, which count already (the MADR's R11); any exported identifier.

## Implementation Steps

Every phase ends staged for the owner, who commits. A phase begins only on the owner's word. Each new test is the finding's probe
from the pass, under a descriptive name, seen to fail on a planted break in a scratch copy (`tools/plants.py`), never in the tree:
the plant restores the code the test guards against. Every phase runs `make pre-add-check` with its files, 0047's check.

### Phase 1: R1, the reader never waits on its peer

1. **The test first,** `connection_answers_test.go`:
   * `TestPeersRefusingEachOtherDoNotDeadlock`: the pass's probe, two connections over `io.Pipe`, each with
     `WithMaxConcurrentRequests(1)`, one handler held, 8 requests each way; after the release every call returns within its
     deadline, and the refused ones are answered `-32603`. With `limit=0` as a control.
   * `TestFaultAnswersDoNotBlockTheReader`: a peer that never reads its input sends invalid lines; the connection keeps reading,
     and a valid notification after them reaches its handler.
   * `TestUnreadAnswersCloseTheConnection`: a peer that never reads sends 1 025 invalid lines; the connection ends, `Err()` names
     the unread peer (an unexported cause, wrapped in `ErrConnectionClosed`), and nothing blocks.
   * Each runs in a synctest bubble where its timing allows.
2. **The queue.** In `connection.go`:
   * `Connection` gains `answers` (a slice of `writeRequest` values' bytes, or of `anyMessage` with their observation),
     guarded by its own mutex, with a wake channel of capacity one; and `answerLoop`, started in `start` (`:927`) under
     `c.goroutines`, which takes each queued answer in order, writes it through the writer (`sendMessage` with
     `context.Background()`, as `answer` does now, `:1957-1963`), then calls the observer, and ends with the connection.
   * `answer` (`:1957`) and `answerObserved` (`observer.go:126`) append to the queue and return without waiting. Past
     `maxPendingAnswers = 1024` queued, the reader closes the connection with `closeWith(errUnreadAnswers)`, an unexported error
     whose text names a peer that does not read its input.
   * `handleInbound`'s nil-handler branch (`:2207-2215`) runs on a request's goroutine, not the reader, and keeps its direct
     write.
   * The answers queued when the connection ends are dropped, as a write after the end is today.
3. **The godoc and docs.**
   * `WithMaxConcurrentRequests` (`:637`), `WithMaxQueuedBytes`, `Shutdown`: the reader neither waits for a handler nor for its
     peer; a refusal is written by the connection's answering goroutine.
   * `WithObserver` (`observer.go:92-110`) and the `ObservedFault` and `ObservedRefused` constants: they are reported on the
     answering goroutine, after the answer is written; the order of the 2026-11-01 amendment holds.
   * `README.md`'s "What the chains do not see": the same.
   * [0046-MADR-sending-and-receiving-middleware.md](0046-MADR-sending-and-receiving-middleware.md): an amendment under D9,
     dated, naming this PLAN, saying where those two kinds now run.
4. **The existing tests** that assert the observer runs on the reader for these kinds (`TestObserverNoLockHeld`,
   `TestObserverReportsEachKind`, `TestObserverRunsAfterActing`) are read; any that depends on the reader's goroutine is changed
   only where the new contract says otherwise, and the record names it.
5. **Seen to fail:** the queue bypassed (answer written on the reader again): the deadlock test times out; the cap removed: the
   unread-peer test does not end; the observer called before the write: `TestObserverRunsAfterActing` fails.
6. **Benchmarks:** `BenchmarkPromptRoundTrip` and `BenchmarkSessionUpdateNotify` against `5c3344c`, step 1.1 of the 0046 PLAN's
   method: allocations equal, time within noise; the answer path is not on them.
7. **Checks:** `go test -race -count=3` of the new and the observer tests; `make pre-add-check` with the phase's files; the
   record; staging.

### Phase 2: the small fixes, R5 to R9

Each with its test, in the file named:

1. **R5** (`connection_queue_bytes_test.go`, `TestByteBudgetAtMaxInt`): `overByteBudget` (`:2066-2072`) doubles with saturation
   at `math.MaxInt64`. The test: `WithMaxQueuedBytes(math.MaxInt)`, a held notification and a second; the connection stays open.
2. **R6** (`connection_queue_size_test.go`, `TestQueueSizeAtMaxInt`): `WithMaxQueuedNotifications` is clamped to a documented
   maximum, `1 << 20`, before the doubling (`:865-868`), so the queue's channel stays allocatable; its godoc names the maximum
   and the memory it costs. The test: `math.MaxInt` builds a connection.
3. **R7** (`jsonrpc_test.go`, `TestMaxMessageSizeCRLF`): `readLine` (`:1494-1513`) measures the line without its `\r\n` or
   `\n`. The test: 50 bytes under a limit of 50, with `\n` and with `\r\n`, both accepted; 51 refused both ways.
4. **R8** (`connection_sending_middleware_test.go`, a `TestSendingMiddlewareMisuse` subtest): `structuredParams`
   (`middleware.go:192-195`) also requires `json.Valid`, and refuses whitespace alone. The test: `{"a":`, `[1,` and `"   "` each
   fail wrapping `ErrSendingMiddleware`, and nothing is written.
5. **R9** (`connection_cancel_test.go`, `TestNoCancelForUnsentRequest`): the request path (`:2539-2550`) sends
   `$/cancel_request` only when `sent`. The test: the pass's probe, a busy writer and a 100 ms deadline; the peer reads no cancel
   for the unsent request, and the observer reports none.
6. **Seen to fail:** each fix reverted in its plant; each test fails with its message.
7. **Checks** as in phase 1.

### Phase 3: R3, R4, R2 and R10

1. **R3** (`connection_notification_barrier_test.go`, `TestBarrierAcrossConnections`): a notification's `messageMark`
   (`:97-100`) gains the `*Connection` that made it; a request's mark stays the zero literal, so it still costs no allocation
   (0046 deviation 1). `waitNotificationsUpTo`'s cap (`:2766-2768`) and `inRunningNotification` (`:2116-2124`) apply only when the
   mark's connection is `c`. The test: the pass's probe; under `x`'s notification context, `y`'s call returns after `y`'s earlier
   notification is handled. The benchmarks: requests' allocations unchanged; a notification's `B/op` may grow by one word,
   recorded.
2. **R4** (`shutdown_test.go`, `TestServeAgentContextEndWaitsForHandlers`): `ServeAgent`'s context branch (`serve.go:47-56`)
   calls `c.WaitContext` under the same bound after `Shutdown`, and wraps `ErrHandlersRunning` when it passes, as the input
   branch does; the godoc says it. The test: the pass's probe; `ServeAgent` returns after `Cancel` ends, or with
   `ErrHandlersRunning`.
3. **R2** (`connection_initialize_guard_test.go`):
   * The emitter (`cmd/generate/internal/emit/dispatch.go:211`) writes, in `NewAgentSideConnection` after `buildConnection`,
     ~~`asc.conn.guardInitialize = true`~~ `asc.conn.guardInitialize = AgentMethodInitialize` (deviation 1, 2026-11-03);
     `dispatch_test.go:221`'s expectation and the regenerated `agent_gen.go` follow. `tools/gates.py regen` is clean after.
   * In `connection.go`, when `guardInitialize` is set and a receiving chain is built, the inner handler is wrapped to note,
     in a cell `handleInbound` puts in the context for `initialize` alone, the result the dispatch returned. After the chain, a
     result other than that one ~~(by interface equality, the dispatch's result being a pointer)~~ (by its encoding:
     deviation 1, 2026-11-03), or a result with no dispatch,
     is answered `-32603` `{"error":"a receiving middleware replaced initialize's result"}` and logged at `Error`; an error from
     the chain is answered as now.
   * `WithReceivingMiddleware`'s godoc states the rule and that the sending side has the same.
   * Tests: a middleware that replaces the result: `-32603`, and the client has no record; one that short-circuits with a
     result: the same; one that fails with an error: that error; one that passes through: the client's and the agent's records
     agree. A raw `NewConnection` with a chain that rewrites a result named `initialize` is not refused.
4. **R10:** `Wait`'s and `Close`'s godoc (`:1035-1040`, `:1087-1093`) name a writer that cannot be interrupted, blocked in `Write`,
   beside the reader; no code change, no test.
5. **Seen to fail:** each of R3, R4 and R2 reverted in its plant.
6. **Checks** as in phase 1, and `go test ./...` in `cmd/generate` for the emitter.

### Phase 4: close, and the release on the owner's ask

1. `tools/gates.py --all`, with the hosts; the MADR's R items each with the test that shows it, in a table.
2. After the owner's push, CI on the head.
3. On the owner's ask: `docs/reports/0049-REPORT-release-notes-v1-31-2.md`, leading with R1; the push; `tools/release_body.py`;
   the owner's tag `v1.31.2` and release (or the job [0048-PLAN-create-releases-in-ci.md](0048-PLAN-create-releases-in-ci.md)
   adds, if it has landed); `tools/publication.py`; `tools/consumer.py v1.31.2`.
4. This PLAN and its row `complete`; the MADR's R findings marked done.

### Phase 5: R11, the overflow decided by the count (amended 2026-11-05)

1. **The seam.** An unexported `ConnectionOption`, `withBeforeDequeue(f func())`, stored in `connectionConfig` and in a
   `Connection` field `beforeDequeue`, set before any goroutine starts. `processNotifications` becomes `for { … queued, ok
   := <-c.notificationQueue; if !ok { return } … }` and calls `beforeDequeue`, when set, before each receive. A test holds
   the loop with it, so that what the loop has taken is the test's to choose. Nothing exported changes; a connection built
   without it runs as before, one nil check a notification.
2. **The admission,** in `queueNotification`, after the sequence number is taken:
   * `pending := seq - c.completedNotificationSeq`, the notifications received and not completed, this one, the one whose
     handler runs and any the loop has not taken included.
   * The limit is `queueLimit + 1`, or `2*queueLimit + 1` under `OverflowBlock`, where the reader reaches it only on the
     read-on path: one for the handler that runs or is next to, and `n`, or `2n`, waiting, as the godoc states.
   * Admitted when `pending` is within the limit and `overByteBudget` does not refuse it, as now. The send to the channel
     then cannot fail, as the channel holds at most the notifications received and not taken; its `default` panics, "an
     admitted notification found the queue full", as the invariant checks beside it do.
   * Refused otherwise, by the rollback and the policy as now: `OverflowDropNewest` drops, the others close.
3. **The capacity,** `buildConnection`: `queueCap` is `n + 1`, or `2n + 1` under `OverflowBlock`. `WithMaxQueuedNotifications`'
   memory sentence becomes `2n + 1` slots, still 64 MiB at the maximum, and the godoc gains that the notification whose
   handler runs or is next to is not among the `n`, whether or not the loop has taken it.
4. **Tests,** `connection_overflow_boundary_test.go`, each under `synctest`, the loop held by the seam:
   * `TestOverflowBoundaryCounted`: after `initialize`, a client with a queue of 1 whose handler blocks, under each policy,
     the loop held before it takes anything: two notifications leave the connection open and, released, both are handled;
     a third closes under `OverflowCloseConnection`, is dropped under `OverflowDropNewest`, and under `OverflowBlock`
     pauses the reader, all three handled once released. The MADR's probe 2, with the boundary's other side.
   * `TestOverflowBlockHeldInitializeLoopBehind`: R11's input, three notifications held for `initialize`'s answer with a
     queue of 1, the loop held before its first receive: the connection stays open, and, released, `Initialize` returns its
     result and the three are handled; a fourth, past `2n + 1`, closes with `ErrNotificationQueueOverflow` (D4).
   * `connection_queue_size_test.go`: the capacities `2n + 1` and `n + 1`, the decision's own numbers.
   * `TestOverflowBlockHeldInitialize` is unchanged: with the fix it no longer depends on the order the loop runs in.
5. **Docs:** `docs/architecture.md`'s overflow section says the policy acts by the count, the channel's capacity, and the
   seam; `README.md` is read for the capacity and changed only where it names it.
6. **Seen to fail,** by `tools/plants.py`: the admission by the channel's room restored, with the capacity `n` and `2n`
   (both new tests fail, as probes 1 and 2 did); the limit without its `+ 1` (two notifications close or drop); the seam's
   call removed (the loop no longer held, the held tests' order lost).
7. **Scheduling no longer matters:** probe 1's delay, the loop sleeping before each receive, applied to the fixed code in a
   `git archive` copy: `TestOverflowBlockHeldInitialize`, `TestHeldNotificationsFillQueue` and the new tests pass, 200
   runs each; and probe 2 under the delay, as the code is: open, 2 handled, under each policy.
8. **Checks:** `go test -race -count=3 -shuffle=on .`; `go test -count=2000 -run '^TestOverflowBlockHeldInitialize$' .`;
   `go test -run '^$' -bench 'Notification' -benchmem .` before and after, allocations per operation unchanged; `make api`,
   no change against `v1.33.0`; `make pre-add-check` with the phase's files; `tools/remote.py tree --host linux`;
   `tools/remote.py pkg --host windows . -test.run 'Overflow|QueueSize|HeldNotifications' -test.count=3`.

### Phase 6: close of R11, and the release on the owner's ask (amended 2026-11-05)

1. `tools/gates.py --all`, with the hosts, and R11 with its tests, in a table.
2. After the owner's push, CI on the head.
3. On the owner's ask: `docs/reports/0049-REPORT-release-notes-v1-33-1.md`, leading with R11 and naming the tooling PLAN;
   the push; `tools/release_body.py`; the owner's tag `v1.33.1` and release; `tools/publication.py`; `tools/consumer.py
   v1.33.1`.
4. This PLAN and its row `complete`; the MADR's R11 marked done.

## Verification

* Each of R1 to R9 has a committed test that fails at `5c3344c`'s code and passes after; R10 is godoc.
* Two SDK peers refusing each other complete every call; a peer that never reads closes the connection past 1 024 answers.
* `make api`: no change against `v1.31.1`.
* The benchmarks' allocations per operation are unchanged for requests.
* `tools/gates.py --all` and CI pass.
* *Amended 2026-11-05, R11:* the overflow policy acts on the same input the same way whatever the order the loop runs in:
  the two new tests, with the loop held, and probe 1's delay on the fixed code, pass; each fails with the admission by the
  channel's room restored. `make api`: no change against `v1.33.0`.

## Rollout and Rollback

* **Rollout.** A patch, `v1.31.2`. What a consumer meets: refusals no longer stall the reader; past 1 024 unread answers the
  connection closes; the observer for faults and refusals runs off the reader; a receiving middleware that replaces
  `initialize`'s result on an agent is refused; `ServeAgent` at its context's end waits for handlers as it does at the input's end;
  `WithMaxQueuedNotifications` is clamped. The notes lead with these.
* **Rollback.** Revert the phase's commit; each phase stands alone, phase 1 included.
* *Amended 2026-11-05, R11:* a patch, `v1.33.1`. What a consumer meets: at the queue's limit a notification is no longer
  refused, by a close or a drop, because the loop had not yet taken the one before; none is refused earlier than before;
  the channel holds one slot more. Rollback: revert phase 5's commit.

## Execution record

### Phase 1: R1, 2026-11-02

Begun on the owner's "proceed" after `9f59ee2`. The MADR is `accepted`.

**What it did:**

* **The queue.** `Connection` gained `answersMu`, `answers` (`[]queuedAnswer`), `answersEnded` and `answerSignal`. `start`
  starts `answerLoop` under `c.goroutines`. `answerObserved` (`observer.go`) queues and signals, and no longer writes.
  `answerLoop` takes the first answer, writes it through `answer` (unchanged: `sendMessage` with `context.Background()`), pops it,
  then calls `observe`. The answer being written stays in the queue until it is written, so the bound counts it:
  `maxPendingAnswers = 1024` unwritten, and the next closes the connection with `closeWith(errUnreadAnswers)`, whose text is "the
  peer does not read its input: more than 1024 answers unwritten". `handleInbound`'s `nil`-handler branch is unchanged.
* **Details the steps did not name**, each needed for the steps' own outcome. None changes a decision or an exported identifier:
  * `EndedCleanly` returns false for `errUnreadAnswers`. Its godoc already says it is true only for `Close` or the peer's end
    of output, and false "for any other error"; without the case, `ErrConnectionClosed` alone would make it true.
  * Each queued answer counts in `inboundRequests`, so `drainInput`, at the end of input, waits for the queued answers as it
    does for the requests received before it. Without it, a peer that sends a fault and ends its output loses the answer, which
    the reader used to write before it read the end. A fourth test pins it, `TestFaultAnswersWrittenAtEndOfInput`.
  * When the connection ends, `answerLoop` reports the answers still queued, unwritten, and sets `answersEnded`. After that the
    reader reports a fault or refusal itself, since nothing can be written. So every message is still observed once (D9), the
    1 025th in `TestUnreadAnswersCloseTheConnection` included.
* **The tests,** `connection_answers_test.go`, each in a synctest bubble: `TestPeersRefusingEachOtherDoNotDeadlock` (limits 0
  and 1: 2 answered and 14 refused, or 16 answered), `TestFaultAnswersDoNotBlockTheReader`, `TestFaultAnswersWrittenAtEndOfInput`
  and `TestUnreadAnswersCloseTheConnection`.
* **Step 4, the observer tests.** `TestObserverNoLockHeld` failed once the queue was in: `observed [dropped unmatched_response
  refused cancel_received], want [dropped refused unmatched_response cancel_received]`. The refusal is now reported on
  `answerLoop` after its write, so the next kind, reported on the reader, could come first. The test now waits
  (`synctest.Wait()`) after reading the refusal; that is the only change. `TestObserverReportsEachKind` and
  `TestObserverRunsAfterActing` are unchanged and pass.
* **The godoc:**
  * `WithMaxConcurrentRequests`: the reader waits for neither a handler nor its peer, and the connection closes past 1024
    unwritten answers.
  * `WithMaxQueuedBytes` and `Shutdown`: the same for their refusals.
  * `WithObserver`: where each kind runs.
  * `ObservedFault` and `ObservedRefused`: reported off the reader, once written.
  * `EndedCleanly`: the new case.
* **The docs:**
  * `README.md`: "Closing and limits", `WithMaxConcurrentRequests`, and "What the chains do not see".
  * `docs/architecture.md`: a paragraph on the queue, and the observer bullet.
  * [0046-MADR-sending-and-receiving-middleware.md](0046-MADR-sending-and-receiving-middleware.md): D9 annotated, and
    "Amendment, 2026-11-02: where faults and refusals are observed".

**Step 5, seen to fail:** `tools/plants.py` with five plants, each in its own copy of the tree, 5/5 as required:

```text
FAILED AS REQUIRED: R1 queue bypassed: the reader writes its refusals (^TestPeersRefusingEachOtherDoNotDeadlock$): exit 1: connection_answers_test.go:110: a call returned {"code":-32603,"message":"Internal error","data":{"error":"context deadline exceeded"}} after the held requests were released: the readers waited on each other
FAILED AS REQUIRED: R1 queue bypassed: the reader writes its fault answers (^TestFaultAnswersDoNotBlockTheReader$): exit 1: connection_answers_test.go:154: the notification after the faults did not happen
FAILED AS REQUIRED: R1 cap removed (^TestUnreadAnswersCloseTheConnection$): exit 1: connection_answers_test.go:227: the end of the connection did not happen
FAILED AS REQUIRED: R1 observer before the write (^TestObserverRunsAfterActing$): exit 1: connection_observer_test.go:393: fault: the observer ran before the fault's answer was written
FAILED AS REQUIRED: R1 end of input does not wait for the queued answers (^TestFaultAnswersWrittenAtEndOfInput$): exit 1: connection_answers_test.go:191: answer 0 did not happen
```

The first two plants restore the old write on the reader. On the first run, two plants failed their tests but not with the
wanted text. The deadlock test counted a timed-out call, which `SendRequest` returns as a `-32603`, among the refusals ("0
answered and 16 refused"). The fault test's sends blocked the bubble's main goroutine, so synctest panicked with a deadlock.
The test now checks `context.DeadlineExceeded` first, and sends from its own goroutine, so each failure says what broke.

**Step 6, the benchmarks:** a `git archive 5c3344c` in the scratch directory and the tree, alternately, three rounds of
`-count=5`; medians of 15:

| Benchmark | ns/op base, tree | B/op base, tree | allocs/op base, tree |
| :--- | :--- | :--- | :--- |
| `PromptRoundTrip` | 31573, 31407 | 5439, 5438 | 99, 99 |
| `PromptRoundTripMiddleware` | 32176, 32706 | 5801, 5801 | 102, 102 |
| `SessionUpdateNotify` | 18412, 17852 | 4879, 4877 | 65, 65 |
| `SessionUpdateNotifyObserved` | 18113, 17818 | 4878, 4878 | 65, 65 |

Allocations are equal in every run, and times are within noise.

**Step 7, the checks:**

* `go test -race -count=3 -run 'TestPeersRefusingEachOtherDoNotDeadlock|TestFaultAnswers|TestUnreadAnswersCloseTheConnection|TestObserver' .`:
  `ok`.
* `make pre-add-check FILES="connection.go observer.go connection_answers_test.go connection_observer_test.go README.md"`: exit
  0, "4 file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on, uncached), tidy, govulncheck."
* `tools/remote.py pkg . -test.run 'Answers|PeersRefusing|Observer' -test.count=3` on the Linux and Windows hosts: "PASS", exit 0,
  on each.
* `make check-records`, `make markdownlint` ("0 issues in 0 files"), `make disclosure-check`: exit 0 each.
* `tools/idscan.py --paths` over the phase's ten files: "10 file(s), 17 pattern(s), 0 match(es)".

Staged for the owner, who commits. Phase 2 begins on the owner's word.

The owner committed phase 1 as `fd4dcaf`.

### Phase 2: R5 to R9, 2026-11-03

Begun on the owner's "committed proceed" after `fd4dcaf`.

**What it did,** each fix with its test, in the file the step names:

* **R5:** `overByteBudget` doubles the budget for the read-on only up to `math.MaxInt64/2`, and above it uses `math.MaxInt64`.
  `TestByteBudgetAtMaxInt` runs budgets of `math.MaxInt` and `math.MaxInt/2 + 1`: one notification held, two more queued, and
  the connection stays open with three lines counted. The godoc of `WithMaxQueuedBytes` and the README add that `math.MaxInt`
  sets no practical budget.
* **R6:** `maxQueuedNotificationsLimit = 1 << 20`, applied in `buildConnection` before the doubling. `WithMaxQueuedNotifications`'s
  godoc names the maximum and the memory: 32 bytes a slot, 2n slots under `OverflowBlock`, 64 MiB at the maximum. The README
  names the maximum. `TestQueueSizeAtMaxInt` builds a connection with `math.MaxInt`, giving a limit of `1 << 20` and a channel
  of `2 << 20`.
* **R7:** `readLine` measures the line without a trailing `"\n"` and then one `"\r"`, so `"\r\n"` counts as `"\n"` does. It
  trims one `"\r"` only, not every trailing `"\r"`, so a peer cannot grow a line past the limit with carriage returns.
  `WithMaxMessageSize`'s godoc names both newlines. `TestMaxMessageSizeCRLF` uses a limit of 100, since `lineOf` cannot build a
  50-byte line: 100 bytes is read with either newline, and 101 is answered `-32600` with either.
* **R8:** `structuredParams` accepts empty params, and otherwise an object or an array that is valid JSON. White space alone is
  refused. `WithSendingMiddleware`'s godoc says "and valid JSON". There are three new cases in `TestSendingMiddlewareMisuse`'s
  table, `{"a":`, `[1,` and `"   "`, through a helper, `passingParams`; each fails with `-32603` wrapping
  `ErrSendingMiddleware`, writes nothing, and leaves nothing pending.
* **R9:** `exchangePrepared` sends `$/cancel_request` only when `sent`. `TestNoCancelForUnsentRequest`, in a synctest bubble:
  the writer is held by an unread first request, and the second request's context ends before the writer takes it. The second
  returns `-32800`, the peer then reads only the first request, and the observer reports no cancel sent.
* `docs/architecture.md`: the saturating double and the clamp.

**Seen to fail:** `tools/plants.py`, five plants, each the fix reverted in its own copy of the tree, 5/5 as required:

```text
FAILED AS REQUIRED: R5 the doubling overflows again (^TestByteBudgetAtMaxInt$): exit 1: connection_queue_bytes_test.go:323: WithMaxQueuedBytes(4611686018427387904): the connection closed with notifications waiting: acp: connection closed: notification queue overflow
FAILED AS REQUIRED: R6 the clamp removed (^TestQueueSizeAtMaxInt$): exit 1: [the stack of] panic: makechan: size out of range
FAILED AS REQUIRED: R7 the line measured with its CR (^TestMaxMessageSizeCRLF$): exit 1: jsonrpc_test.go:273: a line of 100 bytes, ending \r\n, answered null {"code":-32600,"message":"Invalid request","data":{"error":"message exceeds 100 bytes"}}, want -32601 with id 1
FAILED AS REQUIRED: R8 the first byte checked alone (^TestSendingMiddlewareMisuse$): exit 1: connection_sending_middleware_test.go:380: the call = {"code":-32603,"message":"Internal error","data":{"error":"json: error calling MarshalJSON for type *jsontext.Value: unexpected end of JSON input"}}, want -32603 wrapping ErrSendingMiddleware
FAILED AS REQUIRED: R9 the cancel sent unsent (^TestNoCancelForUnsentRequest$): exit 1: connection_cancel_test.go:815: the peer read {"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":2}}, want only the first request
```

On the first run, R5's plant failed with `io: read/write on closed pipe` instead of the wanted text: the connection closed on
the second notification, and the test's third send then failed. The two sends after the first now use `trySend`, so the
check that follows reports the close.

**Checks:**

* `go test -race -count=3 -run 'TestByteBudgetAtMaxInt|TestQueueSizeAtMaxInt|TestMaxMessageSize|TestSendingMiddlewareMisuse|TestNoCancelForUnsentRequest' .`:
  `ok`.
* `make pre-add-check` with the phase's Go files and the README:
  * **First run:** exit 2. `golangci-lint` found one `perfsprint` issue, `fmt.Sprint(budget)` in `TestByteBudgetAtMaxInt`'s
    subtest name, now `strconv.Itoa`. The tests also failed in `agentproc`, a package the phase does not touch:
    `TestBlockedTapDetached` and `TestDetachedTapIsPrefix` ("the bulk agent did not say it was ready within 30s"), and
    `TestInProcessDetachedTapIsPrefix` ("the connection did not read Serve's first 56 MiB within 30s"). The host's load average
    was 73 to 93, from `fseventsd` and Spotlight. The three tests pass alone, and the whole package passes with that run's
    seed, `-shuffle=1793720620141291000`.
  * **Second run,** after the lint fix, at load 57 to 88: exit 2. `remote`'s `TestHTTPServerRouting` failed, a package the
    phase does not touch: "Post …: net/http: HTTP/1.x transport connection broken: http: CloseIdleConnections called".
  * As the MADR's "How the pass was run" did with failures under the same load, these are set aside, not treated as findings.
    The evidence is below.
  * **Third run,** started once the 1-minute load fell below 20 (19.7): exit 0, "7 file(s) gofmt-clean; both modules
    linted, vetted and tested (-race -shuffle=on, uncached), tidy, govulncheck."
* `tools/remote.py tree --host linux`: both modules, every package `ok`, exit 0.
* `tools/remote.py pkg --host windows . -test.run '…the five tests…' -test.count=3`: "PASS", exit 0.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0 each; `tools/idscan.py --paths` over the
  phase's files: 0 matches.

Staged for the owner, who commits. Phase 3 begins on the owner's word.

The owner committed phase 2 as `022d0d1`.

### Phase 3: R3, R4, R2 and R10, 2026-11-03

Begun on the owner's "committed, proceed" after `022d0d1`.

**Deviation 1, 2026-11-03: R2's comparison, and how the guard is set.**

* **Found.** Step 3.3 compares the chain's result with the dispatch's "by interface equality, the dispatch's result being a
  pointer". The dispatch returns `&resp`, a `*InitializeResponse`. A receiving middleware that changes that struct in place
  keeps the pointer, so interface equality passes it, while the client receives what the agent's record does not hold. That
  is R2 itself.
* **Decided.** The owner was asked, with three options, and chose the recommended one: compare the encoded bytes.
  `noteInitialize` encodes the dispatch's answer once, for `initialize` alone, and `handleInbound` refuses a chain's result that
  encodes otherwise. Not chosen: interface equality; identity first, with the bytes for the same pointer. The MADR's R2 design
  carries an amendment of the same date.
* **Two consequences, found while building it.** Neither changes a decision:
  * `guardInitialize` is a `string`, initialize's method, not a `bool`. The generated constructor sets it to
    `AgentMethodInitialize`. As first written, `connection.go` named the generated constant, and `TestEngineBehaviour` failed:
    each engine case generates from a schema without `initialize`, and the runtime "does not type-check …: undefined:
    AgentMethodInitialize". The hand-written runtime names no generated identifier. The emitter takes the constant's name
    from `m.Initialize.Const`, and `TestSideConnectionsFollowTheModel` matches it with a regular expression, since its renamed
    model names the constant otherwise.
  * `buildConnection` wraps the dispatch in `noteInitialize` whenever a receiving chain is built, not only on the agent's side:
    `guardInitialize` is set after `buildConnection` returns, as step 3.3 says. On any other message, the step costs one
    `ctx.Value` lookup per received message, and only under a chain.

**What it did:**

* **R3:**
  * `messageMark` gains `conn *Connection`. `processNotifications` sets it, and a request's mark stays the zero literal.
  * `inRunningNotification` and `waitNotificationsUpTo`'s cap apply only when the mark's connection is `c`. So
    `beginPeerWait`, which `exchangePrepared` calls under `inRunningNotification`, no longer counts another connection's
    notification as a wait of `c`'s.
  * The test: `TestBarrierAcrossConnections`, in `connection_notification_barrier_test.go`, in a synctest bubble, with a helper,
    `connectedPair`. Under `context.Background()` and under `x`'s notification context alike, the request on `y` is still
    waiting once the bubble is idle, and returns after `y`'s notification has run.
* **R4:**
  * `ServeAgent`'s context branch calls `WaitContext(bound)` after `Shutdown`, unless `Shutdown` already met the bound, and
    joins its `context.DeadlineExceeded`. The existing wrap then names `ErrHandlersRunning`, "… 5s after the context's end".
  * The godoc and the README say it waits for every handler, a notification's included.
  * The test: `TestServeAgentContextEndWaitsForHandlers`, in `shutdown_test.go`, with `slowCancelAgent` from the probe, in
    synctest bubbles.
    * A `Cancel` released within the bound: `ServeAgent` returns after it, with the context's cause alone.
    * Never released: it returns at 5 s with the cause and `ErrHandlersRunning`.
* **R2:**
  * The emitter, `dispatch.go`: `side.guardsInitialize` holds the constant's name. `agentSide` sets it from `initializeConst(m)`,
    and the constructor emits `asc.conn.guardInitialize = <it>` between `buildConnection` and the `ConnAware` hand-off.
  * Regenerated: `agent_gen.go`, one line.
  * `connection.go`:
    * `receivingChain` and `guardInitialize` fields.
    * `handleInbound` gives an agent's `initialize` request, under a chain, an `initializeGuard` in its context.
    * After encoding, it answers `-32603` `{"error":"a receiving middleware replaced initialize's result"}`, logged at
      `Error`, when the guard was not noted or its bytes differ.
  * `middleware.go`: `initializeGuardKey`, `initializeGuard` with `answered`, and `noteInitialize`. The godoc of
    `WithReceivingMiddleware` states the rule; the README does too.
  * The tests, `connection_initialize_guard_test.go`:
    * `TestInitializeGuardRefusesAChangedAnswer`: a replaced result, one changed in place, and one without the dispatch. Each
      gives `-32603` with the reason, and the client has no record.
    * `TestInitializeGuardAllowsAnErrorAndAPassThrough`: the middleware's own error is answered; a pass-through leaves both
      sides' records the same.
    * `TestInitializeGuardOnlyOnTheAgent`: a raw `Connection`'s chain may replace a result for a method named `initialize`.
* **R10:** `Wait`'s godoc names a writer `Close` cannot close, blocked in a `Write` to a peer that does not read. `Close`'s
  godoc says a `Write` to a writer without `io.Closer` is not interrupted, and `Wait` waits for it. There is no code change and
  no test, as the step says.
* `docs/architecture.md`: `ServeAgent`'s wait, the mark's connection and the barrier, and the receiving chain's guard.

**Seen to fail:** `tools/plants.py`, four plants, each in its own copy of the tree, 4/4 as required:

```text
FAILED AS REQUIRED: R3 another connection's mark cuts the barrier (^TestBarrierAcrossConnections$): exit 1: connection_notification_barrier_test.go:590: the request on y returned while y's notification before its response was still running: the barrier was skipped
FAILED AS REQUIRED: R4 ServeAgent does not wait at its context's end (^TestServeAgentContextEndWaitsForHandlers$): exit 1: shutdown_test.go:435: ServeAgent = the test's signal while the agent's Cancel still runs
FAILED AS REQUIRED: R2 the guard not applied (^TestInitializeGuardRefusesAChangedAnswer$): exit 1: connection_initialize_guard_test.go:80: Initialize = <nil>, want -32603 naming the replaced result
FAILED AS REQUIRED: R2 the guard by identity: an answer changed in place passes (^TestInitializeGuardRefusesAChangedAnswer$): exit 1: connection_initialize_guard_test.go:80: Initialize = <nil>, want -32603 naming the replaced result
```

* **The fourth plant** keeps the noted value and compares its re-encoding, which is what interface equality sees. It fails
  `a_result_changed_in_place` alone, the case deviation 1 is about. On its first run it only dropped the comparison, and so
  failed every case. That plant was replaced.
* **The emitter's plant** ran by a scratch script, since `plants.py` runs from the tree's root and the emitter is in the
  nested `cmd/generate` module. In a copy of the tree, the emitter stopped setting the guard. `TestSideConnectionsFollowTheModel`
  failed with "with prompt turns: agent_gen.go guards initialize: false, want true".

**The benchmarks,** `HEAD` (`022d0d1`) against the tree, two runs of three alternating rounds of `-count=5`:

| Benchmark | allocs/op | B/op, run 1 | B/op, run 2 | ns/op, run 1 | ns/op, run 2 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `PromptRoundTrip` | 99, 99 | 5438, 5436 | 5437, 5440 | 30538, 27764 | 30992, 31967 |
| `PromptRoundTripMiddleware` | 102, 102 | 5801, 5804 | 5801, 5803 | 33333, 33457 | 31682, 33437 |
| `SessionUpdateNotify` | 65, 65 | 4877, 4886 | 4878, 4886 | 15126, 18118 | 18721, 18112 |
| `SessionUpdateNotifyObserved` | 65, 65 | 4878, 4886 | 4878, 4887 | 17189, 18464 | 18000, 17281 |

* Allocations are equal everywhere.
* A notification's mark grew by one word, `+8` to `+9` B/op, as step 3.1 expected. A request's did not.
* The first run's `SessionUpdateNotify` time, 20% higher, did not repeat: the second run has the tree faster. Both runs were at
  a load average of 13 to 37.

**Checks:**

* `go test -race -count=3` of the phase's tests: `ok`. `go test -race -shuffle=on -count=1 .`: `ok`.
* `go test ./...` in `cmd/generate`: `ok`, after the fix to the type-check failure above.
* `make pre-add-check` with the phase's nine Go files and the README, started at load 18.5: exit 0, "9 file(s) gofmt-clean;
  both modules linted, vetted and tested (-race -shuffle=on, uncached), tidy, govulncheck."
* `make api`: "against v1.31.1: 0 incompatible, 0 compatible".
* The generator run again: no further change.
* `tools/remote.py pkg --host windows . -test.run 'TestBarrierAcrossConnections|TestServeAgent|TestInitializeGuard'
  -test.count=3`: "PASS", exit 0.
* `tools/remote.py tree --host linux`: both modules, exit 0.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0 each; `tools/idscan.py --paths` over the
  phase's files: 0 matches.

Staged for the owner, who commits. Phase 4, the close and the release, begins on the owner's word.

The owner committed phase 3 as `2ed2aa6`.

### Phase 4: the close, 2026-11-03

Begun on the owner's "committed, proceed" after `2ed2aa6`.

**Step 1, the gate set:** `tools/gates.py --all`, with the Linux and Windows hosts named per command, at a load average of 6
to 22: every gate `ok`, exit 0, "all gates passed". The gates were `regen`, `precheck`, `newfiles` ("no new .go files"),
`nojsonv2`, `winvet`, `records`, `mdlint`, `idscan`, `tools`, `lint`, `fix`, `tidy`, `api`, `race3`, `floor`, `platforms`,
`windows` and `linux`.

The MADR's R findings, each with what shows it:

| Finding | Shown by | File |
| :--- | :--- | :--- |
| R1 | `TestPeersRefusingEachOtherDoNotDeadlock`, `TestFaultAnswersDoNotBlockTheReader`, `TestFaultAnswersWrittenAtEndOfInput`, `TestUnreadAnswersCloseTheConnection` | `connection_answers_test.go` |
| R2 | `TestInitializeGuardRefusesAChangedAnswer`, `TestInitializeGuardAllowsAnErrorAndAPassThrough`, `TestInitializeGuardOnlyOnTheAgent`; the emitter's `TestSideConnectionsFollowTheModel` | `connection_initialize_guard_test.go`, `cmd/generate/internal/emit/dispatch_test.go` |
| R3 | `TestBarrierAcrossConnections` | `connection_notification_barrier_test.go` |
| R4 | `TestServeAgentContextEndWaitsForHandlers` | `shutdown_test.go` |
| R5 | `TestByteBudgetAtMaxInt` | `connection_queue_bytes_test.go` |
| R6 | `TestQueueSizeAtMaxInt` | `connection_queue_size_test.go` |
| R7 | `TestMaxMessageSizeCRLF` | `jsonrpc_test.go` |
| R8 | `TestSendingMiddlewareMisuse`, its three new cases | `connection_sending_middleware_test.go` |
| R9 | `TestNoCancelForUnsentRequest` | `connection_cancel_test.go` |
| R10 | godoc of `Wait` and `Close`; no test, as step 3.4 says | `connection.go` |

Each test was seen to fail on its fix reverted, in phases 1 to 3's plants.

Steps 2 to 4 wait on the owner: the push, CI on the head, and the release when asked.

**Step 2, CI.** The owner committed the record as `427c40d` and pushed `d55ba2a..427c40d` after `make push-guard` passed
for its five code and record commits. The owner then said "committed and pushed, proceed". `tools/watch_ci.py 427c40d
--branch main` read two runs:

* CI's run `41720647295`, "completed success", six `validate` jobs on Ubuntu, Windows and macOS at Go `1.27.3` and
  `1.26.10`;
* the Docs workflow's run `41720647338`, "completed success".

**Step 3, the release, begun.**

* [0049-REPORT-release-notes-v1-31-2.md](../reports/0049-REPORT-release-notes-v1-31-2.md) drafts the release text. It leads
  with R1, and each statement names its commit.
* `make api` on `427c40d`: "against v1.31.1: 0 incompatible, 0 compatible".
* The report is staged with its row. `tools/release_body.py v1.31.2 <rev>` runs once it is committed, before the tag.

The owner committed the report as `69eff66` and pushed it, then tagged `v1.31.2` on `69eff66` and pushed the tag. After the
consumer check below, the owner created the release with `gh release create v1.31.2 --title v1.31.2 --notes-file <the body>`,
which printed its URL.

* `tools/release_body.py v1.31.2 69eff66`: exit 0, "77 lines, 1 links"; `idscan` over the body, 0 matches. The one link names
  the MADR at the tag.
* **CI,** read by `tools/watch_ci.py 69eff66`:
  * the tag's run `41729591119`, "completed success", six `validate` jobs on Ubuntu, Windows and macOS at Go `1.27.3` and
    `1.26.10`;
  * the Docs workflow's run `41728264484` on `main`, "completed success".
* `tools/consumer.py v1.31.2`: exit 0, "as required". The module at the floor, `go 1.26.10`, requires `v1.31.2`, its test is
  `ok`, and its Windows build exits 0.
* `tools/publication.py v1.31.2 69eff66 <the body>`: exit 0, "10 of 10 checks hold". The checks:
  * the release exists, not a draft or prerelease, and is marked latest;
  * its body is the extracted text;
  * the tag is annotated, names `69eff66`, and is on origin's `main`;
  * CI passed on that head and on the tag;
  * the proxy serves the tag at that commit;
  * `go list -m @latest` gives `v1.31.2`;
  * `go mod download` works with the checksum database on.

**Step 4.** This PLAN and its row are `complete`. The MADR's F-R section records R1 to R10 as done in `v1.31.2`. The MADR stays
`accepted`, with four PLANs to come (Q1 a, Q7 a).

**Verification,** against the section above:

* R1 to R9 each have a committed test that failed with its fix reverted and passes now. R10 is godoc. See the table in step 1.
* Two SDK peers refusing each other complete every call (`TestPeersRefusingEachOtherDoNotDeadlock`). A peer that never reads
  closes the connection past 1,024 answers (`TestUnreadAnswersCloseTheConnection`).
* `make api`: no change against `v1.31.1`.
* Requests' allocations per operation are unchanged in every benchmark run of phases 1 and 3.
* `tools/gates.py --all`, 18 of 18, and CI on `427c40d`, `69eff66` and the tag, pass.

Not done here, by the MADR's plan: the surface, subpackage, tooling and generator findings, each in its own PLAN, written when
its turn comes.

### Phase 5: R11, the overflow decided by the count, 2026-11-05

The owner committed the amendment as `a4d8ef2` and said "committed, proceed".

**Deviation 1, 2026-11-05: step 5.7's delay cannot show what it was to show.**

* **Found:** step 5.7 asked for probe 1's delay, the loop sleeping 1 ms in fake time before each receive, on the fixed code,
  with `TestOverflowBlockHeldInitialize`, `TestHeldNotificationsFillQueue` and the new tests passing 200 runs each. In a copy
  of the working tree: `TestHeldNotificationsFillQueue` 200 of 200, and probe 2 20 of 20 open with 2 handled under each
  policy, where before the fix `OverflowCloseConnection` closed and `OverflowDropNewest` dropped, 20 of 20. But
  `TestOverflowBlockHeldInitialize` failed, "connection_overflow_block_test.go:332: Initialize did not return: the reader
  paused on the held notifications", the connection no longer ending; and
  `TestOverflowBlockHeldInitializeLoopBehind/past_it` ended in "deadlock: main bubble goroutine has exited but blocked
  goroutines remain". Both are the probe's: `synctest.Wait` returns while the loop sleeps without advancing the clock, so a
  test that then requires the handlers to have run fails, and one that ends with the loop asleep leaves it blocked in the
  bubble. Without the delay, each passes, 3 of 3 under `-race`. The step's claim was wrong for any `synctest` test that
  waits on the loop, whatever the fix.
* **Decided,** by the owner, from three options (the others: narrow the step to probe 2, the fill-queue test and the held
  tests, records only; both): for the `synctest` tests that wait on the loop, untimed yields in place of the sleep,
  `runtime.Gosched` before each receive, which reorder the goroutines without fake time, with
  `TestOverflowBlockHeldInitialize` run 2000 times on `a4d8ef2`'s code and on the fixed code and the failures counted; probe
  2 and the fill-queue test stand under the timed delay, which they passed.
* **Files added to the phase:** none; the probes are scratch scripts. The MADR's decisions and facts stand: no amendment.

**What it did:**

* **The seam,** step 5.1: `withBeforeDequeue(f func())`, unexported, sets `connectionConfig.beforeDequeue`, which
  `buildConnection` copies to `Connection.beforeDequeue`; `processNotifications` is a `for` with an explicit receive and
  calls it, when set, before each.
* **The admission,** step 5.2: `admitLimit()` is `queueLimit + 1`, or `2*queueLimit + 1` under `OverflowBlock`;
  `queueNotification` admits when `seq - completedNotificationSeq` is within it and `overByteBudget` does not refuse, and
  the send's `default` unlocks and panics, "an admitted notification found the notification queue full". Refused, it
  rolls back and meets the policy as before. Its comment, and `notificationQueue`'s, say so.
* **The capacity,** step 5.3: `queueCap++` after the doubling. `WithMaxQueuedNotifications`' godoc says the one running or
  next to run is not among the `n`, and the slots are `n + 1`, or `2n + 1`.
* **The tests,** step 5.4:
  * `connection_overflow_boundary_test.go`, new: `heldLoop(t)` holds the loop until its `release`, which the test's end
    calls too, once, so a failed test leaves no goroutine held in its bubble. `TestOverflowBoundaryCounted`, its subtests
    `close`, `drop` and `block`; `TestOverflowBlockHeldInitializeLoopBehind`, its subtests "at the limit", three
    notifications, and "past it", four.
  * `connection_queue_size_test.go`: the default's channel `2n + 1`, and `TestQueueSizeAtMaxInt`'s; a new
    `TestQueueChannelCapacity`, the channel and `admitLimit` at `n = 7` under each policy, 15, 8 and 8. The Scope's "its
    custom-size test's" was wrong: `TestWithMaxQueuedNotifications_CustomSize` checks `queueLimit` alone, and no test
    checked the `n + 1` of the other two policies, so the table test holds both.
* **Docs,** step 5.5: `docs/architecture.md`'s overflow section, the admission by the count, `admitLimit`, the panic, the
  seam, `2n + 1`, the read-on's close, and the byte budget's overflow path; its tests list names
  `connection_overflow_boundary_test.go`. `README.md` names no capacity, "twice the queue's size" still true of the
  waiting: unchanged.

**Seen to fail,** step 5.6, by `tools/plants.py`, 3/3:

```text
FAILED AS REQUIRED: the admission by the channel's room, capacity n and 2n, restored (^(TestOverflowBoundaryCounted|TestOverflowBlockHeldInitializeLoopBehind)$): exit 1: connection_overflow_boundary_test.go:142: three held notifications, the loop behind: the connection ended: acp: connection closed: notification queue overflow
FAILED AS REQUIRED: the limit without its + 1 (^(TestOverflowBoundaryCounted|TestOverflowBlockHeldInitializeLoopBehind)$): exit 1: connection_overflow_boundary_test.go:142: three held notifications, the loop behind: the connection ended: acp: connection closed: notification queue overflow
FAILED AS REQUIRED: the seam's call removed (^TestOverflowBoundaryCounted$): exit 1: connection_overflow_boundary_test.go:78: a third: ended false (<nil>), want true
```

Under the first two, `TestOverflowBoundaryCounted`'s log is probe 2's table: `close`, "two notifications with a queue of
1, the loop behind: ended true (acp: connection closed: notification queue overflow)"; `drop`, "ended false (<nil>),
dropped 1".

**Scheduling, step 5.7 as amended by deviation 1:**

* The timed delay, the loop sleeping 1 ms before each receive, on the fixed code: `TestHeldNotificationsFillQueue`, 200 of
  200; probe 2, 20 of 20 "closed=false handled=2" under each policy.
* Untimed yields, `runtime.Gosched()` three times before each receive, `TestOverflowBlockHeldInitialize` `-count=2000`:
  on `a4d8ef2`'s code, 303 of 2000 failed, each "connection ended before response"; on the fixed code, 0 of 2000.
* The same under `-race`, `-count=500`: 0 of 500 on either. The race detector's scheduling hides the order, which the
  yields alone bring about; the two failures in the tree were under `-race` at load 24 to 43.

**Checks,** step 5.8:

* `go test -race -count=3 -shuffle=on .`: `ok`, 44.4 s. `go test -count=2000 -run '^TestOverflowBlockHeldInitialize$' .`:
  `ok`.
* `go test -run '^$' -bench 'SessionUpdateNotify|SessionNotificationUnmarshal' -benchmem -count=5 .`, on `a4d8ef2`'s code
  and the tree: allocations per operation 65, 65 and 39 each, before and after; the bytes per operation within 13 of
  each other; the times within the noise of load 15 to 30.
* `make api`: "against v1.33.0: 0 incompatible, 0 compatible".
* `make pre-add-check` with the phase's files: first, exit 2, `thelper`, "connection_overflow_boundary_test.go:26:6: test
  helper function should start from t.Helper()", `heldLoop` taking `t` for its cleanup; with `t.Helper()`, exit 0, at
  load 21 to 62: "3 file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on, uncached), tidy,
  govulncheck."
* `tools/remote.py tree --host linux`: both modules, exit 0. `tools/remote.py pkg --host windows . -test.run
  'Overflow|QueueSize|QueueChannelCapacity|HeldNotifications|WithMaxQueuedNotifications' -test.count=3`: "PASS".
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0 each. `tools/idscan.py --paths` over the phase's
  files: 0 matches.

Staged for the owner, who commits. Phase 6 begins on the owner's word.

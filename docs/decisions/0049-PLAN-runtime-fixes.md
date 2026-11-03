---
status: in-progress
date: 2026-11-02
associated-madr: "0049-MADR-remediate-the-2026-11-01-debugging-pass.md"
---

# Implement 0049, the runtime: R1 to R10

Associated MADR: [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md)

## Goal

Fix the connection runtime's ten findings, R1 to R10, as the MADR states them and as the owner answered on 2026-11-01: Q2 (a)
for R2, and the bound for R1 under "R1's and R2's designs". Each fix lands with its probe as a committed test, seen to fail on
`5c3344c`'s code and pass after. The first of the MADR's five PLANs (Q7 a).

Base: `5c3344c`, whose Go code is `v1.31.1`'s. Every file and line named below is at that commit.

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
     `asc.conn.guardInitialize = true`; `dispatch_test.go:221`'s expectation and the regenerated `agent_gen.go` follow.
     `tools/gates.py regen` is clean after.
   * In `connection.go`, when `guardInitialize` is set and a receiving chain is built, the inner handler is wrapped to note,
     in a cell `handleInbound` puts in the context for `initialize` alone, the result the dispatch returned. After the chain, a
     result other than that one (by interface equality, the dispatch's result being a pointer), or a result with no dispatch,
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

## Verification

* Each of R1 to R9 has a committed test that fails at `5c3344c`'s code and passes after; R10 is godoc.
* Two SDK peers refusing each other complete every call; a peer that never reads closes the connection past 1 024 answers.
* `make api`: no change against `v1.31.1`.
* The benchmarks' allocations per operation are unchanged for requests.
* `tools/gates.py --all` and CI pass.

## Rollout and Rollback

* **Rollout.** A patch, `v1.31.2`. What a consumer meets: refusals no longer stall the reader; past 1 024 unread answers the
  connection closes; the observer for faults and refusals runs off the reader; a receiving middleware that replaces
  `initialize`'s result on an agent is refused; `ServeAgent` at its context's end waits for handlers as it does at the input's end;
  `WithMaxQueuedNotifications` is clamped. The notes lead with these.
* **Rollback.** Revert the phase's commit; each phase stands alone, phase 1 included.

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

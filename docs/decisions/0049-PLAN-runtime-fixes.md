---
status: in-progress
date: 2026-11-03
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

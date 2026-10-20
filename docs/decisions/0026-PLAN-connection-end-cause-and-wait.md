---
status: in-progress
date: 2026-10-20
associated-madr: "0026-MADR-connection-end-cause-and-wait.md"
---

# Implement 0026: report why a connection ended, and wait for its goroutines to exit

Associated MADR: [0026-MADR-connection-end-cause-and-wait.md](0026-MADR-connection-end-cause-and-wait.md)

## Goal

Implement the MADR's O1, as the owner decided questions 1 to 7 on 2026-10-20:

* **`Err() error`** on `Connection`, `AgentSideConnection` and `ClientSideConnection`: nil until the connection ends, then why it ended,
  fixed at the first end (questions 1, 2, 6).
* **Testable causes:** `ErrNotificationQueueOverflow` exported, its text unchanged; the peer's EOF wraps `io.EOF`, its text unchanged
  (question 3).
* **`Wait() error`** on the three: unbounded; it returns once the connection has ended and every goroutine it started has returned,
  handlers included, and then returns `Err` (questions 4, 5, 6).
* **A call or send after a late `Close`** keeps returning `ErrConnectionClosed` (question 7).

`make api` must report only additions: the six methods and `ErrNotificationQueueOverflow`. One behaviour changes: a failed call's error
after the peer's EOF now matches `errors.Is(err, io.EOF)`. The release is proposed as the minor `v1.18.0`, which the owner confirms at
the tag.

## Facts this plan rests on

Each was read on `0afe75e` on 2026-10-20; step 1 re-asserts them with a script. The MADR's E1 to E6 and G1 to G4 hold the probes.

| For | Fact |
| :--- | :--- |
| Err | The lifetime context is made at `connection.go:630`. `closeWith` (`:741`) marks `closedLocally` (`:744`), then ends it with its cause (`:745`). `shutdownReceive` (`:1463`) ends it with its cause (`:1469`), first `errors.New("connection closed")` when the cause is nil (`:1464`–`:1466`). `endError` (`:712`) is what a call meets after the end. |
| Err | The peer's EOF: `receive` (`:1081`) sets `cause := errors.New("peer connection closed")` (`:1084`), drains (`:1098`), then `shutdownReceive(cause)` (`:1105`). Overflow: `closeWith(errNotificationQueueOverflow)` (`:1357`), then `shutdownReceive(errNotificationQueueOverflow)` (`:1360`). `errNotificationQueueOverflow` is declared at `:32`. |
| Err | The sentinels `ErrPeerDisconnected` and `ErrConnectionClosed` are in `errors.go` (`:138`, `:143`). `Done`'s doc is at `connection.go:2150`–`:2161`. |
| Wait | Every `go` statement in the root's non-test files is in `connection.go`: `start`'s four (`:672`–`:675`), `answerUnlessCancelled`'s two (`:244`, `:256`), the drain waiter (`:1118`), the handler (`:1304`) and the notification drain (`:1486`). `context.AfterFunc` runs at `:671`, `:1949` and `:1993`. The generated files hold none. |
| Wait | `inboundRequests` is a `sync.WaitGroup` already (`:346`). `sync.WaitGroup.Go` exists at the floor, go1.26.9. |
| sides | The emitter writes the side connections' `DroppedNotifications`, `Done`, `Close`, `SetLogger` and `Connection` in `emitSideConnection` (`cmd/generate/internal/emit/dispatch.go:166`, the methods at `:225`–`:255`); `TestSideConnectionsFollowTheModel` (`emit/dispatch_test.go:180`) pins them. The generator's stub of the hand-written code, `handWritten` in `cmd/generate/main_test.go`, declares `Connection`'s `Done` and `Close` (`:179`, `:181`). |
| sides | All 73 accept engine cases list `func (*AgentSideConnection).Close() error` in their `want.api`, so each gains the two methods of each side. |
| docs | The README's "Closing and limits" is at `README.md:519`. |
| — | `scripts/check_records.py --next` gives `0027`; this PLAN takes the MADR's number. |

## Scope

In scope:

* **The runtime:** `connection.go` (the recorded end, `Err`, `Wait`, the wait group), `errors.go` (`ErrNotificationQueueOverflow`, the
  peer's EOF cause).
* **The generator:** `emit/dispatch.go` (the side methods), its tests and pins, the stub in `main_test.go`; the 73 accept cases'
  `want.api`, by `-update`.
* **The regenerated files:** `agent_gen.go`, `client_gen.go`.
* **Tests:** new root tests below.
* **Docs:** `Done`'s doc, the README's "Closing and limits", `docs/architecture.md`.
* **The records:** this PLAN and its row; the release notes `docs/reports/0026-REPORT-release-notes-v1-18-0.md` and their row.

Out of scope:

* a bounded `Wait`, or a channel form (question 4's alternatives);
* changing what a call returns after the end (question 7);
* the agent-process helper 0023 left, which stays a record of its own.

## Design

### `Err` and the recorded end

* **`endErr atomic.Pointer[error]`** on `Connection`, set once, by whichever end runs first, before the context ends:
  * `closeWith` records `ErrConnectionClosed`, wrapping its cause when the cause is not `ErrConnectionClosed`, as `endError` does today;
  * `shutdownReceive` records `ErrPeerDisconnected`, wrapping its cause when it is not `ErrPeerDisconnected`.

  "First" is a `CompareAndSwap` from nil. A `Close` after the peer's end records nothing, so E4's answer stays the peer's.
* **`Err`** returns nil while `c.ctx.Err()` is nil, and the recorded error after. The record is stored before the context ends, so a
  caller that has seen `Done` closed reads it.
* **`endError` stays as it is,** for calls and sends (question 7).
* **`ErrNotificationQueueOverflow`** replaces `errNotificationQueueOverflow`, moved to `errors.go` beside the other sentinels, its text
  "notification queue overflow" unchanged.
* **The peer's EOF** becomes `errPeerEOF`, a value of an unexported type whose `Error` returns "peer connection closed" and whose
  `Unwrap` returns `io.EOF`. `receive` starts from it in place of `errors.New("peer connection closed")`.

### `Wait`

* **`goroutines sync.WaitGroup`** on `Connection`. Each of the nine `go` statements becomes `c.goroutines.Go(…)`: the four of `start`,
  `answerUnlessCancelled`'s two, the drain waiter, the handler and the notification drain. Each starts from `start`, before `Wait` can
  run, or from a goroutine already counted, so the counter is above zero then, as `sync.WaitGroup` requires.
* **The `context.AfterFunc` callbacks are not counted** (the MADR's correction of 2026-10-20): one on a caller's context can start after
  the count has reached zero.
* **`Wait`:** `<-c.Done()`, then `c.goroutines.Wait()`, then `return c.Err()`.
* **A guard:** a root test parses the non-test, non-generated files and fails on any `go` statement, so a goroutine cannot start outside
  the wait group unnoticed.

### Docs

* **`Err`'s and `Wait`'s docs** as the MADR specifies; `Done`'s doc names both.
* **The side connections:** the emitter writes `Err` and `Wait` after `Close`, each delegating, each doc naming the connection's.
* **The README's "Closing and limits"** gains a paragraph and a short example: `Close`, then `Wait`, then `errors.Is` on the error for
  `ErrNotificationQueueOverflow` or `io.EOF`.
* **`docs/architecture.md`:** the connection's lifetime names the recorded end and the wait group.

### Tests

* **`TestErr`,** a case per end, through `NewConnection`, as the MADR's probe did, each also through `Done`: before the end, nil;
  `Close`; the peer's EOF, with `errors.Is(…, io.EOF)`; a read error, with `errors.Is` reaching it; overflow, with
  `errors.Is(…, ErrNotificationQueueOverflow)`; the peer's EOF then `Close`, still the peer's, while a call then returns
  `ErrConnectionClosed`; the same answer on every call.
* **`TestWait`:** after `Close` with an `io.Pipe`, it returns `Err` and no goroutine of the connection is left; with a reader that has no
  `Close` and blocks, it does not return within 200 ms of `Close`, and returns once the read does; with a handler that ignores its
  context, likewise; before the end, it blocks.
* **`TestGoStatementsCounted`:** the guard above.
* **`TestSideConnectionsErrAndWait`:** each side's `Err` and `Wait` return its connection's.
* **The emitter:** `TestSideConnectionsFollowTheModel`'s pins include the two methods.

## Implementation Steps

Each step ends with its gates and an execution record. Agents stage; a commit or a push on `main` needs the owner's explicit
permission in that turn (`AGENTS.md`, "Commits"). Every new test or check is seen to fail on a planted break in a scratch copy, never in
the tree, before it is relied on.

1. **Records:** this PLAN and its row `in-progress`; the facts above re-asserted by a script.
2. **`Err`:** the recorded end, `Err`, `ErrNotificationQueueOverflow`, the peer's EOF cause; `Done`'s doc; `TestErr`.
3. **`Wait`:** the wait group and the nine `go` statements; `Wait`; `TestWait` and `TestGoStatementsCounted`.
4. **The side connections:** the emitter's two methods and their docs; the stub; regeneration; the 73 accept cases' `want.api` by
   `-update`, checked to gain only the four lines; the pins; `TestSideConnectionsErrAndWait`.
5. **Docs:** the README and `docs/architecture.md`.
6. **Release notes:** `docs/reports/0026-REPORT-release-notes-v1-18-0.md` and its row, with `make api`'s report.
7. **Verification and close:** the gates over the tree; Windows over SSH and `make push-guard` before the push; after CI's six jobs pass,
   this PLAN and its row `complete`; the owner tags `v1.18.0` and publishes its release.

## Verification

Per code step:

* phase 1's gates on go1.27.2: regeneration, `make pre-add-check`, `GOEXPERIMENT=nojsonv2`, `GOOS=windows go vet`,
  `make check-records`, `make markdownlint`, the identifier scan;
* `make api` against `v1.17.1`: no incompatible change; from step 4, the seven additions and no other;
* the go1.26.9 gate;
* Windows over SSH, before the push.

Planted breaks, at least. Each must be seen to fail on the check named.

| For | Break, in a scratch copy | The check that must fail |
| :--- | :--- | :--- |
| Err | `Err` returning `endError()` before the end | `TestErr`, before the end |
| Err | `Err` returning `endError()` after it, not the record | `TestErr`, the peer's EOF then `Close` |
| Err | `closeWith` recording even when an end was recorded | `TestErr`, the peer's EOF then `Close` |
| Err | the overflow closing with `ErrConnectionClosed` alone | `TestErr`, overflow |
| Err | `errPeerEOF`'s `Unwrap` removed | `TestErr`, the peer's EOF |
| Err | a call after a late `Close` returning the record | `TestErr`, the call after the late `Close` |
| Wait | `Wait` not waiting for `Done` | `TestWait`, before the end  (cannot fail: see the deviation during step 3) |
| Wait | the handler's goroutine started with `go`, outside the wait group | `TestWait`, the handler; and `TestGoStatementsCounted` |
| Wait | `Wait` not waiting for the wait group | `TestWait`, the reader without `Close` |
| sides | the emitter's `Err` returning nil | `TestSideConnectionsErrAndWait`; the accept cases' `want.api` |
| sides | the emitter's `Wait` not emitted | the 73 accept cases' `want.api`; the pins |

Expected `make api` against `v1.17.1`: compatible only, `ErrNotificationQueueOverflow`, `(*Connection).Err`, `(*Connection).Wait`,
`(*AgentSideConnection).Err`, `(*AgentSideConnection).Wait`, `(*ClientSideConnection).Err`, `(*ClientSideConnection).Wait`.

Acceptance:

* every step's checks pass and its record is written;
* every claim the new docs make is mapped to a test that makes it;
* the leak guard passes; `make lint` reports 0 issues and `go fix -diff` prints nothing;
* CI's six jobs pass on the pushed commits;
* `make api` reports exactly the seven additions.

## Rollout and Rollback

Each step is one commit, reverted by reverting it; step 4 reverts the generated methods with the emitter's.

For a consumer:

* six methods and a sentinel are added;
* after the peer's EOF, a failed call's error now also matches `errors.Is(err, io.EOF)`; its text is unchanged;
* a connection's goroutines start through a wait group, with no change to what they do.

The release is proposed as the minor `v1.18.0`, which the owner confirms at the tag.

**Points for the owner's review**, each with the plan's choice:

* **`ErrNotificationQueueOverflow` keeps its text,** "notification queue overflow", without the "acp: " the other sentinels carry, so
  no message changes (question 3).
* **The record is set by `CompareAndSwap`,** so the first end wins even when `Close` and the peer's EOF race.
* **The guard refuses every `go` statement** in the root's hand-written files: a future goroutine must use the wait group.
* **The callbacks of `context.AfterFunc` stay uncounted,** as the MADR's correction says.

## Execution record

### Step 1, records, 2026-10-20

The owner committed the MADR and this PLAN as `3be2b1f` and approved the PLAN ("committed, proceed"). This PLAN and its row are
`in-progress`.

**The facts,** re-asserted on `3be2b1f` by a script outside the tree: 33 of 33 hold. It checks each line this PLAN cites by its
content, and:

* the nine `go` statements of the root's non-test files are exactly those named, all in `connection.go`;
* no `Err`, `Wait` or `ErrNotificationQueueOverflow` exists yet;
* all 73 accept engine cases list `func (*AgentSideConnection).Close() error`;
* `scripts/check_records.py --next` gives `0027`.

**The script, planted.** In a scratch copy of `3be2b1f`, with `errNotificationQueueOverflow` renamed `ErrNotificationQueueOverflow`, it
exits 1, "31 of 33 facts hold": "MISS connection.go:32 the overflow error" and "MISS no Err, Wait or ErrNotificationQueueOverflow yet".

Gates: `make check-records`, `make markdownlint` and the identifier scan, each exit 0.

### Step 2, `Err`, 2026-10-20

The owner committed step 1 as `26b7f27` and approved this step. What changed:

* **`errors.go`:** `ErrNotificationQueueOverflow`, exported beside the other sentinels, its text "notification queue overflow"
  unchanged; `peerEOF`, whose `Error` returns "peer connection closed" and whose `Unwrap` returns `io.EOF`, and its value
  `errPeerEOF`.
* **`connection.go`:**
  * `errNotificationQueueOverflow`'s declaration is gone and its three uses name the export;
  * `receive` starts from `errPeerEOF` in place of `errors.New("peer connection closed")`;
  * `endErr atomic.Pointer[error]`, set by `recordEnd`, called by `closeWith` (`ErrConnectionClosed`) and `shutdownReceive`
    (`ErrPeerDisconnected`) before each ends the context, with `CompareAndSwap` from nil, so the first end wins;
  * `Err`, as specified; it falls back to `endError` should the context end with nothing recorded, which no path does today;
  * `Done`'s doc ends "Err then says why it ended."
* **Tests renamed to the export,** in scope as the rename's: `acp_test.go`, `connection_close_test.go`,
  `connection_overflow_policy_test.go`.
* **`connection_end_test.go`, new:** `TestErr`, a case per end. Before the end: nil. `Close`: "acp: connection closed". The peer's EOF:
  "acp: peer disconnected: peer connection closed", wrapping `io.EOF`, and a later send's error wraps `ErrPeerDisconnected` and
  `io.EOF`. A read error: wrapped. Overflow: "acp: connection closed: notification queue overflow", wrapping
  `ErrNotificationQueueOverflow`. The peer's EOF then `Close`: `Err` unchanged, a send `ErrConnectionClosed`. Every `Err` is read
  twice, with `errors.Is` on the two, which a freshly built error fails. It passed 3 runs of 3 under `-race`.

**Found while linting:** errorlint refused `!=` between the two reads of `Err`; `errors.Is(again, err)` keeps the identity the test
means, since two errors built apart do not match, and the plant "not the record" still fails it.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| `Err` is nil until the end, non-nil once `Done` is closed | `TestErr`, "before the end", and every case through `Done` |
| the same each time | `TestErr`'s double read; the plant "not the record" |
| `ErrConnectionClosed` for `Close`, wrapping `ErrNotificationQueueOverflow` for an overflow | the cases "Close" and "notification overflow" |
| `ErrPeerDisconnected` wrapping `io.EOF` or the read's error | the cases "the peer's end of output" and "a read error" |
| a later call's error wraps `ErrNotificationQueueOverflow`, or `io.EOF` | `connection_close_test.go:206`; the send in "the peer's end of output" |
| a call after a late `Close` returns `ErrConnectionClosed` | "the peer's end, then Close" |
| the first end wins | "the peer's end, then Close"; the plant "record overwritten" |

**Planted breaks.** Each ran in its own scratch copy of the working tree. All 6 failed `TestErr` with the text named:

| Break | Failure |
| :--- | :--- |
| `Err` returning `endError()` before the end | "Err before the end = acp: peer disconnected, want nil" |
| `Err` returning `endError()` after it, not the record | "Err returned acp: peer disconnected: peer connection closed, then acp: peer disconnected: peer connection closed"; "Err after a late Close" |
| `recordEnd` storing over a recorded end | "Err after a late Close = acp: connection closed, want the peer's end" |
| the overflow closing with `ErrConnectionClosed` alone | "want it to wrap notification queue overflow" |
| `peerEOF.Unwrap` returning nil | "want it to wrap EOF"; "a send after the peer's end … want ErrPeerDisconnected and io.EOF" |
| `endError` returning the record, for a call | "a send after the late Close = … want ErrConnectionClosed" |

**Gates** on the working tree, go1.27.2, each exit 0:

* regeneration: the generated files unchanged;
* `make pre-add-check`: "151 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; the new test file alone, after its
  last change: "1 file(s) clean";
* `GOEXPERIMENT=nojsonv2 go test ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint` and the identifier scan;
* `make api` against `v1.17.1`: "0 incompatible, 2 compatible", `(*Connection).Err` and `ErrNotificationQueueOverflow`.

**The go1.26.9 gate:** `go version` reports go1.26.9; `go vet` and `go test -race -count=1` pass in both modules, the root's with the
leak guard.

### Deviation, 2026-10-20, during step 3: `Wait`'s wait for `Done` cannot be caught

**Found.** The plant "`Wait` not waiting for `Done`", `<-c.Done()` removed, left `TestWait` passing, exit 0. The wait group cannot reach
zero while the connection runs: `writeLoop` and `sendCancelRequests` return only once its context ends. The wait group alone implies
the end today, so no test can tell the line's absence.

**Decision (owner, 2026-10-20), the recommended resolution.** Keep `<-c.Done()` as `Wait`'s stated contract, with a comment saying
why it stays; record the plant as one that cannot be caught. The verification table's row is annotated. No test changes.

### Step 3, `Wait`, 2026-10-20

The owner committed step 2 as `48d6ab8` and approved this step. What changed:

* **`connection.go`:**
  * `goroutines sync.WaitGroup`, with its comment, beside `endErr`;
  * the nine `go` statements are `c.goroutines.Go(…)`: `start`'s four, `answerUnlessCancelled`'s two, the drain's waiter, the handler,
    whose arguments became the closure's captures (`&m`, `idKey`, `reqCtx`, `own`), and the notification drain;
  * `Wait`, as specified, its `<-c.Done()` commented as the deviation above decided;
  * `Done`'s doc ends "Err then says why it ended, and Wait returns once its goroutines have."
* **`connection_end_test.go`:**
  * `TestWait`: before the end, it has not returned 200 ms on, and returns `ErrConnectionClosed` after `Close`; with a reader `Close`
    closes, it returns `Err`, and no goroutine's stack names the test's reader type; with a reader `Close` cannot close, it has not
    returned 200 ms after `Close`, and returns once the read does; with a handler that ignores its context, likewise, and no stack
    names the handler. Each case uses a type or function no other test uses, so a stack is the test's own.
  * `TestGoStatementsCounted`: fails on a `go` statement in the package's non-test files.
  * Both passed 3 runs of 3 under `-race`, with `TestErr`.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| `Wait` blocks until the connection has ended | `TestWait`, "before the end" |
| it waits for every goroutine, the handlers' included | "a handler that ignores its context"; the plant "a bare go handler" |
| it returns `Err` | "a reader Close closes" |
| it does not end the connection | "before the end": still waiting while the connection runs |
| a reader `Close` cannot close, or a handler ignoring its context, keeps it waiting | the two cases of those names |
| every goroutine goes through the wait group | `TestGoStatementsCounted` |
| the `context.AfterFunc` callbacks are not counted | the MADR's correction, after `sync.WaitGroup`'s doc; not a test |

**Planted breaks.** Each ran in its own scratch copy of the working tree:

| Break | Result |
| :--- | :--- |
| `<-c.Done()` removed from `Wait` | not caught, as the deviation above records: `TestWait` passes |
| the handler started with `go`, outside the wait group | `TestWait`: "Wait returned acp: connection closed while the handler runs"; `TestGoStatementsCounted`: "a go statement; start it with the connection's goroutines.Go" |
| `c.goroutines.Wait()` removed | `TestWait`: "Wait returned acp: connection closed while the reader's Read blocks", and "… while the handler runs" |

The handler plant's runner first expected "Wait returned <nil>"; `Wait` returns `Err`, so the message reads "acp: connection closed".
The expectation was corrected and the plant run again.

**Gates** on the working tree, go1.27.2, each exit 0:

* regeneration: the generated files unchanged;
* `make pre-add-check`: "152 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)";
* `GOEXPERIMENT=nojsonv2 go test ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint` and the identifier scan;
* `make api` against `v1.17.1`: "0 incompatible, 3 compatible", `(*Connection).Err`, `(*Connection).Wait` and
  `ErrNotificationQueueOverflow`.

**The go1.26.9 gate:** `go version` reports go1.26.9; `go vet` and `go test -race -count=1` pass in both modules, the root's with the
leak guard.

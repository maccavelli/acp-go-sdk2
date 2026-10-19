---
status: complete
date: 2026-10-19
associated-madr: "0023-MADR-connection-lifetime-and-test-leaks.md"
---

# Implement 0023: `Close` ends every handler, the leaking tests close their connections, and a leak guard

Associated MADR: [0023-MADR-connection-lifetime-and-test-leaks.md](0023-MADR-connection-lifetime-and-test-leaks.md)

## Goal

Implement the MADR's outcome, as the owner decided it on 2026-10-19:

* **S1:** the SDK adds no timer. The README states what the application owns.
* **C1:** `Close` cancels the notification handlers' context at once, as it already does the request handlers'.
* **T2:** the 10 tests that leak close their connections. A root `TestMain` fails the run when the standard `goroutineleak` profile
  reports a leaked goroutine.

The exported API does not change. The wire bytes do not change. One behaviour changes, C1, and the release notes name it. The release
is proposed as the patch `v1.16.1`, which the owner confirms at the tag.

## Facts this plan rests on

Each was read or measured on `d004e9a`, in the tree or in a scratch copy of it. Step 1 re-asserts the code facts, snippet at line, with
a script, before any code changes.

| For | The code as it is |
| :--- | :--- |
| C1 | `closeWith` (`connection.go:713`) marks the connection closed locally and calls `c.cancel(cause)`. It does not call `c.inboundCancel`. |
| C1 | Request handlers run under `context.WithCancelCause(base)`, with `base` derived from `c.ctx` (`connection.go:1246-1247`). Notification handlers run under `c.inboundCtx` (`:1469`). |
| C1 | `shutdownReceive` (`:1434`) cancels `inboundCtx` only after `waitForNotificationDrain` returns, at most `notificationQueueDrainTimeout`, 5 s, later (`:1455-1460`). It runs only when `receive` (`:1052`) returns. |
| C1 | `Close`'s doc comment (`:700-705`) and the README's "Closing and limits" (`README.md:478-494`) do not mention the notification handlers. |
| T2 | No root test file declares `TestMain`. |
| T2 | The 10 leaking tests, all in `acp_test.go`: `TestConnectionHandlesErrorsBidirectional` (`:472`), `TestConnectionHandlesConcurrentRequests` (`:522`), `TestConnectionHandlesMessageOrdering` (`:592`), `TestConnectionHandlesInitialize` (`:937`), `TestPromptWaitsForSessionUpdatesComplete` (`:1116`), `TestRequestHandlerCanMakeNestedRequest` (`:1225`), `TestExtensionMethods_ClientToAgentRequest` (`:1352`), `TestExtensionMethods_UnknownRequest_ReturnsMethodNotFound` (`:1395`), `TestExtensionMethods_UnknownNotification_DoesNotLog` (`:1422`), `TestExtensionMethods_AgentToClientRequest` (`:1463`). |
| T2 | `pprof.Lookup("goroutineleak")` after the suite counted 80 goroutines, with and without `-race`. A stack scan counted 81: those and the test runner's. |
| T2 | `cmd/generate` builds no `Connection`, so the guard is the root package's only. |

Measured by the probe of the MADR's "What was measured": after `Close`, a notification handler blocked on its context saw it end after
the 5 s drain with an `io.PipeReader`, and not within 6 s with the same pipe behind a type with no `Close`.

## Scope

In scope:

* `connection.go`: `closeWith` and `Close`'s doc comment;
* `acp_test.go`: the 10 tests' cleanup;
* a new root test file, `leak_test.go`, with `TestMain` and its helper;
* `connection_close_test.go`: the C1 test;
* `README.md`, "Closing and limits";
* `docs/architecture.md`, its paragraph on the tests' gates;
* the release notes, `docs/reports/0023-REPORT-release-notes-v1-16-1.md`, and their `docs/README.md` row;
* these records.

Out of scope:

* an SDK timer, keepalive, `Err()`, `Wait()` or process helper (the MADR's S2 to S4 and "Not decided here");
* the end-of-input drain, which C1 leaves as it is;
* `cmd/generate`.

## Design

### C1: `Close` ends every handler

`closeWith` calls `c.inboundCancel(cause)` right after `c.cancel(cause)`, inside its `closeOnce`:

* A running notification handler sees its context end at once, with the cause `ErrConnectionClosed`, or the overflow error.
* `processNotifications` keeps taking queued notifications until the queue closes. Each one now runs under an ended context.
* `shutdownReceive`'s own `inboundCancel(cause)`, at the end of a drain, then does nothing: the first cause stands.
* At end of input without `Close`, nothing changes.

`Close`'s doc comment gains: "Every handler's context ends: a request's and a notification's alike." The README's "Closing and limits"
says the same. It also states S1: the SDK closes no connection on a timer; a request's deadline is its context; a session ends with
`session/close`; and an agent process is the application's to end, closing its input and then escalating after a bound, as the example
client's `stopAgent` does.

### T2: the 10 tests and the guard

**The 10 tests.** Each builds a pair over `io.Pipe`s. Each gains `t.Cleanup(func() { _ = cs.Close(); _ = as.Close() })`, with that
test's own names, after both connections exist. `Close` closes the pipes it was given, which ends the connections at once.

**The guard, `leak_test.go`:**

```go
func TestMain(m *testing.M) {
	code := m.Run()
	if leaked := waitForLeaks(notificationQueueDrainTimeout + 2*time.Second); leaked != "" {
		fmt.Fprintf(os.Stderr, "goroutines leaked by the tests (0023-MADR-connection-lifetime-and-test-leaks.md):\n%s", leaked)
		code = 1
	}
	os.Exit(code)
}
```

`waitForLeaks(bound)`:

* polls `pprof.Lookup("goroutineleak")` every 50 ms until its `Count()` is 0, and then returns "";
* once `bound` has passed, returns the profile written with `debug=1`, which groups identical stacks with their counts.

The bound waits out any connection still draining its input, at most `notificationQueueDrainTimeout`. The margin covers `-race`.

The guard runs after every `go test` of the root package: the tests, the examples, `make bench` and fuzzing alike. It reports even
when the tests failed, so a failure that leaks shows both.

### C1's test

`TestCloseEndsNotificationHandlers` (`connection_close_test.go`) runs two cases, in a `synctest` bubble each:

* the reader is the `io.PipeReader`;
* the reader is that pipe behind a type with only `Read`.

In each, a connection's handler blocks until its context ends. The test sends one notification and one request, then `synctest.Wait()`
until both handlers are blocked. It calls `Close`, `synctest.Wait()`s again, and requires both handlers to have returned with no time
passed. In the second case the bubble's input pipe is closed by `t.Cleanup`, so the reader's goroutine ends with the bubble.

## Implementation Steps

Each step ends with its gates and an execution record. The owner commits it; agents do not commit on `main`. Every new test and guard is
seen to fail on a planted break in a scratch copy, never in the tree, before it is relied on.

1. **Records.** This PLAN, the MADR as decided, and their `docs/README.md` rows; the facts above re-asserted on the tree by a script.
2. **T2.** The 10 tests' `t.Cleanup`, then `leak_test.go`, in that order within the step, so that the suite passes with the guard.
3. **C1.** `closeWith`, `Close`'s doc comment, `TestCloseEndsNotificationHandlers`, the README's "Closing and limits" (C1 and S1), and
   `docs/architecture.md`.
4. **Release notes.** `docs/reports/0023-REPORT-release-notes-v1-16-1.md` and its `docs/README.md` row.
5. **Verification and close.** The gates over the tree, `make push-guard`, and after CI passes on the pushed commits, this PLAN and its
   row `complete`. The owner tags `v1.16.1` and publishes its release from the report.

## Verification

Per code step, through the Python runner outside the tree:

* regeneration: the generated files unchanged;
* `make pre-add-check`, and `scripts/go-precheck.sh` on each new `.go` file;
* `GOEXPERIMENT=nojsonv2 go test ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint` and the identifier scan;
* Windows `go test -count=1 -shuffle=on ./...` in both modules, so the guard runs on Windows too;
* the API listing against `v1.16.0`: no difference.

The tests a step changes that use a context or a bubble also run `-count=1000 -race`: the 10 tests in step 2, and
`TestCloseEndsNotificationHandlers` and the `Close` and end-of-input tests in step 3. The guard runs at the end of each of these runs.

Planted breaks, at least. Each must be seen to fail on the check named. A break that is not seen to fail stops its step, and the step
adds a check that fails on it.

| For | Break | The check that must fail |
| :--- | :--- | :--- |
| T2 | a planted test that builds a pair over `io.Pipe`s and closes nothing | the guard: the run exits 1 and prints `(*Connection).writeLoop` among the stacks |
| T2 | the `t.Cleanup` removed from one of the 10 tests | the guard, with that test run alone |
| T2 | `waitForLeaks` returning without polling, on a suite whose tests end connections by ending their input | the guard reports a connection still draining; this shows the polling is needed. If it does not fail, the record says so and keeps the bound as a margin only. |
| C1 | `closeWith` without `inboundCancel` | `TestCloseEndsNotificationHandlers`, both cases |
| C1 | `inboundCancel` called before `closedLocally` is set | `TestCloseEndsConnection` or the C1 test, from the cause a handler sees; if neither fails, the record says the order is not observable |

Acceptance:

* every step's checks pass and its record is written;
* the guard reports nothing on the root suite, with and without `-race`, with `-count` above 1, and on Windows;
* a `Close` ends a blocked notification handler's context at once, with a reader that is an `io.Closer` and with one that is not;
* the end-of-input tests pass unchanged;
* the API listing against `v1.16.0` shows no difference;
* CI passes on the pushed commits.

## Rollout and Rollback

Each step is one commit, reverted by reverting it. Step 2 changes only tests. Step 3's change is the one line in `closeWith` and its
documentation; reverting it restores the drain-bound end of notification handlers after `Close`. The release is the patch `v1.16.1`,
which the owner confirms at the tag.

## Execution record

### Step 1, records, 2026-10-19

The owner committed the proposed MADR as `88a08ec`, and the decided MADR with this PLAN as `6f49e13`. This step marks the PLAN and its
`docs/README.md` row `in-progress`.

A script outside the tree re-asserted every fact of "Facts this plan rests on" on `6f49e13`, each snippet at its line. All 27 hold.

One check changed while it ran. "`cmd/generate` builds no `Connection`" was first checked as a search for the constructors' names in
its Go files. That found three matches, none a connection built:

* two are expected output in `cmd/generate/internal/emit/dispatch_test.go:199` and `:211`;
* one is a stub declaration, `func NewConnection(…) *Connection { return nil }`, inside the source text of a fixture,
  `cmd/generate/main_test.go:171`.

The fact is checked instead by `go list -deps -test ./...` in `cmd/generate`. Of its 192 packages, none is
`github.com/maccavelli/acp-go-sdk2`: the module cannot build a connection, tests included.

Gates: `make check-records`, `make markdownlint` and the identifier scan, each exit 0.

### Step 2, T2, 2026-10-19

What changed, on `f1c65e9`:

* **The 10 tests** (`acp_test.go`). Each has `t.Cleanup(func() { _ = <client>.Close(); _ = <agent>.Close() })`, with its own names,
  right after it builds its second connection.
  * Two built a connection they discarded, and now name it: `TestConnectionHandlesInitialize`, `_ =` became `as :=`; and
    `TestExtensionMethods_UnknownRequest_ReturnsMethodNotFound`, a bare call became `ag :=`.
  * A script outside the tree inserted the lines. It asserted each anchor by line number and text, and that each lies inside its
    test, before writing.
* **The guard, `leak_test.go`, new.** As "Design", T2 sets out: `TestMain` runs the tests, then `waitForLeaks` polls
  `pprof.Lookup("goroutineleak")` every 50 ms, for at most `notificationQueueDrainTimeout + 2*time.Second`. If any leaked goroutine
  is left, it prints the profile with `debug=1` and the run exits 1.

The root suite with the guard: `go test -count=1 .` exit 0.

Planted breaks, each in a scratch copy of the tree:

| Break | Result |
| :--- | :--- |
| a planted test that builds a pair over `io.Pipe`s and closes nothing | the run exits 1: "goroutines leaked by the tests …", `goroutineleak profile: total 8`, with `(*Connection).writeLoop` among the stacks |
| the `t.Cleanup` of `TestConnectionHandlesInitialize` removed, that test run alone | the run exits 1, `goroutineleak profile: total 8` |
| `waitForLeaks` with its deadline at once, so it reads the profile once, over the whole suite | **passes** |

**The poll is not needed on this suite.**
* The third break passed. A connection whose input has ended, still in its drain, has no goroutine the profile counts as leaked. Its
  goroutines wait on a timer, or on something a running goroutine still reaches, not on a primitive nothing can reach.
* So no test of the suite needs the bound. As the PLAN provides, it is kept as a margin only.
* The cost is one profile read when nothing has leaked. When something has, the bound adds at most 7 s before the run fails.

The 10 tests ran `-race -count=1000 -timeout 0`: exit 0, in 588 seconds. The guard ran at the end and reported nothing.

**A claim of the MADR is not borne out, and is annotated.**
* The MADR's consequence that the accumulated connections "slowed step 7's pass" came from step 7's record.
* Step 7's selection (`Wait|Drain|EndOfInput|HeldBehind|EndedContext|Notification`), run 100 times under `-race` on this step's tree,
  took 131 s. Its record measured 130 s on `6d60d84`, with the leaks.
* So the accumulation costs no measurable time at that scale. The MADR's line carries a dated annotation. The decision does not
  change.

Gates, with the API listing against `v1.16.0`, each exit 0:

* regeneration: the generated files unchanged by a second run;
* `make pre-add-check`, and `scripts/go-precheck.sh` on the new `leak_test.go`;
* `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint`, and the identifier scan;
* the API listing: 2429 lines at `v1.16.0` and in the tree, 0 removed, 0 added;
* Windows, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0. The guard ran there and reported
  nothing.

### Step 3, C1, 2026-10-19

What changed, on `bb9197b`:

* **`connection.go`:**
  * `closeWith` calls `c.inboundCancel(cause)` right after `c.cancel(cause)`, inside its `closeOnce`. Its doc comment says why.
  * `Close`'s doc comment adds: "every handler's context ends, a request's and a notification's alike".
* **`connection_close_test.go`:**
  * `readOnly`, a reader with no `Close`.
  * `TestCloseEndsNotificationHandlers`, as "Design" sets it out. In a `synctest` bubble per case, a request and a notification
    handler wait on their contexts. The test calls `Close`, and requires both contexts to have ended with no time passed, each with a
    cause wrapping `ErrConnectionClosed`. Its two cases are a reader `Close` closes, and that reader behind `readOnly`.
* **`README.md`, "Closing and limits":**
  * `Close` ends every handler's context.
  * A new paragraph states S1: the SDK closes no connection on a timer. A request's deadline is its context, a session ends with
    `session/close`, and an agent process is the client's to end, as the example client's `stopAgent` does.
* **`docs/architecture.md`:** its list of the tests' checks names `leak_test.go`'s guard.

Seen to fail before the change: the test, written first, failed on the tree's `closeWith`. In both cases: "the _test/notification
handler's context was still live after Close".

Planted breaks, each in a scratch copy of the tree:

| Break | Result |
| :--- | :--- |
| `closeWith` without `inboundCancel`, the case of a reader `Close` closes run alone | fails: "the _test/notification handler's context was still live after Close" |
| the same break, the case of a reader with no `Close` run alone | fails, with the same message |
| `inboundCancel` called before `closedLocally` is set, over the `Close` tests and `TestCloseEndsEndOfInputDrain` | **passes** |

**Why the first two end in a deadlock panic.** After their failure, both runs end in "panic: deadlock: main bubble goroutine has
exited but blocked goroutines remain". The goroutines left are:

* the notification handler, waiting on its context;
* the drain, `waitForNotificationDrain`, in `sync.Cond.Wait`, with its 5 s timeout pending.

`go doc testing/synctest`: time advances "if there is such a time and the root goroutine of the bubble has not exited. Otherwise,
there is a deadlock and Test panics." Without the change, the handler would end only after the 5 s drain, as the MADR's probe measured
outside a bubble. Once the test function has returned, the bubble's clock no longer advances to it.

**The order of the two cancels is not observable.** The third break passed. A handler's context ends with the cause given, whichever
cancel runs first, and no test reads `endError` from a notification handler at the moment of `Close`. As the PLAN provides, this is
recorded, and the code keeps the order "Design" sets: the mark, then the connection's context, then the handlers'.

The 12 tests of `connection_close_test.go` and `end_of_input_test.go`, `TestCloseEndsNotificationHandlers` among them, ran
`-race -count=1000 -timeout 0`: exit 0, in 414 seconds, the guard reporting nothing at the end.

Gates, with the API listing against `v1.16.0`, each exit 0:

* regeneration: the generated files unchanged by a second run;
* `make pre-add-check`;
* `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint`, and the identifier scan;
* the API listing: 2429 lines at `v1.16.0` and in the tree, 0 removed, 0 added;
* Windows, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0.

### Step 4, release notes, 2026-10-19

What changed, on `4a769a5`:

* [0023-REPORT-release-notes-v1-16-1.md](../reports/0023-REPORT-release-notes-v1-16-1.md), the draft of the GitHub release text for
  `v1.16.1`, in the shape of the `v1.16.0` notes;
* its `docs/README.md` row.

Facts the notes rest on, each read on `4a769a5`:

* **The schema:** `git diff v1.16.0 HEAD -- schema/` is empty.
* **The API listing:** 2429 lines at `v1.16.0` and at `4a769a5`, 0 removed, 0 added (step 3's gates).
* **The commits** since `v1.16.0` that change Go files outside `cmd/generate`: `bb9197b` and `4a769a5`. The other four, `d004e9a`,
  `88a08ec`, `6f49e13` and `f1c65e9`, change records only (`git show --name-only`).
* **The SDK's non-test files:** `git diff --stat v1.16.0 HEAD -- '*.go' ':!*_test.go' ':!cmd/generate'` lists `connection.go` only,
  9 lines added and 5 removed.
* **The overflow policy:** `closeWith` has two callers, `Close` (`connection.go:708`) and the overflow (`:1332`). The overflow's cause
  is `errNotificationQueueOverflow`, "notification queue overflow" (`:32`), which does not wrap `ErrConnectionClosed`.
  * The draft said the overflow policy "does the same", which could be read as the same cause, `ErrConnectionClosed`.
  * The text was corrected to name the overflow's own cause before the step ended.

The first `make markdownlint` failed on the new report: MD004, the file's lists were written with `*` where the repository's reports
use `-`. The markers were changed, 29 of them, and it passed.

Gates: `make check-records`, `make markdownlint` and the identifier scan, each exit 0. The step changes no Go file.

### Step 5, verification and close, 2026-10-19

**The push.** `make push-guard` ran after a fetch over the six commits not yet pushed, `d004e9a..0a33f4f`, and passed, exit 0. The six
are the MADR and this PLAN, and steps 1 to 4. The owner pushed `0a33f4f`. CI run `39290154750` on it concluded `success` on its first attempt, on
`validate (ubuntu-24.04)`, `validate (macos-15)` and `validate (windows-2025)`.

**The gates over the tree**, on `0a33f4f`, with the API listing against `v1.16.0`, each exit 0:

* regeneration: the generated files unchanged by a second run;
* `make pre-add-check`, whose tests run with the guard;
* `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint`, and the identifier scan;
* the API listing: 2429 lines at `v1.16.0` and in the tree, 0 removed, 0 added;
* Windows, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0.

**Acceptance.** Every criterion in "Verification" is met:

* **Each step's checks** passed, and its record is written.
* **The guard reports nothing on the root suite:**
  * without `-race`: step 2's `go test -count=1 .`;
  * with `-race`: `make pre-add-check`;
  * with `-count` above 1: steps 2 and 3's 1,000-run passes;
  * on Windows: the Windows gate.
* **A `Close` ends a blocked notification handler's context at once**, with a reader that is an `io.Closer` and with one that is not:
  `TestCloseEndsNotificationHandlers`, step 3.
* **The end-of-input tests pass unchanged.** No commit of this PLAN touches `end_of_input_test.go`: its last change is `95d66e3`, in
  [0019-PLAN-idioms-performance.md](0019-PLAN-idioms-performance.md). Its tests ran 1,000 times in step 3.
* **The API listing** against `v1.16.0` shows no difference.
* **CI** passed on the pushed commits.

Not done, as the MADR decided: an SDK timer or keepalive, `Err()`, `Wait()`, and an agent-process helper. Each is left to a record of
its own.

This PLAN and its `docs/README.md` row are `complete`. The MADR stays `accepted`.

**Left to the owner.**

* Tag `v1.16.1`, an annotated tag whose message is its name, as `v1.16.0` is, on this close commit once its CI passes. Its code is the
  same as at `4a769a5`.
* Publish its release from the release text of
  [0023-REPORT-release-notes-v1-16-1.md](../reports/0023-REPORT-release-notes-v1-16-1.md), with its relative links rewritten to the
  tag.

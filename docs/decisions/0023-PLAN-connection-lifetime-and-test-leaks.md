---
status: proposed
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

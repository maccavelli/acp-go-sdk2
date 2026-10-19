---
status: accepted
date: 2026-10-19
decision-makers: Repository owner
---

# Leave a connection's lifetime to the application, make `Close` end every handler, and fail the tests on a leaked connection

## Context and Problem Statement

[0019-PLAN-idioms-performance.md](0019-PLAN-idioms-performance.md), step 7, ran 37 root tests 1,000 times under `-race`. A first
run reached `go test`'s 10-minute timeout. Its goroutine dump showed 1,978 connections whose goroutines stayed blocked:
`writeLoop`, `sendCancelRequests`, `processNotifications` and the pipe reader. The tests left those connections open. The record
named this as outside that PLAN.

The owner asked on 2026-10-19 for that leak to be scoped and specified, and for the SDK's handling of a connection's lifetime to be
assessed:

* should the SDK close connections itself, for example after a timeout;
* or should it leave them alone and depend on the application;
* and how is either implemented in the most idiomatic and robust way.

This record answers all three. Its sources are:

* the code, read on `d004e9a`;
* measurements in scratch copies of the tree, described below;
* the ACP specification and the official ACP SDKs, read on 2026-10-19;
* MCP, LSP, Go JSON-RPC libraries and the Go standard library;
* five local agent codebases: `codex`, `grok-build`, `opencode`, `kilocode` and `pi`, and the earlier Go SDK `coder/acp-go-sdk`.

### How a connection ends today

Read on `d004e9a`:

* **Four goroutines per connection.** `start` (`connection.go:642`) runs `writeLoop`, `sendCancelRequests`, `receive` and
  `processNotifications`.
  * `writeLoop` (`:671`) and `sendCancelRequests` (`:1777`) return when the connection's context ends.
  * `receive` (`:1052`) returns when its reader returns an error or end of file.
  * `processNotifications` (`:1467`) returns when `shutdownReceive` (`:1434`), run by `receive` as it returns, closes the queue.
* **The connection ends on one of three events:**
  * the input ends (end of file or a read error);
  * `Close` (`:706`);
  * the default overflow policy, `OverflowCloseConnection`, which calls `Close`'s body.
* **No timer ends a connection.** The one bound is `notificationQueueDrainTimeout = 5 * time.Second` (`:22`):
  * at end of input, it bounds how long the requests and notifications already received may run (`drainInput`, `:1086`;
    `shutdownReceive`);
  * a request this side sends waits for its own context only.
* **What `Close` does.** `Close` (`closeWith`, `:713`) runs once. It:
  * ends the connection's context with `ErrConnectionClosed`, so `Done()` closes;
  * closes the reader and the writer when they implement `io.Closer`;
  * returns their errors joined, and nil on a later call.

  The README's "Closing and limits" (`README.md:478-494`) states this. It adds that when the peer ends the connection, the transport
  "is the application's to close".
* **The two side connections** expose `Done()` and `Close()` (`agent_gen.go:49`, `:55`; `client_gen.go:43`, `:49`). Neither exposes
  why the connection ended. Each failed call carries it, wrapping `ErrPeerDisconnected` or `ErrConnectionClosed` (`endError`,
  `connection.go:687`).
* **The example client stops its agent process itself.** `stopAgent` (`example/client/main.go:316`) closes the agent's input, waits
  for it to exit, and cancels its context after five seconds, which kills it.

### What was measured, 2026-10-19

Every measurement ran in a scratch copy of `d004e9a`. None touched the tree.

* **Which tests leak.** A `TestMain` added in the copy ran the tests, waited past the drain bound, and counted the goroutines left in
  each connection role. The test binary was built once, and each of the 208 root tests and examples was run alone.
  * 10 leave connections running, all in `acp_test.go`: `TestConnectionHandlesErrorsBidirectional` (`:472`),
    `TestConnectionHandlesConcurrentRequests` (`:522`), `TestConnectionHandlesMessageOrdering` (`:592`),
    `TestConnectionHandlesInitialize` (`:937`), `TestPromptWaitsForSessionUpdatesComplete` (`:1116`),
    `TestRequestHandlerCanMakeNestedRequest` (`:1225`), and the four `TestExtensionMethods_…` tests (`:1352`, `:1395`, `:1422`,
    `:1463`).
  * Each leaves one pair, two connections, so eight goroutines. Every one is blocked in `io.(*pipe).read` or waits for that read to
    end. None is blocked in a write.
  * The other 198 leave none.
    * Most close their pipes' write ends, which ends each connection's input, through `defer` or `t.Cleanup` (for example
      `acp_test.go:705`, `conformance_test.go:1067`).
    * The tests of `Close` (`connection_close_test.go`), one end-of-input test (`end_of_input_test.go:224`) and the two runnable
      examples (`example_agent_test.go:180`, `example_client_test.go:121`) call `Close`.
* **The whole suite** leaves 20 connections, 80 goroutines, per run.
* **Step 7's 1,978** is one of these tests, `TestPromptWaitsForSessionUpdatesComplete`, which its selection matched by "Wait". It
  ran 1,000 times and left two connections each time.
* **What `Close` ends.** A probe test built a connection whose handler blocks until its context ends. It received one notification
  and one request, then called `Close`:

  | Reader | Request handler's context | Notification handler's context |
  | :--- | :--- | :--- |
  | an `io.PipeReader`, an `io.Closer` | ended at once | ended after the 5 s drain, not within 1 s |
  | the same pipe behind a type with no `Close` | ended at once | not ended within 6 s; it ends only when the input ends |

  Request handlers run under the connection's context (`connection.go:1246-1247`). Notification handlers run under a second one,
  `inboundCtx`, which `shutdownReceive` cancels only after the drain, and `shutdownReceive` runs only when `receive` returns. A
  reader `Close` cannot close leaves `receive` and `processNotifications` running, and their notification handler's context live,
  until the application ends the input.
* **The standard library sees the leak.** Go 1.27 made the `goroutineleak` profile generally available (go.dev/doc/go1.27). After
  the whole suite, `pprof.Lookup("goroutineleak")` in the copy's `TestMain` counted 80 leaked goroutines. A plain stack scan counted
  81 goroutines: those 80 and the test runner's.

### What the protocol and comparable projects do

The sources are the ACP site and the official SDKs, cloned on 2026-10-19 at the revisions named; the MCP and LSP specifications; Go
libraries; and local checkouts under the owner's home directory.

* **The ACP specification sets no connection lifetime.**
  * The transports page's stdio sequence ends "Close stdin, terminate subprocess" (agentclientprotocol.com/protocol/transports).
  * Initialization says a client that rejects the agent's version "SHOULD close the connection"
    (/protocol/initialization#version-negotiation). It is the one rule to close a connection.
  * The streamable HTTP and WebSocket RFD says "Liveness detection is up to the implementer". It defers a standard keepalive to
    protocol v2 (/rfds/streamable-http-websocket-transport).
  * Freeing a session's resources is `session/close`, stable since 2026-04-23: "Clients can track what sessions are actively used by
    a user and automatically close old sessions" (/rfds/session-close).
* **No official ACP SDK closes an idle connection:**

  | SDK, revision | Ends on | Close | Timers | Pending requests at the end |
  | :--- | :--- | :--- | :--- | :--- |
  | TypeScript 1.9.0 (`60619a4`) | stream end, `close(error?)` | idempotent; `closed` promise, `signal` | none on stdio; 60 s output stall on HTTP only | rejected, "ACP connection closed"; incoming handlers aborted |
  | Rust 3.4.0 (`652d100`) | end of file, end of the `connect_with` scope | no method; dropping kills the child's process group | 1 s grace before that kill | internal error, `{"reason":"incoming_transport_closed"}` |
  | Python 0.12.1 (`11f8a4c`) | end of file, `close()` | idempotent; async context manager | `receive_timeout`, off by default; spawn helper 2 s per escalation step | `ConnectionError("Connection closed")` |
  | Kotlin, commit `dc1a8f4` (latest tag `v0.33.0`) | `close()`, parent scope cancelled | transport close "safe to call repeatedly" | `requestTimeout` 60 s, deprecated and unused | `CancellationException("Protocol closed")` |
  | `coder/acp-go-sdk` 0.13.5 (`0845a3b`) | end of input only | none; `Done()` only | 5 s notification drain | internal error, "peer disconnected before response" |

* **MCP closes through the transport too.**
  * 2025-11-25 lifecycle: "No specific shutdown messages are defined—instead, the underlying transport mechanism should be used". For
    stdio, the client closes the server's input, waits, then sends `SIGTERM`, then `SIGKILL`.
  * 2026-07-28: "Servers SHOULD exit promptly when their standard input is closed".
  * Its timeouts are per request ("Implementations SHOULD establish timeouts for all sent requests"), not per connection.
  * The official Go MCP SDK's `Close` is "idempotent and concurrency safe" and waits for ongoing requests. Its keepalive and idle
    session timeout are opt-in, zero meaning off.
* **LSP** alone has an in-band `shutdown` request and `exit` notification. It has no timeout.
* **Go JSON-RPC libraries**, `golang.org/x/tools`' `jsonrpc2` and `jsonrpc2_v2` and `sourcegraph/jsonrpc2`, close no idle
  connection. The `x/tools` idle timeouts stop a server or listener that has no connections.
* **The Go standard library makes a timeout opt-in.**
  * `net/http.Server.IdleTimeout`: "If negative, or if zero and ReadTimeout is zero or negative, there is no timeout".
  * `net.Conn.SetDeadline` is the caller's to set.
  * `os/exec.Cmd.WaitDelay` and `Cancel` let the application bound and escalate a child's exit.
  * `io.Closer`: "The behavior of Close after the first call is undefined. Specific implementations may document their own
    behavior."
* **The five local codebases** (paths under `~/gitrepos/`) confirm it. None closes an idle connection on a timer, and none has a
  JSON-RPC heartbeat. Each connection ends on end of input or an explicit close. Every timer found is a per-request deadline, a
  bounded drain at shutdown, or a TERM-then-KILL escalation of a child process:
  * `codex`: 30 s RPC drain and a 45 s watchdog after stdin closes, `app-server/src/message_processor.rs:110`,
    `app-server-transport/src/transport/stdio.rs:175-178`;
  * `opencode`: 5 s cancel drain, `packages/opencode/src/acp/turn.ts:40-42`;
  * `pi`: 30 s MCP request timeout, `packages/mcp/src/client.ts:38`.

  Their tests tie cleanup to a scope: Rust `Drop` and `AbortOnDropHandle` (`grok-build`, `codex`), TypeScript `Symbol.asyncDispose`
  (`opencode`), Effect `acquireRelease` (`kilocode`), `afterEach` closing the client (`pi`).
* **Leak checks in Go tests:**
  * `go.uber.org/goleak` is an external module. `AGENTS.md`, "Dependencies", requires a MADR naming any dependency.
  * `testing/synctest` requires every goroutine of its bubble to exit, but blocking on real I/O does not count as durably blocked
    there.
  * The Go blog ("Goroutine Leak Profiles", 2026-09-02) encourages "using goroutine leak profiles at various layers, up to, and
    including production, as well as comprehensive test suites". The profile's stated limit: it "may fail to identify leaks caused by
    blocking on concurrency primitives reachable through global variables or the local variables of runnable goroutines".

## Decision Drivers

* **ACP sessions sit idle by design.** A user reads, thinks, or leaves an editor open. An SDK timer that closes an idle connection
  would end sessions an application means to keep, and ACP v1 has no ping to tell idle from dead.
* **The protocol, every official SDK, MCP and the Go JSON-RPC libraries** leave lifetime to the transport and the application, and
  make any timer opt-in.
* **`Close` should end what it starts.** An application that calls `Close` should not find handler goroutines still running, or
  contexts still live, five seconds later, or never.
* **A guard in the tests.** A leak should fail the suite where it is introduced, not be found in a goroutine dump during a 1,000-run
  race pass.
* **No new dependency** without a record of its own (`AGENTS.md`).
* **No change to the exported API or to the wire.** Only `Close`'s effect on running notification handlers changes, and the release
  notes name it.

## Considered Options

For the SDK:

* **S1.** No SDK timer. The application owns the lifetime. The SDK ends a connection at end of input, on `Close`, or by the overflow
  policy, and documents the application's part.
* **S2.** S1, plus an opt-in `WithIdleTimeout(d)` that closes a connection that has received nothing for `d`.
* **S3.** A default idle timeout.
* **S4.** An SDK keepalive: send a ping, and close when it is not answered.

For `Close`:

* **C1.** `Close` also cancels the notification handlers' context at once, as it already does the request handlers'.
* **C2.** `Close` stays as it is, and its documentation states the remaining reach of the notification handlers.

For the tests:

* **T1.** Close each of the 10 leaking tests' connections, and nothing else.
* **T2.** T1, and a `TestMain` guard: after the tests, it polls the standard `goroutineleak` profile until it reports none, for at
  most the drain bound and a margin. Otherwise it fails the run and prints the leaked stacks.
* **T3.** T1, and `go.uber.org/goleak`'s `VerifyTestMain`.
* **T4.** T1, and every test that builds a connection runs in a `testing/synctest` bubble, whose end requires its goroutines to exit.
* **T5.** T1, and a `TestMain` guard that scans `runtime.Stack` for the connection's own functions.

## Decision Outcome

Chosen options, decided by the owner on 2026-10-19 (see "Decided with the owner, 2026-10-19"): **"S1", "C1" and "T2"**:

* **S1:** ACP, every official SDK and every comparable project leave a connection's lifetime to the application. An idle ACP
  connection is normal, and the protocol has no liveness check an SDK timer could rely on.
* **C1:** without it, `Close` leaves notification handlers running for 5 s with a reader it can close, and indefinitely with one it
  cannot, while request handlers end at once. That is the one gap the probe found in the SDK itself.
* **T2:** it guards the whole suite with the standard library alone, and it was seen to count exactly the 80 leaked goroutines.

In detail:

* **S1, the SDK:**
  * No idle, request or keepalive timer.
  * A request's deadline stays its caller's context.
  * Ending a session stays `session/close`.
  * Ending an agent process stays the application's: close its input, then escalate after a bound, as `stopAgent` in the example
    client does and as MCP's stdio shutdown sets out.
  * The README's "Closing and limits" gains this guidance. It already states who closes the transport.
* **C1, `Close`:**
  * `closeWith` cancels `inboundCtx` with its cause as well as the connection's context. A notification handler then sees its
    context end at once.
  * At end of input nothing changes. The notification drain (`shutdownReceive`) still lets the notifications already received run,
    for at most 5 s.
  * `Close`'s doc comment and the README state the new reach: every handler's context ends.
  * `Close` still cannot end a read on a reader that is not an `io.Closer`. Its doc says so already. With C1, the only goroutines left
    in that case are `receive` and `processNotifications`, and no handler holds a live context.
* **T2, the tests:**
  * The 10 tests close their connections through `t.Cleanup` calling `Close` on both sides.
  * The guard is a root `TestMain`. After `m.Run`, it polls `pprof.Lookup("goroutineleak")` until the count is 0, for at most the
    drain bound and a margin. It then fails the run and prints the profile's stacks if any leaked goroutine is left.

    Amended after the close of [0023-PLAN-connection-lifetime-and-test-leaks.md](0023-PLAN-connection-lifetime-and-test-leaks.md),
    2026-10-19: stopping at the first zero read misses a leak made by the last test to run. Its goroutines may not have blocked yet.
    A planted leak in a test run alone was missed 18 times in 30. The guard reports clean only after the count has stayed 0 for a
    settle window of 500 ms. In 400 measured runs the leak showed within 10.951 ms. The PLAN's deviation of that date holds the
    measurements.
  * The guard runs on every `go test` of the root package, CI's included.

### Decided with the owner, 2026-10-19

The owner answered the record's three questions, each as recommended:

| Question | Answer |
| :--- | :--- |
| The SDK's lifetime policy | S1: no SDK timer. |
| `Close` and notification handlers | C1: `Close` also cancels the notification handlers' context. |
| The tests' guard | T2: the standard library's `goroutineleak` profile, in a root `TestMain`. |

Checked after the decision, in a scratch copy of `d004e9a`: under `-race`, the profile again counted the suite's 80 leaked
goroutines.

### Consequences

* Good, because applications keep connections exactly as long as they choose, as with every other ACP SDK.
* Good, because `Close` ends every handler's context at once, as its name and the README imply.
* Good, because a leaked connection fails the run that introduces it, with its stacks.
* Good, because the 1,000-run race passes no longer accumulate connections, which slowed step 7's pass.

  Amended while executing [0023-PLAN-connection-lifetime-and-test-leaks.md](0023-PLAN-connection-lifetime-and-test-leaks.md), step
  2, 2026-10-19: the slowing is not borne out.
  * Step 7's selection, run 100 times under `-race`, took 131 s with the leaks gone. Its record measured 130 s on `6d60d84` with them.
  * The accumulation shows in the goroutine dump, not in the time. Its 10-minute timeout came from the pass's length: 1,252 s at
    1,000 runs.
* Good, because no dependency is added.
* Neutral, because applications that want a dead-peer check still build it themselves. ACP v2 may standardize one, and S2 or S4 can be
  revisited then.
* Bad, because C1 changes behaviour. A notification handler that kept working after `Close`, for at most 5 s, now sees its context
  ended. The release notes must name it.
* Bad, because the guard is package-wide. It names the leaked stacks, not the test that leaked them. A failing run is narrowed by
  running tests alone, as this record's measurement did.
* Bad, because the profile can miss a goroutine blocked on something reachable from a global or a running goroutine. T2 accepts that
  limit for a guard over connections that tests build and drop.

### Confirmation

The PLAN confirms the decision with these checks, each seen to fail on a planted break in a scratch copy:

* **The guard:** a test that leaves one connection open fails the run, with the connection's goroutines in the printed stacks.
* **The 10 tests:** run alone with the guard, each passes. Before their `t.Cleanup`, each fails it.
* **C1:**
  * a test requires a notification handler blocked on its context to see it end within a short bound after `Close`, with a reader
    that is an `io.Closer` and with one that is not;
  * it fails with `closeWith` as it is now;
  * the end-of-input tests (`end_of_input_test.go`) still pass, so the drain is unchanged.
* **The suite:** under `-race`, and with `-count` above 1, the guard reports nothing.
* **The API listing** against the latest tag is unchanged.

## Pros and Cons of the Options

### S1. No SDK timer; the application owns the lifetime

* Good, because it matches the ACP specification, the TypeScript, Rust, Python and Kotlin SDKs, MCP, the Go JSON-RPC libraries and
  the five local codebases.
* Good, because an idle session is never ended against the application's wish.
* Neutral, because it is today's behaviour, so it needs documentation, not code.
* Bad, because a peer that hangs without closing its output is the application's to detect, by a request deadline or a process
  watch.

### S2. An opt-in idle timeout

* Good, because it follows `net/http`'s and the MCP Go SDK's opt-in pattern, zero meaning off.
* Neutral, because no ACP SDK offers one, and no issue or downstream consumer has asked for one.
* Bad, because with no ACP ping, received silence cannot tell a dead peer from a user who is thinking. A timeout long enough not to
  end live sessions is too long to detect a dead peer usefully.
* Bad, because it adds exported API that an application can already build with a timer and `Close`.

### S3. A default idle timeout

* Bad, because it would end idle sessions that applications mean to keep. No source surveyed does this by default.
* Bad, because it changes today's behaviour for every application.

### S4. An SDK keepalive

* Good, because a ping answered distinguishes a live peer from a dead one, as the MCP Go SDK's opt-in `KeepAlive` does.
* Bad, because ACP v1 defines no ping method. A ping in an extension method requires the peer to implement it. The streamable HTTP and
  WebSocket RFD defers a standard keepalive to v2.

### C1. `Close` cancels the notification handlers' context too

* Good, because request and notification handlers then end alike on `Close`.
* Good, because a reader that is not an `io.Closer` no longer leaves a handler's context live after `Close`.
* Neutral, because the drain at end of input is untouched.
* Bad, because it changes behaviour: up to 5 s of notification handling after `Close` goes away.

### C2. `Close` unchanged, documented

* Good, because nothing changes for applications.
* Bad, because "`Close` ends the connection" would need the exception that a notification handler may run on, for 5 s or
  indefinitely depending on the reader. An application cannot shorten that.

### T1. Close the 10 tests' connections only

* Good, because it is the smallest change.
* Bad, because the next test that forgets is found as this one was: by a goroutine dump in a long race pass.

### T2. The 10 tests, and a `goroutineleak` guard in `TestMain`

* Good, because it uses the standard library only, as Go 1.27 made the profile generally available.
* Good, because it covers every root test, including future ones, on every run and in CI.
* Good, because it was seen to count exactly the 80 leaked goroutines of the suite.
* Neutral, because it must wait out the drain bound when a test ends a connection by ending its input. That costs at most about 6 s
  per package run, and only when something is still draining.
* Bad, because it is package-wide, and blind to leaks reachable from globals or running goroutines.

### T3. The 10 tests, and `goleak`

* Good, because `goleak` is widely used, and its `VerifyNone(t)` can attribute a leak to one test.
* Bad, because it is a new dependency, which needs a record of its own.
* Bad, because per test it is incompatible with `t.Parallel`, which 146 of the 206 root tests call.

### T4. Every connection test in a `synctest` bubble

* Good, because the bubble requires its goroutines to exit, per test.
* Bad, because the root tests build connections at 143 sites, and each would move into a bubble.
* Bad, because a bubble's clock is fake, so every test's timing would have to be checked again.

### T5. The 10 tests, and a stack-scan guard

* Good, because it sees a goroutine whatever references it.
* Bad, because it names the SDK's internal functions, so a refactor that renames one silently weakens the guard. The profile is
  generic.

## More Information

* **Not decided here, deliberately:**
  * **`Err()`, an accessor for why the connection ended.** The TypeScript SDK's `signal.reason` and `x/tools`' `jsonrpc2.Conn.Err`
    have one. Today a failed call carries the cause. It would add exported API, and no consumer has asked for it.
  * **`Wait()`, which returns once the connection's goroutines have exited,** as the MCP Go SDK's `Wait` does. `Close` cannot
    promise that for a reader it cannot close.
  * **An SDK helper that starts an agent process and escalates its shutdown,** as the Rust and Python SDKs have. The example client's
    `stopAgent` shows the pattern with `os/exec`.

  Each would be a record of its own.
* **The PLAN** is [0023-PLAN-connection-lifetime-and-test-leaks.md](0023-PLAN-connection-lifetime-and-test-leaks.md).
* **The release:** C1 changes behaviour, documented, with no exported API change. The owner decides the version at the tag.
* **The measurements' scripts** stay outside the tree, as the repository's records do for their scratch tools. The figures above are
  their output.

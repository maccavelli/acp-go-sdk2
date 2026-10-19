# Release notes for v1.16.1, against v1.16.0

Observed 2026-10-19 on `4a769a5`, for [0023-PLAN-connection-lifetime-and-test-leaks.md](../decisions/0023-PLAN-connection-lifetime-and-test-leaks.md),
step 4. This report drafts the text of the GitHub release for `v1.16.1`, as `RELEASING.md` describes it. It decides nothing.

Each statement names its source:

- a record;
- a commit;
- or the exported API listing of the 0013 report's listing program, run on `git archive v1.16.0` (2429 lines) and on the tree at
  `4a769a5` (2429 lines).

Bring it up to date if more changes land before the tag.

- No exported API changes, so the release is proposed as the patch `v1.16.1`
  ([0023-PLAN-connection-lifetime-and-test-leaks.md](../decisions/0023-PLAN-connection-lifetime-and-test-leaks.md), "Goal"). The owner
  confirms it at the tag.
- One behaviour changes, and the release text names it.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.16.1

The ACP schema is still `1.25.0` (`git diff v1.16.0 -- schema/` is empty), and the exported API is unchanged. This release implements
[0023-MADR-connection-lifetime-and-test-leaks.md](../decisions/0023-MADR-connection-lifetime-and-test-leaks.md): `Close` now ends
every handler, the README states what an application owns of a connection's lifetime, and the tests fail on a leaked connection.

#### Behaviour changes

- **`Close` ends the notification handlers' contexts too.** (`4a769a5`; C1.)
  - `Close`, on a connection or on either side connection, already ended a running request handler's context at once.
  - A running notification handler's context ended only once the reader stopped and the input's drain finished:
    - 5 seconds later, with a reader `Close` could close;
    - not at all, until the input ended by itself, with a reader that is not an `io.Closer`.
  - Now both end at once, with the cause `acp.ErrConnectionClosed`.
  - The default overflow policy, `acp.OverflowCloseConnection`, closes the connection through the same code. So it now ends the
    notification handlers' contexts at once too, with its own cause, the notification queue's overflow.
  - When the input ends without `Close`, nothing changes: the notifications already received still run under live contexts, for at
    most five seconds.
  - An application whose notification handlers kept working after `Close` now sees their contexts end.

#### Documentation

- **The README's "Closing and limits"** states that `Close` ends every handler's context. A new paragraph states what the application
  owns. (`4a769a5`; S1.)
  - The SDK closes no connection on a timer: an idle connection is normal in ACP, and the protocol has no ping to tell an idle peer
    from a dead one.
  - A request's deadline is its context, and a session ends with `session/close`.
  - An agent process is the client's to end: close its input, wait for it to exit, and kill it after a bound, as the example client's
    `stopAgent` does.

#### For contributors

- **The root tests fail on a leaked goroutine.** (`bb9197b`; T2.)
  - `leak_test.go`'s `TestMain` reads the standard library's `goroutineleak` profile after the tests, which Go 1.27 made generally
    available.
  - If any goroutine is left leaked, it fails the run and prints the leaked stacks.
- **The 10 tests that left connections open now close them,** in `t.Cleanup`. (`bb9197b`.)

## Sources not in the release text

- **Commits since `v1.16.0` that change Go files outside `cmd/generate`**
  (`git log v1.16.0..HEAD -- '*.go' ':!cmd/generate'`): `bb9197b` and `4a769a5`.
  - The SDK's own non-test file that changes is `connection.go`: 9 lines added and 5 removed, in `closeWith` and the doc comments of
    `Close` and `closeWith`.
  - The tests that change are `acp_test.go`, `connection_close_test.go` and the new `leak_test.go`.
  - The other four commits, `d004e9a`, `88a08ec`, `6f49e13` and `f1c65e9`, change records only (`git show --name-only`).
- **The API listing:** 0 lines removed, 0 added, `v1.16.0` against `4a769a5`.
- **Execution records:** each change, its planted breaks, its 1,000-run `-race` passes and a Windows run are in
  [0023-PLAN-connection-lifetime-and-test-leaks.md](../decisions/0023-PLAN-connection-lifetime-and-test-leaks.md). The research behind
  the decision is in the MADR: the ACP specification, the official ACP SDKs, MCP, LSP, Go JSON-RPC libraries and five agent codebases.
- **Not in this release:** an SDK idle timeout or keepalive, an `Err()` accessor, a `Wait()`, and an agent-process helper, which the
  MADR leaves to records of their own; and schema `1.26.0`.

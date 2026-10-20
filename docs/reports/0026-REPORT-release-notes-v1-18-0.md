# Release notes for v1.18.0, against v1.17.1

Observed 2026-10-20 on `93ad59a` and the working tree of
[0026-PLAN-connection-end-cause-and-wait.md](../decisions/0026-PLAN-connection-end-cause-and-wait.md), step 6. This report drafts the
text of the GitHub release for `v1.18.0`, as `RELEASING.md` describes it. It decides nothing.

Each statement names its source:

- a record;
- a commit;
- `make api`, run on `93ad59a` with nothing uncommitted, and again on step 6's working tree;
- or a probe: a scratch program run against `git archive` trees of `v1.17.1` and `93ad59a`.

Bring it up to date if more changes land before the tag.

- Seven additions to the exported API and none removed or changed. `make api` against `v1.17.1`: "0 incompatible, 7 compatible".
  The release is proposed as the minor `v1.18.0`, as `RELEASING.md` says for compatible changes. The owner confirms it at the tag.
- What a consumer may meet: a notification's error after the peer's end now also matches `errors.Is(err, io.EOF)`. Its text is
  unchanged, and so is a request's error.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.18.0

The ACP schema is still `1.25.0` (`git diff v1.17.1 -- schema/` is empty). This release implements
[0026-MADR-connection-end-cause-and-wait.md](../decisions/0026-MADR-connection-end-cause-and-wait.md): a connection says why it ended,
and can be waited on until its goroutines have exited. `make api` reports seven compatible additions and no incompatible change.

#### New API

- **`Err() error`** on `Connection`, `AgentSideConnection` and `ClientSideConnection`. (`48d6ab8`, `bacac56`.)
  - It is nil until `Done()` closes, and then why the connection ended, the same error each time.
  - `ErrConnectionClosed` when this side ended it, by `Close` or by the overflow policy `OverflowCloseConnection`, which also wraps
    `ErrNotificationQueueOverflow`.
  - Otherwise `ErrPeerDisconnected`, wrapping `io.EOF` when the peer ended its output, or the read's error.
  - The first end is kept: a `Close` after the peer's end leaves `Err()` as it was.
- **`Wait() error`** on the same three. (`d78220e`, `bacac56`.)
  - It blocks until the connection has ended and every goroutine it started has returned, its handlers' included, and then returns
    `Err()`.
  - It does not end the connection: call `Close` first to stop it.
  - A reader that does not implement `io.Closer` keeps it waiting until its `Read` returns, and so does a handler that ignores its
    context.
- **`ErrNotificationQueueOverflow`,** exported, its text "notification queue overflow" unchanged. After an overflow close, `Err()`
  and the errors of later requests and notifications wrap it, beside `ErrConnectionClosed`. (`48d6ab8`.)

```go
select {
case <-ctx.Done():
case <-asc.Done():
}
_ = asc.Close() // Err keeps an earlier end
switch err := asc.Wait(); {
case errors.Is(err, acp.ErrNotificationQueueOverflow):
    log.Print("the client sent notifications faster than they were handled")
case errors.Is(err, io.EOF):
    log.Print("the client closed its output")
}
```

#### Behaviour changes

- **After the peer's end of output, a notification's error also matches `errors.Is(err, io.EOF)`,** after a later `Close` too. Its
  text is unchanged. (`48d6ab8`.)
  - A request's error is unchanged: in flight or sent later, a late `Close` or not, it fails `-32603`, "input ended before a
    response", wrapping `ErrPeerDisconnected` and not `io.EOF`.
- Nothing else a connection does changes. Its goroutines now start through a wait group, which `Wait` waits on. (`d78220e`.)

#### Documentation

- The README's "Closing and limits" describes `Err()` and `Wait()`, with the example above, and `docs/architecture.md` the recorded end
  and the wait group. (`93ad59a`, and step 6's commit.)

#### For maintainers

- **The API gate is now `apidiff`,** as go-llmprovider-sdk and go-selfupdate-lib gate theirs. (`609cb41`.)
  - `scripts/check_api.py` runs `golang.org/x/exp/cmd/apidiff@v0.0.0-20261014143641-f0251eb4aa3e` on the root package of the working
    tree and of the newest `v1` tag, taken with `git archive`. It reads the report strictly: a line it does not recognise is an error,
    never a pass.
  - `make api`, with an optional `BASE=`, runs it; `TAG=` is gone. CI's step "API compatibility (apidiff)" runs it on Linux with Go
    1.27.2, whose checkout fetches the tags.
  - It replaces `gorelease`, whose `-version` asked the module mirror for the proposed version before the tag existed. The mirror
    cached the miss and refused `v1.17.1` for about 30 minutes after it was pushed. `apidiff` asks the mirror nothing about this module.
- **New tests:**
  - at the root, `TestErr`, `TestWait`, `TestSideConnectionsErrAndWait`, and `TestGoStatementsCounted`, which fails on a `go`
    statement in the package's non-test files, so a new goroutine cannot bypass the wait group;
  - in the generator, pins of the side connections' `Err` and `Wait`, and the four new lines in each of the 73 accept engine cases'
    `want.api`.

  (`48d6ab8`, `d78220e`, `bacac56`, and step 6's commit.)

## Sources not in the release text

- **Commits since `v1.17.1`** (`git log v1.17.1..93ad59a`), eleven, and step 6's:
  - `609cb41` changes `ci.yml`, the `Makefile`, `RELEASING.md` and `docs/architecture.md`, and adds `scripts/check_api.py`;
  - `48d6ab8` changes `connection.go` and `errors.go`, adds `connection_end_test.go` and renames the sentinel in three tests;
  - `d78220e` changes `connection.go` and `connection_end_test.go`;
  - `bacac56` changes the generator's emitter, its stub and pins, `agent_gen.go`, `client_gen.go`, the 73 accept engine cases and
    `connection_end_test.go`;
  - `93ad59a` changes `README.md` and `docs/architecture.md`;
  - `1dfce5e`, `9e3f23b`, `2965021`, `0afe75e`, `3be2b1f` and `26b7f27` change records only (`git show --name-only`);
  - step 6's commit corrects `Err`'s doc and a README sentence, adds two request checks to `TestErr`, amends the MADR, and adds this
    report.
- **Go files changed.** `git diff --name-only v1.17.1 93ad59a -- '*.go' ':!*_test.go'` lists five: `agent_gen.go`, `client_gen.go`,
  `connection.go`, `errors.go`, and the generator's `cmd/generate/internal/emit/dispatch.go`.
- **`make api` on `93ad59a`:** exit 0, "against v1.17.1: 0 incompatible, 7 compatible", the seven `(*AgentSideConnection).Err`,
  `(*AgentSideConnection).Wait`, `(*ClientSideConnection).Err`, `(*ClientSideConnection).Wait`, `(*Connection).Err`,
  `(*Connection).Wait` and `ErrNotificationQueueOverflow`, each "added".
- **The probes,** against `v1.17.1` and `93ad59a`, after the peer's end of output:
  - a request, in flight, sent later, or sent after a late `Close`: in both, "input ended before a response", wrapping
    `ErrPeerDisconnected`, not `io.EOF` and not `ErrConnectionClosed`;
  - a later notification: in both, "acp: peer disconnected: peer connection closed"; it wraps `io.EOF` only in `93ad59a`;
  - a notification after a late `Close`: in both, "acp: connection closed: peer connection closed"; it wraps `io.EOF` only in
    `93ad59a`;
  - after an overflow close, on `93ad59a`: a request's and a notification's error both wrap `ErrConnectionClosed` and
    `ErrNotificationQueueOverflow`.
- **The evidence:** [0026-PLAN-connection-end-cause-and-wait.md](../decisions/0026-PLAN-connection-end-cause-and-wait.md). Its steps 2
  to 6 hold the planted breaks and the gates on go1.27.2 and go1.26.9. Its deviations during steps 3 and 6 are in the same record, and
  the MADR's amendment during step 6 corrects what it said a call after the end returns.
- **The gate's switch:** [0024-PLAN-api-gate-and-deprecation.md](../decisions/0024-PLAN-api-gate-and-deprecation.md), its deviation
  after the close and its step 8, and
  [0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](../decisions/0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md),
  question 14.
- **Not in this release:** schema `1.26.0`
  ([0025-REPORT-schema-1-26-0-adoption-assessment.md](0025-REPORT-schema-1-26-0-adoption-assessment.md): not now), and the agent-process
  helper [0023-MADR-connection-lifetime-and-test-leaks.md](../decisions/0023-MADR-connection-lifetime-and-test-leaks.md) left to a
  record of its own.

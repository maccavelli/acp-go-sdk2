# Release notes for v1.28.0, against v1.27.0

Observed 2026-10-30 on `29d1097`, for
[0042-PLAN-in-process-agent-runner.md](../decisions/0042-PLAN-in-process-agent-runner.md), phase 17. This report drafts the text of
the GitHub release for `v1.28.0`, as `RELEASING.md` describes it. It decides nothing. It is committed before the tag, so the tagged
tree holds it. The commit that adds it changes `tools/consumer.py`, `tools/README.md`, records and nothing else, so the API it tags
is `29d1097`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `29d1097` with only `tools/consumer.py`, `tools/README.md`, `docs/README.md` and the records uncommitted,
  which hold no Go package.

Summary:

- Three additions to the exported API: one in the root package and two in `agentproc`. `make api` against `v1.27.0`: "0
  incompatible, 3 compatible". The release is the minor `v1.28.0`, as `RELEASING.md` says for compatible changes.
- One defect fixed: `Process.Wait`'s error said its cause twice when an in-process agent's own error already said it.
- What a consumer may meet:
  - nothing the SDK sends or accepts on the wire changes;
  - `Wait`'s error text changes for an agent whose error already wraps `ErrTerminated` or `ErrKilled`; `errors.Is` and `errors.As`
    give what they gave;
  - `ServeAgent`'s error at its context's end gains the new sentinel's text when its 5 s bound passes with requests unanswered;
  - `Close`, `Kill`, `Wait` and `WaitContext` otherwise behave as before, a hung handler holding them as it did.
- The `go` lines and the requirements are unchanged, and no generated file changed.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.28.0

The ACP schema is still `1.25.0` (`git diff v1.27.0 -- schema/` is empty). This release answers
[issue #12](https://github.com/maccavelli/acp-go-sdk2/issues/12): `agentproc`'s `Process.Close` waits without bound for a client
handler that ignores its context, so a client could not bound its close. It adds a bounded close beside `Close` and `Kill`, a
sentinel `errors.Is` can test for handlers that still run, and `Wait`'s error said once
([0042-MADR-in-process-agent-runner.md](../decisions/0042-MADR-in-process-agent-runner.md), the amendment of 2026-10-30).

`make api` reports three compatible additions and no incompatible change. Nothing sent or accepted on the wire changes.

#### New for clients: a bounded close in `agentproc`

`Close`, `Kill` and `Wait` wait for the connection's handlers, the client's own methods, without bound, as
`Connection.Wait` does: a handler that ignores its context holds them until it returns. That is unchanged, so a `Wait` that
returns still means every handler has returned.

- **`Process.CloseContext(ctx)`** is `Close` bounded by `ctx`.
  - It runs `Close`'s steps and returns `Wait`'s error once `Wait` would return.
  - Should `ctx` end during the steps, the steps left run as `Kill`'s do: no more waiting, no SIGTERM, the kill.
  - Should it end before `Wait` would return, it returns `context.Cause(ctx)`, joined with `acp.ErrHandlersRunning` when the
    connection's handlers are what still runs, and with the agent's error, as `Wait` gives it.
  - The agent has ended by then, and `Done` is closed. What still runs is left to finish, and `Wait`, or `WaitContext`, returns
    the whole error once it has.
  - It returns at most a grace, the kill's own bound, after `ctx` ends.
  - (`82f421b`.)
- **`Process.KillContext(ctx)`** is `Kill` bounded by `ctx` in the same way. (`82f421b`.)
- `WaitContext`, `CloseContext` and `KillContext` share one goroutine, which ends when `Wait` returns, so calls start none of
  their own. A goroutine a hung handler holds, and that one, run until the handler returns, since Go cannot stop a goroutine.

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := p.CloseContext(ctx); errors.Is(err, acp.ErrHandlersRunning) {
    log.Printf("a client handler still runs: %v", err)
}
```

#### New for agents and both sides

- **`acp.ErrHandlersRunning`** says that a wait for a connection's handlers ended before they had returned. (`1b61f33`,
  `82f421b`.)
  - `ServeAgent` wraps it when its 5 s bound passes with handlers still running at its input's end. The error's text is
    unchanged: `acp: the connection's handlers still run 5s after its end`.
  - It wraps it too at its context's end, when `Shutdown`'s 5 s pass with requests unanswered, beside the context's cause and
    `Shutdown`'s `context.DeadlineExceeded`.
  - `CloseContext` and `KillContext` join it, above.

#### Fixed

- **`Process.Wait`'s error said its cause twice** for an in-process agent whose `Serve` returns its context's cause, as
  `acp.ServeAgent` does: `agentproc: agent killed: agentproc: agent killed`. `Wait` now returns an agent's error that already
  wraps the step's sentinel as it is, `agentproc: agent killed`, and joins the sentinel otherwise, as before. A child's error
  and an abandoned `Serve`'s are unchanged. (`1457073`.)

#### For maintainers

- **Docs.** The README and `docs/architecture.md` no longer say "every wait bounded" of `agentproc`: every wait for the agent is
  bounded, and the wait for the client's handlers is bounded by `CloseContext` and `KillContext`. The 0027 MADR's and the 0042
  MADR's same claim is corrected in place. (`9a3ff81`, `6b78036`.)
- **Tests.**
  - In `agentproc`: `closecontext_test.go`, seven tests, in process and for a child; `TestWaitErrorSaysTheCauseOnce`; and
    `ExampleProcess_CloseContext`.
  - At the root: `TestServeAgentHandlersStillRun`, each case in a `synctest` bubble, so the 5 s bounds pass on the bubble's clock.
  - Three tests that failed under load in CI were fixed: `TestStderrHeldByDescendant` and the tests `tappedFrames` serves have
    graces of their own, and `TestHeldNotificationsFillQueue` allows the overflow to close the pipe before its write. (`70cf30c`,
    `f771373`.)
- **Tools.** `tools/consumer.py` builds a module at the Go floor that also closes an in-process agent through `CloseContext`,
  and closes another under a 300 ms bound with a client handler that ignores its context, testing its error with `errors.Is`
  for the bound's cause and `acp.ErrHandlersRunning`.

## Sources not in the release text

- **Commits since `v1.27.0`** (`git log v1.27.0..29d1097`), ten:
  - `18d3609` marks the 0043 PLAN complete;
  - `70cf30c` changes `agentproc/agentproc_test.go` and the 0027 and 0043 PLANs;
  - `f771373` changes `agentproc/tap_test.go`, `initialize_test.go`, `jsonrpc_test.go`, and the 0027 and 0030 PLANs;
  - `03b57e9` marks the 0027 and 0030 PLANs complete;
  - `9a3ff81` amends the 0042 MADR and PLAN, and corrects the 0027 MADR;
  - `1457073` (`agentproc/agentproc.go`, `agentproc/inprocess_test.go`), `1b61f33` (`errors.go`, `serve.go`,
    `shutdown_test.go`) and `82f421b` (`agentproc/agentproc.go`, `errors.go`, `agentproc/closecontext_test.go`,
    `agentproc/example_test.go`, `agentproc/waitcontext_test.go`) change the packages, as above;
  - `6b78036` changes `README.md` and `docs/architecture.md`;
  - `29d1097` records phase 17's checks in the 0042 PLAN;
  - each changes `docs/README.md`, a MADR or a PLAN as well (`git show --name-only`).
- **Go files changed.** `git diff --name-status -M v1.27.0 29d1097 -- '*.go' ':!*_test.go'` lists 3 files, each changed:
  `agentproc/agentproc.go`, `errors.go` and `serve.go`. The tests: `agentproc/closecontext_test.go` added;
  `agentproc/agentproc_test.go`, `agentproc/example_test.go`, `agentproc/inprocess_test.go`, `agentproc/tap_test.go`,
  `agentproc/waitcontext_test.go`, `initialize_test.go`, `jsonrpc_test.go` and `shutdown_test.go` changed.
- **Unchanged** (`git diff --stat v1.27.0 29d1097`, empty):
  - `go.mod`, `go.sum`, `cmd/generate/go.mod` and `cmd/generate/go.sum`;
  - the schema, the workflows, the `Makefile`, `RELEASING.md` and `scripts/`;
  - `acptest/`, `clienthost/`, `remote/` and `internal/`;
  - every `*_gen.go` file.
- **`make api` on `29d1097`:** exit 0, "against v1.27.0: 0 incompatible, 3 compatible", each "added": `ErrHandlersRunning`,
  `agentproc: (*Process).CloseContext` and `agentproc: (*Process).KillContext`; and "reported packages unstable, remote: 0
  incompatible, not gated".
- **CI on `29d1097`**, pushed with the commits from `1457073`:
  - run `41142221134`, "completed success", six jobs on Ubuntu, Windows and macOS at Go `1.27.3` and `1.26.10`;
  - the Docs workflow's run `41142220171`, "completed success".
- **The consumer before the tag.** A scratch script built `tools/consumer.py`'s program and test against `29d1097`'s tree through a
  `replace`, at `go1.26.10`. The output equals `WANT_RUN`, its new lines `close-context <nil>` and `bounded true true <nil>`; the
  test and the Windows build pass. In two scratch copies of the tree, the sentinel never joined and the bound's cause dropped, the
  line read `bounded true false <nil>` and `bounded false true <nil>`, and the output differed from `WANT_RUN`, as required.
  `consumer.py v1.28.0` runs after the tag.
- **The evidence:**
  - the 0042 MADR's amendment, findings I1 to I9: the issue reproduced in process and for a child, and the repeated text on the
    unmodified tree;
  - the 0042 PLAN's phases 13 to 15: ten planted breaks, each failing as required;
  - the new tests passing 20 times on macOS under `-race` and on the Linux and Windows hosts, in phase 15;
  - the gates on macOS and on the Linux and Windows hosts, 18 of 18, in phases 13, 14, 15 and 17;
  - the deviation entries, each recorded in the PLAN.

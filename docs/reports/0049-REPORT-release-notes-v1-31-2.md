# Release notes for v1.31.2, against v1.31.1

Observed 2026-11-03 on `427c40d`, for [0049-PLAN-runtime-fixes.md](../decisions/0049-PLAN-runtime-fixes.md), phase 4, step 3.
This report drafts the text of the GitHub release for `v1.31.2`, as `RELEASING.md` describes it. It decides nothing. It is
committed before the tag, so the tagged tree holds it. The commit that adds it changes records and nothing else, so the API it
tags is `427c40d`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `427c40d` with nothing uncommitted.

Summary:

- No exported API change. `make api` against `v1.31.1`: "0 incompatible, 0 compatible", so the release is a patch, as
  `RELEASING.md` says and the runtime PLAN planned.
- Ten fixes to the connection runtime, R1 to R10 of
  [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](../decisions/0049-MADR-remediate-the-2026-11-01-debugging-pass.md). Nine
  change behaviour, and R10 is documentation.
- What a consumer may meet:
  - a peer that leaves more than 1,024 error answers unread now closes the connection;
  - the observer reports faults and refusals off the reader;
  - an agent's receiving middleware may no longer change `initialize`'s answer;
  - `ServeAgent` waits for notification handlers at its context's end;
  - `WithMaxQueuedNotifications` is capped at 1,048,576.
- The `go` lines, the requirements and the schema are unchanged. One generated line was added, in `agent_gen.go`.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.31.2

The ACP schema is still `1.25.0` (`git diff v1.31.1 -- schema/` is empty). The exported API is unchanged: `make api` reports no
change against `v1.31.1`. This release fixes the ten defects of the connection runtime that the 2026-11-01 debugging pass found
([0049-MADR-remediate-the-2026-11-01-debugging-pass.md](../decisions/0049-MADR-remediate-the-2026-11-01-debugging-pass.md),
R1 to R10). Each fix has a test that fails without it.

#### Fixed: two connections that refuse each other's requests no longer deadlock (R1)

A connection's reader wrote its own error answers: to an invalid line, and to a request refused over
`WithMaxConcurrentRequests`, past `WithMaxQueuedBytes` or during `Shutdown`. Two SDK peers over `io.Pipe`, as `acptest`'s pairs
and `agentproc.InProcess` connect them, each blocked in a write the other's reader had to read. Neither read again, and even
the requests they had admitted never answered. Over OS pipes the same happened once the pipe's buffer filled. (`fd4dcaf`.)

The reader now queues those answers for a goroutine of their own, which writes them in order. What changes:

- **The reader never waits on its peer.** It goes on reading while its answers wait to be written.
- **A peer that does not read is closed.** Past 1,024 unwritten answers, the connection closes itself. `Err()` wraps
  `acp.ErrConnectionClosed`, and `acp.EndedCleanly` is false for it.
- **The observer for `ObservedFault` and `ObservedRefused`** runs on that goroutine, after the answer is written, not on the
  reader. A slow observer of those kinds no longer stalls reading; it delays the next such answer. The order `v1.31.1`
  documented holds. Every other kind runs where it did.
- **At the end of the peer's output,** the queued answers are written before the connection ends, as the reader's own writes
  were.

#### Fixed: an agent's receiving middleware cannot change `initialize`'s answer (R2)

The agent records what `initialize` agreed as its dispatch answers, before the receiving chain returns. A middleware that changed
the result sent the client one answer while the agent kept another. The client's `session/load`, which its answer allowed, was
then refused by the agent's own gate.

On an agent's side connection, `initialize`'s answer must now pass the chain unchanged. A result that encodes otherwise, whether
replaced, changed in place, or returned without calling `next`, is answered `-32603` with `{"error": "a receiving middleware
replaced initialize's result"}` and logged at `Error`. The client then records nothing. A middleware may still fail `initialize`
with an error. A plain `Connection` does not check. This is the rule the sending side has had since `v1.31.0`. (`2ed2aa6`.)

#### Fixed: a request sent under another connection's notification keeps its barrier (R3)

A response is returned only after the notifications received before it are handled, except for a request a notification handler
sends on its own connection. A bridge whose notification handler on one connection sent a request on another skipped that
other connection's barrier: the request returned before the notification that preceded its response had run. A notification's
context now records its connection, and the exception applies only on that connection. (`2ed2aa6`.)

#### Fixed: `ServeAgent` waits for notification handlers at its context's end (R4)

At its input's end, `ServeAgent` waits up to 5 seconds for every handler. At its context's end, it waited only for the requests'
handlers, through `Shutdown`, and could return while a notification's handler, such as an agent's `Cancel`, still ran. It now
waits for every handler within the same 5 seconds. When they pass, its error wraps `acp.ErrHandlersRunning` beside the context's
cause. (`2ed2aa6`.)

#### Fixed: limits

- **R5.** `WithMaxQueuedBytes(math.MaxInt)`, the natural "no practical budget", closed the connection on the second notification
  under the default policy: twice the budget overflowed. It now stops at `math.MaxInt64`. (`022d0d1`.)
- **R6.** `WithMaxQueuedNotifications(math.MaxInt)` panicked building the connection. `n` is now at most 1,048,576; a larger
  value is lowered to it. The queue is allocated whole: 64 MiB at the maximum under the default policy. (`022d0d1`.)
- **R7.** `WithMaxMessageSize` counted a line's `\r` when it ended `\r\n`, so a message at the limit was refused. Both newlines
  are now measured alike. (`022d0d1`.)

#### Fixed: two sending-side cases

- **R8.** Params a sending middleware passes `next` that begin as an object or an array but are not valid JSON, or that are only
  white space, now fail with `acp.ErrSendingMiddleware` and write nothing. Before, they failed later with an encoding error that
  did not wrap it. (`022d0d1`.)
- **R9.** A request whose context ended before it was written no longer sends `$/cancel_request` for an id the peer never saw,
  and the observer no longer reports that cancel. (`022d0d1`.)

#### Documented: `Wait` and a writer that `Close` cannot close (R10)

`Wait` and `Close` now say that a writer without `io.Closer`, blocked in a `Write` to a peer that does not read, keeps `Wait`
waiting until that `Write` returns. (`2ed2aa6`.)

#### For maintainers

- **Tests.** Fifteen tests, new or extended, the emitter's included, each seen to fail with its fix reverted in a copy of the
  tree, are listed in the runtime PLAN's phase 4 by finding.
- **The generator** emits one line more in `NewAgentSideConnection`, `asc.conn.guardInitialize = AgentMethodInitialize`, which
  `TestSideConnectionsFollowTheModel` requires. (`2ed2aa6`.)
- **The benchmarks.** Allocations per operation are unchanged. A received notification's context value grew by one word, 8 bytes.

## Sources not in the release text

- **Commits since `v1.31.1`** (`git log v1.31.1..427c40d`), seven:
  - `d55ba2a` records the 0046 release and proposes the 0048 pair;
  - `5c3344c` adds the 0049 MADR;
  - `9f59ee2` records the owner's answers and adds the runtime PLAN;
  - `fd4dcaf`, phase 1 (R1): `connection.go`, `observer.go`, `connection_answers_test.go`, `connection_observer_test.go`,
    `README.md`, `docs/architecture.md`, and the 0046 MADR's amendment of 2026-11-02;
  - `022d0d1`, phase 2 (R5 to R9): `connection.go`, `middleware.go`, five test files, `README.md`, `docs/architecture.md`;
  - `2ed2aa6`, phase 3 (R2, R3, R4, R10): `connection.go`, `middleware.go`, `serve.go`, `agent_gen.go`, the emitter's
    `dispatch.go` and its test, three test files, `README.md`, `docs/architecture.md`, and the MADR's R2 amendment of
    2026-11-03;
  - `427c40d` records phase 4's gates.
- **Go files changed.** `git diff --name-status v1.31.1 427c40d -- '*.go'` lists seventeen:
  - the non-test files: `connection.go`, `middleware.go`, `observer.go`, `serve.go`, `agent_gen.go`, and
    `cmd/generate/internal/emit/dispatch.go`;
  - two new test files: `connection_answers_test.go` and `connection_initialize_guard_test.go`;
  - nine test files changed.
- **Unchanged** (`git diff --stat v1.31.1 427c40d`, empty for these):
  - `go.mod`, `go.sum` and `cmd/generate/go.mod`;
  - the schema, `.github/`, `RELEASING.md`;
  - `acptest/`, `agentproc/`, `clienthost/`, `remote/`, `internal/`;
  - `errors.go`, `types_gen.go`, `client_gen.go`.
- **`make api` on `427c40d`:** exit 0, "against v1.31.1: 0 incompatible, 0 compatible"; "reported packages unstable, remote: 0
  incompatible, not gated".
- **The gate set** on `2ed2aa6`'s tree, the runtime PLAN's phase 4 step 1: 18 of 18, with the Linux and Windows hosts.
- **CI on `427c40d`:** run `41720647295`, "completed success", six jobs (`validate` on Ubuntu, Windows and macOS at Go
  `1.27.3` and `1.26.10`), the push of `d55ba2a..427c40d` running CI whole; the Docs workflow's run `41720647338`,
  "completed success".
- **The consumer.** `tools/consumer.py` is unchanged; `consumer.py v1.31.2` runs after the tag.

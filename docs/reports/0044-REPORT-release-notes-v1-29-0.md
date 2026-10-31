# Release notes for v1.29.0, against v1.28.0

Observed 2026-10-31 on `934e33b`, for
[0044-PLAN-notification-queue-back-pressure.md](../decisions/0044-PLAN-notification-queue-back-pressure.md), phase 4. This report
drafts the text of the GitHub release for `v1.29.0`, as `RELEASING.md` describes it. It decides nothing. It is committed before the
tag, so the tagged tree holds it. The commit that adds it changes `tools/consumer.py`, `tools/README.md`, records and nothing else,
so the API it tags is `934e33b`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `934e33b` with only `tools/consumer.py`, `tools/README.md`, `docs/README.md` and the records uncommitted,
  which hold no Go package.

Summary:

- One addition to the exported API, `OverflowBlock`. `make api` against `v1.28.0`: "0 incompatible, 1 compatible". The release is
  the minor `v1.29.0`, as `RELEASING.md` says for compatible changes, with the change of default flagged first, as the 0044 MADR's
  D9 decides.
- One change of behaviour: a connection given no overflow policy, or an undefined one, now pauses reading at a full notification
  queue, where it closed.
- What a consumer may meet:
  - nothing the SDK sends or accepts on the wire changes;
  - a peer that sends notifications faster than they are handled now has its writes wait, where its connection closed with
    `ErrNotificationQueueOverflow`;
  - a notification handler that waits for something the peer sends later now waits until the queue has room, which a full queue
    never gives;
  - a consumer that names `OverflowCloseConnection` or `OverflowDropNewest` sees no change.
- The `go` lines and the requirements are unchanged, and no generated file changed.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.29.0

The ACP schema is still `1.25.0` (`git diff v1.28.0 -- schema/` is empty). This release answers
[issue #13](https://github.com/maccavelli/acp-go-sdk2/issues/13): a connection whose notification queue was full closed, or,
under `OverflowDropNewest`, lost updates. It adds back-pressure as a third overflow policy, and makes it the default
([0044-MADR-notification-queue-back-pressure.md](../decisions/0044-MADR-notification-queue-back-pressure.md)).

`make api` reports one compatible addition and no incompatible change. Nothing sent or accepted on the wire changes.

#### Changed: a full notification queue pauses reading, by default

A connection queues the notifications it receives while a notification handler runs, 1024 by default
(`WithMaxQueuedNotifications`). Until now a connection that named no policy closed when that queue was full, and `Err()` wrapped
`acp.ErrNotificationQueueOverflow`. Now it stops reading the peer until a handler returns. (`c4dcb77`.)

- **`acp.OverflowBlock`** is the new policy, and the default for a connection that names none, or names an undefined one.
  - The peer's writes wait while the reader is paused. Nothing is dropped, and the connection stays open.
  - An agent whose client is slow is slowed at its `SessionUpdate` calls, and `session/cancel` still ends its turn.
  - A pause that lasts 5 seconds is logged at `Warn`: `notification queue full; reading paused`, with the queue's `capacity` and
    how long it has been `paused`.
- **What waits behind a pause.** Nothing else is read either: a request from the peer, such as a permission request, waits behind
  the queue, and so does a `$/cancel_request`. A request this side sent returns no later than it would otherwise, since it returns
  only once the notifications received before its answer are handled.
- **The rule.** Under `OverflowBlock`, a notification handler must not wait for a request or a `$/cancel_request` the peer sends
  later, nor for code that waits for one: while the queue is full, it is not read.
- **Where the reader reads on.** While a notification handler waits for the answer to a request it sent under its own context, or
  while `Initialize` waits for its answer behind held notifications, the connection reads on, to twice the queue's size, since only
  the reader can deliver that answer. Past that it closes with `acp.ErrNotificationQueueOverflow`, as `OverflowCloseConnection`
  does.
- **Over package `remote`,** the server's own bounds still apply to a long pause: a WebSocket write waits at most `Grace`, and
  Streamable HTTP holds at most `MaxBuffered`.
- **To keep the close,** name it:

```go
conn := acp.NewClientSideConnection(client, stdin, stdout,
    acp.WithNotificationOverflowPolicy(acp.OverflowCloseConnection))
```

`OverflowCloseConnection` and `OverflowDropNewest` behave as before. Under `OverflowBlock`, the queue's buffer holds twice its
size, for the reading on above; the limit a pause starts at is the size given. (`c4dcb77`.)

#### For maintainers

- **Docs.** The godoc of `MethodHandler`, `WithMaxQueuedNotifications`, the three policies, `WithNotificationOverflowPolicy`,
  `Err`, `EndedCleanly` and `ErrNotificationQueueOverflow`, and `doc.go`'s "How a connection ends", say what the default does
  (`c4dcb77`). The README's "Closing and limits", its hold paragraph and its `Err` section, and `docs/architecture.md`, describe
  the pause, and `ExampleWithNotificationOverflowPolicy` shows one (`3845b28`).
- **Tests.**
  - `connection_overflow_block_test.go`, nine tests, each in a `synctest` bubble: the pause, the default, a response behind a full
    queue, a handler's own request, a held `initialize`, the read-on limit, the end on `Close` and `Shutdown`, an agent slowed by
    its client, and the 5 s log. (`c4dcb77`.)
  - Thirteen existing tests assumed the old default. Those of the close now name `OverflowCloseConnection`; those of the queue's
    size read its limit rather than its channel's capacity. (`c4dcb77`.)
- **Tools.** `tools/consumer.py` builds a module at the Go floor that also opens a client connection with
  `WithNotificationOverflowPolicy(acp.OverflowCloseConnection)` and names `acp.OverflowBlock`.

## Sources not in the release text

- **Commits since `v1.28.0`** (`git log v1.28.0..934e33b`), five:
  - `24e88d1` marks the 0042 PLAN complete;
  - `d9ff0c4` adds the 0044 MADR and PLAN;
  - `c4dcb77` changes `connection.go`, `doc.go` and `errors.go`, adds `connection_overflow_block_test.go`, and changes
    `acp_test.go`, `agentproc/ties_test.go`, `connection_close_test.go`, `connection_end_test.go`,
    `connection_overflow_policy_test.go`, `connection_queue_size_test.go`, `initialize_test.go` and `options_test.go`;
  - `3845b28` changes `README.md`, `docs/architecture.md` and `example_api_test.go`;
  - `934e33b` records phase 3's checks in the 0044 PLAN;
  - each changes `docs/README.md`, a MADR or a PLAN as well (`git show --name-only`).
- **Go files changed.** `git diff --name-status -M v1.28.0 934e33b -- '*.go' ':!*_test.go'` lists 3 files, each changed:
  `connection.go`, `doc.go` and `errors.go`.
- **Unchanged** (`git diff --stat v1.28.0 934e33b`, empty):
  - `go.mod`, `go.sum`, `cmd/generate/go.mod` and `cmd/generate/go.sum`;
  - the schema, the workflows, the `Makefile`, `RELEASING.md` and `scripts/`;
  - `acptest/`, `clienthost/`, `remote/` and `internal/`;
  - every `*_gen.go` file.
- **`make api` on `934e33b`:** exit 0, "compatible: OverflowBlock: added", "against v1.28.0: 0 incompatible, 1 compatible"; and
  "reported packages unstable, remote: 0 incompatible, not gated".
- **CI on `3845b28`**, pushed with `d9ff0c4` and `c4dcb77`:
  - run `41207967160`, "completed success", six jobs on Ubuntu, Windows and macOS at Go `1.27.3` and `1.26.10`;
  - the Docs workflow's run `41207967212`, "completed success".
- **The consumer before the tag.** A scratch script built `tools/consumer.py`'s program and test against `934e33b`'s tree through a
  `replace`, at `go1.26.10`, three times. The output equals `WANT_RUN`, its new line `overflow true 1 <nil> <nil> <nil>`, and its
  other lines are unchanged under the new default; the test and the Windows build pass. `tools/consumer.py v1.28.0`, a release
  without `OverflowBlock`, fails its build with "undefined: acp.OverflowBlock", as required. `consumer.py v1.29.0` runs after the
  tag.
- **The evidence:**
  - the 0044 MADR's findings F1 to F9, the probes of the response barrier, and the survey of other implementations under "Sources";
  - the 0044 PLAN's planted breaks: ten in phase 1, three in phase 3, each failing as required;
  - the new tests passing on macOS under `-race`, and on the Linux and Windows hosts, in phase 1;
  - the gates on macOS and on the Linux and Windows hosts, 18 of 18, in phase 3;
  - the deviation entries, each recorded in the PLAN.

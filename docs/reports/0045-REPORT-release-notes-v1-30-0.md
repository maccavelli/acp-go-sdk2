# Release notes for v1.30.0, against v1.29.0

Observed 2026-10-31 on `70b947f`, for
[0045-PLAN-notification-queue-byte-budget.md](../decisions/0045-PLAN-notification-queue-byte-budget.md), phase 4. This report
drafts the text of the GitHub release for `v1.30.0`, as `RELEASING.md` describes it. It decides nothing. It is committed before the
tag, so the tagged tree holds it. The commit that adds it changes `tools/consumer.py`, `tools/README.md`, records and nothing else,
so the API it tags is `70b947f`'s.

The release also carries the phase that
[0011-PLAN-wire-fixes.md](../decisions/0011-PLAN-wire-fixes.md) added on 2026-10-31, which left a patch release to the owner. The
owner chose to ship it here (that PLAN's deviation of 2026-10-31, and the 0045 PLAN's deviation 2).

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `70b947f` with only `tools/consumer.py`, `tools/README.md`, `docs/README.md` and the records uncommitted,
  which hold no Go package.

Summary:

- Two additions to the exported API, `WithMaxQueuedBytes` and `(*Connection).QueuedBytes`. `make api` against `v1.29.0`: "0
  incompatible, 2 compatible". The release is the minor `v1.30.0`, as `RELEASING.md` says for compatible changes, with the new
  default bound flagged first, as the 0045 MADR's "More Information" says.
- Two changes of behaviour:
  - a connection bounds in bytes what it holds of its peer's unfinished messages, 64 MiB by default, beside the notification
    count;
  - `RequestId` refuses a JSON object, which it took as its `Null` variant.
- What a consumer may meet:
  - under `OverflowBlock`, the default, a peer whose unfinished notifications pass 64 MiB has its writes wait, where only the
    count of 1024 paused it;
  - under `OverflowCloseConnection` or `OverflowDropNewest`, the connection closes or drops at 64 MiB of unfinished
    notifications, where only the count made it;
  - a request received while the bytes held would pass the budget is answered `-32603` at once, and its handler does not run;
  - decoding a JSON object into a `RequestId`, such as `$/cancel_request`'s `requestId`, fails;
  - nothing else sent or accepted on the wire changes.
- The `go` lines and the requirements are unchanged. One generated file changed, `types_gen.go`, from the generator's fix; the
  schema did not.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.30.0

The ACP schema is still `1.25.0` (`git diff v1.29.0 -- schema/` is empty). This release bounds in bytes what a connection keeps of
its peer's unfinished messages, beside the notification count
([0045-MADR-notification-queue-byte-budget.md](../decisions/0045-MADR-notification-queue-byte-budget.md)), and fixes how
`RequestId` decodes an object ([0011-MADR-harden-code-generator-engine.md](../decisions/0011-MADR-harden-code-generator-engine.md),
the amendment of 2026-10-31).

`make api` reports two compatible additions and no incompatible change.

#### Changed: a connection bounds what it holds of its peer in bytes, 64 MiB by default

`v1.29.0` bounds the notifications a connection queues by count, 1024 by default (`WithMaxQueuedNotifications`). With messages of
up to 10 MiB (`WithMaxMessageSize`), that count could hold about 10 GiB, and twice that while reading on. Now a connection also
counts bytes: every message it has received and not finished, notifications and requests together, each counted by its line's
length from receipt until its handler returns. (`7b87714`.)

- **`acp.WithMaxQueuedBytes(n)`** sets the budget, 64 MiB by default. A budget below `WithMaxMessageSize` is raised to it, so a
  single message always fits. A value of 0 or less is ignored.
- **Notifications at the budget.** Whichever of the count and the budget is reached first, each policy acts as it does at the
  count:
  - `OverflowBlock`, the default: the reader pauses until handlers return. Where it reads on past the count, it reads on to twice
    the budget, then closes with `acp.ErrNotificationQueueOverflow`.
  - `OverflowCloseConnection`: the connection closes with `acp.ErrNotificationQueueOverflow`.
  - `OverflowDropNewest`: the notification is dropped, counted by `DroppedNotifications`, and passed to the drop handler.

  A notification meets its policy only while another notification is held. A request's bytes alone never pause the reader, so a
  request handler that waits for the peer's answer to its own request still gets it.
- **Requests at the budget.** A request whose line would take the bytes held past the budget is answered at once with `-32603`
  and `{"error":"too many queued bytes","limit":n}`, and its handler does not run, as `WithMaxConcurrentRequests` refuses one.
  The reader never waits on a request's bytes.
- **Logs.** The pause's `Warn` and the overflow's `Error` add `bytes`, `size` and `byte_limit`.
- **`(*acp.Connection).QueuedBytes()`** reports the bytes held, for metrics. On a side connection it is
  `conn.Connection().QueuedBytes()`.
- **To raise it,** name it:

```go
conn := acp.NewClientSideConnection(client, stdin, stdout,
    acp.WithMaxQueuedBytes(256<<20)) // 256 MiB
```

At the default count of 1024, the budget binds only when the average notification held passes 64 KiB. (`7b87714`, `1e4a198`.)

#### Fixed: `RequestId` refuses a JSON object

JSON-RPC 2.0 allows an id to be a string, a number or `null`. `RequestId` took a JSON object as its `Null` variant, with no
error. Decoding one now fails with `RequestId: no variant matches`, as an array or a boolean already did. This reaches
`CancelRequestNotification`'s and `ElicitationRequestScope`'s `requestId`, and the envelope types; `Connection` checks the
envelope's `id` itself and was already correct. (`04d6673`.)

The generated unions also lose a trial decode that could never succeed, in `AuthMethod`, `McpServer`, `UnstableMcpServer`,
`MultiSelectItems` and `SetSessionConfigOptionRequest`. What they accept and their error texts are unchanged. (`04d6673`.)

#### For maintainers

- **Docs.**
  - The godoc of `WithMaxQueuedNotifications`, `ConnectionOption`, `WithMaxConcurrentRequests`, the three policies and
    `ErrNotificationQueueOverflow` names the budget (`7b87714`).
  - The README's "Closing and limits" gains "The byte budget", `docs/architecture.md` describes the counters, and
    `ExampleWithMaxQueuedBytes` shows a request refused (`1e4a198`).
  - The README's "Wire encoding" says what `RequestId` takes (`04d6673`).
  - Under Go 1.27's default build, `json.RawMessage` is `jsontext.Value`. The README, and the godoc of `MethodHandler` and
    `ExtensionMethodHandler`, say that code written with `encoding/json/v2` may declare params as `jsontext.Value`
    ([0007-MADR-self-contained-acp-sdk.md](../decisions/0007-MADR-self-contained-acp-sdk.md), the amendment of 2026-10-31;
    `9fcbb6f`).
- **Tests.**
  - `connection_queue_bytes_test.go`, seven tests, each waiting in a `synctest` bubble: the pause, the read-on limit, the close and
    the drop, the raise to the message size and a single line, a request refused, the default, and a request handler that awaits
    its peer while notifications arrive. (`7b87714`.)
  - `TestRequestIdRefusesObjects`, and the generator's engine case `accept-null-variant`. (`04d6673`.)
- **Tools.** `tools/consumer.py` builds a module at the Go floor that also sets `WithMaxQueuedBytes`, reads `QueuedBytes`, and
  requires a `RequestId` to refuse `{}`.

## Sources not in the release text

- **Commits since `v1.29.0`** (`git log v1.29.0..70b947f`), eight:
  - `3d552f4` marks the 0044 PLAN complete;
  - `6fc5929` names gobble-cli in the records and records 0044's phase 5;
  - `9fcbb6f` changes `README.md`, `connection.go` and `extensions.go`, doc comments only, and amends the 0007 MADR;
  - `fd8e586` adds the 0045 and 0046 MADRs and PLANs and amends the 0010 and 0011 pairs;
  - `04d6673` changes `cmd/generate/internal/emit/types.go`, `types_gen.go` and `README.md`, and adds
    `cmd/generate/testdata/engine/accept-null-variant.txtar` and a test in `union_wire_test.go`;
  - `7b87714` changes `connection.go` and `errors.go`, and adds `connection_queue_bytes_test.go`;
  - `1e4a198` changes `README.md`, `docs/architecture.md` and `example_api_test.go`;
  - `70b947f` records phase 3's checks in the 0045 PLAN;
  - each changes `docs/README.md`, a MADR, a PLAN or a report as well (`git show --name-only`).
- **Go files changed.** `git diff --name-status -M v1.29.0 70b947f -- '*.go' ':!*_test.go'` lists 5 files, each changed:
  `cmd/generate/internal/emit/types.go`, `connection.go`, `errors.go`, `extensions.go` and `types_gen.go`.
- **Unchanged** (`git diff --stat v1.29.0 70b947f`, empty):
  - `go.mod`, `go.sum`, `cmd/generate/go.mod` and `cmd/generate/go.sum`;
  - the schema, the workflows, the `Makefile`, `RELEASING.md` and `scripts/`;
  - `acptest/`, `agentproc/`, `clienthost/`, `remote/` and `internal/`;
  - every `*_gen.go` file but `types_gen.go`.
- **`make api` on `70b947f`:** exit 0, "compatible: (*Connection).QueuedBytes: added", "compatible: WithMaxQueuedBytes: added",
  "against v1.29.0: 0 incompatible, 2 compatible"; and "reported packages unstable, remote: 0 incompatible, not gated".
- **CI on `1e4a198`**, pushed with `fd8e586`, `04d6673` and `7b87714`:
  - run `41242795930`, "completed success", six jobs on Ubuntu, Windows and macOS at Go `1.27.3` and `1.26.10`;
  - the Docs workflow's run `41242795883`, "completed success".
- **The consumer before the tag.** A scratch script built `tools/consumer.py`'s program and test against the tree through a
  `replace`, at `go1.26.10`, three times. The output equals `WANT_RUN`, whose new lines are `queued bytes 0` and
  `request id RequestId: no variant matches`; the test and the Windows build pass. `tools/consumer.py v1.29.0`, a release without
  the budget, fails its build with "undefined: acp.WithMaxQueuedBytes", as required. With `04d6673`'s `types_gen.go` reverted in
  a scratch copy, the run differs first at line 29, `request id <nil>`. `consumer.py v1.30.0` runs after the tag.
- **The evidence:**
  - the 0045 MADR's findings F1 to F8, and the survey of other implementations under "Sources";
  - the 0045 PLAN's planted breaks: ten in phase 1 and three in phase 3, each failing as required, and the example's in phase 2;
  - the 0011 PLAN's added phase: its planted break, and the 108 decodes compared before and after, of which only the 13
    `RequestId` objects changed;
  - the gates on macOS and on the Linux and Windows hosts, 18 of 18, in the 0045 PLAN's phase 3;
  - the deviation entries, each recorded in its PLAN.

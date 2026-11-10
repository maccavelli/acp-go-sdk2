# Release notes for v1.35.0, against v1.34.2

Observed 2026-11-10 on `62f62aa` with phase 6's changes in the working tree, for
[0052-PLAN-agent-tool-call-snapshot-stream-and-message-size-default.md](../decisions/0052-PLAN-agent-tool-call-snapshot-stream-and-message-size-default.md),
phase 6, step 4. This report drafts the text of the GitHub release for `v1.35.0`, as `RELEASING.md` describes it. It decides
nothing. It is committed before the tag, so the tagged tree holds it. Phase 6 changes `tools/`, a test helper and records,
and no exported API, so the API it tags is `62f62aa`'s.

Each statement names its source:

- a record;
- a commit;
- or a command run in phase 6, whose output the PLAN's execution record holds.

Summary:

- 7 compatible additions and nothing incompatible. `make api` against `v1.34.2`: "0 incompatible, 7 compatible", all in the
  root package, so the release is a minor, as `RELEASING.md` says and the MADR's question 6 decided.
- It adds `ToolCallStream`, by
  [0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md](../decisions/0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md),
  D1 to D5, from item 1 of
  [0051-REPORT-acp-1-25-0-featureset-coverage-and-three-extensions.md](0051-REPORT-acp-1-25-0-featureset-coverage-and-three-extensions.md).
- Two behaviour changes: a prompt turn's unfinished streams are written before its answer (D3), and the default message size
  is 32 MiB (D6).
- The schema, the `go` lines and the requirements are unchanged.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.35.0

The ACP schema is still `1.25.0` (`git diff v1.34.2 -- schema/` is empty). `make api` reports seven compatible additions
against `v1.34.2`, all in the root package, and nothing incompatible. This release lets an agent send a tool call while it
runs, as the schema's replace-whole `tool_call_update` allows, and raises the default message size to the TypeScript SDK's
([0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md](../decisions/0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md)).

Nothing that `v1.34.2` accepted is refused. Read "Changed: the default message size is 32 MiB" for what a reader now holds in
memory.

#### Added: `ToolCallStream` (D1, D2, D4, D5)

`acp.StartToolCallStream(ctx, conn, session, start, opts...)` sends `start`, a `tool_call` such as `acp.StartToolCall`
returns, and returns the call's `*acp.ToolCallStream`. ACP v1 has no update that appends: each `tool_call_update` replaces
the `content` and `locations` it carries. A stream therefore sends the call's whole state each time, the latest, at most once
per interval, so a client may drop or merge updates without losing any, and `acp.ToolCallTimeline` applies them.

- **Output.** A stream is an `io.Writer` and an `io.StringWriter`: `cmd.Stdout = stream` sends a command's output. What is
  written is the call's live text, one text `content` item after the call's other content. It keeps the last 1 MiB
  (`acp.WithToolCallStreamMaxText`), cut at a character boundary; `Dropped` counts what was cut. A character a write began and
  did not finish is held back until a later write does.
- **Other fields.** `SetStatus` takes `pending` or `in_progress`. `Update(func(*acp.ToolCallEntity)) error` changes the
  title, kind, name, content, locations, and `rawInput` and `rawOutput`. The call's `Id` and `Status` are the stream's: a
  change to either is undone and returns an error wrapping `acp.ErrToolCallStreamField`.
- **Sending.** No change blocks on the wire. A sender the connection counts writes the latest state at most every 250 ms
  (`acp.WithToolCallStreamInterval`) and exits when nothing is pending. A change made while a slow client holds a write joins
  the next write; nothing queues. `Flush` writes now, after a write in flight. `Err` returns the first error of a write the
  SDK made, such as a location that is not absolute.
- **Finishing.** `Finish(ctx, status)` writes `completed` or `failed` with the whole state. It honours `ctx` but for the
  turn's own end: after `session/cancel` or `Shutdown`, `Finish` on `Prompt`'s context still writes. A deadline of the
  caller's stops it, and the stream stays open for another `Finish`. After it, the stream's methods return
  `acp.ErrToolCallStreamFinished`.
- **Not done.** No append or delta on the wire, no terminal output (a `terminal` content item is shown by the client), no
  partial `rawOutput`, and no `session/load` replay.

The README's "Streaming a tool call" has the example and the rules. (`38e9772`, `62f62aa`.)

#### Changed: a turn's unfinished streams are written before its answer (D3)

A stream started on the context of an agent's `Prompt`, or one derived from it, belongs to that prompt turn. When `Prompt`
returns, the generated dispatch writes the turn's unfinished streams before the `session/prompt` answer, their status as it
was, so the answer never overtakes a call's last state. It writes on the request's context without its cancellation and
holds no lock while it does, so a `session/cancel` is handled meanwhile. Under `PromptOverlapConcurrent` each turn waits for
its own streams alone. An agent that starts no stream sees no change: its turn holds none. (`4c7f943`.)

#### Changed: the default message size is 32 MiB (D6)

`acp.WithMaxMessageSize`'s default, and `remote`'s `DialOptions.MaxMessageSize` and `HandlerOptions.MaxMessageSize` when
zero, are 32 MiB, up from 10 MiB, the TypeScript SDK's `DEFAULT_MAX_MESSAGE_BYTES`. ACP sets no message size. A line from
10 MiB to 32 MiB, which `v1.34.2` answered `-32600` and discarded, is now read. A connection may now hold up to 32 MiB of one
line while it reads it. `WithMaxQueuedBytes`' 64 MiB default and `remote`'s 64 MiB `MaxBuffered` are unchanged and still hold
one message. An explicit option is unchanged. (`8e7caad`.)

#### For maintainers

- **A start only while the connection lives.** `Connection.goIfLive` starts a goroutine the connection's wait group counts,
  from a goroutine it does not, such as an agent's tool worker. `writeLoop`'s first defer marks the end under the lock it
  takes, which `TestWriteLoopEndsLiveFirst` pins; `TestGoStatementsCounted` is unchanged. (`8969f86`.)
- **`internal/tailbuf`.** `clienthost`'s terminal output buffer moved to an internal package that `clienthost` and the stream
  share; `clienthost`'s API and behaviour are unchanged, and its tests moved with it. (`38e9772`.)
- **The generator.** `promptCase` emits the turn's stream set and its flush. `output.Check` type-checks the root through the
  importer that resolves the module's own packages from source, since the root now imports `internal/tailbuf`. (`38e9772`,
  `4c7f943`.)
- **`tools/consumer.py`** streams a tool call in its test, so its floor is `v1.35.0`.
- **Tests.** Each decision has a test that fails with its change undone in a copy of the tree; the PLAN's execution record
  lists each planted break and its failure. `BenchmarkToolCallStream`: one stream, 1,000 writes of 100 bytes and `Finish`,
  about 1.07 ms and 201 allocations per operation on darwin/arm64.

## Sources not in the release text

- **Commits since `v1.34.2`** (`git log v1.34.2..62f62aa`), eight:
  - `1e945dd` records the 0049 generator fixes as released in `v1.34.2`;
  - `957e60b` adds the 0051 report;
  - `c630d2f` adds the 0052 MADR and PLAN;
  - `8e7caad`, phase 1 (D6): `connection.go`, `remote/client.go`, `remote/handler.go`, `jsonrpc_test.go`,
    `remote/ws_test.go`, `README.md`, `docs/architecture.md`;
  - `8969f86`, phase 2 (D2's start): `connection.go`, `connection_end_test.go`;
  - `38e9772`, phase 3 (D1, D2, D4, D5, and the generator's check): `toolstream.go`, `toolstream_test.go`,
    `example_toolstream_test.go`, `errors.go`, `internal/tailbuf/`, `clienthost/terminals.go`, `clienthost/output.go` and
    `output_test.go` deleted, `cmd/generate/engine_test.go`, `cmd/generate/internal/output/check.go` and `check_test.go`;
  - `4c7f943`, phase 4 (D3): `connection.go`, `toolstream.go`, `toolstream_test.go`, `agent_gen.go`,
    `cmd/generate/internal/emit/dispatch.go` and `dispatch_test.go`;
  - `62f62aa`, phase 5: `README.md`, `docs_test.go`, `docs/architecture.md`.
- **Go files changed.** `git diff --name-status v1.34.2 62f62aa -- '*.go'` lists 22: five new (`toolstream.go`,
  `toolstream_test.go`, `example_toolstream_test.go`, `internal/tailbuf/tailbuf.go`, `internal/tailbuf/tailbuf_test.go`), two
  deleted (`clienthost/output.go`, `clienthost/output_test.go`), and fifteen changed.
- **Unchanged** (`git diff --stat v1.34.2 62f62aa`, empty for these): `go.mod`, `go.sum`, `cmd/generate/go.mod`,
  `cmd/generate/go.sum`, the schema, `.github/`, `unstable/`, `agentproc/`, `acptest/` and `RELEASING.md`.
- **`make api`,** in phase 6: exit 0, "compatible: ErrToolCallStreamField: added", "compatible: ErrToolCallStreamFinished:
  added", "compatible: StartToolCallStream: added", "compatible: ToolCallStream: added", "compatible: ToolCallStreamOption:
  added", "compatible: WithToolCallStreamInterval: added", "compatible: WithToolCallStreamMaxText: added", "against v1.34.2: 0
  incompatible, 7 compatible"; "reported packages unstable, remote: 0 incompatible, not gated". `ToolCallStream`'s nine
  methods are part of the type it reports added, and are not listed one by one.
- **The gate set,** phase 6, step 2, with the Linux and Windows hosts: see the PLAN's execution record, phase 6.
- **Interop,** phase 6, step 2: `python3 tools/interop.py`, exit 0, "all pairs passed", eight pairs against the official
  TypeScript and Python SDKs at their pinned revisions, over Streamable HTTP and WebSocket.
- **The consumers,** read as data in the MADR's F15 to F19; this repository changes neither. gobble-cli gains a producer only
  when its tools get an output hook, and its CLI view would print each snapshot again until it redraws. `tools/consumer.py
  v1.35.0` runs after the tag.

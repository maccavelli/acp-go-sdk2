# Release notes for v1.25.0, against v1.24.0

Observed 2026-10-27 on `43a76cc`, for
[0041-PLAN-close-the-gaps-the-consumers-wait-on.md](../decisions/0041-PLAN-close-the-gaps-the-consumers-wait-on.md),
phase 17. This report drafts the text of the GitHub release for `v1.25.0`, as `RELEASING.md` describes it. It decides nothing. It
is committed before the tag, so the tagged tree holds it. The commit that holds it changes only records and `tools/`, so the API it
tags is `43a76cc`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `43a76cc` with only `tools/consumer.py`, `tools/README.md` and `docs/README.md` uncommitted, which hold no
  Go package.

Summary:

- Thirty-five additions to the exported API: thirteen in the root package, three in `agentproc`, fourteen in `acptest`, and the
  new package `clienthost`'s five. `make api` against `v1.24.0`: "0 incompatible, 35 compatible". The release is the minor
  `v1.25.0`, as `RELEASING.md` says for compatible changes.
- One defect fixed: `agentproc` could lose the agent's last output.
- What a consumer may meet:
  - nothing the SDK sends or accepts on the wire changes;
  - `agentproc`'s `Close` reads the agent's output to its end before it closes the connection, and so can take up to one grace
    longer when the output stays open after the agent exits;
  - `acptest.Connect`'s cleanup waits at most 10 seconds.
- The `go` lines and the requirements are unchanged, and no generated file changed.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.25.0

The ACP schema is still `1.25.0` (`git diff v1.24.0 -- schema/` is empty). This release fixes a defect in `agentproc`, and adds what
the SDK's two consumers had each written for themselves, or were waiting on
([0041-MADR-close-the-gaps-the-consumers-wait-on.md](../decisions/0041-MADR-close-the-gaps-the-consumers-wait-on.md)):

- client hosts for the terminal and file-system methods;
- the causes of a connection's end, and a bounded wait;
- a tool-call timeline;
- a check of a value against the negotiated capabilities;
- `Kind()` on the remaining unions;
- a fuller `acptest`.

`make api` reports thirty-five compatible additions, one new package among them, and no incompatible change. Nothing sent or
accepted on the wire changes.

#### Fixed

- **`agentproc` reads the agent's last output.** `Close` could close the connection while output the agent wrote before it exited
  was still in the pipe, unread. As much as a pipe buffer was lost, 64 KiB on macOS, neither delivered nor counted as dropped. It
  showed as a rare failure of `TestDetachedTapIsPrefix` in `v1.24.0`'s tag CI, and needed a reader behind the agent when it
  exited.
  - Once the agent has exited, the connection now reads to the end of the agent's output, up to the grace counted from the exit,
    before it is closed. `Kill` still closes at once.
  - So `Close` can take up to one grace longer than before, and only when the agent's output stays open after its exit, as when a
    process that left the agent's group or job holds it.
  - (`64d3c19`.)

#### New package: `clienthost`

A client embeds these in its `acp.Client`; the root package still does no file or process access of its own.

- **`Terminals`** serves the five terminal methods.
  - `Command` builds each request's `*exec.Cmd`, for the client's own policy or shell. A nil `Command` is **`DirectCommand`**:
    the request's command run without a shell, this process's environment with the request's after it, and a relative `cwd`
    refused.
  - The output's `outputByteLimit` is kept by dropping from the front at a character boundary, and `truncated` is reported.
    `MaxOutputBytes` bounds what a terminal keeps whatever the agent asks. Its zero value means **`DefaultMaxOutputBytes`**,
    16 MiB.
  - The exit maps to `exitCode` or `signal`.
  - Kill and release end the command's process group, or on Windows its job. When a command exits of itself, what is left of its
    group or job is ended then.
  - `Close` ends every terminal, and an unknown terminal id is answered resource-not-found.
  - (`0ad2fe3`.)
- **`Files`** serves `fs/read_text_file` and `fs/write_text_file` from the disk.
  - `line` and `limit` count from 1, as the schema defines them.
  - A read of a missing file, or a write under a missing directory, is answered resource-not-found, `-32002`, unless `MkdirAll`
    is set for the write.
  - A path that is not absolute, a `line` of 0, and a path that is not a regular file are refused.
  - A write replaces the file's content and keeps its mode.
  - Confining paths to a session's roots, asking the user before a write, and answering from an editor's unsaved changes stay the
    client's. Its own `ReadTextFile` or `WriteTextFile` wins over the embedded one, and calls it.
  - (`cc324d3`, `43a76cc`.)
- **`Refusals`** is `acp.ClientRefusals` a level deeper. Embedded beside `Terminals` or `Files`, it refuses the other optional
  methods without the ambiguous selector `acp.ClientRefusals` would make there. (`0ad2fe3`.)

```go
type client struct {
    clienthost.Terminals // the terminal methods, with c.Command for policy
    clienthost.Files     // the file-system methods
    clienthost.Refusals  // method-not-found for the rest
}
```

#### New for clients

- **`ToolCallTimeline`**, beside `CompactionTimeline`, keeps a session's tool calls, live or replayed by `session/load`.
  - `Apply` takes a `tool_call` as the call, whole, and each `tool_call_update` replaces the fields it carries. `content` and
    `locations` are replaced, never appended, and an empty one clears them.
  - An update for an id never seen starts the call, `pending`.
  - `Get` and `Open` read it back, `Open` listing the calls neither `completed` nor `failed`. Each entity, a
    **`ToolCallEntity`**, is a copy.
  - (`d6ba000`.)
- **`agentproc.LineTap(fn)`** is a writer, for `OutputTap` or `InputTap`, that calls `fn` with each wire frame, without its
  newline. A line split across writes is joined; each frame is a copy. (`18b6dc3`.)
- **`Process.ExitedFirst()`** reports whether the agent's exit was seen before the connection ended. It is final once `Wait` has
  returned, so a client can report the exit status without a timer. (`cb3d050`.)
- **`Process.WaitContext(ctx)`** is `Wait` bounded by `ctx`. (`ba5a06f`.)

#### New for both sides

- **`EndedCleanly(err)`** reports whether a connection's `Err`, or `Wait`'s error, is a clean end:
  - true when this side closed the connection, or the peer ended its output;
  - false for the close a notification overflow makes, for a read error, and for nil.
  - (`cb3d050`.)
- **`Connection.WaitContext(ctx)`** returns `Wait`'s error, or `context.Cause(ctx)` when `ctx` ends first. It leaves the connection
  as it was, and starts at most one goroutine, which ends with the connection. A side connection's is
  `side.Connection().WaitContext(ctx)`. (`ba5a06f`.)
- **`Negotiation.AllowsValue(method, params)`** returns nil, or the error a send of `params` would get before it is sent: the
  `*CapabilityError` that names the property or variant not advertised, such as an MCP server's transport or a prompt's image
  block. So a caller can leave it out first. `Allows(method)` is unchanged. (`2dcb38c`.)
- **`IsResourceNotFound(err)`**, beside `IsMethodNotFound`, recognises a `-32002` answer, such as a client's for a missing file.
  (`cc324d3`.)
- **`Kind()`** on the six stable unions without it: `AuthMethod`, `CreateElicitationRequest`, `CreateElicitationResponse`,
  `McpServer`, `MultiSelectItems` and `SetSessionConfigOptionRequest`. It is also on the unstable-origin `PlanUpdateContent`, which
  stays deprecated.
  - A variant sent with no discriminator returns the schema's own name for it: `"agent"`, `"stdio"`, `"value_id"` or `"titled"`.
  - A typed catch-all returns the value it holds, as it arrived.
  - (`12da4e0`.)

#### New in `acptest`

- **`ConnectRecorded(tb, agent, client, rec, opts...)`** records the client side's frames into a `Recorder`, both ways.
  - Each frame is a **`Record`**, with its **`Direction`**, **`Sent`** or **`Received`**.
  - **`Recorder.Records()`** returns them, and **`Recorder.Wait(ctx, n)`** waits for `n`.
  - (`696398c`.)
- **`ValidateGolden(tb, records)`** decodes each recorded frame into its method's type, a response by its request's, and validates
  it. It fails the test on an error or an unknown method. (`696398c`.)
- **`Turn`** gains:
  - **`Err`**, a turn that fails;
  - **`Usage`**, the token usage its answer reports;
  - **`WaitCancel`**, a turn that waits for `session/cancel` and answers `cancelled`.
  - (`696398c`, `43a76cc`.)
- **`MemoryClient`**:
  - **`SetElicitation`** answers elicitations from a script, each accept checked against its form;
  - **`Terminals`**, a **`TerminalHost`** such as a `*clienthost.Terminals`, serves the terminal methods.
  - Without them, both still answer method-not-found.
  - (`696398c`.)
- **`Connect`'s cleanup waits at most 10 seconds.** A handler that ignores its context now fails the test, with its goroutines
  still running, where it used to hang it. (`ba5a06f`.)

#### For maintainers

- **`internal/proctree`** holds the process-tree code `agentproc` and `clienthost` share, moved from `agentproc` with no exported
  change there. `tools/platforms.py` vets it, and `clienthost`, for each platform their files are built for. (`c197013`,
  `0ad2fe3`.)
- **`make api`** compares `clienthost` too. (`0ad2fe3`.)
- **`TestGoStatementsCounted`** allows exactly one `go` statement in the root package, `WaitContext`'s, and still fails on any
  other. (`ba5a06f`.)
- **Tools.** `tools/consumer.py` builds a module at the Go floor that also:
  - waits for its agent with `Process.WaitContext` and checks `EndedCleanly`;
  - keeps a tool call in a `ToolCallTimeline`;
  - writes and reads a file and runs a terminal through `clienthost`.
- **Tests.**
  - At the root: `allows_value_test.go`, `kind_remaining_test.go`, and `tooltimeline_test.go` with
    `testdata/toolcalls/lifecycle.jsonl`.
  - In `agentproc`: `TestCloseReadsTheAgentsLastOutput`, `TestKillClosesAtOnce`, `TestExitedFirst`, `linetap_test.go` and
    `waitcontext_test.go`.
  - `clienthost`'s own tests, and in `acptest`, `additions_test.go`, `golden_internal_test.go` and `pair_internal_test.go`.
  - `TestRuntimeFilesCoverHandWritten` names `tooltimeline.go`.

## Sources not in the release text

- **Commits since `v1.24.0`** (`git log v1.24.0..43a76cc`), sixteen:
  - `64d3c19`, `18b6dc3`, `cb3d050`, `ba5a06f`, `c197013`, `0ad2fe3`, `cc324d3`, `2dcb38c`, `d6ba000`, `12da4e0` and `696398c`
    change the packages, as above;
  - `0ad2fe3` also `scripts/check_api.py` and `scripts/test_check_api.py`;
  - `c197013` and `0ad2fe3` also `tools/platforms.py`;
  - `d6ba000` also `cmd/generate/engine_test.go`;
  - `43a76cc` changes the docs of `clienthost/files.go` and `acptest/script.go`, `README.md`, `docs/architecture.md` and the
    PLAN;
  - `db92e25` changes `README.md`, `docs/architecture.md` and the PLAN;
  - `8d30c43`, `d9c632f` and `2deef5b` change records only (`git show --name-only`).
- **Go files changed.** `git diff --name-status -M v1.24.0 43a76cc -- '*.go' ':!*_test.go'` lists 31 files.
  - Added: `acptest/golden.go`, `agentproc/linetap.go`, `agentproc/tree.go`, `tooltimeline.go`, the seven files of
    `clienthost`, and the five of `internal/proctree`. `agentproc/tree_windows.go` moved to `internal/proctree/tree_windows.go`.
  - Removed from `agentproc`: `deathsig_none.go`, `deathsig_pdeathsig.go`, `tree_other.go` and `tree_unix.go`.
  - Changed: `acptest/client.go`, `doc.go`, `pair.go`, `recorder.go` and `script.go`; `agentproc/agentproc.go` and `tap.go`; and
    `connection.go`, `errors.go` and `helpers.go`.
- **Unchanged** (`git diff`):
  - `go.mod`, `go.sum`, `cmd/generate/go.mod` and `cmd/generate/go.sum`;
  - the schema, the examples, the workflows, the `Makefile` and `RELEASING.md`;
  - every `*_gen.go` file.
- **`make api` on `43a76cc`:** exit 0, "against v1.24.0: 0 incompatible, 35 compatible", each "added":
  - the root package's thirteen: `(*Connection).WaitContext`, `AuthMethod.Kind`, `CreateElicitationRequest.Kind`,
    `CreateElicitationResponse.Kind`, `EndedCleanly`, `IsResourceNotFound`, `McpServer.Kind`, `MultiSelectItems.Kind`,
    `Negotiation.AllowsValue`, `PlanUpdateContent.Kind`, `SetSessionConfigOptionRequest.Kind`, `ToolCallEntity` and
    `ToolCallTimeline`;
  - `agentproc`'s three: `(*Process).ExitedFirst`, `(*Process).WaitContext` and `LineTap`;
  - `acptest`'s fourteen: `(*MemoryClient).SetElicitation`, `(*Recorder).Records`, `(*Recorder).Wait`, `ConnectRecorded`,
    `Direction`, `MemoryClient.Terminals`, `Received`, `Record`, `Sent`, `TerminalHost`, `Turn.Err`, `Turn.Usage`,
    `Turn.WaitCancel` and `ValidateGolden`;
  - `clienthost`'s five: `DefaultMaxOutputBytes`, `DirectCommand`, `Files`, `Refusals` and `Terminals`;
  - and "reported packages unstable: 0 incompatible, not gated".
- **`MemoryClient`'s methods.** `CreateElicitation` and the five terminal methods are not in `make api`'s list. `MemoryClient`
  had them already, promoted from its embedded `acp.ClientRefusals`; they are now its own, and answer method-not-found as before
  unless `SetElicitation` or `Terminals` is set.
- **CI on `43a76cc`:** run `40609495195`, "completed success", six jobs on Ubuntu, Windows and macOS at Go `1.27.3` and `1.26.10`;
  and the Docs workflow's run `40609495196`, "completed success".
- **The evidence:**
  - the 0041 PLAN's phases 2 to 16: the reproduction of the lost output, 37 failures in 40 runs, and the 64 planted breaks of
    phases 3 to 13;
  - `TestDetachedTapIsPrefix`, `TestCloseReadsTheAgentsLastOutput` and `TestKillClosesAtOnce` passing 100 times each on macOS
    under `-race` and on the Linux host, and the first two on the Windows host;
  - the gates on macOS and on the Linux and Windows hosts;
  - the deviation entries, each recorded in the PLAN and, where a decision changed, in the MADR;
  - the two consumers' sites, read at their heads in
    [0041-REPORT-consumer-adoption-of-v1-21-to-v1-25.md](0041-REPORT-consumer-adoption-of-v1-21-to-v1-25.md).

---
status: accepted
date: 2026-10-27
decision-makers: Repository owner
---

# Close the gaps the two consumers wait on: client hosts, end causes, a fuller acptest, a tool-call timeline, and agentproc's lost output

## Context and Problem Statement

The owner asked, on 2026-10-27, which units of work across this repository's records are still not done, especially those
magic-cli-remote and `<cli-b>` wait on, and for ten items that would close the gaps between this SDK and those two consumers,
including the `TestDetachedTapIsPrefix` failure found in
[0040-PLAN-check-elicitation-forms-answers-and-urls.md](0040-PLAN-check-elicitation-forms-answers-and-urls.md)'s release.
`<cli-b>` stands for the second consumer, as in
[0035-REPORT-consumer-acp-code-for-the-sdk.md](../reports/0035-REPORT-consumer-acp-code-for-the-sdk.md).

Findings, from a read-only investigation at `2200989` (`v1.24.0`) on 2026-10-27, of this repository's records and of both
consumers: magic-cli-remote at its current `main`, and `<cli-b>` at its current `main`, including its uncommitted records.

### What this repository has left open

Every PLAN in `docs/README.md` is `complete`, but for 0002's, `superseded`. Every issue on the repository is closed. What is open
is what the records deferred:

| Source | Open item | State |
| :--- | :--- | :--- |
| 0035 C4 | a client terminal host, truncating at a character boundary | unbuilt; 0039 and 0040 left it to a later record |
| 0035 C5 | client file reads and writes with `line` and `limit`, a missing file as `-32002` | unbuilt |
| 0035 C6 | `Negotiation.AllowsValue`, the value gates exposed | unbuilt |
| 0035 C7 | `agentproc.LineTap`, a tap's bytes as frames | unbuilt |
| 0035 D1, `<cli-b>`'s 0016 D6 | `acptest.ValidateGolden`, and the parts of `<cli-b>`'s harness 0039's `acptest` lacks | unbuilt |
| 0035 E1 | a bounded `Wait` | decided against in [0026-MADR-connection-end-cause-and-wait.md](0026-MADR-connection-end-cause-and-wait.md), question 4 |
| 0035 E4 | the agent's exit versus the connection's end | would amend [0027-MADR-agent-process-helper.md](0027-MADR-agent-process-helper.md) |
| 0035 E2, E3 | the refuse code; `session/close` waiting for its turns | E2 reopens 0021; E3 no record decides |
| 0036, 0038 | `Kind()` on the other unions, "waits for a policy" | 0039's unstable boundary is that policy |
| 0040, "Not this decision" | scripted elicitation in `acptest.MemoryClient`; `example/client` | unbuilt |
| 0040's phase 8 deviation | `TestDetachedTapIsPrefix` failed once in the tag's CI | its cause is left to a record of its own |

Schema `1.26.0` stays deferred by the owner, and is not counted.

### What the consumers wait on, and what they have not adopted

Both consumers pin `v1.20.0` (magic-cli-remote `go.mod:11`, `<cli-b>` `go.mod:15`), four minor releases behind, and call none of
the API `v1.21.0` to `v1.24.0` added. That gap is theirs to close, under their own records, and most of it needs nothing new here:

* **magic-cli-remote** still has, among others:
  * a substring match on "allow" for auto-approval (`internal/provider/acpagent/session.go:2593`–`:2642`), which
    `PickPermissionOption` replaces, and which 0035 G called "the one with user-visible risk";
  * a 300-line elicitation checker (`elicitation.go:151`–`:454`), which 0040's helpers replace;
  * grouped config options silently dropped (`session.go:2288`), which `SessionConfigSelectOptions.All()` covers;
  * error classification by literal code (`runtime.go:342`–`:347`).
* **`<cli-b>`** still has, among others:
  * a 1,270-line `acptest` of its own (`acpclient/acptest/`), used about 380 times;
  * eleven refusal stubs, which `ClientRefusals` and `AgentRefusals` cover;
  * an extension dispatcher, which `ExtensionMux` covers, though its agent side decodes with `encoding/json/v2`;
  * its own stdio command resolution and auth-method filter.

Records of theirs are stale against this repository:
* magic-cli-remote's `0207-MADR-move-to-current-library-releases-and-close-the-adoption-gaps.md` still says `v1.20.0` "is the
  newest tag";
* its 0184 PLAN lists issue #4 as open, though #4 was closed on 2026-10-15;
* `<cli-b>`'s 0034 pair targets `v1.21.0`.

What the consumers wait on from this SDK, in their own words or code, is a shorter list:

* **A default client for the file system and terminals.**
  * `<cli-b>`'s `0031-MADR-sdk-current-releases-and-adoption.md:98`–`:99`: "acp-go-sdk2: … a default client for fs, terminal and
    elicitation (`<cli-b>`'s stubs stay). `<cli-b>` keeps these until the SDKs have them."
  * magic-cli-remote's terminal host (`terminal.go:20`–`:330`) breaks the schema's "The Client MUST ensure truncation happens at a
    character boundary" (`types_gen.go:4126`). Its own comment at `:278` says: "Drop from the front at a byte boundary (may split
    UTF-8; acceptable for tool output)."
  * Its file reads answer a missing file `-32603`, not `-32002` (`session.go:2648`–`:2673`).
* **A bounded wait for a connection's goroutines.** `<cli-b>` keeps three copies of `handlersDone(wait, limit)`:
  `acpclient/conn.go:207`, `acpserver/serve.go:71` and `acpclient/acptest/pair.go:65`. This SDK's own `acptest.Connect` cleanup
  waits with no bound (`acptest/pair.go:34`).
* **Knowing why a connection or an agent ended.**
  * `<cli-b>`'s `acpserver/serve.go:58`–`:66` takes `io.EOF` and `ErrConnectionClosed` as a normal end. That also takes the
    overflow policy's close, which wraps `ErrConnectionClosed`, as normal.
  * magic-cli-remote waits a fixed `exitPreference = 500 * time.Millisecond` (`session.go:1304`) so that the agent's exit status
    wins over the connection's end. Its 0207 PLAN saw the connection end first in 28 of 30 runs (0035 E4).
* **The rest of a test harness.** `<cli-b>`'s 0016 MADR D6 asks for `ValidateGolden`. Its harness records each frame's
  direction, waits for a number of frames, and scripts terminals and error turns (`acpclient/acptest/recorder.go:101`–`:191`,
  `client.go:154`–`:284`). This SDK's `acptest` does none of these, and its `Connect` cannot attach a `Recorder`.
* **Checking a value before it is sent.** magic-cli-remote drops MCP servers and image blocks the agent did not advertise before it
  sends them (`acpagent.go:635`–`:644`, `session.go:566`–`:569`), by code of its own, because the gates are unexported (0035 C6).
* **A tap's frames.** magic-cli-remote splits `agentproc`'s output tap into lines itself (`internal/wirecap/wirecap.go:165`–`:207`).
* **Variant names for the remaining unions.** magic-cli-remote re-parses the plan-update content union's `Unknown` for its type
  (`plans.go:73`). Of the 17 generated unions, 6 have `Kind()`. Six stable ones do not: `AuthMethod`, `CreateElicitationRequest`,
  `CreateElicitationResponse`, `McpServer`, `MultiSelectItems` and `SetSessionConfigOptionRequest`. Nor do five unstable-origin
  ones, among them `PlanUpdateContent` (counted by a script over `types_gen.go`).
* **Tool-call state.** `<cli-b>`'s CLI merges `tool_call` and `tool_call_update` by hand (`internal/cli/view.go:146`–`:165`). The
  schema states the rules: `ToolCallUpdate` is "An update to an existing tool call"; "All fields except the tool call ID are
  optional - only changed fields need to be included"; its `content` and `locations` each "Replace the … collection". The
  v1 tool-calls page adds that omission and `null` leave a `name` unchanged, and the raw values too, and is silent on an update for
  an unknown id (<https://agentclientprotocol.com/protocol/v1/tool-calls.md>).

### The `TestDetachedTapIsPrefix` failure

The tag's CI run `40516219662` failed once, on macOS at Go 1.27.3: `tap_test.go:335: the tap received 33554432 bytes and dropped
33488896, 67043328 in all; the agent wrote 67108864`. 65,536 bytes, a macOS pipe buffer's worth, were neither received nor counted.

The evidence points at `agentproc`, not the test:

* The test calls `Close` (`tap_test.go:326`), and `Close` returns `Wait()` (`agentproc.go:362`–`:365`). `DroppedTapBytes` "is
  final once Wait has returned" (`:341`–`:345`). So the test reads final counts, and a race in its reading is excluded.
* The bytes must then never have been read from the pipe, since every byte read is offered to the tap and either queued or counted
  (`tap.go:47`–`:58`, `:137`–`:146`).
* The watcher, once `Cmd.Wait` returns, closes `Done` and then waits up to the grace for the connection's own end before it
  closes the connection (`agentproc.go:259`–`:273`), as `0027-PLAN-agent-process-helper.md`'s "The watcher", step 4, designed.
  But `Close`'s steps, `stop` (`:383`–`:395`), see `Done` closed (`exitedWithin`) and close the connection at once. That cuts the
  watcher's wait short and leaves what the agent wrote into the pipe buffer just before it exited, up to 64 KiB on macOS, unread
  when the reader is closed.
* That contradicts `stop`'s own comment, "the connection keeps reading what the agent writes before it exits", and the README's
  "so nothing the agent writes before it exits is lost". It also loses the agent's last JSON-RPC lines, not only a tap's copy.

It is rare: no failure in 40 runs of `main`, and 60 local runs passed. The PLAN confirms the cause before it changes anything.

### The question

Which ten units of work close the gaps the two consumers wait on, within the `v1` promise and this repository's rules, and in which
shape?

## Decision Drivers

* **What a consumer waits on first.** An item that a consumer's record or code names as waiting ranks above one only this
  repository's records defer.
* **Protocol-generic, deterministic, self-contained**, 0035's test: the specification or this SDK's own rule fixes the behaviour;
  the same input gives the same result; no new dependency.
* **Correctness before features.** A defect against this SDK's own documentation, the lost output, comes first.
* **Compatibility.** Additions only under `make api`; a decided record is reopened only by the owner's answer.
* **No product policy.** Root confinement, the phone's permission flow, chunk coalescing and the consumers' `_` methods stay
  theirs (0035 F).
* **Adoption is the consumers'.** This repository cannot change their pins; it can make the mapping from their code to its API
  explicit.

## Considered Options

* **O1.** Ten items, under one record and one PLAN of phases, the defect first:
  1. `agentproc` reads the agent's output to its end before it closes the connection;
  2. a terminal host;
  3. a file host, with `IsResourceNotFound`;
  4. `Negotiation.AllowsValue`;
  5. `agentproc.LineTap`;
  6. a bounded wait;
  7. end causes: a clean-end classifier, and the agent's exit order;
  8. `acptest` for what `<cli-b>`'s harness does;
  9. a tool-call timeline;
  10. `Kind()` on the remaining unions.
* **O2.** The same ten, as four records: `agentproc` (1, 5, 7's exit order), the client hosts (2, 3), the core helpers (4, 6, 7's
  classifier, 9, 10), `acptest` (8).
* **O3.** Only what a consumer waits on in its own records: 1, 2, 3, 6 and 8.
* **O4.** No new API: an adoption report mapping each consumer site to the API that exists, and the defect fix.
* **O5.** Nothing further.

## Decision Outcome

Chosen option: "O1", because the ten are the open units the consumers' code and records point at, each is protocol-generic or this
SDK's own defect, and one record keeps the ordering, the defect first, and one release. O2 has the same content in four records,
with four sets of release work, and the items share files (`agentproc`, `acptest`, `helpers.go`). O3 leaves magic-cli-remote's
value check, tap framing and union names to be copied again. O4 fixes the defect and maps the adoption but leaves both consumers
writing the hosts 0035 found they need. O5 leaves the lost output in `agentproc`.

The adoption report O4 describes is in O1 too, as a phase: it decides nothing, and it gives each consumer the list for its own
records.

### The ten items

Each new exported name has a doc comment that opens with its name and cites this record. No existing signature changes.

#### 1. `agentproc` reads the agent's output to its end before closing the connection

**Fix.**
* `Close`'s steps, once the agent has exited, wait up to the grace for the connection's own end, the end of the agent's output,
  before they close the connection, as the watcher already does (`agentproc.go:266`–`:272`). `Kill` keeps closing at once.
* The PLAN first reproduces the loss. It holds the connection's reader back for a moment in a scratch copy, and makes the test fail
  every time, as 0039's `Recorder` deviation did. It changes nothing until the reproduction shows the bytes unread. (Amended
  2026-10-27, "Amended by the PLAN's phase 2": the test failed in 37 of 40 runs, not every time, with the CI run's exact counts.)

**Tests.** `TestDetachedTapIsPrefix`, unchanged, as the regression test. A new `TestCloseReadsTheAgentsLastOutput`: an agent writes
its last lines and exits at once, and every line reaches the connection.

#### 2. A terminal host

**Add.** `clienthost.Terminals`, in a new package `clienthost` (question 1). It holds the five terminal methods of `acp.Client`
(`CreateTerminal`, `TerminalOutput`, `WaitForTerminalExit`, `KillTerminal`, `ReleaseTerminal`), so that a client embeds it:
* `outputByteLimit` kept by dropping from the front at a character boundary, and `truncated` reported;
* the exit status mapped to `exitCode` or `signal`;
* kill and release ending the process tree, with `agentproc`'s tree code, moved to an internal package both use;
* a caller-supplied function that builds the `*exec.Cmd` from the request, for each consumer's shell routing;
* `Close`, ending every terminal.

An unknown terminal id is `NewResourceNotFound`.

#### 3. A file host

**Add.** `clienthost.Files`, embeddable, with `ReadTextFile` and `WriteTextFile`:
* 1-based `line` and `limit`, as the schema defines them;
* a missing file answered `NewResourceNotFound(path)`, `-32002`;
* a write that creates the file and, by an option, its parent directories.

Confining paths to a session's roots stays the consumer's (0035 C5); `Roots()` gives them. In the root package,
`IsResourceNotFound(err)` beside `IsMethodNotFound`, for an agent reading a client's answer (`<cli-b>`'s `acpserver/clientfs.go`).

#### 4. `Negotiation.AllowsValue`

**Add.** `func (n Negotiation) AllowsValue(method string, params any) error`, hand-written over the generated value gates. It
returns nil, or the `*CapabilityError` a send of `params` would get, so a caller can drop an image block or an MCP server before
sending. `Allows(method)` stays as it is.

#### 5. `agentproc.LineTap`

**Add.** `func LineTap(fn func(frame []byte)) io.Writer`, a writer that calls `fn` with each newline-ended line it is given, a
copy, and holds an unended line until its end, for `Command.OutputTap` and `InputTap`.

#### 6. A bounded wait

**Add** (question 2, which reopens 0026's question 4). `WaitContext(ctx) error` on `Connection`, ~~both side connections~~ and
`agentproc.Process` (amended 2026-10-27, "Amended by the PLAN's phase 6": a side connection's is its `Connection()`'s). It returns `Wait()`'s error, or `context.Cause(ctx)` when ctx ends first. It starts at most one goroutine per
connection, which ends with the connection's goroutines, so it leaves none behind. `acptest.Connect`'s cleanup then waits for a
bound of its own and fails the test when a handler outlives it.

#### 7. End causes

**Add.**
* `func EndedCleanly(err error) bool` in the root package. It is true for an `Err()` of `ErrConnectionClosed` from `Close`, or of
  `ErrPeerDisconnected` wrapping `io.EOF`. It is false for the overflow policy's close, a read error, and nil.
* `func (p *Process) ExitedFirst() bool` in `agentproc` (question 3, which amends 0027). It is true when the agent's exit was seen
  before the connection's end, final once `Wait` has returned, so a client reports the exit status without waiting on a timer.

#### 8. `acptest` for what `<cli-b>`'s harness does

**Add.**
* `ConnectRecorded(tb, agent, client, rec, opts...)`, which records both directions into `rec`.
* `Recorder` frames carry their direction; `Recorder.Wait(ctx, n)` waits for `n` frames.
* `ValidateGolden(tb, frames)`: each frame decoded into its method's type, a response by its request's, then `Validate`, failing
  on an error or an unknown method (`<cli-b>`'s 0016 D6).
* `ScriptAgent` turns that answer an error, report `Usage`, or wait for `session/cancel`.
* `MemoryClient` answering elicitations from a script, through 0040's helpers, and terminals through item 2's `Terminals` with a
  scripted command builder.

The canonical form stays `encoding/json` v1, as 0039 decided. `<cli-b>`'s RFC 8785 transcript and its goldens' file layout stay
its own.

#### 9. A tool-call timeline

**Add.** `ToolCallTimeline`, beside `CompactionTimeline`, for a client, live or replayed:
* `Apply(SessionUpdate)` takes a `tool_call` as the entity, and a `tool_call_update` replacing each field it carries;
* `content` and `locations` are replaced, never appended, as the schema says;
* `Get(id)` and `Open()`, the calls not `completed` or `failed`.

An update for an id never seen starts the call with status `pending` (question 4). Each entity is a copy. It is not a session
reducer: like `CompactionTimeline`, it keeps one entity type whose rules the schema states (0039 item 3).

#### 10. `Kind()` on the remaining unions

**Add.** `Kind()` on the six stable unions without it, `AuthMethod`, `CreateElicitationRequest`, `CreateElicitationResponse`,
`McpServer`, `MultiSelectItems` and `SetSessionConfigOptionRequest`, as 0038 added it to four. On the unstable-origin
`PlanUpdateContent` too (question 5): magic-cli-remote re-parses it. A method on a frozen type is additive, and the type stays
deprecated.

#### And the adoption report

`0041-REPORT-consumer-adoption-of-v1-21-to-v1-25.md` lists, for each consumer, every site the investigation found whose code
`v1.21.0` to this record's release replaces, the API that replaces it, and the consumer records that are stale against this
repository. It decides nothing; each consumer adopts under its own records.

### Questions for the owner

1. **Where the client hosts live (items 2, 3).**
   * Recommended: a new package, `clienthost`, holding `Terminals` and `Files`, both embeddable in an `acp.Client`, so that a
     client embeds them with `ClientRefusals` for the rest. `agentproc`'s tree code moves to an internal package both use. It is a
     new import path that `make api` gates, and the root package keeps no file or process access.
   * Alternative: 0035's shapes, `agentproc.TerminalHost` and `ReadTextFileOS` / `WriteTextFileOS` in the root package. No new
     package, but the root gains `os` file access, and the two do not embed together.
   * Alternative: both in `agentproc`, which already holds the process code. One package, but `agentproc` is about the agent's
     process, not the client's host.
2. **A bounded wait (item 6), reopening 0026's question 4.**
   * Recommended: `WaitContext(ctx)`, as item 6 states. 0026 rejected `Wait(ctx)` because "a caller that needs a bound runs it on
     a goroutine and selects". `<cli-b>`'s three copies are that pattern, and this SDK's own `acptest` needs it too.
   * Alternative: no new method; `acptest.Connect`'s cleanup alone gains a bound. 0026 stands.
   * Alternative: leave both as they are.
3. **The agent's exit order (item 7), amending 0027.**
   * Recommended: `Process.ExitedFirst()`, as item 7 states.
   * Alternative: `Process.Wait`'s error also wraps the connection's cause when the connection ended first. That changes an
     existing method's result, which is compatible under `apidiff`, but not under every caller's `errors.Is`.
   * Alternative: not now; E4 stays open.
4. **A `tool_call_update` for an id never seen (item 9).**
   * Recommended: it starts the call, status `pending`, its fields from the update, as `CompactionTimeline` starts a compaction
     first seen by its chunk.
   * Alternative: it is ignored, and `Apply` returns false; the schema says an update is for "an existing tool call".
5. **`Kind()` on the unstable-origin unions (item 10).**
   * Recommended: on `PlanUpdateContent`, which a consumer needs, and on none of the others.
   * Alternative: on every unstable-origin union too.
   * Alternative: the six stable ones only.
6. **The PLAN's releases (the PLAN's phase 17).**
   * Recommended: one minor at the end.
   * Alternative: a patch at once for item 1, the defect, then one minor for the rest.

#### Decided with the owner, 2026-10-27

| Question | Answer |
| :--- | :--- |
| 1. Where the client hosts live | As recommended: a new package, `clienthost`, with `agentproc`'s tree code in `internal/proctree`. |
| 2. A bounded wait | As recommended: `WaitContext(ctx)`, which reopens 0026's question 4. |
| 3. The agent's exit order | As recommended: `Process.ExitedFirst()`, which amends 0027. |
| 4. An update for an unseen id | As recommended: it starts the call, status `pending`. |
| 5. `Kind()` on unstable-origin unions | As recommended: on `PlanUpdateContent` only. |
| 6. The releases | As recommended: one minor at the end. |

[0026-MADR-connection-end-cause-and-wait.md](0026-MADR-connection-end-cause-and-wait.md) and
[0027-MADR-agent-process-helper.md](0027-MADR-agent-process-helper.md) each carry an amendment that points here.

#### Amended by the PLAN's phase 2, 2026-10-27

Item 1's cause stands, and the owner accepted the reproduction as confirming it. Item 1 said the reproduction would make the test fail
every time. In a scratch copy of `8d30c43`, on macOS, 20 runs each:

* **A single 50 ms pause** before the connection's first read after the agent's exit fired in 9 runs, and no run failed. A reader
  that keeps up is already blocked in `Read` when the agent exits, and gets the end of input with nothing left in the pipe.
* **A 5 ms pause before every read, from the start of `Close`'s steps,** keeps the reader behind the agent. The test failed in 18 and
  then 19 of 20 runs, every failure with the CI run's line: received 33554432, dropped 33488896, 65536 bytes neither received nor
  counted. In the runs that passed, the reader caught up before `stop` closed the pipe.
* **The same slow reader, with item 1's fix,** passed all 20 runs.

The loss needs a reader behind the agent when the agent exits, as under the race detector in CI. So phase 3's new test holds the
reader back through a field of `Command` that is unexported and set only by the package's tests.

#### Amended by the PLAN's phase 6, 2026-10-27

The owner decided two points that item 6 did not foresee.

* **No `WaitContext` on the side connections.** A method of `AgentSideConnection` or `ClientSideConnection` that takes a context
  is a protocol method, by the rule of
  [0038-MADR-consumer-helpers-kind-and-agent-refusals.md](0038-MADR-consumer-helpers-kind-and-agent-refusals.md)'s amendment,
  which `side_methods_test.go` enforces. Three tests treat each such method as a wire call: `TestTypedMethodDocsNameErrors`,
  `TestNotInitialized` and `TestConcurrentUseOfOneConnection`. A side connection's `Err` and `Wait` are its connection's, so a
  caller writes `side.Connection().WaitContext(ctx)`. `Connection` and `agentproc.Process` keep the method.
* **One goroutine that `Wait` does not count.** `TestGoStatementsCounted` refused any `go` statement in the root package, so that
  `Wait` could wait for every goroutine. `WaitContext`'s goroutine runs `Wait`, so it cannot be one of them. The test now allows
  exactly one `go` statement, in `connection.go`'s `waitDone`, and still fails on any other, or on none there.

#### Amended by the PLAN's phase 8, 2026-10-27

* **`clienthost.Refusals`, decided by the owner.** Item 2 and question 1 said a client embeds `Terminals` "with `ClientRefusals`
  for the rest". Both declare the five terminal methods, and Go promotes neither of two methods of one name at one depth: a
  scratch program fails with "ambiguous selector". `Files`, in item 3, has the same clash. `clienthost.Refusals` is
  `struct{ acp.ClientRefusals }`, whose methods are a level deeper, so `struct{ clienthost.Terminals; clienthost.Refusals }`
  serves Terminals' methods and refuses the rest.
* **Three names the phase added, which item 2 did not list:**
  * `DirectCommand`, the `Command` a nil field means. It runs the request's command without a shell, with this process's
    environment and the request's env after it, and refuses a relative cwd.
  * `MaxOutputBytes`, which bounds what a terminal keeps whatever the agent's `outputByteLimit`.
  * `DefaultMaxOutputBytes`, 16 MiB, its zero value's meaning, the clamp magic-cli-remote applies.
* **Two behaviours item 2 did not state:**
  * While a command runs, `terminal/output` holds back a character the command has not finished writing, so the output is
    valid UTF-8 at both ends.
  * When a command exits of itself, what is left of its process group or job is ended then, as `agentproc` ends an agent's. That
    also keeps kill and release from signalling a group whose ID could have been reused.
* **No death signal on a terminal.** A `Pdeathsig` follows the thread that started the process (0029), and a handler's thread is
  not kept for the terminal's life. `Close` ends every terminal, and on Windows a terminal's job ends with this process.

#### Amended by the PLAN's phase 9, 2026-10-27

Item 3 named `ReadTextFile` and `WriteTextFile` with 1-based `line` and `limit`, `-32002` for a missing file, and an option for
parent directories. The phase also decided the following, which item 3 did not state. The specification's file-system page names
no error codes, and says nothing of parent directories or of a line past the end.

* **No policy hook.** The specification asks a client to check each path against the session's roots, and to ask its user before
  a write changes a file that controls execution or approval. A client does that in a `ReadTextFile` or `WriteTextFile` of its
  own, which wins over the embedded one, and calls `Files`'. `ExampleFiles` shows it. Unlike `Terminals`, which needs `Command`
  to build the process, `Files` needs no field for it.
* **Refusals beyond a missing file:**
  * a path that is not absolute is invalid params;
  * `line` 0 is invalid params, since lines are numbered from 1;
  * a path that is not a regular file is refused. A device such as `/dev/zero` would never finish a read, and a FIFO would block
    it.
* **A write without `MkdirAll` under a missing directory** is resource-not-found, naming the directory.
* **A write replaces the file's content and keeps its mode.** It is not an atomic rename, which would change the file's inode and
  ownership.

### Consequences

* Good, because the defect that loses an agent's last output before it exits is fixed, with the failing test as its regression
  test.
* Good, because a client gets a file system and terminals that follow the schema's MUSTs by embedding two types, which `<cli-b>`'s
  records wait for and magic-cli-remote's terminal host breaks.
* Good, because the consumers' hand-written waits, end checks, exit timers, tap splitters, value pre-checks and harness pieces each
  get one implementation here.
* Good, because the adoption report gives each consumer its list, and the stale records it should correct.
* Neutral, because neither consumer gains anything until it moves off `v1.20.0`, which is its own work.
* Bad, because the API grows by a new package and about twenty names, each a `v1` promise, and `make api` gates one more package.
* Bad, because moving `agentproc`'s tree code to an internal package touches tested Windows and Unix code, so the PLAN runs every
  platform's tests and the hosts' gates on it.
* Bad, because two items reopen or amend decided records. The owner answered questions 2 and 3 so on 2026-10-27, and both records
  carry an amendment that points here.

### Confirmation

A later PLAN shows each of the following, every new check first seen to fail on a planted break in a scratch copy:

* the lost output reproduced in a scratch copy before the fix, and not after it; `TestDetachedTapIsPrefix` and the new test pass
  many times on every host;
* `Terminals` truncating at a character boundary, with multi-byte output across the limit; exit codes and signals per platform;
  kill and release ending a tree; an unknown id `-32002`;
* `Files` with `line` and `limit` at their edges, a missing file `-32002`, and parent directories created only by the option;
  `IsResourceNotFound` on a peer's `-32002`;
* `AllowsValue` agreeing with what a send refuses, for every value gate the protocol table lists;
* `LineTap` across split and joined writes, copies not aliases, and an unended line held;
* `WaitContext` returning the cause when ctx ends and `Wait`'s error otherwise, leaving no goroutine behind under the leak guard;
* `EndedCleanly` for each end `Err()` reports; `ExitedFirst` both ways, by a scripted agent;
* `acptest`'s additions, `ValidateGolden` refusing a frame that breaks the schema;
* `ToolCallTimeline`'s rules, and a replay equal to the live sequence;
* `Kind()` on each named union, and a catch-all's or unknown variant's name;
* the adoption report's sites read again in both consumers;
* `make api` listing only compatible changes, and the new package as new.

## Pros and Cons of the Options

### O1. Ten items, one record and one PLAN

* Good, because the defect comes first, and every item a consumer waits on is in it.
* Good, because shared files change under one PLAN, and one release carries them.
* Bad, because it is large, which the PLAN sequences into phases, each its own commit.

### O2. The same ten, in four records

* Good, because each record is smaller.
* Bad, because the items share `agentproc`, `acptest` and `helpers.go`, so the records would order each other, with four sets of
  release work.

### O3. Only what a consumer's records wait on

* Good, because it is smaller.
* Bad, because magic-cli-remote's value check, tap splitter and union re-parsing stay copied, though its code shows them.

### O4. An adoption report and the defect fix only

* Good, because it adds no API.
* Bad, because both consumers keep hosts that break or lack the schema's rules.

### O5. Nothing

* Good, because it costs nothing.
* Bad, because `agentproc` keeps losing an agent's last output.

## More Information

### Not chosen, and why

* **An in-process agent runner** (`<cli-b>`'s `acpclient.Start`) and **an exported process-tree primitive** (its
  `internal/proctree`): 0035 F, "`ServeAgent` and an in-process `agentproc`", not recommended; the tree primitive is not ACP's.
* **Session-state reducers** beyond tool calls, **routing updates by session**, **chunk coalescing**: 0035 F, each consumer's
  policy.
* **A URL elicitation trust policy:** `ParseElicitationURL` is the generic part; the policy is magic-cli-remote's 0191's own
  record.
* **The refuse code (E2)** and **`session/close` waiting for its turns (E3)**: E2 reopens 0021, and neither consumer's records
  wait on E3.
* **`RequestError.Reason` reading a `reason` key:** `<cli-b>`'s own agent writes it; this SDK writes `error`.
* **Schema `1.26.0`:** deferred by the owner.

### Sources

* v1 tool calls: <https://agentclientprotocol.com/protocol/v1/tool-calls.md>
* The schema's `ToolCallUpdate`, `CreateTerminalRequest`, `TerminalOutputResponse`, `TerminalExitStatus`, `ReadTextFileRequest`
  and `WriteTextFileRequest` descriptions, `schema/schema.json`
* CI run `40516219662`, attempt 1, job `122230708180`

### Tree citations

* `agentproc/agentproc.go:259`–`:273` (`watch`), `:341`–`:345` (`DroppedTapBytes`), `:362`–`:365` (`Close`), `:383`–`:395`
  (`stop`);
  `agentproc/tap.go:47`–`:58`, `:137`–`:146`; `agentproc/tap_test.go:308`–`:337`
* `types_gen.go:4126`, the terminal truncation MUST
* `acptest/pair.go:34`, `Connect`'s unbounded cleanup
* `errors.go:310`, `:321`, `:330`: `IsMethodNotFound`, `IsAuthRequired`, `IsCancelled`; no `IsResourceNotFound`

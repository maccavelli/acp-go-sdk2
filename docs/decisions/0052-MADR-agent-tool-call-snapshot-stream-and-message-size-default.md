---
status: accepted
date: 2026-11-10
decision-makers: the repository owner
consulted: "this tree at `1e945dd` (`v1.34.2` plus one docs commit); `schema/schema.json` at ACP 1.25.0; the official TypeScript (`08a0d30`) and Python (`9853d8b`) SDKs' clones in the interop cache; gobble-cli at `9eb22bc`, read as data"
informed: gobble-cli, magic-cli-remote
---

# Stream an agent's tool call as coalesced full snapshots, and raise the default message size to 32 MiB

## Context and Problem Statement

[0051-REPORT-acp-1-25-0-featureset-coverage-and-three-extensions.md](../reports/0051-REPORT-acp-1-25-0-featureset-coverage-and-three-extensions.md)
item 1 found that the SDK helps a client *apply* `tool_call_update` snapshots (`ToolCallTimeline`) but gives an agent nothing
to *produce* them: every agent that shows a tool's output while it runs writes its own accumulator, its own throttle and its
own final send. The owner asked for that helper, for an assessment of the constraints the first reading of the report named,
and for a review of the 10 MiB default message size in the same record.

The helper is to be agent-neutral and idempotent: built on the schema's terms only, usable by any agent, and fully used by
gobble-cli through its standard surface, not shaped to it.

### Findings

Each finding is marked **Read** (code or text read at the cited lines) or **Probe** (not used in this record; no probe ran).

#### The schema

* **F1. Replace-whole, no delta.** Read. `$defs.ToolCallUpdate`: "All fields except the tool call ID are optional - only changed
  fields need to be included." `content`: "Replace the content collection." `locations`: "Replace the locations collection."
  `rawInput` / `rawOutput`: "Update the raw input/output", with no partial form. No ACP 1.25.0 type appends.
* **F2. A terminal is embedded, not streamed.** Read. `ToolCallContent`'s `terminal` variant embeds "a terminal created with
  `terminal/create` by its id. The terminal must be added before calling `terminal/release`." The client shows its output; the
  agent sends no output for it.
* **F3. Four statuses, none `cancelled`.** Read. `ToolCallStatus`: `pending` ("input is either streaming or we're awaiting
  approval"), `in_progress`, `completed`, `failed`. The client, not the agent, marks a call cancelled (0017 V6, "Marking tool
  calls is UI state the SDK does not hold").

#### This SDK

* **F4. Constructors, no producer.** Read. `StartToolCall` / `UpdateToolCall` (`helpers.go:796`, `:897`) each build one
  notification. `ToolCallTimeline.Apply` (`tooltimeline.go:57`) applies them on the client side and sends nothing.
  `ToolCallEntity` (`tooltimeline.go:12`) is the call's state as the schema defines it.
* **F5. A cancelled prompt's context cannot send.** Read. `session/cancel` cancels the context `Prompt` runs on
  (`agent_gen.go:627`, `:757`). `SendNotification` returns before writing when its context is done (`connection.go:3086-3090`,
  and `notifyDirect` at `:3105-3109`). An agent that sends a call's last status on the `Prompt` context after a cancel sends
  nothing, while the prompt turn's specification asks for pending updates to be sent before the `cancelled` answer.
* **F6. Goroutines are allowed, if counted.** Read. `TestGoStatementsCounted` (`connection_end_test.go:456`) fails on a bare `go`
  statement with "start it with the connection's goroutines.Go". The rule is
  [0026-MADR-connection-end-cause-and-wait.md](0026-MADR-connection-end-cause-and-wait.md)'s: `Wait` returns only when no goroutine
  of the connection runs. [0041-MADR-close-the-gaps-the-consumers-wait-on.md](0041-MADR-close-the-gaps-the-consumers-wait-on.md)
  allowed one uncounted `go`, `WaitContext`'s. The counted starts are at `connection.go:316`, `:328`, `:1016`-`:1020`, `:1652`,
  `:1864` and `:2171`. 0026's PLAN states that each starts from `start` or from a goroutine already counted, as
  `sync.WaitGroup` requires of an `Add` that can meet a zero counter, and
  [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md) lists
  "`goroutines.Go` called only from counted goroutines" among the invariants it checked. This record did not re-check each site.
* **F7. A side-connection method that takes a context is a protocol method.** Read. The rule of
  [0038-MADR-consumer-helpers-kind-and-agent-refusals.md](0038-MADR-consumer-helpers-kind-and-agent-refusals.md)'s amendment,
  restated in 0041's phase-6 amendment and enforced by `side_methods_test.go`. A helper that sends is therefore a package
  function or a type of its own, not a method of `AgentSideConnection`.
* **F8. The wire has one writer; the work before it is the caller's.** Read. `AgentSideConnection.SessionUpdate`
  (`agent_gen.go:1002`-`:1007`) runs the capability gate, then `SendNotification`. `prepareNotification` (`connection.go:3216`)
  calls `encodeParams` (`:2814`), which runs `validateOutbound` (`:2818`) and `json.Marshal` (`:2821`); `sendMessageSent`
  marshals the envelope (`:2484`). All of it runs on the caller's goroutine. The bytes then go to `writeLoop`
  (`:1041`-`:1051`), the one goroutine that writes. Parallel senders add no wire throughput; they gate, validate and encode in
  parallel.
* **F8a. A send returns once its bytes are written.** Read. `c.writes` is unbuffered (`connection.go:967`): `sendMessageSent`
  hands the request to `writeLoop` or returns at the connection's end or its context's (`:2489`-`:2495`), then waits for the
  `Write`'s result (`:2496`-`:2507`), which `writeLoop` always reports (`:1047`-`:1048`). So a send on a context that never ends
  returns when the bytes are written, or at the hand-off when the connection has ended. The one exception is a `Write` blocked
  on a writer `Close` cannot close, which runs until it returns
  ([0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md), R10).
* **F8b. `writeLoop` ends only with the connection.** Read. Its loop returns on `c.ctx.Done()` alone (`connection.go:1044`-`:1045`),
  and `Wait`'s comment states it (`:1132`-`:1135`). It has no defer today.
* **F8c. Nothing recovers a handler's panic.** Read. Neither `connection.go` nor `agent_gen.go` calls `recover()`. A `Prompt`
  that panics ends the process; no step after it runs.
* **F8d. A cancelled turn's cause.** Read. `session/cancel` cancels the turn with `ErrPromptCancelled` (`connection.go:300`), a
  newer prompt under supersede with `errSuperseded`, which wraps it (`agent_gen.go:161`).

#### The 10 MiB default

* **F9. ACP sets no message size.** Read. [0016-REPORT-acp-v1-conformance-audit.md](../reports/0016-REPORT-acp-v1-conformance-audit.md):
  "The 10 MiB inbound line cap is an SDK addition; the spec states no limit." [0001-REPORT-go-127-acp-parity.md](../reports/0001-REPORT-go-127-acp-parity.md)
  says the same of the stdio section.
* **F10. No record chose 10 MiB.** Read. The value is in the initial commit (`fe457c7`) as the `bufio.Scanner` cap.
  [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md) made it
  `WithMaxMessageSize` and kept the value. [0043-MADR-remote-http-and-websocket-transport.md](0043-MADR-remote-http-and-websocket-transport.md)
  copied it to `remote` (`remote/client.go:13`, "the connection's own default"). [0045-MADR-notification-queue-byte-budget.md](0045-MADR-notification-queue-byte-budget.md)
  D4 raises a byte budget below one message to it.
* **F11. It is a read limit.** Read. `defaultMaxMessageSize` (`connection.go:39`) bounds what *this* SDK reads; an overlong line is
  read through its newline and answered `-32600` with `"id": null` (0017; 0049 P6, P9 for `remote`). It does not bound what
  this SDK sends. A peer built on another SDK applies its own.
* **F12. The official SDKs read more.** Read.
  * TypeScript `08a0d30`: `DEFAULT_MAX_MESSAGE_BYTES = 32 * 1024 * 1024` (`src/core/stream-limits.ts:1`), taken by `ndJsonStream`
    (`src/core/stream.ts:50`). A longer line throws `MessageTooLargeError` (`src/core/line-buffer.ts:121`-`:123`), which errors
    the stream (`src/core/stream.ts:126`-`:131`): the connection ends.
  * Python `9853d8b`: `DEFAULT_STDIO_BUFFER_LIMIT_BYTES = 50 * 1024 * 1024` (`src/acp/core.py:36`). `_read_line` catches
    `LimitOverrunError`, reads what was consumed and continues (`src/acp/_transport.py:88`-`:101`), so on that path the buffer
    limit is not a message limit.
  * So a message between 10 and 32 MiB that a TypeScript peer accepts, this SDK refuses, and the message is lost.
* **F13. What depends on the value.** Read. `WithMaxQueuedBytes`' 64 MiB default is at least one message (0045 D4); `remote`'s
  `MaxBuffered`, 64 MiB, is at least `MaxMessageSize` (`README.md`, "Remote agents"). Both still hold at 32 MiB. The
  README, `docs/architecture.md`, `remote`'s docs and the tests that send "a line just over 10 MiB" name the value.

#### The official SDKs' agent-side helpers

* **F14. Python has a tracker that does not send.** Read. `acp.contrib.tool_calls.ToolCallTracker` (`src/acp/contrib/tool_calls.py`)
  keeps each call's fields and returns update models; `append_stream_text` (`:179`-`:188`) appends to a buffer and returns a
  `tool_call_update` whose `content` is the whole buffer as one text block. It sends nothing, does not coalesce, bound or detach
  from cancellation, and holds no lock. In TypeScript `08a0d30`, a search of the non-generated sources (files whose names hold
  neither `gen`, `schema` nor `types`) found `tool_call_update` only in `src/examples/client.ts:59`: no agent-side tool-call
  helper.

#### gobble-cli at `9eb22bc`

Read by a delegated read-only pass and spot-checked at `acpserver/prompt.go:53`, `:115`-`:132`, `:159`-`:165` and
`acpserver/serve.go:45`-`:47`.

* **F15. No producer yet.** Each call sends a `tool_call` (`pending`), an `in_progress` update with no content, and one final
  update with status and the whole content (`acpserver/prompt.go:96`-`:132`). The final status goes alone in its update "with the
  content" (`:121`-`:122`, its 0008-MADR D19 item 2), and the first content item is a summary of at most 400 bytes (`:205`, D19
  item 3). Local shell output is kept in a spill writer with no hook and sent once, at the end (`tool/builtin/shell.go:82`-`:122`,
  `output.go:23`-`:67`). Its planned `task` subagent tool would relay progress as `tool_call_update`, possibly for several calls
  at once (its 0005-MADR `:128`, `:516`).
* **F16. It already detaches sends.** `newUpdates` sends on `context.WithoutCancel(ctx)` (`prompt.go:53`), so a call cut by
  `session/cancel` still reaches `failed`. It coalesces text and thought chunks at 50 ms or 4 KiB, and flushes them before every
  tool event so that the wire order is the event order (`prompt.go:33`-`:35`, `coalesce.go:10`-`:15`).
* **F17. It restates replaced collections.** The client-terminal reference is sent alone (`prompt.go:164`) and put first again in
  the final content (`:125`-`:127`), as replace-whole requires.
* **F18. Its tools run one at a time,** on the `Prompt` goroutine (`agent/agent.go:741`-`:797`); it serves with
  `PromptOverlapConcurrent` and no size or queue option (`serve.go:45`-`:47`).
* **F19. Its client renders each update anew.** `internal/cli/view.go:282`-`:304` writes the whole content on every update, so
  many snapshots of one call would print many times. That is gobble-cli's to change, by `ToolCallTimeline` or a redraw.

### The question

What should the SDK give an agent so that a tool call's live state reaches the client correctly, cheaply and in order, under
cancellation, back-pressure and concurrent tools, without a wire form ACP 1.25.0 lacks? And should the default message size stay
10 MiB?

## Decision Drivers

* **Schema fidelity.** Only `tool_call` and replace-whole `tool_call_update` (F1). No append, no delta, no `_meta` side channel.
* **Idempotence.** A snapshot sent twice changes nothing; a snapshot skipped loses nothing, since the next carries it. Correctness
  must not depend on every intermediate update arriving: a client may drop (`OverflowDropNewest`, which magic-cli-remote's SDK
  fork uses, as gobble-cli's 0008-MADR `:116` records) or merge (latest wins).
* **Agent neutrality.** Schema types only. Content order, summaries and titles stay the caller's.
* **Bounded cost.** Full snapshots resend everything, so cost is size × count. Both must be bounded by default.
* **Parallel tools.** Many calls at once, none blocked on the wire by another, none blocked by a slow client.
* **Order and completion.** Snapshots of one call never regress; the last state, with its final status, is written before the
  caller goes on and before the turn's answer, also after `session/cancel` (F5).
* **The connection's lifetime rules** (F6, F7): counted goroutines, `Wait` that waits for them, no context-taking side method.
* **Interoperability of the read limit** with the official SDKs (F12), at a memory cost a host can carry.

## Considered Options

* **O1. A sending stream in root `acp`:** a concurrency-safe `ToolCallStream` holding a `ToolCallEntity`, one counted sender
  goroutine per active stream that sends the latest snapshot at most once per interval, a bounded live text, and `Flush` /
  `Finish` that write synchronously and survive cancellation.
* **O2. A snapshot builder in root `acp` that does not send,** as Python's `ToolCallTracker` (F14): the agent sends each update
  itself.
* **O3. O1 in a subpackage** (`acp/toolstream` or similar), outside the root package's goroutine guard.
* **O4. Nothing in the SDK;** document the pattern in `README.md`.

For the message size:

* **M1. Raise the default to 32 MiB,** TypeScript's, on the connection and in `remote`.
* **M2. Keep 10 MiB.**
* **M3. Raise it to 50 MiB,** Python's buffer figure.
* **M4. No default limit.**

## Decision Outcome

Chosen options: **"O1. A sending stream in root `acp`"** and **"M1. Raise the default to 32 MiB"**, because O1 is the
only option that makes the robust behaviour (coalescing, bounds, order, cancellation-safe completion) the default instead of each
agent's homework, inside the guard that keeps `Wait` exact; and M1 makes this SDK read what the reference SDK reads, for a
bounded memory cost.

### D1. `ToolCallStream`, in root `acp`, for agents

**Add**, beside `StartToolCall` and `ToolCallTimeline`:

```go
func StartToolCallStream(ctx context.Context, conn *AgentSideConnection, session SessionId, start SessionUpdate,
    opts ...ToolCallStreamOption) (*ToolCallStream, error)

func (s *ToolCallStream) Write(p []byte) (int, error)          // io.Writer: the live text
func (s *ToolCallStream) WriteString(text string) (int, error) // io.StringWriter
func (s *ToolCallStream) SetStatus(status ToolCallStatus) error
func (s *ToolCallStream) Update(f func(*ToolCallEntity)) error
func (s *ToolCallStream) Entity() ToolCallEntity
func (s *ToolCallStream) Dropped() int64
func (s *ToolCallStream) Err() error
func (s *ToolCallStream) Flush(ctx context.Context) error
func (s *ToolCallStream) Finish(ctx context.Context, status ToolCallStatus) error

type ToolCallStreamOption func(*toolCallStreamConfig)
func WithToolCallStreamInterval(d time.Duration) ToolCallStreamOption
func WithToolCallStreamMaxText(n int) ToolCallStreamOption

var ErrToolCallStreamFinished, ErrToolCallStreamField error
```

* **`StartToolCallStream`** sends `start`, which must be a `tool_call` (`StartToolCall`'s result), with `conn.SessionUpdate(ctx,
  …)`, so the call exists on the wire before it returns; its fields are the stream's first state. A package function, not a
  side method (F7). It refuses, with nothing sent, a `start` that is not a `tool_call`, and returns `SessionUpdate`'s error:
  `ErrNotInitialized`, `*CapabilityError`, `ErrConnectionClosed`. The options follow `ConnectionOption`'s form
  (`connection.go:575`).
* **The state is one `ToolCallEntity`** (F4), the type `ToolCallTimeline` returns, so what an agent's stream holds and what a
  client's timeline rebuilds from its updates are the same value. `Entity` returns a copy, the live text as its last content
  item. Confirmation tests the identity.
* **`Write` and `WriteString`** append to the live text, which the snapshot carries as one text `content` item after the
  entity's content. The stream is an `io.Writer`, so an agent sets `cmd.Stdout = stream`, or tees a buffer of its own through
  `io.MultiWriter`. They never block on the wire and never fail but after `Finish` (`ErrToolCallStreamFinished`). Per
  `io.Writer`'s contract they return `len(p)`, also when D4's bound drops older bytes, as `clienthost`'s terminal output does
  (`clienthost/output.go:27`-`:36`). An agent that wants live text elsewhere in the content keeps its own item through `Update`.
* **`SetStatus`** takes `pending` or `in_progress`. A final status is `Finish`'s, and `SetStatus` refuses it with
  `ErrToolCallStreamField`, so no client sees `completed` and then more updates of the call.
* **`Update`** runs `f` on the entity under the stream's lock, for any other change: title, kind, name, content in the caller's
  order, locations, `rawInput` and `rawOutput` as whole values. `f` must return quickly and must not call the stream. `Id` and
  `Status` are the stream's: a change to either is undone and `Update` returns `ErrToolCallStreamField`, naming the field; the
  other changes stand. After `Finish` it returns `ErrToolCallStreamFinished`.
* **`Flush`** writes the pending state now and returns that write's error, nil at once when nothing is pending.
* **`Finish`** sets `status`, `completed` or `failed` (another is refused before anything is sent), and writes one update with
  it and the whole state, even when nothing else is pending. On success the stream is finished: its sender stops and later
  calls return `ErrToolCallStreamFinished`. On an error the status is restored and the stream is not finished, so `Finish` may
  be called again; resending a whole state is harmless.
* **`Flush`'s and `Finish`'s context** is honoured, as Go's convention asks, except a cancellation whose cause wraps
  `ErrPromptCancelled` or `ErrShutdown`. Those are the turn's own end: `session/cancel`, a superseding prompt, or `Shutdown`
  (F5, F8d), and wrapping up is what these calls exist for after them. A deadline of the caller's, or any other cancellation,
  stops the call. A call stopped before its bytes were handed to the writer leaves the state pending, and the sender writes it.
  This replaces gobble-cli's hand-made `context.WithoutCancel` (F16) without discarding the caller's deadline as that does.
* **`Err`** returns the first error of a background write, as `bufio.Scanner.Err` reports a sticky one. The sender goes on with
  later changes, which may correct a refused value: a location that is not absolute, say.
* **Not checked:** that the call's id is unique in the session. A second `tool_call` of one id replaces the first on the client
  (`tooltimeline.go:61`-`:75`), and that is the agent's choice.
* **Concurrency:** every method is safe for concurrent use. Snapshots pass `WithSendingMiddleware` and `WithObserver` as any
  `session/update` does.

### D2. One counted sender per active stream, latest wins

* **A pending flag, not a queue.** A change sets `pending` under the stream's mutex. A change while a write is blocked by
  back-pressure (`OverflowBlock` at a slow client, F18's gobble client) only keeps the flag set, and the next write takes the
  state as it then is. Memory per stream is one state.
* **The sender runs while something is pending.** A change with no sender running starts one. The sender loops: it waits until
  one interval has passed since the last write, with a `time.Timer` (stopped on exit; Go 1.23's timers need no drain), or until
  the connection ends; it takes the snapshot and clears `pending`; it writes. It exits when it finds nothing pending. It checks
  `pending` and clears `running` under the same lock, so a change either is seen by the running sender or starts a new one,
  and none is lost. The interval carries over through the last write's time, so no sender lingers to enforce it. A stream the
  agent forgets, or one that waits on a long tool, holds no goroutine.
* **One write at a time per stream.** A one-slot channel guards taking the snapshot and writing it. The sender, `Flush` and
  `Finish` all take it: the caller's goroutine writes for `Flush` and `Finish`, and a `select` on the caller's context bounds
  the wait for the slot, which a `sync.Mutex` cannot. Snapshots of one call are therefore written in the order taken, and none
  regresses. A failed write sets `pending` again.
* **The start.** The sender starts through the connection's wait group (F6), so `Wait` counts it and `TestGoStatementsCounted`
  stays as it is. A change may come from any goroutine, a tool worker the agent started included, which the group does not
  count. The connection gains one unexported start, `goIfLive`. It holds a mutex, returns false once the connection is marked
  ended, and calls `goroutines.Go` under that mutex. `writeLoop`'s first statement defers the mark. `writeLoop` is counted and
  returns only once the connection's context ends (F8b), so a start that finds the connection unmarked adds to a counter that
  is above zero, as `sync.WaitGroup` requires. When `goIfLive` refuses, `Err` records `ErrConnectionClosed`.
* **Every snapshot carries the whole state:** title, kind, status, name, content, locations, `rawInput` and `rawOutput` as the
  entity holds them. A client that dropped or merged earlier snapshots is whole again with any later one, which is the
  idempotence driver; the cost is resending `rawInput` and `rawOutput`, values the caller sets once.
* **At most one snapshot per interval,** default 250 ms (question 2), set by an option. A change after a quiet interval goes at
  once; the changes within an interval go together at its end. A stream that never changes after `start` sends nothing until
  `Finish`.
* **The work is parallel, the write is shared.** Each sender copies, gates, validates and encodes its own snapshot (F8) off the
  stream's lock, so concurrent streams do that work in parallel with each other and with the tools. The copy is
  `deepCopy` of the entity (`deepcopy.go:12`) and one `string` of the live text, at most once per interval.
* **The background write's context** is `context.WithoutCancel` of the start's context: it keeps the values, for middleware
  and the turn's stream set, and is bounded by the connection's end (F8a).

### D3. Order

* At most one sender per stream at a time, one write in flight: snapshots of one call are written in the order they were
  taken, and none regresses.
* `Flush` and `Finish` return only after the write, so a caller that orders other updates around a call (gobble-cli flushes its
  text before every tool event, F16) keeps the wire order equal to its event order.
* **When `Prompt` returns, the turn's unfinished streams are flushed before the turn is answered** (question 4), so the
  answer never overtakes a call's last state. Their status is left as it is: the SDK invents no `failed`, and the client marks
  them as F3 says. This touches the generated `promptCase`. Each flush returns after its write (F8a), so the answer is written
  after the snapshots; on the client, the existing notification watermark then has its handler see them before `Prompt`
  returns. The flush runs with no lock of the side connection held, so a `session/cancel` received meanwhile is handled. It
  waits for the client as any write does: a client that stops reading delays the answer, as it would delay the answer itself.
  A `Prompt` that panics gets no flush (F8c).
* **A stream belongs to the turn whose context started it.** `promptCase` puts the turn's stream set in the context `Prompt`
  runs on, and `StartToolCallStream` registers with the set its `ctx` carries; a context derived from it, `WithoutCancel`'s
  included, carries it too. Under `PromptOverlapConcurrent` two turns of one session keep separate sets, so one turn's answer
  never waits on another's streams. A stream started on a context with no turn belongs to none and is flushed by no answer.
* **The flush does not finish the stream.** A call that outlives its turn may go on sending; its later snapshots follow the
  answer, as the agent chose. `Finish` removes a stream from its turn's set.

### D4. Bounds

* The live text keeps at most a configured number of bytes, the tail, cut on a UTF-8 character boundary. Default 1 MiB
  (question 3), set by an option. `Dropped` counts what was cut, so an agent can put its own marker in the content; the SDK
  writes none into the text. A snapshot holds back a character the last write began and did not finish, which a later
  write completes; `Finish` sends all.
* The buffer is `clienthost`'s terminal output, moved to an internal package both use: it compacts once it is 64 KiB past
  its bound, so a write past the bound does not copy the bound's bytes each time (`clienthost/output.go:8`-`:11`), and it
  cuts and holds back as the schema asks of terminal output (`:56`-`:78`). One implementation of that rule serves both.
* `Update`'s content, locations and `rawOutput` are the caller's and are not cut: they are values, not streams.
* The stream does not police the message size: a snapshot over the peer's limit is a caller's content choice, and the peer's
  limit is unknown (F11, F12). D4's default keeps the live text far under every SDK's read limit.

### D5. What the stream does not do

* No append or delta on the wire, no `_meta` side channel (F1).
* No terminal output: a `terminal` item is put in the content by `Update`, and the client shows it (F2). The stream does not poll
  `terminal/output`.
* No partial `rawOutput`: it is replaced whole (F1).
* Nothing gobble-cli-specific: a summary-first content order or a 400-byte summary is built by the caller in `Update`.
* No client-side change: `ToolCallTimeline` already applies these snapshots.
* No replay: `session/load` sends finished calls in their last state with `StartToolCall`, as gobble-cli does (its
  `acpserver/history.go:439`-`:458`). A stream is for a call while it runs.

### D6. The default message size is 32 MiB

* `defaultMaxMessageSize` (`connection.go:39`) and `remote`'s `defaultMaxMessageSize` (`remote/client.go:13`) become
  `32 * 1024 * 1024`, the TypeScript SDK's `DEFAULT_MAX_MESSAGE_BYTES` (F12). `WithMaxMessageSize` and `MaxMessageSize` keep
  their meaning; an explicit value is unchanged.
* `WithMaxQueuedBytes`' 64 MiB default and `remote`'s 64 MiB `MaxBuffered` stay, and stay at least one message (F13).
* It is a widening: a line between 10 and 32 MiB that was answered `-32600` is now read. No API changes; release notes name it.
* An outbound check (refuse to send a message a TypeScript peer would end the connection over) is not part of this record; it
  is recorded in More Information.

### Questions for the owner

1. **Placement and names.**
   * Recommended: root `acp`, `ToolCallStream` and `StartToolCallStream`, next to the constructors and the timeline (O1).
   * Alternative: a subpackage (O3).
2. **The default interval.**
   * Recommended: 250 ms, at most four snapshots a second per call: gobble-cli's records ask to keep the update count low (its
     0008-MADR D19 item 2).
   * Alternative: 100 ms.
   * Alternative: 50 ms, gobble-cli's text coalescing interval.
3. **The default live-text bound.**
   * Recommended: 1 MiB.
   * Alternative: 256 KiB.
   * Alternative: no default bound; the option alone.
4. **Unfinished streams when `Prompt` returns.**
   * Recommended: flushed before the answer, status untouched (D3).
   * Alternative: left alone; the agent's `Finish` is its own duty, and a late snapshot may follow the answer.
5. **The message size.**
   * Recommended: 32 MiB (M1).
   * Alternative: keep 10 MiB (M2).
   * Alternative: 50 MiB (M3).
6. **The release.**
   * Recommended: one minor for D1 to D6.
   * Alternative: D6 alone first, as a minor, then the stream.

#### Decided with the owner, 2026-11-10

| Question | Answer |
| :--- | :--- |
| 1. Placement and names | As recommended: root `acp`, `ToolCallStream` and `StartToolCallStream`. |
| 2. The default interval | As recommended: 250 ms. |
| 3. The default live-text bound | As recommended: 1 MiB. |
| 4. Unfinished streams when `Prompt` returns | As recommended: flushed before the answer, status untouched, streams bound to their turn through the `Prompt` context. The owner weighed it against leaving it to the agent, alone or with an `acptest` check. |
| 5. The message size | As recommended: 32 MiB. |
| 6. The release | As recommended: one minor, `v1.35.0`, for D1 to D6. |

[0052-PLAN-agent-tool-call-snapshot-stream-and-message-size-default.md](0052-PLAN-agent-tool-call-snapshot-stream-and-message-size-default.md)
implements it.

### Consequences

* Good, because an agent streams a tool's output with one start, an `io.Writer` and `Finish`, and gets coalescing, bounds,
  order and cancel-safe completion by default; the writer composes with `os/exec`, `io.Copy` and `io.MultiWriter`.
* Good, because the stream sends only full snapshots, which a client may drop or merge without losing state.
* Good, because concurrent tool calls neither wait on each other's encoding nor on a slow client: a blocked send coalesces.
* Good, because `Wait` still counts every goroutine of the connection, under the guard as it is.
* Good, because this SDK reads every message the TypeScript SDK reads.
* Neutral, because gobble-cli gains a producer only when it adds an output hook to its tools (F15), and its client must render
  snapshots without reprinting (F19); both are gobble-cli's.
* Neutral, because the cost is bounded but not small: at the defaults one stream writes at most four snapshots a second of at
  most its content plus 1 MiB of live text, about 4 MiB a second for a tool that prints without pause. It holds its state plus
  one encoded snapshot while writing. Concurrent streams add up, and the client's queue budget (64 MiB, `OverflowBlock`)
  applies to them as to any update.
* Bad, because the root package gains a goroutine per active stream, a timer in each, and a start path that depends on
  `writeLoop`'s end.
* Bad, because the generated `promptCase` gains a step (D3), with its emitter pins.
* Bad, because one connection may now hold a 32 MiB line in memory while reading it, where it held 10 MiB.

### Confirmation

The PLAN's tests, each seen to fail on a planted break in a scratch copy:

* **Round trip.** A stream's entity equals what a `ToolCallTimeline` builds from the updates it wrote, for scripted and fuzzed
  sequences of mutators, live text and `Finish`.
* **Coalescing.** With the writer blocked, many mutations produce one pending state; once unblocked, the latest is written and no
  earlier one after it.
* **Interval.** A burst of changes within one interval writes at most two snapshots: the first and the last.
* **Cancellation.** `session/cancel` then `Finish(ctx)` on the `Prompt` context writes the final update with its status. Planted
  break: `Finish` sending on `ctx` itself fails the test.
* **Prompt answer.** An unfinished stream's last state is written before the `session/prompt` answer (question 4 as answered).
* **Lifetime.** `Wait` returns after `Close` with streams open and senders ended; a stream started from an uncounted goroutine
  while the connection ends passes `-race` with no `WaitGroup` misuse. Planted breaks: a bare `go` (the guard fails); a start that
  skips the liveness lock (the race test fails).
* **Bound.** A live text past the bound keeps a valid UTF-8 tail of at most the bound, and reports what was dropped.
* **Idle.** A stream with nothing pending holds no sender goroutine once its last write is done.
* **The owned fields.** `Update` changing `Id` or `Status`, and `SetStatus` with a final status, return
  `ErrToolCallStreamField` and leave both as they were.
* **The writer.** `cmd.Stdout = stream` streams a command's output; a write split inside a character is held back until it
  completes.
* **The context.** `Finish` on a context cancelled by `session/cancel` or `Shutdown` writes; on one past its own deadline it
  returns the deadline's error, the stream stays unfinished, and a second `Finish` writes.
* **Turns.** Under `PromptOverlapConcurrent`, one turn's answer does not wait for another turn's stream; a `session/cancel`
  received during the flush is handled.
* **The start path.** `goIfLive` refuses after the end, and `writeLoop`'s first statement is the defer that marks it, which a
  structural test pins.
* **Message size.** The default is 32 MiB, a 20 MiB line is read with no option set, and a line just over 32 MiB is answered
  `-32600`; the same in `remote`.
* `make pre-add-check`, both modules, as CI runs it; `make api` reports only the additions.

## Pros and Cons of the Options

### O1. A sending stream in root `acp`

* Good, because the defaults carry the hard parts: coalescing, a bound, order, cancel-safe completion.
* Good, because it sits beside the constructors and the timeline, and shares `ToolCallEntity` with the latter.
* Good, because its goroutines live under the guard that keeps `Wait` exact.
* Neutral, because the guard needs one new start path, proven by a race test.
* Bad, because it adds lifetime machinery to the root package.

### O2. A snapshot builder that does not send

* Good, because it is small, starts no goroutine and is easy to test.
* Good, because the official Python SDK ships this shape (F14).
* Bad, because coalescing under back-pressure, the order of the last state and sending after `session/cancel` (F5) stay each
  agent's to get right, and are the parts most often got wrong.
* Bad, because a producer that sends per change pays size × count with no bound.

### O3. O1 in a subpackage

* Good, because the root package and its guard stay as they are.
* Bad, because a subpackage cannot use the connection's wait group, so `Wait` would not count its goroutines, or it needs an
  exported hook that every caller could misuse.
* Bad, because D3's flush before the prompt answer needs the generated dispatch, which is root.

### O4. Nothing; a README pattern

* Good, because it adds no API.
* Bad, because the gap 0051 named stays, and each agent repeats the accumulator.

### M1. 32 MiB

* Good, because a message the reference SDK accepts is accepted here.
* Good, because 0045's budget and `remote`'s `MaxBuffered` still hold one message.
* Bad, because a reader may hold up to 32 MiB of one line.

### M2. 10 MiB

* Good, because nothing changes.
* Bad, because messages between 10 and 32 MiB from a TypeScript-built peer are refused and lost (F12), on a value no record
  chose (F10).

### M3. 50 MiB

* Good, because it matches Python's figure.
* Bad, because Python's figure is a buffer, not a message limit (F12), and under the 64 MiB queue budget only one 50 MiB
  message fits, where two 32 MiB ones do.

### M4. No limit

* Bad, because one line could take all memory; 0017 introduced the answered limit to avoid exactly that end.

## More Information

### Not chosen, and why

* **An outbound size check** (refuse to send over a configured size, as a TypeScript peer ends the connection over 32 MiB): peers
  differ (F12), and a sender that refuses its own message needs a policy of its own. A later record can take it.
* **ACP v2 chunks** (`tool_call_content_chunk`, `terminal_output_chunk`): not in 1.25.0. The mutator API takes increments, so a
  later `/v2` could send them without changing callers; that is not a reason for this design and is not promised.
* **A client-side change:** `ToolCallTimeline` already applies replace-whole updates (0041 item 9).
* **One sender goroutine per connection** for every stream: fewer goroutines, but it would gate and encode each stream's snapshot
  in turn (F8), and a connection with no stream would carry it. Per-stream senders that run only while active do neither.
* **`time.AfterFunc` for the deferred send:** no goroutine while idle, but its callback runs outside the wait group, which
  0026's correction of 2026-10-20 notes of `context.AfterFunc` too, so `Wait` would not wait for a send in flight.
* **A sender for the whole stream's life:** simpler, but a forgotten or long-waiting stream holds a goroutine until the
  connection ends.

### Review of 2026-11-10

After the owner's answers, a review against the tree corrected and completed the record before its PLAN was approved. It
changed no answer of the owner's. It added these rules, which the owner accepted with the PLAN on 2026-11-10:

1. F8 rewritten: the gate, validation and encoding are `SessionUpdate`'s, `encodeParams`' and `sendMessageSent`'s, not
   `prepareNotification`'s alone; F8a to F8d added.
2. D1: `Update`'s contract (`Id` restored, a final status undone and reported as `ErrToolCallStreamStatus`), `Flush`'s and
   `Finish`'s return conditions, `Err`, the start's refusals, the unchecked id, middleware and observers.
3. D2: the sender runs only while the stream is active, in place of one for the stream's life; `goIfLive` anchored on
   `writeLoop`'s first defer.
4. D3: the flush holds no side-connection lock, waits as any write does, and does not run after a panic.
5. D5: no replay through a stream.
6. A Neutral consequence for the cost; the Bad one counts active streams.

### Second review of 2026-11-10: Go idioms

The owner asked for the three rules the first review added to be checked against Go's practice. The review kept their aim
and changed their form:

1. **The sender while active (D2).** Kept. The first review's sender lingered one idle interval and had three states; the
   sender now exits when nothing is pending, the interval carried by the last write's time, with `running` and `pending`
   under one lock. `Flush` and `Finish` write on the caller's goroutine, serialized with the sender by a one-slot channel a
   context can bound, in place of asking the sender and waiting. A failed write sets `pending` again. Tests read `running`
   in the package, in place of a package-level counter.
2. **`Update`'s owned fields (D1).** Changed. Undoing a change silently and reporting it later through `Err` hid a
   programming error from its call site. `Update` now returns `ErrToolCallStreamField`; `SetStatus` takes the status
   changes that are not final; `Err` keeps only background write errors, as a sticky `Err` does in the standard library.
   `ErrToolCallStreamStatus` is gone.
3. **The start anchored on `writeLoop` (D2).** Kept. Starting must be atomic with the end's mark, so an `atomic.Bool`
   cannot do it; a watcher goroutine would add one goroutine to every connection and change the set of goroutines
   `docs/architecture.md` describes; locking at each place the connection's context is cancelled (`connection.go:1242`,
   `:2154`) would have to be kept in step; `context.AfterFunc`'s callback is not counted. The defer costs nothing, and a
   structural test pins it.

The same review changed three things the rules touch:

* **`AppendText` became `Write` and `WriteString`,** so the stream is an `io.Writer` and a command's output goes to it
  without an adapter. Its buffer is `clienthost`'s, moved to an internal package (D4), in place of a string appended to,
  which would copy the whole text on every write.
* **`Finish`'s context is honoured but for the turn's own end,** in place of `context.WithoutCancel`, which also discarded
  the caller's deadline. `Flush` has the same rule.
* **`Finish` may be retried** after an error, in place of finishing the stream whatever the write's result.

The owner accepted the record as both reviews left it, and approved its PLAN, on 2026-11-10.

### Observations for gobble-cli, not decided here

* Its final update re-embeds a client terminal already released by the tool's deferred `Release` (`tool/builtin/shell.go:217`
  before `acpserver/prompt.go:125`-`:127`). The schema asks for the terminal to be added before release, which the earlier update
  does; whether the released id should stay in the final content is gobble-cli's to decide.
* Its 0002-MADR (`:987`) says the in-process `bash` tool streams its output as `tool_call_update` content; the code sends it once,
  at the end (F15).

### Sources

* [0051-REPORT-acp-1-25-0-featureset-coverage-and-three-extensions.md](../reports/0051-REPORT-acp-1-25-0-featureset-coverage-and-three-extensions.md), item 1
* [0041-MADR-close-the-gaps-the-consumers-wait-on.md](0041-MADR-close-the-gaps-the-consumers-wait-on.md), item 9 and its phase-6 and
  phase-11 amendments
* [0026-MADR-connection-end-cause-and-wait.md](0026-MADR-connection-end-cause-and-wait.md)
* [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md)
* [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), a null raw value is an absent one
* [0043-MADR-remote-http-and-websocket-transport.md](0043-MADR-remote-http-and-websocket-transport.md)
* [0044-MADR-notification-queue-back-pressure.md](0044-MADR-notification-queue-back-pressure.md)
* [0045-MADR-notification-queue-byte-budget.md](0045-MADR-notification-queue-byte-budget.md)
* [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md)
* [0016-REPORT-acp-v1-conformance-audit.md](../reports/0016-REPORT-acp-v1-conformance-audit.md)
* [0001-REPORT-go-127-acp-parity.md](../reports/0001-REPORT-go-127-acp-parity.md)

### Tree citations

`helpers.go:796`, `:897`; `tooltimeline.go:12`, `:57`, `:61`-`:75`; `agent_gen.go:161`, `:627`, `:757`, `:1002`-`:1007`;
`connection.go:39`, `:300`, `:316`, `:328`, `:967`, `:1016`-`:1020`, `:1041`-`:1051`, `:1132`-`:1135`, `:2484`-`:2507`,
`:2814`-`:2821`, `:3086`-`:3090`, `:3105`-`:3109`, `:3216`; `connection_end_test.go:456`; `remote/client.go:13`;
`clienthost/output.go:58`.

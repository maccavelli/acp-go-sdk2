# ACP schema 1.25.0: current SDK featureset, coverage, and three extensions

Observed 2026-11-10 on `1e945dd` (`v1.34.2-1-g1e945dd`, tagged `v1.34.2` plus one docs commit). At the owner's request: assess the
current codebase featureset and functionality; analyse the ACP 1.25.0 spec/schema; evaluate what still needs to be added or
extended to more fully provide the schema's capabilities; and name three items that would provide benefit, enhance flexibility
and extensibility, future-proof the module, and broaden the variety of use cases.

This report records what was read. It decides no fix, amends no record, and plans no phase. A later `0051` MADR can take any
item here; until then the three items are observations, not work.

## Question

Does the SDK, at `v1.34.2`, already generate and enforce every method, type and MUST of the pinned stable schema `1.25.0`? Where
the schema is present on the wire and the remaining gap is behaviour an application still writes, which three additions would
most increase what a client or an agent can do without inventing protocol?

## How it was assessed

- **This tree.** `git status` clean and origin-equal; `schema/version` is `1.25.0`; SHA-256 of `schema/schema.json` is
  `4bcd1d2051ff7ed379bd9b234fb9b09c5ab9245478698e9fab805bd9b52a33a6` (255170 bytes), of `schema/meta.json`
  `061edb6efa8fb2aa2792459a86ec7268de5fe665bba48b2ffe7939df01481f88`. [0016-REPORT-acp-v1-conformance-audit.md](0016-REPORT-acp-v1-conformance-audit.md)
  recorded the 1.25.0 release assets as byte-identical on 2026-10-12; the committed files have not been regenerated since
  [0004-PLAN-pin-acp-schema-1-25-0.md](../decisions/0004-PLAN-pin-acp-schema-1-25-0.md). `$defs` count in `schema.json`: 178.
- **Schema surface.** `schema/meta.json` (13 agent methods, 11 client methods, `$/cancel_request`). `schema/meta.unstable.json`
  (28 agent methods, 12 client methods, the same protocol method). SessionUpdate `sessionUpdate` consts, StopReason consts,
  AgentCapabilities / ClientCapabilities / SessionCapabilities / ToolCallUpdate, read from `schema.json`.
- **Generated and hand-written code.** `types_gen.go` (`Agent`, `AgentLoader`, `Client`, `ClientExperimental`, `AgentExperimental`),
  `agent_gen.go`, `client_gen.go`, `helpers.go` (`StartToolCall`, `UpdateToolCall`, `OfferedAuthMethods`, `AuthenticateMethodIDs`),
  `tooltimeline.go`, `cmd/generate/internal/emit/dispatch.go` (`promptCase`), `errors.go`, `clienthost/`, `agentproc/`, `remote/`,
  `acptest/`, `unstable/`.
- **Records.** 0016 (audit), 0017 (remediation, PLAN complete), 0025 (schema 1.26.0 adoption, opinion: not now), 0039 (unstable
  in the root package), 0041 item 9 (tool-call timeline, replace-whole), 0043 (remote chosen over MCP-over-ACP and a registry
  client; Last-Event-ID out of scope), 0049 S6/S8, 0050 D8 (prompt constructors and `AuthenticateAdvertised` left out).
- **Consumers, as a pin check only.** gobble-cli and magic-cli-remote both require `github.com/maccavelli/acp-go-sdk2 v1.34.2`.
  Neither product tree imports `acp-go-sdk2/remote` (a mention exists in magic-cli-remote's 0224 MADR). Their adoption work is
  theirs; this report does not re-inventory their copy sites (that is
  [0041-REPORT-consumer-adoption-of-v1-21-to-v1-25.md](0041-REPORT-consumer-adoption-of-v1-21-to-v1-25.md)).

Nothing in this repository or the consumers was written but this report and its index row.

## Tree at this observation

| Fact | Value |
| :--- | :--- |
| HEAD | `1e945dd` `docs(decisions): record 0049 generator fixes as released in v1.34.2` |
| Newest tag | `v1.34.2` |
| Working tree | clean, equal to `origin/main` |
| Next unused record at the start of this write | `0051` (`python3 scripts/check_records.py --next`) |
| Go floor / toolchain | `go 1.26.10` / `toolchain go1.27.3` |
| Retract | `v1.13.0` |
| Schema pin | `1.25.0` (`schema/version`) |
| Live mutation | none |
| 0050 | complete in `v1.34.2`; D8 not done |

Packages: root `acp`; `agentproc`; `clienthost`; `acptest`; `remote` (experimental); `unstable` (experimental); `cmd/generate`
(own module); `example/` (`agent`, `client`, `claude-code`, `gemini`, `remote`).

## Schema 1.25.0 inventory

### Methods (`schema/meta.json`)

**Agent (13):** `initialize`, `authenticate`, `session/new`, `session/load`, `session/set_mode`, `session/set_config_option`,
`session/prompt`, `session/cancel`, `session/list`, `session/delete`, `session/resume`, `session/close`, `logout`.

**Client (11):** `session/request_permission`, `session/update`, `fs/write_text_file`, `fs/read_text_file`, `terminal/create`,
`terminal/output`, `terminal/release`, `terminal/wait_for_exit`, `terminal/kill`, `elicitation/create`, `elicitation/complete`.

**Protocol (1):** `$/cancel_request`.

Every one of these is a generated method on `AgentSideConnection` / `ClientSideConnection` and a method of `Agent` /
`AgentLoader` / `Client` (`types_gen.go`). [0043-MADR-remote-http-and-websocket-transport.md](../decisions/0043-MADR-remote-http-and-websocket-transport.md)
already recorded: a gap at this pin is not a missing method.

### Agent capabilities (`schema.json` `$defs.AgentCapabilities`)

- `loadSession` (boolean, default false) — `session/load`; `AgentLoader`.
- `promptCapabilities`: `image`, `audio`, `embeddedContext` (all default false). Baseline prompt content is text and
  `resource_link`.
- `mcpCapabilities`: `http`, `sse` (default false). Stdio MCP servers are the baseline; HTTP and SSE are advertised.
- `sessionCapabilities`: `list`, `delete`, `additionalDirectories`, `resume`, `close` — each omitted or `null` means
  unsupported; `{}` advertises the method. `session/load` remains the top-level `loadSession` flag (schema note: unify later).
- `auth` (`AgentAuthCapabilities`), including logout when advertised.
- `_meta`.

### Client capabilities (`$defs.ClientCapabilities`)

- `fs.readTextFile` / `fs.writeTextFile` (default false).
- `terminal` (boolean, default false) — all `terminal/*` methods.
- `session`: `compaction`, `notices`, `configOptions.boolean`.
- `auth.terminal` (default false).
- `elicitation` (`form` / `url` modes).
- `_meta`.

### Prompt turn (`$defs.StopReason`)

Closed `oneOf` of five string consts: `end_turn`, `max_tokens`, `max_turn_requests`, `refusal`, `cancelled`. Schema text on
`cancelled`: this stop reason MUST be returned when the client sends `session/cancel`, even if cancellation causes exceptions
in underlying operations.

### Session updates (`$defs.SessionUpdate`)

Fourteen `sessionUpdate` consts: `user_message_chunk`, `agent_message_chunk`, `agent_thought_chunk`, `tool_call`,
`tool_call_update`, `plan`, `available_commands_update`, `current_mode_update`, `config_option_update`, `session_info_update`,
`usage_update`, `notice`, `compaction_update`, `compaction_summary_chunk`.

### Tool-call updates (`$defs.ToolCallUpdate`)

All fields except `toolCallId` optional. Description: "only changed fields need to be included." `content` and `locations`:
"Replace the content collection" / "Replace the locations collection"; `null` or omitted leaves the previous value.
`rawInput` / `rawOutput` are snapshots with the same omit/null leave-previous rule (0012: a null raw value is an absent one in
this SDK).

### Unstable meta (`schema/meta.unstable.json`), generated and frozen-deprecated

Adds, among others: `session/fork`; `providers/{list,set,disable}`; `mcp/message` both ways; `nes/{start,suggest,accept,reject,close}`;
`document/{didOpen,didChange,didClose,didSave,didFocus}`. These sit on `AgentExperimental` / `ClientExperimental` and on the
side connections as `Unstable*` methods, marked Deprecated under
[0039-MADR-five-adopter-additions-on-schema-1-25-0.md](../decisions/0039-MADR-five-adopter-additions-on-schema-1-25-0.md).
Upstream schema 1.26.0 removes the NES and document-event half; this pin still generates them.

## What the SDK already provides

### Wire and connection (`connection.go`, generated dispatch)

- Newline-delimited JSON-RPC 2.0, `"jsonrpc":"2.0"` stamped, mutex-serialized writes, no embedded newline.
- Notification ordering with a watermark so a `session/prompt` response cannot overtake earlier `session/update`s, and
  `session/load` replies after replay (stronger than the spec MUST; 0016).
- JSON-RPC parse/invalid-request answers: `-32700` / `-32600` with `"id": null` as appropriate; overlong lines answered
  `-32600` and reading continues (`README.md`, "Errors on the wire"; 0017 V2). A request whose id is `null` is answered with
  `"id": null`. A response is never answered.
- `$/cancel_request` handled synchronously, outside notification ordering.
- OverflowBlock default from `v1.29.0`; `WithMaxQueuedBytes` default 64 MiB from `v1.30.0`.
- Absolute-path validation on send and receive (0015, 0017 V4).
- Protocol version: agent `initialize` answers `ProtocolVersionNumber` (1) and refuses a mismatch (0017 V5; `agent_gen.go`).
- Capability gates on send and receive from the recorded `initialize` (0017 V3; `Negotiation`, `AllowsValue`).
- `WithInboundRewrite`, `WithInboundValidation`, `ChainRewrites`, `ResolveStdioCommands`.
- `WaitContext`, `EndedCleanly`, `Shutdown`, `ServeAgent`.
- Opt-in `Middleware` / `Observer` (0046); neither known consumer uses them on a product path.

### Prompt turn (0017 V1, V6)

`promptCase` (`cmd/generate/internal/emit/dispatch.go`): a cancelled turn is answered with `StopReasonCancelled` and a nil
error, whatever `Prompt` returns, keeping the rest of the handler's result. `example_agent_test.go` documents this: "the SDK
then answers the turn cancelled, whatever Prompt returns."

`ClientSideConnection.Prompt`: when the caller's context ends after the request was sent, it sends `session/cancel`.
`session/request_permission` runs under `pending.track`; `answerUnlessCancelled` answers
`NewRequestPermissionOutcomeCancelled()` once the prompt turn is cancelled (`client_gen.go`; protocol table
`CancelledByPromptCancel`). That is the spec MUST on pending permission requests.

Marking in-flight tool calls `cancelled` remains application UI state. 0017 recorded it as such: the SDK holds no tool-call
reducer on the client connection. `ToolCallTimeline` is the client-side reducer an application feeds; it does not send.

### Closed enumerations and authenticate (0049 S6, 0049 S8, 0050)

Outbound `Validate` / `checkResult` refuse a closed-set value that is not a member. `PromptResponse{}` is `-32603` so an empty
`stopReason` is not sent. Inbound `judgeInbound` passes `closed: false`: a required field keeps an unknown value; lenient
fields follow `x-deserialize-default-on-error` / `x-deserialize-skip-invalid-items` (0049 Q4 (a)). Receive-side leniency is a
compatibility choice, not schema-strict inbound validation.

`authenticate.methodId` must name a method from that connection's `initialize` `authMethods` (S8). Unadvertised send:
`ErrUnadvertisedAuthMethod`, nothing written. Inbound: `-32602`, handler not run. Terminal ids: `ErrTerminalAuthMethod` first.
`OfferedAuthMethods` drops terminal methods unless the client advertised `auth.terminal`. `AuthenticateMethodIDs` / `AuthMethod.ID()`
are the 0050 helpers; the refusal text lists the accepted ids.

0050 **D8, not done:** `NewPromptResponse` / `PromptEndTurn` / `PromptCancelled`; `AuthenticateAdvertised` on
`ClientSideConnection` (the issue called it "probably unnecessary"); a hint in the `-32603` text; a generator doc-note on
`Agent.Prompt`.

### Tool calls (0041 item 9, `v1.25.0`)

`StartToolCall` / `UpdateToolCall` with option funcs. `ToolCallTimeline.Apply`: a `tool_call` replaces the entity; a
`tool_call_update` patches carried fields; `content` and `locations` replace the collections, never append; empty `[]` clears;
absent or null leaves; an unseen id starts `pending`. Deep-copy so callers share nothing with the timeline (0049 S4).

### Elicitation, compaction, notices, refusals, extensions

`ElicitationSchema.Fields` / `Check` / `CheckSchema` / `ParseValue` / `ParseElicitationURL` / `WithElicitationCheck` (0040).
`CompactionTimeline`, `PresentNotice` / `PresentCompaction` / `PresentCompactionSummary` (capability-aware fallback to an
agent message). `ClientRefusals` / `AgentRefusals`. `ExtensionMux`, `TraceMeta`, `AdvertiseMeta`. `PickPermissionOption`,
`SessionConfigSelectOptions.All`, `ConfigOptionByCategory`, `Kind()` on the remaining unions including `PlanUpdateContent`.

### Host and process packages

- `clienthost.Files` / `Terminals` / `Refusals`: character-boundary truncation; missing file `-32002` via `NewResourceNotFound`.
  Showing an elicitation form remains the client's UI (0041: form helpers exist; a default form host does not).
- `agentproc.Command`, `InProcess`, `LineTap`, `Transcript`, `ExitedFirst`, `WaitContext`. `Close` waits one extra grace for
  leftover agent output after exit.
- `acptest.Connect` / `ConnectRecorded` / `ValidateGolden` / `ScriptAgent` / `MemoryClient` / `ReplayAgent`.

### Remote transport (`remote`, `v1.30.0`, 0043)

Streamable HTTP and WebSocket at RFD revision 2026-07-02, as one `/acp` endpoint, presented as the same newline JSON-RPC
`io` stream `Dial` / `NewHandler` / `ServeAgent` already use. Experimental: outside the Go 1 promise; `make api` reports it
without gating. Defaults: MaxMessageSize 10 MiB, Grace 5 s, KeepAlive 15 s, MaxBuffered 64 MiB. Causes: `ErrDeleted`,
`ErrDisconnected`, `ErrHandlerClosed`, `ErrOverflow`, `ErrAbandoned`.

**Not in 0043, still absent in `remote/` (no `Last-Event-ID` / reconnect identifier in that package):** SSE resumability,
automatic reconnect, JSON-RPC batches (`501` on HTTP POST, `-32600` on WebSocket), WebSocket subprotocols and compression,
WebSocket over HTTP/2, Unix sockets. Auth, TLS, CORS, and per-client connection limits stay on the host. `Acp-Connection-Id`
is not a credential. Close `Handler` before `http.Server.Shutdown`.

## Coverage matrix

| Schema 1.25.0 piece | In the SDK | Remaining application or later-record work |
| :--- | :--- | :--- |
| All 13+11+1 stable methods | Generated, gated | None as methods |
| Unstable methods of the 1.25 pin | Generated, deprecated | NES/document events are dead upstream as of 1.26.0; still in this v1 API |
| StopReason required, five consts | S6 outbound; promptCase forces `cancelled` | Prompt constructors (0050 D8) are DX only |
| session/cancel → cancelled result | 0017 V1 | Agent still implements `Prompt` |
| Pending permission → cancelled outcome | 0017 V6, `answerUnlessCancelled` | Tool-call `cancelled` marks are the client's UI |
| Capability MUST NOT send | Outbound `CapabilityError` | `AllowsValue` for a dry run |
| Absolute paths | Send and receive | — |
| ProtocolVersion 1 | Agent stamps and checks | Client SHOULD close on unsupported (application) |
| Tool_call / tool_call_update replace-whole | Timeline + constructors | Agent-side accumulation of live output into successive full snapshots |
| Content chunks (user/agent/thought) | SessionUpdate constructors | Streaming is one notification per chunk, as the schema says |
| Compaction / notices | Timeline + Present* | Client must advertise the capability |
| Elicitation form/url | Types, Check, URL parse, MemoryClient script | No `clienthost` form/URL UI |
| fs / terminal | `clienthost` | Optional vs a client's own host |
| MCP stdio in session/new | Types; `ResolveStdioCommands` | HTTP/SSE MCP servers are types only; no MCP-over-ACP router (0043 O2 deferred) |
| Auth methods / terminal auth | OfferedAuthMethods, S8, 0050 ids | `AuthenticateAdvertised` declined in 0050 D8 |
| Extensibility `_` / `_meta` | ExtensionMux, TraceMeta | Application extensions |
| Stdio transport | `NewConnection`, `ServeAgent`, `agentproc` | — |
| Streamable HTTP / WebSocket | `remote` Dial / Handler | Resume and reconnect out of 0043; unused by both consumers |
| JSON-RPC errors including `-32002` | Constructors | — |
| Schema 1.26.0 client `systemPrompt` | Absent (stable `schema.json` unchanged; unstable half not adopted) | 0025: 115 incompatible names if regenerated into this v1 module |
| ACP v2 chunks / upsert | Absent; 0041 item 9 forbids a v1 append/delta wire | `/v2` or a stable chunk field |

**Finding.** The schema-geometry layer is complete for 1.25.0. Every remaining gap is either (a) behaviour the schema cannot
express as a type, which 0017 already closed for the MUST-level cases the connection can see, or (b) a helper or transport
feature an application still writes, or (c) a later schema / RFD that this v1 module cannot absorb without a compatibility
boundary.

## Remaining gaps that are not missing methods

1. **Live tool-call output as successive full snapshots.** The schema has no append/delta field. An agent that streams command
   output or growing JSON must accumulate and emit a new `tool_call_update` whose `content` is the whole list so far. The SDK
   helps the *client* apply those snapshots (`ToolCallTimeline`). It does not help the *agent* produce them.
2. **Remote connection resume.** `remote` exists and is unused. A dropped SSE stream today means a new `Dial` and
   `session/load`. The RFD's `Last-Event-ID` path is unimplemented.
3. **Unstable API shares the root package, including stable-named fields** (`AgentCapabilities.Nes`, position encodings, ten
   `AgentMethodNes*` / `AgentMethodDocument*` constants). Schema 1.26.0's removal and its Draft `systemPrompt` cannot land in
   a v1 minor (0025). ACP v2 chunk types have the same lock.
4. **Inbound closed enumerations stay lenient** (0049 Q4). An opt-in strict inbound check would be additive; it is not a 1.25.0
   MUST for a peer that "preserve the raw payload" allows.
5. **MCP-over-ACP and the agent registry** were considered in 0043 (O2, O3) and ranked below remote. Types for `mcp/message`
   exist and are deprecated; nothing routes `serverId`. A registry client that builds `agentproc.Command` from `registry.json`
   was judged an application concern.
6. **clienthost elicitation UI** was left to the client (0039/0041). Files and terminals are the two hosts that close protocol
   MUSTs the types cannot express (character-boundary truncation, `-32002`).
7. **0050 D8 prompt constructors** are shorter names for `PromptResponse{StopReason: …}`. S6 already refuses the empty value.
   They do not add a capability.

## Three items

The three below stay on the 1.25.0 wire. They increase what a program can do without a new RPC. They are ordered by how soon
a consumer could use them inside this module's compatibility promise.

### Item 1. A tool-call snapshot streamer (agent-side, 1.25-faithful)

**Schema fact.** `ToolCallUpdate.content` / `.locations` replace the collection (`schema.json` `$defs.ToolCallUpdate`).
`pending` means input is still streaming or waiting for approval. Live command output is a terminal content item, not appended
text. `rawInput` / `rawOutput` are snapshots.

**Code fact.** `StartToolCall` / `UpdateToolCall` build one notification. `ToolCallTimeline.Apply` (`tooltimeline.go:42-54`)
replaces content and never appends, as 0041 item 9 required. No type in the root package accumulates increments and *emits*
the next full snapshot.

**What to add (shape, not a decision).** An agent-side helper, for example `ToolCallStream`, that starts a call at `pending`,
accepts content / terminal-output / rawOutput increments in memory, and sends `session/update` as successive full
`tool_call_update` snapshots, then a terminal `ToolCallStatus`. It would use the existing constructors and the connection the
agent already holds (`AgentConnAware`).

**Benefit.** Every streaming agent (long `execute` output, progressive diffs, growing `rawOutput`) currently hand-rolls the
accumulator. One implementation keeps every consumer on replace-whole. Wrong code that "appends a delta on the wire" is the
thing 0041 item 9 exists to prevent.

**Flexibility / future-proofing.** The same helper API can later emit ACP v2 `tool_call_content_chunk` / `terminal_output_chunk`
when a negotiated version or a `/v2` module exists, without rewriting agents. v1 stays snapshots. Do not add a v1
`tool_call_delta` or append-content field; that would undo 0041 item 9 and disagree with `$defs.ToolCallUpdate`.

**Use cases broadened.** Progressive file edits, live terminals, large tool JSON, multi-location updates, any agent that today
buffers the whole tool result before the first `tool_call_update`.

### Item 2. Remote SSE resume (`Last-Event-ID`) and reconnect

**Schema / RFD fact.** Stable v1 transport is stdio (`transports`). The only specified non-stdio path is the Active RFD
"Streamable HTTP & WebSocket Transport" (revision 2026-07-02, the revision `remote/doc.go` pins). Custom transports MUST
preserve JSON-RPC message format and lifecycle.

**Code fact.** `remote.Dial` / `remote.NewHandler` already present both profiles as newline JSON-RPC. README "Not supported":
"resuming an event stream with `Last-Event-ID`, and reconnecting automatically. A client dials again and loads its session."
No `Last-Event-ID` symbol exists under `remote/`. Package is experimental (0039/0043 precedent: `make api` does not gate it).
Neither known consumer imports it.

**What to add (shape, not a decision).** On `remote` only: event ids on SSE; honour `Last-Event-ID`; client reconnect that
replays from the last id and keeps `Acp-Connection-Id`; `session/load` / `session/resume` only when the connection id is gone
(`ErrDeleted` / `ErrAbandoned`). Auth, TLS, CORS, and connection limits stay on the host.

**Benefit.** A dropped event stream today loses in-flight `session/update`s and forces a full load. Resume is what makes HTTP
usable on a phone, a relay, or a second host, which stdio cannot be.

**Flexibility / future-proofing.** The Conn seam is unchanged, so `acptest`, `agentproc.Transcript`, OverflowBlock, and
capability gates keep working. The RFD is Active; putting resume in the already-experimental package is the same compatibility
answer 0043 used for Dial/Handler. When the RFD completes or changes, only `remote` moves.

**Use cases broadened.** Remote agents, Magic-style HTTP control planes, mesh/relay, reconnecting IDE sessions, any client that
cannot hold a child process. 0043 chose this transport over MCP-over-ACP and a registry client because it is the only remote
path ACP has and every official SDK ships a form of it.

### Item 3. An unstable / v2 boundary so 1.26+ and v2 can land without breaking v1

**Schema fact.** `schema-v1.26.0` (2026-10-16): `schema.json` and `meta.json` byte-identical to 1.25.0; `schema.unstable.json`
drops NES and the document-event notifications (57 definitions) and adds Draft client `SystemPrompt` / `SystemPromptMode` on
`session/new|load|resume|fork`. ACP v2 drafts add `tool_call_content_chunk`, `terminal_output_chunk`, and `tool_call_update` as
upsert. 0025 regenerated 1.26.0 in a scratch clone: `scripts/check_api.py` against `v1.17.1` reported **115 incompatible and 16
compatible** changes. Forty of the 115 name no `Unstable` identifier because NES merged into stable-named types.

**Code fact.** Unstable definitions share package `acp`. Package `unstable` exists for names beyond the frozen manifest and is
outside the Go 1 promise, but NES still appears as `AgentCapabilities.Nes` and `UnstableDidOpenDocument` on the side
connection. [0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](../decisions/0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md)
holds this module to Go's v1 compatibility rule after `v1.14.0`. 0019 still owes a policy for removing unstable API (question
16, the record 0025 defers to).

**What to add (shape, not a decision).**

1. A published rule: root `acp` is the 1.25.0 stable protocol; unstable and v2 types live in `unstable` and/or a future `/v2`.
2. Deprecate the NES / document-event / position-encoding API in v1 now, naming upstream's removal, so a later major can drop
   what users were warned about (0025's lawful-within-v1 step; the generator cannot yet deprecate a node the schema does not
   mark).
3. Once the boundary exists, generate Draft `systemPrompt` into `unstable` (or behind a build tag / extra schema input) without
   touching gobble-cli's own sysprompt or magic-cli-remote's handshake.

**Benefit.** This is the lock on every later schema. Without it, the useful half of 1.26.0 (session-scoped client instructions)
and v2 chunks require a `/v2` import rewrite of every file that currently imports this module. With it, those features become
additive packages.

**Flexibility / future-proofing.** Agents that never want NES or a client system prompt keep `import acp`. Clients that do
(Claude-style ACP adapters, phone-set instructions, fork-with-prompt) opt in. TypeScript spent a major to isolate unstable;
Rust used a Cargo feature. This module has neither; the boundary is the missing piece 0025 named.

**Use cases broadened.** Client-provided system prompt on `session/new|load|resume|fork` without `_meta` or a private
extension; a clean drop of abandoned NES; a place for v2 chunks next to item 1's streamer.

## Items considered and left for a later record

These are real gaps. They scored below the three: either 0043 already ranked them, or 0050 already declined them, or they do
not broaden use cases as much.

- **Prompt constructors (0050 D8 O2).** Additive DX on a result S6 already checks. 0050 declined them as longer than the
  struct literal for the common case.
- **`AuthenticateAdvertised`.** 0050: the issue itself called it probably unnecessary; `AuthenticateMethodIDs` plus the send
  gate already agree.
- **Opt-in inbound closed-enum strictness.** Compatible default is 0049 Q4 (a). A `WithStrictInbound` would help gateways and
  tests; it does not add a protocol capability.
- **MCP-over-ACP router (0043 O2).** Types exist, deprecated; neither consumer wanted it when 0043 chose remote; the RFD is as
  unsettled as the transport was.
- **Registry client (0043 O3).** Completed standard, sits in client applications; 0043 left it there.
- **clienthost elicitation form host.** Completes a triad; showing a form is still the client's product UI. 0040 already closed
  the protocol MUSTs (check the answer, refuse a bad URL).
- **Adopting schema 1.26.0 as a drop-in regeneration of the root package.** 0025's opinion remains: the stable protocol does
  not change, and the removal is 115 incompatible names. Item 3 is the prerequisite, not the regeneration itself.

## What a later record would decide

If the owner wants any of the three items built, that is a greenfield pair: next unused number after this report is committed
(this file *is* 0051; a MADR for an item would be 0052 unless the owner reuses 0051 for the decision). Suggested split if all
three proceed:

- Item 1 can live in root `acp` as additive helpers next to `StartToolCall` / `ToolCallTimeline`, under Go 1 compatibility.
- Item 2 belongs in `remote` only, as 0043's experimental package, amending 0043 or a new MADR that names the RFD revision it
  implements.
- Item 3 is a policy MADR first (0019 question 16 / 0025's deferred decision). Generating `systemPrompt` waits on that policy.
  Deprecating NES in v1 can be a phase of the same plan; removing NES is a major or a `/v2`.

This report does not choose among those options.

## Sources in this repository

- `schema/version`, `schema/schema.json`, `schema/meta.json`, `schema/meta.unstable.json`
- `types_gen.go` (`Agent`, `Client`, experimental interfaces)
- `helpers.go` (`StartToolCall`, `UpdateToolCall`, `OfferedAuthMethods`, `AuthenticateMethodIDs`)
- `tooltimeline.go` (0041 item 9)
- `cmd/generate/internal/emit/dispatch.go` (`promptCase`)
- `client_gen.go` (`Prompt` cancel, `RequestPermission` cancelled outcome)
- `remote/doc.go`, `README.md` "Remote agents" / "Not supported"
- [0016-REPORT-acp-v1-conformance-audit.md](0016-REPORT-acp-v1-conformance-audit.md)
- [0017-MADR-remediate-acp-v1-conformance-violations.md](../decisions/0017-MADR-remediate-acp-v1-conformance-violations.md)
- [0025-REPORT-schema-1-26-0-adoption-assessment.md](0025-REPORT-schema-1-26-0-adoption-assessment.md)
- [0039-MADR-five-adopter-additions-on-schema-1-25-0.md](../decisions/0039-MADR-five-adopter-additions-on-schema-1-25-0.md)
- [0041-MADR-close-the-gaps-the-consumers-wait-on.md](../decisions/0041-MADR-close-the-gaps-the-consumers-wait-on.md)
- [0043-MADR-remote-http-and-websocket-transport.md](../decisions/0043-MADR-remote-http-and-websocket-transport.md)
- [0043-PLAN-remote-http-and-websocket-transport.md](../decisions/0043-PLAN-remote-http-and-websocket-transport.md)
- [0049-PLAN-surface-fixes.md](../decisions/0049-PLAN-surface-fixes.md) (S6)
- [0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md](../decisions/0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md)
  (D8)

## Addendum, 2026-11-11: the cost of schema 1.26.0, corrected, and item 3 since

Two places state 0025's count as the cost at `1e945dd`, which it was not. The body above is left as written.

- **The places.** The coverage table's row "Schema 1.26.0 client `systemPrompt`", whose last cell reads "0025: 115
  incompatible names if regenerated into this v1 module"; and, among the candidates not chosen, "the removal is 115
  incompatible names".
- **Why they were wrong.**
  [0025-REPORT-schema-1-26-0-adoption-assessment.md](0025-REPORT-schema-1-26-0-adoption-assessment.md)
  measured the 115 against `v1.17.1`. The frozen manifest of
  [0039-PLAN-five-adopter-additions-on-schema-1-25-0.md](../decisions/0039-PLAN-five-adopter-additions-on-schema-1-25-0.md),
  2c, ended them on
  2026-10-25: "With the real `schema-v1.26.0` unstable files, regeneration again leaves every generated file unchanged,
  and the root tests pass." It is `0c8088e`, in `v1.23.0`, eleven minor releases before the `v1.34.2` this report read.
  So at `1e945dd` a regeneration from `1.26.0` removed no name from package `acp`.
- **What stands.** The "Schema fact" paragraph of item 3 gives 0025's measurement with its baseline, and is right as
  history. The "Code fact" paragraph names the manifest; the two places above did not draw its consequence.
- **Item 3 since.**
  [0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md](../decisions/0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md)
  took it, for schema `1.31.0`, and `v1.36.0` released it.
  - By `1.31.0` the client system prompt is in the stable schema, so it is generated into `acp`, not into `unstable`
    as the item's step 3 supposed.
  - Package `unstable` gained its first generated file, 46 names.
  - Measured against `v1.35.0`: "0 incompatible, 57 compatible". Run again for this addendum, on `8c0170d`:
    `make api BASE=v1.35.0`, exit 0, the same line.

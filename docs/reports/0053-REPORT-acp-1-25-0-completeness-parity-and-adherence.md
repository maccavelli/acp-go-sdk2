# ACP 1.25.0 completeness, parity, and adherence

Observed 2026-11-10 on `108b344` (`v1.35.0-4-g108b344`). At the owner's request: how close acp-go-sdk2 is to having
the whole of ACP schema 1.25.0 defined, designed and implemented, and where it falls short of the standard or departs
from it.

This report records what was read and run. It decides no fix, amends no record, and plans no phase. It does not reopen
[0051-REPORT-acp-1-25-0-featureset-coverage-and-three-extensions.md](0051-REPORT-acp-1-25-0-featureset-coverage-and-three-extensions.md),
which named three extensions at `v1.34.2`; the first of them shipped in `v1.35.0` under
[0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md](../decisions/0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md).
It re-scores [0016-REPORT-acp-v1-conformance-audit.md](0016-REPORT-acp-v1-conformance-audit.md) against the tree as it
is now, and adds what 0016 did not do: an inventory of every normative sentence in the v1 book at the pinned tag, each
one assigned to the SDK or to the application.

## Question

At `v1.35.0`, does the SDK implement every method, type and capability of the pinned schema `1.25.0`, and every MUST
of the v1 book that a connection can observe? Where it does not, is the remainder a missing feature, a behaviour the
SDK chose, or a duty the protocol places on the application?

## How it was assessed

- **The pinned schema.** `schema/version` is `1.25.0`. SHA-256 of `schema/schema.json` is
  `4bcd1d2051ff7ed379bd9b234fb9b09c5ab9245478698e9fab805bd9b52a33a6` (255170 bytes), of `schema/meta.json`
  `061edb6efa8fb2aa2792459a86ec7268de5fe665bba48b2ffe7939df01481f88` (1159 bytes). Both match the hashes 0016 recorded
  when it compared them with the `schema-v1.25.0` release assets on 2026-10-12; the assets were not downloaded again
  for this report. `schema/schema.json` last changed in `17adda9` (2026-10-09). Counts were taken from the JSON with a
  script: 178 `$defs`; `meta.json` 13 agent, 11 client and 1 protocol method; `meta.unstable.json` 28, 12 and 1.
- **The v1 book at the pin.** The 19 pages of `docs/protocol/v1/` in the public repository
  `agentclientprotocol/agent-client-protocol`, at the commit the `schema-v1.25.0` tag names (`4cf3dd8`), fetched with
  `gh api` pinned to that commit. Every sentence holding MUST, MUST NOT, SHOULD, SHOULD NOT, REQUIRED or SHALL was
  extracted with its page and line. A raw keyword count over each file equals the extracted count. `error.mdx` at the
  tag is a stub (`_Documentation coming soon_`); `schema.mdx` is generated and was not read.
- **Normative text in the schema.** The 141 `description` strings in `schema.json` that hold MUST or SHOULD. Most
  (the `_meta` boilerplate, "Implementations MUST NOT make assumptions about values at these keys") bind no behaviour;
  the rest repeat book sentences and are counted with them below.
- **The book upstream now.** The same directory on `main` at `f3ed0fc` (2026-11-10), and `gh release list` for the
  repository, for the section on what has moved since the pin.
- **The code.** Each requirement was traced to the code that enforces it and to a test that pins it: `types_gen.go`
  (`Negotiation.Allows`, `gateMessage`, the generated `gates` methods), `agent_gen.go`, `client_gen.go`,
  `connection.go`, `errors.go`, `helpers.go`, `refusals.go`, `elicitation.go`, `tooltimeline.go`, `toolstream.go`,
  `cmd/generate/internal/emit/dispatch.go` and `gates.go`, `clienthost/`, `internal/tailbuf/`, `remote/doc.go`,
  `unstable/`. Line numbers below are at `108b344`.
- **Tests run.** `go test -race -shuffle=on -count=1 ./...` in the root module and `go test -count=1 ./...` in
  `cmd/generate`, both exit 0. A Go 1.27.3 program, outside the tree, decoded a duplicated JSON key with
  `encoding/json`: the last value won, with no error.
- **Records.** 0015, 0016, 0017, 0019 (question 16), 0021, 0025, 0038, 0039, 0040, 0041, 0043, 0044, 0045, 0049, 0050,
  0051 and 0052, read where the code cites them.

Not done: no interop run against another SDK (`tools/interop.py` exists and was not run); no consumer check; the 55
SHOULD and SHOULD NOT sentences were classified only where named below; who a requirement binds (agent, client or
either) is this report's reading of each sentence's subject.

Nothing in this repository was written but this report and its index row.

## Tree and upstream at this observation

| Fact | Value |
| :--- | :--- |
| HEAD | `108b344` `docs(0031): mark verification tools plan complete`, equal to `origin/main` |
| Newest tag | `v1.35.0` |
| Open issues / pull requests | none / none |
| Records not complete | `0048-MADR-create-releases-in-ci.md` and its PLAN, `proposed`; neither touches the protocol |
| Go floor / toolchain | `go 1.26.10` / `toolchain go1.27.3`; `retract v1.13.0` |
| Schema pin | `1.25.0` |
| Upstream schema | `schema-v1.31.0` is the latest stable release (2026-11-10); `schema-v2.0.0-alpha.15` the latest pre-release |
| Upstream book | `main` at `f3ed0fc` changes normative sentences on 13 of the 19 pages and adds two pages (see below) |

## Answer

**The 1.25.0 schema is fully generated.** All 25 stable methods are on the side connections and the `Agent`,
`AgentLoader` and `Client` interfaces. Every `$defs` type, the 14 `sessionUpdate` variants, the five `StopReason`
values and every capability flag are generated types. `unstable/` holds no declarations at this pin: the unstable
schema adds nothing beyond the frozen manifest.

**The book's MUSTs that a connection can observe are enforced, and tested.** The 1.25.0 book has 105 MUST and MUST NOT
occurrences. This report puts 53 in the SDK's connection or generated code, 2 in the optional `clienthost` package,
4 in part in the SDK and in part in the application, and 46 in the application alone. Of the 46, 20 are on the
elicitation page: they concern consent, privacy and identity, which a user interface and an account system decide.
0016's seven violations are all closed (0017, with 0015 for V4).

**What remains is not a missing method.** The gaps are SHOULD-level client behaviour the connection does not hold
(tool calls marked `cancelled`), hosts the SDK does not ship (an elicitation UI, an MCP HTTP or SSE runtime),
optional parts of a transport RFD outside the stable stdio transport, and later schemas. The deviations are choices
recorded in decisions: lenient decoding on receive, prompt-overlap handling, size and queue limits.

## The 1.25.0 surface

| Piece | Count at the pin | In the SDK |
| :--- | :--- | :--- |
| Agent methods (`meta.json`) | 13 | `Agent` (12, `types_gen.go:24601`), `AgentLoader` (`session/load`); generated `ClientSideConnection` senders |
| Client methods | 11 | `Client` (11); generated `AgentSideConnection` senders |
| Protocol method | `$/cancel_request` | Sent when a request's context ends, handled at once on receipt |
| `$defs` | 178 | Each has a generated type of the same name in `types_gen.go`, with `Validate` and constructors |
| `sessionUpdate` variants | 14 | `SessionUpdate` union, constructors, `ToolCallTimeline` |
| `StopReason` values | `end_turn`, `max_tokens`, `max_turn_requests`, `refusal`, `cancelled` | Closed on send (0049 S6) |
| `AgentCapabilities` | `loadSession`, `promptCapabilities`, `mcpCapabilities`, `sessionCapabilities`, `auth`, `_meta` | Gated (below) |
| `ClientCapabilities` | `fs`, `terminal`, `session` (`compaction`, `notices`, `configOptions.boolean`), `auth`, `elicitation`, `_meta` | Gated (below) |
| `ErrorCode` | -32700, -32600, -32601, -32602, -32603, -32800, -32000, -32002 | A constructor for each (`errors.go`) |

`Agent` requires the optional methods too (`logout`, `session/list`, `session/delete`, `session/resume`,
`session/close`). `AgentRefusals` and `ClientRefusals` (`refusals.go`) answer the optional methods of each side for a
program that does not offer them.

### Capability gates

`Negotiation.Allows` (`types_gen.go:25282-25412`) refuses a method that `initialize` did not advertise, and
`gateMessage` (`types_gen.go:25422-25436`) refuses every message but `initialize` before it is answered, with
`ErrNotInitialized`. The generated `gates` methods refuse a property or a union variant that was not advertised. Both
run on send, as `*CapabilityError` with nothing written, and on receipt, as an error answer with the handler not run.
`TestMethodGates`, `TestValueGates`, `TestResultGates`, `TestNotInitialized` and `TestUndeclaredElicitationMode`
(`gates_test.go`) pin them.

| Stable capability | Gates |
| :--- | :--- |
| `loadSession` | `session/load` |
| `sessionCapabilities.list` / `.delete` / `.resume` / `.close` | the method of each name |
| `sessionCapabilities.additionalDirectories` | a non-empty `additionalDirectories` on `session/new`, `session/load`, `session/resume` |
| `auth.logout` | `logout` |
| `fs.readTextFile` / `fs.writeTextFile` | `fs/read_text_file` / `fs/write_text_file` |
| `terminal` | the five `terminal/*` methods |
| `elicitation.form` / `elicitation.url` | the `elicitation/create` variant of that mode; `elicitation/complete` on `url`; an undeclared mode |
| `promptCapabilities.image` / `.audio` / `.embeddedContext` | an `image`, `audio` or `resource` prompt block, and any undeclared block type |
| `mcpCapabilities.http` / `.sse` | an `http` or `sse` MCP server in a session request |
| `session.notices` / `session.compaction` | the `notice`, `compaction_update` and `compaction_summary_chunk` updates |
| `session.configOptions.boolean` | a `boolean` config option in any result or update that carries options, and a `boolean` `session/set_config_option` |
| `auth.terminal` | a terminal auth method in the `initialize` answer; `authenticate` never names one (`ErrTerminalAuthMethod`) |

`authenticate` must also name a method the `initialize` answer listed (`ErrUnadvertisedAuthMethod`, 0049 S8 and 0050).
The same table gates the unstable capabilities (`nes`, `providers`, `sessionCapabilities.fork`, `subagents`, `plan`,
`mcpCapabilities.acp`).

## The book's requirements, by who must meet them

Each MUST and MUST NOT occurrence in the 19 pages at the tag, counted once per keyword. **SDK** means the connection
or generated code enforces it, or makes it hold by construction. **Host** means the optional `clienthost` package
meets it for a client that uses that package. **Part** means the SDK meets part of it and the application the rest.
**App** means only the program can meet it.

| Page | MUST + MUST NOT | SDK | Host | Part | App |
| :--- | ---: | ---: | ---: | ---: | ---: |
| agent-plan | 2 | 0 | 0 | 0 | 2 |
| authentication | 3 | 3 | 0 | 0 | 0 |
| cancellation | 1 | 1 | 0 | 0 | 0 |
| content | 1 | 0 | 0 | 0 | 1 |
| elicitation | 21 | 1 | 0 | 0 | 20 |
| extensibility | 2 | 2 | 0 | 0 | 0 |
| file-system | 3 | 2 | 1 | 0 | 0 |
| initialization | 9 | 7 | 0 | 1 | 1 |
| overview | 1 | 1 | 0 | 0 | 0 |
| prompt-turn | 11 | 7 | 0 | 2 | 2 |
| session-config-options | 5 | 1 | 0 | 0 | 4 |
| session-delete | 3 | 3 | 0 | 0 | 0 |
| session-list | 8 | 4 | 0 | 0 | 4 |
| session-modes, slash-commands | 0 | 0 | 0 | 0 | 0 |
| session-setup | 24 | 15 | 0 | 1 | 8 |
| terminals | 5 | 2 | 1 | 0 | 2 |
| tool-calls | 1 | 1 | 0 | 0 | 0 |
| transports | 5 | 3 | 0 | 0 | 2 |
| **Total** | **105** | **53** | **2** | **4** | **46** |

### What the SDK enforces

- **Initialization.** A client's `initialize` is stamped with `ProtocolVersionNumber` (1) when it carries none, and
  one that asks for another version is refused before it is sent (`client_gen.go:531-536`). The agent side stamps 1
  on an answer that carries none and refuses to send one that names another version (`agent_gen.go:373-383`). A
  client that receives an answer with another version gets `*ProtocolVersionError`, and no negotiation is recorded
  (`client_gen.go:545-558`), so every later send fails with `ErrNotInitialized`. Omitted capabilities decode as
  unsupported, because the schema defaults are applied on decode. `TestProtocolVersion` (`negotiation_test.go`) and
  `initialize_test.go` pin this.
- **Absolute paths.** `validateOutbound` refuses a relative path on send using the portable rule (`isPortableAbs`).
  `judgeInbound` (`connection.go:3298-3308`) refuses one on receipt with `filepath.IsAbs`. `TestSendRefusesRelativePaths`
  and `TestReceiveRefusesRelativePaths` (`paths_test.go`) pin both. This covers `cwd`, `additionalDirectories` and
  every other path property.
- **Cancellation.** After `session/cancel`, `session/close` or `$/cancel_request` cancels a turn, `promptCase` answers
  `cancelled` whatever `Prompt` returns, and keeps the rest of its result (`dispatch.go:30-38`). `session/close`
  cancels the session's turns as `session/cancel` does (`agent_gen.go:634-650`). A pending
  `session/request_permission` is answered with the `cancelled` outcome once its session is cancelled
  (`client_gen.go:244-248`). A request cancelled by `$/cancel_request` whose handler returns the context's error is
  answered `-32800`. `cancel_test.go` (17 tests, among them `TestPromptCancelledAnswersCancelled`,
  `TestCancelAnswersPendingPermissions` and `TestCloseSessionCancelsTurn`) and `connection_cancel_test.go` pin this.
- **Ordering.** A response waits until every notification queued before it has been processed. So the answer to
  `session/load` follows its replay, and the answer to `session/prompt` follows the updates sent before it. A
  `ToolCallStream` started on a prompt's context is flushed before the turn is answered (`toolstream.go:37-39`).
- **The prompt result.** A `PromptResponse` with an empty or unknown `stopReason` is not sent: the agent's answer
  becomes `-32603` (`TestClosedEnumsRefusedOnSend`, `enum_closed_test.go`).
- **Framing and JSON-RPC.** One message per line, with no embedded newline. Every outbound message carries
  `"jsonrpc":"2.0"`, and an inbound message without it is answered `-32600` (`connection.go:1854`, `:2180`). Invalid
  JSON is answered `-32700` and an invalid request `-32600` (`errors.go:310-316`). A line over the size limit is
  answered `-32600`, and reading goes on (`connection.go:1747-1751`). Go's encoder writes UTF-8. `jsonrpc_test.go`
  pins this.
- **Extensibility.** An `_`-prefixed request with no handler is answered `-32601` with its id, and an unrecognized
  notification is ignored. A declared type encodes only the schema's properties. Only the catch-all `…Other` variants
  of open unions, and the unstable `UnstableMcpError`, carry keys the schema does not declare, and they write them
  back as received (`Extra`, `types_gen.go`).
- **Required fields.** A required field is always encoded, and a nil required array is sent as `[]`. So
  `session/list` with no sessions answers `"sessions":[]`, and `session/load`, `session/resume` and `session/close`
  always carry the fields the book lists. An empty `cwd` is refused because it is not an absolute path. The SDK does
  not check that a required string is non-empty.

### What the optional host meets

- **The file is created if it doesn't exist** (file-system). `clienthost.Files.WriteTextFile` creates it. A missing
  directory is answered `-32002` unless `MkdirAll` is set (`clienthost/files.go:25-31`).
- **Truncation happens at a character boundary** (terminals). `clienthost.Terminals` keeps its output in
  `internal/tailbuf`, which drops bytes from the front at a UTF-8 rune boundary (`internal/tailbuf/tailbuf.go:1-4`,
  `:109-120`).

A client that implements these methods itself meets these two requirements itself.

### Met in part

- **initialization line 54**, the agent answering "with the chosen protocol version and the capabilities it
  supports". The SDK ensures the version. Whether the capabilities are true is up to the agent.
- **prompt-turn line 8**, "complete the initialization phase and session setup" before prompting. `ErrNotInitialized`
  ensures initialization. Session setup is the peer's sequence.
- **prompt-turn line 580**, updates after `session/cancel` must come before the prompt's answer. The SDK orders and
  flushes what was sent before `Prompt` returned. It does not stop an update sent after that.
- **session-setup line 310**, `session/close` must "cancel any ongoing work … then free the resources". The SDK
  cancels the turns. Freeing the resources is the agent's job.

### Left to the application

- **elicitation (20 of 21).** Identifying the requesting agent, the review and consent controls, showing the URL in
  full, opening it in a secure context, keeping secrets out of form mode, keeping `elicitationId` unique, binding to
  the user, and ignoring unknown ids. The SDK checks forms, answers and URLs (`Fields`, `CheckSchema`,
  `ParseElicitationURL`, the opt-in `WithElicitationCheck`, 0040), but these decisions belong to a user interface and
  an account system.
- **agent-plan.** The agent sending the whole plan each time, and the client replacing its copy.
- **Agent obligations behind a session.** A unique session id, replaying history on `session/load` but not on
  `session/resume`, honouring `cwd` as the base and root set, supporting stdio MCP servers, and supporting text and
  `resource_link` prompt content.
- **session-list and session-config-options.** Cursors and `nextCursor` treated as opaque, roots not merged, a
  default value for every option, the complete option list, and categories never required for correctness.
- **terminals and transports.** Releasing a terminal, and nothing on stdout or stdin but ACP messages.

## 0016's violations at this HEAD

| 0016 | Remedy | State now |
| :--- | :--- | :--- |
| V1 High, cancelled turn answered `-32800` | 0017 P1 | Closed: `promptCase` answers `cancelled` (above) |
| V2 Medium, malformed input unanswered | 0017 J1 | Closed: `-32700`, `-32600`, `jsonrpc` checked, overlong lines answered |
| V3 Medium, capabilities not enforced | 0017 G1 | Closed: the gates above, on send and on receipt |
| V4 Medium, relative paths sendable | 0015 | Closed: `validateOutbound` |
| V5 Medium, no version negotiation | 0017 N1 | Closed on both sides; the client's SHOULD close is the program's |
| V6 Medium, pending permissions on cancel | 0017 Q1 | Closed for the MUST; the SHOULD on tool calls is open (below) |
| V7 Minor, no `-32002` constructor | 0017 K1 | Closed: `NewResourceNotFound` (`errors.go:344-349`) |

0016's notes, now:

- The 10 MiB inbound cap is 32 MiB (`connection.go:37-41`, 0052 D6). An overlong line is answered, not closed on.
- The outbound `$/cancel_request` queue still holds 1024 (`connection.go:1456`) and still drops a cancel at Debug
  level when full (`connection.go:3000-3008`).
- Prompt overlap is now a policy (0021). `PromptOverlapSupersede`, the default, cancels the running turn.
  `PromptOverlapRefuse` answers the new prompt `-32603` (`connection.go:756-768`).

## Gaps: what a full 1.25.0 program still writes

None of these is a missing method or type.

1. **Tool calls marked `cancelled` on `session/cancel`.** A SHOULD, prompt-turn line 563. The client connection keeps
   no tool-call state. `ToolCallTimeline.Open()` (`tooltimeline.go:125`) lists the unfinished calls of a timeline the
   client keeps, but nothing sends or applies the marks for it.
2. **An elicitation host.** `clienthost` has `Files` and `Terminals`. It has no form or URL host, because the MUSTs in
   question are about a user interface.
3. **MCP over HTTP and SSE.** The `McpServer` `http` and `sse` variants and their gates are types only. No MCP client
   runtime ships, and an agent connects to MCP servers itself. `ResolveStdioCommands` with `WithInboundRewrite`
   resolves a bare stdio command, which the absolute-path MUST otherwise refuses.
4. **Closing on a version mismatch.** A SHOULD, initialization line 98. The SDK returns `*ProtocolVersionError` and
   records no negotiation. Closing the connection and telling the user is the program's job.

## Deviations: what the SDK chose

| Behaviour | Spec at 1.25.0 | SDK | Record |
| :--- | :--- | :--- | :--- |
| Receiving a value a closed enumeration does not list | Silent; the schema marks some fields default-on-error | Kept in a required field; a lenient field reads it as absent. Refused on send | 0049 S6, Q4 (a); `connection.go:3306-3308` |
| Second `session/prompt` for a running session | Silent | Supersede by default, or refuse with `-32603` | 0021 |
| Inbound message size | Silent | 32 MiB default, `WithMaxMessageSize` | 0052 D6 |
| Notification queue full | Silent | `OverflowBlock` default since `v1.29.0`; 64 MiB byte budget since `v1.30.0` | 0044, 0045 |
| JSON-RPC batch | v1 states individual messages | `-32600`; `remote` answers a batch POST `501` | 0016 note; 0043 |
| Duplicate JSON keys | Silent | Last value wins (`encoding/json`) | 0016 note |
| Prompt and auth constructors | Not required | `NewPromptResponse`, `AuthenticateAdvertised` and the like are not provided | 0050 D8 |

## Outside 1.25.0 stable

- **The unstable half of the pin.** `session/fork`, `providers/*`, `nes/*`, `document/*` and `mcp/message` are
  generated as `Unstable*` methods and fields in package `acp`, deprecated and frozen under 0039. They include
  stable-named fields such as `AgentCapabilities.Nes` (`types_gen.go:102`) and unstable `SessionUpdate` variants
  (`plan_update`, `subagent_update` and others). 0025 counted 115 incompatible changes if a later schema is generated
  into this v1 module.
- **The remote transport.** Package `remote` implements the Active RFD "Streamable HTTP & WebSocket Transport" at its
  revision of 2026-07-02, as experimental API (`remote/doc.go`). These are not supported (`README.md`, "Not
  supported"):
  - resuming with `Last-Event-ID`, and reconnecting;
  - batches;
  - WebSocket subprotocols and compression;
  - WebSocket over HTTP/2;
  - Unix sockets.

  The v1 book's only transport is stdio.
- **Upstream since the pin.** Stable schemas `1.26.0` to `1.31.0` have been released. The book on `main` changes
  normative sentences on 13 of the tag's pages. It adds two pages: `security.mdx`, which is 23 SHOULD or SHOULD NOT sentences, and a full
  `error.mdx`, with one MUST: "Receivers MUST treat an unrecognized code as a generic failure". Its new requirements
  include:
  - a `systemPrompt` section with seven MUST or MUST NOT sentences;
  - `-32002` treated as success for `session/close` and `session/delete`;
  - `-32602` for an invalid cursor;
  - root-set and launch-safety SHOULDs for files, terminals and auth.

  None of these binds a 1.25.0 implementation. 0025 is the repository's only assessment of a later schema.

## What a later record could take

- Gap 1: a client helper that marks a turn's open tool calls `cancelled`. It would be additive under v1.
- Gap 2: an elicitation host in `clienthost`. No record has taken it up.
- Gap 3: an MCP HTTP and SSE runtime. No record has taken it up. 0043 chose the remote transport over an
  MCP-over-ACP binding, which is a different feature.
- The receive-side choice: an opt-in strict receive, which would change 0049 Q4.
- `remote` resume: an amendment to 0043, which is 0051 item 2.
- Later schemas and the unstable boundary: 0019 question 16 and 0025, which is 0051 item 3.

This report does not choose among them.

## Sources

In this repository:

- `schema/version`, `schema/schema.json`, `schema/meta.json`, `schema/meta.unstable.json`
- `types_gen.go`, `agent_gen.go`, `client_gen.go`, `connection.go`, `errors.go`, `helpers.go`, `refusals.go`,
  `elicitation.go`, `tooltimeline.go`, `toolstream.go`
- `cmd/generate/internal/emit/dispatch.go`, `cmd/generate/internal/emit/gates.go`
- `clienthost/files.go`, `clienthost/terminals.go`, `internal/tailbuf/tailbuf.go`, `remote/doc.go`, `unstable/doc.go`,
  `README.md`
- `gates_test.go`, `negotiation_test.go`, `initialize_test.go`, `cancel_test.go`, `connection_cancel_test.go`,
  `jsonrpc_test.go`, `paths_test.go`, `enum_closed_test.go`

Records:

- [0015-MADR-validate-absolute-protocol-paths.md](../decisions/0015-MADR-validate-absolute-protocol-paths.md)
- [0016-REPORT-acp-v1-conformance-audit.md](0016-REPORT-acp-v1-conformance-audit.md)
- [0017-MADR-remediate-acp-v1-conformance-violations.md](../decisions/0017-MADR-remediate-acp-v1-conformance-violations.md)
- [0017-PLAN-remediate-acp-v1-conformance-violations.md](../decisions/0017-PLAN-remediate-acp-v1-conformance-violations.md)
- [0019-MADR-remediate-debugging-pass-findings.md](../decisions/0019-MADR-remediate-debugging-pass-findings.md)
- [0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](../decisions/0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md)
- [0025-REPORT-schema-1-26-0-adoption-assessment.md](0025-REPORT-schema-1-26-0-adoption-assessment.md)
- [0039-MADR-five-adopter-additions-on-schema-1-25-0.md](../decisions/0039-MADR-five-adopter-additions-on-schema-1-25-0.md)
- [0040-MADR-check-elicitation-forms-answers-and-urls.md](../decisions/0040-MADR-check-elicitation-forms-answers-and-urls.md)
- [0041-MADR-close-the-gaps-the-consumers-wait-on.md](../decisions/0041-MADR-close-the-gaps-the-consumers-wait-on.md)
- [0043-MADR-remote-http-and-websocket-transport.md](../decisions/0043-MADR-remote-http-and-websocket-transport.md)
- [0044-MADR-notification-queue-back-pressure.md](../decisions/0044-MADR-notification-queue-back-pressure.md)
- [0045-MADR-notification-queue-byte-budget.md](../decisions/0045-MADR-notification-queue-byte-budget.md)
- [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](../decisions/0049-MADR-remediate-the-2026-11-01-debugging-pass.md)
- [0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md](../decisions/0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md)
- [0051-REPORT-acp-1-25-0-featureset-coverage-and-three-extensions.md](0051-REPORT-acp-1-25-0-featureset-coverage-and-three-extensions.md)
- [0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md](../decisions/0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md)

Upstream, the public repository `agentclientprotocol/agent-client-protocol`:

- `docs/protocol/v1/*.mdx` at `4cf3dd8` (tag `schema-v1.25.0`) and at `main` `f3ed0fc`
- its release list, read 2026-11-10

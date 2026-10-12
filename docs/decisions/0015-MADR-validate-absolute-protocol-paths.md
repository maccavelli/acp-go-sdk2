---
status: accepted
date: 2026-10-12
decision-makers: Repository owner
---

# The SDK sends no path ACP requires to be absolute unless it is, refuses one it receives by default, and lets consumers rewrite, extend or relax that

## Context and Problem Statement

GitHub issue #2, "v0.15.0 Validate no longer refuses an empty cwd on session/new, load and resume; a relative cwd was never refused",
opened by the owner on 2026-10-12, reports a regression with security weight and a conformance gap. This record checks the issue against
the code and the ACP specification, maps every surface it touches, and decides the remedy. Facts were read on `23d03b5` (the 0014 step 5
commit) on 2026-10-12; the ACP sources are the specification at <https://agentclientprotocol.com> and the upstream repository
`agentclientprotocol/agent-client-protocol` at `d9d2f5204062`.

### What the issue reports, checked

* **The regression is real and documented in aggregate only.** `v0.14.0`'s `(*NewSessionRequest).Validate` began `if v.Cwd == "" { return
  fmt.Errorf("cwd is required") }`; at `v0.15.0` and at `23d03b5` it validates only each element of `McpServers`.
  `(*ReadTextFileRequest).Validate` went from `if v.Path == "" { return fmt.Errorf("path is required") }` to `return nil`. A script over
  every `Validate` body of `git show
  v0.14.0:types_gen.go` counts exactly 28 empty-string checks, and none survives at `23d03b5`: the `Method` of `AgentRequest`,
  `AgentNotification`, `ClientRequest`, `ClientNotification`, `UnstableMessageMcpRequest` and `UnstableMessageMcpNotification`; the
  `Message` of the three `CreateElicitationRequest` variants; `CreateTerminalRequest.Command`; the `Cwd` of `NewSessionRequest`,
  `LoadSessionRequest`, `ResumeSessionRequest` and `UnstableForkSessionRequest`; `ReadTextFileRequest.Path`; `WriteTextFileRequest.Path` and
  `.Content`; `ReadTextFileResponse.Content`; `TerminalOutputResponse.Output`; the `Uri` of five `UnstableDid…DocumentNotification` types,
  with `Text` and `LanguageId` on open; `UnstableSuggestNesRequest.Uri`; `UnstableSetProviderRequest.BaseUrl`. The removal followed
  [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), outcome 7 ("`Validate` checks what the schema states and
  nothing else ... The `""` and `nil` checks go"); `docs/reports/0013-REPORT-release-notes-v0-15-0.md` said "It no longer rejects a present
  empty string; `v0.14.0` did, in 28 places" and named none of them.
* **A relative path was never refused.** No version's generated code calls `filepath.IsAbs`, and no `Validate` compares a path with
  anything but `""`.
* **A missing key is still refused.** The generated `UnmarshalJSON` requires every required key (`objectKeys`), answered `-32602`; only
  a present empty or relative value gets through.
* **The impact is downstream and real.** The issue reports a downstream agent that keeps no workspace roots for an empty `cwd` and treats
  no roots as no confinement; on `v0.15.0`, an empty `cwd` gave a session in which a system directory counted as inside the workspace.

### What ACP requires

* Overview: "All file paths in the protocol **MUST** be absolute."
* Session setup, working directory: `cwd` "**MUST** be an absolute path", "**MUST** be used for the session regardless of where the Agent
  subprocess was spawned", "**MUST** remain the base for relative-path resolution", "**MUST** be part of the session's effective root set",
  and "This root set **SHOULD** serve as a boundary for tool operations on the file system". Additional workspace roots: "each
  `additionalDirectories` entry **MUST** be an absolute path".
* File system: `path` of `fs/read_text_file` and `fs/write_text_file` is the "Absolute path to the file to read" and "to write".
  Terminals: `cwd` of `terminal/create` is the "Working directory for the command. Must be an absolute path."
* The specification says nothing about what a receiver does with a path that is not absolute.
* The schema states it only in prose. In `schema/schema.json` (ACP `1.25.0`) 15 properties, and in `schema/schema.unstable.json` 17 (adding
  `ForkSessionRequest.cwd` and `.additionalDirectories`), carry "absolute" in their description, each a plain `string`, `string | null`, or
  array of strings, with no `minLength`, `format` or `pattern`.
* Upstream keeps it out of the machine-readable schema too. The v1 Rust schema types `cwd` as `PathBuf` with the doc comment "Must be an
  absolute path" (`agent-client-protocol-schema/src/v1/agent.rs`). The v2 schema introduces `pub struct AbsolutePath(pub PathBuf)`, a
  `serde(transparent)` wrapper with no check on deserialize (`agent-client-protocol-schema/src/v2/mod.rs`), emitted in
  `schema/v2/schema.json` as `{"description": "An absolute filesystem path used by the protocol.", "type": "string"}` and referenced from 14
  properties. The v2 draft keeps "All file paths in the protocol **MUST** be absolute." GitHub code search of the upstream repository finds
  no `is_absolute` call.

### The surfaces

Every property the schema files describe as absolute, in the generated SDK (which merges both files), by who receives it and whether the
SDK validates it on arrival. The 17 schema properties are 18 Go fields: the generator emits `Diff`'s properties twice, in `Diff` and in
`ToolCallContentDiff`, the inline variant `ToolCallContent` builds from `allOf: [{"$ref": "#/$defs/Diff"}]`, which is the type agents
send diffs in.

| Property | Message, direction | SDK validates on arrival |
| --- | --- | --- |
| `NewSessionRequest.cwd`, `.additionalDirectories[]` | `session/new`, client to agent | yes (`agent_gen.go`) |
| `LoadSessionRequest.cwd`, `.additionalDirectories[]` | `session/load`, client to agent | yes |
| `ResumeSessionRequest.cwd`, `.additionalDirectories[]` | `session/resume`, client to agent | yes |
| `UnstableForkSessionRequest.cwd`, `.additionalDirectories[]` | `session/fork` (unstable), client to agent | yes |
| `ListSessionsRequest.cwd` (optional) | `session/list`, client to agent | yes |
| `McpServerStdio.command` | inside `mcpServers` of the session requests | yes, through the parent's `Validate` |
| `ReadTextFileRequest.path` | `fs/read_text_file`, agent to client | yes (`client_gen.go`) |
| `WriteTextFileRequest.path` | `fs/write_text_file`, agent to client | yes |
| `CreateTerminalRequest.cwd` (optional) | `terminal/create`, agent to client | yes |
| `ToolCallLocation.path`, `Diff.path` (and `ToolCallContentDiff.path`) | inside `session/update`, agent to client | yes, and a failure drops the whole notification |
| `SessionInfo.cwd`, `.additionalDirectories[]` | `session/list` response, agent to client | no: responses are not validated |

* Both sides decode and validate every inbound request and notification before dispatch: `json.Unmarshal`, then `p.Validate()`, each
  failure answered with `NewInvalidParams(map[string]any{"error": err.Error()})`, `-32602`. A notification has no response;
  `connection.go` logs `failed to handle notification` and drops it. Outbound requests and every response are not validated.
* Validation propagates: under 0012, a type validates every field whose type validates, so a check placed in `McpServerStdio` or
  `ToolCallLocation` runs from every message that holds one.
* Interoperability evidence for `McpServerStdio.command`: Zed's ACP client builds it as `acp::McpServerStdio::new(id.0.to_string(),
  &command.path)` from the user's context-server configuration (`mcp_servers_for_project` in `crates/agent_servers/src/acp.rs` at
  `725573ec14a2`); this record found no step there that resolves the command, so a configuration that names a bare command such as `npx`
  sends it as written.
* Cross-host evidence: the same function sends MCP servers when the project is local or the server is marked `remote` (`is_local ||
  *remote`), so Zed runs agents for remote projects; a client and an agent on one connection need not share an operating system, and a
  sender cannot know whether its peer reads `/home/u/project` or `C:\Users\u\project` as absolute.
* The SDK's own helpers build path-carrying values: `ToolDiffContent(path, …)`, `StartReadToolCall(…, path, …)`,
  `StartEditToolCall(…, path, …)`, `WithStartLocations` and `WithUpdateLocations` (`helpers.go`); they do not check the path.
* The repository's tests run on `ubuntu-24.04`, `macos-15` and `windows-2025` (`.github/workflows/ci.yml`), and pass POSIX literals such
  as `"/test.txt"` and `"/home/user/project/src/config.json"` through connections (`acp_test.go`) and helpers (`json_parity_test.go`,
  `fidelity_test.go`, `raw_value_test.go`, `example_agent_test.go`); `filepath.IsAbs("/test.txt")` is false on Windows.
* The repository's own example clients repeat the check by hand: `example/client/main.go`, `example/claude-code/main.go`,
  `example/gemini/main.go` and `example_client_test.go` each refuse `!filepath.IsAbs(params.Path)` in `ReadTextFile` and `WriteTextFile`.
  The example agents (`example/agent/main.go`, `example_agent_test.go`) ignore `cwd`.
* Where such a rule would live: `cmd/generate/internal/model/acp.go` holds `ACP`, "the behaviour of the Agent Client Protocol that its
  schema does not express. Each rule names the specification it follows; Build checks every rule against the schema it applies to, and
  `TestProtocolTableAppliesToCommittedSchema` requires every rule to apply to the schema in the repository." A `Validate` check is a
  `model.Check` with a `Kind` and a `Shape` (`RefNamed`, `RefPointer`, `RefSlice`, `RefMap`), emitted by `checkStmt` in
  `internal/emit/types.go`; a new kind reuses the shapes.
* Confinement is not the SDK's: the root set "SHOULD serve as a boundary", which only the agent or client can enforce on its own files.
  The module's floor is `go 1.27.2`, so every user has `os.OpenRoot` (Go 1.24), the standard library's traversal-resistant access confined
  to a directory.

### The open items, against the current standard

The owner asked, on 2026-10-12, that the items a first draft of this record left open be settled with the current ACP standard as the
baseline, its compliance enforced, and that the codebase be evaluated for flexibility consumers can use. Findings:

* **ACP's path MUSTs bind what is sent.** "All file paths in the protocol MUST be absolute"; "Clients MUST only send `additionalDirectories`
  when the Agent advertises `sessionCapabilities.additionalDirectories`". The specification does not say what a receiver does with a path
  that is not absolute. So the SDK's conformance is first a property of what it sends, and what it does with a peer's violation is policy.
* **The SDK validates nothing it sends.** `SendRequest`, `SendRequestNoResult` and `SendNotification` (`connection.go`) marshal `params` and
  write them; a handler's result is marshalled and sent (`connection.go`, after `c.handler(...)`). A Go client on this SDK can send `cwd:
  "relative/dir"`, and a Go agent can answer `session/list` with a relative `SessionInfo.cwd` or stream a relative `ToolCallLocation.path`,
  all against the standard.
* **The SDK validates no response it receives.** `SendRequest` decodes the result into its Go type, whose `UnmarshalJSON` refuses a
  missing required key, but calls no `Validate`; a `session/list` result with a relative `SessionInfo.cwd` reaches the caller unchecked.
* **Every property the standard covers can be enforced on send, if absolute is read portably.** All 17 properties are the SDK's own
  output when it is the sender; refusing to send a relative path costs no interop. But a sender cannot know its peer's operating system
  (the cross-host evidence above), so a send-side check by the sender's own rule would refuse a Windows client's `/home/u/project` for a
  WSL or SSH agent, with no way around it. The interop costs found above (Zed forwarding a bare MCP command name; a relative tool-call
  location dropping a `session/update`) arise only when the SDK refuses what a peer sent.
* **The 22 other checks `v0.15.0` removed are not in the standard.** Each property is a plain `{"type": "string"}` in both schema files
  (`Method` "The method name to invoke"; `Message` "A human-readable message describing what input is needed"; `Command` "The command to
  execute"; `Uri` "The URI of the opened document"; `BaseUrl` "Base URL for requests sent through this provider"; `Content`, `Text`,
  `LanguageId`, `Output`), and the specification states no non-empty rule for any of them. An empty JSON-RPC method already fails dispatch
  with `-32601`.
* **Capability gates are recorded, not enforced.** The protocol table records 14 method gates ("Capability gates the schema describes,
  recorded, not acted on", `acp.go`), and neither connection keeps what `initialize` negotiated: `ClientSideConnection.LoadSession` sends
  `session/load` whatever the agent advertised. Several gates are MUSTs ("Clients MUST NOT call session/load unless the Agent advertises
  loadSession"; an Agent "MUST NOT attempt to call the corresponding filesystem method" when the client does not advertise it, and "MUST
  NOT attempt to call any terminal methods" without the terminal capability), as is the `additionalDirectories` gate above. This is a
  conformance gap beyond paths.
* **Upstream offers no machine-readable form to adopt.** v1 has prose only; v2's `AbsolutePath` is a named string with a description. The
  current standard is enforced from the protocol table without an upstream change.
* **Whether to publish an advisory** is not a question the standard answers; it is the maintainer's process.

### The flexibility the codebase offers today

* Connection options: `WithMaxQueuedNotifications`, `WithNotificationOverflowPolicy`, `WithNotificationDropHandler`, `WithLogger`
  (`connection.go`), applied by `NewAgentSideConnection`, `NewClientSideConnection` and `NewConnection`; `SetLogger` at run time.
* Escape hatches: `NewConnection` takes any `MethodHandler`, `SendRequest[T]` sends any method, `CallExtension` and `NotifyExtension` with
  `ExtensionMethodHandler` carry `_`-prefixed extensions, and every generated type's `Validate` is exported.
* Construction: the hand-written helpers in `helpers.go` and, since 0014, a constructor for every union variant.
* Gaps, each found in the code:
  * **Validation cannot be extended or observed.** Inbound dispatch calls `p.Validate()` and answers `-32602`; nothing lets an application
    add a rule (a path inside its roots, a non-empty command), relax one for a peer it trusts, or learn that a notification was dropped
    except through the log line `failed to handle notification`.
  * **Validation errors are text.** Every generated check returns `fmt.Errorf("NewSessionRequest: ...")` or `errors.New(...)`; a caller
    cannot tell which rule or property failed without parsing the message. The kinds are: a nested type's error wrapped, `minLength`,
    minimum, maximum, exactly one variant set, exactly one member set, and a catch-all holding an excluded value (`checkStmt`,
    `unionValidateBody` and the members check in `internal/emit/types.go`).
  * **`Validate` has one meaning of everything.** It takes no arguments, so any rule that must differ by direction, as absoluteness does
    across hosts, needs a way for the caller to choose.
  * **The session root set has no accessor.** The specification defines it as `[cwd, ...additionalDirectories]`; each agent assembles it
    from two fields on four request types and `SessionInfo`.
  * **No middleware.** There is no hook around sending or receiving for tracing, metrics, authorization or rewriting; the closest Go
    precedent, the MCP Go SDK (`modelcontextprotocol/go-sdk` v1.8.0), has `type Middleware func(MethodHandler) MethodHandler` with
    `AddReceivingMiddleware` and `AddSendingMiddleware`, and this SDK already exports a `MethodHandler`.
  * **Negotiated capabilities are not kept**, so an application cannot ask the connection what the peer advertised.

**Question:** should the SDK refuse a protocol path that is not absolute, and if so, which paths, in which direction, by which meaning of
absolute, implemented where; what may a consumer adjust or rewrite; and what should the release notes and documentation say about the 28
checks `v0.15.0` removed?

## Decision Drivers

* **Conform to the current standard, enforced.** "All file paths in the protocol MUST be absolute" is normative. The schema carries it only
  in prose, so 0012's rule ("what the schema states") left it unchecked; the specification is the standard the SDK implements. Conformance
  is not optional: nothing the SDK sends may break it.
* **Secure by default at the boundary the SDK owns.** The SDK already refuses malformed inbound requests with `-32602` before the handler
  runs; an empty or relative path that sets or targets a filesystem scope is where an unintended scope begins.
* **Never refuse to send what is conforming.** A send-side check has no opt-out, so it must not refuse a path that is absolute for the
  peer, whose operating system the sender cannot know.
* **Adoptable: let the consumer decide what the standard leaves open.** The standard binds senders and leaves a receiver's response to a
  violation unspecified. An application that must interoperate with a peer that sends a bare MCP command name, or a path in another host's
  form, should be able to accept, translate or resolve it deliberately, and an application with stricter needs should be able to add rules,
  without forking the SDK.
* **Handlers see conforming messages.** Whatever a consumer adjusts, a handler should receive a message the SDK has validated, unless the
  consumer explicitly accepted a violation.
* **Function-rich and inspectable.** A consumer should be able to tell which rule and property failed, and get the session's root set,
  without parsing text or reassembling fields.
* **Protocol knowledge lives in the protocol table**, with its citation, checked against the schema, not in heuristics over descriptions
  ([0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md), "Heuristics and hard-coded protocol knowledge",
  F11 and F12, which led to the table).
* **Idiomatic Go.** `path/filepath.IsAbs`, `errors.As`, functional options, and `os.OpenRoot` are the standard library's and the
  repository's idioms.
* **One decision per record.** Findings that are their own decision (capability gates, middleware) are recorded and sent to their own
  records rather than decided here.
* **Say what changed.** A release that removes 28 checks names them, and the one with security weight is flagged.

## Considered Options

Where the rule lives:

* W1. A protocol-table rule: the generator emits a `Validate` check for each listed path property.
* W2. A hand-written helper that agents and clients opt into (`acp.ValidateCwd`, or `IsAbsolutePath`).
* W3. Restore `v0.14.0`'s non-empty checks.
* W4. Documentation only (the issue's option 1).
* W5. Derive the rule from the schema's descriptions (any property whose description says "absolute").

Which properties:

* F1. Every property the standard requires to be absolute: 17 schema properties, 18 Go fields.
* F2. The request properties that set or target the receiver's filesystem scope: `cwd` and `additionalDirectories` of `session/new`,
  `session/load`, `session/resume` and `session/fork`; the `cwd` filter of `session/list`; `path` of `fs/read_text_file` and
  `fs/write_text_file`; `cwd` of `terminal/create`. Twelve properties.
* F3. The session `cwd` alone (the issue's four types).

In which direction:

* E1. Everything the SDK sends is validated, with no way to turn it off; everything it receives (requests, notifications, and responses)
  is validated by default, and the consumer's hooks adjust a received message.
* E2. Received requests and notifications only, as today.
* E3. Sent and received, with no hook.
* E4. Received only, with a switch that turns validation off.

What "absolute" means:

* A1. `filepath.IsAbs` on the host that applies the check, in both directions.
* A2. Absolute in POSIX or in Windows form, in both directions.
* A3. Non-empty only.
* A4. By direction: what the SDK sends must be absolute in POSIX or Windows form; what it receives must be absolute by the receiving
  host's `filepath.IsAbs`.

How a consumer adjusts a received message:

* H1. Two hooks: a rewrite hook that may change the decoded message before the SDK validates it, and a validation hook that decides the
  outcome after, given the SDK's result.
* H2. One hook that may change the message and decides the outcome; the SDK validates again after it.
* H3. One hook that decides the outcome and may not change the message.
* H4. One hook that may change the message and decides the outcome; nothing is validated after it.

What a consumer gets besides the hooks:

* X1. A typed `*ValidationError`, and root-set accessors.
* X2. Nothing beyond the exported `Validate` methods.
* X3. General sending and receiving middleware, decided here.

What is said about the removed checks:

* C1. Name the 28 in the next release notes, flag `cwd` and `path`, add an erratum to the `v0.15.0` record and release, publish a
  GitHub security advisory for `v0.15.0`, document the path rule and confinement in the README, and answer issue #2.
* C2. Name them in the next release notes only.

## Decision Outcome

Chosen options: "W1", "F1", "E1", "A4", "H1", "X1" and "C1", accepted by the owner on 2026-10-12 as written, with the order, the advisory's
timing and the follow-up records decided the same day (see "Decided with the acceptance" below).

* **W1**, because it makes the SDK enforce what the standard requires at the boundary where it already enforces the schema, with no code in
  each application; the rule sits in the protocol table with its citation and is checked against the schema like every other protocol rule.
  W2 leaves every application to remember the check, as the examples show it must today; W3 and A3 refuse `""` and still accept
  `relative/dir`; W4 leaves the regression; W5 is a description heuristic of the kind 0011 found in F11 and F12.
* **F1**, because the standard covers all 17 and E1 with H1 removes the reasons the first draft chose F2: what the SDK sends must conform,
  at no interop cost once absolute is read portably, and a received violation's outcome is the consumer's, so a peer that forwards a bare
  MCP command name can be accepted or resolved deliberately rather than by the SDK leaving the property unchecked for everyone.
* **E1**, because conformance is a property of what the SDK sends, which no option may weaken, while a received violation is a case the
  standard leaves to the receiver: refuse by default, as security asks, and let the consumer decide otherwise. E2 sends non-conforming
  messages and accepts non-conforming responses; E3 makes interop with non-conforming or cross-host peers impossible without a fork; E4 lets
  one switch turn off every check, schema ones included.
* **A4**, because each direction needs a different reading. A sender cannot know its peer's operating system, and its check has no
  opt-out, so it accepts a path absolute in either form, refusing only what is absolute nowhere: `""`, `relative/dir`, `./x`, `\x`, `C:x`.
  A receiver uses the path on its own filesystem ("MUST be used for the session"), so there absolute means `filepath.IsAbs`, the idiom and
  the examples' check; on Linux `C:\proj` is a relative file name, the issue's hazard. A1 would refuse a Windows client's `/home/u/project`
  for a WSL or SSH agent with no recourse; A2 would let `C:\proj` through to a Linux agent as a relative path.
* **H1**, because it is the only shape in which both adjustments are possible and the handler still sees what the SDK validated: the
  rewrite hook runs first (resolve `npx` with `exec.LookPath`, translate `/mnt/c/...` to `C:\...`), the SDK validates the result, and the
  validation hook sees that result and may accept a violation the consumer chooses to keep. H2 cannot accept a violation the hook keeps,
  since validating again refuses it; H3 cannot translate or resolve, so each handler must; H4 hands a handler whatever a buggy hook leaves.
* **X1**, because it gives a consumer what the gaps above lack, each small and in the repository's idiom, without deciding the wider
  middleware question here. X2 leaves interop and stricter policies to forks; X3 is a decision about every message, not paths, and goes to
  its own record.
* **C1**, because the `v0.15.0` notes counted the removed checks without naming them, one carried security weight, and a downstream agent
  was affected.

Concretely:

1. **Protocol table.** `model.Protocol` gains `AbsolutePaths`, rules of a definition and a property, each with the specification sentence
   it follows, listing the 17 properties of F1 by definition as the merged schema names them (`NewSessionRequest`, `LoadSessionRequest`,
   `ResumeSessionRequest`, `UnstableForkSessionRequest`, `ListSessionsRequest`, `SessionInfo`, `McpServerStdio`, `ReadTextFileRequest`,
   `WriteTextFileRequest`, `CreateTerminalRequest`, `ToolCallLocation`, `Diff`). A rule applies to every generated type that holds its
   definition's property: the definition's own type, and an inline variant built from it through `allOf`, so `Diff`'s rule covers
   `ToolCallContentDiff`. `Build` refuses a rule whose property is not a string, an optional string, or an array of strings, and applies a
   rule only where the schema has its definition and property, as the table's other rules do (made explicit 2026-10-12, in step 4 of the
   PLAN: a rule whose property the schema lacks is skipped); `TestProtocolTableAppliesToCommittedSchema` requires each
   to apply to the committed schema.
2. **The check, by direction.** A new `model.CheckAbsolutePath` kind checks a string, an optional one when set, and each element of an
   array, with a rule the caller chooses. The generated code carries the body of each `Validate` in an unexported method that takes that
   rule and passes it to nested types; `Validate()` stays exported and applies the receiving rule, `filepath.IsAbs`. The SDK applies the
   sending rule, a hand-written `isPortableAbs`: a path that begins with `/`, or with a drive letter, a colon and a separator (`C:\`,
   `C:/`), or with two backslashes (`\\server\share`, `\\?\C:\`). Types without a path rule are unchanged in behaviour.
3. **A typed error.** Every generated check returns a `*ValidationError`, a hand-written type in `errors.go` with `Type` (the Go type),
   `Field` (the JSON key, with an index for an element, `additionalDirectories[1]`), `Value` (the offending value, when a scalar), and
   `Rule` (a `ValidationRule` constant: `RuleAbsolutePath`, `RuleMinLength`, `RuleMinimum`, `RuleMaximum`, `RuleOneVariant`,
   `RuleOneMember`, `RuleExcludedValue`). Its `Error` keeps today's text, so messages do not change, with the absolute-path one reading
   `NewSessionRequest: cwd: "" is not an absolute path`, the value quoted with `%q`; a parent's wrapping keeps it reachable with
   `errors.As`.
4. **Sending (E1, A4).** The generated methods that send (`ClientSideConnection.NewSession`, `AgentSideConnection.SessionUpdate`, and the
   rest) validate their params with the sending rule and return a failure without writing; `SendRequest`, `SendRequestNoResult` and
   `SendNotification` do the same for params of a generated type. A handler's result that fails is not sent: the peer gets `-32603` with
   `data.error` naming the type and property, and the connection logs it. There is no option to skip this. Extension methods
   (`_`-prefixed) carry untyped params and are not validated. The hand-written helpers that take a path (`ToolDiffContent`,
   `StartReadToolCall`, `StartEditToolCall`, `WithStartLocations`, `WithUpdateLocations`) say in their documentation that it must be
   absolute.
5. **Receiving (E1, A4, H1).** Each received request, notification and response is handled in this order:
   1. decoded, as today; a decoding failure (malformed JSON, a missing required key) is answered as today and reaches no hook;
   2. passed to the rewrite hook, `WithInboundRewrite(func(ctx context.Context, method string, params any) error)`, with `params` a pointer
      to the decoded value, which the hook may change;
   3. validated with the receiving rule;
   4. passed to the validation hook, `WithInboundValidation(func(ctx context.Context, method string, params any, err error) error)`, with
      `err` the result of step 3 (nil when valid); the hook returns the error to act on, or nil to accept. `params` is the same pointer,
      and this hook must not change it.
   5. acted on: with no error, dispatched (a request or notification) or returned (a response); with an error, a request is answered with it
      (a `*RequestError` as it is, any other error as `-32602` with `data.error`), a notification is dropped and logged, and a response's
      error is returned to the caller, wrapped so that `errors.As` finds the `*ValidationError` when the error is the SDK's own.

   Without hooks the SDK acts on its own result. A hook that accepts a violation takes responsibility for it; the README says so, and
   that a handler then sees what the rewrite hook produced, not what the peer sent.
6. **Root set.** Hand-written methods `Roots() []string` on `NewSessionRequest`, `LoadSessionRequest`, `ResumeSessionRequest`,
   `UnstableForkSessionRequest` and `SessionInfo` return `[cwd, ...additionalDirectories]`, the specification's effective root set, in
   order. The SDK offers no containment check of its own: a lexical one misses symbolic links, and `os.OpenRoot` per root is the standard
   library's traversal-resistant way; the README shows it.
7. **0012 amended.** [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), outcome 7, gains a second source of
   checks, the protocol's normative requirements the schema states only in prose, each listed in the protocol table with its citation;
   `Validate` now runs on what the SDK sends and on the responses it receives; and the path rule's meaning depends on the direction. `""`
   is still accepted wherever no rule applies.
8. **Documentation.** README, "Wire encoding": the path rule and its two readings, both directions, the two hooks and what accepting or
   rewriting means, the root set, and confinement with `os.OpenRoot`; that `McpServerStdio.command` must be absolute when sent, so a
   client resolves a bare name with `exec.LookPath` first; and that a deployment whose client and agent run on different operating systems
   installs a rewrite hook to translate paths. The example clients drop their hand-written `filepath.IsAbs` checks as redundant, and the
   example agent keeps its session's root set.
9. **Release notes, erratum and advisory.**
   * The next release's notes list the path checks and the two new directions under changes to what the SDK accepts and sends, with a
     security note, and name the 28 empty-string checks `v0.15.0` removed.
   * The `v0.15.0` record gains an erratum: an addendum section in `docs/reports/0013-REPORT-release-notes-v0-15-0.md` naming the 28 and
     flagging `cwd` and `path`, and the same text appended to the published `v0.15.0` GitHub release by the owner.
   * The owner publishes a GitHub security advisory for `v0.15.0`, affected `v0.15.0`, fixed in the release that carries this record,
     referring to issue #2, when that release is tagged, so the advisory names a fixed version; its text is drafted in the release-notes
     report.
   * Issue #2 is answered with this record and closed by the commit that implements it.

### Decided with the acceptance, 2026-10-12

Asked, the owner chose:

* **Order with 0014.** This record's PLAN runs before steps 6 to 8 of
  [0014-PLAN-union-constructors-take-every-required-key.md](0014-PLAN-union-constructors-take-every-required-key.md); those steps then
  document both records and write one set of `v0.16.0` release notes, so `v0.16.0` carries the constructors and this fix together. Not
  chosen: finishing 0014 first, which writes the README section and the notes twice; releasing 0014 alone as `v0.16.0`, which delays the
  fix a release.
* **Advisory timing.** Published at the fix release, naming the fixed version. Not chosen: no advisory; publishing before the fix.
* **Follow-up records.** The capability-gate and middleware records are written after this record is implemented, so they build on its
  hooks and typed error. Not chosen: drafting both now; drafting the capability record now.

### Consequences

* Good, because nothing the SDK sends can carry a path that is absolute nowhere where the standard requires an absolute one, and no option
  weakens that.
* Good, because an empty or relative path received in any of the 17 properties is refused before application code runs, unless the
  application decides otherwise, as the standard leaves it free to.
* Good, because a consumer gets one hook to resolve or translate a received message, one to extend, relax or observe validation, a typed
  error to branch on, and the session's root set, each in the repository's idiom; and a handler still sees only messages that passed or
  that the consumer accepted.
* Good, because the check four example clients write by hand is the SDK's, and the rule sits in the protocol table with its citation, so a
  schema that renames or drops a property fails `Build` rather than silently losing the check.
* Bad, because a Go application that sends a non-conforming message, accepted by every version so far, now gets an error instead; the
  release notes list it as a breaking change.
* Bad, because a peer that sends a relative MCP command name or tool-call location is refused or has its notification dropped, unless the
  application installs a hook; the README shows how.
* Bad, because a receiver refuses a path in another host's form by default: a Windows client listing a WSL agent's sessions gets an error
  for `/home/u/project` until it installs a rewrite hook that translates, or a validation hook that accepts.
* Bad, because tests that pass POSIX literals such as `"/test.txt"` through a connection fail on the Windows CI runner and must build
  their paths from `t.TempDir()`.
* Bad, because every generated `Validate` gains an unexported twin that carries the path rule, and every message the SDK sends or receives
  is validated, a cost proportional to its size, paid once per message.
* Neutral, because absolute is not confined: `/a/../../etc` is absolute. Confinement stays with the application, as the specification's
  "SHOULD serve as a boundary" leaves it.

### Confirmation

* `TestProtocolTableAppliesToCommittedSchema` passes with the 17 rules, and fails with a rule renamed to a property the schema lacks.
* A model test requires `Build` to refuse a rule on a property that is not a string, optional string, or array of strings, and requires
  `Diff`'s rule to reach `ToolCallContentDiff`.
* A unit test of `isPortableAbs` accepts `/a`, `C:\a`, `C:/a`, `\\server\share` and `\\?\C:\a`, and refuses `""`, `a`, `./a`, `\a` and
  `C:a`, on every host.
* An engine case carries a definition with an absolute-path rule in each shape and a behaviour test: `""` and `rel` refused, an absolute
  temporary directory accepted, an unset optional accepted, an array with one relative element refused naming its index, and the error
  reachable as a `*ValidationError` with `Rule == RuleAbsolutePath`.
* Root conformance tests over a client and agent connection, for each of the 17 properties: sending `""` or `relative/dir` fails before
  anything is written, and sending a path in either host's absolute form succeeds; receiving `""` or `relative/dir` is answered `-32602`
  (requests), dropped (notifications) or returned as an error (responses); the rewrite hook's change reaches the handler and is validated;
  the validation hook can accept a violation and refuse a valid message; a handler's non-conforming result is answered `-32603`.
* Each new test is seen failing on a planted break: the rule list emptied, the receiving rule replaced by a non-empty test, the sending rule
  replaced by `filepath.IsAbs`, the outbound check removed, each hook bypassed.
* Every test passes on the three CI hosts, `windows-2025` included; tests build their paths with `t.TempDir()` and `filepath.Join`.
* The exported API listing grows only by `ValidationError`, `ValidationRule` and its constants, `WithInboundRewrite`,
  `WithInboundValidation`, and the `Roots` methods.
* The release notes, the erratum and the advisory name the 28 checks, each traceable to `git show v0.14.0:types_gen.go`.

## Pros and Cons of the Options

### W1. A protocol-table rule

* Good, because every application gets the check with no code of its own, at the boundary that already enforces the schema.
* Good, because the rule names its source and is checked against the schema, as the table's method rules are.
* Bad, because it is protocol knowledge the schema does not carry, kept in step with the schema by hand; `Build`'s check of each rule
  limits that to a failure when the schema moves.

### W2. A hand-written helper

* Good, because nothing changes for an application that does not opt in.
* Bad, because the security property depends on every agent and client calling it; the issue arose from an agent relying on the SDK.

### W3. Restore `v0.14.0`'s non-empty checks

* Good, because it undoes the regression exactly.
* Bad, because it is stricter than the schema without being the specification's rule: `relative/dir` still passes.

### W4. Documentation only

* Good, because it costs least.
* Bad, because the SDK keeps sending and accepting what the specification forbids, and every application must write the check.

### W5. Derive the rule from the descriptions

* Good, because a new property described as absolute is covered with no edit.
* Bad, because it is the kind of heuristic 0011 found in F11 and F12 and moved into the protocol table: descriptions are prose, and a
  reworded description would silently drop or add a check.

### F1. Every property the standard requires to be absolute

* Good, because it is the standard in full, in both directions.
* Neutral, because its interop cost falls only on received violations, which the hooks let an application resolve or accept.
* Bad, because without a hook, a peer that forwards a bare MCP command name has its `session/new` refused.

### F2. The request properties that set or target a filesystem scope

* Good, because it covers both directions of the security surface with no interop cost.
* Bad, because five properties the standard covers stay unchecked for everyone, sent or received, which the owner's baseline rules out.

### F3. The session `cwd` alone

* Good, because it is the smallest change that answers the issue.
* Bad, because a relative or empty `fs/read_text_file` path, resolved against the client's own working directory, stays accepted.

### E1. Sending enforced, receiving by default with hooks

* Good, because the SDK conforms whatever the application does, and a received violation is handled as the application decides.
* Bad, because the hooks are a way to accept non-conforming input, which an application must use with care.

### E2. Received requests and notifications only, as today

* Good, because nothing new can fail for a sender.
* Bad, because the SDK sends and accepts responses that break the standard.

### E3. Sent and received, no hook

* Good, because it is the simplest strict form.
* Bad, because interop with a non-conforming or cross-host peer needs a fork, and no application can add its own rules.

### E4. Received only, with an off switch

* Good, because any interop problem can be switched off.
* Bad, because one switch turns off every check, the schema's included, and sending stays unchecked.

### A1. `filepath.IsAbs` on the host, both directions

* Good, because it is one rule, the standard library's.
* Bad, because a send-side check has no opt-out, so a Windows client could not send `/home/u/project` to a WSL or SSH agent at all, and
  the repository's tests that send `"/test.txt"` fail on the Windows runner.

### A2. POSIX or Windows form, both directions

* Good, because no path that is absolute anywhere is refused.
* Bad, because a Linux receiver accepts `C:\proj`, which it resolves as a relative file name: the issue's hazard, by default.

### A3. Non-empty only

* Good, because it is host-independent.
* Bad, because a relative path, which the specification forbids and which resolves against the receiving process's directory, passes.

### A4. Portable on sending, the host's rule on receiving

* Good, because it never refuses to send what a peer could read as absolute, and never lets a receiver use a path that is relative where
  it is used.
* Bad, because it is two rules, the sending one hand-written, and the generated code must carry the choice through nested types.

### H1. A rewrite hook before validation, a validation hook after

* Good, because a consumer can resolve or translate a received value and still have the SDK validate it, and can accept a violation it
  chooses to keep; a handler sees only what passed or what was accepted.
* Bad, because there are two options to learn, and the second must not change `params`, which the documentation, not the compiler,
  enforces.

### H2. One hook that may change the message, validated again after

* Good, because it is one option.
* Bad, because a violation the hook accepts without changing it is refused by the second validation, so accepting is impossible.

### H3. One hook, read-only

* Good, because the handler always sees what the peer sent.
* Bad, because resolving a command or translating a path must be done in every handler that uses it.

### H4. One hook that may change the message, not validated again

* Good, because it is one option and can both change and accept.
* Bad, because a hook's mistake reaches the handler unchecked.

### X1. A typed error and root-set accessors

* Good, because each closes a gap the code shows: errors that are only text, a root set every agent rebuilds.
* Good, because each is in the repository's idiom: `errors.As`, a method on a generated type in a hand-written file.
* Bad, because the exported API grows, and the hooks' `params any` is typed by the method, which the consumer switches on.

### X2. Nothing beyond `Validate`

* Good, because the API does not grow.
* Bad, because interop with a non-conforming peer, and any stricter policy, need a fork or a hand-built `MethodHandler`.

### X3. General middleware, decided here

* Good, because it covers tracing, metrics and authorization as well as validation.
* Bad, because it is a decision about every message, with its own order and error semantics, which this record about paths would decide in
  passing.

### C1. Name the 28, erratum, advisory, README, issue answered

* Good, because downstream agents learn which checks they lost, and that `cwd` and `path` are the ones with security weight.
* Bad, because it edits a published release's text and publishes an advisory, which the owner does by hand.

### C2. Next release notes only

* Good, because no published text changes.
* Bad, because readers of `v0.15.0`'s release learn nothing until they read a later one.

## More Information

* **Found here, decided elsewhere.** Each is its own decision and goes to its own record, written after this record is implemented:
  * **Capability gates (conformance).** The standard's capability MUSTs (`session/load` only when `loadSession` is advertised; `fs/*` and
    `terminal/*` only when the client advertises them; `additionalDirectories` only when advertised) are recorded in the protocol table and
    not enforced; enforcing them needs each connection to keep what `initialize` negotiated, which would also give consumers an accessor for
    the peer's capabilities.
  * **Middleware (flexibility).** Sending and receiving middleware over the existing `MethodHandler`, as the MCP Go SDK has, for tracing,
    metrics and authorization; the two hooks of H1 are a narrow, ordered case it would generalize, and that record says how they relate.
* **Not changed.** The other 22 removed checks stay removed: the standard states none of them. A consumer that wants one adds it with
  `WithInboundValidation`. No upstream change is needed to enforce the current standard; when this repository moves to a v2 schema, the
  protocol table can key the rule to v2's `AbsolutePath` definition instead of listing properties.
* **Order with 0014.** Decided with the acceptance: this record's PLAN first, then 0014's steps 6 to 8.
* **Revision.** A draft of 2026-10-12 chose F2 and one hook; the owner then asked that the open items follow the current standard, that the
  hook may rewrite, and that the record be re-checked. The re-check found 18 Go fields where the draft counted 17, the CI's Windows runner,
  the cross-host topology, and that one hook cannot both rewrite and accept with validation after it; A4 and H1 follow from those facts.
* **Sources.** <https://agentclientprotocol.com/protocol/overview>, <https://agentclientprotocol.com/protocol/session-setup>,
  <https://agentclientprotocol.com/protocol/file-system>, <https://agentclientprotocol.com/protocol/terminals>; upstream
  `agentclientprotocol/agent-client-protocol` at `d9d2f5204062`: `agent-client-protocol-schema/src/v1/agent.rs`,
  `agent-client-protocol-schema/src/v2/mod.rs`, `schema/v2/schema.json`, `docs/protocol/v2/draft/overview.mdx` and `session-setup.mdx`;
  `zed-industries/zed` at `725573ec14a2`, `crates/agent_servers/src/acp.rs`; `modelcontextprotocol/go-sdk` v1.8.0, `mcp/shared.go`
  (`Middleware`, `MethodHandler`); JSON-RPC 2.0, error codes `-32602` and `-32603`; Go: `path/filepath.IsAbs`, `os.OpenRoot` (Go 1.24),
  `os/exec.LookPath`, `errors.As`.
* **Related records.** [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md) (outcome 7, amended here),
  [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md) (the protocol table, heuristics),
  [0013-MADR-close-phase-6-follow-ups.md](0013-MADR-close-phase-6-follow-ups.md) and its report (the `v0.15.0` notes).
* **Implementation.** [0015-PLAN-validate-absolute-protocol-paths.md](0015-PLAN-validate-absolute-protocol-paths.md).

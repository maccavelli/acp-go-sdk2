---
status: in-progress
date: 2026-10-12
associated-madr: "0017-MADR-remediate-acp-v1-conformance-violations.md"
---

# Implement the ACP v1 conformance remediation

Associated MADR: [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md)

## Goal

Carry out the accepted decision: S1 first, then P1 (with the boundary decided at acceptance), Q1, J1, N1, G1 and K1, in the order O1
sets: this plan starts after `v0.16.0` is tagged, and its work is released as `v0.17.0`.

When it is done:

* the side connections are generated, with the protocol state each rule needs as typed fields, and the hand-written runtime is
  transport alone (`connection.go`, `errors.go`, `extensions.go`, `doc.go`);
* a prompt turn whose context was cancelled before its handler returned is answered `{"stopReason":"cancelled"}`, whatever the handler
  returned; a cancel that arrives after the handler returned changes nothing; a superseded turn no longer takes its successor's cancel;
* when the client sends `session/cancel`, every pending `session/request_permission` of that session is answered with the `cancelled`
  outcome at once;
* every line the SDK receives is answered as JSON-RPC 2.0 requires, or deliberately not answered when it is a valid notification or a
  response; a malformed response fails its call; the message size limit is an option;
* each side connection records the `initialize` exchange; the version is filled and checked; the typed methods send nothing before
  `initialize` and nothing the record does not allow; received messages are checked against the record by default, and the validation
  hook can accept what the SDK refuses;
* `NewResourceNotFound` exists; the README, `docs/architecture.md` and the `v0.17.0` release notes say what changed.

Each step's change is proven by a test that has been seen failing on a planted break, and every test passes on the three CI hosts and on
the owner's Windows host.

## Facts this plan rests on

Read on `a438f99`, 2026-10-12, beside the MADR's own facts and probes. Line numbers are at that commit.

* **The split.** `docs/architecture.md` ("What it is") lists `agent.go`, `client.go`, `connection.go`, `errors.go`, `extensions.go`,
  `helpers.go` and `doc.go` as hand-written and the six `*_gen` files as generated. `agent.go` (50 lines) and `client.go` (44 lines)
  declare only `AgentSideConnection` and `ClientSideConnection` (`conn`, and `agent` or `client`; the agent also `mu` and
  `sessionCancels`), `NewAgentSideConnection(agent Agent, peerInput io.Writer, peerOutput io.Reader, opts ...ConnectionOption)` and its
  client twin, which pass the side's `handleWithExtensions` to `NewConnection`, and `DroppedNotifications`, `Done`, `SetLogger` and
  `Connection`, each forwarding to `conn`.
* **Runtime files.** `enginetest.RuntimeFiles` (`cmd/generate/internal/enginetest/api.go:24`) is `agent.go`, `client.go`,
  `connection.go`, `doc.go`, `errors.go`, `extensions.go`; each engine case compiles the code generated for its synthetic schema with
  them, and `TestRuntimeFilesCoverHandWritten` (`cmd/generate/engine_test.go:132`) requires every other hand-written root file to be
  `helpers.go`. A runtime file may therefore name no ACP type, only what every schema generates. `extensions.go` declares
  `handleWithExtensions`, `CallExtension` and `NotifyExtension` on both side types, reads `a.agent`, `c.client` and `conn`, and calls the
  generated `handle` (`:50`, `:66`).
* **The two API listings.** The engine's `enginetest.API` (`api.go`) prints, with `go/types`, every exported declaration made in the
  generated files, a struct type with all its fields, unexported ones included, and the exported methods declared there;
  `TestEngineCases` compares it with each case's `want.api`, and `-update` rewrites them (`engine_test.go:17`, `:97`). 55 of the 88 files
  in `testdata/engine/` carry a `want.api`. The root listing program of the 0013 report prints exported declarations, exported struct
  fields and function types with their parameter names, sorted, with no file names, so moving a declaration between files does not
  change it, and an unexported field does not appear.
* **Generated files** are, in order, `constants_gen.go`, `types_gen.go`, `agent_gen.go`, `client_gen.go`, `helpers_gen.go` and
  `schema_gen_test.go` (`builds`, `cmd/generate/internal/emit/file.go`); `agentFile` and `clientFile` (`dispatch.go:66`, `:105`) build
  the two side files.
* **The protocol table.** `MethodRule` (`cmd/generate/internal/model/protocol.go:45`) has `Side`, `Wire`, `Binding`, `GoName`,
  `PromptTurn`, `CancelsPromptTurn`, `SessionKey` and `Capability`, whose comment says the capability is "recorded and checked, not
  emitted" (`:64`). `checkRule` refuses a prompt-turn role on the client side: "only agent methods take part in a prompt turn; the client
  side holds no session contexts" (`:169`). `checkCapabilities` (`:299`) walks each method's capability path from `AgentCapabilities` or
  `ClientCapabilities`, by the method's side, through pointer fields. `acp.go` holds 14 rules with a capability: `session/load` on
  `loadSession` (`:21`) and 13 under "Capability gates the schema describes, recorded, not acted on" (`:29` to `:42`), of which
  `session/fork` on `sessionCapabilities.fork` is unstable; `TestProtocolTableAppliesToCommittedSchema` counts 14 gates applied to the
  committed schema (`protocol_test.go:311`). `Build` skips a rule whose subject the schema lacks (`TestProtocolTableSkipsAbsentSubjects`,
  `protocol_test.go:120`).
* **The prompt case.** `promptCase` (`cmd/generate/internal/emit/dispatch.go:35`) emits `context.WithCancel`, cancels any previous turn,
  stores the new cancel function in `a.sessionCancels`, calls the handler, then deletes the session's entry unconditionally and returns
  `toReqErr(err)` or `checkResult`. `cancelActivePrompt` (`:54`) cancels and deletes the entry for `session/cancel`. `sessionCancels` is
  `map[string]context.CancelFunc` (`agent.go:16`). Go function values are not comparable, so "the entry is still this turn's" needs a
  comparable token. `TestDispatchFollowsTheModel` pins the emitted strings `reqCtx, cancel = context.WithCancel(ctx)` and the
  `sessionCancels` lines (`cmd/generate/internal/emit/dispatch_test.go:72` to `:75`), through its own `renamedProtocol`.
* **The prompt engine case** `testdata/engine/accept-prompt-cancellation.txtar` defines `PromptResponse` with one property, `stop`, and
  its agent returns `PromptResponse{Stop: "cancelled"}`; nothing there names a stop reason the generator could set.
* **The outbound wrappers** come from `emitOutbound` (`dispatch.go:138`): a notification calls `SendNotification`, a request
  `SendRequest[T]`, and a prompt turn whose call fails with its context done calls the side's cancel wrapper
  (`ClientSideConnection.Prompt`, `client_gen.go:288`). `Initialize` (`client_gen.go:202`) and `Cancel` (`client_gen.go:254`) are plain
  wrappers.
* **The receiving path.** `unmarshalValidate` (`dispatch_helpers.go:27`) decodes and calls `conn.checkInbound(ctx, method, &p)`;
  `callWithResult` (`:102`) and `checkedResult` (`:113`) send a result through `conn.checkResult`. `checkInbound`
  (`connection.go:1189`) runs the rewrite hook, `validate(filepath.IsAbs)`, then the validation hook; `inboundError` (`:1207`) answers a
  `*RequestError` as it is and anything else `-32602`; `checkResult` (`:1177`) answers a failing result `-32603`.
* **The connection.** `anyMessage.ID` is a `*json.RawMessage` (`connection.go:32`). `receive` (`:591`) reads with a `bufio.Scanner` capped
  at `maxBufSize = 10 * 1024 * 1024` (`:594`), registers each request's cancel in `inflight` by its canonical id, and logs and skips what
  does not decode or has neither `id` nor `method`. `handleCancelRequest` (`:774`) cancels with `context.Canceled`. `SendRequest`
  (`:862`) decodes and checks a result only when `len(resp.msg.Result) > 0`. `WithMaxQueuedNotifications` ignores `n <= 0`
  (`:178`).
* **Absent and null members.** A probe program outside the repository, on `go1.27.2`, decoded `{"id":null,...}`, `{...}` without `id`,
  and `{"id":7,"method":5}` into a struct of `json.RawMessage` fields, with the default `encoding/json` and with `GOEXPERIMENT=nojsonv2`:
  an absent member is a `nil` `RawMessage`, a `null` one the four bytes `null`, and a wrongly typed member its raw bytes, with no decode
  error, under both.
* **JSON-RPC 2.0** (<https://www.jsonrpc.org/specification>): "A Notification is a Request object without an "id" member"; "If there
  was an error in detecting the id in the Request object (e.g. Parse error/Invalid Request), it MUST be Null"; "If present, parameters
  for the rpc call MUST be provided as a Structured value". Its section 7 answers `{"jsonrpc": "2.0", "method": 1, "params": "bar"}`,
  which has no `id`, with `-32600` and `"id": null`, and `[]` the same way: an object without `id` that is not a valid Request object
  is not a notification, and is answered.
* **Types the remedy touches.** Unions are structs with one pointer per variant: `ContentBlock` (`Image`, `Audio`, `Resource`, ...),
  `McpServer` (`Http`, `Sse`, `Acp`, `Stdio`), `SessionUpdate` (`Notice`, `CompactionUpdate`, `CompactionSummaryChunk`, ...),
  `SessionConfigOption` (`Select`, `Boolean`), `CreateElicitationRequest` (`Form`, `Url`, `Other`), `AuthMethod` (`Terminal`, `Agent`),
  `RequestPermissionOutcome` (`Cancelled`, `Selected`). Their discriminator values in the schema are `image`, `audio`, `resource`;
  `http`, `sse`, `acp`; `form`, `url`; `terminal`; `boolean`. `InitializeRequest` has `ClientCapabilities` and `ProtocolVersion`;
  `InitializeResponse` has `AgentCapabilities`, `AuthMethods` and `ProtocolVersion`; `ProtocolVersion` is a `uint16`.
  `AuthMethodTerminalInline` has `Id`. `RequestPermissionRequest` has `SessionId`; `RequestPermissionResponse` has `Outcome`;
  `NewRequestPermissionOutcomeCancelled()` is generated (`helpers_gen.go:307`). `StopReason` is a string type with `StopReasonCancelled`
  (`types_gen.go:14762`, `:14769`). In the capability types a flag is a `bool` (`LoadSession`, `Fs.ReadTextFile`, `Terminal`,
  `PromptCapabilities.Image`, `McpCapabilities.Http`, `AuthCapabilities.Terminal`) or a pointer whose presence is the advertisement
  (`SessionCapabilities.List` and its siblings, `AgentAuthCapabilities.Logout`, `ElicitationCapabilities.Form` and `.Url`,
  `ClientSessionCapabilities.Notices`, `.Compaction`, `.ConfigOptions`), under parents that are values
  (`AgentCapabilities.SessionCapabilities`) or pointers (`ClientCapabilities.Session`, `.Elicitation`).
* **Where the gated values sit.** A script over `types_gen.go` lists every field of these types: `[]McpServer` is only `mcpServers` of
  `NewSessionRequest`, `LoadSessionRequest` and `ResumeSessionRequest`; `ContentBlock`, alone or in an array, is `PromptRequest.prompt`
  and otherwise content of the `session/update` variants and of tool-call content, which the agent sends; `[]SessionConfigOption` is
  only in the session-setup and `session/set_config_option` responses and the two config-option updates, which the agent sends;
  `[]AuthMethod` is only `InitializeResponse.authMethods`; `SessionUpdate` is only `SessionNotification.update`. So `ContentBlock` and
  `additionalDirectories` (which `SessionInfo` also has, sent by the agent and not gated) are gated by property, the others by type.
* **Tests the gates reach.** A script over the root tests lists the test functions that call a typed side-connection method or the
  generated `handle` without calling `Initialize`:
  * `acp_test.go`: `TestAgentDispatch_AllowsPartialUnstableMethodImplementation` (`handle`, on an `AgentSideConnection` literal),
    `TestConnectionHandlesErrorsBidirectional`, `TestConnectionHandlesConcurrentRequests`, `TestConnectionHandlesMessageOrdering`,
    `TestConnectionHandlesNotifications`;
  * `connection_notification_barrier_test.go`: `TestSendRequest_WaitsForPreResponseNotification`,
    `TestSendRequest_DoesNotWaitForPostResponseNotification`, `TestSendRequest_ConcurrentRequestsDoNotPanic`,
    `TestLoadSession_NotificationReplayOrdering` (each calls `LoadSession`, gated on `loadSession`),
    `TestShutdownDrainsNotifications_WithBarrier`;
  * `connection_queue_size_test.go`: `TestLargerQueue_SurvivesNotificationBurst`;
  * `fidelity_test.go`: `TestRequiredKeysOnDecode` (`handle`);
  * `paths_test.go`: `TestHandlerResultValidated`, `TestReceiveRefusesRelativePaths`, `TestInboundHooks`.

  `TestPromptCancellationSendsCancelAndAllowsNewSession`, `TestPromptWaitsForSessionUpdatesComplete`,
  `TestRequestHandlerCanMakeNestedRequest`, `Example_client` and `Example_gemini` call `Initialize` first. `Example_agent` only
  constructs a connection.
* **Tests that must keep passing unchanged.** `connection_cancel_test.go` builds raw `NewConnection`s and requires `-32800` from a
  handler cancelled by `$/cancel_request` (lines 80, 144, 208, 381) and from `SendRequest` when the caller's context ends (lines 476,
  580); `TestToReqErr_ContextCanceledMapsToRequestCancelled` (`errors_test.go:10`).
* **Examples.** `example/client` and `example/gemini` advertise `fs` and `terminal`; `example/claude-code` advertises `fs` only and
  implements the terminal methods; `example/client` and `example/gemini` implement the elicitation handlers and advertise no
  `elicitation`; `example/agent` answers `LoadSession: false`. No example calls a gated method or sends a gated variant (a `grep` of
  `example/` for the gated methods and variants). A handler for an unadvertised feature is never reached by a conforming peer; after
  step 9 a non-conforming one is refused, which is the decided behaviour, so the examples need no change for it.

## Scope

In scope:

* `cmd/generate/internal/model`: `protocol.go` and `acp.go` (new rule fields and the capability rules), `methods.go`, `model.go`, a new
  `gates.go` for the capability rules, and their tests.
* `cmd/generate/internal/emit`: `dispatch.go` (the side connections, the cases and wrappers), `dispatch_helpers.go`, `types.go` (the
  `gates` methods, `Negotiation`), and their tests.
* `cmd/generate`: `engine_test.go` and `internal/enginetest/api.go` (the runtime files), every engine case's `want.api`, the changed and
  new engine cases named below.
* Hand-written root files: `errors.go`, `connection.go`; `agent.go` and `client.go` removed.
* Generated files, regenerated only: `types_gen.go`, `agent_gen.go`, `client_gen.go`, and `schema_gen_test.go` if a step names it.
* Root tests: the files listed under the facts, `errors_test.go`, and the new test files named below.
* `example/*` if a step finds a check the SDK now makes; documentation: `README.md`, `docs/architecture.md`; records: this plan,
  [0015-MADR-validate-absolute-protocol-paths.md](0015-MADR-validate-absolute-protocol-paths.md) (a pointer), a release-notes report
  `docs/reports/0017-REPORT-release-notes-v0-17-0.md`, `docs/README.md`.

Out of scope:

* The open steps of [0015-PLAN-validate-absolute-protocol-paths.md](0015-PLAN-validate-absolute-protocol-paths.md) and
  [0014-PLAN-union-constructors-take-every-required-key.md](0014-PLAN-union-constructors-take-every-required-key.md), which come first.
* `helpers.go`, which stays hand-written: it holds the constructors of the three unions the table lists as hand-written helpers.
* The middleware record 0015 names; gates for unstable features the table does not state (`nes`, `plan`, `providers`, `subagents`,
  `positionEncoding`); the outbound cancel queue; JSON-RPC batch processing; duplicate keys; a client `Prompt` that waits for the agent's
  `cancelled` answer (MADR, "More Information").
* Gating `Connection`, `SendRequest[T]`, `SendRequestNoResult`, `SendNotification` and the extension methods (MADR, item 4).
* Rewriting [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md), whose list of runtime files is a fact
  of its time (MADR, "Relation to 0011").
* The tag, the GitHub release and its text, which the owner publishes.

## Implementation Steps

1. **Rules.** As in [0015-PLAN-validate-absolute-protocol-paths.md](0015-PLAN-validate-absolute-protocol-paths.md), step 1:
   * no step starts before `v0.16.0` is tagged (O1); the first step records the commit it starts from;
   * each new test or check is seen failing on a planted break in a scratch copy of the working tree's tracked and untracked files, never
     in the tree; the record quotes the failure;
   * a behaviour no record decides stops the step for the owner, with evidence and resolutions that leave the tree correct, and the
     chosen resolution is recorded as a dated deviation here, and in the MADR when it changes a decision or a fact;
   * after each step: regenerate (`cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .`); `make pre-add-check`; each untracked Go
     file through `GO_PRECHECK_SKIP_VULN=1 scripts/go-precheck.sh`; `GOEXPERIMENT=nojsonv2 go test -count=1 ./...` at the root;
     `GOOS=windows go vet ./...` at the root and in `cmd/generate`; `make check-records`; `make markdownlint`; the identifier scan of
     the changed files; record the outputs; stop for the owner's commit on `main`;
   * every change to a generated file, to an engine case's `want.api` and to the root's exported API listing (the listing program of the
     0013 report, on `git archive HEAD` and on the tree) is asserted by a script against what the step names; anything else stops the
     step. `want.api` files are rewritten only by `go test -run TestEngineCases -update` in `cmd/generate`, then asserted;
   * before handing off a step that changes Go code, the tests run on the owner's Windows host over SSH, on a copy of the tracked and
     untracked files without macOS metadata, removed afterwards; the record notes the result without naming the host.

2. **Records.** No source changes.
   * [0015-MADR-validate-absolute-protocol-paths.md](0015-MADR-validate-absolute-protocol-paths.md), "More Information", "Found here,
     decided elsewhere", capability gates: a dated note that the record was written as
     [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md) at the owner's request,
     before 0015 was complete, building on 0015's hooks and typed error.
   * This plan's status is `in-progress`; `docs/README.md` says so.

3. **Errors and the helper (K1; the error types of P1, G1, N1).** `errors.go` only; it names no ACP type.
   * `ErrPromptCancelled = errors.New("acp: prompt turn cancelled")` and `ErrNotInitialized = errors.New("acp: initialize has not
     completed")`.
   * `type CapabilityError struct { Method, Capability, Field, Variant string }`, whose `Error` reads `<method>: capability <capability>
     not advertised`, followed by `: <field>` or `: variant <variant>` when set.
   * `type ProtocolVersionError struct { Requested, Answered int }`, whose `Error` reads `acp: protocol version <requested> is not
     supported` when `Answered` is 0, and `acp: agent answered protocol version <answered> to a request for <requested>` otherwise.
   * `NewResourceNotFound(uri string) *RequestError`: code `-32002`, message "Resource not found", data `map[string]any{"uri": uri}`.
   * Regenerate: nothing changes. Asserted: the root API listing gains only the two variables, the two types with their `Error` methods
     and fields, and `NewResourceNotFound`; no `want.api` changes.
   * Tests, `errors_test.go`: each `Error` text; `errors.Is` finds `ErrPromptCancelled` through `fmt.Errorf("%w")`; `errors.As` finds each
     type through one wrap; `NewResourceNotFound("file:///a")` marshals to `{"code":-32002,"message":"Resource not found","data":
     {"uri":"file:///a"}}`.
   * Plants: `NewResourceNotFound` with code `-32001`; `CapabilityError.Error` dropping the variant.

4. **Generated side connections (S1).** Behaviour does not change in this step.
   * `emit/dispatch.go`: `agentFile` and `clientFile` first emit the side's connection: the struct with `conn *Connection` and `agent
     Agent` or `client Client`, the field names `extensions.go` reads; for the agent, when a `PromptTurn` rule applies, `mu sync.Mutex` and
     `sessionCancels map[string]context.CancelFunc`, as today; the constructor with today's name, parameter names and body, making
     `sessionCancels` when it exists; and `DroppedNotifications`, `Done`, `SetLogger` and `Connection`. The doc comments are those of
     `agent.go` and `client.go`, word for word.
   * `agent.go` and `client.go` are removed. `enginetest.RuntimeFiles` becomes `connection.go`, `doc.go`, `errors.go`, `extensions.go`,
     and its comment says the runtime refers to generated code through `Agent`, `Client`, `ProtocolMethodCancelRequest`, the two side
     types and their `handle`. `TestRuntimeFilesCoverHandWritten` follows.
   * `docs/architecture.md`, "What it is" and "Tree": the side connections are generated; the hand-written files are `connection.go`,
     `errors.go`, `extensions.go`, `helpers.go` and `doc.go`.
   * `TestAgentDispatch_AllowsPartialUnstableMethodImplementation` keeps building its literal with `conn: &Connection{}`; the field names
     do not change.
   * Regenerate; `-update` the engine cases. Asserted:
     * the root API listing is byte-identical to the one before the step;
     * `agent_gen.go` and `client_gen.go` gain exactly the struct, the constructor and the four methods; no other generated file changes;
     * every `want.api` gains exactly `type AgentSideConnection struct{...}`, `type ClientSideConnection struct{...}`, the two
       constructors and the eight forwarding methods, the agent's struct holding `mu` and `sessionCancels` only in the cases whose schema
       has a prompt turn (`accept-prompt-cancellation.txtar` among them), and nothing else.
   * Tests: every root and generator test passes unchanged; a new emitter test, `TestSideConnectionsFollowTheModel`, requires the
     prompt-turn fields with a prompt rule and none without, and the doc comments.
   * Plants: the constructor not making `sessionCancels` (the prompt tests panic on a nil map); the prompt fields emitted only without a
     prompt rule (the emitter test fails, and the generator refuses the root's output, which no longer type-checks).

5. **Prompt turns (P1).**
   * `protocol.go`: `MethodRule` gains `StopKey` and `StopCancelled` (the result property that holds the stop reason and its cancelled
     value), allowed only with `PromptTurn`; `Method` gains `StopField` and `StopConst`. `linkPromptTurns` resolves them: the property must
     be a required field of the response type whose type is a string enum holding the value; `StopConst` is that value's constant. A
     prompt turn without them is refused (`a prompt turn needs its stop key and cancelled value`). `acp.go`: the `session/prompt` rule
     gains `StopKey: "stopReason", StopCancelled: "cancelled"`, with the `StopReason` sentence of the schema as its comment.
   * Emitted into `agent_gen.go` with the prompt rule: `type promptTurn struct{ cancel context.CancelCauseFunc }`; `sessionCancels`
     becomes `map[string]*promptTurn`; `var promptReturned func()`, the test seam below.
   * `promptCase`, emitting in order:
     1. `reqCtx, cancel := context.WithCancelCause(ctx)`, `turn := &promptTurn{cancel: cancel}`;
     2. under `a.mu`: an existing turn of the session is cancelled with `fmt.Errorf("%w: superseded by a new prompt", ErrPromptCancelled)`,
        and `turn` stored;
     3. the handler is called with `reqCtx`;
     4. under `a.mu`, once: the entry is deleted only if it is `turn`, and `cause := context.Cause(reqCtx)` is read; then `cancel(nil)`;
     5. if `cause != nil`: `err = nil` and `resp.<StopField> = <StopConst>` (on the zero value when the handler returned an error), so the
        handler's `_meta` and any other field it returned are kept;
     6. otherwise as today: `toReqErr(err)`, or `checkResult`.
   * `cancelActivePrompt` cancels with `ErrPromptCancelled` and deletes the entry. `$/cancel_request` keeps `context.Canceled`
     (`handleCancelRequest` unchanged); it reaches `reqCtx` through its parent, and emitted step 5 answers it `cancelled` too.
   * `accept-prompt-cancellation.txtar`: `PromptResponse` gains a required `stopReason`, a string enum `end_turn` | `cancelled`, and its
     agent returns `ctx.Err()` once cancelled; the behaviour test requires the client's `Prompt` to return `stopReason` `cancelled`. A
     new engine case, `refuse-prompt-turn-without-stop.txtar`, requires `Build` to refuse a prompt turn without the stop rule.
     `dispatch_test.go`: `renamedProtocol` gives its prompt-turn rule a stop key and value, and the pinned strings follow the new emission.
   * Regenerate; `-update`. Asserted: `agent_gen.go` changes only in the agent's struct, the new type and variable, and the
     `session/prompt` and `session/cancel` cases; the `want.api` of the cases with a prompt turn change only in the agent struct's
     `sessionCancels` field and `PromptResponse`; the root API listing does not change.
   * Tests, a new root file `cancel_test.go`, each through a client and an agent connection over pipes:
     * `TestPromptCancelledAnswersCancelled`: the handler returns `ctx.Err()`; a wrapped `ctx.Err()`; `end_turn` with `_meta` set, each
       after `session/cancel` reached it; each answer is `stopReason` `cancelled`, the third keeping its `_meta`; `context.Cause` in the
       handler is `ErrPromptCancelled`;
     * `TestPromptCancelRequestAnswersCancelled`: `$/cancel_request` for the prompt's id gives `stopReason` `cancelled`, the handler's
       cause being `context.Canceled`;
     * `TestPromptCancelAfterReturnChangesNothing`: the handler returns `end_turn`; the case stops at `promptReturned`, called between
       emitted steps 4 and 5, until a `session/cancel` for its session has been handled and found no turn; the answer is `end_turn`.
       `promptReturned` is nil outside tests and is the only test seam this plan adds;
     * `TestPromptSupersededKeepsSuccessorCancel`: the probe of the MADR, now passing: the first turn is answered `cancelled` with a cause
       wrapping `ErrPromptCancelled`, and `session/cancel` after the first returned cancels the second;
     * a non-prompt request cancelled by `$/cancel_request` still answers `-32800` (the cancel tests, unchanged, cover the raw path).
   * The race the MADR notes ("Noted, not decided"): a probe in a scratch copy sends `session/prompt` and `session/cancel` back to back
     over the pipes, 1000 times under `-race`, and counts turns that ran to completion. Its count is recorded. A turn that was not
     cancelled stops the step for the owner as a deviation with that evidence; none leaves the MADR's note as it is, closed by the record.
   * Plants: emitted step 5 removed (the first test fails with `-32800`); the delete made unconditional again (the supersede test fails);
     the cause read after `cancel(nil)` instead of before (the after-return test fails: `cancel(nil)` sets a cause); `cancelActivePrompt`
     cancelling with a `nil` cause (the cause check fails).

6. **Permission requests on cancel (Q1).**
   * `protocol.go`: `MethodRule` gains `CancelledByPromptCancel bool` and `CancelledKey`, `CancelledVariant` (the result property and the
     variant that answer a cancelled request); `checkRule` allows this role on the client side and requires `SessionKey`; the message
     at `:169` changes to name the agent-only roles. `methods.go`: `Method` gains `CancelledField` and `CancelledHelper` (the generated
     constructor of the variant, from the 0014 helpers). `linkPromptTurns` resolves them and links the client's cancel wrapper to them.
     `acp.go`: `{Side: client, Wire: "session/request_permission", CancelledByPromptCancel: true, SessionKey: "sessionId", CancelledKey:
     "outcome", CancelledVariant: "cancelled"}`, citing the prompt-turn sentence.
   * Emitted into `client_gen.go` with that rule: the client struct gains `mu sync.Mutex` and `pending
     map[string]map[*pendingPermission]struct{}`, made by the constructor; `type pendingPermission struct{ cancel
     context.CancelCauseFunc }`; `trackPermission(ctx context.Context, session string) (context.Context, func())`, which registers one and
     returns its context and its removal; `cancelPermissions(session string)`, which cancels each registered context of the session with
     `ErrPromptCancelled`.
   * `connection.go`: `func answerUnlessCancelled[T any](ctx context.Context, call func(context.Context) (T, error), cancelled T) (T,
     error)`: runs `call` on its own goroutine; returns `cancelled` at once when `ctx` ends with a cause that `errors.Is`
     `ErrPromptCancelled`, and also when `call` returns after that; otherwise waits for `call` and returns its result. It names no ACP
     type. A discarded late result is logged at Debug by the caller.
   * Emission: the client's case for a `CancelledByPromptCancel` method calls `trackPermission`, defers the removal, and calls the handler
     through `answerUnlessCancelled` with `RequestPermissionResponse{Outcome: NewRequestPermissionOutcomeCancelled()}`, built from
     `CancelledField` and `CancelledHelper`; its result goes through `checkResult`. The client's outbound wrapper of the agent's
     `CancelsPromptTurn` method (`Cancel`) calls `c.cancelPermissions(string(params.<SessionField>))` after `SendNotification` returns,
     so the notification is written before any answer.
   * A new engine case, `accept-permission-cancellation.txtar`, with a minimal permission request, outcome union and cancel; its behaviour
     test requires the cancelled outcome. `refuse-permission-cancel-rule.txtar` requires `Build` to refuse the role without `SessionKey`
     or with a variant the result's union lacks.
   * Regenerate; `-update`. Asserted: `client_gen.go` changes only in the client's struct and constructor, the new type and two methods,
     the `session/request_permission` case and `Cancel`; only the new engine case's `want.api` shows the client fields; the root API
     listing does not change.
   * Tests, `cancel_test.go`:
     * `TestCancelAnswersPendingPermissions`: two pending requests in session `s` and one in `t`, one handler blocked on its context and
       one ignoring it; `Cancel` for `s` answers both `s` requests `cancelled` before either handler returns, and the `t` request is
       still pending; the handler that watches its context sees `ErrPromptCancelled`;
     * `TestPromptCancelAnswersPendingPermissions`: the same through `Prompt`'s context ending;
     * `TestPermissionAfterCancelHandledNormally`: a request arriving after the cancel gets its handler's answer.
   * Plants: `cancelPermissions` not called by `Cancel` (both cancel tests time out with the request unanswered); `answerUnlessCancelled`
     waiting for `call` (the blocked handler holds the answer); the session filter removed (the `t` request is answered).

7. **Malformed input (J1).** `connection.go`.
   * `WithMaxMessageSize(n int)`: the largest line accepted, in bytes; `n <= 0` keeps the default of 10 MiB, as
     `WithMaxQueuedNotifications` does.
   * `receive` reads lines with a `bufio.Reader` and a helper `readLine` that returns a complete line, or reports a line over the limit
     after discarding it through its newline; such a line is logged and answered `NewInvalidRequest(map[string]any{"error": "message
     exceeds <n> bytes"})` with `"id": null`, and reading goes on.
   * Each non-blank line is classified in this order:
     1. `json.Valid` false: `NewParseError` with `"id": null`;
     2. not a JSON object (an array included): `NewInvalidRequest` with `"id": null`;
     3. decoded into an envelope of `json.RawMessage` members (`jsonrpc`, `id`, `method`, `params`, `result`, `error`), so absence is `nil`
        and `null` is the bytes `null`;
     4. `result` or `error` present: a response. It is never answered. With a valid id that matches a pending call, the call fails with
        `NewInternalError` naming the fault when `jsonrpc` is not `"2.0"`, when both or neither member is set, or when `error` does not
        decode as an error object; otherwise it is delivered as today. Unmatched, it is logged;
     5. otherwise a Request object, valid when `jsonrpc` is `"2.0"`, `method` is a string, `id` is absent or a string, a number or
        `null`, and `params` is absent or an object or array. A valid one with an `id` (`null` included) is a request and handled; a
        valid one without `id` is a notification and handled, never answered. An invalid one is answered `NewInvalidRequest` naming
        the fault, with its id when the id is a string, number or `null`, and with `"id": null` otherwise, an absent id included, as
        JSON-RPC's section 7 does.
   * `anyMessage` keeps `ID *json.RawMessage`; a request whose id is `null` is answered with `ID` pointing at the bytes `null`, which
     marshals as `"id": null`. A `null`-id request is not registered in `inflight`, since `null` names no single request; a
     `$/cancel_request` naming `null` cancels nothing, and the README says so.
   * `SendRequest` is unchanged: a malformed response reaches it as an error.
   * Regenerate: nothing changes. Asserted: the root API listing gains only `WithMaxMessageSize`; no `want.api` changes.
   * Tests, a new root file `jsonrpc_test.go`, each writing raw lines to an agent connection and reading its output with a deadline:
     * `TestJSONRPCErrorAnswers`: the six lines of the MADR's probe, `{"jsonrpc":"2.0","id":9,"method":"session/new","params":5}`
       and JSON-RPC's own `{"jsonrpc": "2.0", "method": 1, "params": "bar"}`, each with its exact answer; the `null`-id request
       answered with `"id": null`; `"jsonrpc":"1.0"` with an id refused `-32600` with that id; a valid notification for a method the
       agent does not have gets no answer;
     * `TestJSONRPCNoReplyToResponses`: `{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}` and an unmatched
       result get no reply within the deadline;
     * `TestMalformedResponseFailsCall`: a peer answering `{"jsonrpc":"2.0","id":1}`, one with both members, and one with `"jsonrpc":"1.0"`
       each make `NewSession` return an error;
     * `TestMaxMessageSize`: with `WithMaxMessageSize(1024)`, a 1025-byte line is answered `-32600` with `"id": null` and the next valid
       request is answered; a 1024-byte line is accepted; without the option a line just over 10 MiB is answered the same way.
   * Plants: each answer removed in turn (its line gets no reply); the response check answering a response (the no-reply test fails); the
     neither-member check removed (`NewSession` succeeds again); the overlong path closing the connection (the next request is not
     answered).

8. **Negotiation and version (N1; the record of G1).**
   * `protocol.go`: `MethodRule` gains `Initializes bool` and `VersionKey`, `RequestCapabilities`, `ResponseCapabilities` (the property
     keys of the version and of each side's capabilities); `acp.go`'s `initialize` rule sets them to `protocolVersion`,
     `clientCapabilities` and `agentCapabilities`, citing the initialization sentences. `Build` refuses a second `Initializes` rule and
     keys the request or response lacks.
   * Emitted into `types_gen.go` when the rule applies: `type Negotiation struct { Request InitializeRequest; Response
     InitializeResponse }`, documented as what `initialize` agreed.
   * Emitted into `agent_gen.go` and `client_gen.go` when the rule applies: each side's struct gains `negotiation
     atomic.Pointer[Negotiation]`, and each side `Negotiated() (Negotiation, bool)`, which loads it.
   * The agent's `initialize` case, after its handler: a zero `<VersionKey>` becomes `ProtocolVersionNumber`; any other value than
     `ProtocolVersionNumber` is logged and answered `-32603`; the result goes through `checkResult`; then the `Negotiation` of the request
     and the result is stored, before the answer is written.
   * `ClientSideConnection.Initialize`: a zero request version becomes `ProtocolVersionNumber`; another value returns
     `&ProtocolVersionError{Requested: int(v)}` without sending; an answer whose version is not `ProtocolVersionNumber` returns
     `&ProtocolVersionError{Requested, Answered}` and stores nothing; otherwise the `Negotiation` is stored and the response returned.
   * A new engine case, `accept-initialize-negotiation.txtar`, with a minimal `initialize`; its behaviour test covers both sides' record
     and both version rules. Engine cases without an `initialize` rule generate no `Negotiation` and no `negotiation` field.
   * Regenerate; `-update`. Asserted: `types_gen.go` gains only `Negotiation`; `agent_gen.go` changes only in the struct, the `initialize`
     case and the new `Negotiated`; `client_gen.go` only in the struct, `Initialize` and `Negotiated`; only the new engine case's
     `want.api` changes; the root API listing gains only `Negotiation` with its two fields and the two `Negotiated` methods.
   * Tests, a new root file `negotiation_test.go`:
     * `TestNegotiationRecorded`: after `Initialize`, both sides' `Negotiated` return the same request and response; before it, both
       return `false`; a second `Initialize` replaces the record;
     * `TestProtocolVersion`: a handler leaving the version zero answers 1; a handler answering 2 is answered `-32603`; a client sending
       version 0 sends 1; a client sending 2 gets a `*ProtocolVersionError` and nothing is written; a raw agent answering 2 makes
       `Initialize` return a `*ProtocolVersionError{Requested: 1, Answered: 2}` and `Negotiated` stays `false`.
   * Plants: the zero fill removed (the handler test answers 0); the answer check removed (`Initialize` succeeds with 2); the record
     stored before the version check (`Negotiated` reports `true` after a refused answer).

9. **Method gates and initialization order (G1, part).**
   * `acp.go`: the comment "recorded, not acted on" is replaced by the specification sentences of each gate; the 14 rules keep their
     capabilities, and a 15th gates `elicitation/complete` on `elicitation.url`, since the schema describes it as sent "when a
     URL-based elicitation is complete". `protocol.go`: the `Capability` comment says the gate is enforced.
   * Emission, from each method's side and capability path, through the walk `checkCapabilities` already does: a Go expression over a
     `*Negotiation` that is true when the capability is advertised: a `bool` field read as it is, a pointer field compared with `nil`,
     each pointer parent guarded. Agent methods read `n.Response.<ResponseCapabilities field>`, client methods
     `n.Request.<RequestCapabilities field>`.
   * Emitted into `types_gen.go` with the `initialize` rule: `func (n Negotiation) Allows(method string) error`, a `switch` over the gated
     methods, returning a `*CapabilityError` with `Method` and `Capability` when the expression is false, nil otherwise; and an
     unexported `func gateMessage(n *Negotiation, method string, params any) error`: `initialize` passes; with `n == nil` it returns
     `fmt.Errorf("%s: %w", method, ErrNotInitialized)`; then `n.Allows(method)`; then, from step 10, the value gates of `params`.
   * Emitted on each side with the `initialize` rule: `gate(method string, params any) error`, which loads the side's `negotiation` and
     calls `gateMessage`. A schema without the rule gets no `gate`, and its dispatch passes `nil` below.
   * `connection.go`, with the gate always an argument, never stored:
     * `checkInbound(ctx, method string, params any, gate func(method string, params any) error)`: after the SDK's validation passes, a
       non-nil `gate` is called; the validation hook receives whichever error results;
     * `inboundError`: a `*CapabilityError` with no `Field` or `Variant`, or an error wrapping `ErrNotInitialized`, is answered
       `-32601` (`Method not found`, data naming the method and the reason); one with a `Field` or `Variant`, `-32602`; others as today;
     * an unexported `sendRequest[T](c, ctx, method, params, gate)` holds `SendRequest`'s body and passes `gate` to its `checkInbound`;
       `SendRequest[T]` calls it with `nil`;
     * `checkResult(method, result, gate)` calls a non-nil `gate` on the result after validating it; a refusal is answered `-32603`.
   * Emission: `unmarshalValidate` and `checkedResult` pass the side's `gate` (a method value, `a.gate` or `c.gate`); every outbound
     wrapper but `Initialize` calls the side's `gate(<Const>, &params)` before sending and returns its error without writing; request
     wrappers call `sendRequest[T](..., c.gate)`. The record is stored by step 8 before the agent's `initialize` answer is written, and
     before the client's `Initialize` returns, so a request sent after `Initialize` returns is checked against it.
   * Regenerate; `-update`. Asserted: `types_gen.go` gains only `Allows` and `gateMessage`; `agent_gen.go` and `client_gen.go` change in
     each case's and wrapper's gate argument or call, and gain `gate`; engine cases change only in behaviour tests that call the changed
     runtime functions, if any; the root API listing gains only `Negotiation.Allows`.
   * Tests:
     * a test helper in `negotiation_test.go`, `negotiate(t, side, n *Negotiation)`, stores a record on an `AgentSideConnection` or
       `ClientSideConnection` for tests that call `handle` directly, and `allAdvertised()` returns a `Negotiation` advertising every
       capability the table names;
     * every test listed under "Tests the gates reach" initializes first through the connections, or calls `negotiate`; the
       `LoadSession` tests advertise `loadSession`. Each change is the smallest that keeps the test's subject: no assertion is loosened,
       and the record lists each changed test;
     * a new root file `gates_test.go`: `TestMethodGates` takes each of the 15 gated methods: with a record that does not advertise its
       capability, the typed method returns a `*CapabilityError` naming the method and capability and writes nothing; with one that does,
       it is sent; received with a record that does not advertise, it is answered `-32601` (request) or dropped (notification); and a
       validation hook that accepts the `*CapabilityError` lets the handler run;
     * `TestNotInitialized`: before `initialize`, each typed method but `Initialize` returns an error wrapping `ErrNotInitialized`
       without writing, and a received request is answered `-32601`;
     * `TestRawPathsUngated`: `SendRequest[T]`, `SendNotification` and `CallExtension` send before `initialize`;
     * `TestAllows`: `Negotiation.Allows` agrees with the gate for each method;
     * `TestProtocolTableAppliesToCommittedSchema` counts 15 applied gates.
   * Plants: the wrapper's gate call removed (the send case fails); `checkInbound` not calling `gate` (the receive case fails); the
     `nil`-record branch removed (`TestNotInitialized` fails); the emitted expression of a `bool` capability negated (the advertised case
     is refused); `inboundError` answering `-32602` for a method gate (the code check fails).

10. **Property and variant gates (G1, part).**
    * `protocol.go`: `Protocol` gains `CapabilityRules []CapabilityRule`, `type CapabilityRule struct { Side, Definition, Property,
      Variant, Capability, Source string; Kind GateKind }`, with `GateNonEmpty` (a property that must be empty unless advertised),
      `GateVariant` (a variant that must not be set unless advertised, on the definition itself or on the elements of `Property`) and
      `GateNotTerminalAuthMethod` (a property naming an auth method id that must not name a `terminal` method of the record's response).
      `Side` is whose capabilities the path starts from. `newRules` refuses an empty field and a duplicate.
    * `acp.go`: the rules, each with its source sentence from the MADR's findings, variants named by their discriminator values:
      * client side, gated on the agent's capabilities: `additionalDirectories` of `NewSessionRequest`, `LoadSessionRequest`,
        `ResumeSessionRequest` and `UnstableForkSessionRequest` on `sessionCapabilities.additionalDirectories` (`GateNonEmpty`);
        `McpServer` `http`, `sse` and `acp` on `mcpCapabilities.http`, `.sse` and `.acp`; `PromptRequest.prompt` `image`, `audio` and
        `resource` on `promptCapabilities.image`, `.audio` and `.embeddedContext`; `AuthenticateRequest.methodId`
        (`GateNotTerminalAuthMethod`);
      * agent side, gated on the client's capabilities: `CreateElicitationRequest` `form` and `url` on `elicitation.form` and `.url`;
        `SessionUpdate` `notice` on `session.notices`, `compaction_update` and `compaction_summary_chunk` on `session.compaction`;
        `SessionConfigOption` `boolean` on `session.configOptions.boolean`; `InitializeResponse.authMethods` `terminal` on
        `auth.terminal`.
    * `model/gates.go`: after `buildValidations`, resolve each rule against the schema (as `resolveAbsolutePaths` does; a rule whose
      definition or property the schema lacks is skipped, a property of the wrong shape or a variant the union lacks is refused), then
      mark every type that holds a gated property or variant, directly or through fields, as `buildValidations` marks types that
      validate. `TestProtocolTableAppliesToCommittedSchema` requires every rule to resolve on the committed schema.
    * `emit/types.go`: an unexported `func (v *T) gates(n *Negotiation) error` on each marked type: each rule's check returning a
      `*CapabilityError` with `Field` (the JSON key, with the index of an element) or `Variant` (the discriminator value), and a call into
      each nested marked type. `gateMessage` calls `gates` when `params` has it.
    * No record exists while `initialize` is answered, so both sides check its gates against a `Negotiation` built from the request and
      the result before storing it: the agent's `initialize` case answers a refusal `-32603`, as for any result; the client's
      `Initialize` passes its result to the validation hook as `checkInbound` does for other responses, so by default it returns the
      error and stores nothing, and a hook may accept it.
    * A new engine case, `accept-capability-gates.txtar`, defines a minimal `initialize` with a capability tree, a request with an
      optional array, a union with a gated variant inside an array property, and a gated variant of a union a notification carries; its
      behaviour test covers each kind both ways; `refuse-capability-rule.txtar` requires `Build` to refuse a variant the union lacks and a
      `GateNonEmpty` on a non-array property.
    * Regenerate; `-update`. Asserted: `types_gen.go` gains only the `gates` methods and the call in `gateMessage`; `agent_gen.go` and
      `client_gen.go` change only in the `initialize` paths; only the new engine case's `want.api` changes; the root API listing does not
      change.
    * Tests, `gates_test.go`, `TestValueGates`: for each rule, the sender with a record that does not advertise gets a
      `*CapabilityError` naming the field or variant and nothing is written; with one that does, it is sent; a received message with the
      value is answered `-32602` (request), dropped (notification) or returned as an error (response); an agent whose `NewSession` result
      holds a boolean config option for a client that did not advertise it answers `-32603`; an agent whose `initialize` result lists a
      terminal auth method for a client without `auth.terminal` answers `-32603`, and a raw agent sending one makes `Initialize` return
      the `*CapabilityError` unless a validation hook accepts it; `authenticate` with a terminal method's id is refused before sending.
    * Plants: one rule removed from the table (its case fails); the element loop checking only the first element (a two-element prompt
      with an image second passes); `gateMessage` not calling `gates` (every value case fails); the `initialize` paths skipping their
      gate (the terminal auth cases fail).

11. **Documentation and examples (MADR item 7).**
    * `README.md`, new parts of "Explore the API" or "Wire encoding" as fits the current layout: "Cancellation" (the prompt turn,
      `ErrPromptCancelled`, the after-return boundary, permission requests, that marking unfinished tool calls is the application's);
      "Errors on the wire" (the JSON-RPC answers, responses never answered, `WithMaxMessageSize`, `null` ids, `NewResourceNotFound`);
      "Initialization and capabilities" (the record, `Negotiated`, `Allows`, `ErrNotInitialized`, `*CapabilityError`, the version rule
      and that the application closes the connection and informs the user on a `*ProtocolVersionError`, how a validation hook accepts a
      refused message, and that `Connection`, `SendRequest[T]` and the extension methods are ungated).
    * `docs/architecture.md`: the receive classification, the prompt-turn bookkeeping, the permission registry, the negotiation record,
      where each gate runs, and the protocol table's new rules.
    * Examples: run each against the gates; any check the SDK now makes is dropped; a capability an example uses but does not advertise
      is a deviation for the owner, not a silent change (none is known: see the facts).
    * Regenerate: nothing changes; the API listings do not change.

12. **Release notes.** `docs/reports/0017-REPORT-release-notes-v0-17-0.md`, in the form of the `v0.16.0` release-notes report that
    step 7 of [0014-PLAN-union-constructors-take-every-required-key.md](0014-PLAN-union-constructors-take-every-required-key.md)
    writes: each enforcement as a breaking change with what an application does about it; the additions; the defects fixed (the
    supersede bookkeeping, the malformed response, the `null` id); that the side connections are now generated, with no API change; a
    "Release text" section for the owner. Indexed in `docs/README.md`.

13. **Close.** Run the verification below and record each result; set this plan to `complete` and its row in `docs/README.md`.

## Verification

* Every test this plan names passes, and each failed on its step's plants.
* The MADR's four probes, run again in a scratch copy, show the conforming behaviour: `cancelled` for the cancelled prompt; `true` for the
  successor's cancel; an answer for each malformed line J1 answers; an error for the empty response.
* `agent.go` and `client.go` no longer exist; `TestRuntimeFilesCoverHandWritten` passes with `connection.go`, `doc.go`, `errors.go`,
  `extensions.go` and `helpers.go` as the hand-written files; no generated code is bridged to the runtime through `any` or a stored
  function (a `grep` of `connection.go` for `atomic.Value` and for a `gate` field finds neither).
* The root's exported API listing, against the commit step 3 starts from, gains exactly `ErrPromptCancelled`, `ErrNotInitialized`,
  `CapabilityError` with its fields and `Error`, `ProtocolVersionError` with its fields and `Error`, `NewResourceNotFound`,
  `WithMaxMessageSize`, `Negotiation` with its fields, `Negotiation.Allows`, `AgentSideConnection.Negotiated` and
  `ClientSideConnection.Negotiated`.
* `TestProtocolTableAppliesToCommittedSchema` passes with every method and capability rule applied; `TestEngineCases` passes with the new
  and changed cases; every `want.api` change was asserted in its step.
* `go test -race -count=1 ./...` at the root and in `cmd/generate`; `GOEXPERIMENT=nojsonv2 go test -count=1 ./...` at the root;
  `GOOS=windows go vet ./...` in both modules; the tests on the owner's Windows host; CI on `ubuntu-24.04`, `macos-15` and
  `windows-2025` for each pushed step.
* `make pre-add-check`, `make check-records` and `make markdownlint` pass; the identifier scan finds nothing.

## Rollout and Rollback

Starts after `v0.16.0` is tagged. Commits land on `main`, made by the owner, one per step; steps 4 to 10 carry their regenerated files
and `want.api` changes. The owner pushes after each step and the step closes when CI passes. The owner tags `v0.17.0` and publishes its
release from the report of step 12.

Rollback is `git revert` of a step's commit, latest first: step 10 adds to step 9's `gateMessage`; step 9 reads step 8's record and uses
step 3's errors; steps 5, 6 and 8 add fields to the side connections step 4 generates; steps 5 and 6 use step 3's `ErrPromptCancelled`;
step 7 stands alone. Steps 2, 11, 12 and 13 are records and documentation.

## Execution record

### Step 2, records, 2026-10-12

The owner approved execution with "do 0017 step 2". The plan starts from `271aa1c`, after `v0.16.0` was tagged on `6034536`, as O1
requires; CI on `271aa1c` (run 38305945233) passed. The agent did not commit.

* [0015-MADR-validate-absolute-protocol-paths.md](0015-MADR-validate-absolute-protocol-paths.md), "More Information", "Found here,
  decided elsewhere", capability gates: a dated note that the record was written as the 0017 MADR at the owner's request, before
  0015 was complete, widened by the 0016 audit, building on 0015's hooks and typed error.
* This plan's status is `in-progress`, and its row in `docs/README.md` says so.
* `make check-records` and `make markdownlint` exit 0; the identifier scan of the changed files found nothing. No source file
  changed.

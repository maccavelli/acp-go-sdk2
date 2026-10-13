---
status: proposed
date: 2026-10-13
associated-madr: "0019-MADR-remediate-debugging-pass-findings.md"
---

# Implement phase 1 of 0019: the behaviour fixes, with no exported change

Associated MADR: [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md)

## Goal

Fix the behaviour defects of phase 1 in the MADR's outcome, each with a test seen to fail first, and change no exported identifier:
B1, B3 (with decision 13), B4, B5, B6, B7, B8, B9, B10, B11, B12, B13, B15, G1, and the example fixes E1 to E4. Nothing is released
after this phase (decision 12): phase 1 and phase 2 ship together as `v0.18.0`.

## Standards this plan follows

The ACP schema is 1.25.0 (`schema/version`), the latest stable release: tag `schema-v1.25.0`, published 2026-10-09, and the four
files under `schema/` are byte-identical to the tag's `schema/v1/` files. The unreleased upstream `main` changes none of the methods or
definitions this plan touches; it adds an unstable `systemPrompt` and one sentence to `session/fork`'s description. ACP documents are
quoted from `agentclientprotocol/agent-client-protocol` at `e5bcca615c2560a7d707365c773ae768ae595e88` (2026-10-13), as
`docs/protocol/v1/<page>.mdx:<line>`; the stable pages quoted are unchanged between the tag and that commit. JSON-RPC 2.0 is quoted
from <https://www.jsonrpc.org/specification>. Every quotation was checked, verbatim and at its line, against a downloaded copy (17 of
17 hold).

| Id | Source | Text | Bears on |
| :--- | :--- | :--- | :--- |
| S1 | `prompt-turn.mdx:565` | "The Client **MUST** respond to all pending `session/request_permission` requests with the `cancelled` outcome." | B4 |
| S2 | `prompt-turn.mdx:569` | "the Agent **MUST** respond to the original `session/prompt` request with the `cancelled` [stop reason]" | B4, B5 |
| S3 | `prompt-turn.mdx:580` | "it **MUST** ensure that it does so before responding to the `session/prompt` request" (updates after a cancel) | B1 |
| S4 | `prompt-turn.mdx:586` | "Once a prompt turn completes, the Client may send another `session/prompt`"; nothing allows, refuses or queues a second prompt while one runs | B5 |
| S5 | `session-setup.mdx:310` | "The Agent **MUST** cancel any ongoing work for that session as if [`session/cancel`] had been called, then free the resources associated with the session." The schema's `CloseSessionRequest` says the same (`schema/schema.json:5299`). | B4 |
| S6 | `session-setup.mdx:533` | "Before using HTTP or SSE transports, Clients **MUST** verify the Agent's capabilities during initialization" | B6 |
| S7 | `session-delete.mdx:89` | "Behavior for deleting an active session is implementation-defined." | B4 (delete is not changed) |
| S8 | `cancellation.mdx:18` | On `$/cancel_request`, an implementation "**MAY** cancel the corresponding request activity" and must answer the request | B11 |
| S9 | `overview.mdx:228` | "Notifications never receive responses (success or error)" | B12 |
| S10 | `overview.mdx:232` | The envelope fields "follow the JSON-RPC 2.0 specification." | B3, B12 |
| S11 | `extensibility.mdx:113` | "implementations **SHOULD** ignore unrecognized notifications." | B12 |
| S12 | `schema/schema.json:5017` | The `http` variant of `McpServer`: "Only available when the Agent capabilities indicate `mcp_capabilities.http` is `true`." (`sse` likewise); the description belongs to the variant wherever it appears, `session/fork` included | B6 |
| S13 | `schema/schema.json:164` | `RequestPermissionRequest`: "If the client cancels the prompt turn via `session/cancel`, it MUST respond to this request with `RequestPermissionOutcome::Cancelled`." | B4 |
| J1 | JSON-RPC, Request object | `params`: "This member MAY be omitted." | B3 |
| J2 | JSON-RPC, Parameter Structures | "If present, parameters for the rpc call MUST be provided as a Structured value." | B10, decision 13 |
| J3 | JSON-RPC, Notification | "The Server MUST NOT reply to a Notification" | B12 |
| J4 | JSON-RPC, Parameter Structures | "The absence of expected names MAY result in an error being generated." | B3 |

Where the sources are silent, this plan's choice is SDK policy and is documented as such: what happens to the client's pending
permission requests when it closes a session (S5 restates only the agent's duty); a second prompt while one runs (S4); a reused
request id (JSON-RPC says nothing on uniqueness); a request-only method sent without an id, or a notification-only method sent with
one (neither JSON-RPC nor ACP addresses it). ACP's schema declares each envelope's `params` as the params union or `null` and requires
only `id` and `method` (`schema/schema.json:242`, `:4625`), against J2; the owner's decision 13 settles a received `null`.

## Facts this plan rests on

Each code fact was asserted, snippet at line, on `d0c8d74` (64 of 64 hold). Each defect was reproduced on `d0c8d74` in a scratch copy:
by a probe test that fails while the defect is present, run with `go test -race -count=1`; by an engine behaviour case; or by running
the built example programs. Neither the probes nor the scripts are committed; step by step, the committed tests below replace them.

| Finding | The code as it is | Reproduction on `d0c8d74` |
| :--- | :--- | :--- |
| B1 | `handleResponse` takes `watermark := c.lastEnqueuedNotificationSeq` (`connection.go:1088`); `sendRequest` and `SendRequestNoResult` wait for it (`:1222`, `:1484`); a notification's handler runs with only `receiptKey` in its context (`:1050`). | `TestProbeNotificationHandlerRequestDeadlock` fails: "request sent from a notification handler did not complete", `-32603` "context deadline exceeded" after 2 s. |
| B3 | The dispatch decodes with `json.Unmarshal(params, &p)` (`cmd/generate/internal/emit/dispatch_helpers.go:30`); omitted params are nil. | `logout` and `session/list` without `params` answered `-32602` "unexpected end of JSON input"; no handler ran. |
| Decision 13 | `requestFault` refuses any `params` not starting `{` or `[` (`connection.go:835`); the README states it (`README.md:358`). | The same probe: `"params": null` answered `-32600` "params must be an object or an array". |
| B4 | The agent's `session/close` case only calls `CloseSession` (`agent_gen.go:459`); the client's `Cancel` answers pending requests (`client_gen.go:409`), its `CloseSession` (`:413`) does not; the rule names only the capability (`cmd/generate/internal/model/acp.go:56`). | `TestProbeS_CloseSessionLeavesTurnRunning` and `TestProbeS_ClientCloseSessionLeavesPermissionPending` fail. |
| B5 | A cancel that finds a stored turn cancels and deletes it and records nothing (`emit/dispatch.go:95`); a stored prompt deletes the kept cancel whatever its receipt (`:58`). | `TestProbeS_CancelMissesUnstoredEarlierPrompt` and `TestProbeS_LateCancelConsumedByOlderPrompt` fail. |
| B6 | Capability rules resolve with `m.Type(rule.Definition)` (`model/gates.go:77`); the merge copies a changed unstable definition as `"Unstable" + name` (`load/merge.go:103`); `UnstableForkSessionRequest.McpServers` is `[]UnstableMcpServer` (`types_gen.go:16522`) and its `gates` checks only `additionalDirectories` (`:20171`). | `TestProbeS_ForkMcpServersUngated` fails: `session/new` with an http server refused, `session/fork` with it sent and handled. |
| B7 | A cancel with no turn records `lateCancels[session] = s` (`emit/dispatch.go:98`); only a stored prompt deletes it (`:58`). | `TestProbeLateCancelsGrow` fails: 500 entries after 500 cancels. |
| B8 | `toReqErr` returns `re` whenever `errors.AsType[*RequestError]` matches (`errors.go:199`), a typed nil included; `(*RequestError).Error` is nil-safe (`:21`). | `TestProbeS_TypedNilRequestError` fails: the handler's call answered `null`. |
| B9 | `sendRequest` and `SendRequestNoResult` write before looking at the context (`connection.go:1213`, `:1475`); `SendNotification` looks first (`:1498`). | `TestProbeCancelledCtxStillWritesRequest` fails: the request and its `$/cancel_request` both written. |
| B10 | `prepareRequest` and `prepareNotification` set `msg.Params = b` for any marshalled `params` (`connection.go:1263`, `:1595`). | `TestProbeNilMapParamsSentAsNull` fails: `"params":null` sent, answered `-32600`. |
| B11 | `c.inflight[idKey] = cancel` (`connection.go:899`) overwrites; each request's deferred `delete(c.inflight, idKey)` (`:908`) removes whatever is there. | `TestProbeDuplicateInboundIDLosesCancel` fails. |
| B12 | `handleInbound` treats a message without id as a notification whatever its method (`connection.go:1143`); tests call `handle` directly with an unmarked context (`paths_test.go:427`). | `TestProbeRequestMethodAsNotificationHoldsQueue` fails; `TestProbeS_PromptAsNotification` fails: `session/cancel` with id 9 answered `"result":null`. |
| B13 | `Negotiated` returns `*n` (`agent_gen.go:82`, `client_gen.go:76`); the record is the handler's and the caller's own values (`agent_gen.go:242`, `client_gen.go:320`). | `TestProbeS_NegotiatedAliases` fails: a gate that passed refused after writing through the returned copy. |
| B15 | Five sites log `"raw", string(line)` (`connection.go:849`, `:856`, `:866`, `:975`, `:999`). | A new probe, `TestProbeB15_RawContentLogged`, fails on all four malformed lines: the marker string appears in the log. |
| G1 | `commentText` passes a line starting `//` or `/*` through (`cmd/generate/internal/emit/ast.go:228`). | An engine behaviour case whose description holds `/* x */ func Injected() int { return 42 }` passes `TestEngineBehaviour`: the function was generated, compiled and ran. |
| E1 | The permission prompt ignores the read error (`example/client/main.go:35`, `example/claude-code/main.go:53`, `example/gemini/main.go:53`). | The client with stdin at EOF still ran after 10 s, having printed 1,895,570 permission prompts. |
| E2 | The REPL reads stdin with its own scanner (`example/claude-code/main.go:237`, `example/gemini/main.go:258`) on the goroutine that blocks in `Prompt`. | `:cancel` typed during a turn: the turn ran to its end. A piped "1" meant for the permission prompt was swallowed and the prompt spun 8,659,951 times until killed at 12 s. |
| E3 | `signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)` (`example/agent/main.go:397`), then only `<-asc.Done()` (`:424`). | Still running 3 s after SIGINT; exited 0 only when stdin closed. |
| E4 | The agent sends `/project/README.md`, `/project/config.json` and `/home/user/project/config.json` (`example/agent/main.go:257`, `:302`, `:317`); the receiver validates with the host's `filepath.IsAbs`, and on Windows it returns false when a path has no volume name (go1.27.2 `src/internal/filepathlite/path_windows.go:184-188`). The session's roots are kept (`:168`). | Read; the step 8 run on Windows is its first execution. |

**A downstream consumer.** One module in the owner's workspace requires `v0.17.1`. Read-only, its use bears on three findings: its
agent's `CloseSession` closes the session's log and MCP servers while the SDK leaves a running turn going (B4); its production
logger is passed to the SDK, so B15's content would reach its logs; and it hands the connection to its agent after the constructor,
behind a mutex and nil checks (B2, phase 2). Its notification handlers send no request, so B1 does not reach it today.

## Scope

In scope: the findings above, in `connection.go`, `errors.go`, the generator (`cmd/generate/internal/model`, `internal/emit`, the stub
in `cmd/generate/main_test.go`), the regenerated `*_gen.go` files, tests, engine cases, `example/agent`, `example/client`,
`example/claude-code`, `example/gemini`, `README.md`, `docs/architecture.md`, and the records.

Out of scope: every exported change (phase 2: B2's hand-off, B14, B16 to B21, I8); the generator hardening of phase 3; the tooling
of phase 4, including committed example smoke tests (E12); idioms and performance (phase 5). A release, a tag, release notes.

## Design

Each fix states what changes, where, and the tests that prove it. Every new test is seen to fail on a planted break in a scratch copy
before it is relied on; the break named is the one to plant.

### B1. A request from a notification handler does not wait for that handler

* `connection.go`: a new context key, `messageKey{}`, holding a `messageMark{notification bool; seq uint64}`. `processNotifications`
  builds each handler's context with `receiptKey` and `messageKey{notification: true, seq: queued.seq}`; the request goroutine
  (`:888`) adds `messageKey{notification: false}`. A helper `markOf(ctx) (messageMark, bool)` reads it.
* `waitNotificationsUpTo(ctx, target)` gains the rule: when `ctx` carries a notification mark `seq` and, under `notifyMu`,
  `completedNotificationSeq < seq` (that notification has not completed), the target is capped at `seq - 1`. Notifications before
  `seq` have completed by then, since the queue runs one at a time, so the capped wait returns at once. A context whose notification
  has completed waits as before. The response barrier of `TestSendRequest_WaitsForPreResponseNotification` and its neighbours is
  unchanged for every other caller.
* Limit, documented on `MethodHandler` and in the README's Cancellation section: the cap applies to a request sent under the
  handler's context or one derived from it; a request a notification handler sends under an unrelated context, such as
  `context.Background()`, still waits for the handler.
* Tests, `connection_notification_barrier_test.go`: `TestRequestFromNotificationHandler`, a client whose `SessionUpdate` calls
  `CallExtension` with its context, and with `context.WithTimeout` of it, both answered within the test's deadline; and
  `TestBarrierAfterOwnNotification`, a goroutine started by the handler and sending after the handler returned still waits for
  notifications before the response. Planted break: drop the cap; the first test times out.

### B3 and decision 13. Omitted and null params

* `connection.go`: `receiveLine` treats `params` of exactly `null` as absent (`msg.Params` nil); `requestFault` refuses a `params`
  that is present and is not an object or array, `null` excepted. A new unexported `decodeParams(params json.RawMessage, p any) error`
  decodes absent params as `{}`.
* Generator: `unmarshalValidate` (`emit/dispatch_helpers.go:27-37`) emits `decodeParams(params, &p)` in place of `json.Unmarshal`. The
  stub in `cmd/generate/main_test.go` declares `decodeParams`.
* A request whose params type has required keys and arrives without params is then refused by the type's own decoder, `-32602`
  naming the missing key (J4). The requests with no required property are `LogoutRequest` and `ListSessionsRequest` in the stable
  schema, and `ListProvidersRequest` and `StartNesRequest` in the unstable one.
* Tests, `jsonrpc_test.go`: `TestOmittedParams` (`logout` and `session/list` without params reach their handlers; `session/new`
  without params is `-32602` naming `cwd`); `TestNullParams` (the same three with `"params": null` behave identically; `"params":
  "bar"` stays `-32600`); the existing `TestJSONRPCErrorAnswers` still passes. Planted breaks: emit `json.Unmarshal` again; keep `null`
  in `requestFault`.

### B4. `session/close` cancels the session's work

* Protocol table: a new `MethodRule` role, `ClosesSession`, with a `SessionKey`, set on `session/close` (`model/acp.go:56`) with the
  S5 sentence. `checkRoles` allows it on the agent side only, with a session key; `linkPromptTurns` requires it to be a request and
  records its session field; `linkCancelledRequests` marks it, like the prompt-turn cancel, as answering the client's pending
  requests (`CancelsPending`).
* Agent side: the generated `session/close` case runs the cancel bookkeeping of `cancelActivePrompt` (with B5 and B7) before calling
  `CloseSession`, so a turn received before the close is cancelled with `ErrPromptCancelled` and answered `cancelled` (S2, S5).
  `session/delete` is not changed (S7).
* Client side: `CloseSession` sends the request and, when the agent answers success, cancels the session's pending
  `session/request_permission` requests, which are then answered `cancelled` (S1, S13). On an error answer the turn may still be
  running, so nothing is cancelled. This is SDK policy where S5 is silent, consistent with `Cancel`.
* Tests, `cancel_test.go`: `TestCloseSessionCancelsTurn` (a running turn's context ends with `ErrPromptCancelled`, the prompt is
  answered `cancelled`, the close succeeds); `TestCloseSessionAnswersPendingPermissions`; `TestFailedCloseLeavesPermissionsPending`.
  Model: `TestCheckRoles` cases for `ClosesSession` on the client side, without a session key, and on a notification. Engine case
  `accept-session-close-cancels.txtar` with a behaviour test. Planted breaks: drop the bookkeeping from the close case; drop the
  pending cancel from the wrapper.

### B5 and B7. Cancels ordered against every prompt, and kept only while they can matter

* A kept cancel can apply only to a prompt received before it and not stored yet, which is an in-flight request. The connection
  records the receipt of every in-flight request (`null` id included) in `inflightReceipts`, added in the receive loop before the
  request's goroutine starts and removed when it returns, and exposes `oldestInflight() (uint64, bool)`.
* Cancel (generated `cancelActivePrompt`, used by `session/cancel` and `session/close`): when it cancels a stored turn, it also keeps
  its receipt for the session (B5, case a); in both branches it keeps the receipt only if some in-flight request is older than it,
  and on each keep it removes every kept receipt not newer than the oldest in-flight request (B7).
* Prompt (generated `promptCase`): it is cancelled when a kept cancel is newer than it, as now, and removes the kept cancel only
  when the prompt is newer than the cancel (B5, case b). A prompt stored after a newer one is superseded, as now, so removal is safe.
* The map then holds at most one entry per session cancelled during the life of the oldest in-flight request.
* Tests, `cancel_test.go`: `TestCancelOrderedAcrossTwoPrompts` (both of the probe's orders, with receipts); `TestLateCancelsBounded`
  (500 cancels with nothing in flight keep nothing; with one request in flight they are kept, and after it returns the next cancel
  sweeps them); the existing `TestPromptCancelRightAfterPrompt` and `TestPromptSupersededKeepsSuccessorCancel` still pass. Planted
  breaks: delete the kept cancel unconditionally; drop the sweep.

### B6. Capability rules reach unstable copies of their definition

* `model/gates.go` (`resolveGates`): a rule applies to its definition's type and to every type whose `Origin.Key` is that definition
  in the unstable schema (`Origin` is recorded by the merge). Today only `McpServer` has such a copy, `UnstableMcpServer`, with the
  same `http`, `sse` and `acp` variants; its `stdio` variant is the stable `McpServerStdio`, so no path rule changes.
* Regenerated: `UnstableMcpServer` gains `gates`, and so does every type that holds it, `UnstableForkSessionRequest` included.
* Tests: `gates_test.go` gains `session/fork` cases for `http`, `sse` and `acp` beside the `session/new` ones (`:435-436`), refused on
  sending and answered `-32602` on receipt; `model/gates_test.go` `TestCapabilityRulesApply` gains a schema with an unstable copy;
  `TestProtocolTableAppliesToCommittedSchema` requires each rule to reach every copy. Planted break: resolve on `m.Type` alone.

### B8. A typed-nil `*RequestError`

* `errors.go` `toReqErr`: a matched `*RequestError` that is nil is answered `-32603` with data "handler returned a nil
  *RequestError", not success.
* Test, `errors_test.go`: `TestToReqErrTypedNil`, and through a handler. Planted break: return the match unchecked.

### B9. A request whose context has ended is not written

* `sendRequest` and `SendRequestNoResult` look at `ctx` before taking an id; an ended context returns what `waitForResponse` returns
  for it (`toReqErr(context.Cause(ctx))`), and nothing is written.
* Test, `connection_cancel_test.go`: `TestRequestWithEndedContextWritesNothing`, for both, the wire empty. Planted break: drop the
  check.

### B10. `null` is never sent as params

* `prepareRequest` and `prepareNotification`: params that marshal to `null` are left out; params that marshal to anything but an
  object or an array are refused `-32602` before writing (J2).
* Test, `jsonrpc_test.go`: `TestParamsNeverSentAsNull` (a nil map, a nil pointer to a generated type and `json.RawMessage("null")`
  send no `params`; a string is refused and nothing is written). Planted break: keep `msg.Params = b` for `null`.

### B11. A reused in-flight id keeps the later request's cancel

* `inflight` holds a pointer per request; the deferred removal deletes only its own entry; a reused id is logged at Debug. The later
  request is the one `$/cancel_request` reaches, as an overwrite does today (S8 makes cancelling optional; JSON-RPC is silent on
  reuse).
* Test, `connection_cancel_test.go`: `TestDuplicateInflightIDKeepsLaterCancel`. Planted break: delete unconditionally.

### B12. A message is handled as the kind its method is

* The generated dispatch reads `markOf(ctx)`. A request-only method marked as a notification returns `NewInvalidRequest` naming the
  method, which `handleInbound` logs, and the handler is not called (J3: nothing is answered). A notification-only method marked as a
  request returns `NewMethodNotFound`, answered `-32601`. An unmarked context, a test's direct call, passes as today
  (`paths_test.go:427`).
* Each side knows its methods' kinds from the schema: `mcp/message` is a notification to the agent and a request to the client
  (unstable), and is checked per side.
* Tests, `jsonrpc_test.go`: `TestMethodKindMismatch` (a `session/prompt` without id runs no handler and the `session/cancel` after it
  is handled at once; `session/cancel` with an id is answered `-32601`; both directions of `mcp/message`). Planted break: drop the
  emitted check.

### B13. The negotiation record is the SDK's own

* The record is stored as the JSON form of what was sent and received: each side marshals the request and the response it acted on,
  decodes them into a new `Negotiation`, and stores that, so no pointer is shared with a handler or a caller. `Negotiated` returns a
  fresh decode of the stored record each call. A record that does not round-trip is not stored: the agent answers `-32603` and the
  client returns the error, as for a version refused. `TestConformanceRoundTrip` proves wire-to-Go-to-wire equality for every type,
  not the Go-to-Go identity a copy would need, so the record is defined by its JSON form on purpose: it is what the peer sees.
* Tests, `negotiation_test.go`: `TestNegotiatedIsACopy` (writing through the returned record, through the handler's response, and
  through the request a client passed changes no gate). Planted break: return `*n` again.

### B15. Malformed input is logged without its content

* The five sites log the fault, the size (`"size", len(line)`) and, where valid, the id, never the line.
* Test, `jsonrpc_test.go`: `TestMalformedInputNotLogged`, the probe's four lines, asserting a marker string never reaches the log and
  the fault does. Planted break: restore `"raw"` at one site.

### G1. Descriptions cannot become code

* `commentText` always writes `"// " + line` (`"//"` for an empty line). Regenerating changes no committed file, which proves no
  description or emitted comment used the pass-through.
* Tests: `emit` unit test `TestCommentTextAlwaysComments`; engine case `accept-description-comment-markers.txtar` whose `want.api`
  has no `func Injected`. Planted break: restore the pass-through; the engine case fails on its API.

### E1 to E4. The examples

* E1: one goroutine per program reads stdin into a channel; the permission prompt selects on it and on `ctx.Done()`, and answers
  `cancelled` at EOF or when the context ends.
* E2: in the REPL examples, `Prompt` runs on its own goroutine under a cancellable context; lines read during a turn go to a waiting
  permission prompt if there is one, `:cancel` cancels the turn's context (which sends `session/cancel`), and other lines are refused
  with a message.
* E3: the example agent waits on `ctx.Done()` or `asc.Done()`, and lists only `os.Interrupt`.
* E4: the example agent builds its tool-call paths with `filepath.Join` under the session's first root, its `cwd`.
* No committed test (phase 4, E12). Each is proved by running the built programs: the client with stdin at EOF ends within 5 s
  with the permission answered `cancelled`; the REPL's `:cancel` ends a turn early with `cancelled`, and a piped answer reaches the
  permission prompt; the agent exits within 2 s of SIGINT; the example pair completes a turn on Windows.

## Implementation Steps

Each step ends with its gates and an execution record; the owner commits it. Agents do not commit on `main`.

1. **Records.** This PLAN, the MADR's decision 13, its pointer in
   [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md), and the
   `docs/README.md` row. Gates: `make check-records`, `make markdownlint`, the identifier scan.
2. **Runtime-only fixes:** B1, B8, B9, B10, B11, B15, and decision 13's `receiveLine` and `requestFault` change, in `connection.go`
   and `errors.go`, with their tests. No generator change; regenerating changes nothing.
3. **Dispatch decoding and kinds:** B3 (`decodeParams`, the emitter, the stub) and B12 (the emitted checks; the marks of step 2), with
   their tests. Regenerated `agent_gen.go` and `client_gen.go`.
4. **Prompt-turn bookkeeping:** B5, B7 (`inflightReceipts`, `oldestInflight`, the emitted cancel and prompt changes) and B4 (the
   `ClosesSession` role, the agent case, the client wrapper), with their tests and engine case.
5. **Gates on unstable copies:** B6, with its tests. Regenerated `types_gen.go`.
6. **Negotiation record:** B13, with its test.
7. **Description comments:** G1, with its tests; regenerating changes nothing.
8. **Examples:** E1 to E4, proved by the program runs above, the Windows pair included.
9. **Documentation:** `README.md` ("Errors on the wire": omitted and `null` params, kinds; "Cancellation": `session/close`, a request
  from a notification handler; logging without content) and `docs/architecture.md` (the marks, the capped barrier, the kept cancels,
  the record's JSON form, rules on unstable copies).
10. **Verification and close:** the gates over the tree, the probes re-run (each must now pass), the API listing, Windows; after CI
    passes on the pushed commits, this PLAN and its index row become `complete`. No tag (decision 12).

## Verification

Every gate runs from a Python script outside the tree that runs each command with an argument list and records its exit status
before reading its output. Per code step:

* regenerate (`cmd/generate`, `go run .` with `GOPATH` and `GOMODCACHE` unset), then the diff of the generated files is exactly the
  step's;
* `make pre-add-check` (`gofmt`, `golangci-lint` for the host and `GOOS=windows` in both modules, `go vet`, `go test -race`, `go mod
  tidy -diff`, `govulncheck`), and `scripts/go-precheck.sh` on each new `.go` file;
* `GOEXPERIMENT=nojsonv2 go test ./...` at the root;
* `GOOS=windows go vet ./...` at the root and in `cmd/generate`;
* each new test seen to fail on its planted break in a scratch copy, with the failure recorded;
* the exported API listing of `v0.17.1` and of the tree: no difference in any step;
* `make check-records`, `make markdownlint`, and the identifier scan of the changed files;
* `go test -count=1 -shuffle=on ./...` at the root and in `cmd/generate` on the owner's Windows host, the tree copied there by a
  Python script.

Phase acceptance: every finding above has a committed test or a recorded program run; the phase 1 probes all pass on the tree; the API
listing equals `v0.17.1`'s; CI passes on the pushed commits.

## Rollout and Rollback

Nothing is released after this phase; phase 2 follows, and `v0.18.0` carries both. The behaviour changes the `v0.18.0` notes must
name: `session/close` cancels the session's turn and the client's pending permission requests; omitted and `null` params are
accepted; a method sent as the wrong kind is dropped or answered `-32601`; a request whose context has ended is not written; params are
never sent as `null`; malformed input is logged without its content. Each step is one commit; rolling back a step is reverting it,
after the steps that build on it (3 and 4 use step 2's marks; 4 uses step 3's dispatch).

## Execution record

Nothing executed yet.

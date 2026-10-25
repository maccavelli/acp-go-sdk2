---
status: accepted
date: 2026-10-25
decision-makers: Repository owner
---

# Add the consumers' protocol helpers, Kind() on four more unions, and an AgentRefusals embed

## Context and Problem Statement

[0035-REPORT-consumer-acp-code-for-the-sdk.md](../reports/0035-REPORT-consumer-acp-code-for-the-sdk.md) sorts the ACP code that
the SDK's two consumers, magic-cli-remote and `<cli-b>`, wrote for themselves. Its "What would follow" lists the work as units.
Unit 1, issue #11, shipped in `v1.21.0`
([0036-MADR-session-update-constructors-kind-and-client-refusals.md](0036-MADR-session-update-constructors-kind-and-client-refusals.md)).
The owner asked for units 2 and 3 as one record:

* **Unit 2,** additive helpers in the root package: A1 paging `session/list` and reading its timestamp, A2 choosing a permission
  option, A3 classifying an error, A5 reading a namespaced `_meta` object, B1 resolving a bare stdio MCP command, B2 the auth methods
  a client without `auth.terminal` may be offered, C2 a config option by category and grouped options, and C8 counting refused
  messages.
* **Unit 3:** A4 for unions other than `SessionUpdate`, and the agent half of B3, an embed for the `Agent` methods an agent may leave
  unadvertised.

Each item is code that both consumers, or one consumer and any other, write against a rule the protocol or this SDK fixes. In
several, one consumer's copy is wrong (the report's A1 to A5). The question is which of these the SDK provides, in what form, and
under which names.

Findings, from a read-only investigation at `ea09a07`:

* **R1. `session/list` pages.** `ListSessionsResponse` carries `NextCursor *string` and `ListSessionsRequest` takes `Cursor *string`
  (`types_gen.go:7416`, `:7476`). `ClientSideConnection.ListSessions` (`client_gen.go:910`) sends one page. The method is gated on
  the agent's `sessionCapabilities.list` (`cmd/generate/internal/model/acp.go:57`): sent to an agent that did not advertise it, it
  fails with a `*CapabilityError` and writes nothing. `SessionInfo.UpdatedAt` is a `*string` (`types_gen.go:13242`). `<cli-b>` reads
  one page only; magic-cli-remote follows the cursor and stops on one that repeats (the report's A1).
* **R2. Permission options.** `PermissionOption` has `Kind PermissionOptionKind`, a closed enum of `allow_once`, `allow_always`,
  `reject_once` and `reject_always` (`types_gen.go:10022`, `:10082`). `RequestPermissionResponse.Outcome` is a union of `Cancelled`
  and `Selected`, and `Selected` carries `OptionId` (`:11564`, `:11608`, `:11813`). magic-cli-remote picks an option by substring on
  the kind or the name, so an option named "Don't allow" of kind `reject_once` is taken as an allow (A2).
* **R3. Errors.** `*RequestError` has `Code`, `Message` and `Data any` (`errors.go:18`); `NewMethodNotFound` answers `-32601` with
  data `{"method": …}` (`errors.go:219`). A send the peer did not negotiate fails locally with a `*CapabilityError`
  (`errors.go:171`), whose `Field` and `Variant` are empty for a method gate. The schema's codes are `ErrorCodeAuthenticationRequired`
  (`-32000`) and `ErrorCodeRequestCancelled` (`-32800`) (`types_gen.go:6194`, `:6198`); `ErrPromptCancelled` is a cause
  (`errors.go:122`). What this SDK refuses carries its reason in `data.error`. magic-cli-remote compares `-32000` as a literal (A3).
* **R4. `_meta`.** Every `Meta` field is `map[string]any`, which
  [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md) keeps on purpose. `<cli-b>` repeats
  `meta[key].(map[string]any)` at seven sites; magic-cli-remote round-trips through JSON for a typed view (A5).
* **R5. Stdio MCP commands.** The protocol table requires `McpServerStdio.command` to be absolute (`acp.go:152`), and the agent
  refuses a relative one `-32602`. `McpServer` and the unstable `UnstableMcpServer` both hold it as `Stdio *McpServerStdio`
  (`types_gen.go:8116`, `:19471`), in `NewSessionRequest`, `LoadSessionRequest`, `ResumeSessionRequest` and
  `UnstableForkSessionRequest`. `WithInboundRewrite` (`connection.go:578`) gives a hook the decoded params as a pointer before they
  are validated, so it can rewrite the command; a second `WithInboundRewrite` replaces the first. `<cli-b>`'s
  `acpserver/rewrite.go` resolves a bare command with `exec.LookPath` for the three stable requests, and leaves what does not resolve
  for the SDK to refuse. It does not cover fork. The root package imports no `os/exec`.
* **R6. Terminal auth.** The SDK refuses an `initialize` answer that offers a terminal auth method, `AuthMethod.Terminal`, to a
  client whose `ClientCapabilities.Auth.Terminal` is false (`acp.go:188`; `types_gen.go:591`, `:698`). `<cli-b>` filters its list to
  avoid that refusal (B2).
* **R7. Config options.** `SessionConfigOption` is a union of `Select` and `Boolean` (`types_gen.go:12621`).
  `SessionConfigOptionSelect.Category` is a `*SessionConfigOptionCategory`, and its `Options` is a union of `Ungrouped`, a list of
  options, and `Grouped`, a list of groups each with its options (`:12495`, `:12889`, `:12999`, `:13004`, `:13009`).
  magic-cli-remote reads only `Ungrouped`, so an agent's grouped options are lost (C2).
* **R8. Counting.** `Connection.DroppedNotifications` (`connection.go:849`) counts notifications dropped under
  `OverflowDropNewest`. Nothing counts the messages `checkInbound` (`connection.go:2319`) refuses: a rewrite error, a validation
  error the hook did not accept, or a gate. magic-cli-remote counts them through `WithInboundValidation`, which sees validation
  results only (C8).
* **R9. The agent's unadvertised methods.** Of the 12 `Agent` methods (`types_gen.go:22452`), five are gated on the agent's own
  capabilities: `ListSessions` (`sessionCapabilities.list`), `DeleteSession` (`.delete`), `ResumeSession` (`.resume`),
  `CloseSession` (`.close`) and `Logout` (`auth.logout`) (`acp.go:57`–`:72`). `LoadSession` is already optional, through
  `AgentLoader`. The agent's dispatch refuses a gated method it did not advertise before the handler runs. `<cli-b>` stubs
  `DeleteSession` and `Logout` with `NewMethodNotFound(acp.AgentMethod…)`, the second with the comment "the SDK answers -32601 before
  this runs". The other seven are ungated, and every agent implements them.
* **R10. Unions with a discriminator.** `schemaUnions()` lists 24 unions; 17 have a discriminator key. Of those, every variant
  carries its key in only seven: `ContentBlock`, `ToolCallContent`, `RequestPermissionOutcome`, `SessionConfigOption`,
  `SessionUpdate`, and the unstable `PlanUpdateContent` and `UnstableNesSuggestion`. The other ten have a key-less variant, such as
  `AuthMethod.Agent` or `McpServer`'s stdio, or a catch-all `Other`, so the value a `Kind()` would name for it is not the wire's.
  `SessionUpdate.Kind()` exists (0036). Of the four stable unions left, `ContentBlock` is the one both consumers switch on
  (`<cli-b>`'s `blockText`, magic-cli-remote's `blockKind`); the other three are the remaining stable unions of the same shape.
* **R11. Where code may go.** `cmd/generate/engine_test.go`'s `TestRuntimeFilesCoverHandWritten` requires a new hand-written root
  file to be listed as a runtime file or excluded by name, as step 4 of the 0036 PLAN found. `connection.go` is a runtime file,
  compiled into the generator's synthetic cases, and may use no ACP type; `helpers.go`, `errors.go` and `refusals.go` may.
  `errors.go` is a runtime file too, so A3's helpers may name only what it already declares and the generated `ErrorCode` values
  through their numbers.
* **R12. The API gate.** Each addition is compatible under `apidiff`; `make api` against `v1.21.0` would list them, and the release is
  a minor, `v1.22.0`.

## Decision Drivers

* **The owner's rule,** that anything ACP-related and idempotent belongs in the SDK, read as in the report: protocol-generic,
  deterministic, self-contained.
* **Fix, not copy.** Where the consumers disagree, the SDK takes the behaviour the protocol fixes, not either copy.
* **No silent refusals.** An embed must not turn an ungated method into `-32601`.
* **No change to the wire,** to generated files, to the generator, or to `HandWrittenHelpers`.
* **No new dependency,** and no `os/exec` in the root package.
* **Additive under `v1`.**

## Considered Options

* **O1.** Every item of units 2 and 3, hand-written in the files R11 allows, in the report's shapes, with the fixes R1 to R9 name;
  `Kind()` on the four stable unions whose every variant carries its key; and `AgentRefusals` for R9's five methods.
* **O2.** O1, with `Kind()` generated for every union with a discriminator.
* **O3.** Unit 2 only; unit 3 waits.
* **O4.** Nothing; the consumers keep their copies.

## Decision Outcome

Chosen option: "O1", because each item fixes a defect or removes a copy that both consumers would otherwise keep, all of
it hand-written and additive, with no generator change. `Kind()` goes only where it can name exactly what the wire carries (R10).
`AgentRefusals` follows `ClientRefusals`' rule, which keeps every ungated method compile-required (R9).

* O2 would put a generator change, engine cases and the regeneration gates on 17 unions. For the ten with a key-less or catch-all
  variant it would have to invent a value the wire does not carry (R10).
* O3 leaves `<cli-b>`'s agent stubs and the `ContentBlock` switches in both consumers for no saving: unit 3 is the smaller unit.
* O4 leaves A1's single page, A2's misread "Don't allow" and A3's literal `-32000` in the consumers.

### Specification, as proposed

Each new exported name has a doc comment that opens with its name and cites this record. None changes an existing signature.

#### In `helpers.go`

| Name | Behaviour |
| :--- | :--- |
| `func (c *ClientSideConnection) AllSessions(ctx context.Context, req ListSessionsRequest) iter.Seq2[SessionInfo, error]` | Calls `ListSessions` from `req.Cursor`, yields each session, and follows `NextCursor` until it is absent or empty. A cursor already seen yields `(SessionInfo{}, ErrRepeatedCursor)` and stops (question 2). An error from a call is yielded once, and stops. Breaking out of the loop sends nothing more. |
| `var ErrRepeatedCursor` | the error above |
| `func (s SessionInfo) Updated() (time.Time, bool)` | `UpdatedAt` parsed with `time.RFC3339`, which also reads fractional seconds; `false` when absent or not a date-time |
| `func PickPermissionOption(opts []PermissionOption, kinds ...PermissionOptionKind) (PermissionOption, bool)` | The first option of the first of `kinds` present, by `Kind` alone, never by name. `false` when none is, or when `kinds` is empty. |
| `func (r RequestPermissionResponse) Chosen() (PermissionOptionId, bool)` | `Outcome.Selected.OptionId`; `false` for `Cancelled` or `Unknown` |
| `func MetaObject(meta map[string]any, key string) map[string]any` | `meta[key]` when it is a JSON object; nil otherwise |
| `func DecodeMeta[T any](meta map[string]any, key string) (T, bool, error)` | `meta[key]` decoded into `T` through `encoding/json`; `false` when the key is absent; an error when it does not decode |
| `func ResolveStdioCommands(lookPath func(string) (string, error)) func(ctx context.Context, method string, params any) error` | A rewrite hook. For R5's four requests, each stdio server whose command `filepath.IsAbs` refuses is resolved with `lookPath`; a result that is absolute replaces it, and anything else leaves it for the SDK to refuse. The hook never refuses a message. A caller passes `exec.LookPath`. |
| `func OfferedAuthMethods(caps ClientCapabilities, methods []AuthMethod) []AuthMethod` | `methods` in order, without `Terminal` variants unless `caps.Auth.Terminal`; `Agent` and `Unknown` kept; a new slice |
| `func ConfigOptionByCategory(opts []SessionConfigOption, c SessionConfigOptionCategory) (*SessionConfigOptionSelect, bool)` | The first `Select` option whose `Category` is `c` |
| `func (o SessionConfigSelectOptions) All() []SessionConfigSelectOption` | `Ungrouped` as it is, or every group's options in order; nil for neither |
| `func (b ContentBlock) Kind() string`, and the same on `ToolCallContent`, `RequestPermissionOutcome` and `SessionConfigOption` | As `SessionUpdate.Kind()`: the first set variant's value from R10, in `MarshalJSON`'s order; for `Unknown`, its key read as `UnmarshalJSON` reads it, and `"unknown"` when that is not a non-empty string; `""` for the zero value. `unknownSessionUpdateKind` becomes a helper taking the key. |

#### In `errors.go`

* `func IsMethodNotFound(err error) bool`: a `*RequestError` with code `-32601`, or a `*CapabilityError` whose `Field` and `Variant`
  are empty.
* `func IsAuthRequired(err error) bool`: a `*RequestError` with code `-32000`.
* `func IsCancelled(err error) bool`: `context.Canceled`, `ErrPromptCancelled`, or a `*RequestError` with code `-32800`.
* `func (e *RequestError) Reason() string`: `Data`'s `"error"` when it is a non-empty string, else `Message`.

Each looks through wrapping with `errors.As` and `errors.Is`. `errors.go` is a runtime file (R11), so the codes are written as
numbers, each with the `ErrorCode` constant's name in a comment, and a test pins them to the constants.

#### In `connection.go`

* `func ChainRewrites(fns ...func(ctx context.Context, method string, params any) error) func(ctx context.Context, method string,
  params any) error`: runs each in order, and stops at the first error.
* `func (c *Connection) RefusedMessages() uint64`: how many received messages `checkInbound` refused, by the rewrite hook, by
  validation the hook did not accept, or by a gate (question 3). A line that does not decode is answered and logged, as now, and not
  counted.

#### In `refusals.go`

`type AgentRefusals struct{}`, with value-receiver methods for R9's five: `CloseSession`, `DeleteSession`, `ListSessions`,
`ResumeSession` and `Logout`. Each returns its zero response and `NewMethodNotFound(AgentMethod…)` with its own method's constant.
Its doc comment says what `ClientRefusals`' says: the gates answer first when the agent did not advertise the capability; an
embedder's own method of the same name takes precedence; and an agent that implements some of the five for real should implement
all five itself. The seven ungated methods stay compile-required.

#### Documentation

* `README.md`: the helper list names the new helpers; the agent section names `AgentRefusals`, `ResolveStdioCommands` and
  `OfferedAuthMethods`; the client section names `AllSessions` and `PickPermissionOption`.
* `docs/architecture.md`: the hand-written list says what `helpers.go`, `errors.go` and `refusals.go` now hold.
* Runnable examples for `AllSessions`, `PickPermissionOption`, `ResolveStdioCommands` and `AgentRefusals`.

### Questions for the owner

1. **`Kind()` on which unions.**
   * Recommended: the four stable unions whose every variant carries its key, `ContentBlock`, `ToolCallContent`,
     `RequestPermissionOutcome` and `SessionConfigOption`, with a test that fails when a stable union of that kind lacks one.
   * Alternatives: `ContentBlock` alone, the one both consumers switch on; or every discriminated union, generated (O2).
2. **`AllSessions` on a repeating cursor.**
   * Recommended: yield `ErrRepeatedCursor` and stop, so the caller knows the list is incomplete.
   * Alternative: stop silently, as magic-cli-remote does.
3. **What `RefusedMessages` counts.**
   * Recommended: every message `checkInbound` refuses, gates included.
   * Alternative: validation refusals only, as magic-cli-remote counts them through its hook.
4. **The release.**
   * Recommended: `v1.22.0`, once committed and CI has passed; the release notes before the tag; the tag and the push on the owner's
     ask in that turn.
   * Alternative: hold the release for other work.

#### Decided with the owner, 2026-10-25

The owner chose, out of three options for each:

| Question | Answer |
| :--- | :--- |
| 1. `Kind()` on which unions | As recommended: the four stable unions whose every variant carries its key, with the coverage test. Not chosen: `ContentBlock` alone; all 17, generated. |
| 2. `AllSessions` on a repeating cursor | As recommended: yield `ErrRepeatedCursor` and stop. Not chosen: stop silently; a page cap as well. |
| 3. What `RefusedMessages` counts | As recommended: every message `checkInbound` refuses. Not chosen: validation only; malformed lines and unknown methods too. |
| 4. The release | As recommended: `v1.22.0` once committed and CI has passed, the notes before the tag, the tag and push on the owner's ask. Not chosen: hold for more work; a release per step. |

### Consequences

* Good, because A1's single page, A2's misread option, A3's literal code and C2's lost groups are fixed once, here, for every
  consumer.
* Good, because an agent behind a client that sends `"npx"` works with one option, fork included, and without `os/exec` in the
  root package.
* Good, because nothing generated changes and nothing sent on the wire changes.
* Neutral, because `ChainRewrites` exists only because `WithInboundRewrite` keeps one hook; changing that option would change its
  behaviour, and is not proposed.
* Neutral, because the four `Kind()` methods duplicate values the generated `MarshalJSON` writes, as `SessionUpdate.Kind()` does;
  the `schemaUnions()` test keeps them equal.
* Bad, because the root package's exported surface grows by about twenty names, each one a `v1` promise.
* Bad, because a later schema field named `Kind` or `All` on one of these types would stop `make version` until the method is
  renamed, which would break `v1`; no variant or field has either name today.

### Confirmation

The PLAN shows each of the following, every new check first seen to fail on a planted break in a scratch copy:

* `AllSessions` over two pages, a repeating cursor, an error on the second page, a break after the first item, and an agent that did
  not advertise `sessionCapabilities.list`;
* `PickPermissionOption` refusing an option named "Don't allow" of kind `reject_once` when asked for an allow;
* each `Is*` through a wrapped error, and `IsMethodNotFound` false for a value-gate `*CapabilityError`;
* `ResolveStdioCommands` over all four requests, with a fake `lookPath`, and the SDK still refusing what it leaves;
* `OfferedAuthMethods` making an `initialize` answer that the SDK would otherwise refuse pass;
* `Kind()` equal to every `schemaUnions()` value for the four unions, and the `Unknown` and zero cases;
* `AgentRefusals` having exactly five methods, not implementing `Agent` alone, and a refused method reaching the client as `-32601`;
* `RefusedMessages` counting a rewrite refusal, a validation refusal and a gate refusal, and not a malformed line;
* `make api` against `v1.21.0` listing only compatible changes.

## More Information

* [0035-REPORT-consumer-acp-code-for-the-sdk.md](../reports/0035-REPORT-consumer-acp-code-for-the-sdk.md): A1 to A5, B1 to B3, C2
  and C8, and "What would follow".
* [0036-MADR-session-update-constructors-kind-and-client-refusals.md](0036-MADR-session-update-constructors-kind-and-client-refusals.md):
  `SessionUpdate.Kind()` and `ClientRefusals`, whose shape and rules this record follows.
* [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md): `_meta` stays `map[string]any`.
* [0015-MADR-validate-absolute-protocol-paths.md](0015-MADR-validate-absolute-protocol-paths.md): the absolute-path rule and the
  inbound hooks.
* [0025-REPORT-schema-1-26-0-adoption-assessment.md](../reports/0025-REPORT-schema-1-26-0-adoption-assessment.md): why `Kind()` on
  the unstable unions waits for a policy.
* Not in this record: the report's units 4 to 7 (C3, C6; C4, C5, C7; D1; E3, E4).
* The implementation is [0038-PLAN-consumer-helpers-kind-and-agent-refusals.md](0038-PLAN-consumer-helpers-kind-and-agent-refusals.md).

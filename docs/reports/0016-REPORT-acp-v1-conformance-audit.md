# ACP v1 spec adherence audit

Observed 2026-10-12. This report audits the SDK against the published Agent Client Protocol v1 spec as it stands on that date, and records the adherence violations it found.
It decides no fix, amends no record, and plans no phase. Where a violation is already recorded in a decision, it says so.

## Question

Does the SDK adhere to the currently published ACP v1 spec — the schema release `schema-v1.25.0` (2026-10-09, the newest stable schema tag) plus the 19 published v1 book pages — for the protocol
behavior the schema does not express, and for the behavior it does?

## How this was checked

Downloaded the `schema-v1.25.0` release assets (`meta.json`, `schema.json`, `meta.unstable.json`, `schema.unstable.json`) and compared SHA-256 against the local `schema/` files: all four
byte-identical.
Read the published v1 book: `overview`, `initialization`, `authentication`, `session-setup`, `session-list`, `session-delete`, `prompt-turn`, `content`, `tool-calls`, `elicitation`, `file-system`,
`cancellation`, `terminals`, `agent-plan`, `session-modes`, `session-config-options`, `slash-commands`, `extensibility`, and `transports`, and extracted every MUST / MUST NOT / SHOULD requirement.
Inventoried the SDK's behavior in `connection.go`, `agent.go`, `client.go`, `extensions.go`, `errors.go`, the generated `agent_gen.go`, `client_gen.go`, and `types_gen.go`, and the protocol table
`cmd/generate/internal/model/acp.go`, reading every read with a file and line before concluding.

Not done: no `go test` run for this report; no byte-for-byte diff of schema description text; no evaluation of the repo's own conformance tests (`conformance_test.go`) beyond noting they exist; the v1
transport the schema pins is stdio only, so the streamable-HTTP draft was not audited. ACP v2 (`schema-v2.0.0-alpha.8`, a published draft under review) is out of scope.

The four gaps `docs/reports/0001-REPORT-go-127-acp-parity.md` recorded against the `0.13.5` stamp are closed on the 1.25.0 pin: `elicitation/create` and `elicitation/complete`, `session/delete`,
tool-call `name`, `usage_update` and the other twelve `session/update` variants, `model_config`, and error code `-32800` are all present and no longer mislabeled unstable.

## What the SDK gets right

Verified directly, not asserted:

- **Framing.** Newline-delimited JSON-RPC 2.0, one value plus `\n`, mutex-serialized writes with `"jsonrpc":"2.0"` stamped on every outbound message (`connection.go:774-786`). The writer never emits
  an embedded newline.
- **Notification ordering, stronger than the spec.** Notifications are queued with strictly increasing sequence numbers and processed in order on one goroutine (`connection.go:656-674`). A response
  waits on a watermark until every notification enqueued before it has been processed (`connection.go:941-998`). This makes two spec MUSTs structural rather than application duty: `session/load`
  replies only after the entire replay has been delivered, and no `session/update` received for a turn arrives after that turn's `session/prompt` response.
- **Cancellation plumbing.** Inbound `session/cancel` cancels the stored per-session prompt context and still calls `Cancel` (`agent_gen.go:309-326`). `$/cancel_request` cancels the matching request
  context and is handled synchronously, outside notification ordering, for immediacy (`connection.go:541-546`). Outbound, `ClientSideConnection.Prompt` sends `session/cancel` when its context ends
  (`client_gen.go:288-296`).
- **Unknown-variant tolerance.** Unions keep unknown non-`_` discriminators as raw bytes, or as typed `Other` variants that preserve undeclared keys; `Unknown` writes back byte-for-byte. The "preserve
  the raw payload" MUSTs for elicitation modes and actions and the `_`-prefix reservations hold. Unrecognized `_`-extension notifications are silently ignored, per `extensibility.md:116`.
- **Omitted capabilities decode as unsupported.** The schema defaults (`fs` false, `terminal` false, `loadSession` false, `promptCapabilities` false, …) are injected on absent or null, matching "MUST
  treat all capabilities omitted in the initialize request as UNSUPPORTED".
- **Required-field encode.** Nil required arrays and maps emit `[]` and `{}` (33 pinned sites); a required union with no variant fails to encode rather than emitting garbage (0009).
- **`session/set_config_option` fallback.** The `boolean` variant plus the graceful `value_id` fallback for unknown `type` strings match the schema text.
- **Paths on receive.** `Validate()` enforces absolute paths on 17 properties across requests and notifications (0015).
- **Extensibility edges.** `_`-prefixed requests dispatch with a `-32601` fallback, `CallExtension` / `NotifyExtension` reject non-`_` names, `_meta` passes through untouched, `Error` carries no
  `_meta`, and the reserved trace keys are never interpreted by the SDK.

## Violations

Severity: **High** means the SDK's own easy path puts a non-conforming payload on the wire.
**Medium** means a spec MUST is unmet but application code can route around it. **Minor** means a gap that breaks no conforming exchange.

### V1 — High: a cancelled prompt turn is answered with a `-32800` error instead of `stopReason: "cancelled"`

`prompt-turn.md:574-580`: after `session/cancel`, "Agents **MUST** catch these errors and return the semantically meaningful `cancelled` stop reason"; the turn answer is the result
`{stopReason:"cancelled"}`. `-32800` belongs to `$/cancel_request`-style cancellation, not the ACP notification.

`toReqErr` (`errors.go:127-136`) maps `context.Canceled` to `-32800`, and the prompt dispatcher runs the handler on that same context (`agent_gen.go:415-439`). So the idiomatic Go handler that returns
`ctx.Err()` makes the wire say `{"error":{"code":-32800}}` where a conforming peer expects the `cancelled` result. A handler that checks its context and returns `PromptResponse{StopReason:
StopReasonCancelled}` is fully conforming — but the SDK's default coercion steers implementations into the violation. No decision covers it. A remedy would narrow the mapping (for example, to
cancellation initiated through `$/cancel_request`) or document and test the correct pattern instead.

### V2 — Medium: malformed inbound messages get dead air, and no error defined by JSON-RPC is ever answered

JSON-RPC 2.0, which the spec adopts (`transports.md:9`, `overview.md:163-169`), requires `-32700` for invalid JSON and `-32600` for invalid requests. In the SDK:

- A line that fails to decode is logged and dropped; `NewParseError` (`errors.go:92-94`) is defined and never called (`connection.go:536-539`).
- A message with neither `id` nor `method` is logged and dropped; `NewInvalidRequest` (`errors.go:97-99`) is defined and never called (`connection.go:611-613`).
- The inbound `jsonrpc` field is never validated.

Counterweight: on the request side the SDK is strict (`-32601` for unknown methods, `-32602` for decode and validation failures, `-32603` for marshal failures), and a broken line may carry an unusable
id. But a conforming peer that emits garbage gets silence with no error object, which is the exact thing the error-handling section forbids.

### V3 — Medium: capability gates are recorded, not enforced (documented, still open)

The spec attaches MUST-frames to nearly every optional method and update: `session/load` requires `loadSession`; `session/resume`, `session/close`, `session/delete`, `session/list`,
`additionalDirectories`, and `logout` each require their `{}`-typed capability; `fs/*` and every `terminal/*` require the client advertisement; elicitation is gated per mode; `notice` and the
compaction updates require `session.compaction` / `session.notices`; `type:"boolean"` config options require `session.configOptions.boolean`; prompt content beyond text and `resource_link` requires
the matching `promptCapabilities` flag; a `terminal` auth method requires `auth.terminal`.

The SDK stores nothing from `initialize`; outbound wrappers send unconditionally (`client_gen.go:278-281` for `session/load`, `agent_gen.go:493-501` for `fs/*`), and inbound dispatch gates unstable
methods only by Go interface assertion. Recorded in `docs/decisions/0015-MADR-validate-absolute-protocol-paths.md:133-140` as a planned follow-up. This audit extends that record: 0015 names a subset
of the gates, and the full matrix above is the surface.

### V4 — Medium: send-side absolute-path validation is unwired, so the SDK can send relative paths

"All file paths in the protocol **MUST** be absolute" (`overview.md:158-161`). Receive-side is enforced. Send-side has zero checks: the portable rule `isPortableAbs` exists (`connection.go:54-68`)
with no callers, so `NewSessionRequest.Cwd`, `ReadTextFileRequest.Path`, `CreateTerminalRequest.Cwd`, and every other absolute-path property can be serialized relative. Accepted and pending in 0015
(`0015-MADR:117-121`).

### V5 — Medium: protocol-version negotiation is unimplemented and unrecorded

`initialization.md:97-101`: the agent "MUST respond with the same version" it supports and "MUST respond with the latest version it supports" otherwise; the client "SHOULD close the connection and
inform the user" when the answer is unsupported. The dispatchers never compare the negotiated `ProtocolVersion` against `ProtocolVersionNumber` (`agent_gen.go:115-127`); nothing on the client side
checks the echoed version; `ProtocolVersionNumber = 1` (`constants_gen.go:5-6`) is referenced only by the examples. Unlike V3 and V4, this gap is in no decision record.

### V6 — Medium: the client-side cancel obligations around a prompt turn are unmediated

On sending `session/cancel` mid-turn, the Client "**MUST** respond to all pending `session/request_permission` requests with the `cancelled` outcome" and "**SHOULD** preemptively mark all non-finished
tool calls of the turn as `cancelled`" (`prompt-turn.md:552-584`). The SDK's automated cancel path (`client_gen.go:288-296`) sends the notification but tracks neither pending permission requests nor
active tool-call state, so an application that lets the SDK send the cancel leaves the agent waiting on unanswered permission requests unless the application keeps that bookkeeping itself. The
`cancelled` outcome variant exists; no helper synthesizes it.

### V7 — Minor: `-32002` (Resource not found) has no error helper

The schema `ErrorCode` union lists `-32002`; `errors.go` constructs `-32700`, `-32600`, `-32601`, `-32602`, `-32603`, `-32800`, and `-32000` but not it. The raw integer `Code` field can still send it,
so this is convenience parity only.

## Notes — no violation, recorded for the next reader

- The 10 MiB inbound line cap is an SDK addition; the spec states no limit. An overlong line closes the connection without any error response — the sender-visible side of V2.
- The outbound `$/cancel_request` queue overflows at 1024 and silently drops the cancel at Debug level; the spec lets a receiver ignore cancellation, but a dropped send-side cancel is invisible to the
  peer and can leave a request running.
- JSON-RPC arrays fail to decode and are dropped. The v1 book says messages are individual ("no JSON-RPC batching is stated"); v2 adds batch requirements, so this becomes a v2-scope conformance item,
  not a v1 violation.
- Duplicate object keys resolve last-wins (encoding/json v1 semantics); the spec is silent.
- Terminal truncation at a character boundary, cursor opacity in `session/list`, and the form-vs-URL elicitation privacy rules are application duties the SDK types cannot express; not flagged against
  the SDK.
- The SDK supersedes a second `session/prompt` for the same session by cancelling the first that is still running (`agent_gen.go:423-435`). The spec does not define concurrent prompts for one session;
  this is a safety decision, not a violation, but it is worth a sentence in whichever record next touches the prompt turn.

## Answer

The schema-geometry layer is clean: the pin is the newest published stable schema, byte-identical, and the generated code renders it faithfully, with ordering and unknown-value guarantees that exceed
the spec. Every violation found lives in the layer the schema cannot express: the cancellation semantics of the prompt turn (V1, the only High), the JSON-RPC error-response obligations (V2), and the
negotiated-capability family (V3 through V6), of which V5 and V6 are recorded nowhere and V1 is steered, mildly, by the SDK's own default error coercion.

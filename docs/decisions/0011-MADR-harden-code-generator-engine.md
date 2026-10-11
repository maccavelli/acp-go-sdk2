---
status: accepted
date: 2026-10-11
decision-makers: Repository owner
---

# Harden the code generator into a validated, schema-driven engine

## Context and Problem Statement

[0006-MADR-emit-go-with-go-ast.md](0006-MADR-emit-go-with-go-ast.md) replaced Jennifer with `go/ast` and required byte-identical output, so the new engine
inherited every behaviour of the old one. [0009-MADR-fix-generated-output-on-json-v1.md](0009-MADR-fix-generated-output-on-json-v1.md) then fixed four
defects in the output. The owner asked for an analysis of the whole engine, `cmd/generate` at `ccc6e1c`, for bugs, gaps, incomplete wiring, weak heuristics,
and missing hardening, with the goal of an engine that is robust, flexible, consistent, and production ready.

The analysis read all of `cmd/generate` (about 4,400 lines in `main.go` and the packages `load`, `ir`, `emit`, and `util`), ran the generator's tests with
coverage, counted schema keywords and generated shapes with scripts, ran behaviour probes against the generated SDK in a scratch copy, and ran the
generator on small hostile schemas in a scratch directory. Nothing in the tree changed. The findings follow, most severe first. Each says whether it is
live at schema `1.25.0` or latent.

### Wire correctness (live, confirmed by probes)

* **F1. Unknown union variants are decoded as a known variant, silently.** A union's `UnmarshalJSON` switches on the discriminator, but a value it does
  not know falls through to required-key matching and then to "try each variant in turn", where `json.Unmarshal` into a struct almost always succeeds.
  Probes:
  * `ContentBlock` `{"type":"video",...}` decoded as `Text` with `Type: "video"`, no error.
  * `SessionNotification` with `"sessionUpdate":"future_kind"` decoded, and re-encoded as `"sessionUpdate":"user_message_chunk"`. A client would show a
    newer agent's update as a user message.
  * `InitializeResponse` with an auth method of type `future_auth` re-encoded it as `"type":"terminal"`.
  * `RequestPermissionOutcome` `{"outcome":"future"}` decoded as `Cancelled`.
  * `ToolCallUpdate` whose `content` holds one unknown item decoded with no error into a `Content` variant holding an empty `ContentBlock`, which then
    fails to encode with "ContentBlock: no variant set". Decode and encode disagree.
* **F2. `ContentBlock` encoding drops `_meta` and `annotations`.** `contentBlockShape` in `emit/types.go` hard-codes the keys each variant keeps, and
  lists neither. A text block with `_meta` and `annotations` encodes as `{"text":"hi","type":"text"}`, whether built in Go or decoded from a message that
  carried both. Every prompt, message chunk, and tool output carries `ContentBlock`.
* **F3. Integers lose precision through union object variants.** `unionMarshalBody` encodes the variant, decodes it into `map[string]any`, and encodes it
  again; numbers become `float64` on the way. A `resource_link` with `size` 9007199254740993 is sent as 9007199254740992.

### Fidelity to the schema (live)

* **F4. The ACP deserialization keywords are ignored.** `x-deserialize-default-on-error` appears 385 times in the schema and `x-deserialize-skip-invalid-items`
  24 times. They tell a decoder to use the default when a field fails to decode and to drop array items that fail. The generator models neither, so one
  malformed optional field fails the whole message: `InitializeResponse` with `"agentCapabilities":"garbage"` fails to decode.
* **F5. Decode errors lose their cause and their subject.** A variant that fails to decode returns "invalid variant payload" with the underlying error
  dropped (`ContentBlock` with `"text":5`), and a union with no match returns "no matching variant for union" without naming the union.
* **F6. Type mapping drops schema information.** At least 25 integer fields declared `uint64`, `uint32`, or `int64` are generated as `int` or `*int`,
  signed and 32 bits wide on 32-bit platforms; `minimum`, `maximum`, and `minLength` are not checked. Three `additionalProperties` with a typed value
  (`ElicitationContentValue`, `ElicitationPropertySchema`, `string`) become `map[string]any`. `anyOf` of a non-primitive and `null` becomes `any`;
  an `allOf` keeps only its first `$ref`; every `enum` becomes `string`. 18 fields are `any` and 8 are `map[string]any`, besides `_meta`.
* **F7. Validation is shallow.** `Validate` exists on RPC types and on `ToolCallUpdate`, which is hard-coded. It checks only top-level required strings and
  arrays and does not descend into unions or nested types: a `PromptRequest` with one empty `ContentBlock` passes `Validate` and then fails to encode.
  Decoding does not enforce required keys either: a `ContentBlock` image without `data` decodes.

### Robustness of the engine (confirmed on hostile schemas)

* **F8. Invalid input produces invalid output, a panic, or a half-written tree.** On small schemas in a scratch directory:
  * enum values `a-b` and `a_b`: exit 0, output does not compile (`KindAB redeclared`);
  * properties `foo_bar` and `fooBar`: exit 0, `FooBar redeclared`;
  * a `$ref` to a missing definition: exit 0, `undefined: Missing`;
  * a property named `2fa`: exit 1 at the format step, after `constants_gen.go` was already overwritten;
  * a required variant property named `range`: exit 1 at the format step of `helpers_gen.go`, after the other four files were written;
  * a oneOf named `U` with const-discriminated variants: a panic, `BUG: attempting to emit unregistered type: "Ux"`.
  Files are written one by one with `os.WriteFile`; nothing is checked before the first write, and 8 `panic` calls stand in for errors.
* **F9. Names are computed in several places with different rules.** The panic above is a name registered as `UX` and then re-cased to `Ux` by
  `util.ToExportedField`, whose `TitleWord` lowercases everything after the first letter. `emit/helpers.go` repeats the variant-naming logic of the types
  emitter with its own `usedTypeNames`, so a collision resolved in one need not be resolved the same way in the other. `splitCamelCase` in `emit/types.go`
  and `util.SplitCamel` split differently; `toExportedConst` and `util.ToExportedField` differ. Collisions between constants, fields, and types are never
  detected, and Go keywords are never escaped.
* **F10. Dispatch guesses where the model already knows.** `writeAgentGen`, `writeClientGen`, `emitOutbound`, and `addRPCMethod` derive a response type by
  `TrimSuffix(Req, "Request") + "Response"` instead of the IR's `Resp`, and `ir.IsNullResponse` treats a missing definition as a null result, so a
  response type that does not follow the naming rule would be dropped without an error. A method in `meta.json` with no matching types is skipped
  silently. `meta.ProtocolMethods` is loaded and never emitted; `connection.go` handles `$/cancel_request` with a string literal.

### Heuristics and hard-coded protocol knowledge (live)

* **F11. The unstable merge does not do what its comment says.** `MergeStableAndUnstable` says stable definitions stay unchanged, but
  `promoteChangedSharedDefs` replaces 8 of them with their unstable versions: `AgentCapabilities`, `ClientCapabilities`, `InitializeResponse`,
  `McpCapabilities`, `McpServer`, `PromptResponse`, `SessionCapabilities`, `SessionUpdate`. 56 unstable-only types carry the `Unstable` prefix and 46 do
  not (`PlanUpdate`, the `Nes*` capabilities, and others), so a name does not tell a user whether a type is stable. `ir.isUnstableMethod` decides by
  whether a description contains the word "unstable".
* **F12. Protocol knowledge is spread through the engine as literals.** About 25 string literals name wire methods or types: `session/prompt` (prompt
  cancellation), `session/cancel`, `session/update`, `session/load` (`AgentLoader`), the `ContentBlock` shapes, `ToolCallUpdate` validation, a helper skip
  list (`ContentBlock`, `ToolCallContent`, `SessionUpdate`), and `terminal_wait_for_exit`. The discriminator of 9 unions is guessed as the first `const`
  property in sorted order; 7 declare one. A variant is emitted as `json.RawMessage` when its description contains "extension" or its title contains
  "Ext"; no variant hits that at `1.25.0`.

### Consistency, dead code, and tests

* **F13. Dead and stale code.** `ir.BindClientTerminal` is never assigned, so the `ClientTerminal` branches in `emit/types.go` and
  `emit/dispatch_helpers.go` cannot run. `ir.inferTypeBaseFromMethodKey` has 0% coverage at `1.25.0`. `util.SanitizeComment` is deprecated and unused.
  `defaultKindOf` has a `fmt.Sprint` fallback that JSON decoding cannot reach. A comment in `util.FormatDocComment` still refers to Jennifer.
* **F14. Small inconsistencies in the output.** `omitempty` on 8 struct-valued fields has no effect in v1; `Validate` receivers are `t`, `v`, or `u`;
  the RPC interface types are not separated by blank lines; `RequestIdNull` has no doc comment; generated files are written with mode `0o600`.
* **F15. The layout engine leans on go/printer internals.** `file.go` positions tokens so `go/printer` places comments and line breaks, estimating offsets
  with `lineBytes` and a per-body reservation; `TestLayoutStaysBelowPrinterSentinel` guards the known limit. `firstPos` handles 8 expression kinds; an
  element that starts with an index, binary, paren, function-literal, array, or map expression gets no position.
* **F16. Tests prove the current output, not the engine.** With all of `cmd/generate`'s tests counted across its packages, 82.5% of the generator's statements run, most of them through
  `TestGeneratedFilesUpToDate`. Run alone, the
  `emit` package's tests cover 25.7%, and `ir` and `util` have none. No test runs the generator on a synthetic or hostile schema, round-trips every union,
  or checks unknown-variant handling.

## Decision Drivers

* What the SDK sends and accepts must match the schema, including the ACP deserialization keywords, and must not change a peer's data.
* A newer peer must not be able to make this SDK misread a message.
* The generator must refuse a schema it cannot represent, with an error naming the definition, and must never leave the tree half-written.
* One place computes each name, each type, and each union shape; emitters only render.
* Protocol behaviour that is not in the schema lives in one declared table, not in literals across the engine.
* The exported API changes only where a decision says so; breaking changes wait for a version that allows them.
* No new dependency ([0007-MADR-self-contained-acp-sdk.md](0007-MADR-self-contained-acp-sdk.md)); `encoding/json` v1 stays.

## Considered Options

* Fix the findings in place, one at a time, keeping the engine's structure
* Restructure around a resolved, validated schema model, in phases
* Replace the generator with a third-party JSON Schema code generator
* Leave the engine as it is

## Decision Outcome

Chosen option: "Restructure around a resolved, validated schema model, in phases", because F6 to F12 share one cause, decisions taken late and in several places, which point fixes would
keep, and the owner chose to fix F1 to F3 once, inside the new model, rather than first on the current engine and again after.

The owner accepted this option on 2026-10-10. Asked before acceptance, the owner decided:

* **Order:** the resolved model first, then the wire fixes built on it. The proposal had put the wire fixes first, on the current engine.
* **Unknown variants:** each discriminated union gets an exported `Unknown json.RawMessage` field. A discriminator value that names no variant decodes
  into it without error and encodes back unchanged. (Refined 2026-10-10 by the owner, see the phase 2 amendment below: the six unions with a schema
  catch-all use it instead, and the field is on the other eleven.)
* **Types:** generate the declared integer formats and typed `additionalProperties` now. The proposal had been to keep `int` and `map[string]any`.
* **Unstable types:** keep the current names and mark every unstable type and method in its doc comment. The 46 unprefixed types are not renamed.

The phases, in order:

1. **A resolved model between `load` and `emit`.** One pass resolves `$ref` and `allOf`, rejects missing references, assigns every Go name once with
   collision and keyword checks, classifies each definition (object, union with declared or derived discriminator, enum, open enum, alias), and records
   per-property facts: required, nullable, default, integer format, bounds, typed `additionalProperties`, and the `x-deserialize-*` keywords. The
   emitters read only the model. The unstable merge reports which definitions are unstable from its own data instead of description text, and the
   generated doc comments mark them.
2. **Wire fixes on the model.** F1: a discriminator value that names no variant is never matched by key or by trial; it goes to `Unknown`, and a union
   with no discriminator returns an error naming the union. F2: `ContentBlock` keeps every key its variant struct encodes, and the shape table goes.
   F3: object variants are reshaped through `map[string]json.RawMessage`. F5: errors name the union and wrap the cause. (F1 refined, and F18 added,
   2026-10-10: see the phase 2 amendment below.)
3. **A protocol table.** The behaviour the schema does not express, prompt cancellation on `session/prompt`, `session/cancel`, the `session/load` binding,
   the hand-written helpers that replace generated ones, and the protocol methods, lives in one Go table in the generator, read by the model.
   (Refined 2026-10-10: see the phase 3 amendment below.)
4. **Schema fidelity.** Generated decoders honour `x-deserialize-default-on-error` and `x-deserialize-skip-invalid-items`. Integer fields take the Go type
   of their declared format (`int32`, `int64`, `uint32`, `uint64`), and typed `additionalProperties` become typed maps. `Validate` descends into unions,
   nested types, and slices, and checks the bounds the schema declares. (Refined 2026-10-10: the phase is decided by
   [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), which keeps the integer and map decisions above and adds
   the keyword registry, required keys on decode, raw JSON for untyped values, and a `Validate` limited to the schema's constraints.)
5. **Safe output.** All five files are rendered in memory, type-checked together with `go/types` against the hand-written package, and written only if
   every step passes, each through a temporary file and a rename. Panics become errors. (Refined 2026-10-10: see the phase 5 amendment below.)
6. **Tests of the engine.** Table tests run the generator on synthetic schemas for each construct and each refusal; a generated test round-trips every
   union variant; tests cover unknown variants, the deserialization keywords, and precision. (Refined 2026-10-11: see the phase 6 amendment
   below.)

Each phase has its own PLAN, written and approved before its work starts. Phases 2 and 4 change the exported API: phase 2 adds the `Unknown` fields, and
phase 4 changes the Go type of about 28 fields. The module is at `v0`, so these ship in a new minor version whose release notes list every changed
field.

### Consequences

* Good, because the SDK stops misreading newer peers, stops dropping `_meta`, `annotations`, and integer precision, and follows the schema's
  deserialization rules.
* Good, because a schema the generator cannot represent fails with a named error before anything is written.
* Good, because naming, typing, and union classification have one source, so an emitter cannot disagree with another.
* Good, because protocol knowledge outside the schema is listed in one place a reviewer can read.
* Neutral, because populated messages that use only known variants and carry no `_meta` or `annotations` on content blocks encode as today.
* Bad, because the restructure is large, touches every emitter, and changes much of `types_gen.go`; each phase needs its own diff review.
* Bad, because the unknown-variant field is a new exported field on each discriminated union, and honouring `x-deserialize-default-on-error` makes the
  SDK accept messages it rejects today. (Refined 2026-10-10: `Unknown` on eleven unions and `Extra` on six catch-all variant types; see the phase 2
  amendment below.)
* Bad, because a full type-check of the output adds to the generator's run time.
* Bad, because F1 to F3 stay in the SDK until phase 2 lands, after the model; until then a newer peer's unknown variants are misread, and
  `ContentBlock` drops `_meta` and `annotations`.
* Bad, because changing the integer and map types breaks callers that use those fields, about 28 of them; on `v0` that needs a new minor version and
  release notes, not a new module path.
* Good, because integer fields then carry the range their format declares, so a `uint64` size cannot go negative or overflow a 32-bit `int`.

### Confirmation

* Each probe in this record becomes a test that fails on `ccc6e1c` and passes after its phase: unknown variants (F1), `_meta` and `annotations` (F2),
  precision (F3), `x-deserialize-*` (F4), error text (F5), validation depth (F7).
* Each hostile schema in F8 becomes a test that expects a named error and an untouched output directory.
* `TestGeneratedFilesUpToDate` passes after every phase, and each phase's diff of the generated files is classified by its PLAN.
* `go test ./...` in `cmd/generate` covers the engine without the full-schema test, at a level each PLAN sets.

## Pros and Cons of the Options

### Fix the findings in place, one at a time, keeping the engine's structure

* Good, because each fix is small and reviewable.
* Bad, because names, types, and union shapes would still be decided in several places, so F8 to F12 would recur with the next schema.

### Restructure around a resolved, validated schema model, in phases

* Good, because each wire fix is written once, on the model, and the rest follows behind tests.
* Good, because it removes the cause shared by most findings.
* Bad, because, model first, the wire fixes wait for the first phase to land.
* Bad, because it is the most work, and the output changes in several phases.

### Replace the generator with a third-party JSON Schema code generator

* Good, because someone else maintains the schema handling.
* Bad, because it adds a dependency, which [0007-MADR-self-contained-acp-sdk.md](0007-MADR-self-contained-acp-sdk.md) rules out, and none models
  ACP's unions, deserialization keywords, dispatch, or unstable merge.

### Leave the engine as it is

* Good, because it costs nothing now.
* Bad, because F1 to F3 corrupt or drop data in normal use, and F8 lets a schema update produce a broken tree.

## Amendment 2026-10-10, phase 2 facts and decisions

Before planning phase 2, the wire fixes, the analysis was repeated on `e420cb6`, after phase 1. Measurements came from the resolved model, the schema
files, and probes of the generated SDK in scratch copies; nothing in the tree changed.

### Facts

* **Three kinds of discriminated union.** 17 of the 28 unions have a discriminator, 7 declared by the schema and 10 derived. They differ in what the
  schema says about an unrecognized value:
  * **Closed (7):** every variant has a constant discriminator value and requires the key. `ContentBlock`, `PlanUpdateContent`,
    `RequestPermissionOutcome`, `SessionConfigOption`, `SessionUpdate`, `ToolCallContent`, `UnstableNesSuggestion`.
  * **With a catch-all (6):** the schema defines an `other` variant whose discriminator is a plain string, with a `not` that excludes every known value,
    and, on five of them, `additionalProperties: true`. `CreateElicitationResponse`'s says agents "should preserve the raw payload when storing,
    replaying, proxying, or forwarding" and "MUST NOT treat it as a known elicitation action". `CreateElicitationRequest`,
    `CreateElicitationResponse`, `ElicitationPropertySchema`, `IdleStateUpdate`, `MultiSelectItems`, `StateUpdate`. These are the 6 uses of `not`
    in the schema.
  * **Defaulted (4):** one variant has no discriminator and is the one meant when the key is absent, and there is no catch-all. `AuthMethod`
    (`agent`), `McpServer` and `UnstableMcpServer` (`stdio`), `SetSessionConfigOptionRequest` (`value_id`).
  Two catch-all unions also have a variant for an absent key: `IdleStateUpdate` (`none`, `stopReason` omitted or null) and `MultiSelectItems`
  (`titled`).
* **What an unknown value does today.** Each union was given `{"<key>":"zz_future","extra":1}`. All 17 decoded without error: 15 into a known variant
  (for example `SessionUpdate` into `SessionInfoUpdate`, `IdleStateUpdate` into `EndTurn`, `AuthMethod` into `Terminal`, `McpServer` into `Http`),
  and two of those then failed to encode; `CreateElicitationRequest` and `MultiSelectItems` reached their `other` variant, which dropped `extra`.
* **What an absent key does today.** `AuthMethod`, `McpServer`, and `SetSessionConfigOptionRequest` pick their key-less variant, as the schema means.
  `IdleStateUpdate` given `{}` decodes as `EndTurn`, not `none`; `ContentBlock` `{"text":"hi"}` decodes as `Text` though every `ContentBlock` variant
  requires `type`. The other guesses are wrong: `RequestPermissionOutcome` `{"optionId":"allow-once"}` decodes as `Cancelled`,
  `CreateElicitationResponse` `{}` as `Accept`, `StateUpdate` `{}` as `Running`, a `SessionUpdate` agent chunk or tool-call update without
  `sessionUpdate` as `UserMessageChunk`, and an audio `ContentBlock` without `type` as an empty `Text`.
* **Without the key, few variants can be told apart.** Counting, per union, the variants whose required keys other than the discriminator include no
  other variant's: `ContentBlock` 3 of 5 (`image` and `audio` both need `data` and `mimeType`), `SessionUpdate` 1 of 19, `RequestPermissionOutcome` 1
  of 2, `CreateElicitationResponse`, `ElicitationPropertySchema`, and `StateUpdate` none.
* **The name `Unknown` is taken.** `StateUpdate` has a variant whose value is `unknown`, so its wrapper already has the field `Unknown
  *StateUpdateUnknown`. A union field named `Unknown` there would collide.
* **The catch-all structs keep only declared keys.** `IdleStateUpdateOther` has `_meta`, `stopReason`, `usage`; `StateUpdateOther`,
  `ElicitationPropertySchemaOther`, and `MultiSelectItemsOther` have only the discriminator. Any other key is lost.
* **F2.** The `ContentBlock` shape table drops exactly `_meta` and `annotations` from each of the five variants; every other field of the variant
  structs is in it.
* **F2 and F3 together.** In a scratch copy, encoding object variants through `map[string]json.RawMessage` (constants injected as raw strings) and
  removing the shape table: the root suite, with every golden and the HTML-escaping and sorted-key pins, passed. A `resource_link` `size` of
  9007199254740993 kept every digit, and a text block kept `_meta` and `annotations`, where `e420cb6` gave 9007199254740992 and dropped both.
* **F18, new: decoding into a reused union keeps the old variant.** A `ContentBlock` decoded from a text block and then from an image block has both
  `Text` and `Image` set: `Validate` fails, and encoding sends the old text. No union's `UnmarshalJSON` clears the value first.
* **F19, new: JSON `null` decodes as a variant.** `SessionNotification` with `"update": null` decodes with `Update.UserMessageChunk` set, and `null` into a
  `ContentBlock` sets `Text`; only `RequestId`, which has a null variant, is right. `encoding/json` calls `UnmarshalJSON` "including when the input is
  a JSON null", and states that null "has no effect on the value and produces no error" for types other than interfaces, maps, pointers, and slices.

### Decisions

Asked on 2026-10-10, the owner decided:

* **Unrecognized values: the schema's catch-all, else `Unknown`.** In the six unions with a catch-all, a value that names no known variant decodes
  into the `other` variant, which gains `Extra map[string]json.RawMessage` holding the keys its struct does not declare, so it encodes back unchanged.
  The eleven unions without one, closed and defaulted, gain `Unknown json.RawMessage`; such a value decodes into it and encodes back unchanged. No
  collision arises, since `StateUpdate` has a catch-all. This refines the earlier decision that every discriminated union gets `Unknown`.
* **Absent or null discriminator.** In a union with key-less variants (the four defaulted unions' default, `IdleStateUpdate`'s `none`,
  `MultiSelectItems`' `titled`), only those are candidates. In the other eleven, the payload decodes as the one variant, the catch-all aside, whose
  required keys other than the discriminator are all present; when none or several fit, a closed union keeps the payload in `Unknown`, and the four
  catch-all unions without a key-less variant (`CreateElicitationRequest`, `CreateElicitationResponse`, `ElicitationPropertySchema`, `StateUpdate`)
  return an error naming the union and the key. The owner first chose an error in all eleven, and changed to this rule after the measurements above
  showed what each would reject.

Phase 2 also takes F18 and F19: every union's `UnmarshalJSON` starts from the zero value, except that JSON `null` leaves a union without a null
variant unchanged and without error, as `encoding/json` does for other types.

### Consequences of these decisions

* Good, because a newer peer's value lands where the schema says, `other` or `Unknown`, and is sent on unchanged.
* Good, because `IdleStateUpdate` `{}` decodes as `none`, and a payload without its discriminator is never read as a variant two variants could be.
* Good, because the one unambiguous case seen in use, `ContentBlock` `{"text":"hi"}`, still decodes as `Text`.
* Bad, because the four catch-all unions without a key-less variant reject such a payload when it fits no single variant, `CreateElicitationResponse`
  `{}` among them, which decoded as `Accept` before.
* Bad, because there are two mechanisms, `other` with `Extra` and `Unknown`, and the exported API gains `Unknown` on eleven unions and `Extra` on six
  variant types.

## Amendment 2026-10-10, phase 3 facts and decisions

Before planning phase 3, the protocol table, the generator was read again on `128a920`, after phase 2. The facts come from the source, from a probe of
the resolved model in a scratch copy, from scripts over the schema files, and from the ACP specification; nothing in the tree changed.

### Facts

* **What phases 1 and 2 already removed.** The `ContentBlock` shape table is gone (phase 2). Response types come from `x-method` and `x-side`, a meta
  method without linked types is an error, and `terminal_wait_for_exit` and the description test for unstable methods are gone (phase 1).
* **What is left, by file.** In `internal/model`: `session/load` binds to `AgentLoader` (`methods.go`); `session/update` is renamed `SessionUpdate`,
  and `session_cancel` is renamed `Cancel`, which the default rule gives anyway (`notificationMethodName`); the helper skip list (`ContentBlock`,
  `ToolCallContent`, `SessionUpdate`); `ToolCallUpdate`'s `Validate`; `AgentLoader` declared as a fixed name. In `internal/emit`: `session/prompt`
  twice and `session/cancel` once in `dispatch.go`, which select the prompt-turn context, the cancel that cancels it, and the outbound
  `session/cancel` sent when a prompt's context ends; the literals `Cancel`, `CancelNotification`, and `SessionId` there; `AgentLoader`,
  `ClientTerminal`, and their doc comments in `types.go` and `dispatch_helpers.go`; the unused constant `nameSessionUpdate`.
* **Dead branches.** `BindClientTerminal` is never assigned; the `ClientTerminal` branches in `emit/types.go` and `emit/dispatch_helpers.go` never
  run. `isExtensionVariant` (a title-only variant whose description contains "extension" or whose title contains "Ext" becomes `json.RawMessage`)
  matches no variant at `1.25.0`.
* **The protocol method.** `meta.json` lists `$/cancel_request` under `protocolMethods`; the model resolves it with `CancelRequestNotification`, and
  nothing emits it. `connection.go` spells `"$/cancel_request"` 7 times and decodes it into its own `cancelRequestParams`, whose `json.RawMessage`
  keeps the id's exact JSON for `canonicalJSONRPCIDKey`; the generated `RequestId` union would not.
* **Capabilities gate methods the bindings do not show.** The schema describes capabilities that name methods: `AgentCapabilities.loadSession`
  (`session/load`), `sessionCapabilities.list`, `.delete`, `.resume`, `.close` (and, unstable, `.fork`), `auth.logout` ("the logout method");
  `ClientCapabilities.fs.readTextFile`, `fs.writeTextFile`, and `terminal` ("all `terminal/*` methods"). The specification's session-setup page says
  Clients "MUST NOT attempt to call" `session/load`, `session/resume`, or `session/close` unless the capability is advertised. Only `session/load` is on
  an optional interface; the other gated agent methods are on the required `Agent`.
* **What the specification says the hard-coded behaviour serves.** Prompt turn: "When the Agent receives this notification, it SHOULD stop all
  language model requests and all tool call invocations", and must answer the `session/prompt` with the cancelled stop reason. `$/cancel_request`:
  the receiver "MAY cancel the corresponding request activity" and "MUST send one of these responses", a result or `-32800`. The generated
  `AgentSideConnection` runs `Prompt` under a per-session context that an inbound `session/cancel` cancels; `ClientSideConnection.Prompt` sends
  `session/cancel` when its context ends with an error.

### Decisions

Asked on 2026-10-10, the owner decided:

* **The protocol method becomes an exported constant.** `constants_gen.go` gains `ProtocolMethodCancelRequest = "$/cancel_request"`, beside the agent
  and client method constants, and `connection.go` uses it in place of its literals. `connection.go` keeps `cancelRequestParams`. The exported API
  gains one constant.
* **`isExtensionVariant` is deleted.** A title-only variant takes the type its schema declares, like every other; the output at `1.25.0` does not change.
* **Capability gates are recorded, not acted on.** The table records the capability that gates each method whose capability names it; the model checks
  that the capability exists. No binding changes, and moving the gated agent methods off `Agent` waits for its own decision.

### Consequences of these decisions

* Good, because every piece of protocol behaviour the generator adds is listed in one table a reviewer can read, and a schema whose shape contradicts
  it is refused with an error naming the entry.
* Good, because `connection.go` and the generated constants share one spelling of `$/cancel_request`.
* Neutral, because apart from the new constant the generated files do not change.
* Bad, because five gated agent methods stay on the required `Agent` interface, so an agent must implement them to compile even when it does not
  advertise them.

## Amendment 2026-10-10, phase 5 facts and decisions

Before planning phase 5, safe output, the generator was read again on `d534f27`, after phase 4. The facts come from the source, from probes in scratch
copies and scratch modules, from the Go documentation and the Go source at `go1.27.2`, and from the POSIX, Linux, and Windows documentation; nothing in
the tree changed.

### Facts

* **How the generator writes today.** `run` in `cmd/generate/main.go` builds the model, creates the output directory, and calls `WriteConstants`,
  `WriteTypes`, `WriteDispatch` (`agent_gen.go`, then `client_gen.go`), and `WriteHelpers` in turn. Each renders one file (`format.Node`, then
  `format.Source`) and writes it with `os.WriteFile` at `generatedFileMode`, `0o600`. Nothing checks the output before the first write.
* **What that does with a defect in the engine.** In scratch copies: a declaration `go/format` cannot print, planted in the types emitter, stops the run
  with exit 1 and `types_gen.go: print: format.Node internal error (401:6: expected 'IDENT', found 1 (and 10 more errors))` after `constants_gen.go`
  was rewritten, so the tree is half-written; dispatch that calls a function no file declares (`toReqErrMissing`) exits 0 with all five files
  rewritten, and `go build` then fails with `./agent_gen.go:174:16: too many errors`. Since phase 1 the model refuses the hostile schemas of F8 before
  anything is written (`TestRunRefusesBeforeWriting`); these two are engine defects, which only a check of the output catches.
* **Three panics remain**, in `internal/emit`, the only ones in `cmd/generate`: `fieldSet.fieldList` (a doc comment with no field after it),
  `printedSize` (a function body `go/format` cannot print), and `layout.apply` (a line table `token.File.SetLines` rejects). Only an engine defect
  reaches them.
* **`os.WriteFile` is not atomic.** Its documentation: "Since WriteFile requires multiple system calls to complete, a failure mid-operation can leave
  the file in a partially written state." It keeps an existing file's mode ("otherwise WriteFile truncates it before writing, without changing
  permissions"), so the committed `*_gen.go` are `0644` on disk, as git records them (`100644`), and a fresh output directory gets `0600` files.
* **Rename is the atomic replacement on POSIX, and only there.** POSIX.1-2024 `rename`: a directory entry named new "shall remain visible to other
  threads throughout the renaming operation and refer either to the file referred to by new or old before the operation began", and its rationale
  "requires that the action of the function be atomic". `os.Rename`: "Even within the same directory, on non-Unix platforms Rename is not an atomic
  operation." On Windows Go calls `MoveFileEx` with `MOVEFILE_REPLACE_EXISTING` alone, which replaces an existing file but cannot move across
  volumes, so a temporary file belongs in the destination directory.
* **Durability needs a sync.** Linux `fsync(2)`: "Calling fsync() does not necessarily ensure that the entry in the directory containing the file has
  also reached disk. For that an explicit fsync() on a file descriptor for the directory is also needed." The go command's `renameio` (removed after Go
  1.16) synced the file before renaming it, "otherwise, after a crash the reader may observe a 0-length file instead of the actual contents", and did not
  sync the directory; neither does `github.com/google/renameio`, whose README adds that a temporary file must not be removed once its rename
  succeeded, and which exports nothing on Windows: "It is not possible to reliably write files atomically on Windows". `x/tools` writes source with
  plain `os.WriteFile(..., 0o644)` (`stringer`, the analysis checker's `-fix`). Go issue 22397, a standard package for atomic writes, is open.
* **Temporary files and their mode.** `os.CreateTemp` creates the file "with mode 0o600 (before umask)", and "Multiple programs or goroutines calling
  CreateTemp simultaneously will not choose the same file". A file renamed into place keeps that mode unless it is changed. In a scratch directory,
  staging the five committed files with `CreateTemp`, `Write`, `Sync`, `Chmod(0o644)`, and `Close`, then renaming each, took 24 to 34 ms on this macOS
  host and left five byte-identical `-rw-r--r--` files.
* **gosec and the mode.** gosec arrived in `9fb9ae5`, the commit that changed the generator's `0o644` to `generatedFileMode = 0o600`. With the
  repository's `.golangci.yml` and golangci-lint 2.14.0, on a scratch module: `os.WriteFile` with the literal `0o644` is reported as G306, `os.Chmod`
  with `0o777` as G302, and `os.ReadFile` of a joined path as G304; `(*os.File).Chmod(0o644)`, `os.CreateTemp`, and `os.Rename` are not reported.
* **Type-checking the output.** The root package imports only the standard library. `go/types` over the five generated files and the seven
  hand-written ones (`agent.go`, `client.go`, `connection.go`, `doc.go`, `errors.go`, `extensions.go`, `helpers.go`), with
  `importer.ForCompiler(fset, "source", nil)`, reported 0 errors in 0.47 to 0.85 s across runs; the generator itself runs in about 0.12 s. The `gc`
  importer took 0.91 s cold and 0.03 s warm, and finds the standard library's export data by running `go list -export` (`internal/exportdata`), so it
  needs the go command and writes the build cache; the source importer reads `GOROOT`. The `go/importer` documentation calls its functions "mostly
  deprecated" and recommends `golang.org/x/tools/go/packages`, a dependency [0007-MADR-self-contained-acp-sdk.md](0007-MADR-self-contained-acp-sdk.md)
  rules out. With a nil lookup, the source importer resolves an import outside the standard library through `go/build`, which runs `go list` for it.
* **Selecting the hand-written files.** `build.Default.ImportDir(dir, 0)` applies the go command's rules (build constraints, `GOOS` and `GOARCH` file
  suffixes, names starting with `_` or `.` ignored, test files listed apart) and, for a local directory, does not run the go command. In a scratch copy
  it selected the seven files and skipped a `//go:build ignore` file, a `_test.go` file, a `.types_gen.go.123.tmp` file, and a `_plan9.go` file, each
  using an undefined name. It returned `*build.MultiplePackageError` for a stray `package other` file, and an error for a missing directory. Checked
  alone, as in an empty directory, the generated files give 212 errors, the first `undefined: AgentSideConnection`.
* **The language version.** `types.Config.GoVersion` "describes the accepted Go language version"; empty disables the check. Checked as `go1.21`, the
  package gives 4 errors, the first `helpers.go:95:9: new(v) requires go1.26 or later`; as `go1.27.2`, the `go` line of `go.mod`, none.
* **Leaving an unchanged file alone.** No Go convention found requires it. `gofmt -w` writes only when the bytes differ and keeps the file's mode;
  `stringer` always writes. The go command's build cache keys on content, so rewriting the same bytes rebuilds nothing, but it moves the file's
  modification time, which editors and file watchers see.

### Decisions

Asked on 2026-10-10, the owner chose the recommended option of each question: the check always runs, with no flag to turn it off; unchanged files
are left alone; files are written `0644`; each file is synced before its rename, with no directory sync, no retry on Windows, and no rollback. The
other options asked were a `-check=false` flag; rewriting every file; each existing file's mode and `0600` for a new one; syncing the directory; and
restoring backups when a rename fails partway. The decisions, each with the alternative it was weighed against:

* **Render everything first.** `emit.Render` returns the five files in memory, or an error naming the file; the four `Write*` functions go. The three
  panics become errors that name the file.
* **Type-check before writing, always.** The generated files are checked in memory with `go/types`, together with the hand-written files of the output
  directory: those `build.Default.ImportDir` selects, less the five generated names. The importer is `importer.ForCompiler(fset, "source", nil)`;
  `GoVersion` is the `go` line of the output directory's `go.mod` when there is one. Every error is collected; the first 10 in position order are
  reported, with a count of the rest. Output that does not type-check is refused and nothing is written, and so is a directory without the hand-written
  package. No flag turns the check off. The alternative, a `-check=false` flag for a schema change that breaks hand-written code, was not chosen: the
  hand-written code can be changed first and the generator then checks the result, and unchecked output is what F8 forbids.
* **Stage every file, then replace.** A file whose bytes equal the existing file's is left alone, mode and modification time included. Every other file
  is staged in the output directory as `.<name>.*.tmp`: written, synced, set to mode `0644`, and closed. If any staging step fails, every temporary file
  is removed and no file is replaced. Then each is renamed into place in output order; a temporary file is removed only when its rename failed. If a
  rename fails, the remaining temporary files are removed, and the error names the file, the files already replaced, and those left as they were.
  Nothing is rolled back: version control holds the previous files. The directory is not synced, and nothing retries on Windows. The alternative,
  writing unchanged files as today, keeps the generator simpler and moves every file's modification time on every run.
* **Mode `0644`.** Every file the generator writes is `0644`, as git records it and as `stringer` writes it, set with `(*os.File).Chmod`, so the umask
  does not narrow it. This settles the mode item of F14. The alternative, each existing file's mode and `0600` for a new one, is what the generator does
  today.

### Consequences of these decisions

* Good, because an engine defect, or a schema change that breaks hand-written code, is refused with the type checker's errors before any file
  changes, and a render failure no longer leaves a half-written tree.
* Good, because on POSIX a reader of the output directory sees each file whole, old or new, and a crash after a rename leaves a complete file.
* Good, because a run that changes nothing writes nothing.
* Neutral, because the generated files and the exported API do not change.
* Bad, because a run takes about 0.5 to 0.9 s longer.
* Bad, because the five files are replaced one rename at a time: a failure midway, or two runs at once, can leave a mix of old and new files, each
  whole; and on Windows a replacement is not atomic.
* Bad, because the source importer is deprecated for module-aware loading. It suffices while the root imports only the standard library; a module
  dependency of the root would make it run `go list`.
* Bad, because the check covers the host's `GOOS` and `GOARCH` only. The root has no platform-specific file today, and the precheck lints `GOOS=windows`.
* Bad, because generating into an empty directory, to look at the output, needs the hand-written files copied there first.
* Bad, because an interrupted run can leave `.<name>.*.tmp` files behind. The go command and gofmt ignore names that start with `.`, and `git status`
  shows them.

Carried out on 2026-10-10 by [0011-PLAN-safe-output.md](0011-PLAN-safe-output.md), commits `838a4c4` to `9efc331`, as decided: `emit.Render`,
`output.Check`, and `output.Write`; no panic is left in `cmd/generate`. The generated files and the exported API did not change. Measured warm, the
check made a run 0.48 s longer (0.12 to 0.60 s), just under the 0.5 to 0.9 s estimated above. The plan's deviations concern how two behaviours are
tested; none changes a decision here.

## Amendment 2026-10-11, phase 6 facts and decisions

Before planning phase 6, tests of the engine, the tests of the generator and of the generated SDK were measured on `f2a6a3a`, after phase 5. The facts
come from coverage profiles, from mutation and harness probes in scratch copies and scratch modules, from the Go documentation and source at
`go1.27.2`, from the CI logs, and from the documentation of other projects, linked where cited; nothing in the tree changed.

### Facts

* **What runs today.** `cmd/generate` has 67 test functions, one of them a fuzz target (`FuzzExportedName`); the root has 91, and 33 JSON fixtures in
  `testdata/json_golden`. Each package's own tests cover `internal/emit` 84.0%, `internal/load` 86.4%, `internal/model` 93.9%, `internal/output`
  92.3%, and `cmd/generate` 45.9%; `internal/util` has no test file. Counted across packages (`go test -coverpkg=./... ./...` in `cmd/generate`), 92.3%
  of the generator's statements run.
* **What the committed schema carries.** Nine tests build or read the schema in `schema/`: `TestGeneratedFilesUpToDate`, `TestCheckAcceptsRepository`,
  `TestUnionClassesOfCommittedSchema`, `TestFitKeysOfCommittedSchema`, `TestDiscriminatorExemptOnDecode`, `TestNoOmitemptyOnStructValues`,
  `TestProtocolTableAppliesToCommittedSchema`, `TestCommittedSchemaKeywordsRegistered`, and `TestMergeProvenanceMatchesInputs`. Skipping them leaves
  88.0%. Below 70% then: `clientFile` 31.6%, `emitObjectSchema` 50.0%, `emitSchemaDef` 65.0%, `unionUnmarshalBody` and `catchAllOf` 66.7%,
  `ReadMetaUnstable` and `ReadSchemaUnstable` 44.4%, `defaultKindOf` 60.0%, `sideOf` 50.0%, `firstPos` 25.0% (F15), and `clientAssert`,
  `singleMethodAssert`, `enumConsts`, `missingDisc`, and `keyLessBody` 0%.
* **Refusals.** 93 lines of non-test code in `cmd/generate` create an error (`fmt.Errorf`, `errors.New`, or a recorder such as `goFile.fail`). 38
  never run under any test, with or without the nine: all 12 in `internal/load/merge.go`, 8 of 9 in `load.go` (reading and parsing each of the four
  files), 5 of 23 in `model/protocol.go`, 3 of 4 in `model/validate.go`, 3 of 12 in `model/model.go`, 3 of 6 wrapping lines in `emit/file.go`, 1 each in
  `model/names.go` and `model/methods.go`, and both in `main.go`.
* **How the emitters are tested.** The `emit` tests build small schemas and assert on the generated text with `strings.Contains` (8 places) or
  type-check it (10 places). No test runs code generated from a synthetic schema. Behaviour is tested only in the root, on the committed schema.
* **What the root tests run of the generated code.** 2,079 of 7,187 statements of `types_gen.go` (28.9%), 19.3% of `agent_gen.go`, 23.0% of
  `client_gen.go`, and 18.2% of `helpers_gen.go`. Of the generated methods, 224 of 317 `UnmarshalJSON`, 34 of 76 `MarshalJSON`, and 117 of 145
  `Validate` never run. `types_gen.go` has 27 unions with 110 variants; the root tests decode 39 of the variants, and 6 unions not at all:
  `AgentResponse`, `ClientResponse`, `ElicitationContentValue`, `ElicitationFormMode`, `ElicitationUrlMode`, and `UnstableMessageMcpResponse`.
* **Mutation probe, generated code.** One operator flipped per mutant (`==`/`!=`, a bound, `&&`/`||`, `true`/`false`, `return nil` to an error,
  `continue`/`break`), in a scratch copy of the root at `f2a6a3a`, then `go test -count=1 -failfast .`. 60 mutants sampled (seed 7) from the 2,463 sites
  in the bodies of `UnmarshalJSON`, `MarshalJSON`, and `Validate` in `types_gen.go`: all built, 24 failed a test, 36 survived. The survivors invert an
  error check (`if err != nil` to `==`), return nil where an error belongs, or flip the `match` flag of the key-less trial decode that
  [0012-PLAN-schema-fidelity-framework.md](0012-PLAN-schema-fidelity-framework.md) records as dead in five unions. A mutant took 3.2 s (median).
* **Mutation probe, the engine.** The same operators on a scratch copy of `cmd/generate`, then `go test -count=1 -json ./...`. 80 mutants sampled from
  the 828 sites in `internal/model`, `internal/emit`, and `internal/load`: 6 did not build; of 74, 67 failed a test and 7 survived (whether any is
  equivalent was not analysed). 12 of the 67 failed only tests of the committed schema, 9 of them only `TestGeneratedFilesUpToDate`, which compares
  bytes. In `internal/emit`, 9 of 18 failed a test that does not read the committed schema. A mutant took 3.7 s (median).
* **What a synthetic schema needs to compile and run.** The generated files refer to seven hand-written names: `NewInvalidParams`, `toReqErr`,
  `SendRequest`, `NewMethodNotFound`, `RequestError`, `AgentSideConnection`, and `ClientSideConnection`. Of the seven hand-written non-test files, six
  (`agent.go`, `client.go`, `connection.go`, `doc.go`, `errors.go`, `extensions.go`) refer to generated code only through `Agent`, `Client`, and
  `ProtocolMethodCancelRequest`; `helpers.go` refers to 27 ACP types (`ContentBlock`, `SessionUpdate`, `ToolCallContent`, and others). The model
  refuses a `protocolMethods` entry without a definition for it: `protocol method "$/cancel_request": no request or notification definition is linked
  to it by x-method and x-side`.
* **Harness probe.** A synthetic schema of five definitions (a request and response bound to an agent method `echo/run`, a discriminated union, and
  `CancelRequestNotification` with `RequestId` for the protocol method) ran through the generator binary into a scratch module holding the six files
  above. It generated and type-checked in 1.0 to 1.2 s. A behaviour test file in that module passed `go test` in 0.49 s warm and 0.91 s first: a
  union round-trip of each variant and of an unknown one, an `int64` beyond 2^53 and a `uint64` at its maximum, `x-deserialize-default-on-error` and
  `x-deserialize-skip-invalid-items`, a missing required key refused, and a request answered by an `AgentSideConnection` over `io.Pipe`.
* **Fuzzing.** `go test -fuzz` runs one target of one package; arguments are `[]byte`, `string`, the integer and float types, and `bool`; the seed
  corpus is `f.Add` plus `testdata/fuzz/<Name>`, and a plain `go test` runs only the seeds ([go.dev/doc/security/fuzz](https://go.dev/doc/security/fuzz/)).
  The standard library tests its own codec this way: `FuzzUnmarshalJSON` in `encoding/json` (`fuzz_test.go` and, for the v2-backed build,
  `v2_fuzz_test.go`) unmarshals into each type of a `[]func() any`, and requires a value that decoded to marshal and to unmarshal again.
* **`go test -overlay`.** `go help build`: the overlay makes "a build ... run as if the disk file path exists with the contents given by the backing
  file paths", and "overlays will not appear when binaries and tests are run through go run and go test". A mutant can be compiled without writing the
  tree; a test that reads a file at run time reads the disk.
* **Go 1.26 and 1.27 test features.** `(*testing.T).ArtifactDir` "returns a directory in which the test should store output files", kept under
  `-outputdir` when `go test -artifacts` is given and removed otherwise (`go doc testing.T.ArtifactDir` at `go1.27.2`).
* **`encoding/json` at `go1.27.2`.** The `jsonv2` experiment is on by default (`internal/buildcfg/exp.go`, `JSONv2: true`), and `go list -f
  '{{.GoFiles}}' encoding/json` lists only `v2_*.go` files: the v1 API is served by the v2 implementation. The root tests pass both by default and with
  `GOEXPERIMENT=nojsonv2`; CI runs only the default.
* **CI.** `.github/workflows/ci.yml` runs `go test -shuffle=on ./...` in both modules on `ubuntu-24.04`, `macos-15`, and `windows-2025`, the race
  detector and the linters on Linux only, and is also scheduled weekly (`17 6 * * 1`). At `f2a6a3a` the Windows job passed every package, `internal/output`
  included. [0011-PLAN-safe-output.md](0011-PLAN-safe-output.md) stated in its step 7 record that nothing runs on Windows; that is wrong, and the plan
  carries a dated correction.
* **Harness formats.** The standard library's txtar reader, `internal/txtar`, cannot be imported, and `golang.org/x/tools/txtar` is a module dependency
  ([pkg.go.dev](https://pkg.go.dev/golang.org/x/tools/txtar)); the format is a line `-- name --` before each file's text. `testing/quick` "is frozen and
  is not accepting new features" ([pkg.go.dev](https://pkg.go.dev/testing/quick)); `pgregory.net/rapid` and `github.com/rogpeppe/go-internal/testscript`
  are module dependencies.
* **Mutation tools.** Per their documentation: `go-gremlins/gremlins` v0.6.0 (2025-12-06, Apache-2.0) is an external command that mutates covered
  code only and can fail on a threshold ([gremlins.dev](https://gremlins.dev/latest/usage/commands/unleash/)); `gtramontina/ooze` is a library imported
  by a test, so a module dependency ([github.com/gtramontina/ooze](https://github.com/gtramontina/ooze)); `avito-tech/go-mutesting` is an external
  command with several dependencies of its own ([github.com/avito-tech/go-mutesting](https://github.com/avito-tech/go-mutesting)). None was run here.
* **How other Go generators test themselves.** Per their repositories: `ogen` generates every specification in `_testdata/positive` and requires an
  error for each in `_testdata/negative`, and exercises 39 committed generated packages (`internal/integration/test_*`) end to end
  ([github.com/ogen-go/ogen](https://github.com/ogen-go/ogen)); `omissis/go-jsonschema` keeps a golden `.go` file per case and compiles it in a separate
  test module, `tests/` with its own `go.mod` ([github.com/omissis/go-jsonschema](https://github.com/omissis/go-jsonschema)); quicktype runs each
  generated program on sample JSON in a subprocess ([github.com/glideapps/quicktype](https://github.com/glideapps/quicktype)).
* **Fixtures elsewhere.** ACP's schema declares `https://json-schema.org/draft/2020-12/schema`. The JSON Schema Test Suite (MIT) has per-keyword
  files for that draft ([github.com/json-schema-org/JSON-Schema-Test-Suite](https://github.com/json-schema-org/JSON-Schema-Test-Suite)); its
  `minLength` counts code points, as the generated `Validate` does with `utf8.RuneCountInString`. ACP's Python SDK keeps 38 files of wire
  messages under `tests/golden/` (Apache-2.0) ([github.com/agentclientprotocol/python-sdk](https://github.com/agentclientprotocol/python-sdk)).

### Decisions

Asked on 2026-10-11, the owner chose the recommended option of each question, listed first in each item below: txtar cases (D1); generated code
compiled and run by one `go test` subprocess (D2); a generated registry and a schema-driven conformance test at the root (D3); fuzz targets run by
`make fuzz`, not in CI (D4); a standard-library mutation tool that reports and never gates (D5); levels the plan meets, with no standing threshold (D6);
and a CI step with `GOEXPERIMENT=nojsonv2` (D7). The alternatives named in each item were not chosen.

* **D1. Synthetic schemas as txtar cases, run in process.** Each case is a file under `cmd/generate/testdata/engine/`: the schema files of a schema
  directory, then what is expected, either `want.err` (substrings of the refusal) or `want.api` (the exported declarations of the generated package,
  listed from `go/types`). A test reads the cases with a small reader in the generator's test code, runs `run` into a temporary directory holding the six
  runtime files of the repository, and compares. A refusal also requires the directory to be as it was. Alternatives: Go literals in table tests, as
  today, which suit small schemas and hide large ones; `testscript`, a dependency.
* **D2. Generated code from synthetic schemas is compiled and run.** A case may add `behaviour_test.go`. One test collects those cases into one
  temporary module, one package per case with the six runtime files, and runs `go test -json ./...` once (`GOWORK=off`, `GOFLAGS=-mod=mod`,
  `GOTOOLCHAIN=local`, `GOPROXY=off`), keeping the module in `t.ArtifactDir()` on failure. `-short` skips it. Alternatives: committed generated packages
  in `cmd/generate`, as ogen and go-jsonschema do, with no subprocess but a second set of files to regenerate and the runtime files copied; or no
  execution, which leaves the engine's behaviour tested only through the committed schema.
* **D3. Every definition, variant, and method of the committed schema is exercised at the root, from a generated registry.** The generator writes a
  sixth file, `schema_gen_test.go`: data only, a constructor for every generated type keyed by its schema definition and file, every union's variants
  and discriminator values, every method's wire name, side, and parameter and result types, and a recording `Agent` and `Client`. A hand-written root
  test reads the schema files and builds instances from them independently of the generator: one per variant, with required properties only and with
  all of them, numbers at the limits of their format. It requires each to decode, validate, and encode to equal JSON; each required key, when removed,
  to be refused; each discriminated union to keep an unknown value; each lenient field and array to follow its keyword; each bound to fail `Validate`
  when broken; and each method to reach the recording peer over a connection. `output.Check` type-checks `schema_gen_test.go` with the package.
  Alternatives: a walk by reflection from `Agent` and `Client`, which misses unreachable types such as `AgentResponse`; fully generated test bodies,
  where the engine checks itself; or a fixture per variant, hand-written or taken from the Python SDK with its licence, kept complete by a guard.
* **D4. Fuzz targets.** At the root, `FuzzRoundTrip(which uint16, data []byte)` over the registry: no panic, and a value that decodes encodes,
  decodes again, and encodes to the same bytes; `Validate` does not panic. In `cmd/generate`, `FuzzBuild(schema []byte)`: no panic, and a schema the
  model accepts renders. Seeds are the D1 cases, the D3 instances, and `testdata/json_golden`. `make fuzz` runs each for `FUZZTIME`, 60 s by default.
  Alternatives: also a time-boxed fuzz step in the weekly CI run; no fuzzing.
* **D5. Mutation testing with a stdlib tool in the generator module.** `cmd/generate/internal/mutate`, run as `make mutation`: it lists the operator
  sites of the files given with `go/ast`, compiles each mutant through `go test -overlay` so the tree is never written, applies a per-mutant timeout, and
  writes a JSON report of killed, survived, and unbuildable mutants. It reports and never gates. Alternatives: gremlins v0.6.0 run with `go run` at a
  pinned version, as CI runs `govulncheck`, after a trial; no mutation testing, as today.
* **D6. Levels the plan meets, not standing gates.** Without the nine committed-schema tests, the engine's statements at 95% or more, every error site
  listed above run by a test or named in the plan with its reason, and no engine mutant killed only by `TestGeneratedFilesUpToDate`. At the root,
  `types_gen.go` at 85% or more and every union variant decoded. Every surviving mutant of the generated code fixed or recorded as equivalent.
  Alternative: a coverage threshold enforced by a check.
* **D7. CI runs the root tests with `GOEXPERIMENT=nojsonv2` too**, on Linux, so a difference between the v1 and v2-backed codecs fails a build.
  Alternative: no change.

### Consequences of these decisions

* Good, because each construct and each refusal of the engine is specified by a case a reader can open, and the emitters' behaviour is tested
  without the committed schema.
* Good, because every generated decoder, union variant, and method of the SDK runs in a test, with an oracle independent of the generator.
* Good, because mutation reports show which planted faults the tests miss, without a dependency.
* Neutral, because the five generated SDK files and the exported API do not change; `schema_gen_test.go` is a test file.
* Bad, because the generator gains a sixth file and `output.Check` a test file to check, which changes what phase 5 built.
* Bad, because the compile-and-run test needs the go command and adds about 1 to 3 s to `go test` in `cmd/generate`. (Corrected
  2026-10-11: it adds about 10 s; see "Test time, decided in step 5" below.)
* Bad, because the instance builder is a second reading of the schema to maintain when the schema uses a construct it does not know.
* Bad, because the six runtime files are listed in the generator's test code; when a hand-written file starts to use ACP types, a test fails and
  names it, and the list changes.
* Bad, because a mutation run takes about 3 to 4 s a mutant, so it is a report, not a gate.

### Found in step 3 and decided, 2026-10-11

The construct cases of step 3 of [0011-PLAN-engine-tests.md](0011-PLAN-engine-tests.md) showed two shapes the generator types with loss, with no
error. Neither is in schema `1.25.0`: by script over both schema files, every `allOf` there has one schema and none is on a definition, and every
inline object with properties is a direct `anyOf` or `oneOf` member of a definition, or inside a `not`
(`CreateElicitationRequest.anyOf[2].not.anyOf[0]`), whose subschemas only constrain a value and are never typed.

* A definition made of `allOf` becomes `any`: `classify` (`model.go:306`) sees no type. A property whose `allOf` has several schemas takes the first
  `$ref` (`typeRef`, `fields.go:207`–`219`), what remains of F6.
* An inline object with properties becomes `map[string]any`, its properties dropped, everywhere but a definition, a property of an object
  definition (a nested struct), and a direct union variant (`typeRef`, `fields.go:235`): a nested object two levels down, an array's items, a map's
  values, a property of an inline variant.

Asked with the alternatives of representing both (a merged struct or embedding for `allOf`, nested named types at any depth) and of recording them
for later, the owner chose to refuse both now. The model refuses an `allOf` with more than one schema anywhere, an `allOf` on a definition, and an
inline object with properties outside the three places above, each with an error naming its path, before anything is written; the
subschemas of `not` and `unevaluatedProperties` are not checked. Representing either
later adds API and breaks none; a representation chosen now would fix type names and merge rules with no schema to test them against. This puts
`internal/model` in phase 6's scope for these checks; the generated files do not change.

### Test time, decided in step 5, 2026-10-11

The estimate above counted one `go test` of a few packages. Measured with the cases of steps 2 to 5 (44 accepted, 30 with a behaviour file): `go test
./...` in `cmd/generate` took 34 to 43 s, 19 s with `-short`, and 92 s with `-race`, against 2.2 to 2.5 s on `f2a6a3a`. `TestEngineBehaviour`
alone took 12 to 13 s, of which the `go test` of the 30 packages was about 10 s. Most of the rest was `output.Check` type-checking the standard
library from source again for every generated case, in `TestEngineCases` and in the behaviour runner; the generator itself calls it once.

Asked with the alternatives of also skipping `TestEngineCases` under `-short` and of accepting the cost, the owner chose to have `output.Check` keep
one source importer for the process, behind a mutex, as the test helper `enginetest.LoadPackage` already did. The check, its importer, its version,
and its refusals are as phase 5 decided; only repeated calls in one process share what they imported. In a scratch copy this gave 15 s, 6 s with
`-short`, and 38 s with `-race`.

### Scope added 2026-10-11, a regression guard for raw values

The owner added to phase 6 a regression the type checks did not catch: since
[0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md) made raw values `json.RawMessage`, an empty raw tool input prints
as `null`, and a `null` one is sent on as `null`. The fix, decided in that record's amendment of 2026-10-11, keeps `json.RawMessage` and reads a
`null` raw value as an absent one, as ACP v1 defines them; its regression tests, the generator's change, and the docs are step 6a of the plan.

### Found in step 7 and decided, 2026-10-11

The conformance test of step 7 runs every generated type, union, and method against instances built from the schema files. On the committed
schema it found:

* **An object with properties and a union of references loses half of itself.** `ElicitationFormMode` has a required `requestedSchema` and an
  `anyOf` of two members, each an `allOf` of one `$ref` (`ElicitationSessionScope`, `ElicitationRequestScope`); `ElicitationUrlMode` has the
  required `elicitationId` and `url` and the same members. The model classifies both as unions: the members become the fields `Session` and
  `Request`, and the definition's own properties are dropped, since only inline variants carry a union's properties. `CreateElicitationRequest`'s
  `form` and `url` variants are an `allOf` of those definitions plus properties: their structs merge the definitions' properties and drop the
  union, so `sessionId`, `toolCallId`, and `requestId` are lost. A client decoding `elicitation/create` cannot tell which session or request it
  belongs to, and an agent cannot send it. The shape has been in the committed schema since `1.25.0` (`17adda9`); no other definition has it.
* **An empty optional map is not sent.** An optional map field arrives as `{}` and is sent as nothing, since `omitempty` drops an empty map:
  277 fields, 270 of them `_meta`, as the original SDK did.

Asked, the owner decided:

* **Such an object is a struct with its members flattened.** A definition with properties of its own and an `anyOf` or `oneOf` whose members are
  all references (a `$ref`, or an `allOf` of one `$ref` with no properties) is an object: its own fields, and one pointer field per member, named
  by the member's title and typed by the referenced definition, written into the same JSON object. A union variant that is an `allOf` of such a
  definition keeps its fields and gets the same member fields. Decoding takes the first member, in schema order, whose required keys the payload
  carries, and refuses a payload no member fits; encoding writes the struct's keys and the set member's; `Validate` requires exactly one member
  set. Fields are added; none is removed. Not chosen: one field naming a generated union for the members, a Go field the schema does not name;
  and recording the loss for a later decision.
* **An empty optional map is the same as an absent one.** Neither holds a key, and ACP leaves `_meta`'s keys to the peers. Nothing changes on the
  wire; the conformance test accepts it, and this record states it. Not chosen: `omitzero` on the 277 fields, which would send `{}` back.

A fourth finding came from extending the test to every union, discriminated or not: `ErrorCode`, an `anyOf` of eight integer `const` members
(`-32700` "Parse error", `-32601` "Method not found", and others) and an `Other` integer, decodes every value into its first variant: `-32601`,
`-32000`, and `7` all set `ParseError`. The trial decode takes the first `int32` variant that accepts the number and never compares the `const`.
The wire value survives, so a round trip passes, but Go code that checks a variant reads the wrong one. The type has been a union of nine pointer
fields to named `int32` types since schema `0.10.7` (`c225368`); it is the only integer union in either schema file.

Asked, the owner decided: **an `anyOf` or `oneOf` whose members are all integers, each a `const` with a title or a plain integer, is an open
integer enum**, generated as string const unions are: a named integer type of the members' format and a constant per `const`, named by its title
(`type ErrorCode int32`, `ErrorCodeMethodNotFound ErrorCode = -32601`). Any value of the type decodes; a known one equals its constant. The
exported API changes: `ErrorCode` stops being a struct and the nine `ErrorCode*` variant types go; the release notes list it. A `const` member
without a title, or members of different formats, is refused. Not chosen: decoding a variant only by its `const`, which keeps the nine-field
union; and recording the finding for later.

The plan for these decisions is [0011-PLAN-engine-tests.md](0011-PLAN-engine-tests.md).

### Found in step 8 and decided, 2026-10-11

`FuzzRoundTrip` failed within three seconds of `make fuzz` on `{"stopReAson":"\x820000"}` for `IdleStateUpdate`: decoded and encoded, it gives
bytes that decode into another variant and encode differently. Scratch probes, under the default build and with `GOEXPERIMENT=nojsonv2`, found
two behaviours behind it.

* **A key in another case is absent to the generated code and present to `encoding/json`.** The generated decoders look keys up in a
  `map[string]json.RawMessage` by their exact names: the discriminator of the 17 discriminated unions, the required keys (`objectKeys`), the keys
  that pick a variant or a member, defaults, and the keys a catch-all struct keeps. The structs themselves are decoded by `encoding/json`, which
  matches a name under Unicode case folding, the last matching key winning, as
  [0007-MADR-self-contained-acp-sdk.md](0007-MADR-self-contained-acp-sdk.md) records ("Object names | case-insensitive"). So
  `{"StopReason":"end_turn"}` decodes as `IdleStateUpdateNone`, no stop reason, whose raw `stopReason` field takes the value, and is sent on as
  `{"stopReason":"end_turn"}`, which every peer reads as `end_turn`. `{"ToolCallId":"1","title":"t"}` fails as missing `toolCallId`, while
  `{"toolCallId":"1","Title":"t"}` fails as missing `title`, and an optional key in another case is accepted.
* **A raw value keeps invalid UTF-8.** `encoding/json` replaces invalid UTF-8 in a string with U+FFFD, but a `json.RawMessage` is carried byte
  for byte, so the SDK sends on invalid UTF-8 a peer sent it. This is how Go's `json.RawMessage` behaves, and it made the first finding visible
  to a byte comparison.

Asked, the owner decided:

* **Generated code matches a key as `encoding/json` does.** After decoding an object into a map, the generated code folds its keys: each name it
  reads takes the value of the last key that matches it under Unicode case folding, and the other matching keys are removed. A wrong-case
  discriminator, required key, or member key is then read as `encoding/json` reads the field, and a catch-all keeps only the keys no field
  takes. The generator refuses a schema in which two names that one decoder reads differ only in case, since folding would merge them; the
  committed schema has none. Not chosen: exact names everywhere, which matches the schema and Rust but contradicts the v1 behaviour 0007
  records and costs a filtering pass in every decoder; and a change to the discriminator alone, which leaves exact required keys beside folded
  optional ones.
* **Invalid UTF-8 in a raw value is recorded, not changed.** It follows 0012's byte-for-byte rule for raw values
  ([0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), amendment "key names, and invalid UTF-8 in raw values").
  Not chosen: replacing it on decode.

The wire changes: a payload whose keys differ from the schema's only in case is decoded as its names say, where it was refused or decoded into
another variant. The exported API does not change. The release notes list it.

`FuzzBuild` then found that the integer enum decided in step 7 accepted a member typed `integer` whose `const` is not an integer of its
format: `{"type": "integer", "format": "int32", "const": ""}` rendered a constant with no value, which failed to format. The loader reads a
`const` as `float64`, and the model wrote it with `fmt.Sprint`, so `1.5`, `3000000000` for `int32`, and an integer beyond 2^53 also passed the
model; `-2147483648` was written `-2.147483648e+09`. Asked, the owner decided: **an integer enum's `const` must be an integer, below 2^53 in
magnitude, within its format's Go type; any other is refused**, naming the type, the variant, and the value. A loaded `float64` of 2^53 or more
may have been rounded from another integer, so it is refused rather than read. Not chosen: classifying such a definition as something other
than an integer enum. The committed schema's `ErrorCode` values are small `int32` constants; the generated SDK does not change.

## More Information

Implementation follows in one PLAN per phase, each written and approved before its work starts. The first is the resolved model.

Evidence was gathered on `ccc6e1c` without changing the tree:

* the behaviour probes ran as a scratch test file in a copy of the repository;
* the keyword and shape counts come from scripts over `schema/schema.unstable.json` and `types_gen.go`;
* the merge counts come from a scratch test in a copy of `cmd/generate/internal/load`;
* the hostile schemas ran through a build of `cmd/generate` with `-schema` and `-out` pointing at a scratch directory;
* coverage is `go test -coverpkg=./... ./...` in `cmd/generate`.

F2, F3, and F1 predate the go/ast port: the port kept the Jennifer output byte for byte, as
[0006-MADR-emit-go-with-go-ast.md](0006-MADR-emit-go-with-go-ast.md) required.

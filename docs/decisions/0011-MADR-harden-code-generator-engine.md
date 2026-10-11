---
status: accepted
date: 2026-10-10
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
   union variant; tests cover unknown variants, the deserialization keywords, and precision.

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

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
4. **Schema fidelity.** Generated decoders honour `x-deserialize-default-on-error` and `x-deserialize-skip-invalid-items`. Integer fields take the Go type
   of their declared format (`int32`, `int64`, `uint32`, `uint64`), and typed `additionalProperties` become typed maps. `Validate` descends into unions,
   nested types, and slices, and checks the bounds the schema declares.
5. **Safe output.** All five files are rendered in memory, type-checked together with `go/types` against the hand-written package, and written only if
   every step passes, each through a temporary file and a rename. Panics become errors.
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
  `IdleStateUpdate` given `{}` decodes as `EndTurn`, not `none`; `SessionUpdate` without `sessionUpdate` decodes as `UserMessageChunk`; `ContentBlock`
  `{"text":"hi"}` decodes as `Text` though every `ContentBlock` variant requires `type`.
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
* **Absent or null discriminator: key-less variants only.** Only variants with no discriminator value, other than the catch-all, are candidates: the
  four defaulted unions' default, `IdleStateUpdate`'s `none`, and `MultiSelectItems`' `titled`. In the other eleven unions an absent key is an error
  naming the union and the key.

Phase 2 also takes F18 and F19: every union's `UnmarshalJSON` starts from the zero value, except that JSON `null` leaves a union without a null
variant unchanged and without error, as `encoding/json` does for other types.

### Consequences of these decisions

* Good, because a newer peer's value lands where the schema says, `other` or `Unknown`, and is sent on unchanged.
* Good, because `IdleStateUpdate` `{}` decodes as `none` and `SessionUpdate` stops guessing a variant.
* Bad, because eleven unions now reject a payload without its discriminator, `ContentBlock` `{"text":"hi"}` among them, which decoded before.
* Bad, because there are two mechanisms, `other` with `Extra` and `Unknown`, and the exported API gains `Unknown` on eleven unions and `Extra` on six
  variant types.

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

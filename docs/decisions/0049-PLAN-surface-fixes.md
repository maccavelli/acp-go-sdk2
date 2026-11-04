---
status: in-progress
date: 2026-11-03
associated-madr: "0049-MADR-remediate-the-2026-11-01-debugging-pass.md"
---

# Implement 0049, the SDK surface: S1 to S9

Associated MADR: [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md)

## Goal

Fix the nine findings of the SDK surface against the schema, S1 to S9, as the MADR states them. The owner's answers settle three of
them: Q3 (a), gate the unstable capabilities, for S3; Q4 (a), strict on send and readable on receive, for S6; and Q7 (a), the
order. The MADR's "The surface PLAN's designs, proposed 2026-11-03" settles the four details this PLAN needs: Q8 to Q11, each
answered (a) by the owner on 2026-11-03. Each fix lands with the finding's probe as a committed test, seen to fail
on the code it replaces and to pass after. This is the second of the MADR's five PLANs, and its fixes go out as a minor
release, `v1.32.0`, with notes that lead with the behaviour changes.

Base: `e98987a`, whose Go code is `v1.31.2`'s. Every file and line below is at that commit. The MADR's "Re-assessment at
`e98987a`" shows every S probe still failing there.

## Scope

In scope:

* **The generator,** `cmd/generate`:
  * the union decoder: `internal/emit/types.go:920-1006`, `discUnmarshalBody` and `keyLessBody`;
  * the variant model: `internal/model/model.go:179-230`, `Variant`;
  * the protocol table: `internal/model/acp.go` (`Methods` `:25-88`, `PatchFields` `:92-105`, `CapabilityRules` `:162-189`)
    and its types in `internal/model/protocol.go`: `Protocol` (`:16-41`), `PatchRule` (`:106`), `GateKind` (`:111-131`),
    `CapabilityRule` (`:137`), `MethodRule` (`:167`), and the rule checks (`:366`, `:448-456`);
  * the gates: `internal/model/gates.go:150-219`, and `internal/emit/gates.go`;
  * `validate`: `internal/emit/types.go:1140-1160`, `addValidate` and `validateCall`;
  * the decoders of lenient fields;
  * new engine cases under `cmd/generate/testdata/engine/`;
  * the regenerated `types_gen.go`, `agent_gen.go`, `client_gen.go` and `constants_gen.go`.
* **Hand-written code:**
  * `connection.go:215-241`, the `validator` interface and `validateOutbound`, and `judgeInbound` (`:3107`, its `validate` call
    `:3115`);
  * `errors.go`, one new sentinel beside `ErrTerminalAuthMethod` (`:178`);
  * `helpers.go:909-921`, `WithUpdateContent` and `WithUpdateLocations`;
  * `helpers.go:348-362`, `unknownKind`, which the generator emits from phase 4 on (deviation 4), with
    `internal/emit/decode.go`;
  * `tooltimeline.go:172-179` and `compaction.go:134-139`, the copies, with one new unexported helper file.
* **Tests:** one test per finding, under the names each step gives, in the root package and the generator's packages.
* **Docs:**
  * `README.md`: "Wire encoding", "Initialization and capabilities", and the validation and tool-call text;
  * `docs/architecture.md`;
  * an amendment to [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), treatments 5 and 7.
* **The release,** a minor `v1.32.0`, on the owner's ask. `make api` must report ~~one compatible addition, the sentinel,~~
  26 compatible additions, the sentinel and deviation 3's 25 `Validate` methods, and nothing incompatible.

Out of scope:

* the other three areas' findings (Q7 a);
* any change to an exported type's or field's Go type;
* `mcp/message` and `positionEncoding`, which no finding names (the MADR's re-assessment records them);
* the 1.26.0 schema.

## Design rules

The Go API rules each step follows:

1. **No incompatible change** ([0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md),
   the `v1` promise in `README.md`, "Stability"). No exported type, field, function or method changes its signature, and none is
   removed. A struct tag may change: the pinned `apidiff` does not report it, and the wire meaning it gains is the schema's own.
   Checked 2026-11-03: in a scratch clone of `e98987a`, with the `content` and `locations` tags of `types_gen.go` changed to
   `omitzero`, `scripts/check_api.py` gave "against v1.31.2: 0 incompatible, 0 compatible".
2. **The least new API.** ~~One exported identifier is added, the sentinel of step 4.2.~~ The sentinel of step 4.2, and
   the 25 `Validate` methods of deviation 3, 2026-11-03, are added. It is an `Err…` variable read with
   `errors.Is`, as `ErrTerminalAuthMethod` is. Membership of a closed set is reported by the existing `Validate`, so no enum gains
   an exported method. The deep copy is unexported.
3. **Errors keep their types.** A gate refusal is a `*CapabilityError` or wraps a sentinel, and a validation failure wraps the
   field's path, as today. Nothing is logged in place of returning, and every wrap uses `%w`.
4. **nil and empty.** Go treats a nil and an empty slice alike, except where JSON tells them apart: "a nil slice encodes to null,
   while `[]string{}` encodes to the JSON array `[]`" (the Go wiki, CodeReviewComments, "Declaring Empty Slices"). Only the two
   fields of step 2 make the distinction, and their godoc says so. The option helpers treat nil as empty, so a caller need not
   know.
5. **Behaviour changes are documented where the call is.** Each one is stated in the godoc of the type, field or option it
   changes, in the README, and in the release notes.
6. **The generator states what it applies.** Each new protocol-table rule quotes the schema sentence it follows, and `Build`
   refuses a rule whose definition's description does not contain that sentence, as `resolveNullRetained` does
   (`internal/model/model.go:1451-1486`).

## Implementation Steps

Every phase ends staged for the owner, who commits. A phase begins only on the owner's word. Each new test is a probe from the
pass's surface copy (`TestProbe…` in the session's `p49/surface/src`) under a descriptive name. Each is seen to fail on a
planted break in a scratch copy, `tools/plants.py`, never in the tree, with the plant restoring the code the test guards. An
emitter's plant runs in a scratch copy by script, since `plants.py` runs from the root and `cmd/generate` is its own module
(the runtime PLAN's phase 3). Every phase runs `make pre-add-check` with its files, `tools/gates.py regen`, and
`go test ./...` in `cmd/generate`.

### Phase 1: S1 and S7, the key-less variant (Q8)

1. **The rule,** in `discUnmarshalBody` (`internal/emit/types.go:920-981`), for a union with a key-less variant:
   * *Amended by deviation 1, 2026-11-03: both rules below apply only to a union without a catch-all. Amended by deviation 2,
     2026-11-03: the second rule, the fallback, applies only to a union a protocol-table rule names, with the schema's
     sentence.*
   * **A discriminator equal to the key-less variant's own name** decodes into that variant, as a named variant's does. A
     payload that does not decode is an error, `"<Union>: type \"<name>\": …"`. The name is the variant's schema `title`, which
     `Kind()` already returns for it (`helpers.go:439-470`). `Variant` gains `Title`, set for a key-less variant.
   * **A discriminator that names no variant, in a union without a catch-all,** decodes into the key-less variant when the
     payload has its required keys and decodes as it. Otherwise it is kept in `Unknown`, as now. Under JSON Schema's `anyOf`
     such a payload is valid as the key-less variant, which declares no discriminator and no `additionalProperties`. The
     schema's `SetSessionConfigOptionRequest` says so for its own: "Unknown `type` values with string payloads also gracefully
     deserialize into this variant". The official TypeScript and Python SDKs decode the same way (the MADR's re-assessment).
   * **A union with a catch-all,** such as `MultiSelectItems` or `IdleStateUpdate`, keeps sending an unknown value to the
     catch-all, as the schema's catch-all says, its key-less variant's own name included (deviation 1).
   * Absent or `null`, the discriminator keeps today's `keyLessBody` rule.
2. **The model:** `Variant.Title`, and a check in `Build` that two key-less variants of one union do not share a title.
3. **Engine cases** (`cmd/generate/testdata/engine/`):
   * `accept-key-less-variant-named.txtar`: the own name decodes into the variant, and a payload missing its keys fails.
   * `accept-key-less-variant-fallback.txtar`: an unknown discriminator with the variant's keys decodes into it; one without
     them, or whose values do not decode, is `Unknown`; a union with a catch-all is unaffected.
4. **Root tests,** `union_keyless_test.go`:
   * `TestSetConfigOptionUnknownTypeIsValueId`: the probe, end to end. An agent's handler receives `ValueId` with `sessionId`,
     `configId` and `value`. `type` with a boolean `value` stays `Unknown`.
   * `TestKeylessVariantNamedExplicitly`: `McpServer` `{"type":"stdio",…}` decodes into `Stdio`, and
     `ResolveStdioCommands` resolves it. `AuthMethod` `{"type":"agent",…}` decodes into `Agent`. `"type":"stdio"` without
     `command` is an error naming the variant.
5. **Docs:**
   * The generated `Unknown` field doc for a union with a key-less variant reads "… names no variant this SDK knows and does not
     fit the key-less variant".
   * `README.md`'s "Wire encoding" says the same.
6. **Seen to fail:**
   * the own-name case removed: `TestKeylessVariantNamedExplicitly` fails;
   * the fallback removed: `TestSetConfigOptionUnknownTypeIsValueId` fails;
   * the catch-all guard removed: the fallback engine case fails.
7. **Regenerate.** `git diff` of `types_gen.go` touches only the decoders of the unions with a key-less variant. The record
   names them: `AuthMethod`, `McpServer` and `SetSessionConfigOptionRequest`, with the unstable copy `UnstableMcpServer`; only
   `SetSessionConfigOptionRequest` gains the fallback (deviation 2). ~~`MultiSelectItems` and `IdleStateUpdate`, the
   last two for the own name only.~~ Those two have a catch-all and do not change (deviation 1).

### Phase 2: S2, an empty collection is sent

1. **A protocol-table rule,** `EmptyClears []PatchRule` in `Protocol` (`internal/model/protocol.go`): the optional array
   properties whose empty value replaces the collection. `Build` resolves it as `resolveNullRetained` does
   (`internal/model/model.go:1451-1486`):
   * The rule is refused on a required property, on one that is not an array, or when the description lacks its `Source`.
   * Every field built from the property, an inline variant's included, is tagged `key,omitzero`, so a nil slice is omitted
     and an empty one is sent as `[]`.
   * The field's doc gains "A nil slice leaves the collection unchanged and is not sent; an empty one clears it."
   * The rules are `ToolCallUpdate.content` and `ToolCallUpdate.locations`, with `Source` "Replace the content collection."
     and "Replace the locations collection." They reach `ToolCallUpdate` and `SessionToolCallUpdate`.
2. **Decoding is unchanged.** `skipInvalid` already reads `[]` as a non-nil empty slice and absence or `null` as nil
   (`types_gen.go:23676-23694`), and `ToolCallTimeline.Apply` already clears on empty (`tooltimeline.go:45-47`).
3. **The helpers.** `WithUpdateContent` and `WithUpdateLocations` (`helpers.go:909-921`) set a non-nil empty slice when given
   nil, so passing either option always replaces the collection, and `WithUpdateContent(nil)` clears it. Their godoc says so.
4. **Tests:**
   * `TestToolCallUpdateEmptyCollectionsOnWire`, in `helpers_test.go`, the probe: `WithUpdateContent([]ToolCallContent{})`,
     `WithUpdateContent(nil)` and `WithUpdateLocations(nil)` send `"content":[]` and `"locations":[]`; no option sends neither
     key. A client's `ToolCallTimeline` that held content holds none after the update.
   * A model test in `internal/model/patch_test.go` for the rule's three refusals.
   * a `checkCommittedEmptyClears`, as `patch_test.go:309` and `:361` do for the other rules, so every rule applies to the committed schema.
5. **Seen to fail:** the tag left `omitempty`: the wire test fails; the helper's nil case removed: its subtest fails.

### Phase 3: S6, closed enumerations (Q4 a)

1. **The closed sets.** A closed enumeration is a `KindEnum` or `KindStringConstUnion` type
   (`internal/model/model.go:19-36`, `classify` at `:442-460`), not a `KindOpenEnum`. At `e98987a` they are:
   * in the stable schema: `StopReason`, `PermissionOptionKind`, `PlanEntryPriority`, `PlanEntryStatus`, `ToolKind`,
     `ToolCallStatus`, `Role`, `StringFormat` and `ElicitationSchemaType`;
   * in the unstable schema: `NesDiagnosticSeverity`, `NesRejectReason`, `NesTriggerKind`, `PositionEncodingKind` and
     `TextDocumentSyncKind`.

   The step's test lists them from the model, so a new one is checked without editing the test.
2. **What `validate` checks.** The generated unexported `validate(abs func(string) bool)` becomes `validate(r validation)`, with
   `validation` a hand-written unexported struct in `connection.go`: `abs func(string) bool` and `closed bool`.
   * `validateOutbound` passes `{isPortableAbs, true}`, and the exported `Validate` passes `{filepath.IsAbs, true}`.
   * `judgeInbound` passes `{filepath.IsAbs, false}`, so a received value is kept, as Q4 (a) chose.
   * With `closed` set, a closed enumeration field is checked:
     * a required one always, so `""` is refused;
     * an optional value one when it is not `""`, which `omitempty` sends as absent;
     * a pointer one when it is not nil;
     * each item of a slice.
   * The error names the field and the value: `PromptResponse: stopReason: "" is not a StopReason`.
   * Each closed type gains an unexported `known() bool`.
   * `validateCall`'s external path, for package `unstable`, calls the exported `Validate`. That package holds no generated
     type at `e98987a`, and the step records that it still does not cross a closed type.
3. **Lenient fields, as the schema says** (0012's treatment 5, and treatment 6 for items):
   * A closed enumeration in a field with `x-deserialize-default-on-error` reads an unknown value as absent: `ToolCall.kind`
     and `.status`, `ToolCallUpdate.kind` and `.status`, `ElicitationSchema.type` (its schema default, `"object"`),
     `RejectNesNotification.reason` and `AgentCapabilities.positionEncoding`.
   * An item of a closed enumeration in an array with `x-deserialize-skip-invalid-items` is dropped when unknown:
     `Annotations.audience`, `ClientCapabilities.positionEncodings`.
   * A required, non-lenient field, such as `stopReason`, keeps the value it received.
4. **The SDK's own senders.** Every place the SDK builds a value it sends is checked against the new rule:
   * the generated prompt-turn answer;
   * `acptest`'s `ScriptAgent` and its client;
   * `agentproc`'s `InProcess`;
   * the examples;
   * `tools/consumer.py`'s program.

   One that would send a non-member, such as an empty `stopReason`, is fixed and named in the record. A test that sent one on
   purpose is changed only where it tested something else, and the record names it.
5. **Tests,** `enum_closed_test.go`:
   * `TestClosedEnumsRefusedOnSend`: for each closed type in a required position, `""` and `"bogus"` fail `Validate`. An
     agent whose `Prompt` returns `PromptResponse{}` answers `-32603`, and the client's `Prompt` returns that error.
   * `TestClosedEnumsReadableOnReceive`: a client receives `{"stopReason":"bogus"}`, and `Prompt` returns it with no error.
   * `TestLenientClosedEnumsReadAbsent`:
     * `tool_call` `kind:"browse"`, `status:"queued"` read as `""`;
     * `tool_call_update`'s read as nil;
     * `audience` `["user","system"]` reads `["user"]`;
     * a known value is kept.
   * The generator: an engine case, `accept-enum-closed.txtar`, and a model test that `KindOpenEnum` is not checked.
6. **Docs:**
   * Each closed type's generated doc gains "Validate refuses any other value; one that is received is kept".
   * The README's "Wire encoding" says it.
   * An amendment to [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), dated, naming this PLAN:
     * treatment 7 now checks a closed set's membership, on what is sent and in `Validate`;
     * treatment 5 now reads an unknown closed value as absent in a lenient field.
7. **Seen to fail:** membership not checked: the send test fails; `judgeInbound` passing `closed: true`: the receive test fails;
   the lenient read removed: the absent test fails.

### Phase 4: the gates, S8, S9, S5 and S3 (Q3 a, Q10)

1. **S5:** `{Side: sideClient, Definition: "SetSessionConfigOptionRequest", Variant: "boolean", Capability:
   "session.configOptions.boolean", Kind: GateVariant}`. Its source is the schema's sentence: "the client may send
   `session/set_config_option` requests with `type: \"boolean\"`". Test: `TestBooleanSetConfigOptionGated`, the probe. Sending
   without the capability fails with a `*CapabilityError`; with it, the request succeeds; a received one without it is answered
   `-32602`.
2. **S8:** a gate kind, `GateAdvertisedAuthMethod`, in `internal/model/protocol.go` and `gates.go`, for `AuthenticateRequest.methodId`:
   * The id must name a method of the initialize response's `authMethods`: a known variant's id, or the `"id"` of an entry kept
     in `Unknown`, read with `unknownKind` ~~(`helpers.go:351`)~~, generated beside `foldKeys` (deviation 4, 2026-11-03).
   * The terminal rule runs first and keeps `ErrTerminalAuthMethod`.
   * Otherwise the error is `fmt.Errorf("%s: methodId %q: %w", method, id, ErrUnadvertisedAuthMethod)`.
   * `errors.go` adds `ErrUnadvertisedAuthMethod = errors.New("acp: methodId names no auth method the agent advertised")`, and
     `errors_test.go`'s table adds its text.
   * Test: `TestAuthenticateUnadvertisedMethod`, the probe:
     * an id not advertised fails with `errors.Is(err, ErrUnadvertisedAuthMethod)` and writes nothing;
     * an agent that receives one answers `-32602`;
     * an advertised agent method passes;
     * an unknown-type entry's id passes.
3. **S9:** `GateUndeclaredVariant` extended to a property's union whose unknown payload is the raw `Unknown`:
   * The union is resolved through `Property`, as `GateVariant` does, and `catchAll` (`gates.go:206-219`) accepts `Unknown`,
     reading the discriminator with `unknownKind`, generated (deviation 4).
   * The rule is `{Side: sideAgent, Definition: defPrompt, Property: keyPrompt, Capability: "promptCapabilities", Kind:
     GateUndeclaredVariant}`, sourced from `PromptCapabilities`: "Other variants must be explicitly opted in to."
   * Its error is `*CapabilityError{Capability: "promptCapabilities.<type>", Field: "prompt[i]", Variant: "<type>"}`.
   * Test: `TestPromptUndeclaredBlockGated`, the probe. A `"video"` block is refused even with every capability advertised; a
     text or resource-link block passes.
4. **S3:** the unstable capabilities. Each rule quotes its capability's description:
   * **Methods:**
     * `providers/list`, `providers/set` and `providers/disable` need `providers` ("Supplying `{}` means the agent supports
       provider configuration methods").
     * `nes/start`, `nes/suggest`, `nes/accept`, `nes/reject` and `nes/close` need `nes` ("Omitted or `null` both mean the
       agent does not advertise support for NES methods").
     * `document/didOpen`, `didChange`, `didClose`, `didSave` and `didFocus` need `nes.events.document.<event>` ("Whether the
       agent wants `document/<event>` events").
   * **Variants:**
     * the `SessionUpdate` variants `plan_update` and `plan_removed` need the client's `plan` ("Whether the client supports
       `plan_update` and `plan_removed` session updates").
     * the `SuggestNesResponse.suggestions` variants `jump`, `rename` and `searchAndReplace` need the client's `nes.jump`,
       `nes.rename` and `nes.searchAndReplace`. `edit`, the baseline, is not gated.
   * Test: `TestUnstableCapabilitiesGated`, the probe. Each is refused without its capability and allowed with it. A received
     method is answered `-32601`, and a received variant `-32602`.
5. **Docs:** the README's "Initialization and capabilities" lists the new methods and values, the unstable ones marked, and the
   authenticate and prompt rules; and `docs/architecture.md` lists the gate kinds.
6. **Seen to fail:** each rule removed from the table in its own plant; its test fails.

### Phase 5: S4, the timelines' copies (Q9)

1. **A deep copy,** `deepCopy[T any](v T) T`, unexported, in a new `deepcopy.go`:
   * It walks the value with `reflect`: pointers, slices (a `json.RawMessage` included), maps and interfaces are copied
     element by element, and structs field by field. Every field the timelines reach is exported (checked at `e98987a`: none
     is unexported, and the only interface values are `_meta`'s).
   * `copyToolCall` and `copyEntity` use it.
   * The timelines' godoc says "a copy that shares nothing with the timeline".
2. **Tests:**
   * `TestToolCallTimelineCopies` (`tooltimeline_test.go:142`) and `TestCompactionTimelineCopies` (`compaction_test.go:153`)
     gain the probes' mutations through pointers: a content block's text, a diff's `NewText`, a location's line, an
     annotation's audience, and a nested `_meta` map. Each is changed in the returned copy, and the next `Get` is unchanged.
   * `TestDeepCopySharesNothing`: a reflection filler sets every field of `ToolCallEntity` and `CompactionEntity`, through
     each union variant. The copy must share no pointer, slice or map with the original, and must `reflect.DeepEqual` it. A
     field the schema adds later is then covered without editing the test.
3. **Seen to fail:** `copyToolCall` back to `slices.Clone`: the copies test fails; `deepCopy` skipping interfaces: the filler
   test fails on `_meta`.

### Phase 6: close, and the release on the owner's ask

1. `tools/gates.py --all`, with the hosts named per command. The MADR's S items are listed, each with the test that shows it, in
   a table.
2. `make api`: ~~"1 compatible" (`ErrUnadvertisedAuthMethod`)~~ 26 compatible (`ErrUnadvertisedAuthMethod` and deviation 3's
   25 `Validate` methods), 0 incompatible.
3. After the owner's push, CI on the head.
4. On the owner's ask:
   * `docs/reports/0049-REPORT-release-notes-v1-32-0.md`, leading with the behaviour changes;
   * `tools/release_body.py`;
   * the owner's tag `v1.32.0` and release, or the job [0048-PLAN-create-releases-in-ci.md](0048-PLAN-create-releases-in-ci.md)
     adds, if it has landed;
   * `tools/publication.py`;
   * `tools/consumer.py v1.32.0`, updated first if step 3.4 changed what it sends.
5. This PLAN and its row are marked `complete`, and the MADR's S findings done.

## Verification

* Each of S1 to S9 has a committed test that fails at `e98987a`'s code, by its plant, and passes after.
* `make api`: one compatible addition, nothing incompatible against `v1.31.2`.
* `tools/gates.py regen` is clean, and `go test ./...` passes in both modules.
* The behaviour changes are stated in the godoc, the README and the release notes:
  * the key-less fallback;
  * the empty collections;
  * membership on send;
  * lenient reads;
  * the new gates.
* `tools/gates.py --all` and CI pass.

## Rollout and Rollback

* **Rollout.** A minor release, `v1.32.0`. What a consumer meets, in the notes' order:
  * values the SDK sent before and now refuses: an empty or unknown closed value, such as `PromptResponse{}`'s `stopReason`;
    an unadvertised `methodId`; a boolean config option, an unstable method or variant, or an undeclared prompt block without
    its capability;
  * values read otherwise: an unknown `type` that fits the key-less variant; an unknown closed value in a lenient field, read
    as absent;
  * an empty `content` or `locations` now clears the collection, and the option helpers send it.
* **Rollback.** Revert the phase's commit; each phase stands alone. A consumer that needs the old sending behaviour pins
  `v1.31.2`.

## Execution record

### Phase 1: S1 and S7, 2026-11-03

Begun on the owner's "committed, proceed" after `df1eb2c`.

**Deviation 1, 2026-11-03: the own name applies only in a union without a catch-all.**

* **Found,** reading the schemas before any change: step 1.1 gave the own-name rule to every union with a key-less variant,
  and also kept a catch-all union's unknown values with the catch-all. For two unions the rules collide.
  * In the unstable `IdleStateUpdate`, the key-less variant's title is `none`: "No stop reason: `stopReason` is omitted or
    `null`". A title is a label here, not a wire value. The catch-all `other` takes "Custom or future stop reason … Other
    unknown values are reserved for future ACP variants". At `e98987a`, `{"stopReason":"none"}` decodes into `Other`
    (`types_gen.go:6951-6957`), and step 1.1 as written would have moved it to `None`.
  * `MultiSelectItems`'s key-less `titled` is the same case.
* **Decided.** The owner was asked, with three options, and chose "fallback plus strict own name". Both rules apply only to a
  union without a catch-all:
  * the own name decodes into the key-less variant, and is an error naming the variant when the payload does not decode;
  * any other unknown discriminator decodes into it when the payload fits and decodes, and into `Unknown` otherwise.

  A union with a catch-all is unchanged. Not chosen: the fallback alone, under which a non-fitting own name reads as
  `Unknown`; and keeping the plan as written, which contradicts those schemas.
* **Scope:** `Variant.Title` stays (step 1.2). Step 1.3's engine cases gain a catch-all union with a titled key-less variant,
  which must not change. Step 1.7's list loses the two catch-all unions. The MADR's Q8 carries an amendment of the same date.

**Deviation 2, 2026-11-03: the fallback only where the schema states it, as 0011's F1 requires elsewhere.**

* **Found,** after the change, when the generator's and the root package's tests ran. The general fallback failed three tests
  that pin a decision this PLAN and the MADR's Q8 had not named:
  [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md)'s F1, "a discriminator value that names
  no variant is never matched by key or by trial; it goes to `Unknown`". Its probes included a future auth method re-encoded as
  another type. The failures:
  * `TestRemainingUnionKindsDecoded`: "AuthMethod {"type":"future","id":"x","name":"X"}: decoded Kind() = "agent", want
    "future"";
  * `TestConformanceUnions/AuthMethod`: "an unknown type decoded into [Agent], want Unknown", and "… was sent back as
    {"id":"s","name":"s"}";
  * the engine case `accept-key-less-variant`: "{"type":"q","x":"v"} decoded as Loose, want Unknown", its comment "never
    matched to the key-less variant (F1)".
* **Decided.** The owner was asked, with three options, and chose "strict own name + S1 rule":
  * the own name stays, strictly, in a union without a catch-all, as deviation 1 has it. A named variant is not an unknown one,
    so F1 holds, and S7 is fixed.
  * the fallback applies only to a union that a new protocol-table rule, `KeyLessFallbacks`, names. Its `Source` must be in the
    key-less variant's description. The one rule is `SetSessionConfigOptionRequest`, quoting "Unknown `type` values with
    string payloads also gracefully deserialize into this variant." That fixes S1.
  * `AuthMethod` and `McpServer` keep `Unknown` for a future type, as 0011 decided. The three tests keep their contract and do
    not change.

  Not chosen: keeping the general fallback and amending 0011; and a hand special case for S1 in place of a sourced rule.
* **Scope added:** `Protocol.KeyLessFallbacks` and `FallbackRule` (`internal/model/protocol.go`), their checks, `Variant.Fallback`,
  and the rule in `internal/model/acp.go`. The engine case for the fallback declares a union named
  `SetSessionConfigOptionRequest`, and a model test checks the rule's refusals. The MADR's Q8 carries a second amendment.

**What it did,** under deviations 1 and 2:

* **The model** (`cmd/generate/internal/model/`):
  * `Variant.Title`, a key-less variant's schema title, is set only in a union without a catch-all, by `keyLessTitles` from
    `classifyUnion`. `Variant.Fallback` marks the key-less variant a rule names, and the unexported `description` holds the
    text the rule's source is checked against.
  * `Protocol.KeyLessFallbacks` and `FallbackRule` (`protocol.go`), checked for a definition, a source, and one rule per
    definition. `resolveKeyLessFallbacks` (`model.go`) refuses a rule on a type that is not a discriminated union without a
    catch-all, on one without exactly one key-less variant, or whose key-less variant's description does not state the
    source, compared with white space collapsed.
  * The one rule (`acp.go`) is `SetSessionConfigOptionRequest`, with `srcValueIDFallback`.
  * The step's title check, that a title is not another variant's value, was dropped as unreachable. A title and a
    discriminator value derive the same field name, which `buildUnion` already refuses twice: "field A is already used by
    variant 1", seen when the test tried it.
* **The emitter** (`internal/emit/types.go`):
  * `discUnmarshalBody` adds a case for a key-less variant's title, decoded strictly by `decodeChosen`.
  * In a union with `Unknown`, before the payload is kept there, each `Fallback` variant is tried by `tryKeyLess`: its keys,
    then a decode whose error falls through.
  * The present value stays `v` unless a fallback block needs that name, so the other unions' decoders do not change. A
    first run renamed it everywhere and touched eight unrelated decoders; the rename was confined.
  * `Unknown`'s field doc names the fallback, only where there is one.
* **Regenerated** `types_gen.go`, 48 lines added and 4 removed:
  * the own-name case in `AuthMethod`, `McpServer` and `UnstableMcpServer`;
  * the own name and the fallback in `SetSessionConfigOptionRequest`.

  Nothing else changed, and `tools/gates.py regen` is clean.
* **The tests:**
  * The model: `TestKeyLessTitlesAndFallbacks`, and `checkCommittedKeyLessFallbacks` in
    `TestProtocolTableAppliesToCommittedSchema`. The latter requires the variants whose description says they take unknown
    values to be the ones a rule marks, `[SetSessionConfigOptionRequest.ValueId]`.
  * The engine cases:
    * `accept-key-less-variant-named.txtar`: the own name, strict, and a catch-all union that keeps the title;
    * `accept-key-less-fallback.txtar`: the fallback, a boolean value kept and sent back unchanged, the own name strict, and
      a union no rule names keeping `Unknown`.
  * The root, `union_keyless_test.go`:
    * `TestSetConfigOptionUnknownTypeIsValueId`: the probe end to end, with a boolean value kept.
    * `TestKeylessVariantNamedExplicitly`:
      * `"type":"stdio"`, which `ResolveStdioCommands` then resolves;
      * `"type":"agent"`;
      * the strict error;
      * an unknown type with stdio's keys kept in `Unknown`.
  * The three tests that pin 0011's F1 pass unchanged: `TestRemainingUnionKindsDecoded`, `TestConformanceUnions` and
    `accept-key-less-variant`.
* **Docs:** `README.md`'s "Wire encoding", and `docs/architecture.md`'s union decoding.

**Seen to fail,** each in its own scratch copy of the tree, regenerated there, by script (`plants.py` runs from the root, and
the emitter is in `cmd/generate`'s module), 6/6:

```text
FAILED AS REQUIRED: own name removed: rc 1: union_keyless_test.go:80: McpServer {"type":"stdio",...} decoded as {Http:<nil> Sse:<nil> Acp:<nil> Stdio:<nil> Unknown:{"type":"stdio","name":"fs","command":"npx","args":[],"env":[]}}
FAILED AS REQUIRED: fallback removed: rc 1: union_keyless_test.go:56: handler got {Boolean:<nil> ValueId:<nil> Unknown:{"sessionId":"s","configId":"model","type":"select","value":"fast"}}, Kind() "select"; want the value-id variant {s model fast}
FAILED AS REQUIRED: own name in a catch-all union too: rc 1: behaviour_test.go:74: {"type":"plain","y":"1"} decoded as Plain, want Other plain
FAILED AS REQUIRED: fallback for every key-less variant (F1 broken): rc 1: union_keyless_test.go:107: an unknown type with stdio's keys decoded as {… Stdio:0x… Unknown:null}, want Unknown
FAILED AS REQUIRED: the source check removed: rc 1: keyless_test.go:82: not stated: Build = <nil>, want an error containing "Silent: its key-less variant's description does not state …"
FAILED AS REQUIRED: the rule dropped from the table: rc 1: protocol_test.go:511: key-less variants that say they take unknown values: [SetSessionConfigOptionRequest.ValueId]; marked by a rule: []; want both [SetSessionConfigOptionRequest.ValueId]
```

On the first run, "fallback removed" deleted the whole `if`, which left `vi` unused, and the generator failed to build. The
plant became `if false && vi.Fallback`. The second plant of the model's pair first lacked the schema files in its copy, and the
copy now takes the whole tree.

**Checks:**

* `go test -count=1 ./...` in `cmd/generate`: `ok`.
* `go test -count=1 .`: `ok`.
* `go test -race -count=3` of the two root tests: `ok`.
* `tools/gates.py regen`: `ok`.
* `make pre-add-check` with the phase's eight Go files and the README:
  * **First run:** exit 2. `golangci-lint`'s `gocyclo` found "cyclomatic complexity 43 of func `newRules` is high (> 40)",
    which the fallback rules' checks had added. They moved to `checkFallbackRules`, and `newRules` calls it.
  * **Second run,** at load 21 once the wait's 25-minute bound passed: exit 0, "8 file(s) gofmt-clean; both modules linted,
    vetted and tested (-race -shuffle=on, uncached), tidy, govulncheck."
* `tools/remote.py tree --host linux`: both modules, exit 0.
* `tools/remote.py pkg --host windows . -test.run 'TestSetConfigOptionUnknownTypeIsValueId|TestKeylessVariantNamedExplicitly|
  TestRemainingUnionKindsDecoded' -test.count=3`: "PASS".
  * A first run that also named `TestConformanceUnions` failed with "open schema\\schema.json: The system cannot find the path
    specified". `pkg` sends the test binary alone, and that test reads the schema. The Linux run, which copies the tree,
    passes it.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0 each. `tools/idscan.py --paths` over the phase's
  files: 0 matches.

Staged for the owner, who commits. Phase 2 begins on the owner's word.

The owner committed phase 1 as `22d508e`.

### Phase 2: S2, an empty collection is sent, 2026-11-03

Begun on the owner's "committed, proceed" after `22d508e`. No deviation.

**What it did:**

* **The rule.**
  * `Protocol.EmptyClears []PatchRule` (`internal/model/protocol.go`), checked by `checkPropertyRules`.
  * `resolveEmptyClears` (`internal/model/model.go`) runs after `resolveKeyLessFallbacks`. A rule on a required property, on
    one that is not an array, or whose property's description does not state its source is refused. Every field built from
    the property, an inline variant's included, is tagged `key,omitzero`, and its doc gains `emptyClearsDoc`: "A nil slice
    leaves the collection unchanged and is not sent; an empty one clears it."
  * The two rules, in `internal/model/acp.go`, name `ToolCallUpdate.content` and `.locations`, quoting "Replace the content
    collection." and "Replace the locations collection."
* **`checkPropertyRules`** also checks the patch and null-retained rules, which `newRules` checked in two identical loops,
  with the same error texts. The third loop would have taken `newRules` to a cyclomatic complexity of 41, past
  `golangci-lint`'s 40.
* **Regenerated** `types_gen.go`: the tags and doc of `SessionToolCallUpdate` and `ToolCallUpdate`, 8 lines added and 4
  removed; nothing else. `tools/gates.py regen` is clean.
* **The helpers.** `WithUpdateContent` and `WithUpdateLocations` (`helpers.go`) send `[]` for nil, so the option always
  replaces the collection, and their godoc says so. Decoding is unchanged: `[]` stays an empty slice, and absence nil.
* **The tests:**
  * `TestToolCallUpdateEmptyCollectionsOnWire`, in a new `helpers_test.go`:
    * the options with empty and nil slices send `[]`, and with none send neither key;
    * a hand-built `ToolCallUpdate` sends `[]` for empty and nothing for nil;
    * a received `[]` decodes as an empty slice and is sent back as `[]`, and an absent key decodes as nil;
    * a timeline that held content holds none after the cleared update, read from the wire.
  * `TestEmptyClearsRules` and `checkCommittedEmptyClears`, in `internal/model/patch_test.go`. The latter, called from
    `TestProtocolTableAppliesToCommittedSchema`, requires every property whose description reads "Replace the … collection" to
    be ruled. It also requires the slice fields tagged `omitzero` to be exactly the four, inline variants included. As first
    written it read top-level fields only, and found two of the four.
* **Docs:**
  * `README.md`'s tool-call timeline bullet says how to clear when sending.
  * `docs/architecture.md` lists `KeyLessFallbacks` and `EmptyClears` among the table's rules.

**Seen to fail,** each in its own scratch copy of the tree, regenerated there, by script, 4/4, then again after the lint fixes,
4/4:

```text
FAILED AS REQUIRED: the tag left omitempty: rc 1: helpers_test.go:41: empty: sent {"sessionId":"s","update":{"sessionUpdate":"tool_call_update","toolCallId":"t1"}}, want "content":[]
FAILED AS REQUIRED: the helper's nil case removed: rc 1: helpers_test.go:41: nil: sent {"sessionId":"s","update":{"locations":[],"sessionUpdate":"tool_call_update","toolCallId":"t1"}}, want "content":[]
FAILED AS REQUIRED: the source check removed: rc 1: patch_test.go:472: not stated: Build = <nil>, want an error containing "Cleared.other: its description does not state …"
FAILED AS REQUIRED: a rule dropped from the table: rc 1: protocol_test.go:512: ToolCallUpdate.locations: its description says it replaces its collection, and no empty-clears rule names it
```

**Checks:**

* `go test -race -count=3 -run TestToolCallUpdateEmptyCollectionsOnWire .`: `ok`.
* `go test -count=1 ./...` in `cmd/generate`, and `go test -count=1 .`: `ok`.
* `tools/gates.py regen`: `ok`.
* `make pre-add-check` with the phase's eight Go files and the README:
  * **First run,** at load 16 to 33: exit 2.
    * `gocyclo`: "cyclomatic complexity 41 of func `newRules`".
    * `revive`: "redefinition of the built-in function clear", the resolver's local function, renamed `tag`.
    * Both were fixed as above.
  * **Second run,** at load 43: exit 2.
    * `TestOverflowBlockHeldInitialize` failed once: "Initialize = {"code":-32603,…"connection ended before response"}, want
      its result". It is a synctest test in a package this phase does not change.
    * It passed 30 times alone, and with that run's seed, `-shuffle=1793756062234892000`.
    * It passed 2000 times each in the tree and in archives of `v1.31.1` and `e98987a`, with no failure.
    * It is recorded here as seen once and not reproduced. A synctest test that fails once may still hide an ordering race,
      which the owner may want examined.
  * **Third run,** at load 23: exit 0, "8 file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on,
    uncached), tidy, govulncheck."
* `tools/remote.py tree --host linux`: both modules, exit 0.
* `tools/remote.py pkg --host windows . -test.run 'TestToolCallUpdateEmptyCollectionsOnWire|TestToolCallTimeline$|
  TestToolCallTimelineCopies' -test.count=3`: "PASS".
  * A first run that named `TestToolCallTimelineReplay` failed opening `testdata\toolcalls\lifecycle.jsonl`. `pkg` sends the
    binary alone; the Linux run passes it.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0 each. `tools/idscan.py --paths` over the phase's
  files: 0 matches.

Staged for the owner, who commits. Phase 3 begins on the owner's word.

The owner committed phase 2 as `e0dd5d2`.

### Phase 3: S6, closed enumerations, 2026-11-03

Begun on the owner's "committed proceed" after `e0dd5d2`.

**Deviation 3, 2026-11-03: 25 types gain an exported `Validate`.**

* **Found** by `make api` after the first regeneration: "against v1.31.2: 0 incompatible, 25 compatible". Each type holds a
  closed enumeration's field and had nothing to check before, so the membership check gives it `Validate`:
  * `AgentCapabilities`, `ClientCapabilities`, `Annotations`;
  * the four content types (`TextContent`, `ImageContent`, `AudioContent`, `ResourceLink`) and their `ContentBlock*`
    variants;
  * `PermissionOption`, `Plan`, `PlanEntry`, `PlanItems`, `SessionUpdatePlan`, `PlanUpdateContentItems`;
  * `StringPropertySchema`, `ElicitationPropertySchemaString`;
  * six NES types: `NesCapabilities`, `NesDocumentDidChangeCapabilities`, `NesDocumentEventCapabilities`,
    `NesEventCapabilities`, `UnstableNesDiagnostic`, `UnstableNesSuggestContext`.

  Design rule 2 promised one new exported identifier.
* **Decided.** The owner was asked, with three options, and chose the recommended one: accept them.
  * 0012's treatment 7 has `Validate` check what the schema states, and these types now hold a statement to check.
  * Every type that can fail validation then has `Validate`, and each is a compatible addition in the minor.

  Not chosen: an unexported `validate` alone, reached only through a parent, which no consumer could call on a
  `PermissionOption`; and checking only types that already validated, which would leave S6 partly unfixed.
* **Scope:** design rule 2, the Scope's release line and step 6.2 are amended above. The MADR's Consequences carry it.

**What it did,** under deviation 3:

* **The model** (`internal/model/`):
  * `isClosedEnum` takes a `KindEnum` or `KindStringConstUnion` type, not a `KindOpenEnum`. `closedEnumCheck`, called from
    `ownChecks`, gives each field that holds one, by value, by pointer, or in a slice, a `CheckClosedEnum` with its shape,
    the enumeration, whether the field is required, and the enumeration's constants.
  * It marks the field `Closed`, with `ClosedValues`, and refuses one held in a patch or a map, which no check reaches.
* **The emitter** (`internal/emit/`):
  * Each closed type gains `known()`, a switch, or an `if` for a single constant (`gocritic`'s `singleCaseSwitch` found the
    first form on `ElicitationSchemaType`), and the doc paragraph `closedEnumDoc`.
  * `validate(abs func(string) bool)` becomes `validate(how validation)`. `Validate` passes
    `validation{abs: filepath.IsAbs, closed: true}`, nested calls pass `how`, and the path checks call `how.abs`.
  * `closedEnumStmt` emits each check under `if how.closed`, with `RuleEnum` and a message that quotes the value.
  * `unknownAbsent`, after each lenient field's decode, reads an unknown closed value as absent:
    * a value field as its default or `""`;
    * a pointer as nil;
    * an item of a skip-invalid-items array dropped with `slices.DeleteFunc`, and without skip-invalid-items the whole
      array nil, as Rust's `DefaultOnError` has it.
  * A file of package `unstable` declares its own `validation` (`emitValidationType`). It checks one of acp's enumerations
    with `slices.Contains` over its constants (`knownExpr`), since `known` is acp's and unexported.
* **Hand-written:**
  * `connection.go`: `validation`; `validator.validate(how validation)`; `sending`, which `validateOutbound` passes; and
    `judgeInbound` passing `validation{abs: filepath.IsAbs}`, so a received closed value is kept.
  * `errors.go`: the docs of `RuleEnum` and `ValidationError`. No identifier is added.
* **Regenerated** `types_gen.go`: 1 764 lines added and 410 changed. These are the `known` methods, the checks, the lenient
  reads, every `validate`'s signature, and the 25 `Validate` methods of deviation 3. `tools/gates.py regen` is clean.
* **Step 3.4, the SDK's own senders:**
  * The generated prompt-turn answer, `agentproc`'s `InProcess`, the examples and `tools/consumer.py`'s program send only
    listed values. Every package's tests pass, and the consumer program's stop reasons and permission kinds are constants.
  * `acptest`'s `ScriptAgent` answered a `Turn` without a `StopReason` with `""`, which is now refused. It answers
    `StopReasonEndTurn` instead, and the field's godoc says so. Its test is `TestScriptAgentDefaultStopReason`.
* **Tests changed where they tested something else,** each named here:
  * `gates_test.go`'s `validReply` gives a `PromptResponse` the stop reason `end_turn`, and `receive` passes it to the
    recording agent as it did to the client. `TestValueGates`'s prompt cases had failed with "PromptResponse: stopReason: \"\"
    is not a StopReason".
  * `conformance_test.go`'s `TestConformanceMethods` handled a zero result that cannot be encoded. It now also handles one
    that validation refuses, expecting `-32603`; `session/prompt` had failed the same way. `roundTrip` asserted the old
    signature in an interface literal, which would have skipped validation silently; it now asserts `validator`.
  * `error_text_test.go`'s three calls, and that one, pass `sending` where they passed `isPortableAbs`.
  * The two runtime stubs, in `main_test.go` and `internal/emit/types_test.go`, gain `RuleEnum` and `validation`.
* **The tests:**
  * `enum_closed_test.go`:
    * `TestClosedEnumsRefusedOnSend`: six closed positions, four stable and two optional or unstable. `""` and `"bogus"` fail
      `Validate` with `RuleEnum`, quoting the value, and a listed value passes. An agent answering `PromptResponse{}` gives
      the client `-32603`.
    * `TestClosedEnumsReadableOnReceive`: a raw agent's `"bogus"` stop reason is returned as it came.
    * `TestLenientClosedEnumsReadAbsent`:
      * `tool_call`'s and `tool_call_update`'s kind and status;
      * `audience`;
      * `ElicitationSchema.type`, taking its default `object`;
      * `positionEncoding`;
      * known values kept.
  * The model, `closed_test.go`:
    * `TestClosedEnumChecks`: each shape, an open enum unchecked, a map refused;
    * `checkCommittedClosedEnums`: the fourteen types.
  * The engine case `accept-enum-closed.txtar`: required, optional, a slice, the lenient reads, and package `unstable`'s
    type holding acp's enumeration.
  * The three engine cases whose `want.api` gained `Validate` methods: `accept-enum` (`Paint`), `accept-default-pointer-raw`
    (`Opts`) and `accept-unstable-deprecated` (`Caps`). That is the only change in their files.
* **Docs:**
  * `README.md`'s "Wire encoding": `Validate`, sending, and lenient reads.
  * `docs/architecture.md`: `validate(how validation)`, `sending`, and `known`.
  * [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md)'s "Amendment 2026-11-03, closed
    enumerations": treatments 7, 5 and 6, and the 2026-10-12 amendment's "both directions".

**Seen to fail,** each in its own scratch copy of the tree, regenerated there, by script, 5/5, and the `acptest` default by
`tools/plants.py`, 1/1:

```text
FAILED AS REQUIRED: membership not checked: rc 1: enum_closed_test.go:65: PromptResponse.stopReason "": Validate = <nil>, want a RuleEnum ValidationError quoting ""
FAILED AS REQUIRED: received values checked too: rc 1: enum_closed_test.go:100: Prompt = "", session/prompt response: PromptResponse: stopReason: "bogus" is not a StopReason; want "bogus", nil
FAILED AS REQUIRED: the lenient read removed: rc 1: enum_closed_test.go:121: tool_call kind "browse" status "queued", want both absent
FAILED AS REQUIRED: a oneOf of consts not taken as closed: rc 1: closed_test.go:52: opt: check {…Type: …Values:[]}, want {…Type:S …Values:[SX SY]}
FAILED AS REQUIRED: package unstable calls acp's known: rc 1: --- FAIL: TestEngineCases/accept-enum-closed
FAILED AS REQUIRED: ScriptAgent default removed (^TestScriptAgentDefaultStopReason$): exit 1: additions_test.go:259: a turn without a stop reason answered "", {"code":-32603,…"PromptResponse: stopReason: \"\" is not a StopReason"…}; want end_turn
```

**Checks:**

* `go test -race -count=3` of the three root tests and the `acptest` ones: `ok`.
* `go test -count=1 ./...` in both modules: `ok`.
* `tools/gates.py regen`: `ok`.
* `make api`: "against v1.31.2: 0 incompatible, 25 compatible", the 25 of deviation 3.
* `make pre-add-check` with the phase's seventeen Go files and the README:
  * **First run:** exit 2, `gocritic` `singleCaseSwitch` on `ElicitationSchemaType.known`; `emitKnown` now uses
    `switchOrIf`.
  * **Second run,** at load 24: exit 0, "17 file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on,
    uncached), tidy, govulncheck."
* `tools/remote.py tree --host linux`: both modules, exit 0.
* `tools/remote.py pkg --host windows . -test.run 'TestClosedEnumsRefusedOnSend|TestClosedEnumsReadableOnReceive|
  TestLenientClosedEnumsReadAbsent|TestValueGates' -test.count=3`: "PASS".
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0 each. `tools/idscan.py --paths` over the phase's
  files: 0 matches.

Staged for the owner, who commits. Phase 4 begins on the owner's word.

The owner committed phase 3 as `cedb98d`.

### Phase 4: the gates, S8, S9, S5 and S3, 2026-11-03

Begun on the owner's "committed, proceed" after `cedb98d`.

**Deviation 4, 2026-11-03: `unknownKind` is generated.**

* **Found,** reading the generator before the gates' code was written. Steps 4.2 and 4.3 had the generated gates read a raw
  `Unknown` entry with `unknownKind`, at `helpers.go:351`. Generated code cannot call it:
  * the engine cases build under `model.ACP` with the runtime files of `internal/enginetest/api.go:28`, which leave out
    `helpers.go`, since it uses ACP types (`engine_test.go:218`);
  * `accept-capability-gates.txtar:57` holds a `PromptRequest` whose `ContentBlock` has a raw `Unknown`, so S9's rule would
    emit the call there, and the case would not build: `undefined: unknownKind`.

  It was derived from those lines, before any gate code existed, and not run.
* **Decided.** The owner was asked, with two options, and chose the recommended one:
  * the emitter writes `unknownKind` beside `foldKeys`, in a file that uses it, as `emitDecodeHelpers` does for the other
    helpers;
  * `helpers.go:348-362` loses its copy. The body and the `"unknown"` result are the same, and the eight `Kind` methods that
    call it do not change. `helpers.go` already calls the generated `foldKeys`.

  Not chosen: the read inlined in each gate, which would copy `unknownKind`'s logic into the generated code for each rule.
  No third route left the tree correct: `unknownKind` in a runtime file fails where no `foldKeys` is emitted, and gates that
  do not read the raw entry break step 4.2's unknown-type entry and step 4.3's error.
* **Scope added:** `helpers.go:348-362` and `internal/emit/decode.go`, in the Scope above. No exported identifier.

**What it did,** under deviation 4:

* **The table** (`internal/model/acp.go`), each rule with its schema sentence:
  * **S5:** `SetSessionConfigOptionRequest`'s `boolean` variant needs `session.configOptions.boolean`, quoting "the client may
    send `session/set_config_option` requests with `type: \"boolean\"`" (`srcBooleanSet`).
  * **S8:** `AuthenticateRequest.methodId`, `GateAdvertisedAuthMethod`, after the terminal rule, quoting "Must be one of the
    methods advertised in the initialize response." (`srcAdvertisedAuth`).
  * **S9:** `PromptRequest.prompt`, `GateUndeclaredVariant` with the prefix `promptCapabilities`, quoting "Other variants must
    be explicitly opted in to." (`srcPromptOptIn`).
  * **S3:** 13 method rules, `providers/*` on `providers`, `nes/*` on `nes`, and `document/<event>` on
    `nes.events.document.<event>`, their quotes in the comment above them, since a method rule has no `Source`; and five
    variant rules, `plan_update` and `plan_removed` on the client's `plan` (`srcPlan`), and `SuggestNesResponse.suggestions`'
    `jump`, `rename` and `searchAndReplace` on `nes.jump`, `nes.rename` and `nes.searchAndReplace` (`srcNesJump`,
    `srcNesRename`, `srcNesSearchAndReplace`, one sentence each). The rules name `SuggestNesResponse`, which reaches the
    merged `UnstableSuggestNesResponse` by `ruleTypes`.
* **The model** (`internal/model/`):
  * `GateAdvertisedAuthMethod` (`protocol.go`) takes a property and no variant or capability. `advertisedAuth` (`gates.go`)
    finds the initialize response's array of a union every variant of which holds an id of the property's type, and gives
    `GateCheckAdvertisedAuth` each variant's id field (`Advertised`, `[]VariantField`), the union (`Elem`), the id's type
    (`IDType`) and, for a union with `Unknown`, the variants' one JSON key for the id (`RawKey`). `idField` is shared with
    `terminalAuth`.
  * `GateUndeclaredVariant` takes a property now. `ruleUnion`, shared with `GateVariant`, resolves the union a property
    holds, and `catchAll` takes a discriminated union's raw `Unknown` (`Variant` `"Unknown"`, `RawKey` the discriminator's
    key) besides a catch-all variant.
  * Its capability is the prefix a refusal names, so it need only exist: `capabilitySteps` is split, and
    `capabilityPath` resolves a path without requiring its last field to be a boolean or optional. `promptCapabilities` is
    a required object. Before, an undeclared rule's capability had to be one, which held for `elicitation` only by chance.
* **The emitter** (`internal/emit/`):
  * `eachUnion` runs a check over the union a gate reads, the receiver or a field by value, pointer or array; the variant
    gate and `undeclaredGate` use it, and the variant gates' output is unchanged.
  * `undeclaredGate` reads a raw `Unknown`'s value with `kind := unknownKind(x.Unknown, key)`.
  * `advertisedGate` emits `slices.ContainsFunc` over the list, one `||` term per variant and one for `Unknown`, converting
    `unknownKind`'s string to the id's type, and returns `fmt.Errorf("%s: methodId %q: %w", …, ErrUnadvertisedAuthMethod)`.
  * Deviation 4: `emitUnknownKind` (`decode.go`) emits `unknownKind` with the other decoding helpers, when a file uses it,
    marking `foldKeys`. `markGateHelpers` marks it before `emitDecodeHelpers` runs, since `typesFile` emits the gates after
    the helpers; the file's order does not change.
  * `authGates` and `authGateTypes` (`dispatch.go`) replace `terminalAuthTypes`' bool, so `Authenticate`'s doc names
    `ErrUnadvertisedAuthMethod` too.
* **Hand-written:**
  * `errors.go`: `ErrUnadvertisedAuthMethod`, "acp: methodId names no auth method the agent advertised", its godoc quoting
    the schema.
  * `helpers.go`: `unknownKind` removed (deviation 4), and `sessionUpdateUnknownKind`, which only it used.
* **Regenerated:** `types_gen.go`, 140 lines added and none removed: `unknownKind`; the 13 methods in `Allows`; the gates of
  `AuthenticateRequest`, `PromptRequest`, `SessionUpdate`, `SetSessionConfigOptionRequest` (new) and
  `UnstableSuggestNesResponse` (new). `client_gen.go`: `Authenticate`'s doc. `tools/gates.py regen` is clean.
* **Tests changed where they tested something else,** each named here:
  * `negotiation_test.go`'s `allAdvertised` advertises the unstable capabilities, and an agent auth method whose id is
    `"s"`, the string the conformance builder fills. `TestConformanceReplies`, `TestConformanceMethods` and
    `TestConformanceNotificationErrors` had failed for the 13 methods and `authenticate`, each refused as the gates now
    require.
  * `gates_test.go`: `TestMethodGates`' and `TestValueGates`' bodies became `checkMethodGate` and `checkValueGate`, unchanged,
    for the new tests; `TestAllows` takes the unstable methods too.
  * The model: `TestCapabilityRulesRefuse`'s "undeclared variant of a union without a catch-all" is accepted now and became
    two cases, a type and a property that are not unions; `TestProtocolTableAppliesToCommittedSchema` counts 28 method gates,
    and names `providers/*` and `nes/*` by the descriptions they have, "provider configuration methods" and "NES methods";
    `checkCommittedGates` counts 33 checks from 30 rules and pins `AuthenticateRequest`'s two and `PromptRequest`'s new one.
  * `internal/emit/docs_test.go`'s `TestOutboundErrors` takes `authGates`, with an authenticate row.
  * Three engine cases that define `SessionUpdate` gain its `plan_update` and `plan_removed` variants and an optional `plan`
    capability, as B24 gave them the subagent ones: `accept-capability-gates`, `accept-untyped-object-gates` and
    `refuse-gated-in-patch`. The first two's `want.api` gains the two variant types and the field, and nothing else.
    `accept-capability-gates` now emits `unknownKind`, since its `ContentBlock` keeps a raw `Unknown`.
* **The tests,** `gates_surface_test.go`:
  * `TestBooleanSetConfigOptionGated`: the rule both ways, by `checkValueGate`; a value-id request needs nothing.
  * `TestAuthenticateUnadvertisedMethod`: `x` refused with the exact text and nothing written, by `Authenticate` and
    `AllowsValue`, and answered `-32602`; `t` keeps `ErrTerminalAuthMethod`; `a` and `f`, an `Unknown` entry whose key is
    `ID`, read with folding, pass; `f` is sent; an answer with no methods refuses every id.
  * `TestPromptUndeclaredBlockGated`: a `video` block, with every capability advertised, is refused as
    `{session/prompt, promptCapabilities.video, prompt[1], video}`, writing nothing, answered `-32602`, and by
    `AllowsValue`; text and a resource link pass, advertising nothing.
  * `TestUnstableCapabilitiesGated`: the 13 methods by `checkMethodGate`, the two plan updates by `checkValueGate`, and the
    three suggestion kinds both ways, an agent answering `-32603` and a client returning the `*CapabilityError`; an `edit`
    passes with no NES kind advertised.
  * `docs_test.go`'s `TestTypedMethodDocsNameErrors` requires `Authenticate`'s doc alone to name the two auth sentinels.
  * The model: `TestCapabilityRulesApplyRawUnknown`, the raw `Unknown` on the union and through a property, with a
    required-object prefix; `TestCapabilityRulesRefuse`'s advertised-rule and undeclared-rule refusals.
  * A received NES result is not answered, so "a received variant `-32602`" of step 4.4 applies to the plan updates; a
    suggestion is refused as `TestResultGates` refuses a result.
* **Docs:**
  * `README.md`'s "Initialization and capabilities": the unstable methods and values, the undeclared prompt block, the
    boolean set request, and `ErrUnadvertisedAuthMethod`.
  * `docs/architecture.md`: the gate kinds; the counts, 28 methods and 30 rules, the rules' stale at 19 for 22 before this
    phase; `unknownKind`; and `gates_surface_test.go`.

**Seen to fail,** each in its own scratch copy of the tree, regenerated there, by script, 28/28: each of the 18 rules removed
from the table on its own, and ten breaks of the mechanism.

```text
FAILED AS REQUIRED: S5 rule removed: rc 1: gates_surface_test.go:102: send = {"code":-32603,"message":"Internal error","data":{"error":"context deadline exceeded"}}, want *CapabilityError{session/set_…
FAILED AS REQUIRED: S8 rule removed: rc 1: gates_surface_test.go:147: Authenticate(x) = {"code":-32603,"message":"Internal error","data":{"error":"context deadline exceeded"}}, want ErrUnadvertisedAut…
FAILED AS REQUIRED: S9 rule removed: rc 1: gates_surface_test.go:219: send = {"code":-32603,"message":"Internal error","data":{"error":"context deadline exceeded"}}, want *CapabilityError{session/prom…
FAILED AS REQUIRED: S3 method rule providers/list removed: rc 1: gates_surface_test.go:266: send advertising everything else: {"code":-32603,"message":"Internal error","data":{"error":"context deadlin…
FAILED AS REQUIRED: S3 method rule providers/set removed: rc 1: gates_surface_test.go:266: send advertising everything else: {"code":-32603,"message":"Internal error","data":{"error":"context deadline…
FAILED AS REQUIRED: S3 method rule providers/disable removed: rc 1: gates_surface_test.go:266: send advertising everything else: {"code":-32603,"message":"Internal error","data":{"error":"context dead…
FAILED AS REQUIRED: S3 method rule nes/start removed: rc 1: gates_surface_test.go:266: send advertising everything else: {"code":-32603,"message":"Internal error","data":{"error":"context deadline exc…
FAILED AS REQUIRED: S3 method rule nes/suggest removed: rc 1: gates_surface_test.go:266: send advertising everything else: {"code":-32603,"message":"Internal error","data":{"error":"context deadline e…
FAILED AS REQUIRED: S3 method rule nes/accept removed: rc 1: gates_surface_test.go:266: send advertising everything else: <nil>, want *CapabilityError{Method: "nes/accept", Capability: "nes"}
FAILED AS REQUIRED: S3 method rule nes/reject removed: rc 1: gates_surface_test.go:266: send advertising everything else: <nil>, want *CapabilityError{Method: "nes/reject", Capability: "nes"}
FAILED AS REQUIRED: S3 method rule nes/close removed: rc 1: gates_surface_test.go:266: send advertising everything else: {"code":-32603,"message":"Internal error","data":{"error":"context deadline exc…
FAILED AS REQUIRED: S3 method rule document/didOpen removed: rc 1: gates_surface_test.go:266: send advertising nothing: <nil>, want *CapabilityError{Method: "document/didOpen", Capability: "nes.events…
FAILED AS REQUIRED: S3 method rule document/didChange removed: rc 1: gates_surface_test.go:266: send advertising everything else: <nil>, want *CapabilityError{Method: "document/didChange", Capability:…
FAILED AS REQUIRED: S3 method rule document/didClose removed: rc 1: gates_surface_test.go:266: send advertising everything else: <nil>, want *CapabilityError{Method: "document/didClose", Capability: "…
FAILED AS REQUIRED: S3 method rule document/didSave removed: rc 1: gates_surface_test.go:266: send advertising everything else: <nil>, want *CapabilityError{Method: "document/didSave", Capability: "ne…
FAILED AS REQUIRED: S3 method rule document/didFocus removed: rc 1: gates_surface_test.go:266: send advertising everything else: <nil>, want *CapabilityError{Method: "document/didFocus", Capability: "…
FAILED AS REQUIRED: S3 variant rule plan_update removed: rc 1: gates_surface_test.go:284: send = <nil>, want *CapabilityError{session/update, plan, Field "", Variant "plan_update"}
FAILED AS REQUIRED: S3 variant rule plan_removed removed: rc 1: gates_surface_test.go:284: send = <nil>, want *CapabilityError{session/update, plan, Field "", Variant "plan_removed"}
FAILED AS REQUIRED: S3 suggestion rule jump removed: rc 1: --- FAIL: TestUnstableCapabilitiesGated/suggestion_jump (0.01s)
FAILED AS REQUIRED: S3 suggestion rule rename removed: rc 1: --- FAIL: TestUnstableCapabilitiesGated/suggestion_rename (0.00s)
FAILED AS REQUIRED: S3 suggestion rule searchAndReplace removed: rc 1: --- FAIL: TestUnstableCapabilitiesGated/suggestion_searchAndReplace (0.00s)
FAILED AS REQUIRED: the advertised gate ignores Unknown: rc 1: gates_surface_test.go:165: received f: {"code":-32602,"message":"Invalid params","data":{"error":"authenticate: methodId \"f\": acp: meth…
FAILED AS REQUIRED: catchAll refuses a raw Unknown: rc 1: gates_test.go:117: model: protocol table: capability rule Verdict: Verdict is not a union with a catch-all variant or Unknown
FAILED AS REQUIRED: an undeclared rule's capability must be a boolean or optional: rc 1: gates_test.go:117: model: protocol table: capability rule Verdict: capability "session" is neither a boolean no…
FAILED AS REQUIRED: unknownKind not marked for the gates: rc 1: <scratch>/p4plant24/types_gen.go:25404:141: undefined: unknownKind
FAILED AS REQUIRED: unknownKind returns unknown for every key: rc 1: kind_remaining_test.go:151: AuthMethod {"type":"future","id":"x","name":"X"}: decoded Kind() = "unknown", want "future"
FAILED AS REQUIRED: the authenticate doc omits ErrUnadvertisedAuthMethod: rc 1: docs_test.go:85: an authenticate request: names "ErrUnadvertisedAuthMethod": false, want true
FAILED AS REQUIRED: authGateTypes drops the advertised gate: rc 1: docs_test.go:139: client_gen.go:370:1: Authenticate's doc names ErrUnadvertisedAuthMethod: false, want true
```

* On the first run, three missed:
  * "S8 rule removed" and "S9 rule removed" failed only at the ten-minute test timeout, with a goroutine dump: with the rule
    gone, nothing refused the send, which waited on `t.Context()` for an answer that never comes. The two tests now bound the
    refused sends at two seconds, as `checkMethodGate` does, and fail at once.
  * "authGateTypes drops the advertised gate" failed nothing: no test read the generated `Authenticate` doc.
    `TestTypedMethodDocsNameErrors` now requires it to name both auth sentinels, and only it.
* After the lint fixes below, the six plants the fixes touched ran again, each failing as above: the S5 rule and the three
  suggestion rules, whose lines now name `variantBoolean` and `defSuggestNes`, and the two model plants, whose test moved to
  `TestCapabilityRulesApplyRawUnknown` and which now report `gates_test.go:120`.

**Checks:**

* `go test -count=1 ./...` in both modules: `ok`.
* `tools/gates.py regen`: `ok`.
* `make api`: "against v1.31.2: 0 incompatible, 26 compatible", `ErrUnadvertisedAuthMethod` and deviation 3's 25.
* `make pre-add-check` with the phase's 19 Go files, three engine cases and the README:
  * **First run:** exit 2. `goconst`: a third `"boolean"` in the package, the S5 rule's, now `variantBoolean`, and a third
    `"SuggestNesResponse"`, now `defSuggestNes`; it also named `model.go:483`'s `"boolean"`, which this phase did not add and
    which counted only with the new one. `gocyclo`: `TestCapabilityRulesApply` at 55; the raw-`Unknown` cases moved to
    `TestCapabilityRulesApplyRawUnknown`.
  * **Second run:** exit 2, `goconst`, a third `"suggestions"`, now `keySuggestions`.
  * **Third run:** exit 0, "19 file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on, uncached), tidy,
    govulncheck."
  * The generated files did not change with the fixes.
* `tools/remote.py tree --host linux`: both modules, exit 0.
* `tools/remote.py pkg --host windows . -test.run 'TestBooleanSetConfigOptionGated|TestAuthenticateUnadvertisedMethod|
  TestPromptUndeclaredBlockGated|TestUnstableCapabilitiesGated|TestMethodGates|TestValueGates' -test.count=3`: "PASS".
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0 each. `tools/idscan.py --paths` over the phase's
  files: 0 matches.

Staged for the owner, who commits. Phase 5 begins on the owner's word.

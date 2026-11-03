---
status: proposed
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
  * `tooltimeline.go:172-179` and `compaction.go:134-139`, the copies, with one new unexported helper file.
* **Tests:** one test per finding, under the names each step gives, in the root package and the generator's packages.
* **Docs:**
  * `README.md`: "Wire encoding", "Initialization and capabilities", and the validation and tool-call text;
  * `docs/architecture.md`;
  * an amendment to [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), treatments 5 and 7.
* **The release,** a minor `v1.32.0`, on the owner's ask. `make api` must report one compatible addition, the sentinel, and
  nothing incompatible.

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
2. **The least new API.** One exported identifier is added, the sentinel of step 4.2. It is an `Err…` variable read with
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
   * **A discriminator equal to the key-less variant's own name** decodes into that variant, as a named variant's does. A
     payload that does not decode is an error, `"<Union>: type \"<name>\": …"`. The name is the variant's schema `title`, which
     `Kind()` already returns for it (`helpers.go:439-470`). `Variant` gains `Title`, set for a key-less variant.
   * **A discriminator that names no variant, in a union without a catch-all,** decodes into the key-less variant when the
     payload has its required keys and decodes as it. Otherwise it is kept in `Unknown`, as now. Under JSON Schema's `anyOf`
     such a payload is valid as the key-less variant, which declares no discriminator and no `additionalProperties`. The
     schema's `SetSessionConfigOptionRequest` says so for its own: "Unknown `type` values with string payloads also gracefully
     deserialize into this variant". The official TypeScript and Python SDKs decode the same way (the MADR's re-assessment).
   * **A union with a catch-all,** such as `MultiSelectItems` or `IdleStateUpdate`, keeps sending an unknown value to the
     catch-all, as the schema's catch-all says.
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
   names them: `AuthMethod`, `McpServer`, `SetSessionConfigOptionRequest`, `MultiSelectItems` and `IdleStateUpdate`, the last
   two for the own name only.

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
     in `Unknown`, read with `unknownKind` (`helpers.go:351`).
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
     reading the discriminator with `unknownKind`.
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
2. `make api`: "1 compatible" (`ErrUnadvertisedAuthMethod`), 0 incompatible.
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

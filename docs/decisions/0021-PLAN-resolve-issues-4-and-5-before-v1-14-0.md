---
status: in-progress
date: 2026-10-14
associated-madr: "0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md"
---

# Implement the prompt-overlap policy option and the three-state `Patch[T]` before `v1.14.0`

Associated MADR: [0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md)

## Goal

Before `v1.14.0` is tagged ([0020-PLAN-retract-accidental-v1-tag.md](0020-PLAN-retract-accidental-v1-tag.md), step 3):

* issue #5: the 24 Go fields of the 12 patch-semantic properties are `Patch[T]`, so each receives and sends absent, `null` and a
  value, empty values included, under both JSON builds;
* issue #4: `WithPromptOverlapPolicy` lets an agent-side connection supersede (the default, unchanged), refuse, or run concurrently
  a `session/prompt` for a session whose turn is running.

Both issues are closed when the release that carries them is published.

## Scope

In scope:

* Generator, `cmd/generate`: `internal/model` (a `PatchRule` list in `Protocol` and `ACP`, its resolution, a new `TypeRef` kind for a
  patch, the gate guard), `internal/emit` (the `Patch` type and its decode helpers, field types and tags, `UnmarshalJSON`,
  `Validate`, the prompt-turn and cancel emission), the stub in `main_test.go` if a generated file names a new runtime identifier,
  new engine cases, and the tests of each package touched.
* Runtime: `connection.go` (`PromptOverlapPolicy`, its constants, `WithPromptOverlapPolicy`, the configuration field).
* Regenerated `types_gen.go`, `agent_gen.go`, `schema_gen_test.go` if it changes; root tests (`patch_test.go`, new; `cancel_test.go`;
  `fidelity_test.go` or a new file for the field cases, as the code decides).
* `README.md`, `docs/architecture.md`; the release notes,
  [0019-REPORT-release-notes-v0-18-0.md](../reports/0019-REPORT-release-notes-v0-18-0.md); these records, the 0020 PLAN's dated
  entry, and their `docs/README.md` rows.

Out of scope: `McpError.data` (0019, G3, decision 5); any other nullable field, whose schema makes `null` and omission equivalent;
the client side's prompt handling; a `Patch` helper beyond the MADR's (such as an apply-to-stored-value method), which can be added
compatibly later.

## Implementation Steps

Each step ends with its gates and an execution record; the owner commits it. Agents do not commit on `main`. Every new test is seen
to fail on a planted break in a scratch copy, never in the tree, before it is relied on.

### Step 1, records

This PLAN and its `docs/README.md` row; a dated entry in the 0020 PLAN recording that the tag waits on this PLAN, so the commit it
names, `c1b1185`, is no longer the one to tag. When the owner approves this PLAN, the 0021 MADR's status becomes `accepted`, dated,
in the same records commit as step 1's record. Gates: `make check-records`, `make markdownlint`, the identifier scan.

### Step 2, `Patch[T]` for the patch fields (issue #5)

Model, `cmd/generate/internal/model`:

* `protocol.go`: `Protocol.PatchFields []PatchRule`, `PatchRule{Definition, Property, Source string}`, documented as a property with
  patch semantics: omitted leaves the stored value unchanged, `null` clears it, a value replaces it. `checkRules` refuses a rule
  without a definition, property or source, and two rules for one property.
* `acp.go`: the 12 rules of the MADR's table, each with its schema sentence as `Source`: `CompactionUpdate` `summary`, `error`,
  `_meta`; `SessionInfoUpdate` `title`, `updatedAt`; `SubagentUpdate` `title`, `description`, `capabilities`, `state`, `_meta`;
  `SessionMessage` `content`, `_meta`.
* `fields.go`: a `TypeRef` kind `RefPatch`, whose `Elem` is the value's type, rendered `Patch[T]`; `Field.Patch` set for a
  property a rule names.
* `model.go`, a `resolvePatchFields` beside `resolveAbsolutePaths`: each rule's property, by its `*load.Definition` as `absProps`
  keys it, so every type built from the property gets the rule, the `SessionUpdate` variants included; a rule whose definition or
  property the schema lacks is skipped, one on a required property refused. A patch field's type becomes `RefPatch` of its present
  type with the pointer removed (`*string` gives `Patch[string]`, `[]ContentBlock` gives `Patch[[]ContentBlock]`, `map[string]any`
  gives `Patch[map[string]any]`), and its tag `key,omitzero`.
* `gates.go`: a patch field that holds a gated type is refused with the "shape gates does not descend into" error; none does today.

Emitter, `cmd/generate/internal/emit`:

* The `Patch[T]` type, emitted once into `types_gen.go` when a field uses it, as `orDefault` is: an unexported state and value; the
  constructors `PatchValue[T any](v T) Patch[T]` and `PatchNull[T any]() Patch[T]`; `IsZero() bool` (absent), `IsNull() bool`,
  `Value() (T, bool)`; `MarshalJSON` (`null` unless a value; absent is never marshalled, `omitzero` omits it) and `UnmarshalJSON`
  (`null` gives null, anything else decodes into a value). Documentation: the three states, that the zero value is absent, that a
  value holding a nil slice or map is sent as `null`, and the schema's own rule that `[]` also clears `summary` and `content`.
* Two decode helpers, emitted once when used: one maps a raw value to a `Patch[T]` (empty: absent; `null`: null; a value that
  decodes: that value; one that does not: absent, as `x-deserialize-default-on-error` has it), one does the same for an array
  through `skipInvalid`. The generated `UnmarshalJSON` calls them for patch fields in place of `orDefault` and `skipInvalid`.
* `Validate` reaches a nested value through `Value()`, for a single value and for each element of an array.

Regenerate: `types_gen.go` changes the 24 fields and gains `Patch`, its methods, constructors and helpers; nothing else in it changes,
which a scratch diff checks.

Tests:

* `patch_test.go`, root: `Patch` alone, for `string`, a slice, a map and a union: the three states round-trip; `""`, `[]` and `{}`
  values are sent and read back as values; the zero value is absent and omitted; `IsZero`, `IsNull` and `Value` agree; a value of
  a nil slice is sent as `null`.
* The 24 fields: for each field, a message omitting it, carrying `null`, carrying a value, and carrying a malformed value decodes to
  absent, null, the value and absent; each state encodes back to the same key or its absence. Under the default build and under
  `GOEXPERIMENT=nojsonv2` (the gate runs the root suite under both).
* `TestConformanceRoundTrip` and the JSON goldens pass unchanged or, where an instance holds `null` in a patch field, the change is
  that `null` is now kept; any golden that changes is a deviation to bring to the owner.
* Model: `TestPatchRules` (a rule applies to its definition and to a type built from it; a missing property is skipped; a required
  one is refused; checkRules' refusals); `TestProtocolTableAppliesToCommittedSchema` gains the guard: every property of the
  committed schemas whose description matches the MADR's wording, `null` with "clear", "unchanged", "patch", "retain" or "preserve"
  and not "equivalent" or "both mean", is in `PatchFields`, with the three it states the opposite for listed as exceptions, and every
  rule applies.
* Engine case `accept-patch-fields.txtar`: a definition with a patch string, array and object, its `want.api`, and a behaviour test
  of the three states both ways.
* Planted breaks: the decode helper maps `null` to absent; the tag keeps `omitempty`; `IsZero` reports null as zero; a table entry
  removed (the guard fails); the model resolves the rule on the definition's type only (the variant's field stays a pointer).

### Step 3, `WithPromptOverlapPolicy` (issue #4)

Runtime, `connection.go`: `type PromptOverlapPolicy int`; `PromptOverlapSupersede` (zero, default), `PromptOverlapRefuse`,
`PromptOverlapConcurrent`, each documented with the MADR's behaviour; `WithPromptOverlapPolicy(p PromptOverlapPolicy)
ConnectionOption`, documented as applying to an `AgentSideConnection` and ignored by the other connections, a value the package does
not define being the default; a `connectionConfig` field and an unexported `Connection` field, set by `buildConnection`.

Generator, `cmd/generate/internal/emit/dispatch.go`:

* `sessionCancels` holds a session's turns, `map[string][]*promptTurn`, each turn removing only itself.
* `promptCase`, under `a.mu`, by the connection's policy:
  * supersede: today's behaviour: a stored turn received after this prompt cancels this prompt at once, otherwise every stored turn
    is cancelled with the superseded cause and this turn alone is stored;
  * refuse: a stored turn answers this prompt at once, without calling the handler, `-32603` with `data` `{"error": "a prompt turn is
    already running for this session", "sessionId": <id>}`; otherwise this turn is stored;
  * concurrent: this turn is added;
  * in every policy, a stored turn's kept-cancel check (`lateCancels`) is as today.
* `cancelActivePrompt` cancels and removes every stored turn of the session received before the cancel, or every one when the cancel
  has no receipt; `keepCancel` is unchanged.

Regenerate: `agent_gen.go` changes in the prompt, cancel and close cases and the `sessionCancels` type; nothing else.

Amended 2026-10-14 (see "Deviation, 2026-10-14, during step 3"): the type change also rewrites one line of three engine cases'
`want.api` and the old type in `acp_test.go`, `fidelity_test.go` and `cancel_test.go`; the model gains `Method.SessionKey`.

Tests, `cancel_test.go`:

* `TestPromptOverlapRefuse`: with a turn running, a second prompt for its session is answered `-32603` with the data above and its
  handler does not run; the running turn's context is not cancelled; a prompt for another session runs.
* `TestPromptOverlapRefuseAfterCancel`: after `session/cancel` of the running turn, a prompt for the session is not refused, though
  the cancelled handler has not returned yet.
* `TestPromptOverlapConcurrent`: two prompts for one session both reach the handler; a `session/cancel` received after both cancels
  both and each is answered `cancelled`; a prompt received after that cancel is not cancelled; a kept cancel cancels a prompt
  received before it and stored after it.
* `TestPromptOverlapDefault`: no option, and a value the package does not define, behave as supersede; the existing supersede tests
  pass unchanged.
* Engine case `accept-prompt-overlap.txtar`, a behaviour test of refuse and concurrent through the generated dispatch.
* Planted breaks: refuse never refuses; refuse also refuses after a cancel (the cancel leaves the turn stored); concurrent supersedes;
  the cancel cancels only the first stored turn; an undefined value is not the default.

### Step 4, documentation and release notes

* `README.md`: "Wire encoding" gains the patch fields: the three states, `Patch[T]`'s use, the `[]` rule, and the 24 fields' types;
  "Cancellation" gains the policy option and its three behaviours.
* `docs/architecture.md`: the protocol table's `PatchFields`, the patch decode helpers and `RefPatch`; the turn set and the policy.
* The release notes: "Breaking changes" gains the 24 fields' new types, with the replacement code for setting and reading them, and
  what `null` now means on the wire both ways; "Additions" gains `Patch[T]` and `WithPromptOverlapPolicy` with its constants; the
  API listing difference in "Sources not in the release text" is recounted; issues #4 and #5 are named.

### Step 5, verification and close

The gates over the tree; the probes of the MADR re-run (issue #5's reproduction now distinguishes the three states both ways, under
both builds); the API listing against `v0.17.1`, its difference grown by exactly the changed field types and the new identifiers;
the example checks of 0019 phase 1; Windows. After CI passes on the pushed commits, this PLAN and its row become `complete`, the
owner closes issues #4 and #5 with the commits named, and the 0020 PLAN's step 3, the tag, resumes on the verified commit.

## Verification

Per code step, through the Python runner outside the tree: regenerate, and the diff of the generated files is exactly the step's;
`make pre-add-check`; `scripts/go-precheck.sh` on each new `.go` file; `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet
./...` in both modules; the generator suite under `-race`, engine behaviour cases included; each new test seen to fail on its
planted break in a scratch copy; `make check-records`, `make markdownlint`, the identifier scan; Windows `go test -count=1
-shuffle=on ./...` in both modules; the API listing against `v0.17.1`.

Expected API listing difference, added to the nine additions and seven removals of
[0019-PLAN-api-decisions.md](0019-PLAN-api-decisions.md): the struct lines of the 8 types change for their 24 fields; added: `type
Patch`, its five methods, `PatchValue` and `PatchNull`; `type PromptOverlapPolicy`, its three constants and
`WithPromptOverlapPolicy`. The exact lines are recorded in step 2's and step 3's execution records, and nothing else changes.

Acceptance: every step's checks pass and its record is written; the full suites pass under both builds and on Windows; the API
listing matches; CI passes on the pushed commits.

## Rollout and Rollback

The changes ship in `v1.14.0`, whose notes name them. Before the tag, each step is one commit, reverted by reverting it, step 3
independent of step 2. After the tag, step 2's field types cannot be reverted within `v1`; step 3's option can be deprecated but not
removed.

## Execution record

### Step 1, records, 2026-10-14

The 0021 MADR, committed by the owner after review; this PLAN, its `docs/README.md` row, and the 0020 PLAN's dated entry that the
tag waits on this PLAN, committed by the owner. The owner approved this PLAN on 2026-10-14 ("proceed"), which accepts the MADR: its
status is `accepted` from this commit. `make check-records`, `make markdownlint` and the identifier scan exit 0.

### Step 2, `Patch[T]` for the patch fields, 2026-10-14

What was built:

* Model: `Protocol.PatchFields []PatchRule` (`protocol.go`), with the table's checks (a rule needs a definition, a property and a
  source; two rules for one property are refused); the 12 rules in `acp.go`, each with its schema sentence; `RefPatch` and
  `Field.Patch` (`fields.go`); `resolvePatchFields` (`model.go`), run after `resolveAbsolutePaths` and before validation and gates,
  keyed by the schema property as `absProps` is, so the rule reaches the `SessionUpdate` variant built from each definition. A
  patch field's type becomes `Patch` of its type with the pointer removed, its tag `key,omitzero`. Beyond the PLAN's refusal of a
  required property, the model also refuses a patch property with a default and one without `x-deserialize-default-on-error`,
  since the patch decoders read a malformed value as absent; none of the 12 has either. `nested` (`validate.go`) looks through a
  patch to its value, and a nested `Check` gains `InPatch`; the gate guard (`gates.go`) refuses a patch field holding a gated type,
  and its nested calls skip patches; none holds one.
* Emitter: `typeAST` renders `Patch[T]`; a patch field always takes the lenient decode path, calling `patchOf` or, for an array with
  `x-deserialize-skip-invalid-items`, `patchItems`; `nestedStmt` (`types.go`) carries the nested checks, reached through `Value()`
  for a patch; `emitPatchType`, `emitPatchOf` and `emitPatchItems` (`decode.go`) emit the type, its constructors `PatchValue` and
  `PatchNull`, its methods `IsZero`, `IsNull`, `Value`, `MarshalJSON` and `UnmarshalJSON`, and the two helpers, once, when used.
* Regenerated `types_gen.go`: the 24 fields, their 8 decoders and 6 validators, the type and its helpers; nothing else changed.
  `TestConformanceRoundTrip`, the JSON goldens and every other test passed unchanged, so the deviation the PLAN anticipated for a
  golden did not arise.

Tests added: `patch_test.go`, root: `TestPatch` (the three states sent and read back for a string, a slice, a map and a union,
empty values included; the zero value; `IsZero`, `IsNull`, `Value`; a nil slice value sent as `null`; comparison);
`TestPatchFields` (each of the 24 fields read omitted, null, a value and a malformed value as absent, null, the value and absent,
and sent back as read); `TestPatchFieldsValidate` (an empty block in a summary value fails `Validate` naming `summary[1]`, an empty
state fails naming `state`, a null of each passes). `cmd/generate/internal/model/patch_test.go`: `TestPatchRules`, and
`checkCommittedPatchFields`, called by `TestProtocolTableAppliesToCommittedSchema`, which requires every property of the merged
committed schema whose description gives `null` a meaning omission does not have to be a rule or one of five named exceptions (the
`SessionMessage` and `SessionMessageChunk` sender and recipient, and `UnstableMcpError.data`, the merged name of `McpError`), and the
model to hold the 24 patch fields. Engine case `accept-patch-fields.txtar`: `SessionInfoUpdate` and `SessionMessage` with a patch
string, array and object and an unruled string that stays `*string`, its `want.api` written by `-update` and read, and a behaviour
test of the three states both ways and of a malformed value; it runs under `TestEngineBehaviour` and passed.

Seen to fail, each in its own scratch copy (6 of 6):

* `patch-null-read-as-absent` (`patchOf` returns absent for `null`): `TestPatchFields`, "CompactionUpdate.error null: IsZero true
  IsNull false" and "sent … without the key".
* `patch-tag-omitempty` (the tag is `omitempty`, which does not omit a struct): `TestPatchFields`, "CompactionUpdate.summary
  omitted: sent {…,"summary":null}, want the key left out".
* `patch-iszero-null` (`IsZero` true for null): `TestPatch`, "null: sent {}", and `TestPatchFields`.
* `patch-rule-removed` (the `SubagentUpdate.description` rule removed): the guard, "SubagentUpdate.description: its description
  gives null a meaning of its own, and no patch rule names it", and "22 Patch fields, want 24".
* `patch-definition-only` (the rule not applied to variants): the root package does not build, the variant's `Summary` being
  `[]ContentBlock` again; run against the model as `patch-definition-only-model`, the guard reports "12 Patch fields, want 24".

Gates, through the Python runner: regenerating changes no generated file; `make pre-add-check` exit 0, after `gofmt` aligned
`fields.go` and `patch_test.go` and `staticcheck`'s SA4000 on a comparison of two identical calls in `TestPatch` was rewritten to
compare variables; the new test files pass the same check; `GOEXPERIMENT=nojsonv2 go test ./...` exit 0; `GOOS=windows go vet` exit
0 in both modules; the generator suite under `-race`, engine behaviour cases included: ok; `make check-records` and `make
markdownlint` exit 0; the identifier scan finds nothing. The API listing against `v0.17.1`, 2411 and 2421 lines: the 24 fields'
types change, each from its `v0.17.1` type to `Patch` of it, and `Patch`, its five methods, `PatchValue` and `PatchNull` are added,
besides the 0019 PLAN's nine additions and seven removals; nothing else. Windows: `go test -count=1 -shuffle=on ./...` exit 0 at the
root and in `cmd/generate`.

### Deviation, 2026-10-14, during step 3

Found after regenerating `agent_gen.go` with the turn set: changing the type of `sessionCancels` to `map[string][]*promptTurn`, as step
3 states, reaches four places the step does not name.

* Three existing engine cases, `accept-session-close-cancels`, `accept-prompt-cancellation` and `accept-permission-cancellation`,
  print `AgentSideConnection`'s unexported fields in `want.api`; each differs from the regenerated API in that one line,
  `sessionCancels map[string]*promptTurn` becoming `map[string][]*promptTurn`, and in nothing else.
* The root package does not build: `acp_test.go` (line 444) and `fidelity_test.go` (line 197) construct an `AgentSideConnection`
  with the old map type, and `cancel_test.go`'s `stored` helper (line 550) reads one turn from the map.
* The generator model gains `Method.SessionKey`, the JSON key of the session property, set beside `SessionField` in
  `model/protocol.go` and declared in `model/methods.go`, so that the refusal's `data` names the protocol table's own key rather
  than the emitter assuming ACP's `sessionId`. For ACP the data is as step 3 states.
* `emit/dispatch_test.go` pins the old emitted text; it is a test of a package step 3 touches, and so within the scope as written.

Decision, the owner's: the three `want.api` files are rewritten with `-update` and each diff read to that one line; the three root
tests take the new type in place, `stored` finding the turn of receipt `r` anywhere in the session's turns; `Method.SessionKey` is
kept. Files added to step 3's scope: the three `want.api` files and `acp_test.go`, besides `model/methods.go` and
`model/protocol.go`, which the model's scope line covers but whose list of items did not name this.

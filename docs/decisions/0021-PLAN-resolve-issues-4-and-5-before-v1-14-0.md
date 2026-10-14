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

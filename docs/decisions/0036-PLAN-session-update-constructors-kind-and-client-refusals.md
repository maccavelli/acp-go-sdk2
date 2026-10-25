---
status: in-progress
date: 2026-10-24
associated-madr: "0036-MADR-session-update-constructors-kind-and-client-refusals.md"
---

# Implement 0036: the stable SessionUpdate constructors, SessionUpdate.Kind(), and ClientRefusals

Associated MADR: [0036-MADR-session-update-constructors-kind-and-client-refusals.md](0036-MADR-session-update-constructors-kind-and-client-refusals.md)

## Goal

Issue #11's three items ship, as the MADR's "Specification, as decided" states them:

1. the eight stable `SessionUpdate` constructors and `UpdateCompactionSummaryText`, in `helpers.go`;
2. `SessionUpdate.Kind()`, in `helpers.go`;
3. `ClientRefusals`, in `refusals.go`.

Each is tested against the schema, so that a schema bump that leaves one behind fails. `make api` against `v1.20.0` lists only
compatible changes. If the owner asks, the work is then released as `v1.21.0` for the consumer issues `<cli-b>` #5 and
magic-cli-remote #28 to pin.

This PLAN assumes the MADR's recommended answers to its questions 1 to 4. An answer otherwise amends this PLAN before the step it
changes: question 1 changes step 2, question 2 step 3, question 3 step 4, and question 4 step 6.

## Scope

* **`helpers.go`:** the nine constructors and `Kind()`.
* **`refusals.go`:** new, `ClientRefusals` and its nine methods.
* **Tests:**
  * `json_parity_test.go`: nine subtests in `TestJSONGolden_SessionUpdates`;
  * `testdata/json_golden/`: eight new golden files;
  * `session_update_helpers_test.go`: new; constructor completeness, `Validate`, round trip, and the gated sends;
  * `kind_test.go`: new;
  * `refusals_test.go`: new;
  * `example_api_test.go`: three examples.
* **Docs:** `README.md`'s helper list and its client section; `docs/architecture.md`'s hand-written list.
* **The records:** this pair and their rows in `docs/README.md`;
  [0035-REPORT-consumer-acp-code-for-the-sdk.md](../reports/0035-REPORT-consumer-acp-code-for-the-sdk.md), already aligned with
  issue #11; and, at step 6, `docs/reports/0036-REPORT-release-notes-v1-21-0.md`.
* **Not in scope:**
  * any generated file, `cmd/generate`, `HandWrittenHelpers`, or the schema;
  * the six existing `SessionUpdate` helpers, unless question 1 is answered otherwise;
  * the unstable variants' constructors;
  * `Kind()` on other unions;
  * an agent-side counterpart to `ClientRefusals`, or any working fs, terminal or elicitation implementation;
  * `example/`;
  * both consumers, whose own issues adopt the tag.

## Implementation Steps

Each step ends with its checks, an entry in the execution record below, and its files staged for the owner. The owner commits on
`main` (`AGENTS.md`, "Commits"); the agent commits only with the owner's permission in that turn. Every new check is seen to fail on
a planted break before the step closes. The breaks run through `tools/plants.py`, each in its own copy of the tree, from a list kept
in the session's scratch directory, never in the tree.

Every step's gates are `make pre-add-check`, `make check-records`, `make markdownlint`, `make api`, and `tools/idscan.py` over the
staged files. Steps 2 and 4 also run `tools/gates.py --all`, with `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` set.

### 1. Records

* This pair and their rows in `docs/README.md`, both `proposed`.
* The 0035 report's amendment, aligning it with issue #11.

The owner answers the MADR's questions: the MADR records the answers in a "Decided with the owner" table, and becomes `accepted`. On
approval ("proceed"), this PLAN becomes `in-progress`.

Checks: `make check-records`, `make markdownlint`, and `tools/idscan.py --paths` over the four files.

### 2. The constructors

In `helpers.go`, after `UpdatePlan`, in the MADR's table order:

* `UpdateAvailableCommands`, `UpdateCurrentMode`, `UpdateConfigOptions`, `UpdateSessionTitle`, `UpdateUsage`, `UpdateNotice`,
  `UpdateCompaction`, `UpdateCompactionSummary` and `UpdateCompactionSummaryText`.
* Each sets its variant's `SessionUpdate` field to the wire value (question 1). Add unexported constants for the eight values beside
  `blockTypeText`, in the style of that block.
* Each has a doc comment that opens with its name, as `TestExportedDeclarationsDocumented` and `TestExportedTypeDocsStartWithName`
  require, and names the variant's wire value.
  * `UpdateNotice`, `UpdateCompaction`, `UpdateCompactionSummary` and `UpdateCompactionSummaryText` also name the capability that
    gates them, and say that sending fails with a `*CapabilityError` when the client did not advertise it.
  * `UpdateNotice` says that an empty title fails `Validate`.
  * `UpdateSessionTitle` says what `PatchValue`, `PatchNull` and a zero `Patch` do.

Tests:

* **Goldens.** Each new subtest of `TestJSONGolden_SessionUpdates` passes two builders to `runGolden`, the composite literal and the
  constructor. The golden file is named after the subtest: `session_update_<wire value>.json`, one each for
  `available_commands_update`, `current_mode_update`, `config_option_update`, `session_info_update`, `usage_update`, `notice`,
  `compaction_update` and `compaction_summary_chunk`. `UpdateCompactionSummaryText` is a third builder on the last of these.
  * Each file is written by hand, in the style of `session_update_plan.json`, with the required keys and only those.
  * `session_info_update.json` carries `"title"`.
  * `usage_update.json` has `used` and `size` set to different numbers, so that swapping them fails.
* **`TestStableSessionUpdateConstructors`**, in `session_update_helpers_test.go`:
  * It reads `schema/schema.json`'s `SessionUpdate` variants, the `sessionUpdate` constant of each, and requires that each has a
    builder in the test's table of the fourteen. The table names the six existing helpers and the eight new ones.
  * Every builder's value must `Validate`, and must round-trip through `json.Marshal` and `json.Unmarshal` to a value
    `reflect.DeepEqual` to itself, with its field set.
  * `UpdateNotice(NoticeSeverityInfo, "")` must fail `Validate`, naming `title`.
* **`TestGatedConstructorsRefused`**, in the same file. It uses `gates_test.go`'s helpers for a pair whose client advertises neither
  `session.notices` nor `session.compaction`. `AgentSideConnection.SessionUpdate` with `UpdateNotice`, `UpdateCompaction` and
  `UpdateCompactionSummaryText` must each return a `*CapabilityError` naming its capability, and nothing may be written.
* **`ExampleUpdateSessionTitle`**, in `example_api_test.go`. It prints the JSON of `PatchValue("Refactor")` and of `PatchNull`, with
  `// Output:`.

Docs: `README.md`'s helper list gains "Session updates", which names the fourteen constructors.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| `UpdateUsage` stores `used` in `Size` and `size` in `Used` | `TestJSONGolden_SessionUpdates/session_update_usage_update` |
| `UpdateNotice` leaves `Title` empty | the golden `session_update_notice`, and `TestStableSessionUpdateConstructors` |
| `UpdateCompaction` sets the field to `"compaction_summary_chunk"` | `TestStableSessionUpdateConstructors`, the round trip |
| a copy of `schema/schema.json` gains a stable variant `"example_update"` | `TestStableSessionUpdateConstructors`, naming `example_update` |
| `UpdateNotice` sets `CompactionUpdate` instead | `TestGatedConstructorsRefused`, naming `session.notices` |

### 3. `Kind()`

In `helpers.go`, after the constructors, `func (u SessionUpdate) Kind() string`, as the MADR specifies:

* the 19 pointer fields, in the generated `MarshalJSON`'s order, each returning its literal value;
* then `Unknown`, through `json.Unmarshal` into a `map[string]json.RawMessage`, then `foldKeys(u.Unknown, m, "sessionUpdate")`,
  then the key decoded into a string. A non-empty string is returned; anything else is `"unknown"` (question 2);
* then `""`.

Its doc comment is the MADR's.

Tests, in `kind_test.go`, as `TestSessionUpdateKind`:

* **Every variant.** For each variant `schemaUnions()` lists for `SessionUpdate`, the field is set to a new zero value by
  reflection, and `Kind()` must equal the variant's `Value`. The test also requires that the variants it found are every pointer
  field of `SessionUpdate`, so a field the generated list lacks fails too.
* **Each constructor** of step 2 and the six existing helpers: `Kind()` equals the `sessionUpdate` of its own `json.Marshal` output.
* **`Unknown`:**
  * `{"sessionUpdate":"future_update"}` gives `"future_update"`;
  * `{"SessionUpdate":"folded"}` gives `"folded"`;
  * `{"sessionUpdate":"a","SESSIONUPDATE":"b"}` gives `"b"`, as decoding gives;
  * `{"sessionUpdate":""}`, `{"sessionUpdate":1}`, `{}`, `[]` and `not json` each give `"unknown"`.
* **Decoding agrees.** The four payloads whose discriminator is a string, `""` included, are decoded through `UnmarshalJSON`. Each
  lands in `Unknown` (R4) and gives the same `Kind()` as when set by hand. The others are left out: decoding refuses a non-string
  discriminator, `[]` and `not json`, and decodes `{}` into the key-less `SessionInfoUpdate`.
* **Zero and order.** The zero value gives `""`. A value with `UserMessageChunk` and `Notice` both set gives
  `"user_message_chunk"`, as `MarshalJSON` sends.

**`ExampleSessionUpdate_Kind`** decodes one known update and one unknown one, and prints both kinds.

Docs: the README item "Session updates" names `Kind()`.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| the `Notice` case removed | `TestSessionUpdateKind`, every variant, naming `notice` |
| the `AgentMessageChunk` case returns `u.AgentMessageChunk.SessionUpdate` | `TestSessionUpdateKind`, each constructor |
| `foldKeys` not called | `TestSessionUpdateKind`, `{"SessionUpdate":"folded"}` |
| `""` returned for an empty discriminator | `TestSessionUpdateKind`, `{"sessionUpdate":""}` |
| the `UserMessageChunk` case moved after `Notice` | `TestSessionUpdateKind`, zero and order |

### 4. `ClientRefusals`

`refusals.go`, new. It opens with `package acp` and carries no header, as `helpers.go` carries none:

* `type ClientRefusals struct{}`, with the MADR's doc comment;
* its nine methods, with value receivers, in the `Client` interface's order. Each request method returns the zero response and
  `NewMethodNotFound(ClientMethodFsReadTextFile)` and so on, each with its own method's constant. `CompleteElicitation` returns nil.

Tests, in `refusals_test.go`:

* **`TestClientRefusalsAnswers`.** Each of the eight request methods, called directly, returns a `*RequestError` with code `-32601`
  and `Data` equal to `map[string]any{"method": <its ClientMethod* constant>}`. `CompleteElicitation` returns nil.
* **`TestClientRefusalsMethodSet`.** By reflection, the method set of `ClientRefusals` is exactly the nine names. `ClientRefusals`
  does not implement `Client`. A test type that embeds it and implements `SessionUpdate` and `RequestPermission` does, asserted with
  `var _ Client = refusingClient{}`.
* **`TestClientRefusalsOverTheWire`.** An agent and a client are paired over `io.Pipe`, as `gates_test.go`'s helpers pair them. The
  client advertises `fs.readTextFile` and embeds `ClientRefusals`. The agent's `ReadTextFile` must return a `*RequestError` with
  code `-32601` and data naming `fs/read_text_file`. Both connections are closed in cleanup, so `leak_test.go` passes.
* **`ExampleClientRefusals`**, a client that embeds it.

Docs:

* `README.md`'s client section: one sentence naming `ClientRefusals`, and when not to embed it;
* `docs/architecture.md`'s "Hand-written" list names `refusals.go`, and says that `helpers.go` holds `Kind()`.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| `WriteTextFile` answers with `ClientMethodFsReadTextFile` | `TestClientRefusalsAnswers`, `WriteTextFile` |
| `CompleteElicitation` returns `NewMethodNotFound(…)` | `TestClientRefusalsAnswers`, `CompleteElicitation` |
| a `SessionUpdate` method added to `ClientRefusals` | `TestClientRefusalsMethodSet` |
| `ReadTextFile` returns `ReadTextFileResponse{Content: "x"}, nil` | `TestClientRefusalsOverTheWire` |

### 5. Close the implementation

* `make api` against `v1.20.0`: 0 incompatible, and the compatible changes listed. There should be the nine functions, `Kind`,
  `ClientRefusals` and its nine methods; record what it actually lists.
* `tools/gates.py --all`.
* The README and architecture text read once more against the code.
* This PLAN's execution record complete.
* Issue #11's acceptance list walked item by item, with the check that meets each one, in the execution record.

The owner commits. A push needs the owner's ask in that turn: before it, `git fetch`, `make push-guard`, and
`tools/idscan.py --range origin/main..main`. After it, `tools/watch_ci.py <commit>`.

### 6. Release `v1.21.0`, only on the owner's ask in that turn

Through `RELEASING.md`, "Tag and Publish":

* `docs/reports/0036-REPORT-release-notes-v1-21-0.md` and its row, with its "Release text" and `make api`'s output on the release
  commit;
* `tools/release_body.py v1.21.0 <commit>`;
* the annotated tag and its push;
* the GitHub release;
* `tools/publication.py`;
* `tools/consumer.py v1.21.0`, a module at the floor that calls `UpdateNotice`, `Kind()` and embeds `ClientRefusals`.

Commenting on issue #11 and closing it, and telling the consumer issues, are outward-facing, and need the owner's ask too. When the
release is published, this PLAN and its row are `complete`.

## Verification

Acceptance, issue #11's list as this PLAN meets it:

* Every stable `SessionUpdate` variant not built by `helpers.go` before has a constructor, `UpdateSessionTitle` included; `go doc`
  and the README name them. Checked by `TestStableSessionUpdateConstructors`, and by `go doc . UpdateNotice` and the others.
* Each helper's encoding equals the composite literal's, and each validates: the goldens and `TestStableSessionUpdateConstructors`.
* `Kind()` does what the MADR's comment says, and magic-cli-remote's `Unknown` decoding becomes `u.Kind()`: `TestSessionUpdateKind`.
* A client that embeds `ClientRefusals` and implements only `SessionUpdate` and `RequestPermission` satisfies `Client`; each refused
  method's code is `-32601` and its data names the method; `CompleteElicitation` returns nil: the three `ClientRefusals` tests.
* `apidiff` against `v1.20.0` lists only additions: `make api`.
* The generator does not emit `SessionUpdate` constructors, and `HandWrittenHelpers` is unchanged: `git diff --stat` touches no file
  under `cmd/generate` and no `*_gen.go`; `TestGeneratedFilesUpToDate` passes.
* The root's `go test ./...` and `-race`, the `cmd/generate` tests, and `make pre-add-check` pass on every changed `.go` file.
* No identifier is committed: `tools/idscan.py`.

Every planted break listed in steps 2 to 4 fails its check, each in its own copy of the tree. The execution record keeps each one's
failing line.

## Rollout and Rollback

* **Rollout.** Steps 2, 3 and 4 are one commit each, and each stands without the later ones. Nothing is released until step 6, so
  no consumer sees the change before the tag. Each consumer adopts the tag under its own issue and records.
* **Rollback before the tag:** revert the step's commit.
* **Rollback after the tag:** removing an exported name would break `v1`, so a defect is fixed forward in a patch release. A tag
  found broken is retracted, as
  [0020-MADR-retract-accidental-v1-tag.md](0020-MADR-retract-accidental-v1-tag.md) did, with a `retract` directive and a new patch.

## Execution record

### Step 1, records, 2026-10-24

* **The pair** was written from a read-only investigation at `7581822`, every finding in the MADR's R1 to R10.
* **The 0035 report** was aligned with issue #11, in its C1, A4 and B3, its table and "What would follow", with a dated note at its
  head.
* **The owner's answers:** "take recommendations and proceed". Questions 1 to 4 were answered as recommended; the MADR records them in
  "Decided with the owner, 2026-10-24" and is `accepted`. This PLAN is approved, and `in-progress`.
* **Checks,** each exit 0:
  * `make check-records`;
  * `make markdownlint`, 30 files, 0 issues;
  * `tools/idscan.py --paths` over the four files, "4 file(s), 17 pattern(s), 0 match(es)".

---
status: complete
date: 2026-10-25
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
compatible changes. If the owner asks, the work is then released as `v1.21.0` for the consumer issues gobble-cli [#5](https://github.com/maccavelli/gobble-cli/issues/5) and
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
* **Tools,** added 2026-10-25 at step 6: `tools/consumer.py` and its row in `tools/README.md`.
* **Not in scope:**
  * any generated file, `cmd/generate`, `HandWrittenHelpers`, or the schema; *amended 2026-10-25, see step 4's deviation:
    `cmd/generate/engine_test.go`'s `TestRuntimeFilesCoverHandWritten` excludes `refusals.go`, and nothing else under
    `cmd/generate` changes;*
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
    `reflect.DeepEqual` to itself, with its field set. *Amended 2026-10-24, see the step's deviation: the eight new constructors
    round-trip to an equal value; the six existing helpers, which leave the field empty by question 1, round-trip to the same JSON.*
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

*Added 2026-10-25, see the step's deviation:* `cmd/generate/engine_test.go`'s `TestRuntimeFilesCoverHandWritten` excludes
`refusals.go` beside `helpers.go`, and its comment says so.

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

*Amended 2026-10-25, see step 6's deviations:* the owner pushed the tag before the report was written, so the report is committed
after the tag; and `tools/consumer.py` and its row in `tools/README.md` are extended to build that module, since the tool's
program did not use the three.

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
  under `cmd/generate` and no `*_gen.go`; `TestGeneratedFilesUpToDate` passes. *Amended 2026-10-25: under `cmd/generate`, only
  `engine_test.go`, step 4's deviation.*
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
* The four files are staged for the owner.

### Step 2, the constructors, 2026-10-24

What changed:

* **`helpers.go`:**
  * eight unexported constants, `sessionUpdateAvailableCommands` to `sessionUpdateCompactionSummary`, in the block beside
    `blockTypeText`;
  * after `UpdatePlan`, `UpdateAvailableCommands`, `UpdateCurrentMode`, `UpdateConfigOptions`, `UpdateSessionTitle`, `UpdateUsage`,
    `UpdateNotice`, `UpdateCompaction`, `UpdateCompactionSummary` and `UpdateCompactionSummaryText`. Each sets its variant's
    `SessionUpdate` field (question 1), and each doc comment says what the step requires.
* **`testdata/json_golden/`:** `session_update_available_commands_update.json`, `_current_mode_update`, `_config_option_update`,
  `_session_info_update`, `_usage_update` (`used` 53000, `size` 200000), `_notice`, `_compaction_update` and
  `_compaction_summary_chunk`, each written by hand.
* **`json_parity_test.go`:** eight subtests in `TestJSONGolden_SessionUpdates`, each with the composite literal and the constructor;
  the last has `UpdateCompactionSummaryText` as a third builder.
* **`session_update_helpers_test.go`, new:**
  * `TestStableSessionUpdateConstructors`, over the 14 variants that `schema/schema.json` lists;
  * `TestGatedConstructorsRefused`.
* **`example_api_test.go`:** `ExampleUpdateSessionTitle`.
* **`README.md`:** the helper list's "Session updates" item.

**Deviation, 2026-10-24.** The step required every builder's value to round-trip to a `reflect.DeepEqual` copy of itself, "with its
field set". The six existing helpers leave the variant's `SessionUpdate` field empty, as question 1 decided, and a decoded value has it
set, so for them that requirement cannot hold. The test requires it of the eight new constructors. For the six, it requires that the
decoded value re-encodes to the same JSON. The step's text is annotated. The MADR is unchanged: it decides the six stay as they are.

**Tests,** `go test -count=1 -v` on the step's tests, exit 0:

* the 8 new golden subtests among `TestJSONGolden_SessionUpdates`' 19;
* `TestStableSessionUpdateConstructors`, `TestGatedConstructorsRefused` and `ExampleUpdateSessionTitle`;
* `TestExportedDeclarationsDocumented` and `TestExportedTypeDocsStartWithName`.

The first build failed, exit 1: "cannot call pointer method Validate on SessionUpdate". The test now calls `Validate` through a
variable.

**Planted breaks,** `tools/plants.py`, each in its own copy of the tree, the list in the session's scratch directory. Six of six
failed as required:

| Break | The check that failed |
| :--- | :--- |
| `UpdateUsage` swaps `used` and `size` | "json_parity_test.go:261: session_update_usage_update marshal mismatch" |
| `UpdateNotice` leaves `Title` empty | "json_parity_test.go:267: session_update_notice marshal mismatch" |
| the same | "session_update_helpers_test.go:90: notice: Validate: SessionUpdate: variant Notice: SessionUpdateNotice: title: shorter than minLength 1" |
| `UpdateCompaction` writes `compaction_summary_chunk` | "session_update_helpers_test.go:108: compaction_update: round trip differs" |
| a stable variant `example_update` added to `schema/schema.json` | "session_update_helpers_test.go:85: stable variant \"example_update\" has no constructor" |
| `UpdateNotice` returns a `CompactionUpdate` | "session_update_helpers_test.go:145: notice: send = session/update: capability session.compaction not advertised: variant compaction_update, want *CapabilityError{session/update, session.notices, Variant \"notice\"}" |

The last break was first written as a `return` followed by an unreachable statement, which did not compile ("helpers.go:227:1:
missing return"), so it proved nothing. It was rewritten to return the other variant behind `if title != ""`, and failed as required.

**Gates,** `tools/gates.py --all`, with `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` set, every gate exit 0: "all gates passed", 18 of 18.

* `precheck`: "176 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)".
* `newfiles`: `session_update_helpers_test.go`, "1 file(s) clean".
* `api`: "against v1.20.0: 0 incompatible, 9 compatible". The nine are the nine constructors, each "added".
* `idscan`: "17 file(s), 17 pattern(s), 0 match(es)".
* `floor`: passed at go1.26.10.
* `nojsonv2`, `windows` and `linux`: passed.

Step 1 was committed by the owner as `f171b3a`. Step 2's files are staged for the owner, without the docs index change and the 0037
records that appeared in the tree meanwhile, which are not this PLAN's.

### Step 3, `Kind()`, 2026-10-25

What changed:

* **`helpers.go`:**
  * `SessionUpdate.Kind()`, after the constructors, with the MADR's doc comment. It switches on the 19 pointer fields in
    `MarshalJSON`'s order and never reads a variant's `SessionUpdate` field.
  * `unknownSessionUpdateKind`, which reads `Unknown` as `UnmarshalJSON` does: an object, `foldKeys(b, m, "sessionUpdate")`, and the
    key decoded into a string.
  * Twelve more unexported constants in the block beside `blockTypeText`, for the eleven values step 2 did not name and for
    `"unknown"`.
* **`kind_test.go`, new:** `TestSessionUpdateKind`, with the step's five subtests, each a function of its own.
* **`example_api_test.go`:** `ExampleSessionUpdate_Kind`, which prints `current_mode_update` and `future_update`.
* **`README.md`:** the "Session updates" item names `SessionUpdate.Kind()`.

The step says each case returns "its literal value". The first `make pre-add-check` failed, exit 2, on `goconst` against the string
literals, eight of which step 2 had already named ("string `notice` has 4 occurrences, but such constant `sessionUpdateNotice`
already exists"), and on `modernize` against a `NumField` loop in the test. Each case now returns a named constant holding that
literal, which keeps what the step requires: a fixed value, never the variant's field. The second run failed on `gocognit`,
"cognitive complexity 61 of func `TestSessionUpdateKind` is high (> 50)", and the subtests were split into functions. Neither rule
was loosened.

**Tests,** `go test -count=1 -v`, exit 0: `TestSessionUpdateKind` and its five subtests, and `ExampleSessionUpdate_Kind`. Decoding
put each of the four string-discriminator payloads, `{"sessionUpdate":""}` included, whole into `Unknown`, as R4 says.

**Planted breaks,** `tools/plants.py`, each in its own copy of the tree, the list in the session's scratch directory, run against the
final code. Five of five failed as required:

| Break | The check that failed |
| :--- | :--- |
| the `Notice` case removed | "kind_test.go:76: Notice: Kind() = \"\", want \"notice\"" |
| the `AgentMessageChunk` case returns `u.AgentMessageChunk.SessionUpdate` | "kind_test.go:91: agent_message_chunk: Kind() = \"\", sent \"agent_message_chunk\"" |
| `foldKeys` not called | "kind_test.go:100: {\"SessionUpdate\":\"folded\"}: Kind() = \"unknown\", want \"folded\"" |
| `""` returned for an empty discriminator | "kind_test.go:100: {\"sessionUpdate\":\"\"}: Kind() = \"\", want \"unknown\"" |
| the `UserMessageChunk` case moved after `Notice` | "kind_test.go:135: two set: Kind() = \"notice\", sent \"user_message_chunk\", want \"user_message_chunk\"" |

**Gates,** each exit 0:

* `make pre-add-check`: "177 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)";
* `make check-records`;
* `make markdownlint`: 30 files, 0 issues;
* `make api`: "against v1.20.0: 0 incompatible, 10 compatible", step 2's nine and `SessionUpdate.Kind`, each "added";
* `tools/idscan.py --paths` over the four files: "4 file(s), 17 pattern(s), 0 match(es)".

Step 2 was committed by the owner as `4189d9a`. Step 3's files are staged for the owner.

### Step 4, `ClientRefusals`, 2026-10-25

What changed:

* **`refusals.go`, new:** `ClientRefusals` and its nine value-receiver methods, in the `Client` interface's order. Each request
  method returns its zero response and `NewMethodNotFound` with its own `ClientMethod*` constant; `CompleteElicitation` returns nil.
  The doc comment says the three things the MADR requires.
* **`refusals_test.go`, new:** `TestClientRefusalsAnswers`, `TestClientRefusalsMethodSet`, with `var _ Client = refusingClient{}`,
  and `TestClientRefusalsOverTheWire`, whose agent and client are paired over `io.Pipe` and closed in cleanup.
* **`example_api_test.go`:** `ExampleClientRefusals`, which prints the answer to a file read,
  `{"code":-32601,"message":"Method not found","data":{"method":"fs/read_text_file"}}`.
* **`README.md`:** the client section's first item names `acp.ClientRefusals`, and says that a client implementing some of its
  methods implements all of them instead.
* **`docs/architecture.md`:** the "Hand-written" list names `refusals.go`, and `SessionUpdate.Kind` in `helpers.go`.
* **`cmd/generate/engine_test.go`:** the deviation below.

**Deviation, 2026-10-25.** The first `tools/gates.py --all` failed three gates, `floor`, `windows` and `linux`, each on
`cmd/generate`'s `TestRuntimeFilesCoverHandWritten`: "engine_test.go:163: hand-written files of the root are [connection.go doc.go
errors.go extensions.go helpers.go refusals.go], want [connection.go doc.go errors.go extensions.go helpers.go]: add a new file to
enginetest.RuntimeFiles if it uses no ACP type, or exclude it here as helpers.go is". The failure was not there before: `refusals.go`
caused it. `make pre-add-check` does not run the `cmd/generate` tests, so it passed. This PLAN kept `cmd/generate` out of scope.
Two resolutions were put to the owner: exclude `refusals.go` in that test, or move `ClientRefusals` into `helpers.go`, which would
reverse question 3. The owner chose "option 1". The test's exclusions now name `refusals.go` beside `helpers.go`, and its comment says
so. The scope and the acceptance item on `cmd/generate` are annotated, and the MADR has "Amendment, 2026-10-25".

**Tests,** `go test -count=1 -race -v`, exit 0: the three `ClientRefusals` tests, `ExampleClientRefusals`,
`TestExportedDeclarationsDocumented` and `TestExportedTypeDocsStartWithName`. In `cmd/generate`, `TestRuntimeFilesCoverHandWritten`
passes.

**Planted breaks,** five of five failed as required. The first four ran through `tools/plants.py`, each in its own copy of the tree,
the list in the session's scratch directory:

| Break | The check that failed |
| :--- | :--- |
| `WriteTextFile` answers with `ClientMethodFsReadTextFile` | "refusals_test.go:58: WriteTextFile: {\"code\":-32601,\"message\":\"Method not found\",\"data\":{\"method\":\"fs/read_text_file\"}}, want -32601 naming fs/write_text_file" |
| `CompleteElicitation` returns `NewMethodNotFound(…)` | "refusals_test.go:62: CompleteElicitation: {\"code\":-32601,\"message\":\"Method not found\",\"data\":{\"method\":\"elicitation/complete\"}}, want nil" |
| a `SessionUpdate` method added to `ClientRefusals` | "refusals_test.go:79: methods = [CompleteElicitation CreateElicitation CreateTerminal KillTerminal ReadTextFile ReleaseTerminal SessionUpdate TerminalOutput WaitForTerminalExit WriteTextFile], want [...]" |
| `ReadTextFile` returns `ReadTextFileResponse{Content: "x"}, nil` | "refusals_test.go:105: ReadTextFile: <nil>, want a -32601 *RequestError" |
| `TestRuntimeFilesCoverHandWritten`'s exclusion of `refusals.go` removed | "engine_test.go:163: hand-written files of the root are [connection.go doc.go errors.go extensions.go helpers.go refusals.go], want [connection.go doc.go errors.go extensions.go helpers.go]: ..." |

For the third break, the first run's expected text was "ClientRefusals implements Client by itself". The test failed, at the method
set, but that text was not printed: with `SessionUpdate` alone, `ClientRefusals` still lacks `RequestPermission`, so it is no
`Client`. The expected text was corrected to the method list, and the break failed as required. `tools/plants.py` runs `go test`
from the copy's root, which cannot reach the nested `cmd/generate` module. So the fifth break ran from a script in the session's
scratch directory, which copies the tree, makes the same edit, and runs the test in `cmd/generate`.

**Gates,** `tools/gates.py --all`, with `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` set, after the deviation's fix, every gate exit 0:
"all gates passed", 18 of 18.

* `precheck`: "178 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)".
* `newfiles`: `refusals.go` and `refusals_test.go`, "2 file(s) clean".
* `api`: "against v1.20.0: 0 incompatible, 11 compatible": `ClientRefusals`, `SessionUpdate.Kind` and step 2's nine, each "added".
  `apidiff` lists a new type once, without its methods.
* `idscan`: "13 file(s), 17 pattern(s), 0 match(es)".
* `floor`: passed at go1.26.10, `cmd/generate` included.
* `nojsonv2`, `windows` and `linux`: passed.

Step 3 was committed by the owner as `e0f4854`. Step 4 was committed by the owner as `4152eb7`.

### Step 5, close the implementation, 2026-10-25

Over the commits since `7581822` and step 4's staged files:

* **`make api`:** "against v1.20.0: 0 incompatible, 11 compatible", step 4's gate run. The step expected the nine functions,
  `Kind`, `ClientRefusals` and its nine methods; `apidiff` lists `ClientRefusals` once, as "added", and not its methods.
* **`tools/gates.py --all`:** 18 of 18, step 4's run, on the code staged now.
* **The README and `docs/architecture.md`,** read again against the code: the "Session updates" item names the fourteen
  constructors and `SessionUpdate.Kind()`; the client section names `acp.ClientRefusals`; the "Hand-written" list names
  `refusals.go` and `SessionUpdate.Kind`.
* **The execution record** holds steps 1 to 4, each with its tests, planted breaks, gates and deviations.

Issue #11's "Acceptance", item by item:

| Item | Met by |
| :--- | :--- |
| Every stable variant not covered before has an `Update*` constructor, `UpdateSessionTitle` included; `go/doc` and the README name them | `TestStableSessionUpdateConstructors`, over `schema/schema.json`'s 14; a script over `go doc -all .` and `README.md` found all nine constructors, `Kind` and `ClientRefusals` in each |
| Each helper's encoding equals the composite literal; `Validate` passes | the eight new subtests of `TestJSONGolden_SessionUpdates`, and `TestStableSessionUpdateConstructors` |
| `Kind()` matches the issue's comment; decoding an unknown discriminator becomes `u.Kind()` | `TestSessionUpdateKind`, and `ExampleSessionUpdate_Kind`, which decodes `future_update` and prints `u.Kind()`. One difference, decided as the MADR's question 2: an `Unknown` whose `sessionUpdate` is `""` gives `"unknown"`, where the issue's comment would give `""` |
| Embedding `ClientRefusals` and implementing only `SessionUpdate` and `RequestPermission` satisfies `Client`; each refused method answers `-32601` naming itself; `CompleteElicitation` returns nil | `var _ Client = refusingClient{}`, `TestClientRefusalsAnswers`, `TestClientRefusalsMethodSet` and `TestClientRefusalsOverTheWire` |
| `apidiff` against `v1.20.0` is additions only | `make api`, above |
| The generator does not emit `SessionUpdate` constructors; `HandWrittenHelpers` unchanged | no `*_gen.go` among the 22 files changed since `7581822`; no change to `cmd/generate/internal/model/acp.go`; under `cmd/generate`, only `engine_test.go`, step 4's deviation; the `regen` gate passed |
| Root `go test ./...` and `-race`; `cmd/generate` tests; `make pre-add-check` on changed `.go` files | the `precheck`, `newfiles`, `race3` and `floor` gates, and the `windows` and `linux` gates, which run both modules |
| No identifier in the commits | `tools/idscan.py --range 7581822..HEAD`, "18 file(s), 17 pattern(s), 0 match(es)", and `--paths` over the staged files, "8 file(s), 17 pattern(s), 0 match(es)" |

Not done in this step: the push and its checks (`git fetch`, `make push-guard`, `tools/idscan.py --range origin/main..main`,
`tools/watch_ci.py`), which wait for the owner's ask, and step 6. This PLAN stays `in-progress` until the release is published.

Step 5's record was committed by the owner as `2452f31`. The owner committed the 0037 records as `7f3b210`, and pushed the six
commits. The agent did not run `make push-guard` or the range scan before that push, which the owner made. After it, `tools/watch_ci.py 7f3b210`: run `40192665703`, "completed success", six jobs, on Ubuntu, Windows and macOS at Go
`1.27.3` and `1.26.10`.

### Step 6, release `v1.21.0`, 2026-10-25

The owner pushed the annotated tag `v1.21.0`, message `v1.21.0`, at `7f3b210`, the commit CI passed (`git cat-file -t v1.21.0`:
"tag"). No GitHub release existed (`gh release view v1.21.0`: "release not found").

**Deviation 1, 2026-10-25: the report after the tag.** The step writes the release-notes report before the tag, and `v1.20.0`'s
tagged tree holds its report (`4fa7204`). The tag was pushed first, so `v1.21.0`'s tree holds no report. Moving a published tag was
not offered. Two resolutions were put to the owner, and the owner took the first: the report describes `7f3b210`, says it was written
after the tag, and is committed on `main` after it; `tools/release_body.py` reads it from that later commit.

**Deviation 2, 2026-10-25: `tools/consumer.py` extended.** The step's consumer calls `UpdateNotice` and `Kind()` and embeds
`ClientRefusals`. The tool's program used none of them (`tools/consumer.py:43`, an `agentproc` program with an `OutputTap`). Of two
resolutions, running the tool unchanged with a separate module in the session's scratch directory, or extending the tool, the owner
chose the second. The scope above gains `tools/consumer.py` and its row in `tools/README.md`.

* **`tools/consumer.py`:** the program's client embeds `acp.ClientRefusals` in place of `acp.Client` and implements
  `SessionUpdate` and `RequestPermission`, so passing it to `agentproc`'s `Start` asserts at compile time that the two and the embed
  satisfy `Client`. It prints `UpdateNotice(acp.NoticeSeverityInfo, "t").Kind()` and the answer to `ReadTextFile`, and the expected
  output gains "kind notice" and "refused {"code":-32601,...,"data":{"method":"fs/read_text_file"}}". Its docstring says that the
  version is `v1.21.0` or later.
* **Found and fixed in the same file:** after a failed build, the tool went on to run `./consumer`, which did not exist, and ended
  with a traceback ("FileNotFoundError: [Errno 2] No such file or directory: './consumer'"), exit 1, without its verdict. It was so
  before this change, for any failed build. It now stops at the first failed command and prints "NOT AS REQUIRED".
* **`tools/README.md`:** the tool's row names what the module uses, and `v1.21.0` or later.

Checks:

* `tools/consumer.py v1.21.0`, exit 0, "as required": `go version go1.26.10`, `go get` of `v1.21.0` from the proxy, the run's output
  "Close: <nil>", "dropped 0 0", "protocol version 1", "kind notice", "refused {"code":-32601,"message":"Method not
  found","data":{"method":"fs/read_text_file"}}", the Windows build, and a `go.mod` with `go 1.26.10` and no toolchain line.
* **Planted breaks,** each failed as required:
  * the tool against `v1.20.0`, exit 1, "NOT AS REQUIRED": "./main.go:13:25: undefined: acp.ClientRefusals", and likewise
    `acp.UpdateNotice` and `ReadTextFile`;
  * in a scratch copy of the tree, the expected "kind notice" changed to "kind plan", against `v1.21.0`: exit 1, "NOT AS REQUIRED".
* `make check-tools`: "Ran 53 tests", "OK".

**The report,** `docs/reports/0036-REPORT-release-notes-v1-21-0.md`, and its row in `docs/README.md`. `make api` on `7f3b210`, with
nothing uncommitted, `scripts/check_api.py v1.20.0`: exit 0, "against v1.20.0: 0 incompatible, 11 compatible".

The tool change was committed by the owner as `9fc22d9`.

The report was committed by the owner as `7c53ccd`.

* **`tools/release_body.py v1.21.0 7c53ccd`:** exit 0, "88 lines, 5 links". Each link names a file under `blob/v1.21.0/`, and
  each of the four files exists at the tag (`git cat-file -e`).
* **The GitHub release.** The owner asked for it ("committed, proceed"). The agent's `gh release create v1.21.0 --verify-tag --title
  v1.21.0 --notes-file <the body>` was refused by the agent's permission classifier, as a public action. The owner ran the same
  command, and it created `https://github.com/maccavelli/acp-go-sdk2/releases/tag/v1.21.0`.
* **`tools/publication.py v1.21.0 7f3b210 <the body>`:** exit 0, "10 of 10 checks hold":
  * the release exists, not a draft or prerelease; its body is the extracted release text; it is marked latest;
  * the tag is annotated and names `7f3b210`, which is on origin's `main`;
  * CI on origin `main`'s head, run `40192665703`, and on the tag, run `40193217494`;
  * the proxy serves the `.info`, with origin the tag at that commit; `go list -m @latest` gives `v1.21.0`; `go mod download` of the
    tag passes with the checksum database on.

The release is published, so this PLAN and its row are `complete`.

**The issues,** on the owner's ask ("comment on and close #11, and post to the two consumer issues"), each `gh` call exit 0:

* issue #11: a comment naming the release, the three items with their commits, `make api`'s result, step 5's walk of the acceptance
  list, and the difference question 2 decided; then closed as completed;
* gobble-cli [#5](https://github.com/maccavelli/gobble-cli/issues/5) and magic-cli-remote #28: a comment each, naming the release and what it holds for that consumer, with
  `go get github.com/maccavelli/acp-go-sdk2@v1.21.0`. The comment on magic-cli-remote #28 repeats issue #11's rule that that client
  does not embed `ClientRefusals`. Both issues stay open, for their own repositories to close.

**The last push.** The owner committed this record as `9b20468`, after `9fc22d9` and `7c53ccd`. Before the push: `git fetch`, `main`
three ahead of `origin/main` and none behind; `make push-guard`, "3 outgoing commit(s), 7f3b210..9b20468, main to origin", "the
guard passed (exit 0)"; `tools/idscan.py --range origin/main..main`, "5 file(s), 17 pattern(s), 0 match(es)". The owner pushed.
After it, `tools/watch_ci.py 9b20468 --branch main`: run `40195337860`, "completed success", six jobs, on Ubuntu, Windows and macOS
at Go `1.27.3` and `1.26.10`.

*Note, 2026-10-31: gobble-cli was written `<cli-b>` in this record until the owner lifted that redaction ([0044-PLAN-notification-queue-back-pressure.md](0044-PLAN-notification-queue-back-pressure.md), phase 5).*

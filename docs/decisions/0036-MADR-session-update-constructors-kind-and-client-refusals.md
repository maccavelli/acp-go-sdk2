---
status: accepted
date: 2026-10-24
decision-makers: Repository owner
---

# Complete the stable SessionUpdate constructors, add SessionUpdate.Kind(), and a ClientRefusals embed

## Context and Problem Statement

Issue #11, opened by the owner, asks for three things:

* an `Update*` constructor for every stable `SessionUpdate` variant that `helpers.go` does not build;
* an exported accessor for a `SessionUpdate`'s discriminator;
* an embeddable type that answers method-not-found for the `Client` methods a host may leave unadvertised.

Both consumers have an issue that pins the tag shipping them: `<cli-b>` issue #5 for its agent's send path and its client's stubs,
and magic-cli-remote issue #28 for its client's receive path and its tests.
[0035-REPORT-consumer-acp-code-for-the-sdk.md](../reports/0035-REPORT-consumer-acp-code-for-the-sdk.md) found the same three gaps
in the consumers' code, as its C1, A4 and B3.

The issue gives its direction as decided: "These are the issue's answers, not open questions", and "The pair records the direction
above; it does not re-open it." That direction is:

1. `ClientRefusals` ships in this issue, with the constructors and `Kind()`.
2. Only the stable variants get constructors. The unstable ones are a later issue.
3. The accessor is `Kind()`.
4. `session_info_update` is `UpdateSessionTitle(title Patch[string])`.

This record records that direction, fixes what the issue leaves to the implementation, and asks the owner the four questions it
cannot settle from the issue or the code.

Every finding below was read on 2026-10-24 at `7581822`, the tree only read, against the newest tag, `v1.20.0`.

### What was found

* **R1. What `helpers.go` builds.** It builds six of `SessionUpdate`'s variants:
  * `UpdateUserMessage` / `Text`, `UpdateAgentMessage` / `Text`, `UpdateAgentThought` / `Text` and `UpdatePlan`, at
    `helpers.go:123`–`:156`;
  * `StartToolCall`, `UpdateToolCall` and their options, `StartReadToolCall` and `StartEditToolCall`, at `:158`–`:337`.

  `HandWrittenHelpers` (`cmd/generate/internal/model/acp.go:117`) names `ContentBlock`, `SessionUpdate` and `ToolCallContent`, so
  `helpers_gen.go` writes no `SessionUpdate` constructor.
* **R2. The stable variants left.** `schema/schema.json` (schema 1.25.0) lists 14 variants of `SessionUpdate`; R1's helpers build
  six. These are the other eight, with each one's required keys in the schema's order besides `sessionUpdate`, and its Go type:

  | Variant | Required | Go type | Gate |
  | :--- | :--- | :--- | :--- |
  | `available_commands_update` | `availableCommands` | `SessionAvailableCommandsUpdate` | none |
  | `current_mode_update` | `currentModeId` | `SessionCurrentModeUpdate` | none |
  | `config_option_update` | `configOptions` | `SessionConfigOptionUpdate` | none |
  | `session_info_update` | none; `title` and `updatedAt` are `Patch[string]` | `SessionSessionInfoUpdate` | none |
  | `usage_update` | `used`, `size` | `SessionUsageUpdate` | none |
  | `notice` | `severity`, `title` (`minLength: 1`) | `SessionUpdateNotice` | `session.notices` |
  | `compaction_update` | `compactionId`, `status` | `SessionCompactionUpdate` | `session.compaction` |
  | `compaction_summary_chunk` | `compactionId`, `content` | `SessionUpdateCompactionSummaryChunk` | `session.compaction` |

  The three gates are `CapabilityRules` at `acp.go:180`–`:182`; `gates_test.go:496`–`:501` covers them. `schema.unstable.json`
  adds five more variants: `plan_update`, `plan_removed`, `subagent_update`, `session_message` and `session_message_chunk`.
* **R3. Where the discriminator comes from.**
  * The generated `MarshalJSON` (`types_gen.go:15633`–`:15703`) writes `sessionUpdate` from whichever variant pointer is set: it
    copies the variant, sets its `SessionUpdate` field to the literal value, and encodes it. It checks the fields in the struct's
    order, so the first one set is what is sent.
  * A decoded variant keeps the value in that field, since `decodeKeys` reads it through the variant's `Alias`.
  * R1's six helpers leave the field empty. The `ContentBlock` and `ToolCallContent` helpers set their `Type` discriminator (`:18`–`:90`).

  So today a value from `UpdateAgentMessageText("x")` and the same value decoded from its JSON send identical bytes, but
  `reflect.DeepEqual` reports them different.
* **R4. What `Unknown` holds.**
  * The generated `UnmarshalJSON` (`types_gen.go:15155`–`:15260`) reads the discriminator after `foldKeys` (`types_gen.go:22792`):
    under Unicode case folding, the last matching key wins, as `encoding/json` reads a struct field.
  * A discriminator that is present and not a string fails the decode. One that names no variant, the empty string included, puts
    the whole payload in `Unknown` (`:257`–`:259`).
  * A caller may also set `Unknown` by hand to anything.
* **R5. No accessor today.** `SessionUpdate` has `MarshalJSON`, `UnmarshalJSON`, `Validate` and `gates`, and nothing that names its
  variant. `schemaUnions()` in `schema_gen_test.go` (generated) lists `SessionUpdate`'s variants with each one's field and
  discriminator value, from `schema.unstable.json`. `conformance_test.go` already iterates it.
* **R6. The `Client` interface** (`types_gen.go:22649`) has 11 methods. Nine of them are gated on the client's own capabilities, by
  `CapabilitySteps` (`acp.go:75`–`:87`) and `CapabilityRules`:
  * `fs/read_text_file` and `fs/write_text_file`, by method gate;
  * the five `terminal/*` methods, by method gate;
  * `elicitation/complete`, by `elicitation.url`;
  * `elicitation/create`, by the variant gates of its mode (`:175`–`:179`).

  The client's dispatch applies those gates on receipt, before the handler. `TestMethodGates` requires `-32601` and that no handler
  ran. The remaining two methods, `SessionUpdate` and `RequestPermission`, are ungated. The method names are the `ClientMethod*`
  constants (`constants_gen.go:42`–`:53`), and `NewMethodNotFound(method)` (`errors.go:219`) answers `-32601`, "Method not found",
  with data `{"method": method}`.
* **R7. Promoted methods lose to the outer type's own.** A scratch program, outside the tree: `type host struct{ Refusals }` declares
  `ReadTextFile` itself and embeds a `Refusals` that has `ReadTextFile` and `CreateTerminal`. Called through an interface, it printed
  `real refused`. The Go specification's selector rule says the same: the shallowest depth wins. The issue says magic-cli-remote must
  not embed `ClientRefusals` "as a whole: that would hide its real fs and terminal methods". Embedding would not hide them. The real
  hazard is different: a real method renamed or removed later would be replaced by a refusal without a compile error. The issue's
  rule stands, for that reason.
* **R8. What the consumers copy.**
  * `<cli-b>` writes five `SessionUpdate` literals in its agent: `acpserver/commands.go:531, 597, 728`, `modes.go:84`–`:86` and
    `prompt.go:312`–`:320`.
  * It writes nine client stubs (`acpclient/conn.go:552`–`:600`): eight return `NewMethodNotFound(acp.ClientMethod…)`, and
    `CompleteElicitation` returns nil.
  * magic-cli-remote decodes `Unknown` by hand to name a declined update (`internal/provider/acpagent/session.go:2076`–`:2082`).
* **R9. The API gate.** Adding functions, a type, and a method to a struct type is compatible under `apidiff`. `make api` runs it
  against the newest `v1` tag, `v1.20.0`, and lists such additions as compatible, so a release carrying them is a minor, `v1.21.0`
  (`RELEASING.md`, "Tag and Publish").
* **R10. A future field named `Kind`.** A method `Kind` on `SessionUpdate` collides with a variant field of the same name, should a
  later schema add one. The generator type-checks its output with the hand-written files before writing anything
  ([0011-PLAN-safe-output.md](0011-PLAN-safe-output.md); `TestCheckAcceptsRepository`). The collision therefore stops `make version`
  with the type checker's error, and nothing is written.

## Decision Drivers

* **The owner's direction in issue #11**, which this record does not reopen.
* **No change to the wire.** The new constructors send exactly what the composite literals send today.
* **No drift on a schema bump.** A new stable variant, or a variant `Kind()` does not name, fails a test rather than passing silently.
* **No silent refusals.** A client that forgets `SessionUpdate` or `RequestPermission` must not compile.
* **Additive under `v1`.** `make api` lists only compatible changes.
* **No change to what existing helpers return**, unless the owner chooses one (question 1).

## Considered Options

* **O1.** Hand-written, as the issue directs: eight constructors (nine with the text convenience) and `Kind()` in `helpers.go`;
  `ClientRefusals` in a new hand-written file. Tests driven by `schema/schema.json` and `schemaUnions()` catch drift.
* **O2.** Generated: the generator emits the constructors and `Kind()` from the schema.
* **O3.** O1's constructors and `Kind()`, with a generated `UnimplementedClient` that implements every `Client` method as
  method-not-found, in place of `ClientRefusals`.
* **O4.** Nothing here. The consumers keep their copies.

## Decision Outcome

Chosen option: "O1", because it is the owner's direction in issue #11. It changes no generated file. Its tests fail on a schema
bump that adds a stable variant, or a variant `Kind()` does not name (R5). And it keeps the two ungated `Client` methods
compile-required (R6). The owner decided on 2026-10-24, with the recommended answers to questions 1 to 4 (see "Decided with the
owner, 2026-10-24").

* O2 could generate the constructors only by taking `SessionUpdate` out of `HandWrittenHelpers`. That would replace
  `UpdateAgentMessageText` and the other helpers with the generated `New<Union><Variant>` forms (R1), which the issue forbids, and
  removing the old names would break `v1`. A generated `Kind()` alone would avoid drift by construction, but would put a generator
  change, engine cases and the regeneration gates on a method that R5's test already guards.
* O3 is the "gRPC trap" the issue names: a forgotten `SessionUpdate` or `RequestPermission` compiles and answers `-32601`.
* O4 leaves R8's copies and the drift between them, and the consumers' issues have nothing to pin.

### Specification, as decided

#### The constructors, in `helpers.go`

Each constructor takes the variant's required keys in the schema's `required` order (R2), as
[0014-MADR-union-constructors-take-every-required-key.md](0014-MADR-union-constructors-take-every-required-key.md) orders the
generated ones. Each returns a `SessionUpdate` with that one variant set, and its doc comment names the variant's wire value. The
three that a capability gates also name the capability, and say that sending fails with a `*CapabilityError` when the peer did not
advertise it.

| Constructor | Sets |
| :--- | :--- |
| `UpdateAvailableCommands(commands []AvailableCommand) SessionUpdate` | `AvailableCommandsUpdate`, `AvailableCommands: commands` |
| `UpdateCurrentMode(id SessionModeId) SessionUpdate` | `CurrentModeUpdate`, `CurrentModeId: id` |
| `UpdateConfigOptions(opts []SessionConfigOption) SessionUpdate` | `ConfigOptionUpdate`, `ConfigOptions: opts` |
| `UpdateSessionTitle(title Patch[string]) SessionUpdate` | `SessionInfoUpdate`, `Title: title` |
| `UpdateUsage(used, size uint64) SessionUpdate` | `UsageUpdate`, `Used: used`, `Size: size` |
| `UpdateNotice(severity NoticeSeverity, title string) SessionUpdate` | `Notice`, `Severity`, `Title` |
| `UpdateCompaction(id CompactionId, status CompactionStatus) SessionUpdate` | `CompactionUpdate`, `CompactionId`, `Status` |
| `UpdateCompactionSummary(id CompactionId, content ContentBlock) SessionUpdate` | `CompactionSummaryChunk`, `CompactionId`, `Content` |
| `UpdateCompactionSummaryText(id CompactionId, text string) SessionUpdate` | the same, with `TextBlock(text)` |

How the arguments are treated:

* A nil `commands` or `opts` is sent as `[]`, as the generated `MarshalJSON` sends every nil required array (R3).
* `UpdateSessionTitle(PatchValue(t))` sets the title, and `UpdateSessionTitle(PatchNull[string]())` clears it. A zero `Patch`
  sends a `session_info_update` that changes nothing. `UpdatedAt` is set on the returned variant.
* `UpdateNotice` with an empty title fails `Validate`, and so fails to send (`minLength: 1`). Its doc comment says so.
* Each constructor writes the variant's `SessionUpdate` field as question 1 decides.
* Optional fields are set on the returned variant's pointer. These are usage's `Cost`, the notice's `Description` and `Meta`, and the
  compaction's `Summary`, `Error` and `Meta`. No functional options are added.
* None of the unstable variants gets a constructor.

`helpers_gen.go`, `HandWrittenHelpers` and the generator are unchanged. *Amended 2026-10-25: one generator test,
`TestRuntimeFilesCoverHandWritten`, names `refusals.go`; see "Amendment, 2026-10-25" below.*

#### `Kind()`, in `helpers.go`

```go
// Kind returns the sessionUpdate value of the variant u sends: the first set
// variant, in the order MarshalJSON checks them, such as "agent_message_chunk"
// or "notice". With no variant set, it returns the "sessionUpdate" of Unknown,
// read as UnmarshalJSON reads it, when that is a non-empty string, and
// "unknown" when Unknown holds anything else. The zero SessionUpdate returns "".
func (u SessionUpdate) Kind() string
```

* It names every variant, the unstable ones included. It switches on the pointer fields, in R3's order, and never reads a
  variant's `SessionUpdate` field, so a constructed value and a decoded one give the same answer.
* For `Unknown`, it decodes the object, applies `foldKeys(b, m, "sessionUpdate")`, and decodes that key into a string. This is the
  generated decoder's rule (R4), so `Kind()` agrees with what decoding saw.
* A non-object, a missing key, a non-string, or `""` gives `"unknown"`, as question 2 decides.
* It allocates only for `Unknown`.

#### `ClientRefusals`, in a new hand-written file `refusals.go`

```go
// ClientRefusals answers method-not-found for the Client methods a client may
// leave unadvertised ...
type ClientRefusals struct{}
```

* **The methods.** `ReadTextFile`, `WriteTextFile`, `CreateTerminal`, `KillTerminal`, `TerminalOutput`, `ReleaseTerminal`,
  `WaitForTerminalExit` and `CreateElicitation`. Each has a value receiver, and returns its response type's zero value and
  `NewMethodNotFound(ClientMethod…)` with its own method's constant. `CompleteElicitation` returns nil.
* **Exactly these nine**, and no others, so `SessionUpdate` and `RequestPermission` stay compile-required for any type that embeds it.
* **The doc comment** says three things:
  * the SDK's gates answer before these run when the client did not advertise the capability (R6), so they run only for a capability
    the client advertised;
  * an embedder's own method of the same name takes precedence (R7);
  * a client that implements some of these methods for real should implement all of them itself rather than embed this, because a
    real method renamed or removed would otherwise be refused without a compile error (R7; the issue's rule for magic-cli-remote).
* **Not added:** an agent-side counterpart, and any working fs, terminal or elicitation implementation. `example/client` is
  unchanged.

#### Documentation

* `README.md`'s helper list gains a "Session updates" item. It names every `Update*` constructor, the existing ones included, and
  `Kind()`. The client section names `ClientRefusals`.
* `docs/architecture.md`'s "Hand-written" list names `refusals.go`, and says that `helpers.go` holds `Kind()`.
* Runnable examples are added for `UpdateSessionTitle`, `SessionUpdate.Kind` and `ClientRefusals`.

#### Questions

1. **The variant's `SessionUpdate` field in the new constructors.**
   * Recommended: the new constructors set it to the variant's value. A constructed value then equals the same value decoded from
     its JSON, and this matches the `ContentBlock` helpers and the issue's "discriminator written by the constructor". The six
     existing helpers stay as they are, because changing what an existing function returns would break a consumer test that compares
     it with a composite literal. Their doc comments are unchanged.
   * Alternative: set it in all fourteen, and accept that change to the six.
   * Alternative: set it in none, uniform with the six. The bytes sent are the same either way (R3).
2. **`Kind()` for an `Unknown` whose discriminator is `""`.**
   * Recommended: `"unknown"`, so that `""` means only the zero value. This narrows the issue's wording, "the payload's sessionUpdate
     string when that key is a JSON string", by one case.
   * Alternative: `""`, as that wording reads.
3. **Where `ClientRefusals` lives.**
   * Recommended: a new `refusals.go`. It is not a constructor, and `helpers.go` holds constructors.
   * Alternative: `helpers.go`.
4. **The release.**
   * Recommended: the work is released as `v1.21.0` once it is committed and CI has passed, through `RELEASING.md` and a release-notes
     report, `0036-REPORT-release-notes-v1-21-0.md`. Both consumers' issues pin that tag. The tag and the push need the owner's ask in
     that turn.
   * Alternative: hold the release for other work.

#### Decided with the owner, 2026-10-24

The owner: "take recommendations and proceed".

| Question | Answer |
| :--- | :--- |
| 1. The variant's `SessionUpdate` field | As recommended: the new constructors set it; the six existing helpers are unchanged. |
| 2. `Kind()` for an `Unknown` with `""` | As recommended: `"unknown"`. |
| 3. Where `ClientRefusals` lives | As recommended: a new `refusals.go`. |
| 4. The release | As recommended: `v1.21.0`, once committed and CI has passed; the tag and the push on the owner's ask in that turn. |

#### Amendment, 2026-10-25

This record said the generator is unchanged, and the PLAN kept `cmd/generate` out of scope. That missed
`cmd/generate/engine_test.go`'s `TestRuntimeFilesCoverHandWritten`. It requires every hand-written `.go` file of the root to be
either one of `enginetest.RuntimeFiles`, which are compiled into each synthetic case, or excluded by name, as `helpers.go` is.
`refusals.go` uses ACP types, so it cannot be a runtime file. With it added, the test failed on every host: "hand-written files of the
root are [connection.go doc.go errors.go extensions.go helpers.go refusals.go], want [connection.go doc.go errors.go extensions.go
helpers.go]".

The owner chose, out of two resolutions, "option 1": the test excludes `refusals.go` beside `helpers.go`, and its comment says so.
The other resolution, putting `ClientRefusals` in `helpers.go`, would have reversed question 3. The generator's code, its output and
`HandWrittenHelpers` are still unchanged; only that test's list of exclusions is.

### Consequences

* Good, because `<cli-b>` can drop its five literals and nine stubs, and magic-cli-remote its `Unknown` decoding, once they pin the
  tag.
* Good, because a schema bump that adds a stable variant, or any variant, fails a test until a constructor or a `Kind()` case is
  written.
* Good, because nothing generated changes, and nothing sent on the wire changes.
* Neutral, because `Kind()` duplicates the discriminator values the generated `MarshalJSON` writes. The `schemaUnions()` test keeps
  the two equal.
* Neutral, because, under question 1's recommendation, the new constructors set the field and the six old ones do not. The
  difference is invisible on the wire, but visible to `reflect.DeepEqual`.
* Bad, because a later schema variant named `Kind` stops `make version` until this method is renamed, which would break `v1` (R10).
  No variant in either schema file has that name.

### Confirmation

The PLAN shows each of the following, every new check first seen to fail on a planted break in a scratch copy:

* each constructor's encoding equal to a golden file and to the composite literal;
* every stable variant in `schema/schema.json` having a constructor;
* `Kind()` equal to every `schemaUnions()` value for `SessionUpdate`, and the `Unknown`, zero, folded-key and two-variant cases;
* `ClientRefusals` refusing each of its eight request methods with its own method's name, having exactly nine methods, and not
  implementing `Client` alone;
* a client that embeds it answering `-32601` over the wire;
* `make api` against `v1.20.0` listing only compatible changes.

## More Information

* Issue #11, and the consumer issues `<cli-b>` #5 and magic-cli-remote #28.
* [0035-REPORT-consumer-acp-code-for-the-sdk.md](../reports/0035-REPORT-consumer-acp-code-for-the-sdk.md), C1, A4 and B3: the
  consumers' copies.
* [0014-MADR-union-constructors-take-every-required-key.md](0014-MADR-union-constructors-take-every-required-key.md): constructor
  parameters in the schema's required order.
* [0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md): `Patch[T]`, which
  `UpdateSessionTitle` takes.
* [0025-REPORT-schema-1-26-0-adoption-assessment.md](../reports/0025-REPORT-schema-1-26-0-adoption-assessment.md): why unstable API
  waits for a policy.
* Later issues, out of this record: constructors for the unstable variants; `Kind()` on the other unions (0035's A4); an agent-side
  counterpart to `ClientRefusals` (0035's B3).

---
status: in-progress
date: 2026-10-12
associated-madr: "0015-MADR-validate-absolute-protocol-paths.md"
---

# Implement absolute protocol paths, both directions, with consumer hooks

Associated MADR: [0015-MADR-validate-absolute-protocol-paths.md](0015-MADR-validate-absolute-protocol-paths.md)

## Goal

Carry out the accepted decision: W1, F1, E1, A4, H1, X1 and C1, in the order the owner chose (this plan before steps 6 to 8 of
[0014-PLAN-union-constructors-take-every-required-key.md](0014-PLAN-union-constructors-take-every-required-key.md)).

When it is done:

* nothing the SDK sends carries one of the 17 path properties in a form that is absolute nowhere, and no option weakens that;
* everything it receives is decoded, offered to the consumer's rewrite hook, validated with the receiving host's `filepath.IsAbs`, and
  offered to the consumer's validation hook, before a handler runs or a caller gets a response;
* every generated check returns a `*ValidationError`, and the session's root set has an accessor;
* the records, the README and the `v0.15.0` erratum say what changed, and 0014's remaining steps carry it into the `v0.16.0` notes.

Each step's change is proven by a test that has been seen failing on a planted break, and every test passes on the three CI hosts.

## Facts this plan rests on

Read on `23d03b5`, 2026-10-12, beside the MADR's own facts.

* **The protocol table.** `model.Protocol` (`cmd/generate/internal/model/protocol.go`) has `Interfaces`, `Methods` and
  `HandWrittenHelpers`; `newRules` checks a table before any type is built. `resolve` builds every type, then `buildValidations`
  (`model.go:301`), then applies the method rules (`linkPromptTurns`, `checkCapabilities`, `checkTypeRules`, `model.go:328` to `330`).
  `TestProtocolTableAppliesToCommittedSchema` (`protocol_test.go:276`) requires every method rule to apply to the committed schema and
  counts 14 capability gates.
* **Where a field's facts come from.** `objectFields` (`fields.go:378`) and `plainFields` (`fields.go:427`) build each `Field` from its
  property's `*load.Definition` through `factsOf(prop)`; `Facts` holds `Format`, `Minimum`, `Maximum`, `MinLength`, `Values`,
  `DefaultOnError` and `SkipInvalidItems` (`fields.go:98`). `expandAllOf` copies property pointers unchanged (`schema.go:59`), so the
  inline variant `ToolCallContentDiff` is built from the same `*load.Definition` as `Diff.path`.
* **Checks.** `ownChecks` turns facts into `Check`s through `bounds` (`validate.go:151`, `:225`); a check has a `Kind` and a `Shape`
  (`RefNamed`, `RefPointer`, `RefSlice`, `RefMap`). The emitter writes every `Validate` through `validateMethod` (`types.go:869`), every
  nested call through `validateCall` (`types.go:874`), each own check through `checkStmt` (`types.go:905`), and the union and member counts
  in `unionValidateBody` and the members check. `types_gen.go` has 142 `Validate` methods; no hand-written file has one.
* **Messages a test pins.** `TestValidateSchemaConstraints` (`fidelity_test.go:256`) pins `Notice: title: shorter than minLength 1`,
  `CreateElicitationRequest must have exactly one variant set` and the nested forms, and its case `empty required string`
  (`fidelity_test.go:265`) requires `(&ReadTextFileRequest{SessionId: "s"}).Validate()` to pass: the regression this plan reverses.
* **Dispatch.** The generated inbound path decodes and validates in `unmarshalValidate` (`internal/emit/dispatch_helpers.go:27`) and
  answers through `invalidParams()`; a handler's result goes out through `callWithResult` (`dispatch_helpers.go:99`). A notification's
  error is logged and dropped (`connection.go`, after `c.handler(...)`).
* **Sending.** `SendRequest` and `SendRequestNoResult` marshal params in `prepareRequest`; `SendNotification` marshals its own; `SendRequest`
  decodes the result with `json.Unmarshal` and calls no `Validate` (`connection.go:774` on).
* **Runtime files.** `enginetest.RuntimeFiles` lists the hand-written files that use no ACP type (`agent.go`, `client.go`,
  `connection.go`, `doc.go`, `errors.go`, `extensions.go`); `TestRuntimeFilesCoverHandWritten` requires every other hand-written root file
  to be excluded by name, as `helpers.go` is. Code the generated runtime needs goes in a runtime file; code that names an ACP type goes in
  `helpers.go`.
* **Tests this change reaches.**
  * Over a connection, with POSIX literals that the receiving rule refuses on Windows: `acp_test.go` (`Cwd: "/"`, `"/test"`, `"/tmp"`,
    `Path: "/test.txt"`, `"/file1.txt"` and others, 13 lines), `connection_notification_barrier_test.go:35`, `example_client_test.go:137`,
    `example_gemini_test.go:90`, `example_agent_test.go:83` and `:96`.
  * Marshal only, never validated: `json_parity_test.go`, `fidelity_test.go` (bar the `Validate` table), `raw_value_test.go`,
    `generated_output_test.go`; unaffected.
  * `instances_test.go` fills every string with `"s"` (`:201`), and `roundTrip` (`conformance_test.go:282`) validates each instance and
    each `testdata/json_golden` file; nine goldens hold POSIX paths (`fs_read_text_file_request.json`, `new_session_request.json`,
    `tool_content_diff.json` and six more).
  * `FuzzRoundTrip` calls `Validate` only to require it not to panic (`fuzz_test.go:78`).
* **CI.** Tests run on `ubuntu-24.04`, `macos-15` and `windows-2025` (`.github/workflows/ci.yml:24`); `make pre-add-check` lints for
  `GOOS=windows` but runs tests on the host only, so a Windows failure is first seen in CI.

## Scope

In scope:

* `cmd/generate/internal/model`: `protocol.go` and `acp.go` (the `AbsolutePaths` rules), `fields.go` (`Field`'s property, `Facts`),
  `model.go` (applying the rules), `validate.go` (`CheckAbsolutePath`), and their tests.
* `cmd/generate/internal/emit`: `types.go` (the `Validate` and `validate` pair, the path check, `*ValidationError`),
  `dispatch_helpers.go` and `dispatch.go` (the receiving pipeline, result validation), `registry.go` if the tests need the rule list, and
  their tests.
* `cmd/generate`: engine cases in `testdata/engine/`; `enginetest/api.go` if a new runtime file is added.
* Hand-written root files: `errors.go` (`ValidationError`, `ValidationRule`), `connection.go` (`isPortableAbs`, outbound validation, the
  receiving pipeline, the two options), `helpers.go` (`Roots`, helper documentation).
* Generated files, regenerated only: `types_gen.go`, `agent_gen.go`, `client_gen.go`, `schema_gen_test.go`.
* Root tests: `fidelity_test.go`, `conformance_test.go`, `instances_test.go`, `acp_test.go`, `connection_notification_barrier_test.go`,
  the three example tests, and new tests named below.
* `example/client`, `example/claude-code`, `example/gemini`, `example/agent`.
* Documentation and records: `README.md`, `docs/architecture.md`, an amendment to
  [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), an erratum section in
  [0013-REPORT-release-notes-v0-15-0.md](../reports/0013-REPORT-release-notes-v0-15-0.md), an amendment to
  [0014-PLAN-union-constructors-take-every-required-key.md](0014-PLAN-union-constructors-take-every-required-key.md), `docs/README.md`.

Out of scope:

* Capability-gate enforcement and middleware: their own records, after this plan (MADR, "Decided with the acceptance").
* The `v0.16.0` release notes, the advisory text, the tag and the GitHub release: 0014's step 7 writes the notes and the advisory draft,
  as amended in step 2; the owner tags, publishes, and posts.
* The other 22 removed checks, which stay removed.
* Validating extension (`_`-prefixed) methods, whose params are untyped.

## Implementation Steps

1. **Rules.** As in [0014-PLAN-union-constructors-take-every-required-key.md](0014-PLAN-union-constructors-take-every-required-key.md),
   step 1, as amended there:
   * each new test or check is seen failing on a planted break in a scratch copy of the working tree's tracked and untracked files, never
     in the tree; the record quotes the failure;
   * a behaviour no record decides stops the step for the owner, with evidence and resolutions that leave the tree correct;
   * after each step: regenerate (`cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .`), `make pre-add-check`, each untracked Go file
     through `GO_PRECHECK_SKIP_VULN=1 scripts/go-precheck.sh`, `GOEXPERIMENT=nojsonv2 go test -count=1 ./...` at the root, `make
     check-records`, `make markdownlint`, and the identifier scan of the changed files; record the outputs; stop for the owner's commit on
     `main`;
   * every change to a generated file and to the exported API listing (the listing program of the 0013 report, on `git archive HEAD` and
     on the tree) is asserted by a script against what the step names; anything else stops the step;
   * Windows: `GOOS=windows go vet ./...`, which type-checks every package and test file for Windows, passes locally at the root and in
     `cmd/generate`; the tests themselves run on `windows-2025` in CI when the owner pushes, and the record notes that run. A Windows
     failure is a deviation.

2. **Records.** No source changes.
   * [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md) gains "Amendment 2026-10-12, from 0015", before "More
     Information": outcome 7 gains the protocol's prose requirements listed in the protocol table as a source of checks; `Validate` runs on
     what the SDK sends and on the responses it receives; the path rule reads absolute by direction (0015-MADR, items 2, 4, 5, 7).
   * [0013-REPORT-release-notes-v0-15-0.md](../reports/0013-REPORT-release-notes-v0-15-0.md) gains "Erratum, 2026-10-12": the 28
     empty-string checks `v0.15.0` removed, named by type and field (the MADR's list, from `git show v0.14.0:types_gen.go`), with `Cwd` and
     `Path` flagged as the ones with security weight, and a pointer to issue #2 and the MADR. Its "Release text" is unchanged; the owner
     appends the erratum's text to the published `v0.15.0` release.
   * [0014-PLAN-union-constructors-take-every-required-key.md](0014-PLAN-union-constructors-take-every-required-key.md) gains a dated
     amendment to steps 6 and 7: step 6 documents both records in the README and `docs/architecture.md`; step 7's report covers both, lists
     the path checks and the two directions under changes to what the SDK accepts and sends, with a security note, names the 28 removed
     checks, lists `ValidationError`, the two options and `Roots` under additions, and drafts the advisory for `v0.15.0` (affected
     `v0.15.0`, fixed in `v0.16.0`, referring to issue #2).
   * This plan's status is `in-progress`; `docs/README.md` says so.
   * `make check-records` and `make markdownlint` pass.

3. **The typed error (X1, part).**
   * `errors.go`: `type ValidationRule string` with constants `RuleAbsolutePath`, `RuleMinLength`, `RuleMinimum`, `RuleMaximum`,
     `RuleOneVariant`, `RuleOneMember`, `RuleExcludedValue`; `type ValidationError struct { Type, Field string; Value any; Rule
     ValidationRule; msg string }`, whose `Error` returns `msg`, the message the generator writes, so every message is unchanged.
   * `internal/emit/types.go`: each own check (`checkStmt`), the union count and catch-all checks (`unionValidateBody`) and the member count
     return `&ValidationError{...}` with `Type` the Go type, `Field` the JSON key (empty for a count), `Value` the offending scalar (nil for
     a count), `Rule`, and today's text. Nested wrapping (`validateCall`) is unchanged, so `errors.As` reaches the inner error.
   * Regenerate. Asserted: `types_gen.go` changes only in the return statements of the checks; `agent_gen.go` and `client_gen.go` do not
     change; the API listing gains only `ValidationError`, its `Error` method, `ValidationRule` and the seven constants.
   * Tests: `TestValidateSchemaConstraints` passes unchanged (messages kept); a new root test, `TestValidationErrorRules`, takes one
     failing value per rule (a `Notice` below `minLength`, an empty `ContentBlock`, a catch-all holding another variant's value, a
     flattened-members value with none set, a bound from the conformance bound cases) and requires `errors.As` to find a `*ValidationError`
     with the expected `Rule`, `Type` and `Field`, through at least one level of nesting; a model or emitter test covers `RuleMaximum` and
     `RuleMinimum` if no committed type has both.
   * Plants: one check kind emitted as `errors.New` again, which must fail `TestValidationErrorRules` for that rule; `Error` returning
     an empty string, which must fail `TestValidateSchemaConstraints`.

4. **The path rule and the receiving check (W1, F1, A4 receiving).**
   * `protocol.go`: `Protocol` gains `AbsolutePaths []PathRule`, `type PathRule struct { Definition, Property, Source string }`;
     `newRules` refuses an empty field and a duplicate. `acp.go`: the 17 rules of the MADR's item 1, each `Source` the specification
     sentence it follows ("All file paths in the protocol MUST be absolute." for those the overview alone covers; the session-setup,
     file-system and terminals sentences for theirs).
   * `fields.go`: `Field` gains an unexported `prop *load.Definition`, set by `objectFields` and `plainFields`; `Facts` gains
     `AbsolutePath bool`.
   * `model.go`: before `buildValidations`, a pass resolves each rule whose definition the schema has to that property's `*load.Definition`,
     refusing a rule whose property is missing or is not a string, a `string | null`, or an array of strings (`<Definition>.<Property>:
     an absolute-path rule needs a string, an optional string, or an array of strings`), then sets `Facts.AbsolutePath` on every field of
     every type, inline variants included, whose `prop` is one of them. A rule whose definition the schema lacks is skipped, as the method
     rules are.
   * `validate.go`: `CheckAbsolutePath`, built by `ownChecks` for a field with the fact, in the field's shape; the type validates.
   * `internal/emit/types.go`: every `Validate` becomes `func (v *T) Validate() error { return v.validate(filepath.IsAbs) }` and an
     unexported `func (v *T) validate(abs func(string) bool) error` with today's body; `validateCall` calls `x.validate(abs)`;
     `CheckAbsolutePath` emits, by shape, `if !abs(v.Cwd)`, `if v.Cwd != nil && !abs(*v.Cwd)`, and a loop naming the index; each returns a
     `*ValidationError` with `Rule: RuleAbsolutePath` and the message `<Type>: <key>: %q is not an absolute path` (`<key>[%d]` for an
     element).
   * `connection.go`: `isPortableAbs(p string) bool`, true for a path that begins with `/`, with a drive letter, a colon and `\` or `/`, or
     with `\\`.
   * Regenerate. Asserted: the API listing does not change; `types_gen.go` gains the `validate` methods and the path checks, which appear
     only in the 13 types holding the 18 fields (`NewSessionRequest`, `LoadSessionRequest`, `ResumeSessionRequest`,
     `UnstableForkSessionRequest`, `ListSessionsRequest`, `SessionInfo`, `McpServerStdio`, `ReadTextFileRequest`, `WriteTextFileRequest`,
     `CreateTerminalRequest`, `ToolCallLocation`, `Diff`, `ToolCallContentDiff`); a script counts 18 `is not an absolute path` checks.
   * Tests:
     * `TestProtocolTableAppliesToCommittedSchema` requires the 17 rules to resolve on the committed schema and the 18 fields to carry the
       fact; a model test requires `Diff`'s rule to reach `ToolCallContentDiff`, and `Build` to refuse a rule on a missing property and on
       an integer property;
     * a root unit test of `isPortableAbs` accepts `/a`, `C:\a`, `C:/a`, `\\server\share` and `\\?\C:\a`, and refuses `""`, `a`, `./a`,
       `\a` and `C:a`; it holds on every host;
     * a new engine case, `accept-absolute-paths.txtar`, defines `NewSessionRequest` (`cwd`, `additionalDirectories`),
       `CreateTerminalRequest` (an optional `cwd`), and `ToolCallContent` with a `Diff` variant through `allOf`; its behaviour test
       requires `""` and `rel` to fail, `t.TempDir()` to pass, an unset optional to pass, `additionalDirectories` with one relative
       element to fail naming `additionalDirectories[1]`, the diff variant's relative path to fail, and every failure to be a
       `*ValidationError` with `RuleAbsolutePath`;
     * `TestValidateSchemaConstraints`: `empty required string` becomes `ReadTextFileRequest: path: "" is not an absolute path`, and a case
       for a present empty string on a field with no rule (the elicitation message case already there) keeps 0012's rule;
     * the instance builder (`instances_test.go`) fills each property a rule names with an absolute path for the host (from
       `os.TempDir()`), taking the rule list from a registry entry, `schemaAbsolutePaths()`, added to `schema_gen_test.go` by
       `internal/emit/registry.go`; `roundTrip` validates goldens and instances with `validate(isPortableAbs)`, as they are peers' messages
       from an unknown host;
     * the tests that send POSIX literals over a connection build their paths from `t.TempDir()` (examples from `os.TempDir()`), so they
       pass on Windows.
   * Plants: the rule list emptied (the engine case and the table test fail); the receiving rule replaced by a non-empty test (the engine
     case fails on `rel`); `Diff`'s rule not following into `ToolCallContentDiff` (the model test and the engine case fail);
     `isPortableAbs` treating `\a` as absolute (its unit test fails).

5. **Sending (E1, A4 sending).**
   * `connection.go`: an unexported `validator` interface (`validate(func(string) bool) error`); `validateOutbound(params any) error`,
     which validates params of a generated type with `isPortableAbs`, a pointer directly and a value through an addressable copy; called by
     `prepareRequest` (so by `SendRequest` and `SendRequestNoResult`) and by `SendNotification`, before marshalling; its error is returned
     as is, and nothing is written.
   * `internal/emit/dispatch_helpers.go`: `callWithResult` validates a validating result type with `isPortableAbs`; a failure is logged
     through the connection's logger and answered with `NewInternalError(map[string]any{"error": err.Error()})`, `-32603`.
   * `helpers.go`: the documentation of `ToolDiffContent`, `StartReadToolCall`, `StartEditToolCall`, `WithStartLocations` and
     `WithUpdateLocations` says the path must be absolute.
   * Regenerate. Asserted: `agent_gen.go` and `client_gen.go` change only in the result branch of the cases whose result type validates;
     the API listing does not change.
   * Tests, a new root file `paths_test.go`:
     * `TestSendRefusesRelativePaths`: for a request or notification carrying each of the 17 properties (nested ones through their
       parents: `McpServerStdio` in `NewSessionRequest`, `ToolCallLocation` and the diff in `SessionUpdate`), sending `""` and
       `relative/dir` returns a `*ValidationError` with `RuleAbsolutePath` and writes nothing (a writer that records every byte); sending
       `/a` and `C:\a` writes the message;
     * `TestHandlerResultValidated`: an agent whose `ListSessions` returns a relative `SessionInfo.cwd` answers `-32603` naming
       `SessionInfo` and `cwd`, and logs it.
   * Plants: `validateOutbound` not called (the send test fails); `isPortableAbs` replaced by `filepath.IsAbs` (the `C:\a` case fails on
     Linux and macOS, the `/a` case on Windows); the result check removed (the handler test fails).

6. **Receiving: hooks and responses (E1, H1).**
   * `connection.go`: `connectionConfig` gains `rewrite` and `inbound` functions; options `WithInboundRewrite(func(ctx context.Context,
     method string, params any) error)` and `WithInboundValidation(func(ctx context.Context, method string, params any, err error)
     error)`; a method `receive(ctx, method string, params any) error` runs the rewrite hook, then `validate(filepath.IsAbs)` when `params`
     has it, then the validation hook with that result, and returns the error to act on; `inboundError(err)` returns a `*RequestError` as
     it is and any other error as `NewInvalidParams(map[string]any{"error": err.Error()})`.
   * `internal/emit/dispatch_helpers.go`: `unmarshalValidate` keeps decoding as today and replaces `p.Validate()` with `conn.receive(ctx,
     method, &p)`, answering through `inboundError`; a decoding failure is answered as today and reaches no hook. A notification's error
     is dropped and logged, as today.
   * `connection.go`, `SendRequest`: after decoding a result, `c.receive(ctx, method, &result)`; an error is returned wrapped (`%s
     response: %w`), so `errors.As` finds a `*ValidationError` when the error is the SDK's.
   * Regenerate. Asserted: `agent_gen.go` and `client_gen.go` change only where `p.Validate()` was; the API listing gains only
     `WithInboundRewrite` and `WithInboundValidation`.
   * Tests, in `paths_test.go`:
     * receiving each of the 17 properties as `""` or `relative/dir`: a request is answered `-32602` naming the property, a notification
       is dropped and not delivered, a response's error is returned and wraps a `*ValidationError`;
     * the rewrite hook resolves a bare `McpServerStdio.command` and the handler receives the resolved value; a rewrite that leaves the
       value relative is refused; a rewrite returning an error refuses the message;
     * the validation hook accepts a violation and the handler receives it; it refuses a valid message with its own error; a
       `*RequestError` it returns reaches the peer with its own code;
     * the hooks see responses (`ListSessions`) and notifications (`SessionUpdate`), in order rewrite, validate, validate hook;
     * a malformed message (a missing required key) is answered `-32602` without calling either hook.
   * Plants: `receive` skipping the rewrite hook, skipping validation, or skipping the validation hook (one test fails for each);
     `inboundError` mapping every error to `-32602` (the `*RequestError` case fails); `SendRequest` not calling `receive` (the response
     case fails).

7. **Root set, examples, documentation (X1, part; MADR item 8).**
   * `helpers.go`: `Roots() []string` on `NewSessionRequest`, `LoadSessionRequest`, `ResumeSessionRequest`, `UnstableForkSessionRequest`
     and `SessionInfo`, returning `cwd` then `additionalDirectories` in order, as a new slice.
   * Examples: `example/client`, `example/claude-code`, `example/gemini` and `example_client_test.go` drop their hand-written
     `filepath.IsAbs` checks in `ReadTextFile` and `WriteTextFile`; `example/agent` keeps each session's `Roots()`.
   * `README.md`: a "Paths" part of "Wire encoding": the 17 properties, the two readings of absolute, sending enforced, the receiving
     pipeline and the two options with an example of each (resolving a bare MCP command; accepting a violation), what accepting or
     rewriting means, translating paths between hosts, `Roots`, and confining access with `os.OpenRoot` per root; that a client resolves
     a bare MCP command with `exec.LookPath` before sending. `docs/architecture.md`: the protocol table's `AbsolutePaths`, the
     `Validate`/`validate` pair, and the receiving order.
   * Regenerate: nothing changes. Asserted: the API listing gains only the five `Roots` methods.
   * Tests: `TestRoots` requires the order and that the result does not alias the request's slice; `go vet` builds the examples.
   * Plants: `Roots` returning `additionalDirectories` first (`TestRoots` fails); returning the request's own slice (the alias case
     fails).

8. **Close.** Run the verification below and record each result; draft the reply to issue #2 in the execution record (what changed, the
   records, the release that carries it), for the owner to post; set this plan to `complete` and its row in `docs/README.md`; hand off to
   0014's step 6.

## Verification

* The engine case `accept-absolute-paths`, `TestProtocolTableAppliesToCommittedSchema`, the model rule tests, the `isPortableAbs` test,
  `TestValidationErrorRules`, `TestSendRefusesRelativePaths`, `TestHandlerResultValidated`, the receiving and hook tests, and `TestRoots`
  pass, and each failed on its step's plants.
* `TestValidateSchemaConstraints` refuses an empty `ReadTextFileRequest.Path` and still passes a present empty string where no rule
  applies.
* A script counts 18 absolute-path checks in `types_gen.go`, in the 13 types named in step 4.
* The exported API listing, against `23d03b5`, gains exactly `ValidationError` and its `Error`, `ValidationRule` and its seven constants,
  `WithInboundRewrite`, `WithInboundValidation`, and the five `Roots` methods.
* `go test -race -count=1 ./...` passes at the root and in `cmd/generate`; `GOEXPERIMENT=nojsonv2 go test -count=1 ./...` passes at the
  root; `GOOS=windows go vet ./...` passes in both modules; CI passes on `ubuntu-24.04`, `macos-15` and
  `windows-2025` for each pushed step.
* `make pre-add-check`, `make check-records` and `make markdownlint` pass; the identifier scan finds nothing.

## Rollout and Rollback

Commits land on `main`, made by the owner, one per step; steps 3 to 7 each carry their regenerated files. The owner pushes after each
step, and CI on `windows-2025` is the first run of the tests on Windows; the step is closed when that run passes. Nothing is tagged by
this plan: 0014's steps 6 to 8 follow, and the owner tags `v0.16.0`, publishes the release, appends the erratum to `v0.15.0`'s release,
publishes the advisory, and posts the reply to issue #2.

Rollback is `git revert` of a step's commit, latest first: step 7 documents step 6's options; step 6 calls step 4's `validate`; step 5
uses step 4's `isPortableAbs` and `validate`; step 4's checks return step 3's `*ValidationError`. Step 2 is records only.

## Execution record

### Step 2, records, 2026-10-12

Ran on `main` from `b84e22a`, after the owner committed the pair and said "proceed". The agent did not commit.

* [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md) gains "Amendment 2026-10-12, from 0015", before its
  "More Information", which is unchanged: the second source of checks, both directions, and the reading by direction.
* [0013-REPORT-release-notes-v0-15-0.md](../reports/0013-REPORT-release-notes-v0-15-0.md) gains "Erratum, 2026-10-12" at its end: the 28
  removed checks by type and field, the `Cwd` and `Path` ones in bold with their security weight, and the text for the owner to append to
  the published `v0.15.0` release. Its "Release text" is unchanged.
* [0014-PLAN-union-constructors-take-every-required-key.md](0014-PLAN-union-constructors-take-every-required-key.md) gains a dated
  amendment after its step 7: step 6 documents both records, and step 7's report covers both and drafts the advisory.
* This plan's status is `in-progress`, and its row in `docs/README.md` says so.
* No source changes, so no test and no plant.

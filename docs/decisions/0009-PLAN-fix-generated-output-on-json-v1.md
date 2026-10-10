---
status: in-progress
date: 2026-10-10
associated-madr: "0009-MADR-fix-generated-output-on-json-v1.md"
---

# Implement Fix the generated output on encoding/json v1

Associated MADR: [0009-MADR-fix-generated-output-on-json-v1.md](0009-MADR-fix-generated-output-on-json-v1.md)

## Goal

Change the go/ast generator so the SDK sends what schema `1.25.0` requires, reports an unset union by name, documents every union, and emits idiomatic Go, all on `encoding/json` v1.

## Scope

In scope:

* `cmd/generate/internal/emit/types.go` and `cmd/generate/internal/emit/dispatch.go`; added 2026-10-10 in step 7, `ast.go` and
  `dispatch_helpers.go` in the same directory
* The five regenerated files: `constants_gen.go`, `types_gen.go`, `agent_gen.go`, `client_gen.go`, `helpers_gen.go`
* New tests in the root module and in `cmd/generate/internal/emit`
* New golden files under `testdata/json_golden/`

Out of scope:

* `encoding/json/v2` and `encoding/json/jsontext`. [0007-MADR-self-contained-acp-sdk.md](0007-MADR-self-contained-acp-sdk.md) keeps the SDK on v1
* Hand-written code in the root module: `connection.go`, `errors.go`, `extensions.go`, `helpers.go`
* Renaming or removing an exported identifier, or adding an exported type, function, or field. The `MarshalJSON` methods of step 5 are the only exported additions
* A schema stamp other than `1.25.0`, and protocol v2
* Hand-editing `*_gen.go`
* A tag, a push, or a merge to `main` without the owner's explicit ask

Preconditions:

* [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md) is complete and merged. The generator is the go/ast one, and `TestGeneratedFilesUpToDate` exists.
* The owner has accepted [0009-MADR-fix-generated-output-on-json-v1.md](0009-MADR-fix-generated-output-on-json-v1.md).

## Implementation Steps

1. ~~Create `fix/generated-output-json-v1` from `main`.~~ Amended 2026-10-09: work on `main`. At the end of each step below, run `make pre-add-check`,
   `make check-records`, and `make markdownlint`, record their output, and stop. The owner commits with `git commit --no-edit`.
   An agent commits only on a branch the owner names in that turn, `feature/<slug>` by default, after an explicit ask.
2. Write the failing tests first, in the root module (amended 2026-10-09: each test moves into the step that fixes it; see the deviation below):
   * The zero value of `RequestPermissionResponse` fails to encode with an error containing `RequestPermissionOutcome` and "no variant set".
   * `NewSessionRequest{Cwd: "/home/<user>/project"}` encodes `"mcpServers":[]`, and the decoded value passes `Validate`.
   * A `ContentBlock` whose `type` is the number `1` fails to decode with an error naming `ContentBlock` and `type`.
   * The line above `type ContentBlock struct` in `types_gen.go` is a comment.
   Run them and record each failure message. They must fail now. A test that passes before the change is not testing it.
3. Write the pinning tests, which must pass before and after: text with `<`, `>`, and `&` encodes escaped as `\u003c`, `\u003e`, `\u0026`;
   a `map[string]any` in `_meta` encodes with sorted keys; every existing golden in `testdata/json_golden/` is unchanged.
   Run them and record that they pass.
4. Unset unions. In `unionMarshalBody`, end with `return nil, errors.New("<Union>: no variant set")` instead of `return []byte{}, nil`.
   Regenerate. The union test from step 2 passes.
5. Nil required arrays and maps. In `emitObjectSchema` and in the inline union-variant structs, collect required fields whose Go type is a slice or a map.
   For each such struct emit a value-receiver `MarshalJSON` through an alias type that replaces nil with an empty literal, or add those assignments to the
   `MarshalJSON` that `emitObjectDefaults` already emits. ~~Expect 35 fields in 34 structs at stamp `1.25.0`.~~ Corrected 2026-10-10: 34 fields in 33 structs; see the deviation below. Regenerate. The `NewSessionRequest` test passes.
6. Union docs. In `emitSchemaDef`, hold a union's description and emit it directly above the wrapper type, after the variant types.
   Regenerate. The doc test passes, and 24 union types gain a doc comment.
7. Source cleanups, each regenerated:
   * a blank line after every function `emitOutbound` emits, and between a union's `UnmarshalJSON` and `MarshalJSON`;
   * `match := true` instead of `var match bool = true`;
   * check the discriminator decode error and return an error naming the union and the key;
   * (added 2026-10-10, see the deviation below) drop the `value` parameter of `varStmt` in `cmd/generate/internal/emit/ast.go`, now always nil, and
     the trailing `nil` at its callers;
   * use `_, _ok :=` in the defaulted, nullable branch of `emitObjectDefaults`, and add an emit test with a defaulted nullable property that compiles.
   The discriminator test from step 2 passes.
8. Review the regenerated diff. Every changed hunk must belong to one of steps 4 to 7. Record the line counts per file in the execution record.

## Verification

* The step 2 tests failed before steps 4 to 7, with recorded messages, and pass after.
* The step 3 tests pass before and after.
* `TestGeneratedFilesUpToDate` passes, so the committed files match the generator.
* `go test -race ./...` passes at the root and in `cmd/generate`. `go build ./example/...` passes.
* `make pre-add-check` passes, including `golangci-lint` for the host and for `GOOS=windows`.
* `git grep -nE '"encoding/json/(v2|jsontext)"'` finds nothing.
* `go doc -all` of the root package, compared with `main`, differs only by `MarshalJSON` methods on the structs step 5 names.

## Rollout and Rollback

~~All work is on `fix/generated-output-json-v1`.~~ Amended 2026-10-09: commits land on `main`, made by the owner. No tag. A push waits for the owner's explicit ask.

The change alters what the SDK sends for nil required arrays and the error text for an unset union. The release notes for the next module version say so.

Rollback is `git revert` of the step commits on `main`, newest first, made by the owner. Each step regenerates the files it changes, so a revert leaves the generator and the generated files in step.

## Deviation 2026-10-09, failing tests and commit points

Found before execution: step 1 puts a commit point after every step, gated by `make pre-add-check`, which runs `go test`. Step 2's tests must fail on
the current output, so step 2 could never reach its commit point. Its four tests are fixed by four different steps, so steps 3 to 6 would also have
carried failing tests.

Decision: the owner chose to write each step 2 test together with its fix. The order becomes:

* Step 3, the pinning tests, which pass before and after. Commit point 1.
* Step 4 with the unset-union test, step 5 with the `NewSessionRequest` test, step 6 with the union-doc test, step 7 with the discriminator test.
  In each, the test is written and run first and its failure message is recorded, then the generator changes, the files are regenerated, and the
  test passes. Commit points 2 to 5.
* Step 8, the diff review, closes the plan.

Every step 2 test is still seen to fail before its fix. No test is weakened, skipped, or removed.

## Deviation 2026-10-10, required-collection count

Found in step 5: step 5 expected 35 fields in 34 structs, and the MADR's Context and Consequences said the same. The generator emits fills for 34 fields
in 33 structs. The two fields a tag-only count adds, `ElicitationSchema.Properties` and `InitializeResponse.AuthMethods`, are optional and already
encode their schema defaults. Running the counting script on the committed `types_gen.go` at `15549e0` reproduces the record's basis: 36 fields in 35
structs by tag, two of them defaulted.

Decision: the owner chose to correct both records in step 5. The MADR carries an amendment dated 2026-10-10, and step 5's expectation is struck through
above. No file is added to the step's scope, and the implementation is unchanged.

## Deviation 2026-10-10, step 7 lint and the discriminator consequence

Found in step 7, with the code changes made and every test passing:

* `make pre-add-check` failed at `golangci-lint` for the host and for `GOOS=windows`:
  `cmd/generate/internal/emit/ast.go:122:41: varStmt - value always receives nil (unparam)`. The two `var match bool = true` statements were the only
  callers that passed a value; the 18 left pass `nil`. The failure is new, caused by step 7.
* The MADR said a non-string discriminator "decodes today by falling back to key matching". A field scan and probes in scratch copies showed it can hold
  for at most 6 of the 17 discriminated unions.

Decision: the owner chose to drop the `value` parameter from `varStmt` and the trailing `nil` from its callers, which adds `ast.go` and
`dispatch_helpers.go` to step 7's scope with no change to the generated output, and to amend the MADR now. The MADR carries an amendment dated
2026-10-10, and its consequence line is annotated.

## Execution record

### Step 3, pinning tests, 2026-10-09

The owner accepted [0009-MADR-fix-generated-output-on-json-v1.md](0009-MADR-fix-generated-output-on-json-v1.md) on 2026-10-09. Step 3 ran on `main` from a
clean tree at `ac9516f`. The agent did not commit.

Added `json_v1_pin_test.go` in the root package:

* `TestJSONPinEscapesHTML` encodes `"<a & b>"` through a generated union (`TextBlock`) and through a struct with no `MarshalJSON` (`CancelNotification`)
  and requires `\u003ca \u0026 b\u003e` in both.
* `TestJSONPinSortsMapKeys` encodes a `CancelNotification` whose `_meta` has eight keys and requires the exact bytes with the keys sorted.

Both pass on the current tree. The existing golden suite passes, and no file under `testdata/json_golden/` changed.

Two findings while writing them:

* The first draft's expected string lost its `\u003c` escapes when the file was written, and the test failed against correct output. The expectation was
  corrected; the output was right throughout.
* Escaping happens at two levels. A generated `MarshalJSON` escapes, and v1's outer `json.Marshal` re-escapes whatever a `MarshalJSON` returns. Switching
  only the generated `ContentBlock` method to v2 left the bytes escaped, so the first draft, which used only the union, could not fail on that change. The
  struct case was added so the caller's codec alone decides one of the two checks.

The tests were seen to fail on scratch copies:

* A, the test's codec switched to `encoding/json/v2`: both tests failed. `TestJSONPinSortsMapKeys` got the keys in random order;
  `TestJSONPinEscapesHTML` reported `plain struct: got {"sessionId":"<a & b>"}` and `union: got {"text":"<a & b>","type":"text"}`. v2 at the outer call
  also rewrites the output of a v1 `MarshalJSON`.
* B, A plus `ContentBlock`'s generated `MarshalJSON` switched to v2: the same failures.

Gates: `make pre-add-check FILES=json_v1_pin_test.go`: `go-precheck: 1 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` Without names:
`go-precheck: 47 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Commit point 1: the agent stopped here. The owner commits step 3 on `main`.

### Step 4, unset unions, 2026-10-10

The owner committed step 3 as `088d49d`. Step 4 ran on `main` from that clean tree. The agent did not commit.

Added `generated_output_test.go` in the root package with `TestUnsetUnionFailsByName`: encoding the zero value of `RequestPermissionResponse` must fail
with an error containing `RequestPermissionOutcome` and "no variant set". Run before the generator change, it failed:

```text
--- FAIL: TestUnsetUnionFailsByName (0.00s)
    generated_output_test.go:19: error "json: error calling MarshalJSON for type *acp.RequestPermissionOutcome: unexpected end of JSON input" does not contain "no variant set"
```

The union's name was already in v1's wrapper text; the missing part was the reason.

`unionMarshalBody` in `cmd/generate/internal/emit/types.go` now ends with `return nil, errors.New("<Union>: no variant set")`. Regenerated with
`cd cmd/generate && go run .`. Only `types_gen.go` changed: 28 lines `return []byte{}, nil` became `return nil, errors.New("<Union>: no variant set")`,
one per union, and no other line moved. No hand-written code and no generated code calls a `MarshalJSON` method directly, so the only callers are
encoders, which already failed on the empty output.

After the change `TestUnsetUnionFailsByName`, `TestJSONPinEscapesHTML`, and `TestJSONPinSortsMapKeys` pass. `go test ./...` exits 0 at the root and in
`cmd/generate`, including `TestGeneratedFilesUpToDate`. `go build ./example/...` exits 0. No file under `testdata/json_golden/` changed.

Gates: `make pre-add-check` with the three changed Go files named: `go-precheck: 3 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).`
Without names: `go-precheck: 48 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Commit point 2: the agent stopped here. The owner commits step 4 on `main`.

### Step 5, nil required arrays and maps, 2026-10-10

The owner committed step 4 as `15549e0`. Step 5 ran on `main` from that clean tree. The agent did not commit.

Added `TestNilRequiredArrayEncodesEmpty` to `generated_output_test.go`: `NewSessionRequest{Cwd: "/home/<user>/project"}` must encode
`"mcpServers":[]`, and the decoded value must pass `Validate`. Run before the generator change, it failed on both counts:

```text
--- FAIL: TestNilRequiredArrayEncodesEmpty (0.00s)
    generated_output_test.go:31: got {"cwd":"/home/\u003cuser\u003e/project","mcpServers":null}, want it to contain "mcpServers":[]
    generated_output_test.go:38: decoded {"cwd":"/home/\u003cuser\u003e/project","mcpServers":null} fails Validate: mcpServers is required
```

Generator change, all in `cmd/generate/internal/emit/types.go`:

* `appendRequiredFill` records a required property whose Go type is a slice or a map, with a fresh type node for the literal.
* `emitObjectSchema` collects these for fields with no schema default, `emitInlineNestedTypes` for nested inline objects, and `emitInlineUnionStruct`
  for object union variants, including the shared required fields copied into each variant.
* `emitObjectDefaults` takes the fills. Its `MarshalJSON` sets each nil one to an empty literal, after the existing default fills, and is emitted when
  there are defaults or fills. Its `UnmarshalJSON` is still emitted only for defaults.

The schema check found 24 required collection properties in `schema/schema.unstable.json`, none nullable, none defaulted, and none reached through a
`$ref`; shared required fields copied into union variants account for the larger generated count.

Regenerated with `cd cmd/generate && go run .`. Only `types_gen.go` changed, by 333 added lines and no removed ones. A script over the diff found 33 new
`MarshalJSON` methods with 34 fills of the form `if a.F == nil { a.F = T{} }`, and compared them with the fields the committed file tags as slice or map
with no `omitempty`: every tagged field is filled except the two defaulted ones, and nothing else is filled. The count correction is recorded in the
deviation above.

In a scratch copy, `SessionUpdate{Plan: &SessionUpdatePlan{}}` encoded `{"entries":[],"sessionUpdate":"plan"}` and a `McpServer` with an `McpServerStdio`
variant encoded `{"args":[],"command":"c","env":[],"name":"n"}`, so the fills hold through a union's own encoder.

After the change `TestNilRequiredArrayEncodesEmpty`, `TestUnsetUnionFailsByName`, and both pin tests pass. `go test ./...` exits 0 at the root and in
`cmd/generate`, including `TestGeneratedFilesUpToDate`. `go build ./example/...` exits 0. No file under `testdata/json_golden/` changed, so populated
messages encode to the same bytes.

Gates: `make pre-add-check` with the three changed Go files named: `go-precheck: 3 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).`
Without names: `go-precheck: 49 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Commit point 3: the agent stopped here. The owner commits step 5 on `main`.

### Step 6, union docs, 2026-10-10

The owner committed step 5 as `a546dfc` and pushed it. Step 6 ran on `main` from that clean tree. The agent did not commit.

Added `TestUnionDocAboveWrapper` to `generated_output_test.go`. It parses `types_gen.go` with `go/parser` and requires the doc comment of
`type ContentBlock struct` to start with the union's schema description, and the doc comment of `ContentBlockText`, its first variant, not to contain
it. Run before the generator change, both checks failed:

```text
--- FAIL: TestUnionDocAboveWrapper (0.01s)
    generated_output_test.go:61: ContentBlock doc = "", want it to start with "Content blocks represent displayable information"
    generated_output_test.go:64: ContentBlockText doc carries the union's description: "Content blocks represent displayable information in the Agent Client Protocol. ...
```

The union's description was not lost but misplaced: `emitSchemaDef` wrote it before `emitUnion` emitted the variant types, so it merged into the first
variant's doc.

Generator change, in `cmd/generate/internal/emit/types.go`: `emitSchemaDef` computes `isUnion` with the same case order as its switch and skips the
description for a union; `emitUnion` writes `parentDef.Description` directly above `type <Union> struct`, after the variant types.

Regenerated with `cd cmd/generate && go run .`. Only `types_gen.go` changed, by 135 lines in and 101 out, all comment lines. A script over the old and new
file found:

* every non-comment, non-blank line identical, and the comment lines identical as a multiset, so text moved and none was added or dropped;
* 24 union types gained a doc: `AgentResponse`, `AuthMethod`, `ClientResponse`, `ContentBlock`, `CreateElicitationRequest`, `CreateElicitationResponse`,
  `ElicitationContentValue`, `ElicitationPropertySchema`, `ErrorCode`, `IdleStateUpdate`, `McpServer`, `MultiSelectItems`, `PlanUpdateContent`, `RequestId`,
  `RequestPermissionOutcome`, `SessionConfigOption`, `SessionConfigSelectOptions`, `SessionUpdate`, `SetSessionConfigOptionRequest`, `StateUpdate`,
  `ToolCallContent`, `UnstableMcpServer`, `UnstableMessageMcpResponse`, `UnstableNesSuggestion`;
* 23 first variants kept only their own text, the old doc with the union's text removed from its head;
* `RequestIdNull` lost its doc, which was `RequestId`'s text and nothing else.

Not done: `RequestIdNull` stays undocumented. The generator does not write a null variant's description on its type; the description ("The JSON-RPC
`null` request id.") stays on the wrapper's `Null` field, as before. That is outside step 6, which moves the union's description only, and
`golangci-lint` passes with it.

After the change `TestUnionDocAboveWrapper`, `TestNilRequiredArrayEncodesEmpty`, `TestUnsetUnionFailsByName`, and both pin tests pass. `go test ./...`
exits 0 at the root and in `cmd/generate`, including `TestGeneratedFilesUpToDate`. `go build ./example/...` exits 0. No file under
`testdata/json_golden/` changed.

Gates: `make pre-add-check` with the three changed Go files named: `go-precheck: 3 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).`
Without names: `go-precheck: 49 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Commit point 4: the agent stopped here. The owner commits step 6 on `main`.

### Step 7, source cleanups and the discriminator, 2026-10-10

The owner committed step 6 as `7444fea`. Step 7 ran on `main` from that clean tree. The agent did not commit.

Two tests were written and run before the generator changed:

* `TestNonStringDiscriminatorFailsByName` in `generated_output_test.go` decodes `{"type":1,"text":"x"}` into a `ContentBlock` and requires an error
  containing `ContentBlock` and `"type"`. It failed:

  ```text
  --- FAIL: TestNonStringDiscriminatorFailsByName (0.00s)
      generated_output_test.go:54: error "invalid variant payload" does not contain "ContentBlock"
      generated_output_test.go:54: error "invalid variant payload" does not contain "\"type\""
  ```

* `TestDefaultedNullablePropertyCompiles` in the new `cmd/generate/internal/emit/types_test.go` emits a struct whose `mode` property has type
  `["string","null"]` and default `"auto"`, and type-checks the output with `go/types` and the source importer. Schema `1.25.0` has no such property.
  It failed with the defect the MADR named: `options.go:30:3: declared and not used: _rm`.

Generator changes:

* `unionUnmarshalBody` takes the union's name. When the discriminator key is present, its decode error returns
  `fmt.Errorf("<Union>: discriminator \"<key>\": %w", err)`.
* Both `var match bool = true` statements are `match := true`.
* `emitUnion` puts a blank line between `UnmarshalJSON` and `MarshalJSON`. `emitOutbound` puts a blank line before each wrapper it emits, which also
  separates the first wrapper from `handleFunc`; at the end of the file a blank line after the last wrapper would print nothing.
* `emitObjectDefaults` binds the raw value to `_` when the property allows null, and to `_rm` only when the condition reads it.
* Per the deviation above, `varStmt` in `ast.go` lost its always-nil `value` parameter, and its 18 callers in `types.go`, `dispatch.go`, and
  `dispatch_helpers.go` lost the trailing `nil`. Regenerating after that change left all three changed files byte-identical.

Regenerated with `cd cmd/generate && go run .`. `agent_gen.go` (12 lines added), `client_gen.go` (28 added), and `types_gen.go` (172 added, 110 removed)
changed. A script classified every changed line, with none left over:

```text
agent_gen.go     + blank line                         12
client_gen.go    + blank line                         28
types_gen.go     + blank line                         28
types_gen.go     + closing brace                      17
types_gen.go     + discriminator decode, checked      17
types_gen.go     + discriminator error                17
types_gen.go     + match := true                      93
types_gen.go     - discriminator decode, unchecked    17
types_gen.go     - var match bool = true              93
unclassified: 0
```

No `_rm` line changed, because no property in the schema is both defaulted and nullable. Before the change 12, 28, and 29 places in `agent_gen.go`,
`client_gen.go`, and `types_gen.go` had a declaration directly after a closing brace; after it no two functions abut in any generated file.

Probes in scratch copies, at `7444fea` and on the changed tree: `ContentBlock` with `"type":1` failed before with "invalid variant payload" and now fails with
`ContentBlock: discriminator "type": json: cannot unmarshal number into Go value of type string`; a stdio-shaped `McpServer` with `"type":1` decoded before
with no error and now fails with `McpServer: discriminator "type": ...`. The MADR amendment dated 2026-10-10 names the 6 unions where a payload could newly be rejected.

Not done: the RPC interfaces `emitRPCInterfaces` writes (`Agent`, `AgentLoader`, `AgentExperimental`, `Client`, `ClientTerminal`, `ClientExperimental`)
still follow one another with no blank line. They are type declarations, outside step 7's list and the MADR's "a blank line separates generated
functions".

After the change every test in `generated_output_test.go` and `json_v1_pin_test.go` passes, and so does `TestDefaultedNullablePropertyCompiles`.
`go test ./...` exits 0 at the root and in `cmd/generate`, including `TestGeneratedFilesUpToDate`. `go build ./example/...` exits 0. No file under
`testdata/json_golden/` changed.

Gates: before the `varStmt` change, `make pre-add-check` failed as the deviation records. After it, with the nine changed Go files named:
`go-precheck: 9 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` Without names: `go-precheck: 49 file(s) clean (...)`.
`make check-records` and `make markdownlint` exit 0.

Commit point 5: the agent stopped here. The owner commits step 7 on `main`. Step 8, the diff review, closes the plan.

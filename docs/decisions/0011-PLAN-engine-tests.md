---
status: in-progress
date: 2026-10-11
associated-madr: "0011-MADR-harden-code-generator-engine.md"
---

# Implement phase 6 of the generator hardening: tests of the engine

Associated MADR: [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md)

## Goal

Every construct the generator models and every schema it refuses is specified by a small synthetic case that runs without the committed schema. Code
generated from those cases is compiled and run, so the emitters' behaviour is tested and not only their text. In the SDK, every generated type, union
variant, and method runs in a test whose expectations come from the schema files rather than from the generator. Fuzz targets look for decoders that
panic or do not round-trip, and a mutation tool reports the planted faults the tests miss. The five generated SDK files and the exported API do not
change.

This is phase 6 of the MADR, as refined by its amendment "phase 6 facts and decisions", whose decisions D1 to D7 the owner made on 2026-10-11, each
the recommended option. The owner approved the plan on 2026-10-11.

## Scope

In scope:

* `cmd/generate`: `main_test.go`, new `engine_test.go` and `fuzz_test.go`, and `testdata/engine/*.txtar`
* a new package `cmd/generate/internal/enginetest` (the case reader, the runtime-file list, the API listing), used only by tests
* a new command `cmd/generate/internal/mutate`, with its tests
* `cmd/generate/internal/emit` (a builder for `schema_gen_test.go`, and `Render`), `cmd/generate/internal/output/check.go`, and their tests
* unit tests in `cmd/generate/internal/{load,model,emit,util}` for refusals and functions the cases cannot reach
* `cmd/generate/internal/model/schema.go` and `model.go`: the refusals of two lossy shapes (added 2026-10-11, see step 3's deviation)
* at the root: the new generated file `schema_gen_test.go`, and new hand-written `conformance_test.go`, `instances_test.go`, and `fuzz_test.go`
* `Makefile` (`fuzz`, `mutation-engine`, `mutation-generated`), `.github/workflows/ci.yml` (D7)
* `cmd/generate/internal/output/check.go`: one shared source importer (added 2026-10-11, see step 5's deviation)
* `cmd/generate/internal/load/merge.go` and `internal/model`: where each merged definition came from (added 2026-10-11, see step 6's deviation)
* step 7's findings (added 2026-10-11, see step 7's deviation): `internal/model` (flattened members, untyped definitions), `internal/emit`
  (their codecs and `Validate`), and the engine cases that pin them
* step 6a (added by the owner on 2026-10-11): the decoding of optional raw fields in `internal/model` and `internal/emit`, `withRawPath` in
  `helpers.go`, a regression test at the root, the engine case `accept-raw-value`, `README.md`, and `docs/architecture.md`
* `docs/architecture.md`, `RELEASING.md`, and this plan's records

Out of scope:

* fixing a defect these tests find. Each one stops the step and is put to the owner (see step 1); a fix that changes what the SDK sends or accepts
  needs an amendment of the record that decided that behaviour;
* the dead trial decode of five unions, `_meta` numbers as `float64`, `RequestId`'s `Null`, `cancelRequestParams`, and ACP's v2 schemas, which
  [0012-PLAN-schema-fidelity-framework.md](0012-PLAN-schema-fidelity-framework.md) left; this plan's tests pin the trial decode as it is;
* `util.SanitizeComment` (dead, F13), a `.gitignore` entry for leftover temporary files, and the Windows behaviour of `output.Write`;
* vendored fixtures: the Python SDK's `tests/golden/`, the JSON Schema Test Suite;
* gremlins or any other mutation tool, `golang.org/x/tools/txtar`, `testscript`, `rapid`, and every other dependency;
* a coverage or mutation threshold enforced by a check (D6);
* a tag, a push, an agent commit on `main`.

## Grounding

Read on `f2a6a3a`; the probes ran in scratch copies and scratch modules, and nothing in the tree changed. The MADR's phase 6 amendment holds the
measurements; this section names where the code stands.

* **G1. The entry point.** `run` (`main.go:29`) takes `-schema` and `-out`, defaulting both through `findRepoRoot` (`main.go:101`) and `hasSchema`
  (`main.go:118`); it loads with `loadSchema` (`main.go:73`), builds with `model.Build(model.Input{..., Protocol: &model.ACP})` (`main.go:54`), then
  `emit.Render`, `output.Check`, and `output.Write`.
* **G2. Synthetic schemas today.** `writeSchema` (`main_test.go:78`) writes an empty meta and a `$defs` object; `handWritten` (`main_test.go:147`)
  stubs four hand-written names; `recasedUnion` (`main_test.go:159`); `schemaOf` (`model_test.go:14`) and `load.Schema` literals in the `emit` tests.
  `loadSchema`'s reading and parsing go through `load.ReadMeta`, `ReadSchema`, `ReadMetaUnstable`, `ReadSchemaUnstable` (`load.go:148`–`192`) and the
  unexported `parseMeta` and `parseSchema` (`load.go:114`, `:124`).
* **G3. The renderer and the check.** `emit.Render` (`file.go:90`) returns `constants_gen.go`, `types_gen.go`, `agent_gen.go`, `client_gen.go`,
  `helpers_gen.go`, from `constantsFile` (`constants.go:31`), `typesFile` (`types.go:53`), `agentFile` (`dispatch.go:66`), `clientFile`
  (`dispatch.go:105`), and `helpersFile` (`helpers.go:45`). `output.Check` (`check.go:32`) selects hand-written files with `handWritten`
  (`check.go:81`), which keeps `build.Package.GoFiles` only. `generatedFiles` (`main_test.go:24`) lists the five names.
* **G4. The model's constructs.** `Kind` (`model.go:17`): `KindNone`, `KindEnum`, `KindStringConstUnion`, `KindOpenEnum`, `KindUnion`, `KindObject`,
  `KindPrimitive`, `KindEmptyObject`, `KindAny`. `RefKind` (`fields.go:22`): named, pointer, slice, map, raw. `DefaultKind` (`fields.go:72`): none,
  scalar, array, object. `InlineKind` (`fields.go:138`): struct, named, empty. `DiscSource` (`model.go:180`): none, declared, derived. A variant may be
  the catch-all (`Type.CatchAll`) or key-less (`KeyLess`). `Binding` (`methods.go:13`): unknown (protocol), agent, agent optional, agent
  experimental, client, client experimental. `CheckKind` (`validate.go:18`): minLength, minimum, maximum. The keyword registry is `keywords`
  (`keywords.go`).
* **G5. The refusals not run today**, by file and line on `f2a6a3a`: `load/merge.go` 33, 36, 39, 42, 47, 95, 99, 159, 169, 172, 216, 254;
  `load/load.go` 151, 155, 164, 168, 181, 185, 198, 202; `model/protocol.go` 110, 113, 116, 149, 161; `model/validate.go` 119, 228, 237;
  `model/model.go` 407, 524, 564; `emit/file.go` 105, 125, 291; `model/names.go` 30; `model/methods.go` 131; `main.go` 91, 112.
* **G6. The runtime of a synthetic package.** `agent.go`, `client.go`, `connection.go`, `doc.go`, `errors.go`, and `extensions.go` at the root need
  from generated code only `Agent`, `Client`, and `ProtocolMethodCancelRequest`; `helpers.go` needs ACP types. A schema with a `protocolMethods`
  entry needs a definition for it (`CancelRequestNotification`, with `RequestId`).
* **G7. The root tests.** `union_wire_test.go`, `fidelity_test.go`, `generated_output_test.go`, `json_parity_test.go` with `testdata/json_golden`
  (33 files), `defaults_test.go`, `unstable_types_test.go`, and the connection tests. Union and fidelity tests name single types; no test iterates
  over the generated types.
* **G8. Baselines on `f2a6a3a`**, from the amendment: the engine without the committed-schema tests 88.0%; 38 of 93 error sites not run;
  `types_gen.go` 28.9% from the root tests, 39 of 110 union variants decoded; mutation probes 24 of 60 generated-code mutants and 67 of 74 engine
  mutants killed, 12 of those only by committed-schema tests. With `-count=1`, the slowest package of `go test ./...` in `cmd/generate`, the
  `cmd/generate` package itself, took 2.2 to 2.5 s.
  The probe scripts are `harness.py`, `mutate.py`, `mutsum.py`, `refusals.py`, `unioncov.py`, and `rootcov.py` in the session scratchpad.

## Design

### Synthetic cases (D1)

A case is a txtar file in `cmd/generate/testdata/engine/`. The comment before the first file says what the case pins. Its files:

```text
-- meta.json --              required
-- schema.json --            required; schema.unstable.json and meta.unstable.json together, optionally
-- want.err --               a refusal: one expected substring per line
-- want.api --               an accepted schema: the exported API of the generated package
-- behaviour_test.go --      optional, accepted schemas only: package acp tests run by D2
```

`internal/enginetest`:

* `ParseArchive(b []byte) (*Archive, error)`: the comment and an ordered list of files. A line `-- <name> --` starts a file; text before the first is
  the comment; a duplicate name, or a name with a path separator, is an error.
* `RuntimeFiles = []string{"agent.go", "client.go", "connection.go", "doc.go", "errors.go", "extensions.go"}` and `CopyRuntime(repoRoot, dir string) error`.
* `API(pkg *types.Package) string`: one line per exported object of the package scope in name order, `types.ObjectString` with a qualifier that drops
  the package's own name, and for each named type its exported methods (`types.NewMethodSet` of the pointer type) on following lines, indented.

`TestEngineCases` (`engine_test.go`) runs each case as a subtest in parallel: write the schema files into `t.TempDir()`, copy the runtime into
another, then `run([]string{"-schema", ..., "-out", ...})`.

* With `want.err`: `run` must fail with every substring, and the output directory must hold exactly the runtime files, unchanged.
* With `want.api`: `run` must succeed; the six runtime files and the five generated files are type-checked as in `output.Check`, and `API` of the result
  must equal `want.api`. `-update` (a flag of the test binary, as in `cmd/gofmt`) rewrites `want.api` instead; the diff is reviewed like any golden.

`TestRuntimeFilesCoverHandWritten` lists the non-test Go files `build.Default.ImportDir` selects at the root, less the generated names, and requires
them to be exactly `RuntimeFiles` plus `helpers.go`. A new hand-written file fails it with a message that says to add it to one side or the other.

### Compiling and running generated code (D2)

`TestEngineBehaviour` is skipped with `-short`. It needs the go command (`exec.LookPath("go")`); without it the test fails. For each case with a
`behaviour_test.go`, it runs the generator as `TestEngineCases` does, into `<tmp>/<case>/`, and adds the behaviour file. `<tmp>/go.mod` declares
`module enginetest` and the `go` line of the root's `go.mod`. Then, once:

```text
go test -json -count=1 ./...     in <tmp>, with GOWORK=off GOFLAGS=-mod=mod GOTOOLCHAIN=local GOPROXY=off, under t.Context()
```

The JSON events are grouped by package; each case becomes a subtest that fails with that package's output when its package failed or did not build.
On any failure the module is copied to `t.ArtifactDir()`. The subprocess runs with `exec.CommandContext`, never through a shell; gosec's G204 on it
carries a `//nolint:gosec` naming the reason, as `load.go` does for G304.

### The registry and the conformance test (D3)

`emit.Render` returns a sixth file, last: `schema_gen_test.go`, package `acp`, with only data and test doubles:

```go
type schemaType struct {
	Def  string    // the schema definition, e.g. "ContentBlock"
	File string    // "schema.json" or "schema.unstable.json", from the model's provenance
	New  func() any
}

type schemaVariant struct {
	Field string // the union's field, e.g. "Text"
	Value string // the discriminator value, "" for a variant without one
}

type schemaUnion struct {
	Type     string
	Key      string // the discriminator, "" for none
	Variants []schemaVariant
	Unknown  bool   // the union has an Unknown field
	CatchAll string // the catch-all variant's field, or ""
}

type schemaMethod struct {
	Wire         string
	Side         string // "agent", "client", or "protocol"
	Params       func() any
	Result       func() any // nil for a notification
	Notification bool
}

var schemaTypes = []schemaType{...}     // every generated type, by definition name
var schemaUnions = []schemaUnion{...}   // every KindUnion
var schemaMethods = []schemaMethod{...} // every method of the model
```

It also declares `recordingAgent` and `recordingClient`, which implement every generated agent and client interface. Each method sends its
parameters on a channel and returns a zero result. The file carries the generated-code header, so golangci-lint's `generated: lax` exclusion
(`.golangci.yml`) applies to it as to the other five.

`output.Check` type-checks a generated `_test.go` file together with the package files; the error report is unchanged. `generatedFiles` and
`TestGeneratedFilesUpToDate` list six names. `make version` writes the sixth file like the others.

At the root, `instances_test.go` builds JSON instances from the schema files. It reads `schema/schema.json` and `schema/schema.unstable.json` with
`encoding/json` and does not use the registry's knowledge beyond the definition name and file:

* `$ref` within the file, `allOf` merged, `anyOf` and `type` lists with `null` taken as the non-null member;
* `const`, `enum` (first value and last value), `oneOf` (one instance per member);
* `object` with `required` only and with every property; `additionalProperties` with a schema gives one entry; `items` gives one item;
* integers of each `format` at both ends of its range (`int32`, `int64`, `uint16`, `uint32`, `uint64`), and `number` as `0.5`;
* strings at `minLength`, numbers at `minimum` and `maximum`;
* recursion stops at depth 3 with the required-only form.

`conformance_test.go` checks, for every entry of `schemaTypes`, each instance:

* decodes into `New()` without error, and `Validate`, where the type has it, returns nil;
* encodes, and the result equals the instance as JSON (both decoded with `UseNumber`), except that a key the schema gives a `default` may appear with
  that default;
* without each required key, fails to decode (the discriminator key exempt, as decided in
  [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md));
* with a value of the wrong JSON type in a property marked `x-deserialize-default-on-error`, decodes, and the field holds its default or zero value;
* with one invalid item added to an array marked `x-deserialize-skip-invalid-items`, decodes without that item;
* with a value below `minimum`, above `maximum`, or shorter than `minLength`, fails `Validate` with an error naming the key.

For every `schemaUnions` entry with a key: each variant decodes with only its own field set, and a value with an unknown discriminator decodes into
`Unknown` or the catch-all and encodes back equal. For every `schemaMethods` entry: a request or notification sent by a `ClientSideConnection` or
`AgentSideConnection` over `io.Pipe` reaches the recording peer with parameters equal to those sent, and a request returns the zero result.

A difference between what this test expects and what the SDK does is a finding: the step stops and puts it to the owner (step 1).

### Fuzz targets (D4)

* Root, `fuzz_test.go`: `FuzzRoundTrip(f *testing.F)` with arguments `(which uint16, data []byte)`; `t := schemaTypes[int(which)%len(schemaTypes)]`.
  If `json.Unmarshal(data, v)` fails, return. Otherwise `json.Marshal(v)` must succeed, decoding that output into a fresh value must succeed, and
  encoding it again must give the same bytes. `Validate`, where present, must not panic. Seeds: every file of `testdata/json_golden` with the index of
  its type, and every instance of `instances_test.go`.
* `cmd/generate`, `fuzz_test.go`: `FuzzBuild(f *testing.F)` with `(schema []byte)`. If `json.Unmarshal` into `load.Schema` fails, return. If
  `model.Build` with an empty meta and `&model.ACP` succeeds, `emit.Render` must succeed. Nothing may panic. Seeds: the `schema.json` of every case.
* `make fuzz` runs `go test -run '^$' -fuzz '^FuzzRoundTrip$' -fuzztime $(FUZZTIME) .` and the same for `FuzzBuild` and `FuzzExportedName` in their
  packages; `FUZZTIME` defaults to `60s`. A crasher is written by the go command under `testdata/fuzz/`, is committed with the fix, and stays as a seed.

### Mutation tool (D5)

`go run ./internal/mutate` (in `cmd/generate`) with flags:

```text
-dir <module>      the module to test, e.g. . or ../..
-files <glob,...>  files to mutate, relative to -dir; default every non-test .go file of the packages
-pkgs <pattern>    packages to test, default ./...
-n <count>         mutants to run, default all; sampled with -seed
-seed <int>        default 1
-timeout <dur>     per mutant, default 3m
-o <file>          JSON report; default a file in os.TempDir()
```

* **Operators**, on the `go/ast` of each file: `==` and `!=` swapped; `<`, `<=`, `>`, `>=` moved by one; `&&` and `||` swapped; the identifiers
  `true` and `false` swapped; `return nil` in a function whose last result is `error` replaced by `return errors.New("mutant")` (with the import
  added when missing); `continue` and `break` swapped; `+ 1` and `- 1` swapped.
* **Running**: the mutant is printed with `go/format` to a file in a temporary directory, and `go test -overlay=<json> -count=1 -failfast <pkgs>` runs
  in `-dir` with `exec.CommandContext` and the timeout. Exit 0 is `survived`; a build failure is `invalid`; a timeout is `timeout`; any other failure is
  `killed`, with the failed test names from `-json`. The tree is never written.
* **Report**: one record per mutant (file, line, column, operator, original text, outcome, failed tests, seconds), then counts by package and outcome.
  The summary on standard output lists each survivor.
* `make mutation-engine` runs it on `cmd/generate`'s `internal/load`, `internal/model`, `internal/emit`, and `internal/output`;
  `make mutation-generated` runs it with `-dir ../..` on `types_gen.go`, `agent_gen.go`, `client_gen.go`, and `helpers_gen.go` against the root's
  tests. Both take `MUTANTS` and `SEED`. Neither runs in CI or in `make pre-add-check`.

### CI (D7)

In the `validate` job, on Linux, a step after the race detector: `GOEXPERIMENT=nojsonv2 go test -shuffle=on ./...` at the root.

## Implementation Steps

1. **Rules for every step.** Work on `main`. Each step writes its tests first and records them failing on the current tree, or, for a test of
   behaviour that is already right, records it passing and shows it failing on a planted breakage. Every new check is shown failing on a planted
   breakage in a scratch copy, never in the tree. After each step: regenerate with `cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .`;
   compare the five SDK files with `f2a6a3a`'s by script (byte-identical in every step); `git status` shows no generated file other than
   `schema_gen_test.go` from step 6 on. Run `make pre-add-check`, `make check-records`, and `make markdownlint`, record the output in the execution
   record, and stop for the owner's commit. When a test shows the generator or the SDK doing something its MADR does not decide, or contrary to what
   it decides, stop and prompt with the evidence and resolutions; do not loosen the test to pass.
2. **The case runner.** Add `internal/enginetest` (`ParseArchive`, `RuntimeFiles`, `CopyRuntime`, `API`) with tests: archives with a comment, with
   no files, with a duplicate name (error), with `../x` as a name (error); `API` of a package with a type, a method on the pointer, an unexported name
   (absent), and a constant. Add `TestEngineCases`, `TestRuntimeFilesCoverHandWritten`, the `-update` flag, and two cases that restate existing
   tests: `refuse-dangling-ref.txtar` (from `TestRunRefusesBeforeWriting`) and `accept-recased-union.txtar` (from `TestRunGeneratesRecasedUnion`). Plants:
   `ParseArchive` dropping the last file fails its test; `API` without methods fails its test; a `want.api` line changed fails the case; a stray
   hand-written file in a scratch copy of the root fails the runtime test; `run` that writes before refusing (the check moved after `output.Write`, in
   a scratch copy) fails the refusal case's directory check.
3. **Construct cases.** One accepted case per construct in G4, each with a `want.api`:
   * kinds: `enum`, `string-const-union`, `open-enum`, `primitive-string`, `primitive-integer-formats` (`int32`, `int64`, `uint16`, `uint32`, `uint64`),
     `primitive-number`, `primitive-bool`, `empty-object`, `any`, `null-definition`;
   * objects: `required-and-optional`, `nullable`, `default-scalar`, `default-array`, `default-object`, `typed-map`, `raw-value`, `allof-merge`,
     `nested-inline`, `title-only-variant`;
   * unions: `declared-discriminator`, `derived-discriminator`, `catch-all-variant`, `key-less-variant`, `primitive-union`, `inline-variants`
     (struct, named, empty), `union-in-array`;
   * keywords: `default-on-error`, `skip-invalid-items`, `validate-minlength`, `validate-minimum`, `validate-maximum`, `validate-through-union`;
   * methods: `agent-request`, `agent-notification`, `agent-optional` (with the protocol table's `session/load` subject), `agent-experimental`,
     `client-request`, `client-experimental`, `protocol-method`, `prompt-cancellation` (the protocol table's `session/prompt` and `session/cancel`);
   * unstable: `unstable-merge` (an unstable-only type, a promoted shared definition, an unstable method), and `unstable-doc-marking`.
   (Changed 2026-10-11, see step 3's deviation: `allof-merge` and `nested-inline` are refusal cases, with the model refusals they need.)
   Then run `go test -coverpkg=./... -skip '<the nine committed-schema tests>' ./...` in `cmd/generate`; each function of the amendment's below-70%
   list must be above 0, and each one still below 90% is recorded with the reason, or a case is added. Plants: for six cases chosen across the groups,
   a scratch-copy change in the emitter or model that changes that construct's API (a field type, a method dropped, a constant renamed) fails it.
4. **Refusal cases.** One case or unit test per site of G5, and per refusal already tested that has no case, in `testdata/engine/refuse-*.txtar` where
   a schema can reach it:
   * `load.go`: each of the four files missing (the two unstable files only together with their partner), unreadable (a directory in its place), and
     not JSON, through `run`;
   * `merge.go`: nil inputs as unit tests in `internal/load`; a meta version mismatch, an unstable definition whose new name exists in stable, a
     missing renamed definition, a method key mapped to two wire names, and the two `missing definition` paths as cases;
   * `protocol.go`, `validate.go`, `model.go`, `names.go`, `methods.go`: unit tests with a synthetic `Protocol` where the schema alone cannot reach the
     site; cases otherwise;
   * `emit/file.go`: the three wrapping lines through `goFile` failures in `internal/emit` tests;
   * `main.go`: `findRepoRoot` from a directory with no `schema/` above it (`t.Chdir`, not parallel), and the unstable meta and schema present one
     without the other.
   A site no test can reach is recorded with the reason (for example, a nil the loader never returns), and stays. Plants: for each file, one
   refusal turned into `return nil` in a scratch copy fails its case.
5. **Behaviour of generated code.** Add `TestEngineBehaviour` and `behaviour_test.go` to the cases of step 3 that carry behaviour: every union case
   (each variant round-trips; an unknown discriminator is kept and re-encoded equal; a catch-all keeps its `Extra`; the key-less trial decode as it is
   today), every integer format at both ends of its range through a union and through an object, the two lenient keywords, required keys refused,
   defaults on decode and encode, typed maps, raw values keeping their digits, the three `Validate` checks, and each method case answered over
   `io.Pipe` by a hand-written peer. Plants in a scratch copy: the discriminator comparison inverted in `unionUnmarshalBody`; the `Unknown` assignment
   removed; `skip-invalid-items` keeping invalid items; a bound check inverted; a dispatch case decoding into the wrong type. Each fails its case's
   behaviour test, and a deliberately failing behaviour file shows its package's output under the case's subtest and leaves the module in the
   artifact directory. Time `go test ./...` in `cmd/generate`, three runs before and after.
6. **The registry.** Add the `schema_gen_test.go` builder to `internal/emit` with tests on a synthetic model (one type of each kind, one union with a
   key and a catch-all, one method of each side), make `Render` return it sixth, extend `output.Check` to a generated `_test.go` file (test:
   a generated test file referring to an undefined name is refused), update `generatedFiles`, regenerate in the tree, and commit
   `schema_gen_test.go`. Plants: the builder omitting one union fails its test; `Check` ignoring test files fails the new check case. The five SDK files
   stay byte-identical, and the root's exported API is unchanged (the API listing program of the earlier phases).
   6a. **A null raw value is an absent one** (added 2026-10-11; decided in the 2026-10-11 amendment of
   [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md)). Tests first, each shown failing on the tree as it is:
   * `raw_value_test.go` at the root, the reported case: a `tool_call` and a `tool_call_update` whose `rawInput` and `rawOutput` are absent,
     `null`, `{}`, and an object with an integer beyond 2^53. Absent and `null` give `len(v) == 0`, `string(v) == ""`, and are omitted when sent;
     `{}` and the object are kept and sent byte for byte. `WithStartLocations` adds `path` to an input decoded from `null`, and to
     `json.RawMessage("null")` passed in. Each of the other optional raw fields (`data`, `params`, `stopReason`) decodes `null` to empty. A
     required `result` of `null` round-trips as `null`. The file states, with golang/go issue 81050, that `fmt` prints an empty value as `null`
     under the default build, and does not test `fmt`.
   * `accept-raw-value` gains an optional raw property; its behaviour file requires `null` to decode to empty and be omitted, and the required
     `input` of `null` to stay `null`.

   Then the model marks a raw field that is not required, and the emitter gives its type a decoder, or extends the one it has, that stores nothing
   for a JSON `null` in it; `orDefault` and the lenient path do the same. `withRawPath` treats `null` as empty. Regenerate, accept the changed
   listings with `-update`, reading each. `README.md` and `docs/architecture.md` say that an absent or `null` raw value is empty (`len(v) == 0`,
   `string(v) == ""`), that it is not sent, and that `fmt` prints an empty `json.RawMessage` as `null` under Go 1.27.

   Run the root tests also with `GOEXPERIMENT=nojsonv2`. Plants: the decoder keeping `null`; the same applied to a required field; `withRawPath`
   leaving a `null` input unchanged. Each must fail a regression test. Classify the diff of the generated files, which may change only in how
   optional raw fields decode; the exported API must not change.
7. **The conformance test.** Add `instances_test.go` and `conformance_test.go` at the root. A self-test of the instance builder runs on a small inline
   schema: each construct of the list in Design gives the expected instance. Then run the conformance test on the committed schema; record every
   failure as a finding with the type, the instance, and the error, and stop for the owner before changing generated behaviour (step 1). Record the
   root coverage of `types_gen.go`, `agent_gen.go`, `client_gen.go`, and `helpers_gen.go`, and the union variants decoded (`unioncov.py`). Plants in a
   scratch copy of the root: a required-key check removed from one `UnmarshalJSON`; a variant's discriminator value changed; a `Validate` bound
   inverted; a dispatch case removed from `agent_gen.go`. Each fails the conformance test.
8. **Fuzz targets.** Add `FuzzRoundTrip` and `FuzzBuild`, and `make fuzz`. Plain `go test` runs their seeds. Run `make fuzz` once with the default
   `FUZZTIME` and record the executions and any crasher; a crasher is a finding (step 1). Plants: a `MarshalJSON` that drops a field, and a model
   accepting a schema the renderer then fails on, each in a scratch copy, are found by the seeds or within the fuzz time.
9. **Mutation tool.** Add `internal/mutate` with tests: the operators on a fixture file (each operator's sites and its printed mutant); on a temporary
   module with one function and two tests, a mutant that a test catches is `killed`, one no test catches is `survived`, one that does not compile is
   `invalid`, and one that loops is `timeout`; the tree of that module is unchanged after the run. Add `make mutation-engine` and
   `make mutation-generated`. Run ~~the engine in full~~ the engine (amended 2026-10-11: a sample, see step 9's deviation) and the generated files
   with `MUTANTS=300 SEED=1`, record the reports' counts, and classify
   every survivor: a test added (and the mutant rerun as killed), or equivalent with the reason.
10. **CI.** Add the `nojsonv2` step; `actionlint` passes; the next CI run shows the step on Linux.
11. **Docs.** `docs/architecture.md`: "Checks that guard this" names `TestEngineCases`, `TestEngineBehaviour`, the conformance test, the fuzz targets,
    and the mutation targets; the tree lists `testdata/engine/`, `internal/enginetest/`, `internal/mutate/`, and `schema_gen_test.go`.
    `RELEASING.md`: after `make version`, the conformance test can fail on a construct the new schema introduces; extend the instance builder or put
    the finding to the owner; `make fuzz` and `make mutation-*` are optional before a release.
12. **Review and close.** Run the verification below, record each result, set this plan to `complete`, and record in the MADR that the phase 6
    decisions were carried out, or how they changed.

## Verification

* The five SDK files are byte-identical to `f2a6a3a`'s after every step, `TestGeneratedFilesUpToDate` passes, and the root's exported API is unchanged,
  except for step 6a, whose diff of the generated files changes only how optional raw fields decode, and step 7, whose diff changes only the
  types of its deviation, and whose API changes are listed for the release notes.
* An absent or `null` optional raw value decodes to empty and is not sent, under the default build and with `GOEXPERIMENT=nojsonv2` (step 6a).
* Each new test failed first or was shown failing on a plant, as the execution record shows.
* Without the nine committed-schema tests, `go test -coverpkg=./... ./...` in `cmd/generate` covers at least 95% of statements; every site of G5 runs,
  or is recorded with its reason.
* ~~No engine mutant of the full run is killed only by `TestGeneratedFilesUpToDate`, and every survivor is classified.~~ Amended 2026-10-11:
  the engine run is a sample of 300 mutants, seed 1, with `TestGeneratedFilesUpToDate` skipped, and every survivor of it is classified.
* At the root, `types_gen.go` is covered at 85% or more, every union variant is decoded (`unioncov.py`), and every method is dispatched; every
  survivor of the generated-code mutation run is classified.
* `make fuzz` ran for the default time per target, and any crasher is committed as a seed with its fix.
* `go test -race -count=1 ./...` passes at the root and in `cmd/generate`, with and without `-short`; `GOEXPERIMENT=nojsonv2 go test ./...` passes at the
  root; `go build ./example/...` passes; CI passes on all three systems.
* The time of `go test ./...` in `cmd/generate` is recorded before and after.
* `make pre-add-check`, `make check-records`, and `make markdownlint` pass; the identifier scan finds nothing.

## Rollout and Rollback

Commits land on `main`, made by the owner, one per step. No tag; a push waits for the owner's ask. The SDK's exported API does not change.
Step 6a changes what the SDK sends: a `null` in an optional raw field is no longer sent on, which the next minor version's release notes
state. `schema_gen_test.go` is a test file and is not compiled into a dependent's build. A finding that leads to a fix changes this, under its
own decision.

Rollback is `git revert` of the step commits, newest first. Step 6 is reverted together with step 7 and step 8's root target, which use the
registry; every other step leaves the generator and the SDK working alone.

## Execution record

The owner decided D1 to D7 on 2026-10-11 and committed the records as `3b30641`, then approved the plan ("committed, proceed").

### Step 2, the case runner, 2026-10-11

Ran on `main` from `3b30641`, a clean tree. The agent did not commit.

Added `cmd/generate/internal/enginetest` (`archive.go`: `File`, `Archive`, `ParseArchive`, `Format`, `Archive.File`, `Archive.Set`; `api.go`:
`RuntimeFiles`, `CopyRuntime`, `LoadPackage`, `API`) with `archive_test.go` and `api_test.go`, and `cmd/generate/engine_test.go` (`TestEngineCases`,
the `-update` flag, `TestRuntimeFilesCoverHandWritten`) with three cases in `cmd/generate/testdata/engine/`: `refuse-dangling-ref.txtar`,
`accept-recased-union.txtar`, and `refuse-unchecked-output.txtar`.

The tests failed first: `go test -run 'TestEngine|TestRuntimeFiles' . ./internal/enginetest` exited 1 with `no non-test Go files in
.../internal/enginetest` and `undefined: ParseArchive`, `LoadPackage`, `API`, `RuntimeFiles`, `CopyRuntime`. After the package was written,
`TestAPI` failed once on the test's own expectation (it had `const KindA` before `type Kind`; the listing is in name order, as designed), which was
corrected. The first run of `accept-recased-union` printed the listing with an empty `want.api`; `-update` wrote it, and it was read in full before
being kept: the union `U` with `X`, `Y`, and `Unknown`, `UX` and `UY`, `NewUX(n string)` and `NewUY()`, the empty agent and client interfaces, and
`ProtocolMethodCancelRequest`.

Corrections to the design as written, none changing a decision:

* **`API` lists what the generated files declare.** The design said "one line per exported object of the package scope", with each named type's
  method set under it. The package also holds the six runtime files, so that listing would put the whole runtime API into every `want.api`, and it
  would miss the generated methods on runtime types (`ClientSideConnection`'s request methods are declared in `client_gen.go`). `API` takes a filter
  of file names and lists the objects declared in the generated files, a constant with its value, then the exported methods declared there, one per
  line, by receiver and name.
* **A third case.** The plant "`run` writes before refusing" needs a case refused by the type check; `refuse-dangling-ref` is refused earlier, by the
  model. `refuse-unchecked-output.txtar` restates `TestRunRefusesOutputThatDoesNotTypeCheck`: without `protocolMethods`, `connection.go`'s
  `ProtocolMethodCancelRequest` is undefined.
* **The protocol definition of an accepted case.** Every accepted case declares `$/cancel_request` in its meta and the smallest definition for it,
  `{"type": "object", "x-side": "protocol", "x-method": "$/cancel_request"}`, which the model accepts as an empty object; the case comment says why.
* **Lint.** gosec reported G703 on `CopyRuntime`'s `os.WriteFile`; it carries `//nolint:gosec` with the reason, as `load.go` does for G304. A
  `//nolint:gosec` first put on `engine_test.go`'s `os.ReadFile` was reported unused by nolintlint and removed.

Plants, each in a fresh scratch copy of the working tree (`plant.py` with `s2_plants.json` in the session scratchpad); all five failed as planted:

* `ParseArchive` dropping the last file: `--- FAIL: TestParseArchive`.
* `API` without its method lines: `--- FAIL: TestAPI`.
* `func NewUY() U` changed to `func NewUY(n string) U` in `accept-recased-union.txtar`: `--- FAIL: TestEngineCases/accept-recased-union`.
* a `stray.go` at the root: `--- FAIL: TestRuntimeFilesCoverHandWritten`.
* `run` writing before `output.Check`: `--- FAIL: TestEngineCases/refuse-unchecked-output` with `output directory changed: holds [agent.go agent_gen.go
  client.go client_gen.go connection.go constants_gen.go doc.go errors.go extensions.go helpers_gen.go types_gen.go]`.

Results: `go test -count=1 -cover ./internal/enginetest`: 95.9%. Regenerating with `env -u GOPATH -u GOMODCACHE go run .` in `cmd/generate` exited
0, `git diff --quiet f2a6a3a -- '*_gen.go' testdata` exited 0, and `git status` showed only the new files. `make pre-add-check`: `go-precheck: 76
file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` Its per-file checks cover tracked files, so the five new Go files were also
checked by name, `GO_PRECHECK_SKIP_VULN=1 scripts/go-precheck.sh <files>`: `go-precheck: 5 file(s) clean`. `make check-records` and
`make markdownlint` exit 0; the identifier scan found nothing.

Commit point: the agent stopped here. The owner commits step 2 on `main`.

### Step 3 deviation, 2026-10-11: two lossy shapes are refused

The owner committed step 2 as `b70e692`. Step 3's construct cases were written and their listings accepted with `-update` and read in full. Two
showed the generator losing schema information with no error, which step 1's rule sends to the owner:

* `accept-allof-merge`: the definition `"AB": {"allOf": [A, B]}` generated `type AB interface{}`, and `Holder.Ab` became `AB`, so any JSON decodes.
* `accept-nested-inline`: `Outer.inner` became the struct `OuterInner`, but its property `deep`, an inline object with property `y`, became
  `Deep map[string]any`.

A script over `schema/schema.json` and `schema/schema.unstable.json` (`s3_allof.py` in the session scratchpad) found neither shape: every `allOf`
has one schema (80 and 130 on properties of definitions, the rest in union members), none on a definition, and the inline objects with properties
are all direct `anyOf` or `oneOf` members of a definition (50 and 78). (Corrected below: the script did not walk `not`.) Asked to choose between refusing both, representing both, and recording them
for later, the owner asked for the trade-offs, then chose to refuse both now. The MADR's phase 6 amendment records the decision ("Found in step 3
and decided").

Resolution, in this step: a check in `internal/model`, run with `checkRefs` before any type is built, refuses an `allOf` of more than one schema
anywhere, an `allOf` on a definition, and an inline object with properties anywhere but a definition, a property of an object definition, and a
direct union variant. The two cases become `refuse-allof-several.txtar`, `refuse-allof-definition.txtar`, `refuse-nested-inline.txtar`, and
refusal cases for the items and map-value positions. Files added to the step's scope: `cmd/generate/internal/model/schema.go` and its test.

The first version of the check walked every subschema and refused the committed schema:
`model: CreateElicitationRequest.anyOf[2].not.anyOf[0]: an inline object with properties here has no Go type`, failing
`TestGeneratedFilesUpToDate`, `TestCheckAcceptsRepository`, and five model tests on the committed schema. The scan had not walked `not`. The model
reads a `not` only to see that it is there (`isCatchAll`, `model.go:728`), and `unevaluatedProperties` not at all, so their subschemas are never
typed; the check skips both, and the MADR's statement of the scan was corrected the same day.

### Step 3, construct cases, 2026-10-11

Ran on `main` from `b70e692`. The agent did not commit.

**Cases.** 41 accepted cases were added, with the step 2 case making 44: the kinds (`enum`, `string-const-union`, `open-enum`, `primitive-string`,
`primitive-integer-formats`, `primitive-number`, `primitive-bool`, `empty-object`, `any`, `null-definition`); objects (`required-and-optional`,
`nullable`, `default-scalar`, `default-array`, `default-object`, `default-null`, `typed-map`, `raw-value`, `allof-property`, `nested-inline`,
`title-only-variant`); unions (`declared-discriminator`, `derived-discriminator`, `catch-all-variant`, `key-less-variant`,
`key-less-variant-without-keys`, `primitive-union`, `inline-variants`, `union-in-array`, `array-variants`); keywords (`default-on-error`,
`skip-invalid-items`, `validate-minlength`, `validate-minimum`, `validate-maximum`, `validate-through-union`); methods (`agent-request`,
`agent-notification`, `agent-optional`, `client-request`, `protocol-method`, `prompt-cancellation`); and `unstable-merge`. 8 refusal cases came from
the deviation above (`refuse-allof-definition`, `refuse-allof-definition-one`, `refuse-allof-several`, `refuse-nested-inline`,
`refuse-inline-in-items`, `refuse-inline-in-map`, `refuse-inline-in-variant`, `refuse-inline-nullable`), making 10 with step 2's. The cases were
written by `s3_cases.py`, `s3_cases2.py`, and `s3_cases3.py` in the session scratchpad.

Every accepted case failed first with an empty `want.api` (`API differs from want.api`), and every refusal case failed first with `run accepted the
schema`. One accepted case failed for another reason: `agent-optional` was refused with `model: protocol table: agent method "session/load":
capability "loadSession": AgentCapabilities has no property "loadSession"`, the table's capability check; the case declares `AgentCapabilities`
with `loadSession` and says why. The listings were written with `-update` and read in full, 619 lines (`s3_apis.txt` in the scratchpad), before
being kept. Besides the two shapes of the deviation, they show what the cases pin and no decision contradicts.

* A definition of type `null`, as a response without a result is declared, becomes `type Nothing interface{}`.
* An integer without a format is `int64`.
* An inline `anyOf` in a property (`requestId`) is `json.RawMessage`.
* A property with `"default": null` has no default: JSON `null` decodes to a nil `Default`, which the model treats as absent.

Changes against the plan as written, none changing a decision:

* `unstable-doc-marking` was not added. The listing of `API` has no doc comments, so a case cannot pin them; `TestUnstableMarking` in
  `internal/model` covers the marking, and `TestGeneratedFilesUpToDate` the committed docs.
* `agent-experimental` and `client-experimental` are one case, `unstable-merge`. An experimental binding comes only from a method that is in the
  unstable meta alone, so the case carries the unstable files, and it holds an agent method, a client method, a promoted shared definition, and an
  unstable-only type reached through a method.
* `default-null`, `key-less-variant-without-keys`, and `array-variants` were added after the first coverage run, for branches the first set left
  unrun.
* **Test time.** Each accepted case type-checks its package twice: once in `run` (`output.Check`, the phase 5 check) and once in `LoadPackage` for
  the listing. With 44 accepted cases, `go test ./...` in `cmd/generate` took 28 to 33 s, against 2.2 to 2.5 s before. A CPU profile of
  `TestEngineCases` gave 30.3 s to `output.Check` and 29.4 s to `LoadPackage`, most of it the source importer type-checking the standard library
  again for every package. `LoadPackage` now shares one source importer, behind a mutex, across all cases. After that: three runs of `go test ./...`
  took 11.6, 12.1, and 12.5 s; `go test -race ./...` took 45.8 s; and `make pre-add-check` took 69.6 s. The other half, `output.Check` in `run`, is
  the check phase 5 decided, and it is unchanged.
* **Lint.** goconst reported the keyword names that `checkTypable` and `walkRefs` both listed. The two walks now share one list of a node's
  subschemas, `subschemas`, with the keywords as constants. errcheck reported an unchecked type assertion in `enginetest`; the shared importer keeps
  a `types.Importer` and uses `ImportFrom` when it has it.

`TestCheckTypable` in `internal/model` accepts the shapes the model types: an inline variant, a nested object property, an `allOf` of one `$ref` on
a property and on a variant, and an inline object inside a `not`.

**Coverage**, `go test -coverpkg=./... -skip '<the nine committed-schema tests>' ./...` in `cmd/generate`, with `internal/enginetest` left out of the
total: 92.1%, against 88.0% on `f2a6a3a`. Every function on the amendment's list now runs. The listed functions still below 90%, by the
unexecuted lines (`uncovered.py`, which takes each block's highest count across the test binaries):

* `sideOf` (`protocol.go:68`), 50%: the client and default branches need a protocol table with a client interface, and `run` always uses ACP's.
  This is a unit test in step 4.
* `firstPos` (`file.go:383`), 25%: the expression kinds no emitter places first in an element (F15). This is a unit test in step 4.
* `enumConsts` (`types.go:108`), 83.3%: the branch for no values cannot run, since every enum kind has at least one value.
* `defaultKindOf` (`fields.go:315`), 80%: the branch for a nil default cannot run, for the reason above.
* `ReadMetaUnstable` and `ReadSchemaUnstable` (`load.go:175`, `:192`), 77.8%: the read and parse errors are step 4's.

The error sites of G5 are as they were, for step 4. The three new sites in `schema.go` all run.

Plants, each in a fresh scratch copy of the working tree (`plant.py`, `s3_plants.json`). All ten failed as planted.

* An enum constant renamed in `enumConsts`: `--- FAIL: TestEngineCases/accept-enum`.
* `uint32` mapped to `uint64` in `integerFormats`: `--- FAIL: TestEngineCases/accept-required-and-optional`.
* `HasUnknown` forced false: `--- FAIL: TestEngineCases/accept-declared-discriminator`.
* The `minLength` check dropped from `bounds`: `--- FAIL: TestEngineCases/accept-validate-minlength`.
* An unstable agent method bound to `Agent`: `--- FAIL: TestEngineCases/accept-unstable-merge`.
* The protocol table's `GoName` ignored: `--- FAIL: TestEngineCases/accept-prompt-cancellation`.
* `allOf` on a definition allowed: `--- FAIL: TestEngineCases/refuse-allof-definition-one`.
* `allOf` of several allowed: `--- FAIL: TestEngineCases/refuse-allof-several`.
* Every place counted as typed: all five inline-object refusal cases failed with `run accepted the schema` (a second, verbose run listed them).
* `not` walked again: `--- FAIL: TestCheckTypable`.

**Results.** Regenerating exited 0, and `git diff --quiet f2a6a3a -- '*_gen.go' testdata` exited 0. `make pre-add-check`: `go-precheck: 81 file(s)
clean (gofmt, golangci-lint, go vet, go test, govulncheck).` No new Go file is untracked, so the per-file checks covered all of them. `make
check-records` and `make markdownlint` exit 0, and the identifier scan found nothing.

Commit point: the agent stopped here. The owner commits step 3 on `main`.

### Step 4, refusal cases, 2026-10-11

The owner committed step 3 as `fbf15f1`. Step 4 ran on `main` from that clean tree. The agent did not commit.

**Cases.** 17 refusal cases were added (`s4_cases.py` in the session scratchpad), making 27 refusal cases and 71 cases in all:

* `load.go`: `refuse-meta-missing`, `refuse-meta-not-json`, `refuse-schema-missing`, `refuse-schema-not-json`, `refuse-meta-unstable-not-json`, and
  `refuse-schema-unstable-not-json`.
* `main.go`: `refuse-unstable-meta-alone`.
* `merge.go`: `refuse-unstable-version`, `refuse-unstable-key-two-wires`, `refuse-unstable-prefix-taken`, and `refuse-unstable-null-definition`.
* `validate.go`: `refuse-bound-on-named-primitive`, `refuse-minlength-on-integer`, and `refuse-minimum-on-string`.
* `names.go`, `methods.go`, and `model.go`: `refuse-unexported-definition`, `refuse-method-name-empty`, and `refuse-variant-field-name`.

**Unit tests**, for the sites a schema directory cannot reach:

* `internal/load/errors_test.go` (new): `TestReadErrors`, where each of the four files is unreadable because a directory stands in its place.
  `TestMergeRefusesNilInputs` covers the four nil inputs of `MergeStableAndUnstable`, and the two of `buildUnstableDuplicateSet`, called directly.
* `internal/model`: five cases in `TestProtocolTableRefuses`: an interface without a variable name, two interfaces of one binding, an unexported
  interface name, a prompt turn that is also its cancel, and a capability with an empty segment. `TestNestedNameExhausted` takes a nested struct
  and an inline variant whose every candidate name is taken up to the thousandth. `TestSideOf` covers every binding.
* `internal/emit`: a case in `TestRenderErrors` where `format.Node` itself fails. With grouped imports it re-parses what it printed, so an
  unprintable declaration gives `print: format.Node internal error`, which is how phase 5's probe reached it. `TestRenderNamesTheFile` covers
  `Render`'s wrapper, and `TestFirstPos` every expression kind `firstPos` knows, and nil for an index expression.
* `cmd/generate`: `TestFindRepoRoot` checks the root found from the package's directory, and `run` without flags from a temporary directory, with
  `t.Chdir`, so not in parallel.

**A seam in `internal/emit`.** `Render`'s wrapper names the file only when a builder's output fails to render, and a model from `Build` cannot make a
builder fail. `Render` now calls `renderFiles(m, builds)`, and `builds` is a package-level list of `fileBuild`; the test passes its own list, as
phase 5 injected the rename (`writeWith`). The output does not change.

The refusals were already right, so the new tests passed on their first run, all 27 refusal cases among them (`go test -v` listed 27 `PASS`). Step
1 then asks for each to be shown failing on a plant. Twelve plants, one per file, each in a fresh scratch copy (`s4_plants.json`); all twelve failed
the test named:

* `load.go`: a bad `meta.json` accepted fails `refuse-meta-not-json`, and an unreadable `meta.unstable.json` taken as absent fails
  `TestReadErrors`.
* `merge.go`: a version mismatch accepted fails `refuse-unstable-version`.
* `protocol.go`: an interface without a variable accepted fails `TestProtocolTableRefuses`.
* `validate.go`: a minimum on a string accepted fails `refuse-minimum-on-string`.
* `model.go`: a bad wrapper field accepted fails `refuse-variant-field-name`.
* `names.go`: the exported-name check removed from `declare` fails `refuse-unexported-definition`.
* `methods.go`: an empty method name accepted fails `refuse-method-name-empty`.
* `emit/file.go`: `Render` without the file name fails `TestRenderNamesTheFile`, and the print step without `print:` fails `TestRenderErrors`.
* `main.go`: an unstable meta alone accepted fails `refuse-unstable-meta-alone`, and the working directory taken as the root fails
  `TestFindRepoRoot`.

**The sites of G5.** All run except three, kept with the reason (`refusals.py` over the synthetic-only profile):

* `emit/file.go:300` (G5's `:291`), `func %s:` in `layout.decl`: `printedSize` of a function body fails only for a node type `go/format` does not
  support, and a body is always an `*ast.BlockStmt`. Probed in a scratch copy: a malformed statement inside a body, an `AssignStmt` with no left
  side, makes `go/printer` panic with `index out of range` in `(*ast.AssignStmt).Pos`; it does not return an error. Only an engine defect builds such
  a node. This is recorded for the owner; phase 5 removed the engine's own `panic` calls, and this one is in the standard library.
* `load/merge.go:99` and `:254`: a reachable definition that is null is refused first at `:216`, which visits every reachable name before the
  fixpoint (`:254`) and before the copy (`:99`).

Of the step 3 functions sent to this step, `sideOf` and `firstPos` are now at 100%, and the unstable readers' errors run.

**Coverage**, synthetic only, `internal/enginetest` left out: 95.4%, against 92.1% after step 3 and 88.0% on `f2a6a3a`.

**Results.** Regenerating exited 0, and `git diff --quiet f2a6a3a -- '*_gen.go' testdata` exited 0. `make pre-add-check`: `go-precheck: 81 file(s)
clean (gofmt, golangci-lint, go vet, go test, govulncheck).` The untracked `errors_test.go` passed by name: `go-precheck: 1 file(s) clean`. `make
check-records` and `make markdownlint` exit 0, and the identifier scan found nothing.

Commit point: the agent stopped here. The owner commits step 4 on `main`.

### Step 5 deviation, 2026-10-11: test time, and a shared importer in output.Check

The owner committed step 4 as `5ab7e27`. Step 5's runner and 30 behaviour files passed, but `go test ./...` in `cmd/generate` took 34.7, 39.8,
and 42.9 s, 19.6 s with `-short`, and 92.1 s with `-race`, against the MADR's estimate of 1 to 3 s more than 2.2 s. A CPU profile of
`TestEngineBehaviour` gave 10.5 s of 24.2 to `writeBehaviourCase`, 10.4 of it `output.Check`, and the rest to the `go test` subprocess; the
generation was then made parallel, and the runner took 12.3 to 13.4 s. The remaining cost is `output.Check` importing the standard library from
source on every call, which the generator makes once and the tests 74 times.

Resolutions put to the owner: share one importer in `output.Check` (recommended); the same and skip `TestEngineCases` under `-short`; accept the
cost. A scratch copy with the shared importer (`s5_sharedcheck.py`) passed every test in 16.3 and 14.8 s, 5.7 s with `-short`, and 38.3 s with
`-race`. The owner chose the shared importer. The MADR's consequence is corrected and the choice recorded under its phase 6 amendment ("Test time,
decided in step 5"). File added to the step's scope: `cmd/generate/internal/output/check.go`.

### Step 5, behaviour of generated code, 2026-10-11

Ran on `main` from `5ab7e27`. The agent did not commit.

**The runner.** `cmd/generate/behaviour_test.go` (new) holds `TestEngineBehaviour`, as designed, with three additions:

* The cases are generated in parallel subtests (`generate/<case>`) before the one `go test -json -count=1 ./...`.
* A package that passes but ran no top-level test fails its subtest.
* The module's `go` line is read from the root's `go.mod` (`goLine`).

The subprocess runs with `GOWORK=off GOFLAGS=-mod=mod GOTOOLCHAIN=local GOPROXY=off` under `t.Context()`, through `exec.CommandContext` and no
shell. gosec reported nothing on the test file, so it carries no `//nolint`.

**Behaviour files.** 30 cases gained a `behaviour_test.go`, 38 test functions in all, written in the session scratchpad (`s5beh/`) and appended
by `s5_add.py`:

* the unions `declared-discriminator`, `derived-discriminator`, `catch-all-variant`, `key-less-variant`, `key-less-variant-without-keys`,
  `primitive-union`, `inline-variants`, `union-in-array`, `array-variants`, `title-only-variant`, and `recased-union`;
* integer ranges: `int64` at both ends through a union variant and through a nullable field, `uint32` and `uint16` at their maximum and one
  beyond;
* `required-and-optional` and `nullable`; `default-on-error`, `skip-invalid-items`, `default-scalar`, `default-array`, `default-object`,
  `typed-map`, and `raw-value`;
* the four `Validate` cases;
* the methods, over `io.Pipe` with a hand-written peer: `agent-request`, `agent-notification`, `agent-optional` (with and without `AgentLoader`),
  `client-request`, `prompt-cancellation` (session/cancel cancelling the turn's context, the agent's `Cancel`, session/update, and a client whose
  prompt context ends sending session/cancel), and `unstable-merge` (both experimental interfaces, with and without an implementation).

On the first run of three files, one expectation failed: `{"side":"s"}`, a `Shape` without its discriminator, decoded as `Square`. That is the
behaviour [0011-PLAN-wire-fixes.md](0011-PLAN-wire-fixes.md) decided, and `TestAbsentDiscriminator` at the root pins it: a payload without its
discriminator decodes as the one variant whose required keys it carries, else it is kept in `Unknown`. The expectation was wrong, and the file now
tests that rule (`TestShapeAbsentDiscriminator`). The 30 packages then passed, each with its tests run. No behaviour contradicted a decision.
`key-less-variant` pins the trial decode as it is today: a type no variant names goes to `Unknown`, and without a type the key-less variant is
tried by its keys.

**Test time and the shared importer.** See the deviation above. `output.Check` keeps one source importer per process, behind a mutex
(`sharedImporter`), and parses into its file set; the generator's output, its refusals, and its messages do not change.
`TestCheckConcurrently` runs eight checks at once and requires the calls to share one importer.

Plants, each in a fresh scratch copy (`s5_plants.json`, `s5_plants2.json`, `s5_plant_art.json`); all failed as planted:

* The discriminator compared with a wrong value in `discUnmarshalBody`: seven behaviour packages failed, `accept-declared-discriminator` among them.
* The `Unknown` assignment of a discriminated union removed: `--- FAIL: TestEngineBehaviour/accept-declared-discriminator`. The first try did
  not apply: the assignment occurs twice in `types.go`, and `plant.py` refused text that is not unique; anchored on its comment, it applied.
* `skipInvalid` keeping the items that fail: `--- FAIL: TestEngineBehaviour/accept-skip-invalid-items`.
* The `minimum` check inverted: `--- FAIL: TestEngineBehaviour/accept-validate-minimum`.
* Dispatch decoding into a throwaway value instead of `p`: `--- FAIL: TestEngineBehaviour/accept-agent-request`.
* A behaviour expectation broken in `accept-recased-union`: the subtest printed the package's output (`behaviour_test.go:11: NewUX encoded as
  {"n":"n","type":"x"}, <nil>` and `--- FAIL: TestRecasedConstructors`) and `module kept in .../module`. With `-artifacts -outputdir <scratch>`,
  the module, 30 packages, was still there after the run.
* `sharedImporter` without its lock: `WARNING: DATA RACE` under `-race`.
* A new importer for each check: `Check calls do not share one importer`.

**Times**, `go test -count=1` in `cmd/generate` on this host:

| run | `f2a6a3a` | after step 3 | step 5, before the shared importer | step 5, final |
| --- | --- | --- | --- | --- |
| `./...` | 2.2 to 2.5 s | 11.6 to 12.5 s | 34.7 to 42.9 s | 14.4 to 19.8 s |
| `-short ./...` | | | 19.6 s | 7.7 s |
| `-race ./...` | | 45.8 s | 92.1 s | 28.4 s |
| `-run TestEngineBehaviour .` | | | 12.3 to 13.4 s | 17.3 s (the full run's cache was cold) |

**Results.** Regenerating exited 0, and `git diff --quiet f2a6a3a -- '*_gen.go' testdata` exited 0. `make pre-add-check`: `go-precheck: 82 file(s)
clean (gofmt, golangci-lint, go vet, go test, govulncheck).` One run before that failed on staticcheck SA4000: `sharedImporter() !=
sharedImporter()`, which is meant to compare two calls, now compares two variables; the plant was rerun and still fails. The untracked
`behaviour_test.go` passed by name: `go-precheck: 1 file(s) clean`. `make check-records` and `make markdownlint` exit 0, and the identifier scan
found nothing.

Commit point: the agent stopped here. The owner commits step 5 on `main`.

### Step 6 deviation, 2026-10-11: the merge records where each definition came from

The owner committed step 5 as `d96d88f`. The registry is to key every generated type by its schema definition and file, but the model cannot
say where a merged definition came from. `load.Provenance` records only the names the stable schema lacks (`UnstableDefs`, 102). A probe in a
scratch copy (`prov_probe_test.go`) sorted the 280 merged definitions of the committed schema:

* 170 are the stable definition as it is (`schema.json`, same key);
* 8 keep a stable name with the unstable content: `SessionUpdate`, `ClientCapabilities`, `InitializeResponse`, `McpServer`, `McpCapabilities`,
  `PromptResponse`, `AgentCapabilities`, `SessionCapabilities` (`schema.unstable.json`, same key);
* 46 are unstable only (`schema.unstable.json`, same key);
* 56 are copies renamed with the `Unstable` prefix (`schema.unstable.json`, the key without the prefix); for 1 of them that key is also a stable
  definition.

Resolutions put to the owner: the merge records each definition's origin in `Provenance` (recommended); the root test derives origins by rule;
the registry embeds each merged definition. The owner chose the first. `MergeStableAndUnstable` records, for each merged definition that is not
the stable one as it is, the file and key it came from; the model keeps it on each type; the registry writes it. Files added to the step's
scope: `cmd/generate/internal/load/merge.go` and its test, and `cmd/generate/internal/model/model.go` and its test. The MADR is unchanged: it
asserts nothing about provenance.

### Step 6, the registry, 2026-10-11

Ran on `main` from `d96d88f`. The agent did not commit.

**Origins.** `load.Provenance` gained `Origins map[string]Origin`, and the constants `StableSchemaFile` and `UnstableSchemaFile`.
`MergeStableAndUnstable` lists every merged definition that is not the stable one as it is, by pointer: a renamed copy under the key it was copied
from, and any other under its own key. `model.Type` gained `Origin`, the merge's or, by default, `schema.json` and the type's name. Tests:
`TestMergeOrigins`, with one definition of each kind, and `TestTypeOrigin`, with and without provenance. Both failed first, on `undefined: Origin`
and on `Type has no field or method Origin`.

**The builder.** `internal/emit/registry.go` (new): `registryFile` is the sixth entry of `builds`, `schema_gen_test.go`. Against the design:

* `schemaType` also carries `Type`, the Go type, because 56 types are named apart from their key.
* The lists are functions (`schemaTypes()`, `schemaUnions()`, `schemaMethods()`), not package variables. The layout puts the elements of a
  composite literal on their own lines only inside a function body; a package-level literal would print on one line.
* `New`, `Params`, and `Result` are `newOf[T]`, a generic helper emitted in the file, rather than function literals, which `firstPos` cannot place
  (F15).
* A `var` block asserts that `recordingAgent` and `recordingClient` implement every interface, so `output.Check` proves the recorders complete.
  `Result` is omitted for a notification and for a request without a result. A recorder's request method returns `var r T; return r, nil`, which
  holds for every kind of `T`.

`TestRegistry` builds a model with a type of each kind, a union with a catch-all and one with `Unknown`, an agent request and notification, a
client request without a result, a protocol method, and a type with an unstable origin. It reads the rendered file's AST: the types in model
order with their origin, the two unions' keys, variant counts, `Unknown`, and catch-all, the four methods, the recorders' methods, and the
assertions. It failed first, with `Render wrote no schema_gen_test.go`.

**A layout correction.** The first rendering had a blank line before every element of a literal whose elements are literals, and inside a keyed
literal value. `layout.literals` gave an element its line, then the nested literal moved its brace to the next one. Such an element now opens on
its own line, and a keyed one on its key's line. `TestNestedLiteralLayout` failed first, showing the blank lines. No committed generated file
had the shape: a script counted no `{` or `,` followed by a blank line in any of the five. The five stayed byte-identical.

**The check.** `output.Check` already type-checks every file `Render` returns with the package, `_test.go` included.
`TestCheckIncludesGeneratedTestFiles` passed on its first run: a generated test file that uses `missingName` is refused, naming
`schema_gen_test.go` and `undefined: missingName`, and one that uses the package's names is accepted. Its plant is below. A hand-written
`_test.go` is still not part of the check; a name it shares with the registry would show only in `go test`.

**The committed registry.** Regenerating wrote `schema_gen_test.go` at the root, 2926 lines. By script:

* 280 types, of which 110 come from `schema.unstable.json` and 56 are under another key, which matches the merge probe;
* 28 unions with 111 variants;
* 41 methods: 28 agent, 12 client, and 1 protocol;
* `recordingAgent` with 28 methods and `recordingClient` with 12, asserted against `Agent`, `AgentLoader`, `AgentExperimental`, `Client`, and
  `ClientExperimental`;
* no blank line inside a literal.

`go vet .` and `go test .` pass at the root, also with `GOEXPERIMENT=nojsonv2`.

Plants, each in a fresh scratch copy (`s6_plants.json`); all six failed as planted:

* The registry dropping a union with a catch-all: `--- FAIL: TestRegistry`.
* `output.Check` skipping `_test.go` files: `--- FAIL: TestCheckIncludesGeneratedTestFiles`.
* The merge without the origins of copies: `--- FAIL: TestMergeOrigins`.
* The model ignoring origins: `--- FAIL: TestTypeOrigin`.
* The layout as before: `--- FAIL: TestNestedLiteralLayout`.
* The recorders skipping notifications: `TestCheckAcceptsRepository` failed with `recordingAgent does not implement Agent (missing method Cancel)`.

**Results.**

* The five SDK files are byte-identical to `f2a6a3a`'s (`git diff --quiet`).
* The root's exported API, by the API listing program on `f2a6a3a` and on the tree: 2314 lines each, both exit 0, `cmp` identical.
* `go test ./...` in `cmd/generate`: 12.2 s.
* `make pre-add-check`: `go-precheck: 83 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` The four untracked Go files,
  `registry.go`, `registry_test.go`, `origins_test.go`, and `schema_gen_test.go`, passed by name: `go-precheck: 4 file(s) clean`.
* `make check-records` and `make markdownlint` exit 0, and the identifier scan found nothing.

Commit point: the agent stopped here. The owner commits step 6 on `main`.

### Scope added 2026-10-11: step 6a, raw values

The owner committed step 6 as `87f3db7`, then added to this plan's scope a regression that the type checks did not catch: empty raw tool input
now shows as the literal `null`, and a regression test must guard it. Traced on `87f3db7` (scratch probes at `633281b^` and `87f3db7`), the
facts are in the 2026-10-11 amendment of [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md).

The owner first chose an SDK type, `RawJSON`, from three fixes. Not committed, that choice was then reopened: the owner asked for a solution in
line with idiomatic Go, ACP's documentation, the other SDKs, and other Go projects. A probe in a scratch module (`reprprobe`) compared, under both
builds of `encoding/json`, `json.RawMessage` as it is and with `null` read as absent, `any`, `any` decoded with `UseNumber`, `*json.RawMessage`,
and a named type; a research report covered ACP's v1 and v2 documentation and Rust types, the TypeScript, Python, and original Go SDKs, the MCP Go
SDKs, Kubernetes, protobuf, the Anthropic and OpenAI SDKs, oapi-codegen, and golang/go issues 81050, 40495, and 11937. Three options were put to
the owner: `any` with `json.Number`; `json.RawMessage` with `null` read as absent and the printing documented (recommended); the SDK type. The owner
chose the second. Step 6a is added before step 7; scope, verification, and rollout are amended above. The plan awaits the owner's approval of
step 6a.

### Step 6a, a null raw value is an absent one, 2026-10-11

The owner committed the records as `09118e3` and approved step 6a ("committed, proceed"). It ran on `main` from that clean tree. The agent did not
commit.

**Where null was kept.** Of the 15 optional raw fields, 9 decode through `orDefault[json.RawMessage]` (`Error.data` and `rawInput` and `rawOutput`
on `ToolCall`, `ToolCallUpdate`, `SessionUpdateToolCall`, and `SessionToolCallUpdate`). The other 6 decode straight into the alias:
`AgentNotification`, `AgentRequest`, `ClientNotification`, `ClientRequest` `params`, `IdleStateUpdateNone.stopReason`, and
`UnstableMcpError.data`. All 15 types already had a generated `UnmarshalJSON`. A struct whose only special field is an optional raw one would not,
because `emitObjectCodec` emitted a decoder only for required keys, defaults, lenient fields, or a catch-all.

**Tests first, on the tree as it was:**

* `raw_value_test.go` (new, at the root):
  * `TestRawToolInputNullIsAbsent`: `ToolCall`, `ToolCallUpdate`, `tool_call`, and `tool_call_update` with `rawInput` and `rawOutput` absent,
    `null`, `{}`, and `{"n":9007199254740993}`, 16 cases.
  * `TestOptionalRawFieldsNullIsAbsent`: the 7 other optional raw fields.
  * `TestRequiredRawResultKeepsNull`: `AgentResult.result`.
  * `TestStartLocationsAfterNullInput`: an input decoded from `null`, and `WithStartRawInput(json.RawMessage("null"))`.

  The file's comment cites golang/go issue 81050 and says why `fmt` is not tested. It failed on every `null` case:
  * `rawInput decoded as "null" (len 4)` and `rawInput sent as null, want it omitted`, in 4 of the 16;
  * `null decoded as "null", want empty` for all 7 fields;
  * `rawInput = "null", want {"path":"/p"}` twice.

  The absent, `{}`, and integer cases and the required `result` passed: those were right before.
* `accept-raw-value` gained an optional raw `extra` on `Holder`, and a type `Loose` whose only property is an optional raw one.
  `TestRawNullIsAbsent` in its behaviour file failed: `decoded input "null" and extra "null"`, `encoded as {"extra":null,"input":null}`, and
  `Loose decoded as "null"`.
* `TestNullIsAbsent` in `internal/model` failed to build: `f.NullIsAbsent undefined`.

**The change:**

* `internal/model/fields.go`: `Field.NullIsAbsent`, set by `objectFields` and `plainFields` for a raw field that is not required. Objects, nested
  structs, and inline variants are all covered.
* `internal/emit`: `emitObjectCodec` collects those fields and emits a decoder when there is one. `emitObjectUnmarshal` assigns
  `a.F = absentIfNull(a.F)` after the decode and before defaults. `decode.go` emits `absentIfNull`, a fourth helper written once, which returns
  nil for the bytes `null`.
* `helpers.go`: `withRawPath` treats `null` as empty. It uses the existing `jsonNull` constant, which goconst asked for on the first gate run.
* `README.md` ("Wire encoding") and `docs/architecture.md`:
  * an absent or `null` optional raw value is empty, tested with `len(v) == 0`, read with `string(v)`, and not sent;
  * a required one keeps `null`;
  * `fmt` prints an empty `json.RawMessage` as `null` under Go 1.27's default build, with a link to golang/go issue 81050;
  * `absentIfNull` among the helpers, and `raw_value_test.go` among the checks.

**The generated diff**, against `09118e3`: `types_gen.go` only, 24 lines added, none removed. That is one `a.F = absentIfNull(a.F)` for each of
the 15 fields and the helper with its comment. `constants_gen.go`, `agent_gen.go`, `client_gen.go`, `helpers_gen.go`, and `schema_gen_test.go`
did not change. The root's exported API, by the API listing program, is identical to `f2a6a3a`'s (2314 lines, `cmp`). The only engine listing that
changed is `accept-raw-value`'s, read after `-update`: `Holder` gained `Extra`, and `Loose` gained an `UnmarshalJSON`.

**Results.**

* `go test -count=1 .` at the root passed by default and with `GOEXPERIMENT=nojsonv2`; the four regression tests passed, 27 `PASS` lines with
  their subtests.
* `go build ./example/...` passed, and `go test ./...` in `cmd/generate` passed.

Plants, each in a fresh scratch copy (`s6a_plants.json`):

* `absentIfNull` in `types_gen.go` keeping `null`: `--- FAIL: TestRawToolInputNullIsAbsent`.
* The model applying the rule to required fields: `TestEngineBehaviour/accept-raw-value` failed with `decoded input "" and extra ""`, the
  required `input` emptied. `TestNullIsAbsent` failed too.
* The emitter dropping the assignments: `TestEngineBehaviour/accept-raw-value` failed with `Loose decoded as "null"`. Its first version did not
  compile (`declared and not used: fld`), which is the plant's fault and not a test result; rewritten to compile, it failed as planted.
* `withRawPath` leaving a `null` input unchanged: `--- FAIL: TestStartLocationsAfterNullInput`, also after the `jsonNull` change.

**For the next minor version's release notes:** a `null` in an optional raw field (`rawInput`, `rawOutput`, `data`, `params`, `stopReason`) now
decodes to an empty `json.RawMessage` and is not sent on, as ACP v1 defines it; a required `result` keeps `null`. The exported API does not change.

`make pre-add-check`: `go-precheck: 87 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` The untracked `raw_value_test.go`
passed by name: `go-precheck: 1 file(s) clean`. `make check-records` and `make markdownlint` exit 0, and the identifier scan found nothing.

Commit point: the agent stopped here. The owner commits step 6a on `main`.

### Step 7 deviation, 2026-10-11: what the conformance test found

The owner committed step 6a as `6786efe`. Step 7's `instances_test.go` and `conformance_test.go` were written and run on the committed schema.
Their first runs failed on gaps in the test itself, each corrected in the test and recorded below with step 7's record, and on three behaviours of
the SDK that no record decides, which step 1's rule sends to the owner:

* `ElicitationFormMode` and `ElicitationUrlMode` drop their own properties, and `CreateElicitationRequest`'s `form` and `url` variants drop the
  scope members (`sessionId`, `toolCallId`, `requestId`): 6 `TestConformanceRoundTrip` subtests and the 2 `TestConformanceRequired` subtests.
* `ExtRequest`, `ExtResponse`, and `ExtNotification`, `type X any`, send `9007199254740993` as `9007199254740992`.
* An optional map that arrives as `{}` is not sent: 129 subtests failed on that alone.

The owner chose the recommended resolution of each, recorded in the MADRs: the members of such an object are flattened into its struct, and a
variant that is an `allOf` of it gets them too (0011-MADR, "Found in step 7 and decided"); an untyped definition is an alias of
`json.RawMessage` (0012-MADR, amendment "an untyped definition is raw JSON"); an empty optional map is the same as an absent one (0011-MADR).

Added to step 7, before its conformance test can pass:

* `internal/model`: a definition with properties of its own and members that are all references becomes an object with flattened members, and
  an inline variant whose `allOf` base is one gets them; a definition the model gives no Go type becomes an alias of `json.RawMessage`.
* `internal/emit`: the struct's members as pointer fields, `json:"-"`; `UnmarshalJSON` choosing the member by its required keys; `MarshalJSON`
  writing the member's keys into the struct's object; `Validate` requiring exactly one member; and the alias.
* engine cases for both shapes, with behaviour tests; the comparer of the conformance test accepts an empty map left out.

The exported API gains fields (`RequestedSchema` on `ElicitationFormMode`, `ElicitationId` and `Url` on `ElicitationUrlMode`, `Session` and
`Request` on `CreateElicitationForm` and `CreateElicitationUrl`), and `ElicitationFormMode` and `ElicitationUrlMode` stop being listed as unions;
the three `Ext` types become aliases. The release notes list each. Verification is amended accordingly: the generated diff of step 7 may change
in these types only.

A fourth finding, 2026-10-11: with the first three fixed, the union test was extended to every union, discriminated or not
(`TestConformanceVariants`). It failed on `ErrorCode` alone: each of its nine members' instances decoded into `ParseError`
(`member MethodNotFound: -32601 decoded into [ParseError]`, and so for each). A scratch probe confirmed `-32700`, `-32601`, `-32000`, and `7` all
set `ParseError`. Resolutions put to the owner: an open integer enum (recommended), a decoder that matches a variant only by its `const`, and
recording it for later. The owner chose the integer enum, recorded in the 0011-MADR's step 7 findings. Added to step 7: `internal/model` (the
kind, its classification, its refusals), `internal/emit` (the type and its constants), and an accepted and a refused engine case; the exported API
changes for `ErrorCode` and its nine variant types, listed for the release notes.

### Step 7, the conformance test, 2026-10-11

Ran on `main` from `6786efe`. The agent did not commit.

**The instance builder.** `instances_test.go` (new) reads `schema/schema.json` and `schema/schema.unstable.json` with `UseNumber` and builds
instances of a definition from its file and key alone. It follows `$ref`, merges an `allOf` of one schema, takes a union's members with the
union's own properties merged in (the member's own win), and gives `const`, the first or last `enum` value, objects with required or all
properties to depth 3, one item and one map entry, integers at both ends of their format and bounds, strings at `minLength`, and raw JSON with an
integer beyond 2^53. Three shapes per node, or per member of a union. `TestInstanceBuilder` checks it on a schema of its own.

**The conformance test.** `conformance_test.go` (new), over `schema_gen_test.go`:

* `TestConformanceRoundTrip`: every instance decodes, validates, and encodes equal.
* `TestConformanceRequired`: each required key removed fails decoding; a discriminator is exempt.
* `TestConformanceLenient`: a value of the wrong kind in an `x-deserialize-default-on-error` property decodes as absent; an invalid item in an
  `x-deserialize-skip-invalid-items` array is dropped.
* `TestConformanceBounds`: a value just outside `minimum`, `maximum`, or `minLength`, that the Go type holds, fails `Validate` naming its key.
* `TestConformanceUnions`: in a discriminated union, each member decodes into its variant, and an unknown value is kept in `Unknown` or the
  catch-all and sent back unchanged.
* `TestConformanceVariants`: added during the step. Every union, discriminated or not, decodes each member's instance into that member's variant.
* `TestConformanceWrongKinds`: added during the step. A value of a kind the schema does not accept is refused, as a whole value or as a property
  that is neither lenient nor raw, and the error names the type.
* `TestConformanceMethods`: each agent and client method over `io.Pipe` reaches the recording peer with equal parameters.

The comparer is schema-aware. The output must hold every key of the input with an equal value. It may leave out a key whose input is `null`, the
schema's default, or an empty map. It may add a key only when that key carries the schema's default, or is a struct value of such keys.

On the committed schema the final run checks:

* 280 types, 1248 instances, 464 required-key removals, 540 lenient properties, and 39 out-of-bounds values;
* 17 discriminated unions, and all 26 unions member by member;
* 40 methods.

`go test -v` listed 280 `PASS` subtests for each per-type test, 17 for unions, and 40 for methods.

**Gaps in the test, found by its own failures and corrected, none an SDK change:**

* `setFields` called `Len` on a pointer and panicked, which stopped the test binary.
* A lenient unsigned field holds no `-1`: it falls back to absent, so decoding "succeeded". A bound case now counts only when the value is
  held.
* A discriminator is not lenient: one that is not a string is an error (`TestNonStringDiscriminatorFailsByName`). It is now exempt from the
  lenient check.
* The recording peer returns the zero result, and for `CreateElicitationResponse`, `RequestPermissionOutcome`, and
  `UnstableMessageMcpResponse` that zero value is a union with no variant set, which does not encode. A request now passes when it succeeds, or,
  when its zero result does not encode, when it fails with that error. The plan had said "a request returns the zero result", which cannot hold
  for these three.
* The union's own properties were merged over a member's, so `IdleStateUpdate`'s `stopReason` lost each member's `const`. The member's own now
  wins, pinned by a case in `TestInstanceBuilder`.
* The comparer looked up a property in the first member that declared it; it now picks the member whose `const` values the input carries. For a
  union of arrays, it tries each member's item schema.
* A key whose input is `null` or the schema's default may be left out, as the schema defines it: 31 default `false` booleans, and
  `IdleStateUpdate`'s `none` member with `"stopReason": null`.
* A raw field (`json.RawMessage`, treatment 3) takes any kind: the envelopes' `params` and `result`, and `stopReason`. The wrong-kind check skips
  them by reflection on the decoded value.

**Findings and decisions**, as the deviation entries above record:

* The elicitation shapes were flattened: `internal/model` gained `Member`, `Type.Members`, `Inline.Members`, `flatMembers`, `checkMembers`, and
  `Validation.Members`. `internal/emit` writes the member fields, `memberDecode`, the member write in `MarshalJSON` through a new helper,
  `mergeObject`, and the member count in `Validate`.
* Untyped definitions became aliases of `json.RawMessage` (`emitSchemaDef`'s default case). The null-typed definitions of the engine cases
  (`accept-null-definition`, `accept-client-request`'s `WriteResponse`) follow, as the rule "constrains no type, or is only null" says.
* Empty optional maps are accepted as absent, in the comparer (`omitOK`).
* `ErrorCode` became an open integer enum: `KindIntegerEnum`, `isIntegerEnum`, and `buildIntegerEnum` in `internal/model`, and the type and
  constants with `integerLit` in `internal/emit`.

Engine cases added:

* `accept-flattened-members`, whose behaviour file covers decoding by required keys, the merged encoding, a payload no member fits, a struct with
  no member set, and two members set;
* `accept-integer-enum`, with a behaviour file;
* `refuse-integer-enum-untitled` and `refuse-integer-enum-formats`;
* a behaviour file for `accept-any`, for digits.

Each failed first: `Mode has no field or method Schema`, `AskForm has no field or method Request`,
`sent as {"f":1,"n":9007199254740992}`, `cannot convert 7 (untyped int constant) to type Code`, and `run accepted the schema` twice.
`TestFlattenedMembers` in `internal/model` covers the members, their validation, an `allOf` variant, and a field collision.

**Root fixtures corrected.** Three root tests built or decoded a form elicitation with no scope, which the type could not hold before. The schema
requires one, through the `anyOf` of two scopes that each require a key:

* `TestCreateElicitationRequest_MessageIsRequired`, which now also requires a form with no scope to fail `Validate`;
* `TestValidateSchemaConstraints`' "form with an empty message";
* `TestAbsentDiscriminator`'s "CreateElicitationRequest form".

Each now carries a session scope.

**Coverage of the generated code by the root tests**, against `f2a6a3a`:

| file | `f2a6a3a` | now |
| --- | --- | --- |
| `types_gen.go` | 28.9% | 86.7% |
| `agent_gen.go` | 19.3% | 64.7% |
| `client_gen.go` | 23.0% | 53.4% |
| `helpers_gen.go` | 18.2% | 18.2% |

No generated `UnmarshalJSON`, `MarshalJSON`, or `Validate` is left at 0%; before, 224, 34, and 117 were. Every union variant and flattened member
decodes in a test: `TestConformanceVariants` for the 26 unions, and the 8 member assignments of the four flattened types run (`unioncov.py` counts
only `u.<Field> =` assignments, so it shows those four as missing). `helpers_gen.go`'s constructors are not covered by this step.

**The generated diff**, against `6786efe`: `types_gen.go` +267 and −521 lines, and `schema_gen_test.go` −76 lines (`ErrorCode`,
`ElicitationFormMode`, and `ElicitationUrlMode` leave `schemaUnions`). By hunk, the changes are in:

* `ElicitationFormMode`, `ElicitationUrlMode`, `CreateElicitationForm`, `CreateElicitationUrl`, and `CreateElicitationRequest`'s `Validate`;
* `ErrorCode` and its variant types;
* the `Validate` of `Error`, `ErrorStopReason`, `IdleStateUpdateError`, `AgentError`, and `ClientError`;
* the three `Ext` types;
* the new `mergeObject`.

`constants_gen.go`, `agent_gen.go`, `client_gen.go`, and `helpers_gen.go` did not change.

**Exported API changes, for the release notes** (the API listing program, `f2a6a3a` against the tree: 2314 and 2308 lines, 50 lines differ):

* Added:
  * the fields `CreateElicitationForm.Session` and `.Request`, `CreateElicitationUrl.Session` and `.Request`,
    `ElicitationFormMode.RequestedSchema`, `ElicitationUrlMode.ElicitationId`, and `ElicitationUrlMode.Url`;
  * `MarshalJSON` on `CreateElicitationForm` and `CreateElicitationUrl`, and `Validate` on `CreateElicitationUrl`;
  * the constants `ErrorCodeParseError`, `ErrorCodeInvalidRequest`, `ErrorCodeMethodNotFound`, `ErrorCodeInvalidParams`, `ErrorCodeInternalError`,
    `ErrorCodeRequestCancelled`, `ErrorCodeAuthenticationRequired`, and `ErrorCodeResourceNotFound`.
* Changed:
  * `ErrorCode` from a struct of nine pointer fields to `int32`;
  * `ExtRequest`, `ExtResponse`, and `ExtNotification` from `any` to aliases of `json.RawMessage`.
* Removed:
  * the nine `ErrorCode*` variant types, and `ErrorCode`'s `MarshalJSON`, `UnmarshalJSON`, and `Validate`;
  * `Validate` on `Error`, `ErrorStopReason`, and `IdleStateUpdateError`, which checked only the code's variant.

**Results.**

* `go test -count=1 .` at the root passed, also with `GOEXPERIMENT=nojsonv2` and with `-race`; it takes 2.0 s.
* `go build ./example/...` and `go test ./...` in `cmd/generate` passed.

Plants, each in a fresh scratch copy (`s7_plants.json`):

* A required-key check removed from `ToolCall`'s decoder fails `TestConformanceRequired/ToolCall`.
* `ContentBlock`'s `"text"` case changed fails `TestConformanceVariants` and `TestConformanceUnions`.
* `Notice`'s `minLength` check disabled fails `TestConformanceBounds/Notice`.
* `session/new`'s dispatch case removed fails `TestConformanceMethods/agent_session/new`.
* `outputByteLimit` tagged `json:"-"` fails the round trip with `outputByteLimit: not sent`.
* `ElicitationFormMode`'s first member chosen without its keys fails `TestConformanceRoundTrip/ElicitationFormMode`.
* `mergeObject` dropping the member fails the round trips of the flattened types.
* `ExtRequest` back to `any` fails `TestConformanceRoundTrip/ExtRequest`.
* The builder merging the union's properties over the member's fails `TestInstanceBuilder`.
* No flattened members in the model fails `TestEngineBehaviour`.
* No integer enum fails `TestEngineCases/accept-integer-enum`.
* An untitled integer `const` accepted fails `TestEngineCases/refuse-integer-enum-untitled`: the constant then collides with the type's name, a
  different refusal. The plant's first version did not compile (`declared and not used: i`); rewritten, it failed as planted.
* The emitter writing `any` for an untyped definition fails `TestEngineBehaviour/accept-any`.

Lint: gocognit reported `comparer.diff` at 53 (above 50). It was split into `diffObject`, `diffArray`, and `itemSchemas`, and two plants were
rerun after the split and still failed.

`make pre-add-check`: `go-precheck: 88 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` The untracked `instances_test.go`
and `conformance_test.go` passed by name: `go-precheck: 2 file(s) clean`. `make check-records` and `make markdownlint` exit 0, and the identifier
scan found nothing.

Commit point: the agent stopped here. The owner commits step 7 on `main`.

### Step 8 deviation, 2026-10-11: key names match as encoding/json matches them

The owner committed step 7 as `0e32255`. `fuzz_test.go` at the root and in `cmd/generate`, and `make fuzz`, were written; their seeds passed
(1281 and 74). The first `make fuzz` failed in `FuzzRoundTrip` after 3 s (the invalid byte and U+FFFD are shown escaped):

```text
fuzz_test.go:91: IdleStateUpdate: {"stopReAson":"\x820000"} encodes as {"stopReason":"\x820000"}, and that as {"stopReason":"\ufffd0000"}
Failing input written to testdata/fuzz/FuzzRoundTrip/6abafd94758f7256
```

Scratch probes traced it to two behaviours, both pre-existing, recorded in the 0011-MADR's "Found in step 8 and decided": the generated
decoders look keys up by exact name while `encoding/json` matches the struct fields under case folding, so a wrong-case `stopReason` decodes as
`IdleStateUpdateNone` and is sent on as a stop reason; and a raw value keeps invalid UTF-8 where a string replaces it.

Resolutions put to the owner: match every key as `encoding/json` does (recommended), exact names everywhere, or the discriminator alone; and
for invalid UTF-8, record it (recommended) or replace it on decode. The owner chose the recommended two.

Added to step 8, before its fuzz run can pass:

* `internal/emit`: a generated helper, `foldKeys`, called on each map of keys the generated decoders read, with the names that decoder reads;
  the array-item match of a union decodes each item through it; and a refusal when two of those names differ only in case. ~~The refusal
  lives in `internal/emit`.~~ Amended 2026-10-11: `FuzzBuild` failed on the seed of `refuse-folded-names`, since the model accepted a schema
  the renderer then refused, so the refusal is `checkFoldedKeys` in `internal/model`; the emitter keeps its check as a backstop, which
  `FuzzBuild` would report if the two ever disagree.
* `types_gen.go`, regenerated: the helper and its calls.
* an accepted engine case, `accept-folded-keys`, with a behaviour test, and a refused one, `refuse-folded-names`.
* a root test, `TestFoldKeysMatchesEncodingJSON`, that compares the helper's choice with `encoding/json`'s on keys in other cases, Unicode
  folds, and duplicates.
* the crasher, `testdata/fuzz/FuzzRoundTrip/6abafd94758f7256`, committed as a seed (D4).
* `README.md`: both behaviours. The 0012-MADR is amended.

The wire changes for payloads whose keys differ from the schema's only in case; the release notes list it. Verification is amended: the
generated diff of step 8 changes only the decoders' key lookups.

A second finding, 2026-10-11: with the first fixed, `make fuzz` ran `FuzzRoundTrip` for its 60 s and `FuzzBuild` failed after 7 s:

```text
fuzz_test.go:41: the model accepted {"$defs":{"A000":{"AnYOf":[{"title":"00000000","tYpe":"integer","formAt":"int32","Const":""}]}}},
but rendering failed: types_gen.go: format: 9:1: expected operand, found ')' (and 4 more errors)
Failing input written to testdata/fuzz/FuzzBuild/2e4f0f4e77985ba7
```

The integer enum of step 7 took any `const` of an integer member (the loader matches the schema's keys in any case, as `encoding/json`
does). Resolutions put to the owner: refuse a `const` that is not an integer of the format (recommended), or classify such a definition as no
integer enum. The owner chose the refusal, recorded in the 0011-MADR's step 8 findings. Added to step 8: `integerConst` in `internal/model`,
which also writes the value as a plain integer; the engine case `refuse-integer-enum-const`; `TestIntegerEnumConsts`; and the crasher as a seed.

### Step 8, fuzz targets, 2026-10-11

Ran on `main` from `0e32255`. The agent did not commit.

**The targets.**

* `fuzz_test.go` (new, root): `FuzzRoundTrip(which uint16, data []byte)`, as D4 states, over `schemaTypes()`. Its seeds are the 33 files of
  `testdata/json_golden`, each with its type through a table of file prefixes (a file the table does not name stops the target), and the 1248
  instances of `instances_test.go`: 1281 seeds.
* `cmd/generate/fuzz_test.go` (new): `FuzzBuild(schema []byte)`, as D4 states, with an empty meta. Its seeds are the `schema.json` of every
  engine case: 74 of 75 at first, since `refuse-schema-missing` has none, and 77 of 78 at the end. `engineCases` now takes a `testing.TB`, so
  a fuzz target can list the cases.
* `Makefile`: `make fuzz` runs `FuzzRoundTrip`, `FuzzBuild`, and `FuzzExportedName` for `FUZZTIME`, `60s` by default.

**A gap in the plan's property, corrected in the test, not an SDK change.** The second run of `FuzzRoundTrip` failed on the input `null` for
`IdleStateUpdate`: `null decoded but does not encode: ... IdleStateUpdate: no variant set`. Decoding JSON `null` into a union leaves it as it
was (README), and a union with no variant set does not encode
([0009-MADR-fix-generated-output-on-json-v1.md](0009-MADR-fix-generated-output-on-json-v1.md)). JSON `null` decodes to no value, so D4's
"decoded, it must encode" does not apply to it; the target now skips `null`, and the input is kept as a seed,
`testdata/fuzz/FuzzRoundTrip/0b27a31d61b773cb`.

**Findings and decisions**, as the deviation above records:

* The generated decoders read keys under folding as `encoding/json` does: `foldKeys` and `lastKey`, emitted into `types_gen.go`; a call after
  every map of keys a decoder reads (`objectKeys` folds the required keys; a struct with defaults, members, or extra keys folds those; a
  discriminated union folds its discriminator and the keys that fit a variant; a union matched by keys folds them; each array item is folded);
  and `checkFoldedKeys` in `internal/model`, with the emitter's `foldNames` as a backstop.
* An integer enum's `const` must be an integer of its format, below 2^53 in magnitude: `integerConst` in `internal/model`.
* Invalid UTF-8 in a raw value is recorded in the README and the 0012-MADR, not changed.

New tests, each run first on the tree without its fix:

* `accept-folded-keys`, with a behaviour test of required keys, a default, a discriminator with its key-less and catch-all variants, a
  catch-all's extra keys, a flattened member, array items, and two matching keys in either order. Before the fix, all four tests failed,
  among them `{"ID":"a"} ... missing required property "id"`, `{"id":"a","Count":5} decoded as {Count:3 Id:a}` (a default overwrote a value
  given in another case, a third effect of the same cause), `{"StopReason":"end_turn"} decoded as ... None`, and `Options: no variant
  matches`. A test bug was corrected: a case with the Kelvin sign in `count`, which has no `k`, expected a match that neither
  `encoding/json` nor `foldKeys` makes.
* `refuse-folded-names`: before, `run accepted the schema`.
* `TestFoldKeysMatchesEncodingJSON` (new, `fold_keys_test.go`) compares `foldKeys` with `encoding/json`'s struct decode on 14 objects:
  keys in other cases, the Kelvin sign and the long s, an escaped key, `_` and `-` (not matched), several matches in either order, and
  `null`. Before: `undefined: foldKeys`. It passes under the default build and with `GOEXPERIMENT=nojsonv2`.
* `refuse-integer-enum-const` and `TestIntegerEnumConsts` (5 refused, 3 accepted values): before, the case failed with the format error, and
  the test with `<nil>` for each refusal and `-2.147483648e+09` for `-2147483648`. A first version refused only magnitudes above 2^53, and
  `9007199254740993` passed as 2^53; the bound is now 2^53 or more.

**The generated diff**, against `0e32255`: `types_gen.go` +113 and −5 lines: the two helpers, `objectKeys`' doc and its call, 43
`foldKeys` calls in the decoders (2 of them on array items), and the array-item loop of the one union matched by array items.
`schema_gen_test.go` and the other generated files did not change. The exported API is unchanged: the API listing program gives 2308 identical lines for `0e32255` and the tree.

**`make fuzz`**, the run after the last fix, each target for 60 s on 10 workers:

| target | baseline corpus | executions | new interesting |
| --- | --- | --- | --- |
| `FuzzRoundTrip` | 1580 | 5263815 | 133 |
| `FuzzBuild` | 146 | 1697625 | 94 |
| `FuzzExportedName` | 55 | 6644214 | 1 |

No crasher. The three crashers of the earlier runs are committed as seeds, and `go test` runs them: `testdata/fuzz/FuzzRoundTrip/6abafd94758f7256`
(the wrong-case discriminator), `testdata/fuzz/FuzzRoundTrip/0b27a31d61b773cb` (`null`), and `cmd/generate/testdata/fuzz/FuzzBuild/2e4f0f4e77985ba7`
(the string `const`). The go command's own corpus stays in its cache, outside the tree.

Plants, each in a fresh scratch copy (`s8_plants.json`):

* `AvailableCommandsUpdate`'s `MarshalJSON` dropping `availableCommands` fails `FuzzRoundTrip` on its seeds: `decoded, but its encoding {} does
  not: AvailableCommandsUpdate: missing required property "availableCommands"`.
* The model accepting keys that differ only in case fails `FuzzBuild` on the seed of `refuse-folded-names`: `the model accepted ...`.
* `foldKeys` doing nothing fails `TestFoldKeysMatchesEncodingJSON` and the seed `FuzzRoundTrip/6abafd94758f7256`.
* `lastKey` taking the first match fails `TestFoldKeysMatchesEncodingJSON`.
* The emitter folding no names fails `TestEngineBehaviour/accept-folded-keys` (`TestFoldedDiscriminator`).
* `objectKeys` not folding fails it with `{"ID":"a"} decoded as {Count:0 Id:}`.
* An integer `const` not checked fails the seed `FuzzBuild/2e4f0f4e77985ba7`.
* The target decoding `null` as a value fails the seed `FuzzRoundTrip/0b27a31d61b773cb`.

The plant helper stopped once on the invalid UTF-8 of a test's output; it now decodes output with replacement, and the plants were rerun.

**Results.**

* `go test -count=1 .` at the root passed, also with `-race` and with `GOEXPERIMENT=nojsonv2`; it takes 3.8 s, from 2.0 s, the 1283 seeds of
  `FuzzRoundTrip` included.
* `go test -count=1 ./...` in `cmd/generate` passed in 14.4 s, and passed with `-short`. `go build ./example/...` passed.
* `make pre-add-check`: `go-precheck: 90 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` The untracked `fuzz_test.go`,
  `fold_keys_test.go`, and `cmd/generate/fuzz_test.go` passed by name: `go-precheck: 3 file(s) clean`. `make check-records` and
  `make markdownlint` exit 0, and the identifier scan of the 23 changed and new files found nothing.

Commit point: the agent stopped here. The owner commits step 8 on `main`, with the three seed files under `testdata/fuzz/` and
`cmd/generate/testdata/fuzz/`.

### Step 9 deviation, 2026-10-11: the engine run skips the byte comparison, and runs mutants in parallel

The owner committed step 8 as `77d053b`. `internal/mutate` was written to D5's flags, with its tests. Two probes on the engine:

* One mutant (seed 1) took 17.3 s, `internal/model/fields.go:393:56`, `!=` to `==`; its report names one failed test,
  `TestGeneratedFilesUpToDate`. With `-failfast`, a report names the first failing test only, so D6's "no engine mutant killed only by
  `TestGeneratedFilesUpToDate`" cannot be read from it. Rerun without `-failfast` in a scratch copy, the same mutant failed that test alone: it
  is such a mutant.
* The engine has 1065 sites; at 15 to 17 s a mutant, the full run is about 5 hours serially. Three generated-code mutants took 3.1 to 3.2 s each,
  of 2576 sites.

Resolutions put to the owner: for D6, skip the byte comparison in the engine run (recommended), turn `-failfast` off, or rerun selected mutants;
for the time, parallel workers (recommended), a serial run in the background, or a sample. The owner chose the recommended two:

* `-skip <regexp>`, passed to `go test -skip`; `make mutation-engine` skips `TestGeneratedFilesUpToDate`. A mutant only that test kills then
  survives, and D6's check is part of classifying the survivors. `-failfast` stays.
* `-j <count>`, mutants run at once, each with its own overlay and mutant file; default 1. A test requires `-j 2` to give the outcomes `-j 1`
  gives. `make mutation-engine` takes `JOBS`, set from a timed sample ~~before the full run~~.

Timed samples of the engine: 8 mutants (seed 3) took 147 s with `-j 1` and 80 s with `-j 4`; 24 mutants (seed 5) took 291 s with `-j 4` and
258 s with `-j 6`. Each pair gave the same outcomes, and no mutant timed out. 9 of the 32 survived. At 10.8 s a mutant, the full run would take
about 3.2 hours and leave some 300 survivors to classify.

Amended 2026-10-11, asked by the owner: the engine run is a sample, `MUTANTS=300 SEED=1`, with `JOBS=6`, as the 0011-MADR's "D6 amended in step
9" records. Step 9 and Verification are annotated.

A finding, 2026-10-11: `TestConformanceAbsentDiscriminator`, written for survivors of the generated-code run, failed on `SessionConfigOption`
and `CreateElicitationRequest`. Without the discriminator, the generated code picks a variant by the variant's own required keys, not those
the union requires of every variant, so a payload without `id` decodes as `Boolean` and is refused naming `id`, where the README read as
`Unknown`. A third failure, `SetSessionConfigOptionRequest`, was the test's: the key-less `ValueId` variant refuses a `value` of the wrong
kind, which a rule of key names cannot see. Resolutions put to the owner: keep the behaviour and clarify the README (recommended), or count the
union's keys in the fit. The owner kept it; the 0012-MADR is amended, the README states the rule, and the test follows it.

The engine sample, 2026-10-11 (`JOBS=6`, 4631 s): 235 killed, 54 survived, 4 invalid, 7 timeout. The 4 invalid mutants are refused by
`go vet`'s `bools` check (`suspect and: pt == schemaTypeArray && pt == schemaTypeObject`), which `go test` runs. The 7 timeouts each took 186 to
195 s, at the 3 min test timeout under six parallel runs; rerun alone in a scratch copy, 5 were killed and 2 survived, in 16 to 29 s. The
sample is therefore 240 killed, 56 survived, 4 invalid.

Each survivor was regenerated in a scratch copy against the committed schema (`s9_outdiff.py`): 10 change only the generated text's layout,
7 change generated code that only the byte comparison catches, and 39 change nothing. Asked, the owner classified the 10 as layout only, recorded
in the 0011-MADR's "D6 amended in step 9".

A finding, 2026-10-11: the engine cases `accept-discriminated-shapes` and `accept-union-matching`, written for survivors, failed on the
unmutated generator: `{"kind":"start","at":9007199254740993} decoded as {None:<nil> Start:<nil> Stop:<nil> Other:0x...}, want start`, and
`{"k":"v"} decoded as {Loose:0x... Keyed:<nil>}, <nil>; want Keyed`. `buildVariant` read a variant's facts without following a direct `$ref`.
Resolutions put to the owner: follow the `$ref` (recommended), or refuse such a member. The owner chose to follow it, recorded in the
0011-MADR's "Found in step 9 and decided"; `TestVariantFacts`, which pinned the old reading, follows. Added to step 9: `buildVariant` in
`internal/model`. The committed output does not change.

A correction, 2026-10-11: two step 9 amendments, "D6 amended in step 9" in the 0011-MADR and "the variant a payload without its
discriminator fits" in the 0012-MADR, were written in place of each MADR's "## More Information" heading instead of before it. Both headings
are restored; no text under them changed.

A deviation, 2026-10-11: the scratch scripts that reran and regenerated survivors swapped an operator's text, which Go re-parses with other
grouping for `&&` and `||`: `internal/model/protocol.go:272:45` survived the tool and was killed by the text rerun. Asked, the owner chose a
flag in the tool, `-only file:line:col,...`, which runs just those sites as the tool builds them; `TestOnly` covers it. Every rerun below uses
it. A process slip is recorded too: to show the test of a mutant's time failing, the fixed line of `internal/mutate/exec.go` was reverted in
the working tree, run, and restored, where step 1 asks for a scratch copy; the file was new and uncommitted.

### Step 9, the mutation tool, 2026-10-11

Ran on `main` from `77d053b`. The agent did not commit.

**The tool.** `cmd/generate/internal/mutate` (new): `main.go` (flags, file list, sample, the parallel runner, the report and summary),
`sites.go` (the operators on `go/ast`, each site with the change that makes its mutant), and `exec.go` (the overlay, `go test`, and the
outcome). D5's flags, and `-skip`, `-j`, and `-only` from the deviations above. `make mutation-engine` and `make mutation-generated` take
`MUTANTS`, `SEED`, and `JOBS`; the engine run skips `TestGeneratedFilesUpToDate`.

Its tests: `TestSites` lists the 20 sites of a fixture (`testdata/ops.go`) and checks each mutant changes only its line; `TestRun` runs a
module of one file and two tests and requires killed (`TestSum`), survived, invalid, and timeout, the same outcomes with `-j 2`, the outcomes
`-skip TestSum` gives, the report in file order, and the module unchanged; `TestOnly` runs two named sites and refuses one that is not a site.
They failed first on `undefined: sites`, `unknown field jobs`, and `unknown field only`. Plants in scratch copies (`s9_plants.json`), each
failing its test: the mutant is the original; a function literal's result not seen; a labelled branch made a site; a build failure counted
as a kill; a test timeout counted as a kill; `-skip` not passed; results out of order with `-j`; no overlay. Three first runs did not fail as
planted and were corrected: the fixture gained a function literal with a non-error result, `checkRun` gained the order check, and the build
plant, which did not compile, was rewritten.

**The generated-code sample**, `make mutation-generated MUTANTS=300 SEED=1 JOBS=4`: 2576 sites; 263 killed, 37 survived, none invalid or
timed out, in 335 s.

**The engine sample**, `make mutation-engine MUTANTS=300 SEED=1 JOBS=6`: 1065 sites; 235 killed, 54 survived, 4 invalid, 7 timeout, in
4631 s. The 4 invalid are refused by `go vet`'s `bools` check. The 7 timeouts ran one at a time: 5 killed, 2 survived. 240 killed, 56
survived, 4 invalid.

**Tests added for survivors.** At the root:

* `TestConformanceAbsentDiscriminator`: the 17 discriminated unions, each variant without its discriminator, as is and without each other
  required key;
* `TestConformanceNullUnion`: `null` into each union;
* `TestCatchAllWithoutExtraKeys`;
* `TestConformanceReplies`: each request with a result brings back the peer's result, an instance of its type, and its error;
* `TestConformanceNotificationErrors`: each notification's handler error is logged by the connection that received it.

For the last two, the recording peers of `schema_gen_test.go` gained a `reply` hook (`internal/emit/registry.go`), the only change in
the generated files: `schema_gen_test.go` +126 and −2 lines.

In `cmd/generate`, the engine cases:

* new: `accept-union-matching`, `accept-discriminated-shapes`, `accept-null-member`, and `accept-default-map`;
* extended: `accept-folded-keys` (a union of objects sharing a required key, after a variant without required keys) and
  `accept-declared-discriminator` (a constructor sets the discriminator in the Go value).

Unit tests:

* `internal/load`: `merge_edges_test.go`, three tests;
* `internal/model`: `edges_test.go`, ten tests; three refusal cases in `TestProtocolTableRefuses`; `TestVariantFacts` amended;
* `internal/output`: `report_test.go`, two tests;
* `internal/emit`: `TestRegistryEdges`.

Three first versions did not kill their mutant, and were strengthened: the shared-key union now lists a variant without required keys
first, the secondary error in `report_test.go` now has an error after it, and the allOf test covers an array.

**Rerun with `-only` after the tests**, the generated survivors against the root (`-j 2`, 127 s), the engine survivors with
`TestGeneratedFilesUpToDate` skipped (`-j 4`, 712 s, and the three strengthened at once).

Generated code, killed now (14):

* `client_gen.go:103:49` eq-neq (`TestConformanceNotificationErrors`)
* `client_gen.go:155:10` eq-neq (`TestConformanceReplies`)
* `client_gen.go:168:10` eq-neq (`TestConformanceReplies`)
* `types_gen.go:12697:12` true-false (`TestConformanceAbsentDiscriminator`)
* `types_gen.go:12709:12` true-false (`TestConformanceAbsentDiscriminator`)
* `types_gen.go:13762:12` boundary (`TestCatchAllWithoutExtraKeys`)
* `types_gen.go:14836:12` true-false (`TestConformanceAbsentDiscriminator`)
* `types_gen.go:16406:10` return-error (`TestConformanceNullUnion`)
* `types_gen.go:17281:12` true-false (`TestConformanceAbsentDiscriminator`)
* `types_gen.go:17326:12` true-false (`TestConformanceAbsentDiscriminator`)
* `types_gen.go:4467:30` and-or (`TestConformanceAbsentDiscriminator`)
* `types_gen.go:627:10` return-error (`TestConformanceNullUnion`)
* `types_gen.go:8959:28` and-or (`TestConformanceAbsentDiscriminator`)
* `types_gen.go:9000:14` eq-neq (`TestConformanceAbsentDiscriminator`)

Generated code, equivalent (23):

* Only the error text changes; the payload lacks a key the variant requires, and either path refuses it: `types_gen.go:339:13`, `types_gen.go:1578:13`, `types_gen.go:6888:12`.
* The trial decode that follows takes the same variant: `types_gen.go:656:12`, `types_gen.go:10813:13`.
* Dead: the trial decode reaches a variant only when its required keys are absent, which its decoder refuses; an array decode with an empty body: `types_gen.go:975:11`, `types_gen.go:3792:29`.
* Only the order of keys can change, which JSON does not fix: `types_gen.go:3239:16`.
* Unreachable: a variant with no keys of its own besides the discriminator fits every payload without it, so this variant is never the only fit: `types_gen.go:3358:41`, `types_gen.go:3376:11`, `types_gen.go:4497:11`, `types_gen.go:9723:11`, `types_gen.go:12768:11`, `types_gen.go:12771:41`, `types_gen.go:12810:11`, `types_gen.go:12817:11`, `types_gen.go:12859:11`.
* `content` makes the three message-chunk variants and `SessionInfoUpdate` fit as well, so the count never falls to one: `types_gen.go:12719:12`.
* The default `false` equals the zero value: `types_gen.go:6645:11`, `types_gen.go:9180:11`.
* `foldKeys` with one matching key finds it again; `orDefault` fails to decode an empty value either way; `skipInvalid`'s caller assigns nothing on either path: `types_gen.go:18658:16`, `types_gen.go:18697:14`, `types_gen.go:18719:15`.

Engine, killed now (39); `model.go` sites after line 617 are named at their place in the sample, before `buildVariant` gained 8 lines:

* `internal/emit/decode.go:255:4` continue-break (`TestEngineBehaviour`)
* `internal/emit/helpers.go:27:16` eq-neq (`TestEngineBehaviour`)
* `internal/emit/registry.go:120:16` boundary (`TestRegistryEdges`)
* `internal/emit/registry.go:61:4` continue-break (`TestRegistryEdges`)
* `internal/emit/types.go:590:39` eq-neq (`TestEngineBehaviour`)
* `internal/emit/types.go:628:40` true-false (`TestEngineBehaviour`)
* `internal/emit/types.go:636:39` true-false (`TestEngineBehaviour`)
* `internal/emit/types.go:948:4` continue-break (`TestEngineBehaviour`)
* `internal/load/merge.go:307:18` and-or (`TestPromoteSkipsNullStableDefinition`)
* `internal/load/merge.go:325:4` continue-break (`TestPromoteContinuesPastMissingDefinition`)
* `internal/load/merge.go:364:10` eq-neq (`TestEnsureRefsCopiesUnstableDependency`)
* `internal/model/fields.go:237:9` eq-neq (`TestAllOfOfOneInlineSchema`)
* `internal/model/fields.go:305:5` continue-break (`TestNullableWithNullFirst`)
* `internal/model/fields.go:393:56` eq-neq (`TestEngineBehaviour`)
* `internal/model/methods.go:71:38` and-or (`TestHalfLinkedDefinitionIgnored, TestRegistryEdges`)
* `internal/model/model.go:409:54` and-or (`TestIntegerEnumMembers`)
* `internal/model/model.go:409:69` and-or (`TestIntegerEnumMembers`)
* `internal/model/model.go:520:21` and-or (`TestReferenceWithPropertiesIsNotNested`)
* `internal/model/model.go:561:4` continue-break (`TestEngineBehaviour`)
* `internal/model/model.go:585:4` continue-break (`TestEngineBehaviour`)
* `internal/model/model.go:880:4` continue-break (`TestEngineBehaviour`)
* `internal/model/model.go:895:28` boundary (`TestEngineCases`)
* `internal/model/model.go:924:9` eq-neq (`TestEngineBehaviour`)
* `internal/model/model.go:932:57` and-or (`TestEngineBehaviour`)
* `internal/model/protocol.go:242:14` and-or (`TestProtocolTableRefuses`)
* `internal/model/protocol.go:266:10` true-false (`TestProtocolTableRefuses`)
* `internal/model/protocol.go:272:45` and-or (`TestProtocolTableRefuses`)
* `internal/model/protocol.go:357:17` eq-neq (`TestMethodNameCollisionNamesInterface`)
* `internal/model/schema.go:181:41` and-or (`TestOpenStringEnumMembers`)
* `internal/model/schema.go:181:61` and-or (`TestOpenStringEnumMembers`)
* `internal/model/schema.go:87:28` and-or (`TestExpandAllOfKeepsOwnUnion`)
* `internal/model/validate.go:124:100` eq-neq (`TestBoundOnNamedPrimitiveRefused`)
* `internal/model/validate.go:124:74` eq-neq (`TestBoundOnNamedPrimitiveRefused`)
* `internal/model/validate.go:124:81` and-or (`TestBoundOnNamedPrimitiveRefused, TestEngineCases`)
* `internal/model/validate.go:211:12` true-false (`TestNestedArraysWithoutValidation`)
* `internal/model/validate.go:211:19` return-error (`TestNestedArraysWithoutValidation`)
* `internal/output/check.go:157:13` return-error (`TestGoVersionWithoutGoLine`)
* `internal/output/check.go:167:52` boundary (`TestReportGroupsSecondaryErrors`)
* `internal/output/check.go:169:4` continue-break (`TestReportGroupsSecondaryErrors`)

Engine, not killed (17):

* Layout only (the owner's class): blank lines, doc comments, constant grouping, and literal line breaks; `file.go:359:21` is reachable only through an embedded field, which no emitter writes: `internal/emit/file.go:62:69`, `internal/emit/file.go:359:21`, `internal/emit/file.go:383:86`, `internal/emit/file.go:390:73`, `internal/emit/file.go:392:6`, `internal/emit/types.go:137:12`, `internal/emit/types.go:161:19`, `internal/emit/types.go:200:18`, `internal/model/methods.go:175:5`.
* Equivalent: `printedSize` never fails (step 4 records the branch unreachable): `internal/emit/file.go:300:26`.
* Equivalent: the decode into `arr` has an empty body when no array variant is matched by keys: `internal/emit/types.go:643:16`.
* Equivalent: the caller returns on the error before it reads the found flag: `internal/load/load.go:202:15`.
* Equivalent: the definition is already marked; scanning its other references adds nothing: `internal/load/merge.go:292:6`.
* Equivalent: `checkRefs` refuses a `$ref` that is not local or does not resolve before `flatMembers` runs: `internal/model/model.go:742:10`.
* Equivalent: the schema is never nil, and a part that is not a local `$ref` names no definition: `internal/model/schema.go:44:61`.
* Equivalent: merged properties are read only by their length, so an empty map and nil are the same: `internal/model/schema.go:53:31`.
* Not tested: the mutant differs only when `Write` fails, which cannot be made to happen portably without a seam: `internal/output/write.go:79:9`.

**Results.**

* The five SDK files are unchanged from `77d053b`; `schema_gen_test.go` gained the `reply` hook. The exported API does not change.
* `go test -count=1 .` at the root passes, also with `-race` and with `GOEXPERIMENT=nojsonv2`, in 2.0 s. `go test -count=1 ./...` in
  `cmd/generate` passes in 23.1 s, from 14.4 s, `internal/mutate`'s 22.4 s run beside the rest. `go build ./example/...` passes.
* `make pre-add-check`: `go-precheck: 93 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` The seven new Go files passed
  by name: `go-precheck: 7 file(s) clean`. Lint first reported nine issues in the tool (unchecked errors, two unused directives, repeated
  strings), fixed before this run.
* `make check-records` and `make markdownlint` exit 0, and the identifier scan of the 26 changed and new files found nothing.

Commit point: the agent stopped here. The owner commits step 9 on `main`.

### Step 10, CI, 2026-10-11

Ran on `main` from `41b8840`, which the owner committed and pushed. The agent did not commit.

`.github/workflows/ci.yml` gained a step in the `validate` job, after the race detector and on Linux only: `root tests with
GOEXPERIMENT=nojsonv2`, which runs `go test -shuffle=on ./...` at the root with that experiment in its environment (D7).

* `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` exits 0 on the tree.
* The step's command passes locally: `GOEXPERIMENT=nojsonv2 go test -shuffle=on ./...` exits 0.
* Plants in scratch copies (`s10_plants.json`):
  * A root test that holds only under the v2-backed codec, `fmt.Sprint(json.RawMessage(nil)) == "null"`, fails under the step's command
    (`--- FAIL: TestCodecProbe`) and passes without the experiment, so the step catches a codec difference the default run does not.
  * `runner.oss` in the step's condition fails `actionlint`:
    `property "oss" is not defined in object type {arch: string; ...}`. The first run of this plant stopped on `actionlint`'s search for
    a git repository, which a scratch copy is not; given the workflow file by path, it failed as planted.
* `make check-records` and `make markdownlint` exit 0, and the identifier scan of the changed files found nothing. No Go file changed.

Open: the plan's "the next CI run shows the step on Linux" waits for the owner's push of this step.

Commit point: the agent stopped here. The owner commits step 10 on `main`.

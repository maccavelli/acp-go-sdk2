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
* at the root: the new generated file `schema_gen_test.go`, and new hand-written `conformance_test.go`, `instances_test.go`, and `fuzz_test.go`
* `Makefile` (`fuzz`, `mutation-engine`, `mutation-generated`), `.github/workflows/ci.yml` (D7)
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
   `make mutation-generated`. Run the engine in full and the generated files with `MUTANTS=300 SEED=1`, record the reports' counts, and classify
   every survivor: a test added (and the mutant rerun as killed), or equivalent with the reason.
10. **CI.** Add the `nojsonv2` step; `actionlint` passes; the next CI run shows the step on Linux.
11. **Docs.** `docs/architecture.md`: "Checks that guard this" names `TestEngineCases`, `TestEngineBehaviour`, the conformance test, the fuzz targets,
    and the mutation targets; the tree lists `testdata/engine/`, `internal/enginetest/`, `internal/mutate/`, and `schema_gen_test.go`.
    `RELEASING.md`: after `make version`, the conformance test can fail on a construct the new schema introduces; extend the instance builder or put
    the finding to the owner; `make fuzz` and `make mutation-*` are optional before a release.
12. **Review and close.** Run the verification below, record each result, set this plan to `complete`, and record in the MADR that the phase 6
    decisions were carried out, or how they changed.

## Verification

* The five SDK files are byte-identical to `f2a6a3a`'s after every step, `TestGeneratedFilesUpToDate` passes, and the root's exported API is unchanged.
* Each new test failed first or was shown failing on a plant, as the execution record shows.
* Without the nine committed-schema tests, `go test -coverpkg=./... ./...` in `cmd/generate` covers at least 95% of statements; every site of G5 runs,
  or is recorded with its reason.
* No engine mutant of the full run is killed only by `TestGeneratedFilesUpToDate`, and every survivor is classified.
* At the root, `types_gen.go` is covered at 85% or more, every union variant is decoded (`unioncov.py`), and every method is dispatched; every
  survivor of the generated-code mutation run is classified.
* `make fuzz` ran for the default time per target, and any crasher is committed as a seed with its fix.
* `go test -race -count=1 ./...` passes at the root and in `cmd/generate`, with and without `-short`; `GOEXPERIMENT=nojsonv2 go test ./...` passes at the
  root; `go build ./example/...` passes; CI passes on all three systems.
* The time of `go test ./...` in `cmd/generate` is recorded before and after.
* `make pre-add-check`, `make check-records`, and `make markdownlint` pass; the identifier scan finds nothing.

## Rollout and Rollback

Commits land on `main`, made by the owner, one per step. No tag; a push waits for the owner's ask. The SDK's API and wire behaviour do not change, so
the release notes need no entry; `schema_gen_test.go` is a test file and is not compiled into a dependent's build. A finding that leads to a fix
changes this, under its own decision.

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

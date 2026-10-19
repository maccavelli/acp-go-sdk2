---
status: in-progress
date: 2026-10-18
associated-madr: "0019-MADR-remediate-debugging-pass-findings.md"
---

# Implement phase 5 of 0019: idioms and performance

Associated MADR: [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md)

## Goal

Implement phase 5 of the MADR's outcome, as "Amended while planning phase 5, 2026-10-18" sets it: designs D1 to D14, for I1 to I7,
I9, I11, I13 to I16 and E16's linters with lint over generated code, as questions 15 and 18 to 21 decided them.

- **Tests:** each change has a test, an engine case or a guard, each seen to fail on a planted break in a scratch copy first.
- **API and behaviour:** the exported API and the documented behaviour do not change.
- **Wire format:** no byte changes, which the encodings guard of D14 holds from step 2 on.
- **Release:** phase 5 releases as a minor version, `v1.16.0` (question 21), which the owner confirms at the tag.

Phases 1 to 4 and the issue fixes are complete and released, the last as `v1.15.2`. The schema stays at `1.25.0` (question 16).

## Standards this plan follows

| Id | Source | Text | Bears on |
| :--- | :--- | :--- | :--- |
| Q1 | `go doc testing.B.Loop` (`go1.27.2`) | `func (b *B) Loop() bool` | D13 |
| Q2 | `go doc context.AfterFunc` | `func AfterFunc(ctx Context, f func()) (stop func() bool)` | D8 |
| Q3 | `go doc context.Cause` | "Cause returns a non-nil error explaining why c was canceled … Cause returns nil if c has not been canceled yet." | D8 |
| Q4 | `go doc reflect.TypeAssert` | `func TypeAssert[T any](v Value) (T, bool)` | D6 |
| Q5 | `go doc os/signal` | "A SIGHUP, SIGINT, or SIGTERM signal causes the program to exit." | D11 |
| Q6 | [0006-MADR-emit-go-with-go-ast.md](0006-MADR-emit-go-with-go-ast.md) | Its title: "Emit generated Go with go/ast instead of Jennifer". | D1 to D4, D7, D9 |
| Q9 | `go doc encoding/json.Marshal` | "Pointer values encode as the value pointed to." | D6 |
| Q7 | [0007-MADR-self-contained-acp-sdk.md](0007-MADR-self-contained-acp-sdk.md) | "`encoding/json` stays the call-site package." | D1 to D3 |
| Q8 | `AGENTS.md`, "Dependencies" | "No dependency may be added without a MADR in this repository that names it." | D13 (no comparison tool) |

## Facts this plan rests on

Each was read on `2ce6421` on 2026-10-18; the code is unchanged since `f67c9c1`, where the MADR's amendment measured it. Step 1
re-asserts each code fact, snippet at line, with a script, before any code changes.

| For | The code as it is |
| :--- | :--- |
| D1 | `unionMarshalBody` (`cmd/generate/internal/emit/types.go:913`) emits, for an object variant, `_b, _e := json.Marshal(*u.X)` (`:937`), a decode into `map[string]json.RawMessage` (`:941`), one assignment per entry of `vi.ConstPairs` (`:945`), and `json.Marshal(m)`. On `types_gen.go`, 88 arms in 22 unions: 67 with one pair whose key is a string field of the variant's struct (12 of those variants have their own `MarshalJSON`), and 21 with none. |
| D2 | `emitObjectUnmarshal` (`types.go:411`) starts every decoder with `objectKeys` (`:441`); a union's chosen variant is decoded by `decodeStrict` (`:655`) as `json.Unmarshal(b, &v)`, which parses again. 291 `objectKeys` calls in `types_gen.go`. Six union variants keep undeclared keys "as received" in `Extra`: `CreateElicitationOther`, `CreateElicitationResponseOther`, `ElicitationPropertySchemaOther`, `IdleStateUpdateOther`, `MultiSelectItemsOther`, `StateUpdateOther`. A discriminated union folds its discriminator and every variant's `FitKeys` (`cmd/generate/internal/model/model.go:209-212`). |
| D3 | A missing (or refused-null) field with a default is decoded from its JSON (`types.go:474`); a lenient field passes the default's JSON to `orDefault` (`:597`, `:613`). In `types_gen.go`: 24 such decodes for 22 fields, each flagged by `errcheck`; 475 `orDefault` calls, 20 with a default. Decoded in a scratch test: 19 of the 24 give the type's zero value (11 `bool` `false`, 8 capability structs); `ElicitationSchema.Properties` (2 sites) gives an empty non-nil map, `InitializeResponse.AuthMethods` (2 sites) an empty non-nil slice, and `ElicitationSchema.Type` gives `"object"`, the constant `ElicitationSchemaTypeObject` (`types_gen.go:5207`). |
| D4 | `arrayMatch` (`types.go:746`) emits the decode into `arr` whether or not an array variant exists (`:769-777`); 7 sites in `types_gen.go` with empty bodies. |
| D5 | `canonicalJSONRPCIDKey` (`connection.go:778`) builds a `json.Decoder` per id. Keys, measured: `0`→`0`, `12345`→`12345`, `18446744073709551615`→itself, `-0`→`0`, `1e2`→`100`, `1.0`→`1`, `007` refused. `FuzzRoundTrip` (`fuzz_test.go:39`) is the root's only fuzz target; `make fuzz` lists targets by name (`Makefile:79-82`). |
| D6 | `validateOutbound` (`connection.go:165`) copies a value through `reflect.New` and asserts `ptr.Interface().(validator)` (`:178`, flagged by `modernize`). It is called by `encodeParams` (`:1718`) and `checkResult` (`:2029`), which passes `addressable(result)` to the gate (`:2031`). The emitter passes `&params` to the gate (`emit/dispatch.go:690`) but `params` to the sends (`:709`, `:718`, `:724`, `:746`) and `resp` to `checkResult` (`emit/dispatch_helpers.go:128`, `emit/dispatch.go:608`). |
| D7 | `emit/dispatch.go:325` emits `superseded := fmt.Errorf("%w: superseded by a new prompt", ErrPromptCancelled)` inside the policy switch's `default:` (`agent_gen.go:148-149`). |
| D8 | Goroutines that wait for a context to end and then broadcast: `start` (`connection.go:642-647`), `waitNotificationsUpTo` (stop channel at `:1859`) and `waitForNotificationDrain` (stop channel at `:1927`). The nil-cause branches: `:1835` (`waitForResponse`) and `:1904` (`waitNotificationsUpTo`). |
| D9 | Emitted hygiene: the alias assignment (`types.go:354-355`); `_b, _e` (`:931-938`), 97 arms in `types_gen.go`, 88 object arms (D1) and 9 non-object arms that end `return _b, nil`; the `UnmarshalTypeError` assertion in `else { if` (`:736`); the `fit` appends (`:889`), 3 chains `gocritic` flags; one-case `switch` statements, 3 from `discUnmarshalBody` (`:816`) and 1 from the catch-all check (`:983`); `string(…)` on a string in the length check (`:1066`), 2 sites; receivers `c` (`emit/dispatch.go:198`, `:675`) and `a` (`:300`) on the same type; `resp, err := …; return resp, err`, 26 sites (10 in `agent_gen.go`, 16 in `client_gen.go`). Hand-written: `errors.As` at `connection.go:2079`, `:2086`, `output/check.go:86`, `:147` and 31 in root tests; `fmt.Errorf` without a verb at `extensions.go:32`, and 22 `perfsprint` error-format findings in the generator (`load/merge.go` 6, `model/protocol.go` 16); the view struct at `errors.go:52`; the index loop at `connection.go:924`. `go fix -diff`: 5 changes at the root, 6 in the generator. |
| D10 | Root tests: 3 `wg.Add`, 49 `context.Background()`, 3 `fmt.Sprintf("%d", …)` (`connection_cancel_test.go:378`, `:604`, `:618`), 1 `slog.NewTextHandler(io.Discard, …)` (`connection_logger_test.go:61`). `thelper`: 10 at the root (`raw_value_test.go`), 10 in the generator (`engine_test.go:20`, `output/check_test.go` 9). |
| D11 | `cmd/generate/internal/mutate/main.go`: `defer os.RemoveAll(work)` (`:112`), mutants run under `context.Background()` (`:144`), `exec.Command` without a context (`:166`); `exec.go:88` already uses `exec.CommandContext`. |
| D12 | `.golangci.yml`: 21 linters enabled (`:8-28`), generated files excluded (`exclusions.generated: lax`, `:87` for linters, `:119` for formatters), tests exempt from six linters (`:96-103`). CI lints through `make lint` (`.github/workflows/ci.yml:70`); the pre-add check through `scripts/go-precheck.sh`. The extended probe on `f67c9c1`: 128 issues at the root (20 hand-written, 108 generated), 28 in `cmd/generate`. |
| D13 | No `func Benchmark` in either module. |
| D14 | No root test declares a flag. The conformance round trip (`conformance_test.go:282-302`) and the JSON goldens (`json_parity_test.go:13-23`) compare decoded values. |

Baseline benchmarks, medians of six runs on `f67c9c1`, `go1.27.2`, `darwin/arm64`, from the MADR's amendment:

| Benchmark | ns/op | allocs/op |
| :--- | ---: | ---: |
| `BenchmarkSessionUpdateMarshal` | 3,530 | 37 |
| `BenchmarkSessionUpdateUnmarshal` | 6,530 | 45 |
| `BenchmarkSessionNotificationUnmarshal` | 8,415 | 58 |
| `BenchmarkCanonicalIDKey` | 542 | 10 |
| `BenchmarkValidateOutboundValue` | 62 | 2 |
| `BenchmarkPromptRoundTrip` | 37,519 | 149 |
| `BenchmarkSessionUpdateNotify` | 34,432 | 115 |

## Scope

In scope:

- **Generator:** `cmd/generate/internal/emit` and `internal/model` for D1 to D4, D6, D7 and D9, and their tests and engine cases.
- **Generated files:** the regenerated files, and `schema_gen_test.go`'s table of defaults (D3).
- **Runtime:** `connection.go`, `errors.go`, `extensions.go`.
- **Tests:** root tests, the new `bench_test.go` and `testdata/encodings.golden`, and generator tests.
- **Mutation tool:** `cmd/generate/internal/mutate`.
- **Configuration and docs:** `.golangci.yml`, `Makefile`, `docs/architecture.md`, `README.md` where it states the lint or the
  benchmarks.
- **Records:** the release notes for `v1.16.0`, and these records.

Out of scope:

- the schema (question 16);
- any exported API change;
- decoding fields one by one from the map, which D2 rejects;
- a benchmark-comparison dependency (Q8);
- `goconst`, `gocognit` and `gocyclo` on generated files (question 15);
- the examples, beyond what the new linters flag in them.

## Design

### D14 and D13 first: the guards and the measures

- **D14, `TestEncodingsUnchanged`** (root):
  - For each entry of `schemaTypes()`, in order, and each of its schema instances from `newBuilder(st.File).instances(st.Def)`, it
    decodes the instance into `st.New()`, encodes it, and writes one line `<Type>\t<bytes>`.
  - The lines are sorted and compared with `testdata/encodings.golden`, failing with the first differing line.
  - `go test -run '^TestEncodingsUnchanged$' -update .` rewrites the file. `-update` is the root package's first flag.
  - An instance that does not decode or encode writes its error on the line instead, so the file pins that too.
  - On `f67c9c1` the scratch form gave 1248 lines.
  - Annotated during step 2, 2026-10-18 (see "Deviation, 2026-10-18, during step 2"): the instances' absolute path, the host's
    temporary directory, is written as `<instancePath>` on each line, raw and JSON-quoted, so the file is the same on every host.
- **D13, `bench_test.go`** (root, `package acp`): the seven benchmarks of the table above, each with `b.Loop()`.
  - The two over pipes use connections with `WithLogger(slog.New(slog.DiscardHandler))`, both negotiated with `allAdvertised()`.
  - `make bench` runs `go test -run '^$' -bench . -benchmem -count=6 .`.
  - CI does not run them.
  - A step that changes a hot path records the medians of six runs before and after, from a script outside the tree.

### D9 and D7: emitted hygiene

Each change below alters the generated code and leaves its behaviour and its bytes as they are, which D14 and the root suite check.

- **The alias assignment:** the emitter writes `a := Alias(v)` in place of `var a Alias; a = Alias(v)`.
- **The outbound wrappers:** a wrapper with nothing after its call returns `sendRequest[…](…)` directly. `Prompt` and the
  pending-cancelling `CloseSession` keep their bodies.
- **`errors.AsType`:** the array match uses `errors.AsType[*json.UnmarshalTypeError](err)` in `else if`. That clears the 8 `errorlint`
  issues and the `else { if`.
- **`fitBody`:** it writes the unconditional names as one composite literal, `fit := []string{…}`.
- **One-case `switch` statements:** `discUnmarshalBody` writes an `if` where its `switch disc` would have one case, and the catch-all
  check where its `switch` would have one.
- **The length check:** it calls `utf8.RuneCountInString` on a string value without a conversion.
- **Receivers:** `AgentSideConnection`'s generated methods all take the receiver `a`.
- **`Prompt`'s cancel:** the emitter keeps the deliberate discard of its best-effort cancel's error with a `//nolint:errcheck` naming
  the reason, so `errcheck`'s `check-blank` sees it is meant.
- **D7:** a package-level `errSuperseded`, `fmt.Errorf("%w: superseded by a new prompt", ErrPromptCancelled)`, emitted once into
  `agent_gen.go` and used by every supersede.

The 9 non-object union arms become `return json.Marshal(*u.X)`, after their nil-array check, here. D1 rewrites the 88 object arms.

### D3: defaults as Go values

The emitter gains a renderer from a field's default JSON and Go type to a Go expression. Each case below either has an expression or
is refused:

| Default | Rendered as |
| :--- | :--- |
| `false`, `0`, `""` or `null` | zero |
| a JSON object, for a field whose type is a generated struct | zero when each member is zero by these rules, and each field of the struct the object leaves out has no default or a zero one, recursively; otherwise refused |
| `{}`, for a map type | `map[K]V{}` |
| `[]`, for a slice type | `[]T{}` |
| a string, for a type with an enum constant of that value | the constant |
| any other string, number or `true` | a literal of the field's type |
| anything else | refused, with an error naming the definition and the property, and an engine case |

How the rendered value is used:

- **A zero default emits nothing:**
  - the decoder emits no injection block for the field;
  - a lenient field calls `orZero[T](raw)`, a new helper for the 455 calls with no default and the 19 with a zero one.
- **A non-zero default:**
  - the injection assigns the expression, `a.Properties = map[string]ElicitationPropertySchema{}`;
  - the lenient field calls `orDefault[T](raw, def T)`, which now takes the value, its one use being `ElicitationSchema.Type`.
- **Run time:** no default is decoded, and the 24 `errcheck` issues go.

The check:

- `schema_gen_test.go` gains `schemaDefaults()`: per field with a default, its type, field, JSON, a `New` that returns a new value of
  the field's type, and a `Value` that returns the rendered expression.
- A root test, `TestSchemaDefaultsMatchTheirJSON`, decodes each JSON into `New()` and requires `reflect.DeepEqual` with `Value()`.
- A model or emitter unit test covers each renderer case and each refusal.

### D1 and D4: encode once

- **An object arm.** When every key of its `ConstPairs` names a string field of the variant's struct, the arm is
  `v := *u.X; v.F = "c" …; return json.Marshal(v)`. The generator knows the fields from the model: an inline variant's fields, or the
  referenced type's.
- **An arm with no pairs** is `return json.Marshal(*u.X)`.
- **Any other object arm** keeps the map path. In the committed schema no arm takes it: all 67 pairs name a field. `constPairs`
  (`cmd/generate/internal/model/model.go:1071`) reads the variant's merged definition, so a variant that refers to a definition and
  declares a constant beside the reference may have no field for it. Step 5 begins by writing that shape as an engine case. If the
  generator accepts it, the case pins the map path. If it does not, the map path is removed and the reason recorded, as
  [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md)'s D6 asks of a site no input reaches.
- **The emitted forms:** the `_b, _e` names, and the "Reshape as raw values" map, are emitted only on that path.
- **D4:** the array match emits its decode into `arr` only when the union has an array variant.

### D2: decode once at each level

- **The decoders.** Each type whose decoder starts with `objectKeys` gains `decodeKeys(b []byte, m map[string]json.RawMessage) error`:
  - it folds the type's required keys and the other keys it reads on `m`, which leaves a folded map as it is;
  - it checks the required keys, with the message `objectKeys` gives;
  - it then runs today's body after `objectKeys`.

  `UnmarshalJSON` becomes `m, err := objectKeys(…)`, then `return v.decodeKeys(b, m)`.
- **The unions.** A union with its map in scope (`discUnmarshalBody`, `fitBody`, `keyLessBody` and the object path) decodes the chosen
  variant with `v.decodeKeys(b, m)` when the variant's decoder is `objectKeys`-based and keeps no `Extra`.
- **The exception.** The six `Extra`-keeping catch-alls (see "Facts") keep `json.Unmarshal(b, &v)`. The union's map is folded with the
  union's key list, so it could spell a key differently from how it was received, which `Extra` must keep. The MADR's D2 is annotated
  with this exception, found while drafting this PLAN.
- **What the emitter knows.** A pre-pass over the model records which types get an `objectKeys` decoder and which keep `Extra`.
- **The tests.** A root test decodes, through its union, each of the six catch-alls with an undeclared key whose spelling differs in
  case from a key the union folds, and requires `Extra` to hold the key as received. Another requires a missing required key of a
  variant decoded through `decodeKeys` to give the error `objectKeys` gives.

### D5, D6, D8, and the hand-written hygiene

- **D5, the id fast path:** `canonicalJSONRPCIDKey`, after trimming, returns the text for 1 to 19 ASCII digits whose first digit is
  not `0`, or for exactly `0`; any other input takes the decoder.
  - `FuzzCanonicalIDKey` (root) requires the fast path and the decoder, called directly, to give the same key and the same error, if
    any, for every input; it is seeded with the measured samples and listed in `make fuzz`.
- **D6, no copy for a typed send:**
  - the emitter passes `&params` to the sends and `&resp` to `checkResult`; `json.Marshal` of a pointer writes the bytes of its value
    (Q9);
  - `validateOutbound`'s value path uses `reflect.TypeAssert[validator](ptr)`, an idiom change, flagged by `modernize`, with no
    allocation claimed.
- **D8, the wake goroutines:**
  - `start`'s broadcast goroutine becomes `context.AfterFunc(c.ctx, broadcast)`, kept for the connection's life;
  - in `waitNotificationsUpTo` and `waitForNotificationDrain`, the stop-channel goroutine becomes
    `stop := context.AfterFunc(ctx, broadcast); defer stop()`;
  - after `<-ctx.Done()`, `waitForResponse` and `waitNotificationsUpTo` return `toReqErr(context.Cause(ctx))`, with the nil branch and
    its "without cause" error deleted (Q3).
- **Hand-written hygiene (D9):**
  - `errors.AsType` at every `errors.As`, the 31 in root tests included;
  - `errors.New` for each `fmt.Errorf` without a verb;
  - `Error()` encodes the error's own exported fields, whose names and tags are the view's (`errors.go:18-20`, `:53-55`), and the
    view struct goes; a test of `Error()` on a code, a message and data pins its text;
  - `connection.go:924`'s loop as `for i := range len(raw)`;
  - each change `go fix -diff` proposes, in both modules.

### D10, D11, D12

- **D10, the tests:**
  - `t.Context()` where a test's context fits;
  - `wg.Go` for the three `wg.Add`;
  - `slog.DiscardHandler`;
  - `strconv.Itoa`;
  - `t.Helper()` for the 20 `thelper` findings.
- **D11, the mutation tool:** `main` runs under `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`;
  `syscall.SIGTERM` is defined for Windows too (`GOOS=windows go doc syscall.SIGTERM`). It passes that context to `runAll`, each `runMutant` and `listFiles`'s
  `exec.CommandContext`. An interrupt then ends the running `go test`, `runAll` stops scheduling, and `main` returns, so the deferred
  `os.RemoveAll` runs. A test of the tool calls `runAll` with a context already ended and requires that no mutant runs.
- **D12, the lint configuration:**
  - `.golangci.yml` enables `copyloopvar`, `intrange`, `modernize`, `perfsprint`, `thelper` and `usestdlibvars`;
  - it sets `exclusions.generated: disable` for linters;
  - it adds one exclusion rule for `goconst`, `gocognit` and `gocyclo` on `_gen\.go$`, with a comment giving question 15's reason;
  - the formatters keep their exclusions;
  - `make lint` and the pre-add check must report 0 issues for `darwin` and `windows`, in both modules.

## Implementation Steps

Each step ends with its gates and an execution record, and the owner commits it; agents do not commit on `main`. Every new test,
engine case and guard is seen to fail on a planted break in a scratch copy, never in the tree, before it is relied on. From step 2 on,
every step's gates include `TestEncodingsUnchanged` unchanged and the API listing against `v1.15.2` unchanged.

1. **Records.** This PLAN and its `docs/README.md` row; the MADR's D2 annotation; the facts above re-asserted on the tree by a script.
2. **D14 and D13.** `testdata/encodings.golden` written from the tree, `TestEncodingsUnchanged`, `bench_test.go` and `make bench`;
   the baseline medians re-measured on the tree. No generated file changes.
3. **D9 (emitted) and D7.** The emitter changes, then regeneration; the generator's tests and engine cases updated to the new output.
4. **D3.** The renderer, `orZero` and the new `orDefault`, the injection change, `schemaDefaults()` and
   `TestSchemaDefaultsMatchTheirJSON`, the refusal engine cases.
5. **D1 and D4.** The arms, the map-path engine case, the array-match change; the encode benchmarks before and after.
6. **D2.** `decodeKeys`, the union calls, the `Extra` exception; the decode benchmarks before and after.
7. **D5, D6, D8, and the hand-written D9.** The fast path and its fuzz target; the pointers; `context.AfterFunc`; the deleted branches;
   `errors.AsType`, `errors.New`, the view struct, the loop, `go fix` at the root; the round-trip and id benchmarks before and after.
8. **D10, D11, and the generator's D9.** The test idioms, `t.Helper()`, the mutation tool's signal handling, `errors.New` and `go
   fix` in `cmd/generate`.
9. **D12.** The lint configuration; 0 issues in both modules for both `GOOS` values; `go fix -diff ./...` reports nothing in either
   module, as the MADR's "Confirmation" requires.
10. **Release notes.** `docs/reports/0019-REPORT-release-notes-v1-16-0.md` and its `docs/README.md` row.
11. **Verification and close.** The gates over the tree; every benchmark against the baseline; the pre-push guard (`make push-guard`);
    after CI passes on the pushed commits, this PLAN and its row become `complete`, and the owner tags `v1.16.0` and publishes its
    release from the report.

## Verification

Per code step, through the Python runner outside the tree:

- regeneration: the generated files' diff is the step's;
- `make pre-add-check`, and `scripts/go-precheck.sh` on each new `.go` file;
- `GOEXPERIMENT=nojsonv2 go test ./...`, and `GOOS=windows go vet ./...` in both modules;
- `TestEncodingsUnchanged`;
- each new check seen to fail on its planted break in a scratch copy;
- `make check-records`, `make markdownlint` and the identifier scan;
- Windows `go test -count=1 -shuffle=on ./...` in both modules;
- the API listing against `v1.15.2`.

The tests that use a context or a bubble also run `-count=1000 -race`, as the issue fixes' deviation after their close showed a 50-run
check can miss a race.

Planted breaks, at least. Each must be seen to fail on the check named; a break that is not seen to fail stops its step, and the step
adds a check that fails on it.

| For | Break | The check that must fail |
| :--- | :--- | :--- |
| D14 | one byte of one encoding changed | `TestEncodingsUnchanged` |
| D3 | each renderer case rendering a different value | `TestSchemaDefaultsMatchTheirJSON` |
| D3 | an unsupported default accepted | the refusal's engine case |
| D1 | a constant written with the wrong value | `TestEncodingsUnchanged` |
| D1 | the map-path engine case's variant given the field | that engine case |
| D2 | `decodeKeys` not checking a required key | D2's required-key test |
| D2 | the union passing its map to an `Extra` catch-all | D2's spelling test |
| D5 | the fast path accepting a leading zero | `FuzzCanonicalIDKey`, from its seeds |
| D8 | each `context.AfterFunc` not registered | `TestWaitNotificationsEnds` (`branches_test.go`), its `context` and `connection` cases |
| D11 | the context not passed to `runMutant` | D11's test |
| D12 | an issue of each new linter planted | `make lint` |

Expected API listing difference against `v1.15.2`: none.

Acceptance:

- every step's checks pass and its record is written;
- `TestEncodingsUnchanged` holds from step 2 to the end;
- every benchmark of the table has a median of six runs' allocs/op at the end no higher than its baseline median, on the same host;
  `BenchmarkSessionUpdateMarshal`, `BenchmarkSessionUpdateUnmarshal` and `BenchmarkSessionUpdateNotify` have lower medians;
- the ns/op medians are recorded, not gated, since they vary with the host. allocs/op held constant across six runs for six of the
  seven baselines; `BenchmarkSessionUpdateNotify`, over pipes, gave 114 once and 115 five times;
- `make lint` reports 0 for both `GOOS` values, in both modules;
- `go fix -diff ./...` reports nothing in either module;
- CI passes on the pushed commits.

## Rollout and Rollback

Each step is one commit, reverted by reverting it. Steps 3 to 7 change the generator and regenerate, so a revert of one reverts its
regenerated output with it. The exported API, the documented behaviour and the wire bytes do not change. The release is the minor
`v1.16.0`, as the owner decided.

## Execution record

### Step 1, records, 2026-10-18

The owner committed this PLAN, its `docs/README.md` row and the MADR's D2 annotation as `581c7d4`, and approved the PLAN. This PLAN
and its row are now `in-progress`.

Each fact the table "Facts this plan rests on" states was re-asserted on `581c7d4` by a script outside the tree: 83 cited lines, each
holding its snippet, and 13 counts. 96 of 96 hold.

The script was seen to fail while this PLAN was drafted: run on the `v1.15.1` tree, whose code differs, it held 76 of 91 and exited 1.
It then had 91 facts; five were added after.

Gates on the tree: `make check-records` and `make markdownlint` exit 0; the identifier scan finds nothing. No Go file changed.

### Deviation, 2026-10-18, during step 2

**Found.** The instance builder uses one absolute path, `instancePath`, which is `filepath.Join(os.TempDir(), "acp")`
(`instances_test.go:82`). It appears in 57 of the 1248 encoding lines, always as the whole of a string value. Written as received,
the golden file would carry the path of the host that wrote it. That path is a real-machine path, which the identifier scan
refuses. The test would also fail on every other host, Linux CI and Windows among them. "Design", D14, did not foresee it.

**Decision (owner, 2026-10-18).** `TestEncodingsUnchanged` writes the path as `<instancePath>`, both JSON-quoted (as it appears in an
encoding, with a Windows path's backslashes escaped) and raw (as an error message would carry it). No other file changes for it.
"Design", D14, is annotated, and the MADR's D14 is amended, since it said the bytes are compared as encoded.

### Step 2, D14 and D13, 2026-10-18

What changed, on `9227a5b`:

- **`encodings_test.go`** (new): `TestEncodingsUnchanged` and the root package's first flag, `-update`. The encodings are built as
  "Design", D14, says, with the deviation above. A difference fails with the first differing line, both sides, and the count of
  differing lines.
- **`testdata/encodings.golden`** (new), written by `go test -run '^TestEncodingsUnchanged$' -update .`: 1248 lines. 57 carry
  `<instancePath>`. None carries a decode or encode error, and the identifier scan finds nothing in it. With `<instancePath>` put back
  to this host's path, the file is byte-identical to the scratch dump made on `f67c9c1` while this PLAN was drafted.
- **`bench_test.go`** (new): the seven benchmarks of the baseline table, each with `b.Loop()`. The two over pipes use
  `WithLogger(slog.New(slog.DiscardHandler))`, both sides negotiated with `allAdvertised()`, and `b.Context()`.
- **`Makefile`**: a `bench` target, `go test -run '^$' -bench . -benchmem -count=6 .`, listed by `make help`.

No generated file changed: the regeneration gate left the tree as it was.

Planted breaks, each in a scratch copy of the tree:

| Break | Result |
| :--- | :--- |
| `Diff`'s `json:"newText"` tag spelled `newTexT` in `types_gen.go`, one byte of every `Diff` encoding | `TestEncodingsUnchanged` fails: "testdata/encodings.golden line 193", got `"newTexT":"s"`, want `"newText":"s"`; "3 lines differ" |
| One byte of one golden line, `Diff\t{"newText":"s",…}`, made `"t"` | `TestEncodingsUnchanged` fails: "line 195", got `"s"`, want `"t"`; "1 lines differ" |

The baseline re-measured on the tree with `make bench`, medians of six runs:

| Benchmark | ns/op | allocs/op | allocs/op, the six runs |
| :--- | ---: | ---: | :--- |
| SessionUpdateMarshal | 3,723 | 37 | 37 each |
| SessionUpdateUnmarshal | 6,798 | 45 | 45 each |
| SessionNotificationUnmarshal | 8,601 | 58 | 58 each |
| CanonicalIDKey | 477 | 10 | 10 each |
| ValidateOutboundValue | 51 | 2 | 2 each |
| PromptRoundTrip | 30,861 | 149 | 149 each |
| SessionUpdateNotify | 28,820 | 115 | 115 each |

Each allocs/op median equals its baseline on `f67c9c1`, the table above. The ns/op medians are recorded, not gated.

Gates, with the API listing against `v1.15.2`, each exit 0:

- regeneration: the generated files unchanged;
- `make pre-add-check`, and `scripts/go-precheck.sh bench_test.go encodings_test.go`;
- `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`, and `GOOS=windows go vet ./...` in both modules;
- `make check-records`, `make markdownlint`, and the identifier scan;
- the API listing: 2429 lines at `v1.15.2` and in the tree, 0 removed, 0 added;
- Windows, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0. `TestEncodingsUnchanged` passes
  there against the file written on macOS, so the placeholder holds across hosts.

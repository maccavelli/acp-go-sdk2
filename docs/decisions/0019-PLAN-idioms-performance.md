---
status: complete
date: 2026-10-19
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
  the reason, so `errcheck`'s `check-blank` sees it is meant. Annotated during step 3, 2026-10-18 (see "Deviation, 2026-10-18,
  during step 3"): the emitter cannot print a comment inside a function body. `Prompt` instead logs a failed cancel at Debug.
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

Annotated during step 4, 2026-10-18 (see "Deviation, 2026-10-18, during step 4"): each row gives what decoding the JSON into a new
value of the field's type gives. A non-empty JSON array for a slice type renders `[]T{…}`, and a non-empty JSON object for a map type
renders `map[string]T{…}`. Each element is rendered by these rules with a zero element written out, and an element no rule renders is
refused. A pointer field renders `null` as zero and any other default as `new(…)` of the value its element type renders. A raw or
`any` field renders `json.RawMessage(<the default's JSON>)`, `null` included. A default on a union field is refused.

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

  Annotated during step 6, 2026-10-18 (see "Deviation, 2026-10-18, during step 6"): such a key exists for three of the six. The
  test also pins, for each, the keys its union folds that the catch-all does not declare.

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
  - `wg.Go` for the three `wg.Add`; annotated during step 8, 2026-10-19 (see "Deviation, 2026-10-19, during step 8"): for the one
    that starts a goroutine, `acp_test.go:573`; the other two count handler calls;
  - `slog.DiscardHandler`;
  - `strconv.Itoa`;
  - `t.Helper()` for the 20 `thelper` findings.
  - Annotated during step 8, 2026-10-19 (see "Deviation, 2026-10-19, during step 8"): also `reflect.TypeAssert` at the three
    `Interface().(T)` assertions in root tests, and `errors.New` at the two verbless `fmt.Errorf` calls "Facts" left out.
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
  - Annotated during step 8, 2026-10-19 (see "Deviation, 2026-10-19, during step 8"):
    - `errcheck` sets `exclude-functions: [errors.AsType]`, with its reason, for the 8 `check-blank` issues of the array match;
      it lands in step 8, as the third decision there records;
    - a hand-written `typedGate` in `connection.go` carries `Initialize`'s gate, with a root test of its failed assertion, for the
      unchecked type assertion in `client_gen.go`.

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

### Deviation, 2026-10-18, during step 3

**Found.** "Design", D9, has the emitter mark `Prompt`'s discard of its best-effort cancel's error, `_ = c.Cancel(…)`
(`client_gen.go:713` on `62f5989`), with `//nolint:errcheck` naming the reason. The emitter cannot print that comment. Its layout
(`cmd/generate/internal/emit/file.go:158-310`) positions declarations, struct fields, interface methods and doc comments only. No
statement inside a function body has a position, so `go/printer` has nowhere to attach a line comment to one. Nothing generated
carries a comment inside a function body. The discard is an issue: the phase 4 lint probe, with generated files linted, reported
`errcheck` "Error return value of `c.Cancel` is not checked" on it.

**Decision (owner, 2026-10-18).** The error is handled, not discarded. When the cancel fails, the generated `Prompt` logs it at
Debug on the connection's logger, "failed to send the cancel of a prompt turn", with the method and the error. This is what
`connection.go` does with other best-effort work that fails, such as "failed to answer an invalid message" (`connection.go:1399`).
Nothing is suppressed, so `errcheck` has nothing to report.

The Debug record is new behaviour, the one exception to "Design", D9's "leaves its behaviour and its bytes as they are". The wire
bytes are unchanged. Step 3 adds a test that requires the record when the cancel cannot be written, and plants the record's
removal to see it fail. "Design", D9, is annotated, and the MADR's D9 is amended.

### Step 3, D9 (emitted) and D7, 2026-10-18

What changed, on `62f5989`, with the deviation above:

- **`cmd/generate/internal/emit/ast.go`:**
  - `elseIf` moves here from `dispatch.go`.
  - New `switchOrIf`: an `if tag == value` where a switch would have one case of one value. A case whose body breaks out of the
    switch stays a switch, since in an `if` the `break` would leave an enclosing loop instead; `breaksOut` finds such a break.
- **`cmd/generate/internal/emit/types.go`:**
  - `a := Alias(v)` in each `MarshalJSON`.
  - The 9 non-object union arms return `json.Marshal(*u.X)`.
  - The array match's `else { if … }` is `else if _, ok := errors.AsType[*json.UnmarshalTypeError](err); !ok`.
  - `fitBody` starts `fit` with a composite literal of the variants that always fit. Only `len(fit) == 1` and `fit[0]` are read, so
    the order of the list does not matter.
  - The discriminator switch, `fit[0]`'s switch and the catch-all check go through `switchOrIf`.
  - The length check converts to `string` only for a named string type.
- **`cmd/generate/internal/model/validate.go`:** `Check.NamedString`, true for a `minLength` check on a named type over `string`.
- **`cmd/generate/internal/emit/dispatch.go`:**
  - `side.recv`, `a` for the agent and `c` for the client, is the receiver of every generated method. `emitOutbound` takes the side.
  - A wrapper with nothing after its call returns the call.
  - `Prompt` logs a cancel that fails, as the deviation above decided.
  - D7: `var errSuperseded` is emitted once into `agent_gen.go`, and `storeTurn` cancels with it.
- **Tests:**
  - `dispatch_test.go` and `types_test.go` are updated to the new output. `dispatch_test.go` also pins `errSuperseded`, the logged
    cancel, and a wrapper that returns its call.
  - New `cmd/generate/internal/emit/ast_test.go`, `TestSwitchOrIf`.
  - The engine case `accept-validate-minlength.txtar` gains `label`, a nullable property of the named string type `Label`. It is
    checked with its conversion.
  - New root test `TestPromptLogsFailedCancel` (`connection_cancel_test.go`), with a writer that refuses writes once told to.
- **Regenerated:** `types_gen.go`, `agent_gen.go`, `client_gen.go`.

The generated shapes, counted by a script outside the tree on `62f5989` and on the tree:

| Shape | `62f5989` | Tree |
| :--- | ---: | ---: |
| `var a Alias` then `a = Alias(v)` | 46 | 0 |
| `a := Alias(v)` | 0 | 46 |
| `_b, _e :=` arms | 97 | 88 |
| `return json.Marshal(*u.…)` | 0 | 9 |
| `err.(*json.UnmarshalTypeError)` | 8 | 0 |
| `errors.AsType[*json.UnmarshalTypeError](err)` | 0 | 8 |
| `} else {` then `if` | 8 | 0 |
| unconditional `fit = append(fit, …)` | 13 | 0 |
| `fit := []string{…}` | 0 | 5 |
| switches of one case of one value | 4 | 0 |
| `RuneCountInString(string(…))` | 2 | 0 |
| `func (c *AgentSideConnection)` | 21 | 0 |
| `func (a *AgentSideConnection)` | 4 | 25 |
| `resp, err := …` then `return resp, err`, agent and client | 10 and 16 | 0 and 0 |
| `superseded :=` | 1 | 0 |
| `var errSuperseded` | 0 | 1 |
| `_ = c.Cancel(` | 1 | 0 |

The 88 `_b, _e` arms left are the object arms that D1 rewrites in step 5.

`TestEncodingsUnchanged` passes, under `encoding/json` v2 and with `GOEXPERIMENT=nojsonv2`. The wire bytes are unchanged.

Observed, not changed: an optional property of a named string type with `minLength` is a value field, not a pointer. Its absent value
`""` therefore fails `Validate`. The first form of the new engine rows showed it: `{Title:x}` gave "Named: label: shorter than
minLength 1". The ACP schema has no such property; its one `minLength`, `Notice.title`, is a plain string. The engine case uses a
nullable property, a pointer, instead. This record names it so that it is not lost; it is outside this PLAN.

Planted breaks, each in a scratch copy of the tree:

| Break | Result |
| :--- | :--- |
| `Prompt`'s Debug call replaced by `_ = cerr` in `client_gen.go` | `TestPromptLogsFailedCancel` fails: "no Debug record of the failed session/cancel" |
| `switchOrIf` without its `breaksOut` guard | `TestSwitchOrIf` fails: "a break out of the switch: *ast.IfStmt, want an if: false", and the nested `if` row |
| `NamedString` always false | `TestEngineCases/accept-validate-minlength` fails: "generated code does not type-check" |
| `NamedString` always true | `TestGeneratedFilesUpToDate` fails: "types_gen.go is out of date … first difference at line 8999" |
| `switchOrIf` never writing an `if` | `TestGeneratedFilesUpToDate` fails: "first difference at line 668" |

Run `-count=1000 -race`, each exit 0:

- `TestPromptLogsFailedCancel`, `TestPromptEndedBeforeHandOverSendsNoCancel`, `TestPromptWithEndedContextWritesNothing` and
  `TestPromptCancellationSendsCancelAndAllowsNewSession`.
- The tests matching `Overlap|Supersed|NewerPrompt|PromptTurn`, which run `storeTurn` and its cancels.

Gates, with the API listing against `v1.15.2`, each exit 0:

- regeneration: the generated files unchanged by a second run;
- `make pre-add-check`, and `scripts/go-precheck.sh cmd/generate/internal/emit/ast_test.go`;
- `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`, and `GOOS=windows go vet ./...` in both modules;
- `make check-records`, `make markdownlint`, and the identifier scan;
- the API listing: 2429 lines at `v1.15.2` and in the tree, 0 removed, 0 added;
- Windows, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0.

### Deviation, 2026-10-18, during step 4

**Found.** Two kinds of default that the D3 table either refuses or does not settle are accepted by the generator today:

- **A non-empty array default.** The engine case `accept-default-array.txtar` accepts `"names": {"type": "array", "items": {"type":
  "string"}, "default": ["a"]}`. Its behaviour test requires `{}` to decode `Names` as `[]string{"a"}`, and `Opts{}` to encode it.
  The table refuses that default. A script outside the tree listed every property default in the engine cases and in both
  committed schemas: this is the only one outside the table. The stable schema has 21 defaults and the unstable one 22, and none
  of them is outside the table.
- **Defaults on a pointer, raw or union field.** The rendered value must be what decoding the JSON into a new value of the field's
  type gives, which `TestSchemaDefaultsMatchTheirJSON` checks. On these fields the table's zero row does not hold, and its literal
  row does not say how the value is written:
  - **Pointer:** `false` decodes to a non-nil pointer, not to `nil`. The model test `TestPatchRules`
    (`cmd/generate/internal/model/patch_test.go:19`) builds `"withDefault": {"type": ["string", "null"], "default": "x"}`, a
    `*string` that decodes to a pointer to `"x"`.
  - **Raw or `any`:** a value decodes to its JSON, `null` included.
  - **Union:** `admits` accepts a default that fits a member, and decoding sets that variant.

  None of the ACP defaults sits on such a field.

**Decision (owner, 2026-10-18), first question.** A non-empty array or map default is rendered as a literal: `[]T{…}` for a slice
type and `map[string]T{…}` for a map type. Each element is rendered by the same rules, with a zero element written out (`false`, `0`,
`""`, `T{}`), and an element no rule renders is refused. Nothing the generator accepts today is refused.

**Decision (owner, 2026-10-18), second question.** A pointer field renders `null` as zero and any other default as `new(…)` of the
value its element type renders, using `new` of an expression, which Go has had since 1.26; the floor is the `go 1.27.2` of
`go.mod`. A raw or `any` field renders `json.RawMessage(<the default's JSON>)`, `null` included, which is what decoding gives. A
default on a union field is refused with an error naming the property, and an engine case: writing a union value out would mean
building its variant by hand. A first form of this entry, before the second question was asked, refused all three; it was not
committed.

"Design", D3's table is annotated. The MADR's D3 is amended, since it named the forms a default is written in.

### Step 4, D3, 2026-10-18

What changed, on `172e9cc`, with the deviation above:

- **`cmd/generate/internal/model/defaults.go`** (new): `Value` and `ValueKind`, and `renderDefaults`. It runs in `Build` after
  `checkDefaults`. It sets `Default.Value` on every field with a default, in the top-level, nested and inline-variant structs, or
  refuses the default with "`<Struct>.<key>`: default `<JSON>`: …". The rules are "Design", D3's table as annotated.
  - A struct default is zero when every key it has renders zero, and every field it leaves out, or holds null where the field
    refuses null, has no default or a zero one, recursively.
  - A struct whose decoding fills more than its fields (flattened members, a key order, or undeclared keys) is refused, as is a
    default without one of its struct's required keys.
- **`cmd/generate/internal/model/fields.go`:** `Default.Value`.
- **`cmd/generate/internal/emit/defaults.go`** (new): `valueExpr` writes a `Value` as a Go expression, and `typedExpr` as an
  expression of the field's type: the operand of `new`, and each `schemaDefaults` value.
- **`cmd/generate/internal/emit/types.go`:**
  - `MarshalJSON` assigns a nilable default, `a.Properties = map[string]ElicitationPropertySchema{}`.
  - `UnmarshalJSON` assigns a non-zero default and writes nothing for a zero one. `UnmarshalJSON` is still emitted for a struct
    whose defaults are all zero, so the errors of a payload that is not an object are as they were.
  - A lenient field calls `orZero[T](raw)`, or `orDefault[T](raw, def)` for a non-zero default. The skip-invalid fallback assigns
    the default.
- **`cmd/generate/internal/emit/decode.go`:** the helpers `orZero[T any](raw json.RawMessage) T` and `orDefault[T any](raw
  json.RawMessage, def T) T`.
- **`cmd/generate/internal/emit/registry.go`:** `schemaDefaults()` and `schemaDefault` in `schema_gen_test.go`.
- **Tests:**
  - New `cmd/generate/internal/model/defaults_test.go`: `TestRenderDefaults`, one row per renderer case, and
    `TestRenderDefaultsRefuses`, one row per refusal.
  - New root test `schema_defaults_test.go`, `TestSchemaDefaultsMatchTheirJSON`.
  - New engine cases `accept-default-pointer-raw.txtar`, with a behaviour test, and `refuse-default-union.txtar`.
  - `TestLenientDecoder` is updated to the new output, and gains a field with a zero default.
- **Regenerated:** `types_gen.go`, `schema_gen_test.go`.

`schemaDefaults()` lists 22 fields with a default. Nineteen render zero. The other three are
`ElicitationSchema.Properties` (`map[string]ElicitationPropertySchema{}`), `ElicitationSchema.Type`
(`ElicitationSchemaTypeObject`) and `InitializeResponse.AuthMethods` (`[]AuthMethod{}`).

The decoder shapes of `types_gen.go`, counted by a script outside the tree on `172e9cc` and on the tree:

| Shape | `172e9cc` | Tree |
| :--- | ---: | ---: |
| `json.Unmarshal([]byte("…"), &a.X)`, a default decoded at run time | 24 | 0 |
| `orDefault[…](…)` calls | 474 | 1 |
| `orZero[…](…)` calls | 0 | 472 |
| injection blocks, `if !_ok …` | 22 | 3 |
| `foldKeys(b, m, …)` calls | 43 | 36 |

On `172e9cc`, the 474 `orDefault` calls were 453 without a default, 19 with a zero one, 1 with `"object"`, and 1 with `nil` raw
and `"[]"`, the skip-invalid fallback. "Design", D3 counted "455 calls with no default", and the fact D3 counted "475 `orDefault`
calls, 20 with a default". Both were off: the 475 included the helper's definition. The tree's 472 `orZero` calls are the 453 and
the 19; the `"object"` call is the one `orDefault`; the fallback is an assignment. The 7 `foldKeys` calls that went folded only the
keys of zero defaults, which nothing reads now.

`TestEncodingsUnchanged` passes, under `encoding/json` v2 and with `GOEXPERIMENT=nojsonv2`, as does
`TestSchemaDefaultsMatchTheirJSON`. The wire bytes are unchanged.

`new` of an expression was proven to need Go 1.26, by a module outside the tree. At `go 1.25`, `go vet` reported "new(int32(3))
requires go1.26 or later". At `go 1.26` it exited 0.

Planted breaks, each in a scratch copy of the tree. The tool now reports a break that the generator itself refuses as not seen on
its check. Four breaks first failed for the wrong reason: three did not compile, and one tripped the generator's guard. They were
rewritten until each failed on its check:

| Break | Result |
| :--- | :--- |
| a zero `bool` default written `true` | `TestSchemaDefaultsMatchTheirJSON` fails: "AgentCapabilities.LoadSession: default false decodes to false; the generated code writes true", and the other `bool` defaults |
| an empty map written `nil` | `TestSchemaDefaultsMatchTheirJSON` fails: "ElicitationSchema.Properties: default {} decodes to map[string]acp.ElicitationPropertySchema{}; the generated code writes \<nil\>" |
| an empty slice written `nil` | `TestSchemaDefaultsMatchTheirJSON` fails: "InitializeResponse.AuthMethods: default [] decodes to []acp.AuthMethod{}; …" |
| a constant chosen without its value | `TestRenderDefaults` fails: "mode: default \"manual\" renders … ModeAuto, want … ModeManual" |
| a number rendered one higher | `TestRenderDefaults` fails on `n`, `f` and `count` |
| a pointer default without `new` | `TestRenderDefaults` fails on `title`, `count` and `modeP`; `TestEngineBehaviour` fails: `accept-default-pointer-raw` "does not type-check" |
| a raw default rendered `null` | `TestRenderDefaults` fails on `raw` and `raws` |
| a non-zero struct default accepted | `TestRenderDefaultsRefuses` fails on "non-zero struct" and "a field's non-zero default" |
| a union default accepted | `TestEngineCases/refuse-default-union` fails: "run accepted the schema" |
| no default assigned on decode | `TestEngineBehaviour/accept-default-array` fails: "defaults decoded as acp.Opts{List:[]string(nil), Names:[]string(nil)}" |

The ACP schema has one enum constant default, `ElicitationSchemaType`, whose only value is `"object"`. A wrongly chosen constant
therefore cannot be seen in the root package, and the model test sees it instead.

No benchmark of the table decodes a type with a default, so this step records none; steps 5 and 6 measure the decoders.

Gates, with the API listing against `v1.15.2`, each exit 0:

- regeneration: the generated files unchanged by a second run;
- `make pre-add-check`, and `scripts/go-precheck.sh` on the four new `.go` files;
- `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`, and `GOOS=windows go vet ./...` in both modules;
- `make check-records`, `make markdownlint`, and the identifier scan;
- the API listing: 2429 lines at `v1.15.2` and in the tree, 0 removed, 0 added;
- Windows, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0.

### Step 5, D1 and D4, 2026-10-18

**The map path's reachability, first.** "Design", D1 had step 5 begin with a variant that refers to a definition and declares a
constant beside the reference, expecting it might leave a constant with no field. A probe outside the tree built that shape and
others with the model, and listed, for each constant pair, the Go type of the field it names. The pair never lacks a field:

- **A direct `$ref`.** The variant takes its facts from the definition it names alone (`cmd/generate/internal/model/model.go:905-911`
  on this step's tree). A constant beside the reference is not a pair.
- **An `allOf` of one reference with properties beside it.** This is not the single-reference form (`schema.go:106`), so the
  variant is an inline struct built from the merged properties, and it has the field.

The probe found the map path reached another way. D1's direct form needs a string field, and a constant can sit in a field that
is not one: `*string` for a nullable constant, `json.RawMessage` for an untyped one. The union's constructors write the
discriminator's constant into its field (`model.go:780-797`), so a discriminator in such a field does not compile, and the
generator refuses it at its type check. A second constant, beside the discriminator, does compile, and takes the map path. The
committed schemas have only string fields: in the generated code, 67 arms set a constant, all on a string field. As D1 says for a
generator that accepts the shape, an engine case pins the map path: `accept-union-const-not-string.txtar`.

What changed, on `d494964`:

- **`cmd/generate/internal/model/consts.go`** (new): `resolveConstFields` runs in `Build` after `renderDefaults` and sets each
  object variant's new `ConstFields`. That is the Go field of each of its `ConstPairs`, when every one is a string field: `string`,
  an enum kind, or a primitive over `string`. It is nil when one is not, or when an empty constant's field omits empty values.
- **`cmd/generate/internal/model/model.go`:** `Variant.ConstFields`, and the call.
- **`cmd/generate/internal/emit/types.go`:**
  - D1: an object arm with no constant is `return json.Marshal(*u.X)`. One whose constants have fields is
    `v := *u.X; v.F = "c" …; return json.Marshal(v)`. Any other keeps the map path, the only place `_b, _e` and the map are left.
  - D4: the array match decodes into `arr` only when the union has an array variant to match.
- **Tests:** new `cmd/generate/internal/model/consts_test.go`, `TestConstFields`, and the engine case
  `accept-union-const-not-string.txtar` with a behaviour test.
- **Regenerated:** `types_gen.go`, 20976 lines to 20318.

The shapes of `types_gen.go`, counted by a script outside the tree on `d494964` and on the tree:

| Shape | `d494964` | Tree |
| :--- | ---: | ---: |
| `_b, _e :=` arms | 88 | 0 |
| a constant assigned into the map | 67 | 0 |
| `v := *u.X` copies | 0 | 67 |
| `return json.Marshal(*u.X)` | 9 | 30 |
| `var arr []map[string]json.RawMessage` | 7 | 0 |
| `var arr []json.RawMessage` | 1 | 1 |

`TestEncodingsUnchanged` passes, under `encoding/json` v2 and with `GOEXPERIMENT=nojsonv2`. The wire bytes are unchanged. No
generator test pinned the old union arms, so none changed.

The benchmarks, medians of six runs with `make bench`, before on `d494964` and after on the tree:

| Benchmark | ns/op before | ns/op after | allocs/op before | allocs/op after |
| :--- | ---: | ---: | ---: | ---: |
| SessionUpdateMarshal | 4,924 | 1,060 | 37 | 9 |
| SessionUpdateUnmarshal | 6,256 | 6,371 | 41 | 41 |
| SessionNotificationUnmarshal | 8,087 | 8,254 | 53 | 53 |
| CanonicalIDKey | 491 | 544 | 10 | 10 |
| ValidateOutboundValue | 78 | 58 | 2 | 2 |
| PromptRoundTrip | 69,472 | 33,209 | 142 | 128 |
| SessionUpdateNotify | 35,283 | 18,513 | 110 | 81 |

Before this step, steps 3 and 4 had already moved three of step 2's baseline medians: SessionUpdateUnmarshal 45 to 41,
SessionNotificationUnmarshal 58 to 53, and PromptRoundTrip 149 to 142. SessionUpdateNotify's six runs gave 109 and 110 before
this step and 81 in all six after. The ns/op medians are recorded, not gated.

Planted breaks, each in a scratch copy of the tree:

| Break | Result |
| :--- | :--- |
| a constant written with the wrong value | `TestEncodingsUnchanged` fails: "line 28", "265 lines differ" |
| the map-path engine case's variant given the field, every type counted a string | `TestEngineCases/accept-union-const-not-string` fails: "generated code does not type-check" |
| the map path setting its constants under other keys | `TestEngineBehaviour/accept-union-const-not-string` fails: encoded as `{"kind":"","kind_":"a","tag_":"t","x":"1"}` |
| the array decode emitted with no array variant | `TestGeneratedFilesUpToDate` fails: "types_gen.go is out of date … line 345" |
| the empty-constant guard dropped | `TestConstFields` fails: "variant eo … ConstFields [Kind Tag], want []" |
| an enum kind not counted a string | `TestConstFields` fails: "variant enum … ConstFields [], want [Kind Tag]" |
| a primitive over `string` not counted a string | `TestConstFields` fails: "variant prim … ConstFields [], want [Kind Tag]" |

Gates, with the API listing against `v1.15.2`, each exit 0:

- regeneration: the generated files unchanged by a second run;
- `make pre-add-check`, and `scripts/go-precheck.sh` on the two new `.go` files;
- `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`, and `GOOS=windows go vet ./...` in both modules;
- `make check-records`, `make markdownlint`, and the identifier scan;
- the API listing: 2429 lines at `v1.15.2` and in the tree, 0 removed, 0 added;
- Windows, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0.

### Deviation, 2026-10-18, during step 6

**Found.** "Design", D2's spelling test decodes each of the six `Extra`-keeping catch-alls through its union, with an undeclared key
whose spelling differs in case from a key the union folds. Such a key exists for three of the six only. Read from the `foldKeys`
call of each union's decoder and the json tags of each catch-all, on this step's tree:

| Union | Keys it folds | Of them, undeclared by its catch-all |
| :--- | :--- | :--- |
| `CreateElicitationRequest` | `mode`, `requestedSchema`, `elicitationId`, `url` | `elicitationId`, `requestedSchema`, `url` |
| `ElicitationPropertySchema` | `type`, `items` | `items` |
| `MultiSelectItems` | `type`, `enum`, `anyOf` | `anyOf`, `enum` |
| `CreateElicitationResponse` | `action` | none |
| `IdleStateUpdate` | `stopReason` | none |
| `StateUpdate` | `state` | none |

For the last three, the union's fold renames only a key the catch-all declares and folds itself, which `Extra` does not keep. So
decoding from the union's map would change nothing they hold. The MADR's D2 annotation said the union "could spell a key `Extra`
keeps differently"; it can for the first three.

**Decision (owner, 2026-10-18).** The test tables all six. For each, it pins the keys its union folds that the catch-all does not
declare, read from `types_gen.go` with `go/ast`, so a schema change that adds one fails the test until its row is updated. The three
with such a key decode it in another case and require `Extra` to keep it as received. The other three decode another undeclared key
and require it kept as received. "Design", D2's tests are annotated, and the MADR's D2 annotation is amended.

### Step 6, D2, 2026-10-18

What changed, on `5d32071`, with the deviation above:

- **`cmd/generate/internal/emit/types.go`:**
  - Every `objectKeys`-based decoder is split in two. `UnmarshalJSON` is `m, err := objectKeys(b, "T")` then
    `return v.decodeKeys(b, m)`. The new `decodeKeys` folds and requires the required keys with `requireKeys`, folds the other keys
    it reads, and runs the body that followed `objectKeys`. Its map parameter is `_` when the body never reads it.
  - `decodesKeys` holds the rule for which structs get such a decoder, which `emitObjectCodec` used to inline. The pre-pass
    `keyDecoders(m)`, run first in `typesFile`, applies it to every struct and leaves out those that keep `Extra`.
  - A union decodes its chosen variant with `v.decodeKeys(b, m)` where its map is in scope and the variant is in `keyDecoders`: the
    discriminator switch, the fit and key-less paths, and the object path. Trial decodes, the array match and the catch-all branch
    keep `json.Unmarshal(b, &v)`.
- **`cmd/generate/internal/emit/decode.go`:** `objectKeys(b, typ)` only decodes the object. The new helper
  `requireKeys(b, m, typ, required...)` folds and requires keys, with the message `objectKeys` gave.
- **`cmd/generate/internal/emit/file.go`:** `goFile.keyDecoders`.
- **Tests:**
  - New root tests: `catchall_extra_test.go`, `TestCatchAllExtraKeepsSpelling`, and `decode_keys_test.go`,
    `TestUnionVariantMissingKey`. The latter's four messages are those the generated code gave on `5d32071`, from the comparison
    below.
  - New engine case `accept-union-extra-variant.txtar`, with a behaviour test. It is a `$ref` variant whose definition keeps `Extra`,
    the one input that reaches `keyDecoders`' `Extra` rule. In the committed schemas, no `Extra` type is decoded where the union's
    map is in scope.
  - `TestRequiredKeysDecoder` is updated to the new output.
- **Regenerated:** `types_gen.go`, 20318 lines to 22645.

The shapes of `types_gen.go`, counted by a script outside the tree on `5d32071` and on the tree:

| Shape | `5d32071` | Tree |
| :--- | ---: | ---: |
| `objectKeys(b, …)` calls | 291 | 291 |
| of them, with required keys | 192 | 0 |
| `decodeKeys` methods | 0 | 291 |
| `decodeKeys(b, m)` calls | 0 | 423 |
| `requireKeys(b, m, …)` calls | 0 | 192 |
| `if err := json.Unmarshal(b, &v); err != nil` | 143 | 11 |

The 423 calls are 291 from `UnmarshalJSON` and 132 from unions. The 11 decodes left are:

- the six catch-alls;
- `RequestPermissionOutcomeCancelled`, twice, which has no `objectKeys` decoder;
- `SessionConfigSelectOptions`' two array variants, decoded where no map is in scope;
- `Patch[T]`, which is not a union.

**The decoders give what they gave.** A test outside the tree decoded, for each schema instance of each registered type that is a
JSON object, the object with each top-level key removed, upper-cased, and doubled as an upper-cased copy holding null: 7812
payloads, 1302 of them errors. Each result, the error or the re-encoding, was written on `5d32071` and on the tree, under
`encoding/json` v2 and with `GOEXPERIMENT=nojsonv2`. Both pairs of files are byte-identical. The two backends word some errors
differently from each other, so each was compared with itself. `TestEncodingsUnchanged` passes under both.

The benchmarks, medians of six runs with `make bench`. Before is the after of step 5, measured on the code `5d32071` holds; after
is on the tree:

| Benchmark | ns/op before | ns/op after | allocs/op before | allocs/op after |
| :--- | ---: | ---: | ---: | ---: |
| SessionUpdateMarshal | 1,060 | 1,154 | 9 | 9 |
| SessionUpdateUnmarshal | 6,371 | 4,386 | 41 | 27 |
| SessionNotificationUnmarshal | 8,254 | 5,724 | 53 | 39 |
| CanonicalIDKey | 544 | 508 | 10 | 10 |
| ValidateOutboundValue | 58 | 55 | 2 | 2 |
| PromptRoundTrip | 33,209 | 32,675 | 128 | 121 |
| SessionUpdateNotify | 18,513 | 17,326 | 81 | 68 |

Each allocs/op was the same in all six runs after. The ns/op medians are recorded, not gated.

Planted breaks, each in a scratch copy of the tree:

| Break | Result |
| :--- | :--- |
| `decodeKeys` not checking a required key | `TestUnionVariantMissingKey` fails: `{"sessionUpdate":"agent_message_chunk"}` "decoded with \<nil\>", and the other rows |
| the union passing its map to an `Extra` catch-all | `TestCatchAllExtraKeepsSpelling` fails on the three unions with a key to fold: "kept Extra map[url:\"u\"], want \"URL\" holding \"u\" as received", and `items`, `enum` |
| `keyDecoders` keeping `Extra` types | `TestEngineBehaviour/accept-union-extra-variant` fails: "decoded as &{Kind:a Extra:map[other:\"x\"]}, want Extra holding OTHER as received" |
| no union decoding from its map | `TestGeneratedFilesUpToDate` fails: "types_gen.go is out of date … line 366" |

Gates, with the API listing against `v1.15.2`, each exit 0:

- regeneration: the generated files unchanged by a second run;
- `make pre-add-check`, and `scripts/go-precheck.sh` on the two new `.go` files;
- `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`, and `GOOS=windows go vet ./...` in both modules;
- `make check-records`, `make markdownlint`, and the identifier scan;
- the API listing: 2429 lines at `v1.15.2` and in the tree, 0 removed, 0 added;
- Windows, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0.

### Step 7, D5, D6, D8 and the hand-written D9, 2026-10-18

What changed, on `6d60d84`:

- **D5, `connection.go`:** `canonicalJSONRPCIDKey` returns a plain integer as its own key: `0`, or 1 to 19 ASCII digits whose first is
  not `0` (`isPlainIDInteger`). Anything else goes to `decodeJSONRPCIDKey`, the former body. The new `FuzzCanonicalIDKey`
  (`fuzz_test.go`) requires the two to give every id the same key and the same error. It has 22 seeds: the ids the PLAN measured, the
  fast path's edges, and ids it must leave to the decoder. It is listed in `make fuzz`. A 60-second run fuzzed 1,579,557 inputs and
  found no difference.
- **D6:**
  - The emitter passes `&params` to every send and `&resp` to `checkResult` (`cmd/generate/internal/emit/dispatch.go`,
    `dispatch_helpers.go`).
  - `validateOutbound`'s value path uses `reflect.TypeAssert[validator]`.
  - A handler's result reaches the connection as a pointer, so two root tests that type-asserted it as a value now assert the pointer:
    `acp_test.go`, `UnstableForkSessionResponse`, and `cancel_test.go`, `PromptResponse`. The emitter's tests are updated to the new
    calls.
- **D8, `connection.go`:**
  - The three wake goroutines are `context.AfterFunc` registrations of the new `wakeNotificationWaits`: `start`'s, for the
    connection's life, and `waitNotificationsUpTo`'s and `waitForNotificationDrain`'s, each stopped by a deferred call.
  - After `<-ctx.Done()`, `waitForResponse` and `waitNotificationsUpTo` return `toReqErr(context.Cause(ctx))`. Their nil-cause
    branches are deleted. `go doc context.Cause`: "Otherwise Cause(c) returns the same value as c.Err()". `go doc context.Context`, on
    `Err`: "If Done is closed, Err returns a non-nil error".
  - The three `go func()` in `connection.go` that remain are not wake goroutines.
- **D9, hand-written:**
  - `inboundError` uses `errors.AsType` for both its checks, the first applied by `go fix`.
  - `extensions.go`'s verbless `fmt.Errorf` is `errors.New`.
  - `RequestError.Error` encodes `e` itself. The view struct is gone; its fields and tags were `RequestError`'s exported ones.
  - `parseBoundedInt`'s loop is `for i := range len(raw)`.
  - `go fix ./...` at the root applied its 5 proposals in `connection.go`, `gates_test.go` and `instances_test.go`. `go fix -diff ./...`
    then reports nothing.
  - Every `errors.As` call in the root tests is `errors.AsType`: 29 calls, none left. Two more matches were the text of messages,
    now reworded. `gates_test.go` gains `isCapabilityError` and `isValidationError` for the checks that only test the type. A script
    outside the tree checked each new `ok :=` for an `ok` it could shadow or reassign. The six it flagged are each in a scope of their
    own.
  - New `TestRequestErrorText` (`errors_test.go`) pins `Error()`'s text: data, no data, a cause that is not encoded, HTML escaping,
    data that does not encode, and nil. It passes on `6d60d84` too, in a scratch copy, so removing the view struct changed no text.

**The D8 checks, strengthened.** The planted-break table names `TestWaitNotificationsEnds` for each `context.AfterFunc` not
registered. All three such breaks passed it as it stood. It ended the wait before the wait had blocked, and the wait's loop checks
`ctx.Done()` and `c.Done()` before it waits. So no wake was needed, by the goroutines this step replaces either. As the PLAN's
"Verification" requires of such a break, the step adds checks that fail on them:

- **`TestWaitNotificationsEnds`** (`branches_test.go`) runs each case in a `synctest` bubble and ends the wait only after
  `synctest.Wait()`, once the wait is blocked in `notifyCond.Wait`, which `go doc testing/synctest` lists as durably blocking.
  - Its `connection` case also requires the wait to return before `notificationQueueDrainTimeout`. Without `start`'s registration,
    the drain's own timeout wakes it five seconds later.
- **`TestEndOfInputNotificationDrainIsBounded`** (`end_of_input_test.go`, new) ends the input while a notification handler waits on
  its context. It requires the handler's context to end with a cause, after the drain's timeout.

Both tests pass on `6d60d84` as well, in a scratch copy with its goroutines.

Planted breaks, each in a scratch copy of the tree:

| Break | Result |
| :--- | :--- |
| the fast path accepting a leading zero | `FuzzCanonicalIDKey` fails from its seeds: `id "007": key "007", error <nil>; the decoder gives "", invalid json-rpc id: trailing data` |
| `start`'s `context.AfterFunc` not registered | `TestWaitNotificationsEnds/connection` fails: "the wait returned 5s after the connection ended, as the drain timed out" |
| `waitNotificationsUpTo`'s not registered | `TestWaitNotificationsEnds/context` fails: "deadlock: all goroutines in bubble are blocked" |
| `waitForNotificationDrain`'s not registered | `TestEndOfInputNotificationDrainIsBounded` fails: "deadlock: all goroutines in bubble are blocked" |
| `Error()` encoding the cause | `TestRequestErrorText` fails on each row with a cause field |

The tests matching `Wait|Drain|EndOfInput|HeldBehind|EndedContext|Notification`, 37 of them, ran `-race -count=1000 -timeout 0`:
exit 0, in 1,252 seconds.

- A first run under `go test`'s default 10-minute timeout reached the timeout. Its goroutine dump shows no wait blocked in the code
  this step changes; one goroutine was in `sync.Cond.Wait`.
- It does show 1,978 connections whose goroutines (`writeLoop`, `sendCancelRequests`, `processNotifications` and the pipe reader)
  stay blocked. The tests leave those connections open, which this step did not change, and they slow later iterations.
- Run 100 times, the same selection took 130 seconds on `6d60d84` and 126 on the tree, so the run's length is not this step's.
  `6d60d84` lacks the new drain test.

The benchmarks, medians of six runs with `make bench`. Before is the after of step 6, measured on the code `6d60d84` holds; after is
on the tree:

| Benchmark | ns/op before | ns/op after | allocs/op before | allocs/op after |
| :--- | ---: | ---: | ---: | ---: |
| SessionUpdateMarshal | 1,154 | 946 | 9 | 9 |
| SessionUpdateUnmarshal | 4,386 | 4,089 | 27 | 27 |
| SessionNotificationUnmarshal | 5,724 | 5,732 | 39 | 39 |
| CanonicalIDKey | 508 | 15 | 10 | 1 |
| ValidateOutboundValue | 55 | 53 | 2 | 2 |
| PromptRoundTrip | 32,675 | 25,493 | 121 | 97 |
| SessionUpdateNotify | 17,326 | 13,738 | 68 | 65 |

Each allocs/op was the same in all six runs after. ValidateOutboundValue keeps its 2: "Design", D6 claims no allocation for
`reflect.TypeAssert`, and the value is still copied to be addressed.

Gates, with the API listing against `v1.15.2`, each exit 0:

- regeneration: the generated files unchanged by a second run;
- `make pre-add-check`;
- `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`, and `GOOS=windows go vet ./...` in both modules;
- `make check-records`, `make markdownlint`, and the identifier scan;
- the API listing: 2429 lines at `v1.15.2` and in the tree, 0 removed, 0 added;
- Windows, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0.

### Deviation, 2026-10-19, during step 8

**Found.** Before step 8's edits, `.golangci.yml` was linted with D12's six linters added and `exclusions.generated: disable`,
through a copy of the file outside the tree, on `95d66e3`, for both `GOOS` values in both modules. Leaving aside `goconst`, `gocognit`
and `gocyclo` on `_gen.go` files, which step 9 excludes, it reports 65 issues, the same for `darwin` and `windows`. 51 are the sites
"Facts" names for D9 and D10. 14 are not in "Facts":

| Issues | Where | Since |
| :--- | :--- | :--- |
| `modernize` `reflecttypeassert`, 3 | `gates_test.go:301`, `patch_test.go:138`, `catchall_extra_test.go:81` | the first two on `f67c9c1`; the third added by step 6 |
| `perfsprint` `error-format`, 2 | `connection_notification_barrier_test.go:470`, `example/agent/main.go:354` | `f67c9c1` |
| `errcheck`, 8 | `types_gen.go`, the array match's `_, ok := errors.AsType[*json.UnmarshalTypeError](err)` | step 3 |
| `errcheck`, 1 | `client_gen.go:414`, `Response: *result.(*InitializeResponse)` in `Initialize`'s gate | `f67c9c1` |

- The MADR's count of E16's hand-written issues includes the first five ("Hand-written code: 20 (`modernize` 13, `perfsprint` 6,
  `intrange` 1)"). "Facts" left them out. D9's design already covers the two `error-format` sites: "`errors.New` for each `fmt.Errorf`
  without a verb".
- The 8 `errcheck` issues come from `check-blank`, because `errors.AsType`'s first result has an error type. They took the place of the
  8 `errorlint` issues that step 3 cleared, as "Design", D9 planned.
- The MADR counts the type assertion among the generated `errcheck` issues ("and one type assertion"). No design fixes it. The gate
  must stay a `gateFunc`, because `checkInbound` passes the gate's error to the inbound hook. The emitter cannot print a `//nolint`
  inside a function body, which step 3's deviation found.
- Without a resolution, step 9 cannot report 0 issues.

An `exclude-functions: [errors.AsType]` setting for `errcheck` was tried in the same copy, outside the tree. It left exactly the type
assertion: `errors.AsType`'s 8 issues were gone and nothing else changed. The setting spelled
`errors.AsType[*encoding/json.UnmarshalTypeError]` left all 9.

**Decision (owner, 2026-10-19).**

- The five hand-written sites are fixed in step 8: `reflect.TypeAssert`, as D6 applies it, and `errors.New`.
- Step 9's `.golangci.yml` excludes `errors.AsType` from `errcheck`, with a comment giving the reason: its first result is the match,
  not a failure to handle. The generated code stays as it is.
- Step 9 adds a hand-written `typedGate[T any](func(method string, result *T) error) gateFunc` to `connection.go`. It asserts the
  result's type, and returns an error naming both types if the assertion fails. The emitter passes `Initialize`'s gate through it.
  A root test drives the failed assertion.

"Design", D10 and D12 are annotated. The MADR's D12 is amended.

**Found, second.** "Design", D10 asks for "`wg.Go` for the three `wg.Add`". Only `acp_test.go:573` starts a goroutine. At
`acp_test.go:796` (`wg.Add(n)`) and `:1154` (`wg.Add(1)`), the `WaitGroup` counts the handler calls the connection makes:
`wg.Done` runs in the handler, and the test starts no goroutine. `wg.Go(f)` runs `f` on a new goroutine, so it cannot express them.

**Decision (owner, 2026-10-19).** `wg.Go` at `acp_test.go:573` only. The two handler counters keep `wg.Add` and `wg.Done`.
"Design", D10 is annotated.

**Found, third.** `make pre-add-check` failed on step 8's tree, with one `errcheck` issue at
`cmd/generate/internal/output/check.go:146:5`: "Error return value of `errors.AsType` is not checked". That line is `go fix`'s
rewrite, `if _, ok := errors.AsType[*build.NoGoError](err); ok`, which D9 requires. It is the `check-blank` issue of the first
decision, in hand-written code, which the configuration lints today.

**Decision (owner, 2026-10-19).** The `errcheck` exclusion of `errors.AsType` lands in step 8 rather than step 9. Step 9 keeps the
rest of D12.

### Step 8, D10, D11 and the generator's D9, 2026-10-19

What changed, on `95d66e3`, with the decisions of "Deviation, 2026-10-19, during step 8":

- **D10, root tests:**
  - `t.Context()` replaces `context.Background()` at all 46 sites in root tests, in 8 files. The three examples keep theirs: an
    example has no `t`.
    - `t.Context()` ends as its test returns, which matters only for a goroutine that can outlive the test. Nine of the 46 sites are
      on a goroutine the test starts. Seven of them call no `t` method.
    - The other two are `prompt` and `askPermission` (`cancel_test.go`), which report a failed call with `t.Errorf`. A script outside
      the tree found each of their 20 answers received before the test returns: directly, through `within`, or from a literal that is
      ranged and received from.
    - Six of the 46 are in `acp_test.go`'s two functions run through `synctest.Test`, where `t` is the bubble's.
  - `wg.Go` at `acp_test.go:573`, the one `wg.Add` that starts a goroutine, as decided.
  - `slog.DiscardHandler` (`connection_logger_test.go`), and `strconv.Itoa` at the three `fmt.Sprintf("%d", …)`
    (`connection_cancel_test.go`).
  - `t.Helper()` first in `raw_value_test.go`'s ten decoding literals.
- **The deviation's five sites:** `reflect.TypeAssert` in `gates_test.go`, `patch_test.go` and `catchall_extra_test.go`; `errors.New`
  in `connection_notification_barrier_test.go` and `example/agent/main.go`.
- **D9 in `cmd/generate`:**
  - `errors.New` for the 22 verbless `fmt.Errorf` calls: 6 in `internal/load/merge.go`, 16 in `internal/model/protocol.go`.
  - `errors.AsType` at both `errors.As` calls of `internal/output/check.go`: `:86` by hand, `:147` by `go fix`.
  - `go fix ./...` applied its 6 proposals, in `mutate/main.go` (3), `mutate/sites.go`, `model/keywords_test.go` and `output/check.go`.
    `go fix -diff ./...` then reports nothing.
  - `thelper`: `engineCases`'s `testing.TB` parameter is `tb`, and `output/check_test.go`'s nine directory makers call `t.Helper()`
    first.
- **D11, `cmd/generate/internal/mutate`:**
  - `main` runs `run` under `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`.
  - `run`, `listFiles` and `runAll` take the context. `listFiles` runs `go list` through `exec.CommandContext`, and `runAll` passes
    the context to each `runMutant`.
  - `runAll` hands out no mutant once the context has ended. A mutant it did not start keeps a zero result.
  - Once the context has ended, `run` returns `stopped: <cause>` and writes no report. Its deferred `os.RemoveAll` of the temporary
    directory runs as it returns. The package comment says so.
- **`.golangci.yml`, by the third decision:** `errcheck` sets `exclude-functions: [errors.AsType]`, with its reason in a comment.
  The rest of D12 stays in step 9.

**The D11 check, strengthened.** "Design", D11 names a test that calls `runAll` with a context already ended and requires that no
mutant runs, and the planted-break table names it for "the context not passed to `runMutant`". That break passed it: `runAll` starts
no mutant once the context has ended, so `runMutant`'s context is never used. As "Verification" requires of such a break, the step adds
a check that fails on it:

- **`TestRunAllEndedContext`** is the named test. It also requires `run`, under an ended context, to return the context's cause and
  write no report.
- **`TestRunAllInterrupted`** ends the context while a mutant's `go test` runs. Its fixture module has one mutant, and that mutant's
  test writes a file and waits until the file is removed. The test requires `runAll` to return within 30 seconds of the context
  ending, and the second mutant, chosen after it, not to start. It then removes the file, so the mutant's test ends either way.

What an interrupt ends, read from `go doc os/exec.CommandContext` and `cmd/go`'s source at `go1.27.2`:

- `exec.CommandContext` kills the `go` command. The test binary that command runs is its child, not the tool's.
- `cmd/go` reads an interrupt only to start no further work (`internal/test/test.go:1459`, `internal/work/exec.go:244`). So a test
  binary already running is left to end by itself. In `TestRunAllInterrupted` it ends because its file is removed.
- An interrupt from a terminal reaches its whole process group, the tool, the `go` command and the test binaries alike. A `SIGTERM`
  sent to the tool alone leaves a running test binary to its own `-timeout`, the tool's `-timeout` flag.

Planted breaks, each in a scratch copy of the tree:

| Break | Result |
| :--- | :--- |
| the context not passed to `runMutant` | `TestRunAllInterrupted` fails: "runAll was still running 30s after its context ended"; `TestRunAllEndedContext` passes, as above |
| `runAll` handing out mutants after the context ends | `TestRunAllEndedContext` fails on each mutant: "mutant 0 ran after the context ended: {File:m.go … Outcome:invalid …}" |
| `run` writing its report after the context ends | `TestRunAllEndedContext` fails: "run = <nil>, want the context's cause" and "run wrote a report after the context ended" |
| `errcheck` without the `errors.AsType` exclusion | a file planted in the copy, `_, ok := errors.AsType[*RequestError](err)` and `_ = os.Remove(…)`: with the exclusion, `errcheck` reports the `os.Remove` line only; without it, both lines |

The other changes keep their tests' behaviour and add no check: the suites that ran before run unchanged.

Run with `-race -count=1000 -timeout 0`, each exit 0:

- the 49 root tests whose code the step changed, found by a script outside the tree, including the tests that call a changed
  helper: 1,436 seconds;
- `TestRunAllEndedContext`: 4 seconds;
- `TestRunAllInterrupted`: 830 seconds. Each iteration runs the fixture's `go test`, and no fixture test binary was left running
  afterwards.

Gates, with the API listing against `v1.15.2`, each exit 0:

- regeneration: the generated files unchanged by a second run;
- `make pre-add-check`. Its first run, before the exclusion, failed only on `output/check.go:146`, as the third decision records;
- `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`, and `GOOS=windows go vet ./...` in both modules;
- `make check-records`, `make markdownlint`, and the identifier scan;
- the API listing: 2429 lines at `v1.15.2` and in the tree, 0 removed, 0 added;
- Windows, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0.

What step 9 is left with. The deviation's probe was run again on this step's tree: the tree's `.golangci.yml`, D12's six linters
added, and `exclusions.generated: disable`, from a copy outside the tree. It gives the same issues for `darwin` and `windows`:

- **`cmd/generate`:** 0.
- **The root:** 93.
  - 92 are `goconst` (86), `gocognit` (4) and `gocyclo` (2), all in `_gen.go` files, which step 9's exclusion covers.
  - 1 is `errcheck` at `client_gen.go:414`, `Initialize`'s type assertion, which step 9's `typedGate` removes.

### Step 9, D12, 2026-10-19

What changed, on `711a1d5`:

- **`.golangci.yml`:**
  - It enables `copyloopvar`, `intrange`, `modernize`, `perfsprint`, `thelper` and `usestdlibvars`, in the list's alphabetical order.
  - It sets `exclusions.generated: disable` for linters, so generated files are linted. The formatters keep `generated: lax`.
  - One exclusion rule covers `goconst`, `gocognit` and `gocyclo` on `_gen\.go$`. Its comment gives question 15's reason: the repeated
    strings are schema literals, and the dispatch and decoders are long by construction.
  - The `errcheck` exclusion of `errors.AsType` landed in step 8, as that step's third decision records.
- **`typedGate`**, by the step 8 deviation's first decision:
  - `connection.go` gains `typedGate[T any](gate func(method string, result *T) error) gateFunc`. It passes the gate a result of
    type `*T`, and answers a result of another type with `<method>: result of type <got>, not <want>`, without calling the gate.
  - The emitter (`cmd/generate/internal/emit/dispatch.go`, `initializeWrapper`) passes `Initialize`'s gate through it, so the gate
    reads `*result` with no type assertion. `client_gen.go` is the one regenerated file that changes. The emitter's pin
    (`dispatch_test.go`) is updated.
  - New root test `TestTypedGate` (`gates_test.go`). It requires the gate to be given the `*InitializeResponse` and its error
    returned, and a value `InitializeResponse` to be refused with that text and no call of the gate.
- **`docs/architecture.md`:** the pre-add check's paragraph says that the lint covers the generated files, except for the three
  linters `.golangci.yml` excludes for `_gen.go` files.

The lint, after the change, each exit 0:

- `make lint`: 0 issues for `GOOS=darwin` and `GOOS=windows`, at the root and in `cmd/generate`;
- `go fix -diff ./...`: no output at the root or in `cmd/generate`, as the MADR's "Confirmation" requires.

Planted breaks, each in a fresh scratch copy of the tree. For the lint, one file was planted and `make lint` run:

| Break | Result |
| :--- | :--- |
| a `for` variable copied, `v := v` | `make lint` exit 2: `zz_plant.go:5:3: The copy of the 'for' variable "v" can be deleted (Go 1.22+) (copyloopvar)` |
| `for i := 0; i < 3; i++` | exit 2: `for loop can be changed to use an integer range (Go 1.22+) (intrange)` |
| a range over `strings.Split` | exit 2: `stringsseq: Ranging over SplitSeq is more efficient (modernize)` |
| `fmt.Sprintf("%d", n)` | exit 2: `integer-format: fmt.Sprintf can be replaced with faster strconv.Itoa (perfsprint)` |
| a test helper without `t.Helper()` | exit 2: `zz_plant_test.go:5:6: test helper function should start from t.Helper() (thelper)` |
| `r.StatusCode == 200` | exit 2: `"200" can be replaced by http.StatusOK (usestdlibvars)` |
| the `intrange` loop in a file with the generated-code header, `zz_plant_gen.go` | exit 2: `zz_plant_gen.go:6:2: … (intrange)`, so generated files are linted |
| one string four times in that generated file | `goconst` reports nothing on it: the exclusion holds |
| the same string four times in a hand-written file | exit 2: ``string `zzplant` has 4 occurrences, make it a constant (goconst)``, so the exclusion covers `_gen.go` files only |
| `typedGate` returning nil for a result of another type | `TestTypedGate` fails: "gate(InitializeResponse) = <nil>, want the error naming both types" |
| `typedGate` calling the gate for a result of another type | `TestTypedGate` fails: "the gate was called 1 times with a result of another type" |
| the emitter writing the old `*result.(*T)` | `TestDispatchFollowsTheModel` fails: `client_gen.go lacks "Response: *result,"` |

`TestTypedGate` first tracked the gate's argument and not its calls. The second `typedGate` break passed it, because the gate was
given a nil `*InitializeResponse`. The test now counts the calls, and both breaks fail it.

No test this step adds or changes uses a context or a bubble, so no `-count=1000` run applies.

Gates, with the API listing against `v1.15.2`, each exit 0:

- regeneration: the generated files unchanged by a second run;
- `make pre-add-check`, which lints with the new configuration for both `GOOS` values in both modules;
- `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`, and `GOOS=windows go vet ./...` in both modules;
- `make check-records`, `make markdownlint`, and the identifier scan;
- the API listing: 2429 lines at `v1.15.2` and in the tree, 0 removed, 0 added;
- Windows, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0.

### Step 10, release notes, 2026-10-19

What changed, on `82d071d`, after the owner pushed steps 1 to 9:

- [0019-REPORT-release-notes-v1-16-0.md](../reports/0019-REPORT-release-notes-v1-16-0.md), the draft of the GitHub release text for
  `v1.16.0`, in the shape of the `v1.15.2` notes. Each statement names a commit, a record, the API listing or the benchmark run below.
- Its `docs/README.md` row, after the `v1.15.2` notes.

Facts the notes rest on, each read on `82d071d`:

- **The schema:** `git diff v1.15.2 HEAD -- schema/` is empty.
- **The API listing:** 2429 lines at `v1.15.2` and at `82d071d`, 0 removed, 0 added (step 9's gates).
- **The commits** since `v1.15.2` that change Go files outside `cmd/generate`: `62f5989`, `172e9cc`, `d494964`, `5d32071`, `6d60d84`,
  `95d66e3`, `711a1d5` and `82d071d`. The other four, `f67c9c1`, `2ce6421`, `581c7d4` and `9227a5b`, change records only
  (`git show --name-only`).
- **The SDK's non-test files:** `git diff --stat v1.15.2 HEAD -- '*.go' ':!*_test.go' ':!cmd/generate' ':!example'` lists six,
  3754 lines added and 2314 removed, 5671 of the changed lines in `types_gen.go`.
- **The one behaviour change:** `Prompt`'s failed best-effort cancel, logged at Debug (step 3's deviation). At `v1.15.2`,
  `client_gen.go:713` discarded it (`_ = c.Cancel(…)`).
- **What callers cannot see:** a handler's result reaches only `checkResult`, which validates and gates it and returns it to be
  encoded. No `With…` option takes a function that receives it. The decoded messages the hooks receive were pointers already.

The benchmarks, which are also the measurement step 11 compares with the baseline:

- **The baseline** is `git archive 62f5989`: the `v1.15.2` code with `bench_test.go` added. `git diff v1.15.2 62f5989 -- '*.go'
  ':!*_test.go'` is empty.
- **The run:** a script outside the tree ran `go test -run '^$' -bench . -benchmem -count=1 .` six times on each, alternating, on
  `darwin/arm64` with `go1.27.2`.

Medians of the six runs:

| Benchmark | ns/op, baseline | ns/op, `82d071d` | allocs/op, baseline | allocs/op, `82d071d` |
| :--- | ---: | ---: | ---: | ---: |
| `BenchmarkSessionUpdateMarshal` | 3,188 | 943 | 37 | 9 |
| `BenchmarkSessionUpdateUnmarshal` | 5,959 | 4,086 | 45 | 27 |
| `BenchmarkSessionNotificationUnmarshal` | 7,516 | 5,723 | 58 | 39 |
| `BenchmarkCanonicalIDKey` | 472 | 15 | 10 | 1 |
| `BenchmarkValidateOutboundValue` | 51 | 53 | 2 | 2 |
| `BenchmarkPromptRoundTrip` | 30,974 | 25,496 | 149 | 97 |
| `BenchmarkSessionUpdateNotify` | 18,526 | 13,167 | 115 | 65 |

- Each allocs/op of `82d071d` was the same in all six runs.
- Against "Verification"'s acceptance:
  - each median allocs/op is no higher than the baseline's;
  - `BenchmarkSessionUpdateMarshal`, `BenchmarkSessionUpdateUnmarshal` and `BenchmarkSessionUpdateNotify` are lower.
- The baseline's allocs/op equal the table of "Facts", measured on `f67c9c1`.

The step changes no Go file. Gates, each exit 0: `make check-records`, `make markdownlint`, and the identifier scan.

### Step 11, verification and close, 2026-10-19

**The pushes.** `make push-guard` ran, after a fetch, before each push the owner made, and passed, exit 0:

- over the twelve commits `f7defb4..82d071d`, steps 1 to 9; CI run `39270585336` on `82d071d` concluded `success` on its first attempt,
  on `validate (ubuntu-24.04)`, `validate (macos-15)` and `validate (windows-2025)`;
- over the one commit `82d071d..4b3797f`, step 10; CI run `39272624637` on `4b3797f` concluded `success` on its first attempt, on
  all three runners.

**The gates over the tree**, on `4b3797f`, with the API listing against `v1.15.2`, each exit 0:

- regeneration: the generated files unchanged by a second run;
- `make pre-add-check`, for both `GOOS` values in both modules;
- `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`, and `GOOS=windows go vet ./...` in both modules;
- `make check-records`, `make markdownlint`, and the identifier scan;
- the API listing: 2429 lines at `v1.15.2` and in the tree, 0 removed, 0 added;
- Windows, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0.

**Acceptance.** Every criterion in "Verification" is met:

- **Each step's checks** passed, and its record is written, with the deviations of 2026-10-18 and 2026-10-19.
- **`TestEncodingsUnchanged`** held from step 2 to the end, with `testdata/encodings.golden` unchanged since `62f5989`
  (`git diff 62f5989 HEAD -- testdata/encodings.golden` is empty).
- **The benchmarks**, against the baseline, are step 10's measurement. No Go file has changed since `82d071d`, which it measured
  (`git diff 82d071d HEAD -- '*.go'` is empty).
  - Each median allocs/op is no higher than its baseline.
  - `BenchmarkSessionUpdateMarshal` (37 to 9), `BenchmarkSessionUpdateUnmarshal` (45 to 27) and `BenchmarkSessionUpdateNotify` (115 to
    65) are lower.
- **`make lint`** reports 0 issues for `GOOS=darwin` and `GOOS=windows`, in both modules, through `make pre-add-check` above. CI's
  `make lint` on `ubuntu-24.04` passed too.
- **`go fix -diff ./...`** prints nothing in either module, exit 0, on `4b3797f`.
- **CI** passed on the pushed commits.

The MADR's "Confirmation" for phase 5, "`go fix -diff ./...` reports nothing in either module, and the lint configuration of E16 is
clean", holds by the `make lint` and `go fix -diff` points.

This PLAN and its `docs/README.md` row are `complete`. The MADR stays `accepted`. Phase 5 is the last phase of its program. Schema
`1.26.0` is left to a record of its own (question 16).

**Left to the owner.**

- Tag `v1.16.0`, an annotated tag as the earlier ones are, on this close commit once its CI passes. The code is the same as at
  `82d071d`.
- Publish its release from the release text of
  [0019-REPORT-release-notes-v1-16-0.md](../reports/0019-REPORT-release-notes-v1-16-0.md), with its relative links rewritten to the
  tag.

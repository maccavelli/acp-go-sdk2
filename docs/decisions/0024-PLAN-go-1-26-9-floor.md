---
status: complete
date: 2026-10-19
associated-madr: "0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md"
---

# Implement phase 1 of 0024: the Go 1.26.9 floor

Associated MADR: [0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md)

## Goal

Implement phase 1 of the MADR's outcome, findings G1 to G9, as the owner decided questions 2 and 8 on 2026-10-19:

* both `go` lines become `1.26.9`;
* the tests build and pass on go1.26.9 and go1.27.2;
* the leak guard runs on both versions, and fails with an instruction, not a panic, when the profile is absent;
* CI tests a matrix of go1.26.9 and a pinned go1.27.2 on its three runners, and checks the matrix's floor against the `go` line;
* the README states the minimum Go version;
* 0002's and 0004's Go 1.27.2 floor is marked superseded.

The exported API, the wire bytes and the SDK's behaviour do not change. The release is proposed as the patch `v1.16.2`, which the owner
confirms at the tag.

## Facts this plan rests on

Each was measured or read on `c4777ff`, in the tree or in a scratch copy, on 2026-10-19 (the MADR's "How the assessment was made").
Step 1 re-asserts the code facts with a script.

| For | The tree as it is |
| :--- | :--- |
| G1 | `go.mod:3` and `cmd/generate/go.mod:3` read `go 1.27.2`. At `1.26.9`, both modules build on go1.26.9, and go1.27.2's `vet` reports no newer API once G2 is fixed. `make lint` reports 0 issues and `go fix -diff` prints nothing at that line. |
| G2 | `gates_test.go:440`, `:444`, `:449`, `:454`, `:461`: `return valueGate{wire: …` sets fields of the embedded `gatedMethod`, a Go 1.27 language feature. |
| G3 | `TestCheckAcceptsRepository` (`cmd/generate/main_test.go:315`) fails on go1.26.9 through G2 only. |
| G4 | `cmd/generate/internal/mutate/mutate_test.go:92` and `:287` write `go 1.27.2` into fixture modules. |
| G5 | `leak_test.go:35`, `p.WriteTo(&buf, 1)`, with `p := pprof.Lookup("goroutineleak")`, panics on go1.26.9 unless `GOEXPERIMENT=goroutineleakprofile` is set. go1.27.2 rejects that setting: "go: unknown GOEXPERIMENT goroutineleakprofile". Annotated in step 1: 0023's step 6 (`85ac115`) moved them; `p := pprof.Lookup("goroutineleak")` is at `:37` and `p.WriteTo(&buf, 1)` at `:42`. |
| G6 | go1.27.2 builds with `goexperiment.jsonv2` by default, go1.26.9 without. |
| G7 | `ci.yml:32-34`: `actions/setup-go` with `go-version-file: go.mod`; the matrix is `os` only (`ci.yml:21-24`; corrected in step 1 from `21-23`). |
| G9 | The README's "Installation" (`README.md:12-18`) gives `go get` with no Go version. |
| — | `README.md:146` says an empty `json.RawMessage` "prints as `null` under Go 1.27's default build". What it prints under go1.26.9 is not yet measured. |
| — | The Makefile's `test`, `fuzz`, `bench` and `mutation-*` targets and `scripts/go-precheck.sh` (`:132`) run `go test` with the environment they are given. |

## Scope

In scope:

* both `go.mod` files;
* `gates_test.go`, `leak_test.go` and `cmd/generate/internal/mutate/mutate_test.go`;
* `Makefile` and `scripts/go-precheck.sh`, which set the experiment on Go 1.26;
* `.github/workflows/ci.yml`;
* `README.md`: the minimum Go version and the raw-value note;
* the records:
  * 0002's and 0004's annotation;
  * this PLAN and its row;
  * the release notes `docs/reports/0024-REPORT-release-notes-v1-16-2.md` and their row.

Out of scope:

* phases 2 to 4 of the MADR;
* the 0023 leak guard's false negative, which amends [0023-PLAN-connection-lifetime-and-test-leaks.md](0023-PLAN-connection-lifetime-and-test-leaks.md);
* ~~any `toolchain` directive.~~ Annotated before step 2, 2026-10-19 (see "Deviation, 2026-10-19, before step 2"): both modules gain `toolchain go1.27.2`.

## Design

### The floor (G1 to G4, G9)

* **The `go` lines.** `go 1.27.2` becomes `go 1.26.9` in both modules. No `toolchain` line is added: the matrix pins the versions CI
  runs, and a developer's own Go, 1.26.9 or newer, builds the module.

  Annotated before step 2, 2026-10-19 (see "Deviation, 2026-10-19, before step 2"): both modules gain `toolchain go1.27.2` after the `go` line. In this repository the
  `go` command then runs go1.27.2. CI's `GOTOOLCHAIN: local` and the floor gate's `GOTOOLCHAIN=go1.26.9` keep their versions.
* **G2.** Each of the five literals names the embedded struct, as before `95d66e3`:
  `valueGate{gatedMethod: gatedMethod{wire: …, advertise: advertise}, field: …, variant: variant}`. `go fix` at the `1.26.9` line does
  not propose the flattened form again; the MADR's G1 measured `go fix -diff` empty there.
* **G4.** The two fixture modules write `go 1.26.9`.
* **G9.** "Installation" states: "Requires Go 1.26.9 or newer, the `go` line of `go.mod`."
* **The raw-value note.** Step 2 measures what an empty `json.RawMessage` prints under go1.26.9's default build, with `%v`, `%s` and
  `fmt.Sprint`. The README then states each version's result as measured.

### The leak guard without the profile (G5, question 2)

`waitForLeaks` checks `p == nil` before using `p`. When it is nil, `waitForLeaks` returns a message, and `TestMain` prints it and exits 1:

```text
the goroutineleak profile is not available: on Go 1.26, set GOEXPERIMENT=goroutineleakprofile
(0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md, question 2)
```

The rest of `waitForLeaks` is unchanged by this PLAN.

**Setting the experiment on Go 1.26:**

* The `Makefile`, at its top, appends `goroutineleakprofile` to `GOEXPERIMENT`, and exports it, when `go env GOVERSION` starts with
  `go1.26`. A `GOEXPERIMENT` already set, such as `nojsonv2`, keeps its value, joined with a comma.
* `scripts/go-precheck.sh` does the same before its tests, so the pre-add check runs the guard on either version.
* Annotated during step 2, 2026-10-19 (see "Deviation, 2026-10-19, during step 2"): exported to the whole `Makefile` and script, the setting reaches `golangci-lint`,
  built with Go 1.27, which rejects it. It is applied to the root's `go test` commands only: the `Makefile`'s `LEAKGUARD_ENV`
  prefixes them, and the script sets it on its `go test -race` line.
* A bare `go test` on Go 1.26 without the setting fails with the message above, as decided.

### CI (G7, question 8)

`validate`'s matrix becomes `os` × `go`, with the experiment carried in the matrix:

```yaml
matrix:
  os: [ubuntu-24.04, macos-15, windows-2025]
  go: ['1.26.9', '1.27.2']
  include:
    - go: '1.26.9'
      floor: true
      experiment: goroutineleakprofile
```

* **The Go version.** `actions/setup-go` takes `go-version: ${{ matrix.go }}`. The job sets `GOTOOLCHAIN: local`, so the `go` command
  runs the version installed, never a download.
* **The experiment.** The job's `GOEXPERIMENT` is `${{ matrix.experiment }}`, empty on 1.27.2.
* **The floor check,** on the floor's Linux job:
  `test "$(go list -m -f '{{.GoVersion}}')" = "${{ matrix.go }}"`, run in both modules. It fails when the `go` line and the matrix's
  floor differ.
* **Which steps run where:**
  * the test steps run in all six jobs;
  * the race detector and `govulncheck` run on Linux for both versions, since the standard library they check differs;
  * the `GOEXPERIMENT=nojsonv2` step runs on Linux for 1.27.2 only. 1.26.9's default build already uses the v1 backend, and its job
    tests it.
  * lint, formatting, tidiness and the docs checks run on Linux for 1.27.2 only, as now.

## Implementation Steps

Each step ends with its gates and an execution record. The owner commits it; agents do not commit on `main`. Every new check is seen to
fail on a planted break in a scratch copy, never in the tree, before it is relied on.

1. **Records.**
   * This PLAN and its `docs/README.md` row.
   * 0002 and 0004 each gain a dated annotation: their Go 1.27.2 floor is superseded by the 0024 MADR. Neither is rewritten.
   * The facts above re-asserted on the tree by a script.
2. **The floor:**
   * both `go` lines;
   * G2's literals and G4's fixtures;
   * the leak guard's `nil` check;
   * the `Makefile` and `scripts/go-precheck.sh`;
   * the README: the minimum Go version, and the raw-value note as measured.
3. **CI.** The matrix, `GOTOOLCHAIN: local`, the experiment, the floor check, and the steps' placement. `actionlint` passes.
4. **Release notes.** `docs/reports/0024-REPORT-release-notes-v1-16-2.md` and its `docs/README.md` row.
5. **Verification and close.**
   * The gates over the tree.
   * `make push-guard`.
   * After CI passes on the pushed commits, in all six jobs, this PLAN and its row become `complete`.
   * The owner tags `v1.16.2` and publishes its release from the report.

## Verification

Per code step, through the Python runner outside the tree:

* **The existing gates on go1.27.2:**
  * regeneration;
  * `make pre-add-check`;
  * `GOEXPERIMENT=nojsonv2 go test ./...`;
  * `GOOS=windows go vet ./...` in both modules;
  * `make check-records`, `make markdownlint` and the identifier scan;
  * the API listing against `v1.16.1`;
  * Windows `go test -count=1 -shuffle=on ./...` in both modules.
* **A new gate on go1.26.9.** `go build`, `go vet` and `go test -count=1 ./...` in both modules, with `GOTOOLCHAIN=go1.26.9` and
  `GOEXPERIMENT=goroutineleakprofile`. The toolchain and its caches stay in the session's scratch directory, as the MADR's measurement
  kept them.
* **CI's six jobs** after the push, in step 5.

Planted breaks, at least. Each must be seen to fail on the check named. A break that is not seen to fail stops its step, and the step
adds a check that fails on it.

| For | Break | The check that must fail |
| :--- | :--- | :--- |
| G2 | one literal flattened again | go1.27.2's `go vet` at the `1.26.9` line ("requires go1.27 or later"), and the go1.26.9 gate's build of the tests |
| G4 | one fixture back at `go 1.27.2` | the go1.26.9 gate's `cmd/generate/internal/mutate` tests |
| G5 | the `nil` check removed | the go1.26.9 suite without the experiment panics instead of printing the instruction |
| G5 | a planted leaking test | the guard, on go1.27.2 and on go1.26.9 with the experiment |
| G5 | the `Makefile`'s condition reversed, setting the experiment on 1.27 | `make test` on go1.27.2: "unknown GOEXPERIMENT goroutineleakprofile" |
| G7 | the matrix floor set to `1.26.8` while the `go` line reads `1.26.9` | the floor check's command, run locally with that value |

Expected API listing difference against `v1.16.1`: none.

Acceptance:

* every step's checks pass and its record is written;
* the root suite and the generator's pass on go1.26.9 with the experiment, and on go1.27.2 without it;
* the guard reports the planted leak on both versions;
* CI's six jobs pass on the pushed commits;
* `make lint` reports 0 issues and `go fix -diff` prints nothing at the `1.26.9` line;
* the API listing against `v1.16.1` shows no difference.

## Rollout and Rollback

Each step is one commit, reverted by reverting it.

* Reverting step 2 restores `go 1.27.2`. It needs step 3 reverted too, since the floor check would then fail.
* For consumers, the change only widens the Go versions the module accepts.

The release is the patch `v1.16.2`, which the owner confirms at the tag.

## Execution record

### Step 1, records, 2026-10-19

The owner committed the decided MADR with this PLAN as `805d997`. At the owner's direction, [0023-PLAN-connection-lifetime-and-test-leaks.md](0023-PLAN-connection-lifetime-and-test-leaks.md)'s
leak-guard fix landed first, as its step 6, `85ac115`, closed in `11c50df`.

This step:

* marks this PLAN and its `docs/README.md` row `in-progress`;
* annotates [0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md) and
  [0004-MADR-pin-acp-schema-1-25-0.md](0004-MADR-pin-acp-schema-1-25-0.md), under their decision. Their Go 1.27.2 floor is superseded
  by a `go 1.26.9` line once step 2 lands, and their reason still holds at the new floor. Neither is otherwise rewritten.

A script outside the tree re-asserted "Facts this plan rests on" on `11c50df`, each snippet at its line: 19 facts, all hold, after two
corrections:

* **G5.** 0023's step 6 moved `leak_test.go`'s lines. `p := pprof.Lookup("goroutineleak")` is at `:37`, and `p.WriteTo(&buf, 1)` at
  `:42`, not `:35`. The fact's row is annotated.
* **G7.** The matrix block is `ci.yml:21-24`, not `21-23`. The row is corrected, and says so.

`cmd/generate/main_test.go:315` is the `t.Fatal(err)` of `TestCheckAcceptsRepository` (`:300`), which G3's failure printed. The script's
first expectation of that line was wrong; the fact holds.

Gates: `make check-records`, `make markdownlint` and the identifier scan, each exit 0.

### Deviation, 2026-10-19, before step 2

**Found.** The owner asked whether CI would cover both 1.26.9 and 1.27.2, with 1.27.2 preferred.

* CI covers both once step 3 lands. Today it covers 1.27.2 only, through `go-version-file: go.mod`.
* Nothing in the design made 1.27.2 the version of development. With no `toolchain` line, the `go` command in this repository runs
  whatever Go a developer has, from 1.26.9 up. "Scope" excluded "any `toolchain` directive".

**Decision (owner, 2026-10-19).** Add `toolchain go1.27.2` to both `go.mod` files, after `go 1.26.9`.

**Checked first, in a scratch copy with `go 1.26.9` and `toolchain go1.27.2` in both modules:**

| In each module | Result |
| :--- | :--- |
| `go mod tidy -diff`, go1.27.2 | exit 0, no output |
| `go env GOVERSION`, `GOTOOLCHAIN=local` (as CI's jobs) | `go1.27.2` |
| `go env GOVERSION`, `GOTOOLCHAIN=go1.26.9+auto` (a developer on go1.26.9) | `go1.27.2`: the `toolchain` line switches it |
| `go env GOVERSION`, `GOTOOLCHAIN=go1.26.9` (the floor gate) | `go1.26.9` |

**What it changes, and what it does not:**

* **Users of the module are not affected.** go.dev/doc/toolchain: "The choice of Go toolchain being used depends on the GOTOOLCHAIN
  environment setting and the go and toolchain lines in the main module's go.mod". A dependency is never the main module. So a consumer
  still needs only 1.26.9.
* **The `Makefile`'s experiment condition** reads `go env GOVERSION`, which reports the switched toolchain. A developer on go1.26.9 is
  switched to 1.27.2 and so does not get the experiment, which 1.27.2 would reject. Step 2's planted break of that condition covers it.

**Files added to step 2's scope:** none; both `go.mod` files were in it.

"Scope" and "Design", "The floor", are annotated. The MADR's phase 1 gains an amendment.

### Deviation, 2026-10-19, during step 2

**Found.** A positive check of step 2's planted breaks ran `scripts/go-precheck.sh` on a forced go1.26.9
(`GOTOOLCHAIN=go1.26.9`), in a scratch copy initialised as a git repository.

* It failed. Each of `golangci-lint`'s four runs reported:
  `level=error msg="Running error: context loading failed: … stderr: go: unknown GOEXPERIMENT goroutineleakprofile"`.
* `make lint` failed the same way.
* The design exported `GOEXPERIMENT=goroutineleakprofile` to the whole `Makefile` and the whole script. `golangci-lint` v2.14.0 was
  built with go1.27.2, which rejects the setting.
* `golangci-lint run ./...` on go1.26.9 without the setting exits 0.
* **The exposure is narrow.** A developer on go1.26.9 is switched to go1.27.2 by the `toolchain` line, so the condition is false.
  CI's floor job runs neither lint nor the script. It fails only when 1.26.9 is forced.

**Decision (owner, 2026-10-19).** Apply the setting to the root's `go test` commands only:

* `Makefile`: a variable, `LEAKGUARD_ENV`, empty unless `go env GOVERSION` starts with `go1.26`. It prefixes the root's `go test`
  in `test`, both root fuzz targets, `bench`, and the `go run` of `mutation-generated`.
* `scripts/go-precheck.sh`: it computes `test_experiment` once, and sets `GOEXPERIMENT="$test_experiment"` on `check_module`'s
  `go test -race` only.
* Lint, vet, tidiness and `govulncheck` never see it.

"Design", "The leak guard without the profile", is annotated. A positive check is added: `make lint` on go1.26.9 passes.

**Files added to step 2's scope:** none.

### Step 2, the floor, 2026-10-19

What changed, on `09d4a79`:

* **`go.mod` and `cmd/generate/go.mod`:** `go 1.26.9`, then `toolchain go1.27.2`, by the deviation before this step.
* **`gates_test.go`, G2:** the five `valueGate` literals name the embedded struct, `valueGate{gatedMethod: gatedMethod{…}, …}`.
* **`cmd/generate/internal/mutate/mutate_test.go`, G4:** both fixture modules write `go 1.26.9`.
* **`leak_test.go`, G5:**
  * `TestMain`, after `m.Run`, checks `pprof.Lookup("goroutineleak") == nil`. If so, it prints the instruction "Design" gives and
    exits 1.
  * The check sits in `TestMain` rather than in `waitForLeaks`, so the instruction is printed alone, not under "goroutines leaked by
    the tests". What it prints and how it ends are as designed. `waitForLeaks` is unchanged.
* **`Makefile`, as the deviation during this step decided:** `LEAKGUARD_ENV` is `GOEXPERIMENT=…goroutineleakprofile` when
  `go env GOVERSION` starts with `go1.26` and the setting is not already there, and empty otherwise. It prefixes the root's `go test`
  in `test`, `fuzz` (both root targets) and `bench`, and the `go run` of `mutation-generated`.
* **`scripts/go-precheck.sh`, as decided:** it computes `test_experiment` with a `case` on `go env GOVERSION`, and sets it as
  `GOEXPERIMENT` on `check_module`'s `go test -race` only.
* **`README.md`:**
  * "Installation" states the requirement: Go 1.26.9 or newer, the `go` line, with CI testing 1.26.9 and 1.27.2.
  * The raw-value note adds Go 1.26's output.

**The raw-value note, measured.** A scratch module at `go 1.26.9` printed a nil and an empty `json.RawMessage`:

| Go, default build | nil: `%v`, `%s`, `fmt.Sprint` | empty, not nil |
| :--- | :--- | :--- |
| go1.27.2 | `null`, `null`, `null` | `""`, `""`, `""` |
| go1.26.9 | `[]`, `""`, `[]` | `[]`, `""`, `[]` |

* An absent or `null` `rawInput` decodes to a nil value, on both versions (a probe test in a scratch copy). So the note's 1.27 text,
  which describes the nil value, stands.
* The note adds: "Under Go 1.26's default build it prints as `[]` with `%v` and `fmt.Sprint`, and as an empty string with `%s`."

**The go1.26.9 gate.** In a scratch copy, with `GOTOOLCHAIN=go1.26.9` and `GOEXPERIMENT=goroutineleakprofile`, every command exits 0
in both modules:

* `go version`: `go1.26.9 darwin/arm64`;
* `go build ./...`;
* `go vet ./...`;
* `go test -count=1 ./...`: every package `ok`, the root's leak guard included.

On go1.27.2 in the tree: `go vet ./...` and `go mod tidy -diff` exit 0 in both modules, `go test -count=1` passes in both, and
`make markdownlint` passes.

Planted breaks and positive checks, each in a fresh scratch copy; 14 of 14 as required. The pre-add script's copies were initialised as
git repositories with their files staged, since the script starts from `git rev-parse --show-toplevel`. The script read the installed
`golangci-lint` through `GOLANGCI_LINT`.

| Case | Go | Command | Required | Result |
| :--- | :--- | :--- | :--- | :--- |
| G2: one literal flattened again | go1.27.2 | `go vet .` | fails | "use of promoted field gatedMethod.wire … requires go1.27 or later" |
| G2: the same | go1.26.9 | `go vet .` | fails | "unknown field wire in struct literal of type valueGate" |
| G4: one fixture back at `go 1.27.2` | go1.26.9 | `go test ./internal/mutate` | fails | "go: go.mod requires go >= 1.27.2" in the mutant's error |
| G5: the check, without the experiment | go1.26.9 | `go test -run ^TestTypedGate$ .` | fails | "the goroutineleak profile is not available: on Go 1.26, set GOEXPERIMENT=goroutineleakprofile" |
| G5: the check removed | go1.26.9 | the same | fails | "panic: runtime error: invalid memory address or nil pointer dereference" |
| G5: a planted leaking test | go1.27.2 | that test alone | fails | "goroutines leaked by the tests …" |
| G5: the same, with the experiment | go1.26.9 | that test alone | fails | "goroutines leaked by the tests …" |
| the `Makefile` as it is | go1.26.9 | `make test` | passes | `ok` |
| the `Makefile` as it is | go1.27.2 | `make test` | passes | `ok` |
| the `Makefile`'s condition reversed | go1.27.2 | `make test` | fails | "go: unknown GOEXPERIMENT goroutineleakprofile" |
| the `Makefile`'s condition removed | go1.26.9 | `make test` | fails | the guard's instruction |
| the `Makefile` as it is | go1.26.9 | `make lint` | passes | "0 issues." for each run |
| `go-precheck.sh` as it is | go1.26.9 | the script, `GO_PRECHECK_SKIP_VULN=1` | passes | "go-precheck: 136 file(s) clean (gofmt, golangci-lint, go vet, go test)" |
| `go-precheck.sh`'s condition removed | go1.26.9 | the same | fails | the guard's instruction |

The last three rows came after the deviation during this step. Before it, the script on go1.26.9 failed on `golangci-lint`.

Gates on go1.27.2, with the API listing against `v1.16.1`, each exit 0:

* regeneration: the generated files unchanged by a second run;
* `make pre-add-check`;
* `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint`, and the identifier scan;
* the API listing: 2429 lines at `v1.16.1` and in the tree, 0 removed, 0 added;
* Windows, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0.

### Step 3, CI, 2026-10-19

What changed, in `.github/workflows/ci.yml` only:

* **The matrix** is `os` × `go`: `ubuntu-24.04`, `macos-15`, `windows-2025` by `1.26.9`, `1.27.2`, six jobs. An `include` adds
  `floor: true` and `experiment: goroutineleakprofile` to the three `1.26.9` jobs.
* **`actions/setup-go`** takes `go-version: ${{ matrix.go }}`.
* **The job's environment:**
  * `GOTOOLCHAIN: local`, so the `go` command runs the version installed, not the `toolchain` line's go1.27.2;
  * `GOEXPERIMENT: ${{ matrix.experiment }}`, empty in the `1.27.2` jobs.
* **"the floor is the go line"**, on the floor's Linux job, as "Design" sets it. It runs `go list -m -f '{{.GoVersion}}'` in both
  modules against `matrix.go`. It also checks the third field of `go version` against `go${{ matrix.go }}`. "Design" does not name that
  line; it confirms that the job runs the version the matrix installed.
* **Where each step runs:**
  * the test steps run in all six jobs;
  * the race detector and `govulncheck` run on Linux for both versions;
  * the `nojsonv2` step and the lint and docs steps run on Linux for `1.27.2` only, as `if: runner.os == 'Linux' && !matrix.floor`.
    The `nojsonv2` step's own `GOEXPERIMENT` overrides the job's.

**Checked, in scratch copies, before the push:**

* **`actionlint` v1.7.12**, the version CI runs, exits 0 on the new workflow. With `matrix.floor` misspelled `matrix.flor`, it exits 1:
  "property "flor" is not defined in object type {experiment: string; floor: bool; go: string; os: string}".
* **`govulncheck` v1.9.0 builds on the floor.** Its `go.mod` reads `go 1.26.0`.
* **The floor job's steps.** Run on go1.26.9, with `GOEXPERIMENT=goroutineleakprofile`, in a scratch copy, as `ci.yml` runs them on
  Linux. 8 of 8 as required:

  | Step | Result |
  | :--- | :--- |
  | the floor is the go line | exit 0 |
  | the same, with `1.26.8` as the matrix value | exit 1, as the planted break "Verification" names for G7 requires |
  | `go mod download` | exit 0, "no module dependencies to download" |
  | `go test -shuffle=on ./...` | exit 0, every package `ok` |
  | test generator module | exit 0, every package `ok` |
  | `go build ./example/...` | exit 0 |
  | race detector | exit 0, both modules |
  | `govulncheck`, both modules | exit 0, "No vulnerabilities found." twice |

The `1.27.2` jobs run the steps as before this step: `GOTOOLCHAIN: local` on go1.27.2 is today's default, and their `GOEXPERIMENT` is
empty. The six jobs run for real only once the commits are pushed, so step 5 records them.

Gates on go1.27.2, with the API listing against `v1.16.1`, each exit 0:

* regeneration: the generated files unchanged by a second run;
* `make pre-add-check`;
* `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint`, and the identifier scan;
* the API listing: 2429 lines at `v1.16.1` and in the tree, 0 removed, 0 added;
* Windows, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0.

### Step 4, release notes, 2026-10-19

What changed, on `1179253`:

* [0024-REPORT-release-notes-v1-16-2.md](../reports/0024-REPORT-release-notes-v1-16-2.md), the draft of the GitHub release text for
  `v1.16.2`, in the shape of the `v1.16.1` notes;
* its `docs/README.md` row.

Facts the notes rest on, each read on `1179253`:

* **The schema:** `git diff v1.16.1 HEAD -- schema/` is empty.
* **No SDK code changes:** `git diff --stat v1.16.1 HEAD -- '*.go' ':!*_test.go'` lists no file.
* **The `go.mod` change:** both files change `go 1.27.2` to `go 1.26.9` and add `toolchain go1.27.2`, and nothing else.
* **The commits** since `v1.16.1` and what each changes, by `git show --name-only`:
  * `df19114`, `1179253` and `85ac115` change code, tests, CI or the README;
  * `ce49693`, `c4777ff`, `805d997`, `11c50df` and `09d4a79` change records only.
* **The API listing:** 2429 lines at `v1.16.1` and at `1179253`, 0 removed, 0 added (step 3's gates).

The owner pushed steps 1 to 3 after step 3, `85ac115..1179253`; `make push-guard` passed over the four commits first. CI's first
six-job run, `39325017852` on `1179253`, is recorded in step 5.

Gates: `make check-records`, `make markdownlint` and the identifier scan, each exit 0. The step changes no Go file.

### Step 5, verification and close, 2026-10-19

**The pushes.**

* **After step 3.** `make push-guard` ran over the four commits not yet pushed, `85ac115..1179253`, and passed, exit 0. The owner pushed
  `1179253`.
* **CI run `39325017852` on `1179253`,** the first of the matrix, concluded `success` on its first attempt in all six jobs:
  `ubuntu-24.04`, `macos-15` and `windows-2025`, each with `1.26.9` and `1.27.2`.
* **Its two Linux jobs' steps:**
  * **`1.26.9`:** "the floor is the go line" `success`; the `nojsonv2` step and the lint and docs steps `skipped`; the tests, the race
    detector and `govulncheck` `success`.
  * **`1.27.2`:** "the floor is the go line" `skipped`; every other step `success`.
* **Still to push:** step 4's commit, `a162ba6`, release notes only, and this close. CI on that push is the tag's precondition.

**The gates over the tree**, on `a162ba6`, with the API listing against `v1.16.1`, each exit 0:

* regeneration: the generated files unchanged by a second run;
* `make pre-add-check`;
* `GOEXPERIMENT=nojsonv2 go test -count=1 ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint`, and the identifier scan;
* the API listing: 2429 lines at `v1.16.1` and in the tree, 0 removed, 0 added;
* Windows, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0.

**Acceptance.** Every criterion in "Verification" is met:

* **Each step's checks** passed and its record is written, with the deviations before step 2 (the `toolchain` line) and during it (the
  experiment's scope).
* **Both suites pass on both versions:**
  * go1.26.9 with the experiment: step 2's gate, and CI's three `1.26.9` jobs;
  * go1.27.2 without it: the gates above, and CI's three `1.27.2` jobs.
* **The guard reports the planted leak on both versions:** step 2's planted breaks.
* **CI's six jobs passed** on the pushed commits.
* **At the `1.26.9` line,** `make lint` reports 0 issues, through `make pre-add-check` above, and `go fix -diff ./...` prints nothing in
  either module, exit 0, on `a162ba6`.
* **The API listing** against `v1.16.1` shows no difference.

This PLAN and its `docs/README.md` row are `complete`. The MADR stays `accepted`, with phases 2 to 4 to come, each with a PLAN of its
own.

**Left to the owner.**

* Push step 4's commit and this close, after `make push-guard`.
* Tag `v1.16.2` on this close commit once its CI passes, an annotated tag whose message is its name, as `v1.16.1` is. Its code is the
  same as at `1179253`.
* Publish its release from the release text of
  [0024-REPORT-release-notes-v1-16-2.md](../reports/0024-REPORT-release-notes-v1-16-2.md), with its relative links rewritten to the
  tag.

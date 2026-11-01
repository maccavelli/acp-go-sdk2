---
status: accepted
date: 2026-11-01
decision-makers: the repository owner
consulted: "`go help test` (Go 1.27.3), on test caching; probes in scratch exports of `cc2a72d`"
informed: none
---

# Run the pre-add check's vet and tests over both modules, uncached, whenever a named file can reach a test

## Context and Problem Statement

`make pre-add-check` (`scripts/go-precheck.sh`) is the check `AGENTS.md` requires before Go files are staged, and the one each
PLAN's phases run on the files they change. Given files, it tests only the packages that hold them. Twice it has passed a change
that broke tests elsewhere:

* [0036-PLAN-session-update-constructors-kind-and-client-refusals.md](0036-PLAN-session-update-constructors-kind-and-client-refusals.md),
  2026-10-25: `refusals.go`, a new root file, broke `cmd/generate`'s `TestRuntimeFilesCoverHandWritten`. "`make pre-add-check`
  does not run the `cmd/generate` tests, so it passed."
* [0046-PLAN-sending-and-receiving-middleware.md](0046-PLAN-sending-and-receiving-middleware.md), deviation 3, 2026-10-31:
  `connection.go`'s calls into `middleware.go` and `observer.go` broke every engine case from phase 2 on. Four phases' checks
  passed; the `floor` and host gates of phase 6 found it. Deviation 3 left "`go-precheck.sh`'s module selection" for a record of
  its own. This is that record.

The owner asked for this to be researched so that it accounts for the edge cases and gives complete coverage. The question: what
must the pre-add check run so that no test whose result a change can alter is skipped, or answered from a stale cache?

## Findings

From a read-only investigation of `cc2a72d` on 2026-11-01, and probes in scratch exports and clones of it on this macOS host, Go
`1.27.3`, `darwin/arm64`. Nothing ran in the tree.

### F1. What the check runs today

* `scripts/go-precheck.sh:42-58`: named `.go` files that exist are checked; a named `.go` file that does not is taken as a
  deletion, and its directory is kept; any other named file is ignored. With no file named, every tracked `.go` file.
* `:60-63`: with no `.go` file named, it prints "no Go files to check" and exits 0, running nothing: no lint, no tidy.
* `:76-85`: gofmt on the named files. `:98-107`: golangci-lint, both modules, for the host and `GOOS=windows`, always.
  `:203-212` and `:235-244`: `go mod tidy -diff` and govulncheck, both modules, always.
* `:153-200`: `go vet` and `go test -race` on the packages of the named files only (`module_of`, `pkg_of`, `consider`); a
  deleted directory with no `.go` file left takes its whole module. With no file named, both modules, `./...`.
* `:146`: `go test -race` without `-count` or any other uncacheable flag, so its results are cached.

### F2. Who calls it

* `Makefile:141-143`, `make pre-add-check [FILES=...]`: the agents' and the PLANs' per-phase check, with `FILES`.
* `tools/gates.py:70`, the `precheck` gate: no file. `tools/gates.py:73-77`, the `newfiles` gate: the untracked `.go` files.
* `RELEASING.md:30` and `:43`: `make release` and the manual steps run it with no file, after `make test`.
* No git hook calls it, in the repository or in the global hooks directory.

### F3. A change reaches tests beyond its own package, five ways

1. **By import, within a module.** Every package of the root module imports the root package, in its code or its tests:
   `acptest`, `agentproc`, `clienthost`, `remote` and the five `example/` packages (`go list -test -f '{{.Imports}}' ./...`).
   Naming `connection.go` tests `.` alone.
2. **By path, across modules.** `cmd/generate` is a separate module whose tests read the root's files:
   * `cmd/generate/main_test.go:24`, `repoRoot = "../.."`; `:50` and `:60` read the root's generated files and `unstable/`, to
     compare the generator's output with them;
   * `cmd/generate/engine_test.go:208` lists the root's hand-written files (`TestRuntimeFilesCoverHandWritten`);
   * `cmd/generate/internal/enginetest/api.go:31-42`, `CopyRuntime`, copies `RuntimeFiles` from the root for every engine case;
   * `cmd/generate/behaviour_test.go:34` reads the root's `go.mod`;
   * `cmd/generate/internal/model/keywords_test.go:74` and `sources_test.go:50` read `schema/`.
3. **Through a subprocess.** `example_pair_test.go:32-33`, `TestExamplePair` in the root package, runs `go build` on
   `./example/agent` and `./example/client`. `example/client` imports `agentproc`, which the root test binary does not contain.
4. **By path, within a module, from files that are not Go.** `README.md` (`docs_test.go:164`, `:190`, `:216`, `:243`); `schema/`
   (`instances_test.go:45`, `kind_test.go:255`, `session_update_helpers_test.go:48`); `testdata/` (`compaction_test.go:187`,
   `elicitation_check_test.go:50`, `encodings_test.go:36`, `fuzz_test.go:45`, `json_parity_test.go:27`,
   `tooltimeline_test.go:178`); `acptest/golden_internal_test.go:17` reads `../constants_gen.go`. And `go.mod`, `go.sum` and
   `.golangci.yml` reach the build or the lint. Named alone, any of these runs nothing (F1).
5. **Through the environment.** `acptest/golden.go:102`: with `ACPTEST_UPDATE=1` set, golden tests rewrite their golden files
   instead of comparing. Set in the shell that runs the check, it would write the tree.

### F4. The test cache does not see ways 2 and 3

`go help test`: "Tests that open files within the package's module or that consult environment variables only match future runs
in which the files and environment variables are unchanged." A file outside the package's module, or one a subprocess reads, is
not re-checked. The check runs `go test` cached (F1), so even its full run can answer from a result recorded before the change.

A probe (`cache_probe.py` in the session's scratch directory), each case in a fresh export of `cc2a72d`: warm the cache with
`go test -race ./...`, make one change, then run again with the cache and with `-count=1`:

| Change | Reaches | Cached run | `-count=1` |
| :--- | :--- | :--- | :--- |
| `example/client/main.go` prints `stopped, reason` | `TestExamplePair` (way 3) | `ok … (cached)`, exit 0 | `FAIL` in the root package, exit 1 |
| a comment in `agentproc/agentproc.go` | `TestExamplePair` (way 3) | `ok … (cached)` | ran, `ok` |
| a comment in `connection.go` | `cmd/generate`'s tests (way 2) | `ok … (cached)` | ran, `ok` |

The first row is a failing test reported as passing. The other two show the cache reusing a result that the change reached.

This also explains deviation 3's first gate run: `precheck`, which runs both modules with no file named, was one of the fifteen
gates `ok` on `f8e7ecb`, while every engine case failed in `floor`, which runs `-count=1`. `cmd/generate`'s files had not changed
since its last cached run. This is inferred: that run's output was not kept.

### F5. The rest of the gate set and CI run uncached

`tools/floor.py:55-57` (`-count=1`), `tools/remote.py:72` (`-count=1 -shuffle=on`), `tools/gates.py:80` (`nojsonv2`,
`-count=1`) and `:112` (`race3`, `-count=3 -shuffle=on`, root package only), and CI (`.github/workflows/ci.yml:110-125`,
`-shuffle=on`, on fresh runners). Only `scripts/go-precheck.sh:146` and `make test` (`Makefile:62-65`) use the cache. `make test`
is in `RELEASING.md`'s schema bump, where `make version` changes `schema/` and the generated files that `cmd/generate`'s tests
read across the module boundary.

### F6. Edge cases of the file list

* `module_of` (`:109-114`) matches `cmd/generate/` as a prefix, so `./cmd/generate/main.go` or an absolute path is taken as the
  root module's, and `go vet ./cmd/generate` fails loudly at the root.
* A named `.go` file under `testdata/` becomes a package path that `./...` would never test.
* A Windows-only file (`_windows.go`) is linted for `GOOS=windows`, but vetted and tested only on the host, which excludes it.
  The Windows host gate and CI test it.
* With no file named, gofmt checks tracked files only; an untracked new file is formatted only by the `newfiles` gate.

### F7. The cost

Timed in a scratch clone of `cc2a72d` (`timing.py`), with the build cache warm and the tests uncached:

| Run | Time |
| :--- | :--- |
| today, `FILES=connection.go`: the root package alone, govulncheck skipped | 50.1 s |
| today, no file, in a clone whose test results are not cached | 166.0 s |
| golangci-lint, both modules, host and windows | 8.2 s |
| `go vet ./...`, both modules | 1.5 s |
| `go test -race -shuffle=on ./...`, root module | 31.1 s |
| `go test -race -shuffle=on ./...`, `cmd/generate` | 83.6 s |
| `go mod tidy -diff`, both modules | 0.2 s |

So both modules' vet and tests, uncached, cost about 125 s here before govulncheck, against 50 s for one root file today.
`cmd/generate`'s main package is 82 s of its 84, most of it `TestEngineBehaviour`, the test that found deviation 3.

### F8. One unexplained failure

In the probe's first export, one root package failed in each of its three runs (`-count=1`, warm-up, cached), and the agentproc
case's warm-up failed once. The probe kept only exit statuses. Thirteen later runs of the root suite, uncached, warm and cached,
in fresh exports, all passed. An uncached check will meet such a failure where a cached one would not; a PLAN that runs it keeps
the output of any failure.

## Decision Drivers

* **Complete:** every test whose result a change can alter runs, by any of F3's five ways, including ways a static list cannot
  name.
* **Never stale:** no result recorded before the change is reused.
* **Agreeing with CI:** the check fails before a push where CI would fail after it.
* **Simple to keep correct:** no list of which files reach which tests, which the next test that reads a file would make wrong.
* **Bounded cost:** a check run at each phase and each commit, not at each edit.

## Considered Options

* **A.** Keep the selection, and add `cmd/generate`'s tests when a root file is named: deviation 3's minimum.
* **B.** Select by an affected set: reverse imports from `go list -deps -test`, plus a declared map of the inputs that are not
  imports, cached.
* **C.** Both modules whole, the root cached, with `TestExamplePair` made to read its inputs in the test process, and
  `cmd/generate` uncached.
* **D.** Both modules whole and uncached, `-race -shuffle=on` as CI's race step, whenever a named file is outside CI's
  docs-safe set.

## Decision Outcome

Chosen option: **"D"**, proposed, because it is the only option complete by construction. It runs what CI runs, so no test is
skipped and no result is stale, and it needs no list that a new test could falsify. It costs about 75 s more than testing one root
package (F7).

The parts:

* **D1. Vet and test cover both modules, always.** `go vet ./...` and `go test -race -shuffle=on ./...`, at the root and in
  `cmd/generate`, whatever files are named. `-shuffle=on` is CI's flag, and disables the cache as `-count=1` does; the seed is
  printed when a test fails. The root keeps the `goroutineleakprofile` experiment on Go 1.26 (`:26-38`). `module_of`, `pkg_of`
  and `consider` go: no path is mapped to a package any more, which removes F6's first two cases.
* **D2. What a named file decides.** gofmt checks the named `.go` files that exist. Whether the Go checks run at all: when every
  named file is docs-safe, the paths CI's `paths-ignore` lists (`.github/workflows/ci.yml:11-20`), it prints so and exits 0; any
  other file, Go or not, runs every check. `scripts/check_ci_filters.py` gains `--docs-safe PATH...`, which normalizes each path
  to the repository root, matches it with the module's existing `glob_re`, and exits 0 only when every path matches. The CI
  filter check already refuses a docs-safe path that Go code reads ("readers"), so the two definitions cannot part.
* **D3. With no file named,** gofmt checks every tracked and every untracked, not ignored `.go` file
  (`git ls-files --cached --others --exclude-standard`), so a new file is formatted before it is first staged.
* **D4. The environment.** The tests run with `ACPTEST_UPDATE` removed, so the check never rewrites a golden file (F3, way 5).
* **D5. The `newfiles` gate** runs gofmt on the untracked `.go` files only. The `precheck` gate runs the suites, and lint, vet and
  tests read the files on disk, untracked ones too; running the suites twice adds about two minutes to the gate set and finds
  nothing more.
* **D6. `make test`** runs `go test -shuffle=on ./...` in both modules, so the schema bump in `RELEASING.md` cannot pass on a
  cached result (F5).
* **D7. The words.** The script's header; `AGENTS.md`'s "Pre-add checks"; `docs/architecture.md`'s paragraph on the check;
  `tools/gates.py`'s docstring, which `gates.py --help` prints. Deviation 3 in the 0046 PLAN and the 0036 PLAN's entry point
  here.

Not decided here: Windows-only files stay the Windows host gate's and CI's (F6); the floor toolchain stays `tools/floor.py`'s; the
`nojsonv2` build stays its gate's.

### Questions for the owner

* **Q1. Uncached root, or C's cached root?** (a) D, uncached (recommended): up to 31 s more than C, when nothing in the root
  module changed, and nothing to maintain. (b) C: the root cached, `TestExamplePair` changed to open, in the test process, every in-module file of every package
  `go list -deps` gives for the two examples, and `go.mod` and `go.sum`, so the cache sees them. C is correct only while every
  test that builds or runs code in a subprocess does the same, a rule nothing enforces.
* **Q2. `-shuffle=on` or `-count=1`?** (a) `-shuffle=on` (recommended): CI's flag, so an order dependence fails here first, and
  the seed is in the output. (b) `-count=1`: the fixed order, as `floor` uses.
* **Q3. Named files that are all docs-safe:** (a) skip the Go checks (recommended), as CI does. (b) Run everything for any file.

#### Answered by the owner, 2026-11-01

Each question was put with three options; the owner chose the recommended one each time:

* Q1: **uncached, both modules.** Not chosen: the root cached with `TestExamplePair` opening its inputs; both modules uncached
  but run in parallel, which saves wall time and doubles the load.
* Q2: **`-shuffle=on`.** Not chosen: `-count=1`; `-shuffle=on -failfast`.
* Q3: **skip when every named file is docs-safe, by CI's list.** Not chosen: always run everything; a hard-coded `docs/**/*.md`
  rule in the script.
* The order of work: **this record's PLAN first**, then
  [0046-PLAN-sending-and-receiving-middleware.md](0046-PLAN-sending-and-receiving-middleware.md)'s phase 8, so that phase is
  checked by the new check. Not chosen: phase 8 first; the two interleaved by phase.

The decision stays `proposed` until the owner accepts it.

*2026-11-01:* the owner committed the pair with these answers as `f00a567` and said "committed, proceed". The decision is
`accepted`.

### Consequences

* Good, because a change can no longer pass the check while breaking a test in another package, another module, a subprocess
  build, or through a file that is not Go. Deviation 3's tree and F4's first probe fail it (Confirmation 1 and 2); 0036's miss
  took the same way 2.
* Good, because the check no longer passes on a stale cached result, here or in `make test`.
* Good, because the check now agrees with CI's race step, so a failure shows before the push.
* Good, because about 75 lines of path-to-package mapping (`:109-135`, `:153-200`) go, and their edge cases with them.
* Good, because a change to `README.md`, `schema/`, `testdata/`, `go.mod` or `.golangci.yml` alone is checked, where today it
  runs nothing.
* Bad, because a check on one root file takes about 125 s instead of 50 s here, and a docs-safe change outside CI's list, such as
  a `tools/` script, runs the suites too.
* Bad, because an intermittent failure shows wherever an uncached run meets it (F8). That is information, but it can stop a
  commit that changed nothing near it.
* Neutral, because the `precheck` gate already ran both modules; it becomes uncached, and `newfiles` stops repeating it.

### Confirmation

Each is seen in the PLAN, in scratch clones, never in the tree:

1. The 0046 deviation 3 tree, `f8e7ecb`, fails the new check with `FILES=connection.go`, and passes the old one.
2. F4's first probe, an `example/client` change, fails the new check with `FILES=example/client/main.go`, with a warm cache.
3. A change to `README.md` that breaks `TestReadmeMiddlewareIsTheExample`, named alone, fails the new check; the old check
   exits 0 with "no Go files to check".
4. A `go.mod` that `go mod tidy -diff` refuses, named alone, fails the new check.
5. Named docs-safe files alone, `docs/README.md` and `AGENTS.md`, exit 0 without running the Go checks; adding `README.md` runs
   them.
6. A second run of the check, with nothing changed, prints no `(cached)` line.
7. With `ACPTEST_UPDATE=1` in the environment, the check leaves the tree unchanged: `git status --porcelain` is empty after it.
8. An unformatted untracked `.go` file fails the check with no file named.
9. `--docs-safe`'s unit tests, in `scripts/test_check_ci_filters.py`, each seen to fail on a planted break.
10. `make check-tools`, `tools/gates.py --all` and CI pass.

## Pros and Cons of the Options

### A. Keep the selection, add `cmd/generate` for root files

* Good, because it is the smallest change, and closes the two misses that happened.
* Bad, because ways 1, 3, 4 and 5 stay open, and the cache can still answer `cmd/generate` with a stale result (F4).

### B. An affected set from imports and a declared map

* Good, because it runs the fewest tests.
* Bad, because ways 2 to 4 are not imports. The map must list every file a test opens and every subprocess build, by hand, and
  the next test that reads a file makes it wrong without failing anything.
* Bad, because it stays cached, so it needs C's rule as well.

### C. Both modules whole, the root cached with tracked inputs, `cmd/generate` uncached

* Good, because it saves about 31 s at the root when nothing there changed.
* Bad, because its correctness rests on every subprocess-running test opening its inputs itself, which nothing checks.
* Bad, because `cmd/generate` must be uncached anyway, and it is most of the time.

### D. Both modules whole and uncached

* Good, because it is complete by construction and matches CI.
* Good, because it is the simplest script.
* Bad, because it is the slowest: about 125 s here.

## More Information

* Related records: [0003-PLAN-install-fleet-agent-workspace.md](0003-PLAN-install-fleet-agent-workspace.md), which installed the
  script; [0031-MADR-verification-tools-in-repository.md](0031-MADR-verification-tools-in-repository.md), the gate set;
  [0037-MADR-align-docs-ci-and-disclosure-gates.md](0037-MADR-align-docs-ci-and-disclosure-gates.md), the docs-safe set and its
  check; and the two misses above.
* The per-package selection predates the records: it is in the repository's first commit, `9fb9ae5`.
* No exported API changes, and nothing is released.

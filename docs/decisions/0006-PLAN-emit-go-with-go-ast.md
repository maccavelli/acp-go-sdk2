---
status: complete
date: 2026-10-09
associated-madr: "0006-MADR-emit-go-with-go-ast.md"
---

# Implement Emit generated Go with go/ast instead of Jennifer

Associated MADR: [0006-MADR-emit-go-with-go-ast.md](0006-MADR-emit-go-with-go-ast.md)

## Goal

Replace Jennifer with `go/ast` and `go/format` in `cmd/generate`, and leave the generated SDK files unchanged.

## Scope

In scope:

* `cmd/generate/internal/emit/` — the four writers and `jenwrap.go`
* Amended 2026-10-09: `cmd/generate/internal/emit/dispatch_helpers.go`, which also builds Jennifer code, and the new `ast.go` and `file.go` that replace `jenwrap.go`
* `cmd/generate/main.go` call names once the `Jen` suffix no longer matches
* Amended 2026-10-09: `cmd/generate/main.go` returning errors instead of panicking, and searching for the repository root only when a flag is missing
* Amended 2026-10-09: new tests `cmd/generate/main_test.go` and `cmd/generate/internal/emit/file_test.go`
* `cmd/generate/go.mod` and `cmd/generate/go.sum` when Jennifer is the last requirement to drop

Out of scope:

* `internal/load` and `internal/ir`, except a compile fix caused by an emit signature change
* The module rename in [0005-PLAN-separate-module-from-coder.md](0005-PLAN-separate-module-from-coder.md)
* Hand-editing `*_gen.go`
* ~~A new branch, a commit,~~ a tag, or a push (amended 2026-10-09: the branch and commits are named in [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md))
* Amended 2026-10-09: any change to the generated output, including the quirks and defects listed in [0006-MADR-emit-go-with-go-ast.md](0006-MADR-emit-go-with-go-ast.md). The empty-diff rule holds for this whole plan
* Reverting uncommitted edits already in the working tree

## Implementation Steps

1. ~~Stay on the current branch. Do not create a branch. Do not commit.~~ Amended 2026-10-09: work on the branch that [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md) names.
   Commit after each of steps 3, 4, and 5 once the pre-add checks in `AGENTS.md` pass.
   Superseded 2026-10-09: commits land on `main`; see [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md#amendment-2026-10-09-commits-land-on-main).
2. Add local constructors that return `go/ast` nodes, and a writer that prints an `*ast.File` with `go/format`. Do not import Jennifer from the new writer.
   Amended 2026-10-09: the writer follows the layout design in the 2026-10-09 amendment of [0006-MADR-emit-go-with-go-ast.md](0006-MADR-emit-go-with-go-ast.md): a synthetic `token.File`, 4 KiB lines,
   function bodies reserved on their lines, `go/format.Node` then `go/format.Source`. Appendix A of [0008-REPORT-self-contained-sdk-review.md](../reports/0008-REPORT-self-contained-sdk-review.md) is a verified reference implementation
   (`ast.go`, `file.go`). Add `TestRenderLayout`, `TestStructTagQuoting`, and `TestLayoutStaysBelowPrinterSentinel` from that appendix in this step.
3. Port `WriteConstantsJen` to that writer. Regenerate `constants_gen.go`. Compare it with the file from before this step after `gofmt` on both. An empty diff is required. A nonempty diff stops this plan.
4. ~~Port `WriteHelpersJen` the same way, then `WriteDispatchJen`, then `WriteTypesJen`.~~ Amended 2026-10-09: port `WriteDispatchJen` with
   `dispatch_helpers.go`, then `WriteTypesJen` and `WriteHelpersJen` together. `helpers.go` calls `jenTypeFor`, `expandAllOf`, `generateNestedTypeName`,
   `singleRefVariant`, and `isStringConstUnion` in `types.go`, so porting helpers first would need temporary go/ast copies of them.
   After each writer, regenerate only the files that writer owns and require an empty `gofmt` diff before the next writer.
   Amended 2026-10-09, before step 5: build the Jennifer generator and the go/ast generator as separate binaries. Run both from the repository root on the
   schema files at the 13 commits listed in Appendix B of the report (stamps `0.4.5` to `1.25.0`, with and without unstable files), each extracted with
   `git show` into a scratch directory outside the tree. Every output file must be byte-identical. A difference stops this plan.
5. When all five generated files match, delete `jenwrap.go` and every import of `github.com/dave/jennifer/jen`. Remove the requirement from `cmd/generate/go.mod`. Tidy that module so `go.sum` no longer names it, and delete `go.sum` if tidy leaves it empty. Do not add a `replace` or an `exclude`. Rename the `Write*Jen` functions so the names no longer say Jen. Update `main.go`.
   Amended 2026-10-09: `main.go` returns errors through `run() error` and exits 1, and it calls `findRepoRoot` only when `-schema` or `-out` is empty.
   Add `TestGeneratedFilesUpToDate` in `cmd/generate/main_test.go`.
6. Run `go test ./...` at the repository root and in `cmd/generate`.
7. Amended 2026-10-09: on a scratch copy outside the tree, break each new test's subject once and confirm the test fails: append a line to `types_gen.go`;
   change one emitted error string; split empty-struct braces; put function braces on one line; raise the line length to `1<<20`.
   Record each failure message in the execution record.

The files each writer owns:

* constants: `constants_gen.go`
* helpers: `helpers_gen.go`
* dispatch: `agent_gen.go` and `client_gen.go`
* types: `types_gen.go`

## Verification

* No `.go` file imports `github.com/dave/jennifer/jen`.
* Neither `go.mod` requires, replaces, or excludes `github.com/dave/jennifer`.
* `cmd/generate/go.sum` does not name `github.com/dave/jennifer`. If the sum file would be empty, it is deleted.
* `go list -m all` in `cmd/generate` does not list `github.com/dave/jennifer`.
* `gofmt` of the five generated files matches the pre-port bytes after `gofmt`.
* `go test ./...` exits 0 at the root and in `cmd/generate`.
* `gofmt -l` on the edited generator Go files prints nothing.
* Amended 2026-10-09: `make pre-add-check` passes, including `golangci-lint` for the host and for `GOOS=windows` and `go test -race`.
* Amended 2026-10-09: `TestGeneratedFilesUpToDate`, `TestRenderLayout`, and `TestLayoutStaysBelowPrinterSentinel` pass, and step 7 recorded each one failing.
* Amended 2026-10-09: the step 4 cross-commit comparison found no difference on any of the commits.
* Amended 2026-10-09: the generator binary built from `cmd/generate`, run with `-schema` and `-out` set from a directory outside the repository, exits 0.

## Rollout and Rollback

~~No branch, no commit, no tag, and no push in this plan. Rollback is restoring `cmd/generate` and the generated files from the pre-port tree.~~
A nonempty generated diff is not fixed by editing the generated file or by weakening the comparison.

Amended 2026-10-09: no tag and no push. The plan lands as commits on the branch named in [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md). Rollback is `git revert` of those commits.
The generated files do not change in this plan, so a revert never needs a regeneration.

Superseded 2026-10-09: commits land on `main`; see [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md#amendment-2026-10-09-commits-land-on-main).

## Execution record

### Steps 2 and 3, 2026-10-09

Executed on `main` from a clean tree at `c3883f0`, as step 3 of [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md). The agent did not commit.

Step 2 added `cmd/generate/internal/emit/ast.go`, `file.go`, and `file_test.go`, copied from the verified prototype that Appendix A of [0008-REPORT-self-contained-sdk-review.md](../reports/0008-REPORT-self-contained-sdk-review.md) reproduces,
and checked with `cmp` when copied. Two differences from that appendix, both organizational:

* `fieldSet` and `structField` sit in `ast.go`. The appendix keeps them in `types.go`, which is still the Jennifer emitter until step 4, and the layout tests
  need them now.
* `ast.go` holds only the functions that step 3 and the tests use (30 top-level functions and methods). The first `make pre-add-check` failed with `unused: 26` from `golangci-lint`, for
  `assign`, `binary`, `boolLit`, `byteSlice`, `bytesOf`, `caseOf`, `define`, `emptyBytes`, `exprs`, `ifElse`, `ifStmt`, `incr`, `index`, `mapOf`, `method`,
  `nilExpr`, `not`, `paren`, `rangeStmt`, `rawMap`, `sliceOf`, `switchStmt`, `typeAssert`, `typeStmt`, `varStmt`, and `(*fieldSet).interfaceType`. They were
  removed and come back from the appendix in the step that first uses them. A lint suppression was not used.

Step 3 replaced the body of `WriteConstantsJen` with the go/ast writer. The shared `const` block moved from `jenwrap.go` to `constants.go` unchanged. The
function keeps its `Jen` name until step 5. `jenwrap.go` now holds only the Jennifer aliases.

Verification, as observed:

* The generator, run into a scratch directory, wrote `constants_gen.go` byte-identical to the committed file, before and after `gofmt`. The other four files,
  still from Jennifer, were byte-identical as well.
* `go test ./internal/emit`: `TestRenderLayout`, `TestStructTagQuoting`, and `TestLayoutStaysBelowPrinterSentinel` pass. Step 7 records their failures.
* `make pre-add-check`: `go-precheck: 44 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` That run checks tracked files only, so it was
  repeated with the five changed and new files named: `go-precheck: 5 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).`
* `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits steps 2 and 3 on `main`.

### Step 4, 2026-10-09

Steps 2 and 3 were committed by the owner as `53ae152`. Step 4 ran on `main` from that clean tree. The agent did not commit.

Part 1, dispatch: `dispatch.go` and `dispatch_helpers.go` were replaced with the prototype files. `ast.go` regained the 26 constructors deferred in step 2,
and `constants.go` gained `pkgContext`, `pkgErrors`, `pkgFmt`, and `pkgJSON`. The generator then wrote all five files byte-identical to the committed ones,
before and after `gofmt`.

Part 2, types and helpers together: `types.go` and `helpers.go` were replaced with the prototype files. `fieldSet` and `structField` moved back into
`types.go`, so `ast.go` is the prototype file exactly. All five generated files were again byte-identical, before and after `gofmt`.

The public writers keep their `Jen` names until step 5. With those names normalised, `ast.go`, `file.go`, `file_test.go`, `constants.go`, `dispatch.go`,
`dispatch_helpers.go`, `helpers.go`, `types.go`, and `doc.go` match the verified prototype in Appendix A of
[0008-REPORT-self-contained-sdk-review.md](../reports/0008-REPORT-self-contained-sdk-review.md). Only `jenwrap.go` still imports Jennifer, and nothing calls
its aliases.

Cross-commit comparison: the Jennifer generator was built from `c3883f0`, taken with `git archive` into a scratch directory, and the go/ast generator from
this tree. Both ran from the repository root on schema files extracted with `git show`:

```text
HEAD schema=1.25.0 files=5 unstable=True IDENTICAL
9fb9ae5 schema=0.13.5 files=5 unstable=True IDENTICAL
0845a3b schema=0.13.5 files=5 unstable=True IDENTICAL
d03a894 schema=0.13.4 files=5 unstable=True IDENTICAL
192e108 schema=0.13.0 files=5 unstable=True IDENTICAL
6595492 schema=0.12.2 files=5 unstable=True IDENTICAL
72c8f1d schema=0.12.0 files=5 unstable=True IDENTICAL
c180b9d schema=0.11.7 files=5 unstable=True IDENTICAL
584abe6 schema=0.10.8 files=5 unstable=True IDENTICAL
c225368 schema=0.10.7 files=5 unstable=False IDENTICAL
cc23bb6 schema=0.6.3 files=5 unstable=False IDENTICAL
2c1aef2 schema=0.4.9 files=5 unstable=False IDENTICAL
da204ff schema=0.4.5 files=5 unstable=False IDENTICAL
failures: 0
```

The comparison was seen to fail: a scratch copy with one emitted string changed (`"error"` to `"err"` in `invalidParams`) reported
`DIFF ['agent_gen.go', 'client_gen.go']` for `HEAD` and `da204ff`, and exited 1.

Gates: `make pre-add-check` with the six changed files named: `go-precheck: 6 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).`
Without names: `go-precheck: 47 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 4 on `main`. Steps 5 to 7 follow.

### Steps 5 to 7, 2026-10-09

Step 4 was committed by the owner as `b568d1c`. Steps 5 to 7 ran on `main` from that clean tree. The agent did not commit.

Step 5: `jenwrap.go` was deleted. `require github.com/dave/jennifer v1.7.1` was removed from `cmd/generate/go.mod`. `go mod tidy` left `go.sum` at 0 bytes,
and it was deleted; `go mod tidy -diff` is then clean. `WriteConstantsJen`, `WriteTypesJen`, `WriteDispatchJen`, and `WriteHelpersJen` lost the `Jen`
suffix. `main.go` was replaced with the prototype: `run() error`, exit status 1 on error, and `findRepoRoot` only when `-schema` or `-out` is empty.
`main_test.go` adds `TestGeneratedFilesUpToDate`. After this step every file under `cmd/generate` that the port touched is byte-identical to the verified
prototype in Appendix A of [0008-REPORT-self-contained-sdk-review.md](../reports/0008-REPORT-self-contained-sdk-review.md).

Step 6: `go test ./...` exits 0 at the root and in `cmd/generate`. `TestGeneratedFilesUpToDate`, `TestRenderLayout`, `TestStructTagQuoting`, and
`TestLayoutStaysBelowPrinterSentinel` pass.

Step 7: each breakage ran on its own scratch copy outside the tree. Results, as printed:

```text
[FAILS as expected] stale generated file -> TestGeneratedFilesUpToDate
    main_test.go:53: types_gen.go is out of date with schema/; run make version (first difference at line 2)
[FAILS as expected] emitter text change -> TestGeneratedFilesUpToDate
    main_test.go:53: types_gen.go is out of date with schema/; run make version (first difference at line 1656)
[FAILS as expected] empty struct braces split -> TestRenderLayout
    file_test.go:148: render mismatch
[FAILS as expected] function braces on one line -> TestRenderLayout
    file_test.go:148: render mismatch
[FAILS as expected] line length past printer sentinel -> TestLayoutStaysBelowPrinterSentinel
    file_test.go:177: layout spans 12588178880 bytes
```

Closing verification, as printed:

```text
PASS  no .go file imports Jennifer (0)
PASS  neither go.mod names jennifer (require/replace/exclude)
PASS  cmd/generate/go.sum absent
PASS  go list -m all in cmd/generate: github.com/maccavelli/acp-go-sdk2/cmd/generate
PASS  jenwrap.go deleted
PASS  no Write*Jen names remain
PASS  generated files unchanged since c3883f0 (pre-port)
PASS  gofmt -l on edited Go files empty
PASS  go test ./... at root
PASS  go test -race ./... in cmd/generate
PASS  go build ./example/...
PASS  generator run from / with -schema and -out exits 0
PASS  that run wrote all five files byte-identical
PASS  *.go *.mod *.sum free of old path and Jennifer (0)
PASS  schema/version is 1.25.0
PASS  both go lines are 1.27.2
PASS  no encoding/json/v2 or jsontext import
PASS  LICENSE unchanged since cf2e730
```

The checks were seen to fail: a scratch clone with the Jennifer requirement put back in `cmd/generate/go.mod`, an empty `go.sum`, and `WriteTypes`
renamed back to `WriteTypesJen` failed eight of them.

The 13-commit comparison was repeated with the final generator binary against the Jennifer binary from `c3883f0`: 13 of 13 identical, `failures: 0`.

Gates: `make pre-add-check` with the changed files named: `go-precheck: 6 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).`
Without names: `go-precheck: 47 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Every acceptance criterion in Verification holds. Commit point: the agent stopped here. The owner commits step 5 on `main`.

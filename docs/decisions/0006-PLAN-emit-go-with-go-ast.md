---
status: proposed
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

## Execution record

Not started.

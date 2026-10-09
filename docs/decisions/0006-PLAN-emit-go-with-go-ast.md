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
* `cmd/generate/main.go` call names once the `Jen` suffix no longer matches
* `cmd/generate/go.mod` and `cmd/generate/go.sum` when Jennifer is the last requirement to drop

Out of scope:

* `internal/load` and `internal/ir`, except a compile fix caused by an emit signature change
* The module rename in [0005-PLAN-separate-module-from-coder.md](0005-PLAN-separate-module-from-coder.md)
* Hand-editing `*_gen.go`
* A new branch, a commit, a tag, or a push
* Reverting uncommitted edits already in the working tree

## Implementation Steps

1. Stay on the current branch. Do not create a branch. Do not commit.
2. Add local constructors that return `go/ast` nodes, and a writer that prints an `*ast.File` with `go/format`. Do not import Jennifer from the new writer.
3. Port `WriteConstantsJen` to that writer. Regenerate `constants_gen.go`. Compare it with the file from before this step after `gofmt` on both. An empty diff is required. A nonempty diff stops this plan.
4. Port `WriteHelpersJen` the same way, then `WriteDispatchJen`, then `WriteTypesJen`. After each writer, regenerate only the files that writer owns and require an empty `gofmt` diff before the next writer.
5. When all five generated files match, delete `jenwrap.go` and every import of `github.com/dave/jennifer/jen`. Remove the requirement from `cmd/generate/go.mod`. Tidy that module so `go.sum` no longer names it, and delete `go.sum` if tidy leaves it empty. Do not add a `replace` or an `exclude`. Rename the `Write*Jen` functions so the names no longer say Jen. Update `main.go`.
6. Run `go test ./...` at the repository root and in `cmd/generate`.

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

## Rollout and Rollback

No branch, no commit, no tag, and no push in this plan. Rollback is restoring `cmd/generate` and the generated files from the pre-port tree. A nonempty generated diff is not fixed by editing the generated file or by weakening the comparison.

## Execution record

Not started.

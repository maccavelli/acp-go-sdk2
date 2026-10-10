---
status: proposed
date: 2026-10-09
associated-madr: "0005-MADR-separate-module-from-coder.md"
---

# Implement Separate this module from github.com/coder/acp-go-sdk

Associated MADR: [0005-MADR-separate-module-from-coder.md](0005-MADR-separate-module-from-coder.md)

## Goal

Make this tree import as `github.com/maccavelli/acp-go-sdk2`, with no Coder remote and no Coder requirement.

## Scope

In scope:

* `go.mod`: `module github.com/maccavelli/acp-go-sdk2`
* `cmd/generate/go.mod`: `module github.com/maccavelli/acp-go-sdk2/cmd/generate`
* Import paths in `example/` and `cmd/generate` that use the old module path
* `README.md`, `AGENTS.md`, and `docs/architecture.md` where they name the old module path
* The `mdsh` directive on `README.md` line 14 (amended 2026-10-09)
* The amendments already written on [0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md) and [0004-MADR-pin-acp-schema-1-25-0.md](0004-MADR-pin-acp-schema-1-25-0.md). Execution does not edit them again

Out of scope:

* Adding or removing git remotes. `origin` already points at this repository. Do not add `upstream`.
* A `require` or `replace` of `github.com/coder/acp-go-sdk`
* Removing `github.com/dave/jennifer`. [0006-PLAN-emit-go-with-go-ast.md](0006-PLAN-emit-go-with-go-ast.md) owns that removal. This plan must not add the requirement back if 0006 has already removed it.
* Rewriting historical sentences inside 0001, 0002, 0003, or 0004, other than the two amendments above
* A module tag, a push, ~~or a new branch~~ (amended 2026-10-09: the branch is named in [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md))
* `LICENSE`. Its Apache-2.0 copyright notice stays (amended 2026-10-09)
* `docs/README.md` and every file under `docs/decisions/` and `docs/reports/`. Their old-path titles are historical (amended 2026-10-09)
* Reverting uncommitted edits already in the working tree

## Implementation Steps

1. ~~Stay on the current branch. Do not create a branch. Do not commit.~~ Amended 2026-10-09: work on the branch that [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md) names, and commit this plan's
   change as its own commit after the pre-add checks in `AGENTS.md`. `HEAD` was `main`, and no commit may land on `main`, so the original step left work that could
   never be committed.
2. Replace `github.com/coder/acp-go-sdk` with `github.com/maccavelli/acp-go-sdk2` in both `go.mod` files, in Go imports, in `AGENTS.md`, and in `docs/architecture.md`.
   Amended 2026-10-09: name those files. Do not run a replace over all `*.md`; it rewrites the record titles in `docs/README.md`. The Go files are
   `cmd/generate/main.go`, five files in `cmd/generate/internal/emit`, `cmd/generate/internal/ir/ir.go`, and the four `example/*/main.go` programs.
3. In `README.md`, point the install line, the pkg.go.dev links, and the example links at `github.com/maccavelli/acp-go-sdk2`. Drop `@v0.13.5` from the install line. Do not substitute `@v1.25.0`. No module tag exists, and `1.25.0` is the schema stamp.
   Amended 2026-10-09: also change the `mdsh` directive on line 14 to `printf 'go get github.com/maccavelli/acp-go-sdk2\n'`, with no `@v%s` and no
   `$(cat schema/version)`. Otherwise the next `make version` writes `@v1.25.0` back into line 17. See [0008-REPORT-self-contained-sdk-review.md](../reports/0008-REPORT-self-contained-sdk-review.md).
4. Leave the root `go.mod` without a `require` block. Do not add `github.com/dave/jennifer`. If it is still required, leave it for 0006. If 0006 has already removed it, do not restore it. Run `go mod tidy` in `cmd/generate` only if the module line change makes the sum stale.
5. Do not edit 0002 or 0004 again. Their module-path amendments are already in those records.

## Verification

* `git remote` prints `origin` only.
* The root `go.mod` module line is `github.com/maccavelli/acp-go-sdk2` and the file has no `require` and no `replace`.
* `cmd/generate/go.mod` module line is `github.com/maccavelli/acp-go-sdk2/cmd/generate`. This plan does not add `github.com/dave/jennifer`.
* A search of `*.go`, `*.mod`, `README.md`, `AGENTS.md`, and `docs/architecture.md` finds no `github.com/coder/acp-go-sdk`.
* `go test ./...` exits 0 at the repository root and in `cmd/generate`.
* `gofmt -l` on the edited Go files prints nothing.
* Amended 2026-10-09: `README.md` line 14 contains neither `@v%s` nor `schema/version`. Running the directive's `printf` by hand prints the line 17 text exactly.
* Amended 2026-10-09: `git diff` of this phase touches no file under `docs/` other than `docs/architecture.md`, and does not touch `LICENSE`.
* Amended 2026-10-09: `go build ./example/...` exits 0. `go mod tidy -diff` is clean in both modules.

## Rollout and Rollback

~~No branch, no commit, no tag, and no push in this plan. The edits stay in the working tree on the current branch. Rollback of the path change is restoring those files' module path. Uncommitted work that was already in the tree before this plan stays.~~

Amended 2026-10-09: no tag and no push. The change is one commit on the branch named in [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md). That plan commits the records before any phase runs, so
rollback is `git revert` of this commit, and it cannot reach the uncommitted record edits the original rollback could not tell apart from this plan's edits.

## Execution record

Not started.

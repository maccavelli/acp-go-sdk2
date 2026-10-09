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
* The amendments already written on [0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md) and [0004-MADR-pin-acp-schema-1-25-0.md](0004-MADR-pin-acp-schema-1-25-0.md). Execution does not edit them again

Out of scope:

* Adding or removing git remotes. `origin` already points at this repository. Do not add `upstream`.
* A `require` or `replace` of `github.com/coder/acp-go-sdk`
* Removing `github.com/dave/jennifer`
* Rewriting historical sentences inside 0001, 0002, 0003, or 0004, other than the two amendments above
* A module tag, a push, or a new branch
* Reverting uncommitted edits already in the working tree

## Implementation Steps

1. Stay on the current branch. Do not create a branch. Do not commit.
2. Replace `github.com/coder/acp-go-sdk` with `github.com/maccavelli/acp-go-sdk2` in both `go.mod` files, in Go imports, in `AGENTS.md`, and in `docs/architecture.md`.
3. In `README.md`, point the install line, the pkg.go.dev links, and the example links at `github.com/maccavelli/acp-go-sdk2`. Drop `@v0.13.5` from the install line. Do not substitute `@v1.25.0`. No module tag exists, and `1.25.0` is the schema stamp.
4. Leave the root `go.mod` without a `require` block. Leave Jennifer as the generator's only requirement. Run `go mod tidy` in `cmd/generate` only if the module line change makes the sum stale.
5. Do not edit 0002 or 0004 again. Their module-path amendments are already in those records.

## Verification

* `git remote` prints `origin` only.
* The root `go.mod` module line is `github.com/maccavelli/acp-go-sdk2` and the file has no `require` and no `replace`.
* `cmd/generate/go.mod` module line is `github.com/maccavelli/acp-go-sdk2/cmd/generate` and its only requirement is `github.com/dave/jennifer v1.7.1`.
* A search of `*.go`, `*.mod`, `README.md`, `AGENTS.md`, and `docs/architecture.md` finds no `github.com/coder/acp-go-sdk`.
* `go test ./...` exits 0 at the repository root and in `cmd/generate`.
* `gofmt -l` on the edited Go files prints nothing.

## Rollout and Rollback

No branch, no commit, no tag, and no push in this plan. The edits stay in the working tree on the current branch. Rollback of the path change is restoring those files' module path. Uncommitted work that was already in the tree before this plan stays.

## Execution record

Not started.

---
status: complete
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
   Superseded 2026-10-09: commits land on `main`; see [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md#amendment-2026-10-09-commits-land-on-main).
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

Superseded 2026-10-09: commits land on `main`; see [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md#amendment-2026-10-09-commits-land-on-main).

## Deviation 2026-10-09, unused `ptrTo` in `acp_test.go`

Found during execution: after the rename, `make pre-add-check` failed only on `golangci-lint`, host and `GOOS=windows`:
`acp_test.go:983:6: func ptrTo is unused (unused)`. A clean clone of `50e1b4a`, before the rename, fails the same way. `aeee903` replaced the last two
`ptrTo(x)` calls with `new(x)` and left the `//go:fix inline` helper with no callers. Every other pre-add check passed.

Decision: the owner chose to delete the helper in this phase. `acp_test.go` is added to this plan's scope for that deletion only. No other line of the file
changes. Behavior does not change, because nothing calls the helper.

## Execution record

Executed 2026-10-09 on `main`, from a clean tree at `50e1b4a`, as step 2 of [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md).
The agent did not commit. The owner commits this phase on `main`.

What was done:

* Step 2: the old path was replaced in exactly these files, with the counts asserted before writing: `go.mod` 1, `cmd/generate/go.mod` 1,
  `cmd/generate/main.go` 2, `cmd/generate/internal/emit/constants.go` 2, `dispatch.go` 2, `dispatch_helpers.go` 1, `helpers.go` 2, `types.go` 3,
  `cmd/generate/internal/ir/ir.go` 2, each `example/*/main.go` 1, `AGENTS.md` 1, `docs/architecture.md` 1.
* Step 3: `README.md` line 14 now reads ``<!-- `$ printf 'go get github.com/maccavelli/acp-go-sdk2\n'` as bash -->``. Line 17 is
  `go get github.com/maccavelli/acp-go-sdk2`. The two example links and the two pkg.go.dev links use the new path.
* The deviation above: the unused `ptrTo` helper was deleted from `acp_test.go` (5 lines with its blank line).

Not done, by design: no `go mod tidy` (no sum went stale); `github.com/dave/jennifer` left for 0006; 0002, 0004, `LICENSE`, `docs/README.md` record titles,
and every other record untouched by the rename; no remote change; no tag; no push.

Verification, as printed:

```text
PASS  git remote prints origin only (origin)
PASS  root module line
PASS  root go.mod has no require/replace
PASS  generator module line
PASS  jennifer requirement unchanged (0006 owns its removal)
PASS  old path absent from *.go *.mod README AGENTS architecture (0 hits)
PASS  README line 14 has no @v%s and no schema/version
PASS  directive printf output equals README line 17 (go get github.com/maccavelli/acp-go-sdk2)
FAIL  diff touches no docs/ file but architecture.md, and not LICENSE (1 others)
PASS  gofmt -l on edited Go files is empty
PASS  go test ./... at root
PASS  go test ./... in cmd/generate
PASS  go build ./example/...
PASS  go mod tidy -diff at root
PASS  go mod tidy -diff in cmd/generate
```

The one `FAIL` is this record: `git diff --name-only -- docs LICENSE` lists `docs/architecture.md` and
`docs/decisions/0005-PLAN-separate-module-from-coder.md`, which carries the deviation and this execution record. The rename itself touched no other file
under `docs/` and did not touch `LICENSE`. Before the deviation entry was written, the same check passed with 0 others.

`make pre-add-check` before the deviation exited 1 on the `ptrTo` finding only. After it:
`go-precheck: 44 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` `make check-records` and `make markdownlint` exit 0.

The checks were seen to fail. On a scratch copy with the old path put back in `README.md` line 17 and `example/agent/main.go`, and a line appended to
`LICENSE`, six checks failed: the old-path search (2 hits), the line 17 comparison, the docs/LICENSE check, `go test`, `go build ./example/...`, and
`go mod tidy -diff` at the root.

The generator, run after the rename into a scratch directory, wrote all five `*_gen.go` files byte-identical to the committed ones.

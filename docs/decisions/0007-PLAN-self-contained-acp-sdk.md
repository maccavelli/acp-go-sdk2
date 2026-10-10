---
status: complete
date: 2026-10-09
associated-madr: "0007-MADR-self-contained-acp-sdk.md"
---

# Implement Keep this SDK self-contained on the current ACP schema

Associated MADR: [0007-MADR-self-contained-acp-sdk.md](0007-MADR-self-contained-acp-sdk.md)

## Goal

Remove the previous module path and Jennifer, and leave the Go 1.27.2 floor and the schema stamp `1.25.0` in place.

## Scope

In scope:

* ~~Amended 2026-10-09: the branch `refactor/self-contained-sdk`, and a first commit of the records as they stand before any phase runs~~
  Superseded 2026-10-09: no branch; the records were committed on `main` in `cf2e730`
* Every step in [0005-PLAN-separate-module-from-coder.md](0005-PLAN-separate-module-from-coder.md)
* Every step in [0006-PLAN-emit-go-with-go-ast.md](0006-PLAN-emit-go-with-go-ast.md), after 0005

Out of scope:

* Protocol v2 and any schema stamp other than `1.25.0`
* Moving call sites from `encoding/json` to `encoding/json/v2`. Amended 2026-10-09: rejected by the owner, not deferred; see [0007-MADR-self-contained-acp-sdk.md](0007-MADR-self-contained-acp-sdk.md)
* Amended 2026-10-09: any change to the generated output. [0009-MADR-fix-generated-output-on-json-v1.md](0009-MADR-fix-generated-output-on-json-v1.md) proposes those changes, after this plan
* Amended 2026-10-09: `LICENSE`
* Replacing the generator's `os.ReadFile` calls with `os.OpenRoot`
* ~~A new branch, a commit,~~ a tag, or a push. Amended 2026-10-09: a push needs the owner's explicit ask in the same turn
* Reverting uncommitted edits already in the working tree

## Implementation Steps

1. ~~Stay on the current branch. Do not create a branch. Do not commit.~~ Amended 2026-10-09: `HEAD` was `main`, and no commit may land on `main`.
   Create `refactor/self-contained-sdk` from `main` without discarding the working tree. Confirm `git rev-parse --path-format=absolute --git-path hooks`
   resolves to the global hooks. Run `make check-records` and `make markdownlint`, then commit the records with `git commit --no-edit`:
   `docs/README.md`, the 0005, 0006, and 0007 MADRs and PLANs, the 0009 MADR and PLAN, and [0008-REPORT-self-contained-sdk-review.md](../reports/0008-REPORT-self-contained-sdk-review.md).
   Every later step starts from that commit.
   Superseded 2026-10-09: done. The owner committed the records on `main` in `cf2e730`. No branch is created.
2. Execute [0005-PLAN-separate-module-from-coder.md](0005-PLAN-separate-module-from-coder.md). Stop if its verification fails.
   Amended 2026-10-09: as amended that day, including the `mdsh` directive. Commit it as one commit after `make pre-add-check`.
   Superseded 2026-10-09: the agent stops after the checks pass and the owner commits on `main`.
3. Execute [0006-PLAN-emit-go-with-go-ast.md](0006-PLAN-emit-go-with-go-ast.md) on the renamed tree. Stop if a regenerated file differs after `gofmt`.
   Amended 2026-10-09: as amended that day, including the layout design, the reordered port, the cross-commit comparison, the new tests, and the
   deliberate-breakage step. Commit at the points that plan names.
   Superseded 2026-10-09: at each of those points the agent stops and the owner commits on `main`.
4. Confirm `schema/version` is still `1.25.0` and both `go` lines are still `1.27.2`.
5. Amended 2026-10-09: record each phase's commit, verification output, and any deviation in the execution records of this plan, 0005, and 0006.
   Update the three PLAN statuses and the `docs/README.md` rows as the phases land.

## Verification

* The verification lists in 0005 and 0006 both hold.
* A search of `*.go`, `*.mod`, and `*.sum` finds neither `github.com/coder/acp-go-sdk` nor `github.com/dave/jennifer`.
* `schema/version` reads `1.25.0`. Both `go` lines read `1.27.2`.
* `go test ./...` exits 0 at the repository root and in `cmd/generate`.
* Amended 2026-10-09: `git grep -nE '"encoding/json/(v2|jsontext)"'` finds nothing in either module.
* ~~Amended 2026-10-09: `git log --reverse main..refactor/self-contained-sdk` lists the records commit, then the 0005 commit, then the 0006 commits.~~
  Superseded 2026-10-09: `git log --reverse cf2e730..main` lists the 0005 commit, then the 0006 commits.
* Amended 2026-10-09: `LICENSE` is unchanged against `main`.
* Amended 2026-10-09: `make check-records` and `make markdownlint` pass at the end.

## Rollout and Rollback

~~No branch, no commit, no tag, and no push in this plan. Rollback is restoring the pre-plan tree for the files each phase changed.~~
A generated diff is not repaired by hand-editing `*_gen.go`.

~~Amended 2026-10-09: all work is on `refactor/self-contained-sdk`. No tag. A push, and any merge to `main`, wait for the owner's explicit ask.
Rollback of a phase is `git revert` of that phase's commits. Because the records are committed first, a revert never touches them.
Abandoning the whole change is deleting the unmerged branch, which needs the owner's explicit ask.~~

Superseded 2026-10-09 by the amendment below: commits land on `main`.


## Amendment 2026-10-09, commits land on main

The owner said on 2026-10-09 that this repository is committed on `main` most of the time. The owner committed the records on `main` in
`cf2e730`, before any phase ran. Every branch named in this plan's earlier amendments, and in the plans it runs, is withdrawn.

* Phase work happens on `main`, in the working tree.
* At each commit point the agent runs the pre-add checks in `AGENTS.md`, writes their output into the execution record, and stops.
  The owner commits with `git commit --no-edit`. The global `prepare-commit-msg` hook writes the message.
* An agent does not commit, push, or merge on `main`. The organization's git rule forbids it, and an authorization given in a turn does not lift that rule.
* When the owner wants an agent to commit a phase, the owner names a branch for it, `feature/<slug>` by default, and asks for the commit in that turn.
* A push needs the owner's explicit ask in the same turn. No tag.
* Before a phase is committed, abandoning it means restoring the files that phase touched, which needs the owner's explicit ask.
  After it is committed, rollback is `git revert` of that commit on `main`, made by the owner.

## Execution record

* Step 1, 2026-10-09: done by the owner. The records were committed on `main` in `cf2e730`. The commit policy amendment followed in `50e1b4a`.
* Step 2, 2026-10-09: [0005-PLAN-separate-module-from-coder.md](0005-PLAN-separate-module-from-coder.md) executed on `main` from `50e1b4a`, with one
  deviation recorded there (the unused `ptrTo` test helper deleted). All its checks passed and `make pre-add-check` is clean. The agent stopped at the
  commit point. The owner commits it.
* Step 2 was committed by the owner as `c3883f0` and pushed.
* Step 3, 2026-10-09: [0006-PLAN-emit-go-with-go-ast.md](0006-PLAN-emit-go-with-go-ast.md) in progress. Its steps 2 and 3 (the go/ast writer and the
  constants port) are done and checked; the agent stopped at that plan's first commit point. Its steps 4 to 7 follow.
  The owner committed them as `53ae152`. Step 4 (dispatch, then types and helpers, and the 13-commit comparison) is done and checked; the agent
  stopped at the second commit point.
  The owner committed step 4 as `b568d1c`. Steps 5 to 7 (Jennifer removed, writers renamed, `main.go` and the standing test, the breakage
  experiments) are done and checked; the agent stopped at the last commit point. That plan is complete.
* Step 4, 2026-10-09: `schema/version` reads `1.25.0`, and both `go` lines read `go 1.27.2`.
* Step 5, 2026-10-09: the execution records of 0005 and 0006 hold each phase's output and the one deviation. The statuses of 0005, 0006, and this plan
  are `complete`, and `docs/README.md` shows them.
* Verification: the 0005 and 0006 lists hold, as recorded in those plans. `*.go`, `*.mod`, and `*.sum` contain neither `github.com/coder/acp-go-sdk`
  nor `github.com/dave/jennifer`. No file imports `encoding/json/v2` or `encoding/json/jsontext`. `LICENSE` is unchanged since `cf2e730`.
  `git log --reverse cf2e730..main` lists `50e1b4a` (the commit policy), `c3883f0` (0005), then `53ae152` and `b568d1c` (0006); the owner's step 5
  commit follows. `make check-records` and `make markdownlint` exit 0.

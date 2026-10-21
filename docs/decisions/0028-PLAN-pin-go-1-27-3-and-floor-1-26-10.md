---
status: in-progress
date: 2026-10-21
associated-madr: "0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md"
---

# Implement 0028: pin Go 1.27.3 as the toolchain and Go 1.26.10 as the floor

Associated MADR: [0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md](0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md)

## Goal

Implement the MADR's O1, as the owner decided on 2026-10-21: every place that pins go1.27.2 names go1.27.3 (T1 to T4); every place
that pins the floor, 1.26.9, names 1.26.10 (F1 to F5, question 2); and CI fails when its newest job and a `toolchain` line disagree
(question 1).

## Facts this plan rests on

The MADR's T1 to T4 and F1 to F5, and its reading of the machines, the release notes and the policy, all on 2026-10-21. Step 1 re-asserts T1 to
T4, F1 to F5 and the `go list -deps` result with a script.

## Scope

* **Changed:** `go.mod`, `cmd/generate/go.mod` (the `go` and `toolchain` lines); `.github/workflows/ci.yml` (the matrix, the floor's
  `include` entry, the new step); `README.md:20`–`:21`; `cmd/generate/internal/mutate/mutate_test.go:92`, `:287`.
* **Unchanged:** the records that name go1.27.2 or 1.26.9 as what they decided or ran on, and the index's titles of them; `go.sum`;
  the Makefile and `scripts/go-precheck.sh`, which match any `go1.26*`.
* **The records:** this PLAN and its row; the MADR's row.

## Implementation Steps

Each step ends with its gates and an execution record. Agents stage; a commit or a push on `main` needs the owner's explicit permission
in that turn (`AGENTS.md`, "Commits"). Every new check is seen to fail on a planted break in a scratch copy, never in the tree, before it
is relied on.

1. **Records:** this PLAN and its row `in-progress`; the facts re-asserted by a script.
2. **The pins:** T1 to T4 to go1.27.3; F1 to F5 to 1.26.10; the CI step "the newest is the toolchain line":

   ```yaml
   - name: the newest is the toolchain line
     if: '!matrix.floor && runner.os == ''Linux'''
     run: |
       set -euo pipefail
       test "$(go version | cut -d' ' -f3)" = "go${{ matrix.go }}"
       test "$(sed -n 's/^toolchain //p' go.mod)" = "go${{ matrix.go }}"
       test "$(sed -n 's/^toolchain //p' cmd/generate/go.mod)" = "go${{ matrix.go }}"
   ```

3. **Verification and close:** the gates; the push; CI's six jobs; this PLAN and its row `complete`.

## Verification

Per step:

* the gates on the working tree: regeneration, `make pre-add-check`, `GOEXPERIMENT=nojsonv2`, `GOOS=windows go vet`,
  `make check-records`, `make markdownlint`, the identifier scan, `actionlint` on `ci.yml`;
* the floor gate, now at go1.26.10: `go vet` and `go test -race` in both modules under `GOTOOLCHAIN=go1.26.10`, the leak guards'
  experiment set;
* the Windows and Linux hosts reached over SSH, and `make push-guard`, before the push.

Planted breaks, each in a scratch copy:

| For | Break | The check that must fail |
| :--- | :--- | :--- |
| the pins | one `toolchain` line left at go1.27.2 | `git grep -n "1\.27\.2"` outside the records finds it |
| the floor | one `go` line left at 1.26.9 | `git grep -n "1\.26\.9"` outside the records finds it; CI's "the floor is the go line", run locally |
| the `toolchain` line | the root's line removed | `GOTOOLCHAIN=go1.27.2+auto go version`, run in the root, prints go1.27.2 |
| the CI step | the matrix at `1.27.3` and `cmd/generate`'s line at go1.27.2 | the step's commands, run locally with the matrix value substituted, exit 1 |

Acceptance:

* no file outside `docs/decisions/` and `docs/reports/`, and the index's titles of earlier records, names `1.27.2` or `1.26.9`;
* the floor gate passes on go1.26.10, and CI's floor jobs install and pass on it;
* `GOTOOLCHAIN=go1.27.2+auto go version` selects go1.27.3 in both modules;
* CI's six jobs pass on the pushed commit, the newest at go1.27.3;
* `make api` is unchanged.

## Rollout and Rollback

One commit, reverted by reverting it. No release follows from this alone. The floor reaches consumers: one whose `go` line is under
1.26.10 has it raised when it requires the next release, which is a minor; the next release's notes name it.

The work lands between the 0027 PLAN's steps; the 0027 PLAN's own step 5 is committed first.

## Execution record

### Step 1, records, 2026-10-21

The owner committed the MADR, this PLAN, 0024's pointer and the 0027 PLAN's correction as `cfc3d12`, and approved execution. What
changed: this PLAN and its `docs/README.md` row `in-progress`, and a correction in the MADR.

**The facts,** re-asserted by a script outside the tree, reading `cfc3d12`'s tree: 12 of 12 hold.

| Id | What the script read |
| :--- | :--- |
| T1, T2 | `go.mod:5` and `cmd/generate/go.mod:5` are `toolchain go1.27.2` |
| T3, F3 | `ci.yml:29` is `go: ['1.26.9', '1.27.2']`; `:31` is `- go: '1.26.9'` |
| T4, F4 | `README.md:20` is "It requires Go 1.26.9 or newer, the `go` line of `go.mod`; CI tests Go 1.26.9", `:21` "and 1.27.2." |
| F1, F2 | `go.mod:3` and `cmd/generate/go.mod:3` are `go 1.26.9` |
| F5 | `mutate_test.go:92` and `:287` write `go 1.26.9` into their fixtures' `go.mod` |
| M1 | `Makefile:37` filters `go1.26%`; `scripts/go-precheck.sh:32` matches `go1.26*)` |
| G1 | `git grep` for `1.27.2` and `1.26.9`, the records and the index left out, finds exactly those ten lines |
| X1 | `go list -deps` finds no `crypto/x509` or `net/http` for the root package, `agentproc`, `example/client` or `example/agent`, nor `crypto/x509` in `cmd/generate` |

**Planted breaks,** four at once in a scratch copy, `.git` included: the root's `toolchain` line at go1.27.1; the Makefile filtering
`go1.25%`; `1.27.2` added to `AGENTS.md:9`; a file in `agentproc` importing `net/http`. The script reported exactly T1, M1, G1 and X1
missing, "8 of 12 facts hold", exit 1; G1 named the extra `AGENTS.md:9` and the missing `go.mod:5`.

**Found, and corrected:**

* the MADR cited `Makefile:36` and `go-precheck.sh:31`; the lines are `:37` and `:32`. The MADR carries the correction;
* the script's first G1 counted the lines, and the planted breaks, one removing a `1.27.2` and one adding one, kept the count at ten: it
  passed when it should have failed. G1 now compares the set of file and line pairs, and failed as above.

**Gates:** `make check-records` and `make markdownlint` exit 0; the identifier scan finds nothing in the changed records.

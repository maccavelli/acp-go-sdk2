---
status: complete
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

### Step 2, the pins, 2026-10-21

The owner committed step 1 as `ed150be` and approved this step. What changed, by one asserted script:

* `go.mod` and `cmd/generate/go.mod`: `go 1.26.10`, `toolchain go1.27.3`;
* `.github/workflows/ci.yml`: the matrix `['1.26.10', '1.27.3']`, the floor's `include` entry `'1.26.10'`, and the step "the newest is the
  toolchain line" after "the floor is the go line", on the newest Linux job: `go version` must print `go` plus the matrix version, and
  each module's `toolchain` line, read with `sed`, must equal it;
* `README.md:20`–`:21`: "It requires Go 1.26.10 or newer, the `go` line of `go.mod`; CI tests Go 1.26.10 and 1.27.3.";
* `cmd/generate/internal/mutate/mutate_test.go:92`, `:287`: the two fixtures write `go 1.26.10`.

`git diff --stat`: 5 files, 20 lines added, 10 removed. `go.sum` is unchanged: `go mod tidy -diff` prints nothing in either module.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| nothing outside the records names 1.27.2 or 1.26.9 | `git grep` for both, the records and the index left out: no line |
| the `toolchain` lines select go1.27.3 | `GOTOOLCHAIN=go1.27.2+auto go version`, in each module: "go version go1.27.3 darwin/arm64" |
| CI's two version steps pass | their commands, read from `ci.yml` and run here with the matrix value: "the floor is the go line" at 1.26.10 and "the newest is the toolchain line" at 1.27.3, exit 0 each |
| `ci.yml` is a valid workflow | `actionlint`, exit 0 |
| the tree builds and its tests pass on the new floor | the floor gate at go1.26.10, below |

**Planted breaks.** Each ran in its own scratch copy of the working tree, `.git` included. 5 of 5 checks failed as required:

| Break | The check that failed |
| :--- | :--- |
| `cmd/generate`'s `toolchain` line left at go1.27.2 | `git grep`: "cmd/generate/go.mod:5:toolchain go1.27.2"; "the newest is the toolchain line, at 1.27.3: exit 1" |
| the root's `toolchain` line removed | `GOTOOLCHAIN=go1.27.2+auto go version` in the root: "go version go1.27.2" |
| `cmd/generate`'s `go` line left at 1.26.9 | `git grep`: "cmd/generate/go.mod:3:go 1.26.9"; "the floor is the go line, at 1.26.10: exit 1" |

**Gates,** each exit 0, run alone:

* go1.27.3: regeneration unchanged; `make pre-add-check`, "166 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)";
  `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet ./...` in both modules; `make check-records`, `make markdownlint` and the
  identifier scan; `make lint`, "0 issues." four times; `go fix -diff ./...` prints nothing;
* `make api` against `v1.18.0`: "0 incompatible, 4 compatible", unchanged;
* the Windows host: `go test -count=1 -shuffle=on ./...` in both modules;
* the Linux host: the `agentproc` test binary, built with go1.27.3, 2 runs, "PASS".

**The floor gate, now at go1.26.10** (`GOTOOLCHAIN=go1.26.10`, which downloaded it into the gate's scratch module cache): `go version`
prints "go version go1.26.10 darwin/arm64"; `go vet ./...` and `go test -race -count=1 ./...` pass in both modules, the root's tests and
`agentproc`'s with `GOEXPERIMENT=goroutineleakprofile`, the leak guards on.

### Step 3, verification and close, 2026-10-21

The owner committed step 2 as `0665658` and pushed `main`, which carried ten commits, `d034801..0665658`: the 0027 PLAN's steps 1 to 5
and this PLAN's records and steps.

**The push guard ran after the push, not before** as Verification asks. The global pre-push guard ran on the owner's push. The same
guard, run here afterwards over the pushed range, with `refs/heads/main 0665658 refs/heads/main d034801` as its pre-push input, the line
`scripts/push_guard.py` gives it: "guard over 10 commit(s), d034801..0665658", "the guard passed (exit 0)". Step 2's gates had already run
on both hosts reached over SSH.

**CI,** run `39622421451` on `0665658`: `success` on its first attempt, in all six jobs, `ubuntu-24.04`, `macos-15` and `windows-2025` at
`1.26.10` and at `1.27.3`. On the Linux jobs:

* at 1.26.10, `setup-go` printed "go version go1.26.10 linux/amd64"; "the floor is the go line" `success`; "the newest is the toolchain
  line" `skipped`;
* at 1.27.3, `setup-go` printed "go version go1.27.3 linux/amd64"; "the newest is the toolchain line" `success`; "the floor is the go
  line" `skipped`.

**Acceptance,** each met:

* no file outside `docs/decisions/` and `docs/reports/`, and the index's titles of earlier records, names `1.27.2` or `1.26.9` (step 2);
* `GOTOOLCHAIN=go1.27.2+auto go version` selects go1.27.3 in both modules (step 2);
* the floor gate passes on go1.26.10 (step 2), and CI's floor jobs installed and passed on it;
* CI's six jobs pass on the pushed commit, the newest at go1.27.3;
* `make api` is unchanged: "0 incompatible, 4 compatible" against `v1.18.0`.

**No release follows from this alone.** The floor reaches consumers, so the next release's notes name it: the 0027 PLAN's release
notes, `v1.19.0`.

This PLAN and its `docs/README.md` row are `complete`.

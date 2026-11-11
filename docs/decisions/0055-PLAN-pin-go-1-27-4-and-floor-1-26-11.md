---
status: proposed
date: 2026-11-10
associated-madr: "0055-MADR-pin-go-1-27-4-and-floor-1-26-11.md"
---

# Implement 0055: pin Go 1.27.4 as the toolchain and Go 1.26.11 as the floor

Associated MADR: [0055-MADR-pin-go-1-27-4-and-floor-1-26-11.md](0055-MADR-pin-go-1-27-4-and-floor-1-26-11.md)

## Goal

Move every pin of the Go toolchain to go1.27.4 and of the floor to go1.26.11, so that `govulncheck` passes again at
both versions, here and in CI. At the end of it:

* both `go.mod` files read `go 1.26.11` and `toolchain go1.27.4`;
* CI's matrix installs `1.26.11` and `1.27.4`;
* the README and the two mutation fixtures name them;
* the machines this PLAN runs on run go1.27.4, by the MADR's question 1;
* `make pre-add-check` exits 0, and
  [0054-PLAN-adopt-acp-schema-1-31-0-and-client-system-prompts.md](0054-PLAN-adopt-acp-schema-1-31-0-and-client-system-prompts.md)'s
  phase 4 can run its check again.

## Scope

**Preconditions.**

* The MADR is `accepted`, with its two questions answered. The owner approves this PLAN.
* The MADR, this PLAN and their two `docs/README.md` rows are committed alone, as the bootstrap exception allows.
* 0054's phase 4 is written and unstaged in the same tree: `README.md`, `docs/architecture.md`, `docs/README.md` and
  0054's PLAN. This PLAN does not stage or change those edits. Two files are shared, so each of this PLAN's commits
  stages its own lines of them alone, by writing the index entry from HEAD's version plus this PLAN's change, and
  checks the staged diff before the commit:
  * `docs/README.md`: this pair's two rows;
  * `README.md`: lines 20 and 21.

**Files changed.**

| Step | Files |
| :--- | :--- |
| 2, the pins | `go.mod`; `cmd/generate/go.mod`; `.github/workflows/ci.yml`; `README.md` (lines 20 and 21); `cmd/generate/internal/mutate/mutate_test.go` |
| 2, by question 2's answer | `tools/test_tools.py`, only if the owner chose to move its version strings |
| 5, the records | this PLAN; the MADR; `docs/README.md` |

**Outside the tree.** By question 1's answer, Go's environment file on this machine and on the two test hosts
(`go env -w GOTOOLCHAIN=go1.27.4`), and the go1.27.4 and go1.26.11 toolchains the go command downloads into each
machine's module cache.

**Not changed:** `go.sum` (the root module's requirements do not change); any `.go` file but the two fixtures; the two
CI steps that check the pins; `scripts/go-precheck.sh` and the Makefile, whose leak-guard switch reads any `go1.26*`
(0028).

**Out of scope:** a test for the race of GO-2026-6859 in `remote`; a way to learn of a Go security release; the
consumers' own `go` lines.

## Implementation Steps

### Step 1: the machines, by question 1's answer

As recommended:

1. On this machine, print `go env GOTOOLCHAIN` and `go version`, then run `go env -w GOTOOLCHAIN=go1.27.4`.
   `go version` must then print `go1.27.4`: the go command downloads the toolchain on that first use.
2. The same on the Linux and the Windows test host, over SSH, each host named by its variable (`ACP_LINUX_HOST`,
   `ACP_WINDOWS_HOST`).
3. Record the value replaced on each, so the setting can be put back.

If the owner moves the machines instead, this step only prints `go version` on each and stops when one is not go1.27.4.
If no machine setting changes, every command below runs with `GOTOOLCHAIN=go1.27.4` in its environment, the hosts'
runs are recorded as go1.27.3, and step 4's host gates say so.

### Step 2: the pins

1. `go.mod` and `cmd/generate/go.mod`: `go 1.26.11` and `toolchain go1.27.4`, with `go mod edit -go=1.26.11
   -toolchain=go1.27.4` in each module.
2. `.github/workflows/ci.yml`: `go: ['1.26.11', '1.27.4']` (`:57`), and `go: '1.26.11'` in the two `include` entries
   (`:59`, `:66`).
3. `README.md:20`-`:21`: "It requires Go 1.26.11 or newer, the `go` line of `go.mod`; CI tests Go 1.26.11 and 1.27.4."
4. `cmd/generate/internal/mutate/mutate_test.go:92` and `:287`: `go 1.26.11`.
5. `tools/test_tools.py`: by question 2's answer; as recommended, nothing.

### Step 3: the checks here

Each command's output goes to a scratch file, and its exit status is captured before any filter.

1. `git grep -n -E '1\.27\.3|1\.26\.10' -- . ':!docs/decisions' ':!docs/reports'` finds only `docs/README.md`'s two
   rows for 0028 and, by question 2's answer, `tools/test_tools.py`.
2. `go version` prints `go1.27.4`. `go mod tidy -diff` prints nothing in both modules.
3. `govulncheck`, at the version CI pins (`ci.yml:143`), in both modules, at both versions:
   * `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` at go1.27.4, exit 0;
   * the same with `GOTOOLCHAIN=go1.26.11`, exit 0. This is CI's floor job's step.
4. CI's two pin checks, their commands copied from `ci.yml:87`-`:103` into a scratch script that takes the tree and the
   matrix's two versions:
   * on the tree, both pass;
   * on a copy whose `go.mod` says `toolchain go1.27.3`, "the newest is the toolchain line" fails;
   * on a copy whose `cmd/generate/go.mod` says `go 1.26.10`, "the floor is the go line" fails.

   These two steps are 0024's and 0028's, not new. The run shows this change keeps them meaningful.
5. `actionlint .github/workflows/ci.yml` exits 0.
6. `make pre-add-check FILES="go.mod cmd/generate/go.mod .github/workflows/ci.yml README.md
   cmd/generate/internal/mutate/mutate_test.go"` exits 0, its `govulncheck` step included.
7. `make check-tools` exits 0: a workflow changed.
8. `python3 tools/gates.py floor api tidy nojsonv2 winvet platforms`. `floor` runs both modules' vet and race tests at
   go1.26.11.

### Step 4: the hosts

`python3 tools/gates.py linux windows`: the tree's tests on each test host. Each log's `go version` line must name
go1.27.4, by question 1's answer. A host that cannot be reached is recorded as not run, with the reason; it is not a
pass.

### Step 5: the records, and the commit

1. This PLAN's execution record: each command with its exit status, and the value `GOTOOLCHAIN` had on each machine.
2. `make check-records`, `make markdownlint`, `make disclosure-check`.
3. Stage this PLAN's lines alone, as Scope says, and show the staged diff. On `main` the agent commits only with the
   owner's explicit permission in that turn (`AGENTS.md`, "Commits"); otherwise the owner commits.
4. Then 0054's phase 4 runs `make pre-add-check` again, under its own PLAN.
5. After the owner's push, `python3 tools/watch_ci.py <rev>`: CI's six jobs pass at `1.26.11` and `1.27.4`. This PLAN is
   `complete` when they have.

## Verification

* `go.mod` and `cmd/generate/go.mod` read `go 1.26.11` and `toolchain go1.27.4`.
* Step 3's grep finds no pin of the old versions outside the records.
* `govulncheck` exits 0 in both modules at go1.27.4 and at go1.26.11.
* `make pre-add-check`, `make check-tools` and the gates of step 3 exit 0.
* CI's two pin checks pass on the tree and fail on the two copies.
* Both test hosts pass the tree's tests, at the version step 1 gave them.
* CI's six jobs pass on the pushed commit.

## Rollout and Rollback

* **Rollout:** on `main` with the commit of step 5. It is released in `v1.36.0`, whose notes, written in 0054's phase 5,
  name the floor and its reason.
* **Rollback before the release:** revert the commit, and put `GOTOOLCHAIN` back on each machine to the value step 1
  recorded. `govulncheck` then fails again on GO-2026-6859.
* **After the release:** the pins only move forward, by a record like this one. A later release does not lower the
  `go` line.

## Execution record

Not started. The PLAN is `proposed` until the owner approves it.

---
status: in-progress
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

### Approval (2026-11-10)

The owner committed the pair (`3b46d36`) and said "committed, proceed". That is the MADR's acceptance and this PLAN's
approval. `3b46d36` is the base. 0054's phase 4 is unstaged in the tree, as Scope says.

### Step 1, the machines (2026-11-10)

| Machine | Before | What was changed | After |
| :--- | :--- | :--- | :--- |
| This machine | `go env GOTOOLCHAIN` `go1.27.3`, from Go's environment file; `go version` go1.27.3 | `go env -w GOTOOLCHAIN=go1.27.4` | `go version go1.27.4 darwin/arm64`; `GOROOT` is the toolchain module in the module cache |
| The Linux host | `go1.27.3`, from Go's environment file; go1.27.3 | `go env -w GOTOOLCHAIN=go1.27.4` | `go version go1.27.4 linux/amd64` |
| The Windows host | `go1.27.3`, from a user-level Windows environment variable; go1.27.3 | the user variable set to `go1.27.4` (deviation 1) | `go version go1.27.4 windows/amd64` |

Each first `go version` printed "go: downloading go1.27.4". No machine has `GOTOOLCHAIN` as a machine-level or shell
variable now, but the Windows host's user variable.

To put a machine back: `go env -w GOTOOLCHAIN=go1.27.3` on the first two; on the Windows host, the user variable set to
`go1.27.3`.

### Deviation 1, the Windows host's variable (2026-11-10)

* **Found:** in step 1. On the Windows host `go env -w GOTOOLCHAIN=go1.27.4` printed "warning: go env -w
  GOTOOLCHAIN=... does not override conflicting OS environment variable", and `go version` still printed go1.27.3.
  `[Environment]::GetEnvironmentVariable("GOTOOLCHAIN", "User")` was `go1.27.3`; the machine-level one was empty. The
  step's command had written Go's environment file there (`GOTOOLCHAIN=go1.27.4`), to no effect.
* **Put to the owner** with three options. The other two were removing the user variable, and leaving the host on
  go1.27.3 with its run recorded as such.
* **Decided:** set the user variable to `go1.27.4` (the owner, 2026-11-10). Done with
  `[Environment]::SetEnvironmentVariable("GOTOOLCHAIN", "go1.27.4", "User")`.
* **Then:** a new SSH session still had `go1.27.3`, with the registry at `go1.27.4` and no shell profile setting it.
  That looked like an environment the host's SSH service held. Put to the owner with three options, the others being
  that the owner restarts it and that the host stays on go1.27.3, the owner chose that the agent restart `sshd` over
  SSH. It was restarted by a one-time scheduled task run as SYSTEM (`net stop sshd & net start sshd`), so that the
  stop could not end the command before the start. The service came back (a new `sshd` process, started 21:56:27), and
  the task was removed.
* **The cause was not the service.** After the restart the session still had `go1.27.3`. Two older `sshd` processes
  remained, from 21:46: the agent's own SSH client keeps one connection to the host open and runs each command on it,
  so every "new session" had the environment of a connection opened before the variable changed. With that connection
  closed (`ssh -O exit`), the next one had `go1.27.4`. So the restart was very likely not needed. It did no harm that
  was found: the service runs, and its start type is still Automatic.
* **Left on the host:** the user variable at `go1.27.4`, and Go's environment file with `GOTOOLCHAIN=go1.27.4`, which
  agrees with it.

### Step 2, the pins (2026-11-10)

* `go mod edit -go=1.26.11 -toolchain=go1.27.4` in each module: both `go.mod` files read `go 1.26.11` and
  `toolchain go1.27.4`.
* `.github/workflows/ci.yml`: the matrix's `go: ['1.26.11', '1.27.4']`, and `go: '1.26.11'` in the two `include`
  entries.
* `README.md:20`-`:21` and the two fixtures of `cmd/generate/internal/mutate/mutate_test.go` (`:92`, `:287`).
* The workflow, the README and the fixtures were edited by a script that asserts how many times each old text occurs
  before it replaces it: 1 and 2 in the workflow, 1 in the README, 2 in the fixtures.
* `tools/test_tools.py`: nothing, by question 2's answer.

### Step 3, the checks here (2026-11-10)

| Check | Result |
| :--- | :--- |
| 1, the old versions outside the records | found in `docs/README.md`'s two rows for 0028, in this pair's own MADR row there, and in 14 lines of `tools/test_tools.py` |
| 2, `go version`; `go mod tidy -diff` in both modules | `go version go1.27.4 darwin/arm64`; exit 0, nothing printed |
| 3, `govulncheck@v1.9.0 ./...`, both modules, at go1.27.4 and with `GOTOOLCHAIN=go1.26.11` | four runs, each exit 0, "No vulnerabilities found." |
| 4, CI's two pin checks on the tree | both exit 0 |
| 4, on a copy whose `go.mod` says `toolchain go1.27.3` | "the newest is the toolchain line" exit 1; the other exit 0 |
| 4, on a copy whose `cmd/generate/go.mod` says `go 1.26.10` | "the floor is the go line" exit 1; the other exit 0 |
| 5, `actionlint .github/workflows/ci.yml` | exit 0 |
| 6, `make pre-add-check` with the step's five files | exit 0: "1 file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on, uncached), tidy, govulncheck." |
| 7, `make check-tools` | exit 0: 89 tests of `tools/`, 41 of `scripts/`, "check-ci-filters: 0 problem(s)" |
| 8, `tools/gates.py floor api tidy nojsonv2 winvet platforms` | exit 1: `floor` failed, the other five passed; `floor` run again alone, exit 0 (below) |

* **Check 1** found one row more than the step listed: this pair's MADR row in `docs/README.md` names go1.27.3 and
  go1.26.10 as the versions the advisory fails. It is a record's index row, not a pin.
* **Check 4** ran the two steps' commands as `ci.yml` has them, in bash with `set -euo pipefail`, each with
  `GOTOOLCHAIN` set to the version its job installs.
* **Check 8, `api`:** against `v1.35.0`, 0 incompatible and 57 compatible. `platforms`: 13 of 13.
* **Check 8, `floor`, the first run: exit 1.** At go1.26.11, `go vet` passed in both modules and `cmd/generate`'s race
  tests passed. The root's `go test -race -count=1 ./...` failed in `agentproc` alone, three tests on their 30 s
  bounds: `TestBlockedTapDetached` and `TestDetachedTapIsPrefix`, "the bulk agent did not say it was ready within 30s"
  (`tap_test.go:294`, `:335`), and `TestInProcessDetachedTapIsPrefix`, "the connection did not read Serve's first 56
  MiB within 30s" (`inprocess_test.go:461`). The gate set had been started while check 6's pre-add check was still
  running, the agent's error, and this machine's load averages were 64.75, 72.08 and 55.39 on 10 CPUs just after.
* **The second run, `tools/gates.py floor` alone: exit 0,** `agentproc` in 17.705 s at `go version go1.26.11
  darwin/arm64`. It waited 25 minutes for the load to fall under 5 and ran when it had not: the one-minute average was
  24.67 at its start, from other work on this machine. So the pass was not on an idle machine.
* These are the three tests that failed once under load in
  [0031-PLAN-verification-tools-in-repository.md](0031-PLAN-verification-tools-in-repository.md), step 13, and passed
  when run again. Their bounds are left as they are, as there: changing them is outside this PLAN. The same tests
  passed under `-race` in check 6, and again in the pre-add check of step 4 below.

### Step 4, the hosts (2026-11-10)

Each host's `go version` was printed on a new connection first: go1.27.4 on both.

| Run | Linux host | Windows host |
| :--- | :--- | :--- |
| `tools/gates.py linux windows`, exit 1 | exit 0, both modules, `go version go1.27.4 linux/amd64` | the root module passed; `cmd/generate` exit 1, `go version go1.27.4 windows/amd64` |
| `tools/gates.py windows`, after the fix below, exit 0 | not run again | exit 0, both modules, `go version go1.27.4 windows/amd64` |

Each run printed that its directory on the host was removed.

### Deviation 2, a generator test fails on Windows (2026-11-10)

* **Found:** in step 4's first run. On the Windows host, `cmd/generate`'s `TestGeneratedFilesUpToDate` failed:
  "`main_test.go:79`: `unstable\unstable_gen.go` is a generated file run would remove".
* **Not this PLAN's change.** The test compared a path built with the platform's separator with the generator's slash
  paths. It fails on Windows since `894726f` added the first generated file in a subdirectory, at go1.27.3 as at
  go1.27.4. CI's two Windows jobs would have failed on it at the first push.
* **Put to the owner** with three options. The other two were the fix inside this PLAN's commit, and a new record
  pair.
* **Decided:** a deviation of 0054's PLAN, in a commit of its own before this one (the owner, 2026-11-10). The cause,
  the one-line fix, its planted break on both machines and its checks are in
  [0054-PLAN-adopt-acp-schema-1-31-0-and-client-system-prompts.md](0054-PLAN-adopt-acp-schema-1-31-0-and-client-system-prompts.md),
  deviation 6.
* **Here:** nothing in this PLAN's files changed for it. Step 4's second run is on the tree with that fix. The
  fix's pre-add check, `make pre-add-check FILES="cmd/generate/main_test.go"`, exited 0 on the same tree, which holds
  step 2's pins.

### Step 5, the records (2026-11-10)

* `make check-records`, `make markdownlint` and `make disclosure-check` exited 0.
* 0054's deviation 6 was committed first, as `8cf69bf`, at the owner's word ("Commit to main. Then proceed").
* This PLAN's lines are staged alone on that base: of `README.md` the Go version sentence, of `docs/README.md` this
  pair's two rows. The other hunks of those two files are 0054's phase 4 and stay unstaged.
* **Not done yet:** the owner's push, and CI's six jobs at `1.26.11` and `1.27.4`. This PLAN stays `in-progress` until
  they pass.

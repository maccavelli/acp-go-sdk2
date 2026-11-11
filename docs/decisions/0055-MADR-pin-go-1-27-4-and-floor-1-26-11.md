---
status: accepted
date: 2026-11-10
decision-makers: the repository owner
consulted: "this tree at `894726f`; the Go vulnerability database's GO-2026-6859; go.dev/doc/devel/release and go.dev/dl; the module mirror; `go version` on this machine and both test hosts; gobble-cli's `go.mod` at `d6c10a5`, read as data"
informed: gobble-cli, magic-cli-remote
---

# Pin Go 1.27.4 as the toolchain and Go 1.26.11 as the floor

## Context and Problem Statement

On 2026-11-11 at 02:51 UTC the Go vulnerability database published GO-2026-6859. From then on `govulncheck`, the last
step of `make pre-add-check` and a step of CI, fails on every commit of this repository. It stopped
[0054-PLAN-adopt-acp-schema-1-31-0-and-client-system-prompts.md](0054-PLAN-adopt-acp-schema-1-31-0-and-client-system-prompts.md)
at the end of its phase 4 (its deviation 5), and the owner chose to move the Go pins under a record of their own first,
as [0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md](0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md) did for the last ones.

Every fact below was read on 2026-11-10 (local time), on `894726f`, with 0054's phase 4 written and unstaged. The
repository was only read.

### The advisory

* **GO-2026-6859** (CVE-2026-97054): "HTTP/1 server crash due to MaxBytesReader header race in net/http". Its details:
  "For HTTP/1 servers, when MaxBytesReader's limit is exceeded, "Connection: close" header is automatically set without
  synchronization, which might race with another header being set explicitly in the server handler. As a result, an
  HTTP/1 server that utilizes MaxBytesReader or MaxBytesHandler can crash due to runtime panic if its handler reads the
  request body concurrently with writing the response header."
* **Affected:** the standard library before go1.26.11, and go1.27 before go1.27.4.
* **What `govulncheck` reports here:** `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` in the root module exits
  with "Your code is affected by 1 vulnerability from the Go standard library", "Found in: net/http@go1.27.3", "Fixed
  in: net/http@go1.27.4", and 46 traces. In `cmd/generate` it exits 0.
* **Where this module uses it.** `go list -deps` finds `net/http` under `./remote` alone, of the module's packages:
  not under the root package, `agentproc`, `clienthost`, `acptest` or `unstable`, nor under any package of
  `cmd/generate`. `remote/httpserver.go:86` reads a POST's body with
  `io.ReadAll(http.MaxBytesReader(w, r.Body, …))`, on the handler's goroutine, and writes no header until that read
  has returned (`:86`-`:103`). So `remote.Handler` does not read the body while it writes the header, which is the
  advisory's condition. That is a reading of the code. No test for the race was run.

### What the two releases are

go.dev/doc/devel/release: "go1.27.4 (released 2026-11-10) includes security fixes to cgo, the go command, the compiler,
the linker, and the net/http package, as well as bug fixes to the assembler, the compiler, the runtime, toolchain
bootstrapping, and the crypto/fips140, crypto/hkdf, crypto/rsa, crypto/tls, and net packages." The entry for go1.26.11,
released the same day, names the same security fixes.

Both are available. go.dev/dl lists go1.27.4 and go1.26.11 as stable. The module mirror serves their toolchain modules:
`golang.org/toolchain@v0.0.1-go<version>.<os>-<arch>.info` answers HTTP 200 for `darwin-arm64`, `linux-amd64` and
`windows-amd64`, for both versions.

### Where the pins are

`git grep -n -E '1\.27\.3|1\.26\.10'`, with the records under `docs/decisions/` and `docs/reports/` left out, finds:

| Id | Place | What it pins |
| :--- | :--- | :--- |
| T1 | `go.mod:5`, `toolchain go1.27.3` | the toolchain the root module prefers where it is the main module |
| T2 | `cmd/generate/go.mod:5`, `toolchain go1.27.3` | the same for the generator's module |
| T3 | `.github/workflows/ci.yml:57`, `go: ['1.26.10', '1.27.3']` | the version CI's newest jobs install; they run with `GOTOOLCHAIN: local` |
| T4 | `README.md:20`-`:21`, "CI tests Go 1.26.10 and 1.27.3." | what the README tells a user |
| F1 | `go.mod:3`, `go 1.26.10` | the version every consumer must have |
| F2 | `cmd/generate/go.mod:3`, `go 1.26.10` | the same for the generator's module |
| F3 | `ci.yml:57`, `:59` and `:66`, the matrix's `'1.26.10'` and its two `include` entries | the version CI's floor jobs install |
| F4 | `README.md:20`, "It requires Go 1.26.10 or newer" | what the README tells a user |
| F5 | `cmd/generate/internal/mutate/mutate_test.go:92`, `:287`, `go 1.26.10` in two fixtures' `go.mod` | the floor, as 0024 decided for these fixtures |

It also finds 14 lines of `tools/test_tools.py` and two rows of `docs/README.md`. The `docs/README.md` rows name what
0028 decided, and stay. The `tools/test_tools.py` lines are inputs of the tools' own unit tests: a version string given
to `floor.floor_env` or written into a scratch `go.mod`, with the result compared to that same string. They read no pin
of this repository.

Two CI steps keep the pins together, both from earlier records: "the floor is the go line" (0024) and "the newest is the
toolchain line" (0028). `ci.yml:139`-`:144` runs `govulncheck` on the Linux job at **each** version, so the floor job,
at go1.26.10, fails on this advisory as the newest job does.

### What the machines run

* **Here:** `go version` prints `go1.27.3`. `go env GOTOOLCHAIN` prints `go1.27.3`, set in Go's environment file
  (`go env -changed`), and `GOROOT` is a go1.27.3 installation. No go1.27.4 is installed.
* **The Linux and the Windows test hosts,** over SSH: `go version` prints `go1.27.3` on each, and `go env GOTOOLCHAIN`
  is `go1.27.3` on each.

A `GOTOOLCHAIN` set to one version overrides the `toolchain` line (0028, quoting go.dev/doc/toolchain). So moving T1 and
T2 alone changes nothing these machines run: `make pre-add-check` here would still scan go1.27.3's standard library and
still fail. An environment variable `GOTOOLCHAIN=go1.27.4` on a command overrides the environment file for that
command, and the go command then downloads that toolchain into the module cache.

### The consumers

gobble-cli's `go` line is `1.27.3` (`go.mod`, at `d6c10a5`), above either floor. A floor of 1.26.11 raises no `go` line
that is already at 1.27. magic-cli-remote was not read for this record; 0028 found it at 1.27.2.

## Decision Drivers

* **The gates must pass.** `govulncheck` is a pre-add check and a CI step, and `AGENTS.md` commits no file that fails.
  0054's phase 4 and its release wait on it.
* **0024's policy:** CI runs the floor and the latest stable release, each pinned exactly.
* **One version in every pin,** so CI tests what the machines build with (0028).
* **Consumers:** a dependency's `toolchain` line does not reach them. Its `go` line does.
* **The fix should reach users of `remote`,** who serve HTTP with the Go they build with.

## Considered Options

* **O1.** Move the toolchain pins, T1 to T4, to go1.27.4, and the floor, F1 to F5, to go1.26.11.
* **O2.** Move the toolchain pins to go1.27.4 and leave the floor at go1.26.10.
* **O3.** Leave the pins, and accept or exclude the advisory.

## Decision Outcome

Chosen option: "O1", accepted by the owner on 2026-11-10, because it is the only option under which every gate passes with no check loosened.

* O2 leaves CI's floor job failing `govulncheck`, since that step runs at both versions.
* O3 leaves a check failing, or loosens it. `scripts/go-precheck.sh`'s `GO_PRECHECK_SKIP_VULN=1` is for offline work,
  not for a finding.

### Specification

* `go.mod` and `cmd/generate/go.mod`: `go 1.26.11` and `toolchain go1.27.4`.
* `ci.yml`: `go: ['1.26.11', '1.27.4']`, and the two `include` entries `'1.26.11'`. No step changes: the two checks of
  0024 and 0028 read the matrix.
* `README.md`: "It requires Go 1.26.11 or newer, the `go` line of `go.mod`; CI tests Go 1.26.11 and 1.27.4."
* The two fixtures of F5 write `go 1.26.11`.
* The floor's reason goes in the release notes: go1.26.11 carries the `net/http` fix of GO-2026-6859, which `remote`'s
  users want.

This record supersedes the versions of
[0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md](0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md), `1.26.10` and `go1.27.3`.
0024's policy and 0028's check stand.

It is released in `v1.36.0`, with
[0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md](0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md):
`main` already carries 0054's first three phases, so no release of this change alone can be cut from it.

### Questions for the owner

1. **The machines.** `GOTOOLCHAIN` is `go1.27.3` on this machine and both test hosts, so each keeps running go1.27.3
   whatever `go.mod` says. 0028 left that setting to each machine's owner.
   * Recommended: the PLAN moves all three, with `go env -w GOTOOLCHAIN=go1.27.4`, after printing the value it
     replaces, and checks `go version` on each. Every later `make pre-add-check` then passes with no prefix.
   * Alternative: the owner moves them; the PLAN checks `go version` on each and stops if one is not go1.27.4.
   * Alternative: no machine setting changes. The PLAN's own commands run with `GOTOOLCHAIN=go1.27.4` in their
     environment, here alone, and the hosts' runs are recorded as go1.27.3. Every later check on this machine needs the
     same prefix, 0054's included.
2. **The version strings in `tools/test_tools.py`.**
   * Recommended: leave them. They are test inputs, and no rule says they follow the pin.
   * Alternative: move all 14 lines, so the `git grep` of the pins finds only records.
   * Alternative: move those that stand for a floor or a toolchain, and leave the CI job names of the `ci_jobs` tests.

#### Decided with the owner, 2026-11-10

Each question was put with three options, the recommended one first. The owner chose the recommendation for both.

| Question | Answer |
| :--- | :--- |
| 1. The machines | The PLAN moves all three with `go env -w GOTOOLCHAIN=go1.27.4`, recording the value it replaces, and checks `go version` on each. |
| 2. The strings in `tools/test_tools.py` | Left as they are. |

### Consequences

* Good, because `govulncheck` passes again at both versions, so 0054 can finish and `v1.36.0` can be released.
* Good, because a program that serves `remote.Handler` and builds with this module's `toolchain` line gets the
  `net/http` fix.
* Bad, because the floor moves: a consumer whose `go` line is under 1.26.11 has it raised when it requires `v1.36.0`.
  Both known consumers are on 1.27.
* Bad, because each security release of Go repeats this change by hand, as 0028 already said. The interval this time
  was three weeks.
* Neutral, because the `toolchain` line does not reach consumers.

### Confirmation

* `git grep -n -E '1\.27\.3|1\.26\.10'` finds only records and, by question 2's answer, `tools/test_tools.py`.
* `go version` prints go1.27.4 on each machine the PLAN runs on, by question 1's answer.
* `govulncheck` exits 0 in both modules at go1.27.4 and at go1.26.11.
* `make pre-add-check`, `make check-tools` and `python3 tools/floor.py` pass.
* The commands of CI's two pin checks, run locally, pass on the tree and fail on a copy whose matrix and `toolchain`
  line disagree.
* The test hosts pass the tree's tests. CI's six jobs pass on the pushed commit.

## Pros and Cons of the Options

### O1. Both pins

* Good, because every gate passes, and the fix reaches both supported lines.
* Bad, because the floor moves for consumers.

### O2. The toolchain alone

* Good, because no consumer's `go` line can be raised.
* Bad, because CI's floor job fails `govulncheck` on every run, the weekly one too.

### O3. Leave the pins

* Good, because nothing changes.
* Bad, because a failing security check is left failing, or is switched off.

## More Information

* **The advisory:** <https://pkg.go.dev/vuln/GO-2026-6859>, with its fix <https://go.dev/cl/862185> and report
  <https://go.dev/issue/82563>.
* **Related records:** [0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md),
  the policy; [0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md](0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md), the last
  pins and the newest version's check;
  [0043-MADR-remote-http-and-websocket-transport.md](0043-MADR-remote-http-and-websocket-transport.md), which brought
  `net/http` into the module.
* **Not decided here:** a way to learn of a Go security release without a failed check. CI's weekly run
  (`ci.yml:32`) is what would have shown this one without a push.
* **The PLAN** is [0055-PLAN-pin-go-1-27-4-and-floor-1-26-11.md](0055-PLAN-pin-go-1-27-4-and-floor-1-26-11.md).

## Released in v1.36.0, 2026-11-11

The pins shipped in `v1.36.0`, the tag naming `3be7af6`, with
[0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md](0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md),
as this record's Specification said. CI's six jobs passed at `1.26.11` and `1.27.4` on `main` and on the tag; the
PLANs' execution records hold the runs.

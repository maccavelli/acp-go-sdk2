---
status: accepted
date: 2026-10-21
decision-makers: Repository owner
---

# Pin Go 1.27.3 as the toolchain and Go 1.26.10 as the floor

## Context and Problem Statement

On 2026-10-21 the owner asked that the repository be pinned to Go 1.27.3. The repository pins go1.27.2 in four places, while the
machines that build and test it already run go1.27.3. This record holds where the pins are, what go1.27.3 changes, why the drift went
unnoticed, and what to change.

Every fact below was read that day, on `5930996` with the 0027 PLAN's step 5 staged; the repository was only read.

### Where go1.27.2 is pinned

`git grep -n "1\.27\.2"`, records under `docs/decisions/` and `docs/reports/` left out, finds:

| Id | Place | What it pins |
| :--- | :--- | :--- |
| T1 | `go.mod:5`, `toolchain go1.27.2` | the toolchain the root module prefers where it is the main module |
| T2 | `cmd/generate/go.mod:5`, `toolchain go1.27.2` | the same for the generator's module |
| T3 | `.github/workflows/ci.yml:29`, `go: ['1.26.9', '1.27.2']` | the version CI installs for its newest job; the jobs run with `GOTOOLCHAIN: local`, "Each job runs the version it installed, never one go.mod selects" (`:37`) |
| T4 | `README.md:21`, "CI tests Go 1.26.9 and 1.27.2." | what the README tells a user |

The other hits are the titles and rows of earlier records in `docs/README.md` (0002, 0004), which name what those records decided and
stay as written.

### What the machines run

* **Here:** `go version` prints "go version go1.27.3 darwin/arm64"; `go env GOTOOLCHAIN` prints `go1.27.3`, from line 9 of Go's
  environment file (`go env -w`), not from the shell; `GOROOT` is a go1.27.3 installation dated 2026-10-19.
* **The Windows host and the Linux host** reached over SSH: `go version` prints go1.27.3 on each, and `go env GOTOOLCHAIN` is `go1.27.3`
  on each.
* **When it changed here:** every binary in this session's scratch directories records its compiler (`go version <file>`). Those built on
  2026-10-20 up to 19:44 name go1.27.2; those built from 22:31 that day name go1.27.3. So the gates of
  [0027-PLAN-agent-process-helper.md](0027-PLAN-agent-process-helper.md), steps 2 to 5, ran on go1.27.3, while their records say
  go1.27.2; that PLAN carries the correction. The 0026 work and the `v1.18.0` release ran before the change, on go1.27.2, as their
  records say.

A `GOTOOLCHAIN` set to one version overrides the `toolchain` line. go.dev/doc/toolchain: "When GOTOOLCHAIN is set to <name> (for
example, GOTOOLCHAIN=go1.21.0), the go command always runs that specific Go toolchain"; the line can select a newer toolchain only under
`<name>+auto`, `auto` being "shorthand for GOTOOLCHAIN=local+auto". Under `local`, "the go command always runs the bundled Go
toolchain", so CI's `local` ignores the line, and T3 alone decides what CI runs.

### Where the floor, 1.26.9, is pinned

`git grep -n "1\.26\.9"`, the records left out, finds:

| Id | Place | What it pins |
| :--- | :--- | :--- |
| F1 | `go.mod:3`, `go 1.26.9` | the version every consumer must have |
| F2 | `cmd/generate/go.mod:3`, `go 1.26.9` | the same for the generator's module |
| F3 | `.github/workflows/ci.yml:29` and `:31`, the matrix's `'1.26.9'` and its `include` entry | the version CI's floor job installs; its step "the floor is the go line" requires the two to agree |
| F4 | `README.md:20`, "It requires Go 1.26.9 or newer, the `go` line of `go.mod`; CI tests Go 1.26.9" | what the README tells a user |
| F5 | `cmd/generate/internal/mutate/mutate_test.go:92`, `:287`, `go 1.26.9` in two fixtures' `go.mod` | the floor, as 0024 decided "The two fixtures of G4 write the floor" |

The Makefile's `LEAKGUARD_ENV` and `scripts/go-precheck.sh` set the leak guard's experiment for any `go1.26*` (`Makefile:36`,
`go-precheck.sh:31`), so a 1.26.10 floor needs no change there. Both consumers' `go` lines are `1.27.2`, above either floor.

### What go1.27.3 changes

go.dev/doc/devel/release: "go1.27.3 (released 2026-10-19) includes security fixes to the crypto/x509 package." The same day,
"go1.26.10 (released 2026-10-19) includes security fixes to the crypto/x509 package." go.dev/dl lists both.

No package of this module imports `crypto/x509` or `net/http`: `go list -deps` finds neither for the root package, `agentproc`,
`example/client` or `example/agent`, nor `crypto/x509` for any package of `cmd/generate`. The fix reaches no code here; the pin is for
consistency with the machines and for the policy below.

Both are available: go.dev/dl lists go1.27.3 and go1.26.10 as stable, 45 files each, and the module mirror serves their toolchain
modules (`golang.org/toolchain@v0.0.1-go1.27.3.<os>-<arch>.info` and the go1.26.10 ones, HTTP 200 for `darwin-arm64`, `linux-amd64` and
`windows-amd64`).

### The policy this applies

[0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md), phase 1:

* "both modules also gain `toolchain go1.27.2`, the owner's preferred version. It applies only where this module is the main module.
  Consumers still need 1.26.9";
* "CI runs a matrix of the floor, `1.26.9`, and the latest stable release … Decided under question 8: the newest version is pinned,
  `1.27.2`, and a CI step checks that the matrix's floor equals the `go` line."

The floor has a check (`ci.yml`, "the floor is the go line"); the newest version has none, so T1 to T3 can drift from each other and
from the machines without a failure.

## Decision Drivers

* **The owner's request:** go1.27.3.
* **0024's policy:** CI's newest job runs the latest stable release, pinned exactly; the `toolchain` line is the owner's preferred version.
* **One version in every pin,** so CI tests what the machines build with.
* **Consumers unaffected:** a dependency's `toolchain` line is ignored; only the main module's applies.
* **No change to the floor unasked:** the `go` line is what consumers must have.

## Considered Options

* **O1.** Move T1 to T4 to go1.27.3, and add a CI check that the matrix's newest version equals the `toolchain` line in both modules;
  the floor, F1 to F5, as question 2 decides.
* **O2.** Move T1 to T4 to go1.27.3 only.
* **O3.** Move only the `toolchain` lines, T1 and T2.

## Decision Outcome

Chosen option: "O1", with the owner's answers of 2026-10-21 (see "Decided with the owner, 2026-10-21"): question 1 as recommended,
question 2 the alternative, a 1.26.10 floor. It applies 0024's policy to a new release and, with the check, keeps the pins from drifting
apart again.

* O2 leaves the drift found here possible again.
* O3 leaves CI testing go1.27.2 while the `toolchain` line names go1.27.3, the drift this record exists to end.

### Specification, as decided

* `go.mod` and `cmd/generate/go.mod`: `go 1.26.10` and `toolchain go1.27.3`.
* `ci.yml`: `go: ['1.26.10', '1.27.3']`, and the floor's `include` entry `'1.26.10'`; a step on the newest job, beside "the floor is the go line": "the newest is the toolchain line",
  which requires `go version` to print `go1.27.3`, and each module's `toolchain` line to equal `go` plus the matrix version
  (`go mod edit -json` reports the line as `.Toolchain`, a string, `go1.27.2` today).
* `README.md`: "It requires Go 1.26.10 or newer, the `go` line of `go.mod`; CI tests Go 1.26.10 and 1.27.3."
* The two fixtures of F5 write `go 1.26.10`.

### Questions

Each has a recommended answer.

1. **A check for the newest version.**
   * Recommended: yes, the CI step above, so a matrix and a `toolchain` line that disagree fail CI, as the floor's do.
   * Alternative: no check; the pins move together by hand.
2. **The floor.**
   * Recommended: unchanged, `1.26.9`. Raising a `go` line requires every consumer to have that version; the `crypto/x509` fix reaches no
     code here, and a consumer's own toolchain decides its exposure.
   * Alternative: `1.26.10`, which carries the same fix as go1.27.3, as 0024 moved the floor to the release carrying go1.27.2's fixes.

#### Decided with the owner, 2026-10-21

| Question | Answer |
| :--- | :--- |
| the version | The owner's request: go1.27.3. |
| 1. A check for the newest version | As recommended: the CI step "the newest is the toolchain line". |
| 2. The floor | **The alternative:** `1.26.10`, in F1 to F5. |

This record supersedes the floor version of [0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md),
`1.26.9`, and its `toolchain go1.27.2`; 0024's policy, the floor and the newest release with the newest pinned, stands.

### Consequences

* Good, because CI, the `toolchain` lines and the machines name one version.
* Good, because, with question 1's check, a later bump that misses a place fails CI.
* Neutral, because the `toolchain` line does not reach consumers: a dependency's `toolchain` line does not apply to them.
* Bad, because the floor does: a consumer whose `go` line is under 1.26.10 has it raised to 1.26.10 when it requires the next release;
  both known consumers are on 1.27.2. The release notes name it.
* Neutral, because a machine whose `GOTOOLCHAIN` is `auto` and whose Go is older now downloads go1.27.3 into its module cache on first
  use.
* Bad, because each patch release repeats this change by hand; nothing in the repository proposes it.

### Confirmation

The PLAN will check, each on a planted break in a scratch copy:

* no non-record file names `1.27.2` or `1.26.9` (`git grep`);
* CI's floor job, at 1.26.10, and the go1.26.10 gate pass;
* `GOTOOLCHAIN=go1.27.2+auto go version`, run in each module, selects go1.27.3 from the `toolchain` line;
* the CI step fails when the matrix and a `toolchain` line disagree, as a local run of its commands shows;
* `actionlint` passes on `ci.yml`, and CI's six jobs pass on the pushed commit.

## Pros and Cons of the Options

### O1. All four places, and a check

* Good, because it meets every driver and ends the drift.
* Bad, because it adds a CI step to keep.

### O2. All four places

* Good, because it is the smallest complete change.
* Bad, because nothing stops the next drift.

### O3. The `toolchain` lines only

* Good, because it is one line in each module.
* Bad, because CI keeps testing go1.27.2.

## More Information

* **Related records:** [0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md),
  the floor and the policy; [0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md), the
  first pin, superseded by 0024; [0027-PLAN-agent-process-helper.md](0027-PLAN-agent-process-helper.md), whose step records carry the
  correction of the toolchain they ran on.
* **Not decided here:** setting `GOTOOLCHAIN` on any machine, which is each machine's owner's; and a dependency bot, which the
  repository has none of.
* **The PLAN** is [0028-PLAN-pin-go-1-27-3-and-floor-1-26-10.md](0028-PLAN-pin-go-1-27-3-and-floor-1-26-10.md).

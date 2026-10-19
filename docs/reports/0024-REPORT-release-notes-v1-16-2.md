# Release notes for v1.16.2, against v1.16.1

Observed 2026-10-19 on `1179253`, for [0024-PLAN-go-1-26-9-floor.md](../decisions/0024-PLAN-go-1-26-9-floor.md), step 4. This report
drafts the text of the GitHub release for `v1.16.2`, as `RELEASING.md` describes it. It decides nothing.

Each statement names its source:

- a record;
- a commit;
- or the exported API listing of the 0013 report's listing program, run on `git archive v1.16.1` (2429 lines) and on the tree at
  `1179253` (2429 lines).

Bring it up to date if more changes land before the tag.

- No exported API changes, and no SDK code changes, so the release is proposed as the patch `v1.16.2`
  ([0024-PLAN-go-1-26-9-floor.md](../decisions/0024-PLAN-go-1-26-9-floor.md), "Goal"). The owner confirms it at the tag.
- The one change a consumer meets is the minimum Go version.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.16.2

The ACP schema is still `1.25.0` (`git diff v1.16.1 -- schema/` is empty), and the exported API and the SDK's behaviour are unchanged.
This release is phase 1 of
[0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](../decisions/0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md):
the SDK now builds on Go 1.26.

#### Go version

- **The minimum Go version is 1.26.9,** where it was 1.27.2. `go.mod` reads `go 1.26.9`. Go 1.26 is one of the two supported Go
  releases. go1.26.9 carries the same security fixes as go1.27.2: both were released on 2026-10-08, with fixes to the go command and
  the `crypto/tls`, `html/template`, `net/http`, `net/textproto` and `os` packages. (`df19114`.)
- **`go.mod` also reads `toolchain go1.27.2`.** It applies only where this module is the main module, as when working in this
  repository. A module that depends on the SDK needs Go 1.26.9 or newer, nothing more. (`df19114`.)
- **CI tests both versions.** Every push runs the tests on Go 1.26.9 and Go 1.27.2, on Linux, macOS and Windows. (`1179253`.)
- **`encoding/json` differs between the two by default.** Go 1.27's default build uses `encoding/json/v2` underneath; Go 1.26's does
  not. The SDK's wire bytes are the same on both: the tests, the encoding goldens included, pass on each. The README's note on
  printing an empty `json.RawMessage` with `fmt` now gives each version's output. (`df19114`.)

#### For contributors

- **The root tests' leak guard runs on both versions.** (`df19114`.)
  - On Go 1.26 the `goroutineleak` profile is an experiment, which Go 1.27 rejects.
  - The `Makefile`'s and `scripts/go-precheck.sh`'s root test commands set `GOEXPERIMENT=goroutineleakprofile` on Go 1.26 only.
  - A bare `go test` on a forced Go 1.26 without it fails with that instruction.
- **The leak guard no longer misses a leak made by the last test to run.** It reports clean only after the profile has counted no
  leaked goroutine for 500 ms. Before, a planted leak in a test run alone was missed 18 times in 30. Runs of the whole suite were not
  affected. (`85ac115`.)
- **Five test literals no longer use a Go 1.27 language feature,** and two test fixtures write the new floor into their `go.mod`.
  (`df19114`.)

## Sources not in the release text

- **Commits since `v1.16.1`** (`git log v1.16.1..HEAD`):
  - `df19114` changes `go.mod`, `cmd/generate/go.mod`, the `Makefile`, `scripts/go-precheck.sh`, the README and three test files;
  - `1179253` changes `.github/workflows/ci.yml`;
  - `85ac115` changes `leak_test.go`;
  - `ce49693`, `c4777ff`, `805d997`, `11c50df` and `09d4a79` change records only (`git show --name-only`).
- **No SDK code changes.** `git diff --stat v1.16.1 HEAD -- '*.go' ':!*_test.go'` lists no file.
- **The API listing:** 0 lines removed, 0 added, `v1.16.1` against `1179253`.
- **The floor's evidence:** the MADR's findings G1 to G9 and the PLAN's steps 2 and 3. They include the scratch go1.26.9 toolchain's
  results, the planted breaks, and a simulation of CI's floor job, 8 of 8 steps as required. go.dev/doc/devel/release gives the two
  versions' security fixes. go.dev/doc/toolchain: "The choice of Go toolchain being used depends on the GOTOOLCHAIN environment
  setting and the go and toolchain lines in the main module's go.mod".
- **The leak guard's fix:** [0023-PLAN-connection-lifetime-and-test-leaks.md](../decisions/0023-PLAN-connection-lifetime-and-test-leaks.md),
  "Deviation, 2026-10-19, after the close", and step 6.
- **Not in this release:** the MADR's phases 2 to 4 (documentation, behaviour fixes, the `gorelease` gate and `Deprecated:`), and
  schema `1.26.0`.

---
status: in-progress
date: 2026-11-18
associated-madr: "0058-MADR-pin-golangci-lint-2-15-0.md"
---

# Implement 0058: pin golangci-lint v2.15.0

Associated MADR: [0058-MADR-pin-golangci-lint-2-15-0.md](0058-MADR-pin-golangci-lint-2-15-0.md)

## Goal

Every live place that names the `golangci-lint` version names v2.15.0, and the lint gate is clean with it.

## Scope

Approved in advance by the owner's ruling quoted in the MADR: "One short record pair per repository, approved now through CI (Recommended)". A finding whose fix is a real choice, a behaviour change, a new dependency or a release stops the work and goes to the owner.

Files changed:

| File | Place |
| :--- | :--- |
| `Makefile` | the install hint in `lint` |
| `scripts/go-precheck.sh` | the install hint |
| `.github/workflows/ci.yml` | the `go install` line of the lint step |
| `RELEASING.md` | the toolchain list |
| `docs/architecture.md` | the sentence on CI's `GOLANGCI_LINT` |
| `docs/decisions/0003-MADR-install-fleet-agent-workspace.md` | one dated pointer line, added |
| `docs/README.md` | this pair's two rows |

Not changed: any committed record's existing lines, `.golangci.yml`, any Go file, `go.mod`, `go.sum`.

## Implementation Steps

1. Run `make lint` with a v2.15.0 binary on a `git archive` copy of `b8b4145`. Done 2026-11-18: 4 of 4 runs report "0 issues.", exit 0.
2. Move the five pins to v2.15.0.
3. Add the dated pointer line to the 0003 MADR and the two index rows.
4. Run the gates below.

## Verification

gofmt; `go vet ./...` on the host and `GOOS=windows`, both modules; `make lint` with v2.15.0 (host and windows, both modules); `make pre-add-check`; `go mod tidy -diff` in both modules; the full test suite once, with `-race` as CI runs it; `make check-records`, `make markdownlint`, `make disclosure-check`; `actionlint` on `ci.yml` if installed.

## Rollout and Rollback

A tooling pin only, with no release. Rolling back is reverting the change commit.

## Execution record

* 2026-11-18, step 1: the red is clean; no finding to fix.

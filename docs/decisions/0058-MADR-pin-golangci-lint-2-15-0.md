---
status: accepted
date: 2026-11-18
decision-makers: the repository owner
consulted: "this tree at `b8b4145`; golangci-lint v2.15.0 run on a `git archive` copy of that commit"
informed: the coordinator
---

# Pin golangci-lint v2.15.0

## Context and Problem Statement

The fleet's repositories are moving to one `golangci-lint` release. On 2026-11-18 the owner ruled, in the owner's own words: "standardize on golangci-lint v2.15.0". Asked how the moves proceed in the repositories still pinned lower, the owner chose "One short record pair per repository, approved now through CI (Recommended)". As put to the owner: each repository gets a record, the pin moved everywhere it appears, the findings fixed in the same change, the coordinator's own gate, then commit and push; no releases are cut for this; an agent comes back to the owner only if a finding needs a real decision.

This repository pins v2.14.0. [0003-MADR-install-fleet-agent-workspace.md](0003-MADR-install-fleet-agent-workspace.md) decided that pin: CI's lint gate and the `make lint` install command. [0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md) only kept it ("remains the linter version"), and other records only mention it.

Read on 2026-11-18, on `b8b4145`:

* The pin appears in five live places: `Makefile:91` (the install hint), `scripts/go-precheck.sh:119` (its install hint), `.github/workflows/ci.yml:156` (the `go install` line), `RELEASING.md:16` and `docs/architecture.md:984`.
* No script or test holds the lint pin against another file, and none holds the Go pins together; the two CI steps at `ci.yml:104`-`:105` compare the `toolchain` lines to the matrix only.
* `make lint GOLANGCI_LINT=<v2.15.0 binary, go1.27.4>` on a `git archive` copy of `b8b4145` exits 0 with "0 issues." for the host, `GOOS=windows`, and `cmd/generate` on each. The new release reports nothing here.

## Considered Options

* Move the pin to v2.15.0 everywhere it appears now. The release reports nothing, so no finding needs a fix.
* Move it and suppress what it reports. Nothing is reported, so this has no content here.
* Stay at v2.14.0.

## Decision Outcome

Chosen option: "Move the pin to v2.15.0 everywhere it appears now", because the owner ruled the fleet standard, and the release is clean on this tree.

This supersedes [0003-MADR-install-fleet-agent-workspace.md](0003-MADR-install-fleet-agent-workspace.md) for the linter's version only. Every other part of 0003 stands. Committed records that name v2.14.0 stay as written: they say what was true when they were written.

### Consequences

* Good, because this repository lints with the version the fleet uses.
* Good, because the change is five version strings and no code.
* Neutral, because no check ties the lint pin to a second file, so a future move edits the five places by hand. The owner's ruling does not ask for one, and none is added.
* Bad, because a developer's installed binary at v2.14.0 differs from CI until reinstalled.

### Confirmation

* `make lint` with a v2.15.0 binary exits 0 on the host and `GOOS=windows`, in both modules.
* `grep -rn 'golangci-lint/v2/cmd/golangci-lint@v' Makefile scripts .github` and `grep -n 'v2\.1[45]\.0' RELEASING.md docs/architecture.md` show v2.15.0 only.
* `make pre-add-check`, `make check-records`, `make markdownlint` and CI pass.

Implementation is [0058-PLAN-pin-golangci-lint-2-15-0.md](0058-PLAN-pin-golangci-lint-2-15-0.md).

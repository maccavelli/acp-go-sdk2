---
status: accepted
date: 2026-10-25
decision-makers: Repository owner
---
# Align docs-only CI and disclosure gates

## Context and Problem Statement

This repository needs a focused Docs run for documentation-only pushes and pull requests. The earlier local decision is 0003-MADR-install-fleet-agent-workspace.md. This new decision adds a common quality and disclosure contract across the seven repositories.

Observed in this checkout on 2026-10-25: `ci.yml` has no path filter and there is no Docs workflow. Its Linux/current-Go matrix leg runs Markdownlint, actionlint, `make check-records` and `make check-tools`. `docs_test.go` reads the root README quickstart, and `tools/idscan.py` depends on the owner's private deny list.

Across the repositories, the common documentation contract is pinned Markdownlint, numbered-record and relative-link validation, a portable disclosure check for real home paths in tracked text, and any repository-specific check that reads Markdown. The owner's global pre-push disclosure guard uses private deny patterns and remains mandatory before a real push. Go lint, race, vulnerability, build and release checks remain in full CI.

## Decision Drivers

* A documentation-only change has a successful, visible Docs run.
* A Markdown file read by code must run that reader check in Docs or full CI.
* Non-document changes, tags, schedules and manual runs keep full CI behavior.
* Both workflows use the same documentation commands and pins.
* CI disclosure checks work without private host data; the pre-push guard still protects publication.

## Considered Options

* Keep the current workflow split and existing checks.
* Align the documentation gates and add a portable disclosure check, with a path and gate parity test.
* Detect changed paths inside one full workflow and conditionally skip expensive jobs.

## Decision Outcome

Chosen option: "Align the documentation gates and add a portable disclosure check, with a path and gate parity test", because it gives both workflows the same common documentation checks, covers local Markdown readers, and makes filter drift observable. This is proposed for the owner; it does not approve CI implementation.

Accepted 2026-10-25. The owner: "approve 0037 and proceed". The PLAN is approved for execution; it still authorizes no push or
tag.

### Consequences

* Good, because documentation-only changes can complete after focused checks.
* Good, because records, links, Markdown style and generic disclosure faults are checked consistently.
* Neutral, because a mixed change may start both workflows.
* Bad, because private identity patterns remain enforceable only by the local pre-push guard; CI has narrower coverage.
* Bad, because path-filtered full-CI jobs may be pending for a required pull-request check; required-status settings need review before rollout.

### Confirmation

The paired [0037-PLAN-align-docs-ci-and-disclosure-gates.md](0037-PLAN-align-docs-ci-and-disclosure-gates.md) requires failing planted inputs for each new gate, static workflow checks, and observed Docs-only and full-CI runs after an approved push. Local checks alone do not prove GitHub path-filter behavior.

## Pros and Cons of the Options

### Keep the current workflow split and existing checks

* Good, because it requires little work.
* Bad, because docs-only changes still start full CI and the common gates are incomplete.

### Align the documentation gates and add a portable disclosure check, with a path and gate parity test

* Good, because it establishes the same baseline while preserving local Markdown readers.
* Bad, because it adds a maintained scanner and parity test.

### Detect changed paths inside one full workflow and conditionally skip expensive jobs

* Good, because a single workflow can expose one stable status.
* Bad, because conditional jobs add control logic and can obscure whether documents passed.

## More Information

Add a narrow docs-only filter to push and pull_request, excluding root `README.md` and any Markdown read by tests, generated-code checks or maintainer tools. Add `docs.yml` with pinned Markdownlint, `make check-records`, any Markdown-reading checks identified by the audit, and a portable tracked-text disclosure target; run the same disclosure target in full CI. Keep actionlint in full CI for workflow edits; add a filter/gate parity test to `make check-tools`. Preserve schedule and tag behavior and the private-list `make push-guard` before actual pushes. Update `AGENTS.md` and `docs/architecture.md`.

This fleet alignment decision cites 0003-MADR-install-fleet-agent-workspace.md without rewriting its history. The implementation is [0037-PLAN-align-docs-ci-and-disclosure-gates.md](0037-PLAN-align-docs-ci-and-disclosure-gates.md).


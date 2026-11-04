---
status: proposed
date: 2026-11-04
decision-makers: the repository owner
consulted: "the repository's workflows, release tools and settings at `151b6e8`; GitHub's documentation of `GITHUB_TOKEN` and `gh release create`"
informed: none
---

# Create a tag's GitHub release in CI, once every check on the tag has passed

## Context and Problem Statement

Each release ends with steps the owner runs by hand ([RELEASING.md](../../RELEASING.md), "Tag and Publish", `:89-121`). The owner
tags the release commit and pushes the tag. An agent extracts the release body with `tools/release_body.py`. The owner runs
`gh release create` with that body. An agent then runs `tools/publication.py` and `tools/consumer.py`. The owner asked, on
2026-11-01, while `v1.31.1`'s tag was in CI, to move the creation of the release into CI/CD so it is no longer done by hand, and
for the best way to do it to be evaluated.

The question: where and how should CI create a tag's release, and what must hold before it does?

## Findings

From a read-only investigation of `151b6e8` on 2026-11-01.

### F1. The release steps today

* The body is the "Release text" section of the release-notes report, `docs/reports/NNNN-REPORT-release-notes-vX-Y-Z.md`,
  committed before the tag (`RELEASING.md:122-131`). `tools/release_body.py VERSION REV` extracts it as at `REV`, makes each
  relative link absolute at the tag, fails when a link's target is missing at `REV`, and scans the body with `tools/idscan.py`
  (`tools/release_body.py:77-81`).
* The owner runs `gh release create vX.Y.Z --verify-tag --title vX.Y.Z --notes-file <body>`, as for `v1.29.0` to `v1.31.0`.
* `tools/publication.py VERSION COMMIT BODY` then checks ten things: the release exists, is not a draft or prerelease, and has
  the extracted body; it is marked latest; the tag is annotated, names the commit, and is on `origin`'s `main`; CI passed on
  `main`'s head and on the tag; the proxy serves the version; and `go list -m @latest` and `go mod download` reach it.
  `tools/consumer.py VERSION` builds and runs a consumer at the Go floor against the published version.

### F2. CI on a tag

* `.github/workflows/ci.yml:10` runs CI on every `v*` tag push, with no path filter. Its one job, `validate`, is a matrix of six,
  Ubuntu, macOS and Windows at Go `1.26.10` and `1.27.3`.
* The workflow's permissions are `contents: read` (`:41-42`), and `scripts/check_ci_filters.py:289` requires that of `ci.yml` and
  `docs.yml`, at the workflow's level. The repository's default token is read-only. A job may ask for more.
* The concurrency group is `ci-${{ github.ref }}`, never cancelling a push's run (`:37-39`), so a tag has one run at a time.
* Every checkout is `actions/checkout` pinned by SHA, with `persist-credentials: false`. The Linux job already runs `actionlint`
  (`:153-158`), so a change to the workflow is linted in CI.

### F3. What a release in CI needs, and what it cannot have

* **The token.** `gh release create` needs `contents: write`. `GITHUB_TOKEN` gives it to the one job that asks for it; no stored
  secret is needed. `gh` is installed on GitHub's Ubuntu runners.
* **The identifier scan.** `idscan.py` reads the push guard's deny list from `~/.config/git/disclosure-deny` or the file
  `DISCLOSURE_DENY` names (`tools/idscan.py:32-35`), never from the repository. A runner has neither, so `release_body.py` exits 1
  there. The body is, however, an extract of a file that is committed and pushed: the global pre-push guard scanned it on the
  push, and CI's `make disclosure-check` scans the tracked text. The extraction adds only the repository's own public GitHub
  URLs.
* **Events.** A release created with `GITHUB_TOKEN` starts no other workflow (GitHub's rule against recursive runs, except
  `workflow_dispatch` and `repository_dispatch`). So anything that must follow the release runs in the same workflow run, and a
  tag pushed with `GITHUB_TOKEN` would start no CI at all.
* **The tag.** The tag stays a human act: its push publishes the version to the Go proxy at once, and a cached version cannot be
  withdrawn, only retracted ([0020-MADR-retract-accidental-v1-tag.md](0020-MADR-retract-accidental-v1-tag.md): `v1.13.0`, pushed
  on a commit whose CI failed, had no release, and stays in the proxy). A release made only after the tag's CI passes keeps that
  rule: a failing tag gets no release.

### F4. A notes file fixed after its tag cannot help

The body is read at the tag's commit. If the notes are missing or broken there, CI cannot create the release, and a fix needs a
new commit the tag does not name; the tag cannot move without the problem of 0020. The notes therefore have to be checked before
the tag. Today the agent runs `release_body.py` before asking for the tag; nothing in CI does.

### F5. Repository settings

The repository is public, with no rulesets and no tag protection. Third-party actions are allowed; SHA pinning is not enforced,
though every workflow here pins.

## Decision Drivers

* No release step by hand after the tag, as the owner asked.
* A release only for a tag whose every CI job passed, and whose notes exist and extract.
* No broader permission than the one job needs, and no stored secret.
* No new dependency or third-party action; the tools stay the repository's stdlib Python
  ([0031-MADR-verification-tools-in-repository.md](0031-MADR-verification-tools-in-repository.md)).
* A rerun is safe: it never overwrites a release silently.
* The notes are checked before the tag, when a fix is still possible.

## Considered Options

* **A.** A `release` job in `ci.yml`, after `validate`, on a `v*` tag only, running a repository tool that checks the tag and
  creates the release with `gh`.
* **B.** A separate `release.yml` started by `workflow_run` when CI completes on a tag.
* **C.** A separate `release.yml` on the tag push, with its own checks, not waiting for CI.
* **D.** A third-party tool: GoReleaser, release-please, or a release action such as `softprops/action-gh-release`.
* **E.** A `workflow_dispatch` workflow that creates both the tag and the release.

## Decision Outcome

Chosen option: **"A"**, proposed. It is gated on the very run that tested the tag, needs no new trigger, secret or dependency,
and keeps the release logic in a tested tool rather than in workflow shell.

The parts:

* **D1. The job.** `release` in `ci.yml`: `needs: validate`, so all six jobs must pass;
  `if: github.event_name == 'push' && startsWith(github.ref, 'refs/tags/v')`; on `ubuntu-24.04`; job-level
  `permissions: contents: write`, the workflow's staying `contents: read`; the same pinned checkout, with `fetch-depth: 0` and
  `persist-credentials: false`; `GH_TOKEN: ${{ github.token }}` for its one step.
* **D2. The tool,** `tools/publish_release.py VERSION COMMIT`, stdlib Python, tested in `tools/test_tools.py`. In order, failing at
  the first that does not hold:
  * the version is plain semver, `vMAJOR.MINOR.PATCH`;
  * the tag is annotated and names `COMMIT`, and `COMMIT` is on `origin/main`;
  * the body extracts with `release_body.py`'s `body()`, every link target present at `COMMIT`;
  * if the release does not exist, `gh release create VERSION --verify-tag --title VERSION --notes-file <body>`; if it exists
    with the same body, nothing (a rerun); if it exists with another body, fail, changing nothing.

  `--dry-run` does every check and prints the command it would run, creating nothing; it needs only read access.
* **D3. The identifier scan in CI.** `release_body.py` gains `--no-idscan`, which says it skipped the scan and why. Only the CI job
  passes it (F3). Locally the scan runs as now.
* **D4. The notes before the tag.** `make check-release-notes` runs `tools/publish_release.py --check`: for every release-notes
  report whose version has no tag yet, the body must extract at `HEAD` with every link target present. It joins `DOC_GATES` in
  `scripts/check_ci_filters.py`, so CI and the Docs workflow both run it, and the check refuses a workflow that does not (F4). The
  release-notes commit is docs-only, and so it is the Docs workflow that checks it.
* **D5. After the release.** A `verify-release` job, `needs: release`, runs `tools/publication.py` and `tools/consumer.py`.
  `publication.py` gains `--run-id ID`: the run that is executing stands for "CI on the tag", since `needs` has already required
  its `validate` jobs. The proxy can lag a new tag, so the job retries the proxy checks for up to ten minutes.
* **D6. Hardening.** `check_ci_filters.py` also refuses any job but `release` asking for a permission, and `release` asking for
  more than `contents: write`. No third-party action; `gh` and the tools only.
* **D7. The words.** `RELEASING.md`'s "Tag and Publish": the owner tags; CI creates and verifies the release; the manual
  `gh release create` stays as the fallback for a failed job. `tools/README.md` gains `publish_release.py`, and
  `docs/architecture.md` the job.

`v1.31.1`, tagged before this lands, is released by hand as before. *Amended 2026-11-04*
([0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md), "Re-assessment
at `277aca0`"): `v1.31.2` and `v1.32.0` were released by hand too, with `gh release create VERSION --title VERSION
--notes-file <the body>`, `--verify-tag` left out; F1's figures stay those at `151b6e8`.

### Questions for the owner

* **Q1. Verifying the release in CI (D5).**
  * (a) A `verify-release` job, with retries for the proxy (recommended). Nothing is left to do by hand; a failure shows in the
    run.
  * (b) Keep `publication.py` and `consumer.py` local, run by an agent after the release, as now.
  * (c) The job, but allowed to fail without failing the run.
* **Q2. Checking the notes before the tag (D4).**
  * (a) `make check-release-notes` in both workflows, through `DOC_GATES` (recommended).
  * (b) Only in the `release` job: a broken notes file then shows only after the tag.
  * (c) Only in CI's `validate` job, not in Docs: a docs-only notes commit is then not checked.
* **Q3. A release that already exists with another body (D2).**
  * (a) Fail and change nothing (recommended): a release's text is the owner's to change.
  * (b) Overwrite it with the notes' text.
  * (c) Fail on any existing release, even one with the same body; a rerun then fails.
* **Q4. A tag that is not plain semver, such as `v2.0.0-rc.1`.**
  * (a) No release, with a notice in the run (recommended): no such tag exists, and a prerelease needs its own decision.
  * (b) A release marked prerelease.
  * (c) The job fails.

#### Answered by the owner, 2026-11-01

Each question was put with three options; the owner chose the recommended one each time: Q1 (a), a `verify-release` job;
Q2 (a), the gate in both workflows; Q3 (a), fail and change nothing; Q4 (a), no release and a notice. The decision stays
`proposed` until the owner accepts it.

### Consequences

* Good, because after the tag the release, its checks and the consumer run with no further step.
* Good, because only a tag whose six CI jobs passed, whose notes extract, and which is annotated and on `main` gets a release.
* Good, because one job holds `contents: write`, for one step, with the run's own token, and the filter check refuses any other
  escalation.
* Good, because a broken notes file fails the push that adds it, before the tag (Q2 a).
* Bad, because a release now waits for the slowest CI job, where the owner could release at once: `v1.31.0`'s tag run,
  `41319985865`, took 5 min 28 s, and the two runs on `main` since, 5 min 20 s and 5 min 59 s.
* Bad, because the release's author is `github-actions[bot]`, not the owner.
* Bad, because CI does not run the identifier scan on the body (D3); it relies on the push guard having scanned the pushed text.
* Neutral, because the tag stays manual, and so does a fix when the release job fails: the manual command is the fallback.

### Confirmation

1. `tools/publish_release.py`'s tests, each seen to fail on a planted break: an unannotated tag, a tag off `main`, a missing
   notes file, a missing link target, a prerelease tag, an existing release with another body, and a rerun with the same body.
2. `make check-release-notes` fails on a scratch notes file without its "Release text", and passes on the tree.
3. `check_ci_filters.py` refuses a workflow whose `validate` job asks for `contents: write`, and one where Docs lacks the new gate.
4. `actionlint` and `make check-tools` pass on the changed workflow.
5. `tools/publish_release.py --dry-run v1.31.1 151b6e8`, locally against this repository once `v1.31.1`'s release exists: every
   check holds, and it reports the release as existing with the same body, so it would change nothing. Optionally, on the owner's
   word, a rehearsal on a scratch repository of the owner's, never this one.
6. The first release after this lands is created by the job, and `verify-release` passes.

## Pros and Cons of the Options

### A. A `release` job in `ci.yml`

* Good, because it runs only after the tag's own CI passed, with no extra trigger.
* Good, because the release logic is a tested tool, and the workflow gains one job.
* Bad, because `ci.yml` holds a job with write permission, which the filter check must then guard (D6).

### B. `release.yml` on `workflow_run`

* Good, because `ci.yml` stays read-only throughout.
* Bad, because `workflow_run` runs the default branch's workflow file, not the tag's, and passes the tag only as `head_branch`,
  so the job must establish which tag and commit it serves, and that every job of that run passed.
* Bad, because it is a second workflow for the filter check to cover.

### C. `release.yml` on the tag push, not waiting for CI

* Good, because the release is quick.
* Bad, because a tag whose CI fails gets a release, the case 0020 warns of, unless it repeats CI's checks.

### D. A third-party tool

* Good, because GoReleaser and release-please are widely used.
* Bad, because GoReleaser builds binaries this library does not ship, and release-please writes the version and the changelog
  from commit messages, replacing the release-notes reports the records rely on.
* Bad, because each adds a dependency or an action outside the repository.

### E. `workflow_dispatch` creates the tag and the release

* Good, because one button releases.
* Bad, because a tag pushed with `GITHUB_TOKEN` starts no CI on the tag (F3), and the tag would be the bot's, not the owner's.

## More Information

* Related records: [0020-MADR-retract-accidental-v1-tag.md](0020-MADR-retract-accidental-v1-tag.md);
  [0031-MADR-verification-tools-in-repository.md](0031-MADR-verification-tools-in-repository.md), whose tools the job runs;
  [0037-MADR-align-docs-ci-and-disclosure-gates.md](0037-MADR-align-docs-ci-and-disclosure-gates.md), the filter check and
  `DOC_GATES`.
* No exported API changes, and no release follows from this record itself.

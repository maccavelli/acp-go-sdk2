---
status: proposed
date: 2026-11-04
associated-madr: "0048-MADR-create-releases-in-ci.md"
---

# Implement 0048: CI creates and verifies a tag's release

Associated MADR: [0048-MADR-create-releases-in-ci.md](0048-MADR-create-releases-in-ci.md)

## Goal

After the owner pushes an annotated `vX.Y.Z` tag on `main`, CI creates its GitHub release from the release-notes report once all
six `validate` jobs pass, then checks the publication and runs the consumer, as the MADR's D1 to D7 state. Before the tag, every
push checks that untagged release notes extract. The owner answered on 2026-11-01: Q1 (a) to Q4 (a), the recommended answers.
The notes in italics, on what another answer would change, are kept for a later reader and are not steps.

Base: `151b6e8`. Every file and line named below is at that commit.

## Scope

In scope:

* **New:** `tools/publish_release.py`, and its tests in `tools/test_tools.py`.
* **Changed:**
  * `tools/release_body.py`: `--no-idscan` (D3);
  * `tools/publication.py`: `--run-id ID` and `--proxy-wait SECONDS` (D5);
  * `Makefile`: `check-release-notes` (D4);
  * `scripts/check_ci_filters.py` and its tests: the gate in `DOC_GATES`, and the permissions rule (D6);
  * `.github/workflows/ci.yml`: the gate in the documentation step, and the `release` and `verify-release` jobs;
  * `.github/workflows/docs.yml`: the gate;
  * `RELEASING.md`, `tools/README.md`, `docs/architecture.md`, `docs/README.md`.

Out of scope:

* the tag, which stays the owner's (MADR F3); tag protection or rulesets on GitHub;
* prereleases (Q4 a); release assets; signing;
* any Go file, the exported API;
* `v1.31.1`, released by hand.

## Implementation Steps

Every phase ends staged for the owner, who commits. A phase begins only on the owner's word. Every new check is seen to fail on a
planted break, in a scratch copy, never in the tree.

### Phase 1: the tools and the gate, with no workflow change

1. **`tools/release_body.py --no-idscan`.** Skips the `idscan.py` call (`:77-81`) and prints "idscan: skipped (--no-idscan): the
   body is an extract of pushed text, which the pre-push guard scanned". The exit status then rests on the link targets alone. A
   test: with `--no-idscan` and no deny list, exit 0; without it, exit 1, as today.
   * *Amended 2026-11-04,* before execution, by
     [0049-PLAN-tooling-fixes.md](0049-PLAN-tooling-fixes.md)'s step 1.8 (the MADR's Q19 a): that PLAN's phase 3 makes the scan's
     failure print its reason and status (T11). `--no-idscan` builds on it: the skip is printed in the same place, and a scan
     that runs keeps that phase's report.
2. **`tools/publish_release.py`,** stdlib Python, in the style of the other tools (`_common`, `main_guard`, a docstring with usage
   and exit codes):
   * `publish_release.py VERSION COMMIT [--dry-run] [--no-idscan]`: in order, failing at the first that does not hold, each
     printed as `ok` or `MISS`:
     1. `VERSION` matches `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`; otherwise it prints "not a plain semver tag;
        no release" and exits 0 (Q4 a). *Under Q4 (c):* exit 1. *Under (b):* go on, with `--prerelease`.
     2. `git cat-file -t refs/tags/VERSION` is `tag`, and `refs/tags/VERSION^{commit}` starts with `COMMIT`.
     3. `git merge-base --is-ancestor COMMIT origin/main`, after `git fetch origin main` when `origin/main` is missing.
     4. The body: `release_body.body(root, VERSION, COMMIT, repo)`, no missing link target; written to a temporary file.
     5. `gh release view VERSION --json body`: absent, create with `gh release create VERSION --verify-tag --title VERSION
        --notes-file <file>`; present with the same body (`publication.same_body`), print "exists with this body; nothing to
        do"; present with another body, exit 1, changing nothing (Q3 a). *Under Q3 (b):* `gh release edit`. *Under (c):* exit 1
        on any existing release.
     With `--dry-run`, step 5 prints the command instead of running it.
   * `publish_release.py --check`: for every `docs/reports/*-REPORT-release-notes-vX-Y-Z.md` at `HEAD` whose `vX.Y.Z` is not a
     tag (`git tag -l`), steps 1 and 4 against `HEAD`, with `--no-idscan`'s reasoning. Exit 0 when every one extracts, or none is
     pending, printing what it checked.
     * *Amended 2026-11-04,* before execution
       ([0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md),
       "Re-assessment at `277aca0`"): a report's version is read from its title line, `# Release notes for vX.Y.Z, …`, not
       from its file name. [0019-REPORT-release-notes-v0-18-0.md](../reports/0019-REPORT-release-notes-v0-18-0.md) is
       `v1.14.0`'s, its title says so, and `git tag -l 'v0.18*'` prints nothing, so by its file name `--check` would count it
       pending. Step 3's tests gain that case: a report whose file name and title differ, its title's tag present.
   * `gh` is called with `--repo OWNER/NAME` from `_common.origin_repo`, never inferred.
3. **Tests,** `PublishReleaseTest` in `tools/test_tools.py`, in a temporary repository with a fake `gh` on `PATH`, as
   `WatchCiTest`'s `FakeApi` stands in for the API: a plain semver tag on `main` with notes, created; a lightweight tag; a tag
   whose commit is not on `main`; notes missing; a link target missing; `v2.0.0-rc.1`; a release with the same body, nothing done;
   a release with another body, exit 1 and no edit; `--dry-run`, no `gh release create` called; `--check`, a pending report that
   extracts and one that does not.
4. **`tools/publication.py`.**
   * `--run-id ID`: "CI on the tag" holds when run `ID` is the tag's and is in progress or succeeded, since the job runs only after
     `validate` passed in that run. Without it, as today.
   * *Amended 2026-11-04,* before execution, by [0049-PLAN-tooling-fixes.md](0049-PLAN-tooling-fixes.md)'s step 1.8 (the
     MADR's Q19 a). Its phase 1, `72a4667`, has both CI checks read the CI workflow's runs alone (`ci_runs_command`), and "CI on
     origin main's head" take the head's own run from any ref, or its nearest docs-safe ancestor's (`ci_of_head`). `--run-id`
     builds on them: the run `ID` stands for "CI on the tag"'s answer, and, the release commit being main's head, for
     `ci_of_head`'s too, as the run in progress on the tag's commit. Its tests extend `PublicationTest`.
   * `--proxy-wait SECONDS`: the three proxy checks are retried every 30 s until they hold or the time is spent, printing each
     attempt. Default 0, as today.
   * Tests for both, with the fake API.
5. **`make check-release-notes`:** `python3 -B tools/publish_release.py --check`. `scripts/check_ci_filters.py`'s `DOC_GATES`
   gains it; its tests' fixtures run it in both workflows, and a case without it in Docs fails. *Under Q2 (b):* no gate; *(c):* not
   in `DOC_GATES`, in CI's step alone.
6. **The permissions rule,** `check_ci_filters.py`: in both workflows, no job but `ci.yml`'s `release` has a `permissions` key;
   `release`'s is exactly `contents: write`. Tests: `validate` with `contents: write` fails; `release` with `contents: write,
   packages: write` fails; Docs with any job permission fails.
7. **Seen to fail,** each in a scratch copy of `tools/` or `scripts/` with the tests run there: `publish_release.py` creating on
   another body; accepting a lightweight tag; skipping the `main` check; `--check` passing a report without "Release text";
   `--dry-run` calling `gh release create`; the permissions rule dropped; the gate dropped from `DOC_GATES`.
8. **Checks:** `make check-tools`; `make check-release-notes` on the tree (no pending report once `v1.31.1` is tagged: "0 pending");
   on a scratch copy with a fake `v9.9.9` report, its failure and its pass; the record, markdown and disclosure checks; the
   execution record; staging.

### Phase 2: the workflows and the words

1. **`docs.yml`:** the documentation step runs `make check-release-notes` after `make disclosure-check`. Its checkout gains
   `fetch-depth: 0`, since `--check` needs the tags.
2. **`ci.yml`:**
   * the documentation step (`:153-160`) runs `make check-release-notes`; that job already fetches every tag;
   * `release`: `needs: validate`; `if: github.event_name == 'push' && startsWith(github.ref, 'refs/tags/v')`;
     `runs-on: ubuntu-24.04`; `timeout-minutes: 10`; `permissions: contents: write`; the pinned checkout with `fetch-depth: 0`
     and `persist-credentials: false`; one step, `python3 -B tools/publish_release.py "$GITHUB_REF_NAME" "$GITHUB_SHA"
     --no-idscan`, with `GH_TOKEN: ${{ github.token }}`;
   * `verify-release`: `needs: release`, the same `if`, `runs-on: ubuntu-24.04`, `timeout-minutes: 30`, no `permissions` key;
     the pinned checkout and `setup-go` at `1.27.3`; `tools/release_body.py "$GITHUB_REF_NAME" "$GITHUB_SHA" --no-idscan --out
     "$RUNNER_TEMP"`, then `tools/publication.py "$GITHUB_REF_NAME" "$GITHUB_SHA" "$RUNNER_TEMP/release-body-$GITHUB_REF_NAME.md"
     --run-id "$GITHUB_RUN_ID" --proxy-wait 600`, then `tools/consumer.py "$GITHUB_REF_NAME"`, with `GH_TOKEN` read-only.
     *Under Q1 (b):* no `verify-release`; *(c):* `continue-on-error: true`.
   * A comment on each job names this record.
3. **Lint:** `actionlint` at CI's pin, `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`, exit 0; `make check-tools`.
4. **A dry run:** with `v1.31.1`'s release made by hand, `tools/publish_release.py --dry-run v1.31.1 151b6e8` on this machine:
   every check `ok`, "exists with this body; nothing to do".
5. **The words:**
   * `RELEASING.md`, "Tag and Publish": step 3 says CI creates the release once the tag's CI passes, and checks it; the notes
     must be committed before the tag, and the gate checks them; if the job fails, fix the cause and rerun it, or, for notes
     wrong at the tag, create the release by hand with the old command, which stays as the fallback.
   * `tools/README.md`: a row for `publish_release.py`; `release_body.py`'s and `publication.py`'s rows name their new flags.
   * `docs/architecture.md`: the two jobs and the gate, beside the CI paragraph.
6. **Checks:** `tools/gates.py --all`; the record, markdown and disclosure checks; the execution record; staging. Then, on the
   owner's ask, the push and CI on `main`.

### Phase 3: the first release by the job

On the next release the owner asks for, the agent prepares the notes as now, and the owner pushes the tag; nothing else.

1. `tools/watch_ci.py --branch vX.Y.Z <commit>`: the six `validate` jobs, `release` and `verify-release` pass.
2. The release exists with the notes' body, by `github-actions[bot]`; `verify-release`'s log shows "10 of 10 checks hold" and
   the consumer "as required".
3. A rerun of the `release` job (`gh run rerun <id> --job <release job id>`, on the owner's ask): "exists with this body;
   nothing to do", exit 0.
4. This PLAN and its row `complete`; the MADR's Confirmation 6 recorded.

## Verification

* `PublishReleaseTest`, the `publication.py` additions and the filter check's new cases pass, and each was seen to fail.
* `make check-release-notes` passes on the tree and fails on a scratch report without its section.
* `actionlint`, `make check-tools` and `tools/gates.py --all` pass; CI and Docs pass on the push.
* The dry run on `v1.31.1` holds every check and changes nothing.
* The first release after this lands is created by the job, and `verify-release` passes.

## Rollout and Rollback

* **Rollout.** Phases 1 and 2 change nothing until a tag is pushed. The first tag after phase 2 exercises the job; the owner can
  still create the release by hand if the job fails.
* **Rollback.** Revert phase 2's commit: the workflows lose the jobs and the gate, and releases are made by hand again. Phase 1's
  tools stay usable locally.

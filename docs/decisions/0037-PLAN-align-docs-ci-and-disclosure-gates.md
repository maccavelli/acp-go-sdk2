---
status: in-progress
date: 2026-10-25
associated-madr: "0037-MADR-align-docs-ci-and-disclosure-gates.md"
---
# Implement aligned docs-only CI and disclosure gates

Associated MADR: [0037-MADR-align-docs-ci-and-disclosure-gates.md](0037-MADR-align-docs-ci-and-disclosure-gates.md)

## Goal

A documentation-only push or pull request gets a Docs run with the common Markdown, record/link and portable disclosure gates plus this repository's Markdown-reading checks. Other changes retain full CI. The private pre-push guard remains part of publication.

## Scope

Affected files: `.github/workflows/ci.yml`, `.github/workflows/docs.yml` (new), `Makefile`, `scripts/check_disclosure.py` (new), `scripts/check_ci_filters.py` (new), their tests under `tools/` or `scripts/`, `AGENTS.md`, `docs/architecture.md`, `docs/README.md`.

Repository-specific work: Add a narrow docs-only filter to push and pull_request, excluding root `README.md` and any Markdown read by tests, generated-code checks or maintainer tools. Add `docs.yml` with pinned Markdownlint, `make check-records`, any Markdown-reading checks identified by the audit, and a portable tracked-text disclosure target; run the same disclosure target in full CI. Keep actionlint in full CI for workflow edits; add a filter/gate parity test to `make check-tools`. Preserve schedule and tag behavior and the private-list `make push-guard` before actual pushes. Update `AGENTS.md` and `docs/architecture.md`.

Boundary: Do not filter schedule or `v*` tags. Preserve the Go floor/current matrix in full CI and the private pre-push disclosure guard.

## Implementation Steps

### Phase 1: inventory and shared checks

1. Re-read workflow triggers, job commands, Markdownlint globs, tracked Markdown files and every code or script reader of tracked Markdown. Record the docs-safe path set in this PLAN before editing filters. Include hidden agent-rule trees and test fixtures. Review required-status settings; if a filtered job would stay pending, amend this pair with a concrete migration before changing filters.
2. Add missing record/link and Markdownlint targets, reusing the current commands where they exist. Run identical pinned documentation commands in full CI and Docs. Keep Go lint, tests, vulnerability, builds and releases in full CI.
3. Implement a standard-library portable disclosure check over tracked text. It rejects real macOS, Linux and Windows user-home paths, accepts documented placeholders, prints only file, line and category, and fails on unreadable tracked input. Run the same target in full CI and Docs. Retain the private deny list and global pre-push guard locally.
4. Add fixture tests for new gates. Prove each fails on deliberately broken input in a scratch copy and passes on clean input. Record commands, statuses and redacted diagnostics. Commit the approved phase after checks pass.

### Phase 2: workflow routing and parity

1. Keep push and pull-request CI ignore sets identical and limited to audited docs-safe paths. Make Docs paths cover every ignored path and the Docs workflow itself. Mixed changes run full CI. Preserve tags, schedules, dispatch, release jobs and permissions.
2. Add or extend a Python workflow contract check. It validates actual YAML structure, compares event filters, confirms ignored tracked paths are in Docs, refuses known Markdown inputs to code tests unless their reader runs in Docs, and compares common docs-gate commands and pins. Integrate it in the workflow/tool test target.
3. Run actionlint and the contract check. In independent scratch copies, plant a mismatched path, a Markdown reader wrongly ignored, a missing Docs gate and a differing pin; require each to fail. Commit the approved phase after checks pass.

### Phase 3: documentation and observed rollout

1. Explain routing, docs-safe paths, common checks, local readers and the private pre-push guard in AGENTS.md and the architecture or CI guide. Verify this pair's exact filenames and current statuses in the records index; use the repository index generator if present.
2. Run `make check-records markdownlint check-tools disclosure-check`; actionlint; scratch plants for MD018, a broken record link, a real home path, root README incorrectly ignored, divergent filters and a missing docs gate. Capture status before filtering output. Check this pair and edited documents for real-machine identifiers. Read each edited file in full and verify links.
3. After explicit authorization to push in that same turn, observe a docs-only push, a non-doc or mixed push and the next tag or scheduled run. Record run IDs, conclusions and which workflow started. Mark this PLAN complete only after the criteria below hold; otherwise record the outstanding observation. Commit at the end of each approved phase under repository rules.

## Verification

* Both workflows pass pinned Markdownlint, numbered-record/index and relative-link validation, and portable tracked-text disclosure scanning.
* Each Markdown reader runs on changes to its input, either in Docs or full CI.
* A docs-only push starts Docs but not full CI; a non-doc or mixed push starts full CI. Tags, schedules and dispatch retain existing behavior.
* Scratch plants fail their intended gates; filter or gate drift fails the parity test.
* Before an authorized push, the local pre-push guard checks outgoing commits. Its private deny list is never copied into CI.

## Rollout and Rollback

Land gates before relying on the filters. If routing misclassifies a change, revert the routing commit through a new reviewed commit so full CI runs for every push again. Keep the shared gates while repairing the audit. This PLAN does not authorize a push or tag.

## Execution record

### Approval, 2026-10-25

The owner: "approve 0037 and proceed". The MADR is `accepted`, with `decision-makers` and a dated note; this PLAN is `in-progress`.
The two rows in `docs/README.md` say so, and the blank line that split them from the records table is removed.

### Phase 1, step 1: the inventory, 2026-10-25

Read at `9b20468`, read-only.

**The workflow.** `.github/workflows/ci.yml` is the only workflow. It runs on `push` to `main` and to `v*` tags, on every
`pull_request`, and on a weekly `schedule`. It has no `workflow_dispatch`, no path filter, `permissions: contents: read`, and one
job, `validate`, over `ubuntu-24.04`, `macos-15` and `windows-2025` at Go `1.26.10` and `1.27.3`. Its last step, on the Linux
current-Go job alone, runs `npx --yes markdownlint-cli2@0.23.2`, `actionlint@v1.7.12`, `make check-records` and `make check-tools`.
GitHub does not evaluate path filters for tag pushes, so a filter on `push` leaves `v*` tags running full CI.

**The documentation commands.**

* `make markdownlint` runs `npx --yes markdownlint-cli2@0.23.2`, the same pin as CI. `.markdownlint-cli2.jsonc` lints `**/*.md`
  and excludes records named `*MADR*`, `*PLAN*` and `*[Pp]lan*`.
* `make check-records` runs `python3 scripts/check_records.py --check-all`: record names, pairing and placement, and the relative
  links and anchors of the records, `docs/guides/**`, `docs/README.md`, `docs/architecture.md` and the root `README.md`.
* There is no disclosure target. `tools/idscan.py` and `make push-guard` read the owner's private deny list, which CI does not have.

**Tracked Markdown,** 109 files of 546 tracked: 100 under `docs/`; `README.md`, `AGENTS.md`, `RELEASING.md` and `tools/README.md`;
and one agent-rule file each in `.claude/rules/`, `.codex/rules/`, `.grok/rules/`, `.kilo/agent/` and `.opencode/rules.md`. No
Markdown is tracked under `testdata/` or as a test fixture.

**Code that reads tracked Markdown,** by a scan of every tracked non-Markdown file for a string literal or glob that names a `.md`
file, and of every `os.ReadFile`, `os.ReadDir`, `filepath.Glob` and `go:embed` in Go:

| Reader | Reads | Runs in |
| :--- | :--- | :--- |
| `docs_test.go:164`, `:190` | the root `README.md`'s quickstart and `agentproc` example blocks | full CI, `go test` |
| `Makefile`'s `README.md` and `version` targets, by `mdsh` | the root `README.md`, which they rewrite | by hand, never in CI |
| `scripts/check_records.py` | every Markdown file it checks, above | `make check-records` |
| `markdownlint-cli2` | `**/*.md`, less the records | `make markdownlint` |
| `tools/release_body.py` | a release-notes report, at a commit | by hand, at a release |

No Go test, generator check or tool reads any other tracked Markdown file. The other hits of the scan are record names inside
strings and doc comments, such as `cmd/generate/internal/emit/dispatch.go`'s, which no code opens.

**The docs-safe path set,** each a path that only the Docs gates read:

* `docs/**`;
* `AGENTS.md` and `RELEASING.md`;
* `tools/README.md`;
* `.claude/rules/**`, `.codex/rules/**`, `.grok/rules/**`, `.kilo/agent/**` and `.opencode/rules.md`.

Left out: the root `README.md`, which `docs_test.go` reads; `.markdownlint-cli2.jsonc`, `scripts/**`, the `Makefile`, the
workflows and every other file. A change to any of them runs full CI.

**Required status checks.** `gh api repos/<owner>/acp-go-sdk2/branches/main/protection`: "Branch not protected" (404). The
repository's rulesets and the rules for `main`: `[]` each. No check is required, so a full-CI job a filter skips cannot leave a pull
request pending, and no migration is needed before the filters change.

### Phase 1, steps 2 to 4: the shared checks, 2026-10-25

What changed:

* **`Makefile`:**
  * a new target, `disclosure-check`, runs `python3 -B scripts/check_disclosure.py`;
  * `check-tools` also compiles `scripts/*.py` and runs `scripts/`' tests, `python3 -B -m unittest discover -s scripts -p
    'test_*.py'`;
  * `check-records` and `markdownlint` were already the record/link and Markdownlint targets, and are unchanged.
* **`.github/workflows/ci.yml`:** the documentation step runs `make markdownlint` in place of its own `npx --yes
  markdownlint-cli2@0.23.2`, so the pin lives in the `Makefile` alone, and `make disclosure-check` after `make check-records`. The
  Docs workflow, in phase 2, runs the same targets.
* **`scripts/check_disclosure.py`, new, standard library only.** Over every file `git ls-files` lists, it refuses a macOS
  (`/Users/<name>`), Linux (`/home/<name>`) or Windows (`<drive>:\Users\<name>`, either separator, JSON-escaped or not) home path
  whose account is not a placeholder. A placeholder is an account in angle brackets, `<user>`, or JSON-escaped, `\u003cuser\u003e`;
  one that starts with `$` or `%`; or `user` or `u`, the generic names the tree's examples and tests use. A path inside a longer path
  or a URL, such as `/tmp/home/...`, is not a home path. A tracked file that cannot be read, or is not UTF-8 and holds no NUL byte, is
  "unreadable"; one with a NUL byte is binary, and skipped. Each finding prints as `FILE:LINE: CATEGORY`, never the text found. Exit
  1 on a finding, 2 when `git` fails.
* **`scripts/test_check_disclosure.py`, new:** seven tests, over each form and its line, each placeholder on each form, paths that
  are not home paths, unreadable and binary files, and `main` over a scratch git repository: its output names file, line and
  category and not the account, a clean tree passes, and an untracked file is not read. The test builds each real-looking path from
  split strings, so it passes the check itself.

The first test run failed one subtest: `C:\Users\` followed by a JSON-escaped placeholder was found as `windows-home`, because the
separator pattern took a double backslash first and left `u003cuser` as the account. The pattern now tries one backslash first.

**Checks,** each exit 0:

* `make disclosure-check`: "check-disclosure: 546 tracked file(s), 0 finding(s)";
* `make check-tools`: the tools' "Ran 53 tests", "OK", and the scripts' "Ran 7 tests", "OK";
* `actionlint@v1.7.12` over the workflow.

**Planted breaks,** each in its own scratch copy of the tree with its `.git`, from a script in the session's scratch directory; four
of four failed as required, each with exit 2 from `make`:

| Break | What failed |
| :--- | :--- |
| a Linux home path with a real-looking account added to `docs/architecture.md` | `make disclosure-check`: "docs/architecture.md:610: linux-home" |
| a Windows home path, JSON-escaped, added to `docs_test.go` | `make disclosure-check`: "docs_test.go:302: windows-home" |
| a byte that is not UTF-8 added to `RELEASING.md` | `make disclosure-check`: "RELEASING.md:0: unreadable" |
| `placeholder()` returns true for every account | `make check-tools`: "FAIL: test_main_prints_file_line_category_only" and the other scan tests |

In the three `disclosure-check` breaks, the output did not hold the planted account name. The first run of the script required that of
the fourth break too, and called it not as required: `unittest`'s failure text quotes the test's inputs. That requirement belongs to
the check's own output only, and was dropped for the fourth.

Not checked locally: what the changed CI step does on GitHub, until a push.

Phase 1 was committed by the owner as `c1b86d1`.

### Phase 2: workflow routing and parity, 2026-10-25

What changed:

* **`.github/workflows/ci.yml`:** `push` and `pull_request` each gain the same `paths-ignore`, the docs-safe path set of step 1:
  `docs/**`, `AGENTS.md`, `RELEASING.md`, `tools/README.md`, `.claude/rules/**`, `.codex/rules/**`, `.grok/rules/**`,
  `.kilo/agent/**` and `.opencode/rules.md`. `branches`, `tags: ['v*']`, the `schedule`, the concurrency, the permissions and the
  jobs are unchanged. A comment says that GitHub evaluates no path filter for a tag push.
* **`.github/workflows/docs.yml`, new:** `push` to `main` and `pull_request`, each with `paths` equal to CI's ignored paths and
  `.github/workflows/docs.yml`. One job, `docs`, on `ubuntu-24.04`, `permissions: contents: read`, checks out with CI's
  `actions/checkout` pin and runs `make markdownlint`, `make check-records` and `make disclosure-check`, the targets CI's
  documentation step runs. A change to `docs.yml` itself is not in CI's ignored paths, so it runs full CI as well, with actionlint.
* **`scripts/check_ci_filters.py`, new:** the contract check. It reads both workflows as YAML and checks six parts: `filters`,
  `docs`, `tracked`, `readers`, `gates` and `pins`, as its docstring sets out. `readers` finds the Markdown that Go code reads by the
  string literals of tracked Go files that name a tracked `.md` file, which today finds `README.md` in `docs_test.go`.
* **`scripts/test_check_ci_filters.py`, new:** sixteen tests: the YAML subset and what it refuses, the globs, a clean scratch
  repository, one break per part, the "missing" message, and the repository itself.
* **`Makefile`:** `check-tools` also runs `python3 -B scripts/check_ci_filters.py`, so CI's documentation step runs it.

The step requires the check to read "actual YAML structure". The standard library has no YAML reader, and `AGENTS.md` admits no
dependency that a MADR does not name; this MADR names none. So the check has a reader of its own, for the subset the workflows use,
which refuses anchors, aliases, tags, flow mappings, document markers, duplicate keys and tabs in indentation rather than guessing.
PyYAML 6.0.3, installed on this host, was used once from the session's scratch directory, as an oracle and not a dependency: on
both workflows, the reader's value equalled `yaml.safe_load`'s, with YAML 1.1's `on: true` key read as `on`.

The first run of the new tests failed two of fifteen. The `docs` message listed paths that were not missing: in
`set(ignore) | {DOCS_WORKFLOW} - set(dpush)`, Python binds `-` before `|`. The condition was right; the message is fixed, and a test
pins it. And the tests' helper required every problem of a break to be in one part, where a missing Docs path rightly fails both
`docs` and `tracked`; it now requires the named part among them.

**Checks,** each exit 0:

* `python3 -B scripts/check_ci_filters.py`: "check-ci-filters: 0 problem(s)";
* `make check-tools`: the tools' "Ran 53 tests", "OK"; the scripts' "Ran 23 tests", "OK"; and the contract, "0 problem(s)";
* `actionlint@v1.7.12` over both workflows;
* `check_disclosure.scan` over the three new files: no finding.

**Planted breaks,** on the real workflows, each in its own scratch copy of the tree, from a script in the session's scratch
directory; four of four failed `make check-tools` as required, with exit 2:

| Break | What failed |
| :--- | :--- |
| CI's `pull_request` list loses `RELEASING.md` | "filters: CI's push and pull_request paths-ignore differ, or are empty" |
| the root `README.md` added to CI's ignored paths and to Docs' paths | "readers: CI ignores README.md, which Go code reads" |
| Docs no longer runs `make disclosure-check` | "gates: Docs does not run make disclosure-check" |
| Docs checks out with another `actions/checkout` pin | "pins: actions/checkout differs, CI [...], Docs [...]" |

Each was found by `RepositoryTest`, the scripts' test of this repository, before the target's own run of the check.

Not checked locally: that GitHub starts Docs alone for a docs-only push, both for a mixed one, and full CI for a tag. Phase 3, step 3
observes them after an authorized push.


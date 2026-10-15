---
status: complete
date: 2026-10-14
associated-madr: "0020-MADR-retract-accidental-v1-tag.md"
---

# Implement the `v1.14.0` release and the retraction of `v1.13.0`

Associated MADR: [0020-MADR-retract-accidental-v1-tag.md](0020-MADR-retract-accidental-v1-tag.md)

## Goal

`go get github.com/maccavelli/acp-go-sdk2@latest` resolves to `v1.14.0`, phases 1 and 2 of 0019, and `go.mod` retracts `v1.13.0`.

## Scope

In scope: the root `go.mod`'s `retract` directive; the release notes in
[0019-REPORT-release-notes-v0-18-0.md](../reports/0019-REPORT-release-notes-v0-18-0.md), which keeps the name it is cited by; these
records, the 0019 MADR's amendment of decision 12 and the 0019 phase 2 PLAN's deviation entry, with their `docs/README.md` rows; and
the close of [0019-PLAN-api-decisions.md](0019-PLAN-api-decisions.md), which waits on the tag. Out of scope: `cmd/generate/go.mod`, a
separate module never tagged; any code.

## Implementation Steps

Agents do not commit, push or tag. At each commit point the agent runs the checks and stops; the owner commits, pushes and tags.

1. **Records.** This pair; the 0019 MADR's amendment of decision 12; the 0019 phase 2 PLAN's deviation entry; the `docs/README.md`
   rows. Checks: `make check-records`, `make markdownlint`, the identifier scan.
2. **The directive and the notes.** The root `go.mod` gains, after the `go` line, `retract v1.13.0 // Published accidentally from an
   unfinished commit.` The release notes become `v1.14.0`'s: the title and the release text's heading, the version's source (this
   MADR), and a "Versions" paragraph: the first `v1` release, and `v1.13.0`, a tag pushed by accident at an unfinished commit,
   retracted. Checks: `go mod edit -json` shows the retraction; `make pre-add-check` (it runs `go mod tidy -diff`);
   `GOEXPERIMENT=nojsonv2 go test ./...`; Windows `go test -count=1 -shuffle=on ./...` in both modules; `make check-records`, `make
   markdownlint`, the identifier scan; the API listing against `v0.17.1` unchanged from step 11 of the 0019 phase 2 PLAN. The probe
   of the MADR's "Confirmation", with the committed `go.mod` as `v1.14.0`'s `.mod`: `@latest` is `v1.14.0`, `-versions` leaves out
   `v1.13.0`, and `v1.13.0` pinned reports the retraction; seen to fail with the directive removed in a scratch copy of the
   `go.mod`: `v1.13.0` listed, `retracted=[]`.
3. **Publication** (owner). Push `main`; when CI passes on it, tag that commit `v1.14.0`, push the tag, and publish the GitHub
   release from the report's release text.
4. **Verification and close.** From a scratch consumer, default proxy, scratch `GOMODCACHE` and `GOPATH`: `go list -m
   github.com/maccavelli/acp-go-sdk2@latest` gives `v1.14.0`; `go list -m -versions` does not list `v1.13.0`; the proxy's
   `@v/v1.14.0.mod` holds the directive. Then this PLAN, the 0019 phase 2 PLAN and their rows become `complete`.

## Verification

Acceptance: the directive is committed and the checks of step 2 pass; `v1.14.0` is tagged on a commit whose CI passed and its
release published; and step 4's `go list` results hold against the public proxy.

## Rollout and Rollback

Once `v1.14.0` is tagged and fetched, the proxy keeps it, and its retraction holds while it is the highest version; it can be
superseded only by a higher version's `go.mod`, which keeps the directive. Before step 3 the change is one commit, reverted by
reverting it.

## Execution record

### Step 1, records, 2026-10-14

This pair, the 0019 MADR's amendment of decision 12, the 0019 phase 2 PLAN's deviation entry, and the `docs/README.md` rows;
`make check-records`, `make markdownlint` and the identifier scan exit 0. The owner committed and pushed them as `3bef7cd`. The
owner's choice of 2026-10-14, the directive added by the agent, approved step 2.

### Step 2, the directive and the notes, 2026-10-14

* `go.mod`: `retract v1.13.0 // Published accidentally from an unfinished commit.` after the `go` line. `go mod edit -json` shows
  `"Retract": [{"Low": "v1.13.0", "High": "v1.13.0", "Rationale": "Published accidentally from an unfinished commit."}]`.
* [0019-REPORT-release-notes-v0-18-0.md](../reports/0019-REPORT-release-notes-v0-18-0.md): the title, the header and the release
  text's heading name `v1.14.0`; the header says the report was drafted for `v0.18.0` and keeps its file name; the release text
  gains "Versions": the first `v1` release, the `/v2` rule for a later incompatible change, and `v1.13.0` retracted. Its
  `docs/README.md` row follows the title.

The probe of the MADR's "Confirmation", a scratch file proxy for `github.com/maccavelli/acp-go-sdk2` holding `v0.17.1`, `v1.13.0`
and a `v1.14.0` whose `.mod` is the committed `go.mod`: `@latest` is `v1.14.0`; `-versions` lists `v0.17.1 v1.14.0`; `v1.13.0`
pinned reports `retracted=[Published accidentally from an unfinished commit.]`, update `v1.14.0`. Seen to fail with the directive
removed in a scratch copy of the `go.mod`: `-versions` lists `v0.17.1 v1.13.0 v1.14.0`, and `v1.13.0` pinned reports
`retracted=[]`.

Gates, through the Python runner: regenerating changes no generated file; `make pre-add-check` exit 0, `go mod tidy -diff` among
its checks; no new `.go` file; `GOEXPERIMENT=nojsonv2 go test ./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make
check-records` and `make markdownlint` exit 0; the identifier scan finds nothing; the API listing against `v0.17.1` unchanged from
the 0019 phase 2 PLAN's step 11. Windows: `go test -count=1 -shuffle=on ./...` exit 0 at the root and in `cmd/generate`.

Deviation, 2026-10-14, before step 3: CI passed on `c1b1185` (run `38565152865`), and the owner, before tagging, asked for the two
open issues, #4 and #5, to be resolved first. They are decided in
[0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md) and implemented by
[0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md](0021-PLAN-resolve-issues-4-and-5-before-v1-14-0.md). Step 3 waits on that PLAN:
`v1.14.0` is tagged on the commit it verifies, not on `c1b1185`. The `retract` directive and the release notes of step 2 stand; the
0021 PLAN adds to the notes.

### Step 3, publication, 2026-10-14

The 0021 PLAN closed on `e24efbb`, and CI run `38672326828` on it concluded `success` on `validate (ubuntu-24.04)`, `validate
(macos-15)` and `validate (windows-2025)`. The owner tagged `e24efbb` `v1.14.0`, an annotated tag as `v0.17.1` is, pushed the tag,
and published the GitHub release `v1.14.0`, marked latest, from the release text of
[0019-REPORT-release-notes-v0-18-0.md](../reports/0019-REPORT-release-notes-v0-18-0.md) as the 0021 PLAN amended it, extracted
without its two headings (130 lines; the identifier scan finds nothing in it).

### Step 4, verification and close, 2026-10-14

From a scratch consumer, `GOPROXY` the default `https://proxy.golang.org,direct`, a scratch `GOMODCACHE` and `GOPATH`, checksum
database on:

* the proxy's `@v/v1.14.0.info` answers `200`, its origin `refs/tags/v1.14.0` at `e24efbb`;
* its `@v/v1.14.0.mod` answers `200` and holds `retract v1.13.0 // Published accidentally from an unfinished commit.`;
* `go list -m github.com/maccavelli/acp-go-sdk2@latest` gives `v1.14.0`;
* `go list -m -versions` lists `v0.14.0 v0.15.0 v0.16.0 v0.17.0 v0.17.1 v1.14.0`, without `v1.13.0`;
* `v1.13.0` pinned, `-u -retracted`, reports `retracted=[Published accidentally from an unfinished commit.]` and the update
  `v1.14.0`.

The check, a Python script outside the tree, was seen to fail on a planted break in a scratch copy of it: expecting `retract v1.13.1`,
it reported that the result does not hold and exited 1.

Every acceptance criterion in "Verification" is met, with the deviation before step 3 recorded above, so this PLAN and its
`docs/README.md` row are `complete`, and so are the 0019 phase 2 PLAN and its row, whose close followed this step. The MADR stays
`accepted`. Issues #4 and #5, which `v1.14.0` resolves, are left to the owner to close.

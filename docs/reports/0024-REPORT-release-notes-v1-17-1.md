# Release notes for v1.17.1, against v1.17.0

Observed 2026-10-20 on `0a9b5c1`, for [0024-PLAN-api-gate-and-deprecation.md](../decisions/0024-PLAN-api-gate-and-deprecation.md),
step 6. This report drafts the text of the GitHub release for `v1.17.1`, as `RELEASING.md` describes it. It decides nothing.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `0a9b5c1` with nothing uncommitted. These are the first notes to take their API facts from it, as
  `RELEASING.md` says since `803c77b`.

Bring it up to date if more changes land before the tag.

- No exported API change, and no change to the package's code: the root's non-test `.go` files, `go.mod` and `README.md` are the
  same as at `v1.17.0` (`git diff --quiet v1.17.0 0a9b5c1` exits 0 for them).
- The release is proposed as the patch `v1.17.1`. `make api TAG=v1.17.1` exits 0: "v1.17.1 is a valid semantic version for this
  release". The owner confirms it at the tag.
- What a consumer may meet: nothing. The changes are to the generator, CI, the Makefile and the release procedure.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.17.1

The ACP schema is still `1.25.0` (`git diff v1.17.0 -- schema/` is empty). The exported API and the package's code are unchanged:
`make api` reports no change against `v1.17.0`. This release is phase 4 of
[0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](../decisions/0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md):
an API-compatibility gate, and the generator's path to `Deprecated:`.

#### For maintainers

- **An API-compatibility gate.** (`803c77b`.)
  - CI's new step "API compatibility (gorelease)" runs `gorelease`, pinned at
    `golang.org/x/exp@v0.0.0-20261014143641-f0251eb4aa3e`, on each push to `main`, each pull request and the weekly run, on Linux
    with Go 1.27.2. It compares the exported API with the latest release. A change it calls incompatible, such as a removed
    declaration or a method added to an interface, fails the job.
  - `make api` runs the same check on the last commit, and `make api TAG=vX.Y.Z` also checks a proposed tag. It needs nothing
    uncommitted, and the network.
  - `RELEASING.md`: `make api TAG=vX.Y.Z` before tagging, each release's notes record its report, and tags are annotated
    (`git tag -a vX.Y.Z -m vX.Y.Z`).
- **The generator models the schema's `deprecated` keyword.** (`45fa905`, `61ac81e`.)
  - Before, a schema that marked anything deprecated stopped `make version`: "unknown field "deprecated"".
  - Each Go declaration built from a marked node gets the paragraph "Deprecated: the ACP schema marks <place> deprecated.":
    - a definition's type, and for a request or notification, its interface method and its side connection's method;
    - a property's field, and an inline object property's nested type;
    - a union variant's field, inline type and helper constructor, or a constant variant's constant;
    - a flattened member's field.
  - staticcheck's SA1019 reports the notice to callers in other packages.
  - The protocol table can add a note naming the replacement.
  - Refused: a `deprecated` that is not `true` or `false`, `null` included; and one on a node no Go declaration is built from.
  - No published ACP schema marks anything deprecated today, so no generated file changes.
- **Hand-written notices** are held to the paragraph form staticcheck honours by `TestDeprecationNotices`. (`0a9b5c1`.)
- **New tests:**
  - in the generator: `TestDeprecationNotes`, `TestWithNotice`, `TestDeprecations`, the engine case `accept-deprecated`, and five
    refuse cases;
  - engine cases now compare a `want.deprecated` section, so a case without one fails on any notice;
  - at the root, `TestDeprecationNotices`.

  (`45fa905`, `61ac81e`, `0a9b5c1`.)

## Sources not in the release text

- **Commits since `v1.17.0`** (`git log v1.17.0..0a9b5c1`), eight:
  - `803c77b` changes `ci.yml`, the `Makefile`, `RELEASING.md` and `docs/architecture.md`;
  - `45fa905` changes the generator's loader, registry and placement check, with four refuse cases;
  - `61ac81e` changes the generator's model, emitter and harness, with a case and tests;
  - `0a9b5c1` adds `deprecation_test.go`;
  - `b463216`, `f6f9d73`, `d078835` and `1545949` change records only (`git show --name-only`).
- **Go files changed.** `git diff --name-only v1.17.0 0a9b5c1 -- '*.go' ':!*_test.go'` lists eleven, all in `cmd/generate`. Outside
  `cmd/generate` and `docs/`, the files that change are `.github/workflows/ci.yml`, `Makefile`, `RELEASING.md` and
  `deprecation_test.go`.
- **`make api` on `0a9b5c1`:** exit 0, "Inferred base version: v1.17.0", "Suggested version: v1.17.1". With `TAG=v1.17.1`: exit 0,
  "v1.17.1 is a valid semantic version for this release".
- **The evidence:** [0024-PLAN-api-gate-and-deprecation.md](../decisions/0024-PLAN-api-gate-and-deprecation.md). Its steps 2 to 5
  hold the planted breaks (11 under each of GNU Make 3.81 and 4.4.1, then 4, 13 and 7), and the gates on go1.27.2 and go1.26.9. Its
  deviations during steps 4 and 5 are in the same record.
- **Not in this release:** schema `1.26.0` and the record on the unstable API that it needs. From this release on, that update fails
  the gate until the record decides (the MADR's R10).

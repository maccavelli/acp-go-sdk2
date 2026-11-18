# Releasing

This project follows the ACP schema version published by
[`agentclientprotocol/agent-client-protocol`](https://github.com/agentclientprotocol/agent-client-protocol).
Each Go release should align with a specific schema tag so that the generated
code, helper APIs, and library version remain in sync.

## Prerequisites

- GNU make 4.2 or newer, `curl`, and `git` in your `PATH`. The Makefile reads
  `schema/version` with `$(file)`, which Make 3.81 does not implement; under
  3.81, `make version`, `make release` and the schema downloads stop with
  "schema/version read as empty", and the docs gates still run. Homebrew
  installs this binary as `gmake`.
- Go at the version in the `go` line of `go.mod`.
- `gofmt`, `golangci-lint` v2.15.0, and `make pre-add-check`.
- `mdsh`, which `make version` runs to rewrite `README.md` (`make version
  MDSH=path` names another binary).

## Bump the Schema Version

1. Decide which upstream ACP schema version to adopt (for example `1.25.0`).
2. Update `schema/version` and regenerate code. There are two supported ways to
   do this:

### Option A: Use the release helper

```bash
make release VERSION=1.25.0
```

The helper performs the following steps:

- writes the requested number to `schema/version`
- runs `make version` to download the new schema files and regenerate Go code
- runs `make fmt`, `make test`, and `make pre-add-check`
- asserts that `schema/version` and `version` now match

If any command fails, fix the issue and rerun the helper. The target does not
create commits or tags; it just prepares the tree.

### Option B: Run the steps manually

```bash
printf '1.25.0\n' > schema/version
make version
make fmt
GOCACHE=$(pwd)/.gocache make test
make pre-add-check
cmp -s schema/version version
```

`make version` downloads the schema files for tag `schema-v` plus the bare number
in `schema/version`, regenerates all Go code, and formats the repository with
`gofmt`. A stamp of `1.25.0` downloads `schema-v1.25.0`. The `cmp` command is a
lightweight guard that ensures both `schema/version` and the top-level `version`
file agree before you publish. The `vX.Y.Z` commands below are the Go module tag,
not the schema download tag.

The generator type-checks its output with the hand-written files before it
writes any file. When a new schema renames or removes a type the hand-written
code uses, `make version` stops with the type checker's errors and changes no
generated file. Change the hand-written code to the new names first, then rerun
`make version`. Generated files are written with mode `0644`, and a file whose
content did not change is not rewritten.

### After regenerating

The conformance test at the root builds instances of every definition from the
new schema files. It can fail on a construct the new schema introduces that
its instance builder (`instances_test.go`) does not know, or on a generated type
that does not behave as the schema says. Extend the builder for a new
construct; put a behaviour the decision records do not decide to the owner
before changing the generator.

`make fuzz` and `make mutation-engine` / `make mutation-generated` are
optional before a release. They report; neither gates.

A schema bump never rewrites `schema/unstable-v1.json`. `make version`
downloads the four schema files and not the manifest, and
`-write-unstable-manifest` refuses to replace it, so `acp`'s unstable-origin
API stays as it was frozen. What the new unstable schema adds goes to package
`unstable`. After the bump, `make api` lists `unstable`'s changes under their
own heading, and they do not gate. Read that report and record it in the
bump's record, with what `unstable` gained and lost
(0039-PLAN-five-adopter-additions-on-schema-1-25-0.md, 2f).

## Review and Commit

1. Inspect the changes: `git status` and `git diff` should show updated schema
   files, generated Go code, and the version files.
2. Commit with `git commit --no-edit`. The repository hook writes the message.
3. Push the branch to GitHub and open a pull request if review is required.

## Tag and Publish

1. Check the exported API against the latest release, on the release commit.
   It must exit 0:

   ```bash
   make api
   ```

   `make api` runs `apidiff`, through `scripts/check_api.py`, on the root
   package, `agentproc`, `acptest` and `clienthost` against the newest `v1`
   tag, a package the tag lacks against an empty one. It fails on an
   incompatible change, and lists the compatible ones: when there are any, the
   release is a minor (`vX.Y+1.0`), otherwise a patch. It compares `unstable`
   and `remote` too, lists their incompatible changes under their own heading
   without failing, and counts their compatible ones with the rest. It needs
   the tags.

2. Tag the release commit with an annotated, Go-compatible tag whose message
   is its name:

   ```bash
   git tag -a vX.Y.Z -m vX.Y.Z
   git push origin vX.Y.Z
   ```

3. Create a GitHub release for the tag, named for the tag, neither a draft nor a
   prerelease, and the latest release. Its body is the "Release text" of the
   release's notes report (see "Release notes and the API"), which includes a
   summary of notable changes, the upstream ACP schema version, and any change to
   what the SDK sends or accepts on the wire, as the decision records under
   `docs/decisions/` describe it. Extract the body with:

   ```bash
   python3 tools/release_body.py vX.Y.Z REV
   ```

   `REV` is the release commit. The tool reads the report from git as at `REV`,
   so the report must be committed. It takes the text under the report's
   `### vX.Y.Z` heading, up to its `## Sources not in the release text` heading,
   makes each relative link absolute at the tag, and scans the body for
   identifiers. It writes `release-body-vX.Y.Z.md` to `.cache/tools/release/`,
   or to the directory `--out DIR` names. It exits 1 when a link target is
   missing at `REV` or the scan matches, and 0 otherwise.

4. After the tag is pushed, check the release. CI does not run these; each is run
   by hand, and each is described in `tools/README.md`:

   ```bash
   python3 tools/watch_ci.py REV
   python3 tools/publication.py vX.Y.Z REV BODY
   python3 tools/consumer.py vX.Y.Z
   ```

   - `watch_ci.py` waits for every CI run of the commit `REV` (up to 20 minutes
     for the first, then 30 seconds for others, such as the tag's), watches them
     to their end, and prints each run and job. It exits 0 only when every run
     and job succeeded (a skipped job is allowed), 1 when one did not or did not
     settle in time, and 2 when no run appeared. `--branch B` limits it to that
     branch's runs; by default it watches every run of the commit.
   - `publication.py` runs ten checks of the publication on GitHub and the Go
     module proxy. `REV` is the commit the tag must name, and `BODY` is the
     file `release_body.py` wrote: the published release's body must equal it. It
     exits 0 only when every check holds.
   - `consumer.py` builds a fresh module at the `go` line's version that runs
     `go get vX.Y.Z` from the proxy with the checksum database on, then builds,
     runs and tests a program that uses the release. It exits 0 when everything
     is as required. `vX.Y.Z` must be `v1.36.0` or later.

   `watch_ci.py` and `publication.py` read GitHub through `gh`.

Consumers rely on the `vX.Y.Z` semver tag for `go get`, so ensure the tag is
pushed before announcing the release.

## Release notes and the API

- `CHANGELOG.md` is the changelog. Each change adds its line under
  `## Unreleased` in the same phase as the change. The release's PLAN turns
  `Unreleased` into the version and its date, `## vX.Y.Z — YYYY-MM-DD`, and
  links the release-notes report from that section. Releases before `v1.36.0`
  are not listed there; a closing paragraph points to their reports.
- Each release has a release-notes report,
  `docs/reports/NNNN-REPORT-release-notes-vX-Y-Z.md`, numbered with
  `python3 scripts/check_records.py --next` and indexed in `docs/README.md`. It
  keeps its number and its index row. Its "Release text" section is the GitHub
  release's body, extracted as "Tag and Publish" step 3 says.
- Each release's notes record what `make api` reports on the release commit
  against the last tag: its compatible and incompatible changes.
- CI runs the same check on every push and pull request, so an incompatible
  change fails before a release is prepared.
- An incompatible change to the exported API needs a new major version, `/v2`
  ([0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](docs/decisions/0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md)).

## Additional Notes

- If the new schema introduces breaking changes, update examples and docs in
  the same commit.
- A change to code names the docs it makes false or newly needs: `README.md`,
  `docs/architecture.md`, `RELEASING.md`, `tools/README.md`, a package's
  `doc.go`, the changed names' doc comments, and `CHANGELOG.md`'s `Unreleased`.
  It changes them in the same commit, and its record says which docs changed,
  or that none needed to.
- The helper uses a repository-local Go build cache (`.gocache`) to avoid
  sandbox restrictions in CI and local development. You can delete it with
  `rm -rf .gocache` if needed.
- `make clean` removes the downloaded schema files and the `version` file; rerun
  the release steps afterwards if you invoke it.

# Releasing

This project follows the ACP schema version published by
[`agentclientprotocol/agent-client-protocol`](https://github.com/agentclientprotocol/agent-client-protocol).
Each Go release should align with a specific schema tag so that the generated
code, helper APIs, and library version remain in sync.

## Prerequisites

- GNU make 4.2 or newer, `curl`, and `git` in your `PATH`. The Makefile reads `schema/version` with `$(file)`, which Make 3.81 does not implement. Homebrew installs this binary as `gmake`.
- Go at the version in the `go` line of `go.mod`.
- `gofmt`, `golangci-lint` v2.14.0, and `make pre-add-check`.

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

## Review and Commit

1. Inspect the changes: `git status` and `git diff` should show updated schema
   files, generated Go code, and the version files.
2. Commit with `git commit --no-edit`. The repository hook writes the message.
3. Push the branch to GitHub and open a pull request if review is required.

## Tag and Publish

1. Check the proposed tag against the latest release, on the release commit
   with nothing uncommitted. It must exit 0:

   ```bash
   make api TAG=vX.Y.Z
   ```

   `make api` runs `gorelease` at the version CI pins. It fails on a change
   to the exported API it calls incompatible, and on a tag that does not
   follow from the changes, such as a patch that adds API. It needs the
   network.

2. Tag the release commit with an annotated, Go-compatible tag whose message
   is its name:

   ```bash
   git tag -a vX.Y.Z -m vX.Y.Z
   git push origin vX.Y.Z
   ```

3. Create a GitHub release for the tag. Include a summary of notable changes and
   reference the upstream ACP schema version. Call out any change to what the SDK
   sends or accepts on the wire, as the decision records under `docs/decisions/`
   describe it.

Consumers rely on the `vX.Y.Z` semver tag for `go get`, so ensure the tag is
pushed before announcing the release.

## Release notes and the API

- The changelog is the release notes report,
  `docs/reports/NNNN-REPORT-release-notes-vX-Y-Z.md`, numbered with
  `python3 scripts/check_records.py --next` and indexed in `docs/README.md`.
  Its "Release text" section is the GitHub release's body.
- Each release's notes record what `make api` reports on the release commit
  against the last tag: its compatible and incompatible changes, and the
  version it suggests.
- CI runs the same check on every push and pull request, so an incompatible
  change fails before a release is prepared.
- An incompatible change to the exported API needs a new major version, `/v2`
  ([0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](docs/decisions/0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md)).

## Additional Notes

- If the new schema introduces breaking changes, update examples and docs in
  the same commit.
- The helper uses a repository-local Go build cache (`.gocache`) to avoid
  sandbox restrictions in CI and local development. You can delete it with
  `rm -rf .gocache` if needed.
- `make clean` removes the downloaded schema files and the `version` file; rerun
  the release steps afterwards if you invoke it.

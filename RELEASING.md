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

## Review and Commit

1. Inspect the changes: `git status` and `git diff` should show updated schema
   files, generated Go code, and the version files.
2. Commit with `git commit --no-edit`. The repository hook writes the message.
3. Push the branch to GitHub and open a pull request if review is required.

## Tag and Publish

1. Tag the release commit with a Go-compatible tag:

   ```bash
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```

2. Create a GitHub release for the tag. Include a summary of notable changes and
   reference the upstream ACP schema version. Call out any change to what the SDK
   sends or accepts on the wire, as the decision records under `docs/decisions/`
   describe it.

Consumers rely on the `vX.Y.Z` semver tag for `go get`, so ensure the tag is
pushed before announcing the release.

## Additional Notes

- If the new schema introduces breaking changes, update examples and docs in
  the same commit.
- The helper uses a repository-local Go build cache (`.gocache`) to avoid
  sandbox restrictions in CI and local development. You can delete it with
  `rm -rf .gocache` if needed.
- `make clean` removes the downloaded schema files and the `version` file; rerun
  the release steps afterwards if you invoke it.

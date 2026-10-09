# Releasing

This project follows the ACP schema version published by
[`agentclientprotocol/agent-client-protocol`](https://github.com/agentclientprotocol/agent-client-protocol).
Each Go release should align with a specific schema tag so that the generated
code, helper APIs, and library version remain in sync.

## Prerequisites

- `make`, `curl`, and `git` in your `PATH`.
- Go at the version in the `go` line of `go.mod`.
- `gofmt`, `golangci-lint` v2.14.0, and `make pre-add-check`.

## Bump the Schema Version

1. Decide which upstream ACP schema tag to adopt (for example `v0.4.3`).
2. Update `schema/version` and regenerate code. There are two supported ways to
   do this:

### Option A: Use the release helper

```bash
make release VERSION=0.4.3
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
printf '0.4.3\n' > schema/version
make version
make fmt
GOCACHE=$(pwd)/.gocache make test
make pre-add-check
cmp -s schema/version version
```

`make version` downloads the schema files for the requested ACP tag, regenerates
all Go code, and formats the repository with `gofmt`. The `cmp` command is a
lightweight guard that ensures both `schema/version` and the top-level `version`
file agree before you publish.

## Review and Commit

1. Inspect the changes: `git status` and `git diff` should show updated schema
   files, generated Go code, and the version files.
2. Commit with `git commit --no-edit`. The repository hook writes the message.
3. Push the branch to GitHub and open a pull request if review is required.

## Tag and Publish

1. Tag the release commit with a Go-compatible tag:

   ```bash
   git tag v0.4.3
   git push origin v0.4.3
   ```

2. Create a GitHub release for the tag. Include a summary of notable changes and
   reference the upstream ACP schema version.

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

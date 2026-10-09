---
status: complete
date: 2026-10-09
associated-madr: "0003-MADR-install-fleet-agent-workspace.md"
---

# Implement Install the fleet workspace and library CI

Associated MADR: [0003-MADR-install-fleet-agent-workspace.md](0003-MADR-install-fleet-agent-workspace.md)

## Goal

Install the fleet agent workspace and replace this repository's CI, formatting, and agent guidelines with the shared library gates. Leave schema generation in place.

## Scope

In scope:

* Fleet file set from the workspace-scaffolding templates: `.gitattributes` plus a provenance comment, merged `.gitignore`, `Makefile` shell snippet, `.markdownlint-cli2.jsonc`, `scripts/check_records.py`, `docs/architecture.md`, five harness pointers, both pointer gitignores, `opencode.json`
* `AGENTS.md` replaced by the template and the slots in this plan. The current "Repository Guidelines" body is not appended
* `.golangci.yml` from the shared linter set, with the named deltas below
* `scripts/go-precheck.sh` adapted from the library pre-add script, with the named deltas below
* `Makefile` library targets, keeping schema download, `version`, and `release`
* `.github/workflows/ci.yml` as specified below
* Removal of `.github/workflows/ci.yaml`, `.github/workflows/release-notes.yaml`, `mise.toml`, and `treefmt.toml`
* `RELEASING.md` toolchain paragraphs updated so they no longer name mise, treefmt, or `make check`
* Line wrap of prose longer than 200 characters in `docs/reports/0001-REPORT-go-127-acp-parity.md`
* Hand-written Go fixes required by the new linter, stopping for an amendment when a finding is a design change
* Scratch-copy plants, then one commit with `git commit --no-edit`

Out of scope:

* Executing the schema pin or editing the `go` line. That remains [0002-PLAN-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-PLAN-pin-go-1-27-2-and-acp-schema-1-24-1.md). Do not recreate `mise.toml`
* Release build or publish workflows, installer checks, fuzz, API diff, coverage floors, parity maps, and import bans copied from either sibling
* `docs/guides/`
* Replacing the product root `README.md` except where markdownlint requires a wrap. The mdsh comment in the install section stays; nothing in CI runs mdsh
* `.editorconfig`, Dependabot, `SECURITY.md`, `CODEOWNERS`, `CONTRIBUTING.md`, `CHANGELOG.md`, `.claude/settings.json`, and `CLAUDE.md`
* Editing a byte-copied file, or disabling a linter, to silence a failure
* Pushing or tagging

## Slots

`{{SCOPE}}`:

This repository is the Go SDK for the Agent Client Protocol, module `github.com/coder/acp-go-sdk`. How the tree is put together now is documented in `docs/architecture.md`. The documentation index is `docs/README.md`. The language floor is the `go` line in `go.mod` and `cmd/generate/go.mod`.

`{{DEPENDENCIES}}`:

No dependency may be added without a MADR in this repository that names it. Lockfiles change in the same commit as the first import, and are cleaned in the commit that removes the last. `go mod tidy -diff` is clean at every commit.

`{{PREADD}}`:

- Before staging Go files, run `make pre-add-check`. It runs `scripts/go-precheck.sh`: `gofmt`; `golangci-lint` with `.golangci.yml` for the host and for `GOOS=windows`, at the root and in `cmd/generate`; `go vet`; `go test -race`; `go mod tidy -diff`; and `govulncheck` at the version pinned in `.github/workflows/ci.yml`. `GO_PRECHECK_SKIP_VULN=1` skips the vulnerability check offline. A file that fails is not committed.
- Before every commit, run `make check-records` and `make markdownlint`.
- A file that fails is not committed.

`docs/architecture.md` `{{WHAT_IT_IS}}`:

This repository is the Go module `github.com/coder/acp-go-sdk`. Hand-written session and transport code sits at the repository root. `cmd/generate` reads `schema/` and writes the `*_gen.go` files. `example/` holds agent and client programs. `testdata/` holds fixtures. GitHub Actions runs `.github/workflows/ci.yml`. The schema stamp lives in `schema/version`. The module `go` line lives in `go.mod` and `cmd/generate/go.mod`.

`{{WHAT_IS_NOT_HERE}}`:

`docs/guides/` does not exist. The first guide creates it. The Go floor and the schema pin live in the decision records indexed from `docs/README.md`.

## Named deltas

* `.gitattributes`: one provenance comment naming `docs/decisions/0003-MADR-install-fleet-agent-workspace.md`. No `-text` line. A scan on 2026-10-09 found no CRLF in the Go, Markdown, JSON, YAML, TOML, module, Makefile, or `version` files.
* `.golangci.yml`: same linters, settings, formatter list, and test exclusions as `go-llmprovider-sdk`. Drop comments that cite another repository's record ids. Do not copy `depguard` or any import allow-list. ~~Add path exclusion `example$` beside `examples$`, because this tree's example programs live in `example/`.~~ The path is `^example/`. See the deviation of 2026-10-09.
* `scripts/go-precheck.sh`: same stages as the library script. Do not pass `--build-tags live_gateways`. Lint and test the root module and `cmd/generate`. Comments name this plan, not another repository's records.
* `Makefile` `release` calls `fmt`, `test`, and `pre-add-check` in place of `make check`. Schema recipes stay, including `gofumpt` removal: `make version` runs the generator and then `gofmt -w .` at the root and in `cmd/generate`.
* `.gitignore`: drop the `mise.local.toml` and `.mise.local.toml` lines. Keep the MacOS and Go cache lines. Append the skill fragment's missing entries.

## Workflow

Write `.github/workflows/ci.yml` with this behavior, and delete `.github/workflows/ci.yaml` and `.github/workflows/release-notes.yaml`.

* Name `CI`. On pull requests, on pushes to `main`, on tags `v*`, and on cron `17 6 * * 1`.
* `permissions: contents: read`.
* Concurrency group `ci-${{ github.ref }}`. `cancel-in-progress` only when the event is `pull_request`.
* One job `validate`, `fail-fast: false`, timeout 30 minutes, matrix `ubuntu-24.04`, `macos-15`, `windows-2025`.
* Checkout `actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1` with `fetch-depth: 1` and `persist-credentials: false`.
* Setup Go `actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` with `go-version-file: go.mod`.
* On every OS: `go mod download`, `go test -shuffle=on ./...`, `go test -shuffle=on ./...` inside `cmd/generate`, and `go build ./example/...`.
* On Linux only: `go test -race -shuffle=on ./...` at the root and in `cmd/generate`; `go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...` at the root and in `cmd/generate`; `go vet ./...` at the root and in `cmd/generate`; `test -z "$(gofmt -l .)"`; `go mod tidy -diff` at the root and in `cmd/generate`; `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0` then `GOLANGCI_LINT` set to that binary and `make lint`; `npx --yes markdownlint-cli2@0.23.2`; `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`; `make check-records`.

## Implementation Steps

1. Copy `.gitattributes` and apply its named delta. Merge `.gitignore` and apply its named delta. Remove `mise.toml` and `treefmt.toml`.
2. Prepend the Makefile shell snippet. Add the library targets. Point `fmt` at `gofmt`. Point `lint` at host plus `GOOS=windows`, root plus `cmd/generate`, with no build tag. Point `vuln` at the pin parsed from `ci.yml`, via `go run`, for both modules. Keep the schema recipes. Change `version` and `release` as in the named deltas. Remove the treefmt `check` target.
3. Add `.golangci.yml` and `scripts/go-precheck.sh` as in the named deltas. `make pre-add-check` calls the script.
4. Write `.github/workflows/ci.yml` and delete the two workflows named above.
5. Copy `.markdownlint-cli2.jsonc` and `scripts/check_records.py`. `cmp` them to the templates. Write `docs/architecture.md` from the slots. Do not create `docs/guides/`.
6. In `docs/README.md`, link `architecture.md` and add the "understand how this fits together" row. Leave the record rows.
7. Replace `AGENTS.md` with the template and the slots. Do not append the old guidelines.
8. Copy the eight byte-for-byte pointer and config files. `cmp` each to its template.
9. Update `RELEASING.md` so the toolchain list is Go from the `go` line, `gofmt`, `golangci-lint` v2.14.0, and `make pre-add-check`. Remove mise, treefmt, `make check`, and the `ANTHROPIC_API_KEY` prerequisite. Keep the schema-tag release rule and the statement that `make release` does not commit or tag.
10. Re-measure, then wrap prose lines longer than 200 characters in `docs/reports/0001-REPORT-go-127-acp-parity.md`. At plan time that count was 48. Do not change claims. Do not wrap MADR or PLAN files.
11. Run `make lint` after installing `golangci-lint` v2.14.0. Fix hand-written findings. If a finding needs a behavior or API change, stop and amend this plan before editing. Do not disable a linter. A `//nolint` names its linter and is added only after a deviation entry in this plan. The lint-findings deviation of 2026-10-09 is that entry for `SendRequest` and the four schema reads. `toReqErr` changes to `errors.As` in the same entry.
12. Run the plants in a scratch copy outside this repository, then delete the copy.
13. Run the target commands. Scan added and changed files for the local account name, mail, git-host domains, and user-profile path prefixes, read from the environment at run time. Do not write those strings into a file.
14. Commit once with `git commit --no-edit`. Do not push. `go install`, `go run`, and `npx` during verification write tool caches outside the tree; that is part of this step and is not a separate toolchain pin.

## Verification

Plants, each in a scratch copy:

* `[x](missing.md)` in `README.md` makes `python3 scripts/check_records.py --check-all` print `broken relative link:` and exit 1.
* A `* item` list in `README.md` makes markdownlint report `MD004/ul-style`.
* A paragraph of 45 `word` tokens (224 characters with spaces) in `README.md` and, separately, appended to `AGENTS.md`, makes markdownlint report `MD013`.
* The same paragraph in a `NNNN-MADR-*.md` passes markdownlint.
* A Go file `gofmt` would rewrite makes `gofmt -l` print that file.
* A scratch module with an unused exported func makes `golangci-lint run -c .golangci.yml` exit non-zero under the new config.
* Appending the local account name to a scratch copy of `AGENTS.md` makes the identifier scan report it. Do not quote the name.

Target:

* `python3 scripts/check_records.py --next` prints `0004`.
* `python3 scripts/check_records.py --check-all` exits 0.
* `npx --yes markdownlint-cli2@0.23.2` exits 0.
* `make check-records`, `make markdownlint`, and `make lint` exit 0.
* `make -n test` and `make -n pre-add-check` exit 0.
* `gofmt -l .` prints nothing. `go mod tidy -diff` exits 0 at the root and in `cmd/generate`.
* `go test ./...` exits 0 at the root and in `cmd/generate`. `go build ./example/...` exits 0.
* `cmp` is silent for every byte-copied file. ~~`.markdownlint-cli2.jsonc` matches the template with no local keys.~~ The `.tools` deviation of 2026-10-09 adds `"ignores": [".tools/**"]`. That file is the one named exception.
* `actionlint` on `.github/workflows/ci.yml` exits 0.
* `git ls-files --eol` reports `attr/text=auto eol=lf` on tracked text files after they are added.

## Rollout and Rollback

No tag and no push. Rollback is reverting the commit. ~~The module API is unchanged except where a linter fix, recorded in a deviation, had to change a signature.~~ No exported signature changes. `toReqErr` unwraps a wrapped `*RequestError` with `errors.As` (lint-findings deviation, 2026-10-09). Callers of the previous CI workflow see `ci.yml` instead, and published releases no longer receive a model-written body.

Leave the schema pin unexecuted. Do not start it from this plan.

## Deviation 2026-10-09

`make lint` with `golangci-lint` v2.14.0 exited 2. Findings included `example/client/main.go` and the other programs under `example/`. `pkg/result/processors/exclusion_paths.go` in that release compiles each `paths` entry as a regexp and matches it against the issue's relative path. `example$` matches only a path that ends in `example`, so `example/client/main.go` stays in the run.

The owner chose to exclude the directory. Both `paths` lists in `.golangci.yml` use `^example/` in place of `example$`. No linter is disabled. Example programs are still built by `go build ./example/...`.

## Deviation 2026-10-09, lint findings

These three were still reported by a single-OS `golangci-lint` v2.14.0 run after the edits that do not change an API or a wire result. Each one is pre-existing code. The new linter is what reports it. No linter is disabled.

`connection.go:774` revive `context-as-argument`:

```text
func SendRequest[T any](c *Connection, ctx context.Context, method string, params any) (T, error) {
```

`ctx` is the second parameter. Moving it first changes the exported signature. Callers include `extensions.go`, the connection tests, and the generated client and agent methods. The owner chose to keep the signature. The signature line carries `//nolint:revive` and names `context-as-argument`.

`errors.go:82` errorlint:

```text
if re, ok := err.(*RequestError); ok {
```

The type assert does not unwrap. A handler error wrapped with `%w` becomes internal error `-32603`. `toReqErr` already unwraps `context.Canceled` with `errors.Is`. The owner chose `errors.As`, so a wrapped `*RequestError` keeps its code. A wrapped `context.Canceled` that contains no `*RequestError` still maps to `-32800`. The new test is `TestToReqErr_WrappedRequestErrorKeepsCode`. It was run against the old assert in a scratch copy before this tree changed. That run printed `expected code -32602, got -32603` and exited 1.

`cmd/generate/internal/load/load.go` gosec G304 at lines 86, 99, 113, and 130, on `os.ReadFile` of `meta.json`, `schema.json`, `meta.unstable.json`, and `schema.unstable.json`. The path is `filepath.Join(schemaDir, <name>)`, and `schemaDir` is the generator's explicit input. The Go 1.21 floor has no `os.OpenRoot`. G304 still flags a non-constant path after a prefix check. The owner chose one `//nolint:gosec` on each of those four calls. G304 stays enabled for every other file.

## Deviation 2026-10-09, markdownlint and `.tools`

`npx --yes markdownlint-cli2@0.23.2` exited 1 with 3622 issues in 45 files. A filter of that output found no finding outside `.tools/`. The directory is an untracked cargo and rustup cache. `.git/info/exclude` lists `/.tools/`. markdownlint-cli2 does not read `.git/info/exclude`. Adding `.tools/` to a scratch `.gitignore` did not change the result, because the fleet config leaves `gitignore` unset.

A scratch config that adds `"ignores": [".tools/**"]` reported `Linting: 7 files` and `Summary: 0 issues`, and exited 0.

The question was left open. The resolution applied is that ignore, in `.markdownlint-cli2.jsonc`. The fleet rule set stays. This file no longer matches its template. Every other byte-copied file still matches. `.tools` is not deleted.

Product markdown fixes in the same pass: 26 hard tabs in `README.md` became four spaces each (`MD010`), and the restarted `1.` items in `RELEASING.md` became `1.` `2.` `3.` (`MD029`). A scratch tree with `.tools` removed, those fixes, and a 224-character paragraph in `0099-MADR-plant.md` exited 0. The planted MADR file was excluded.

The plan's unused-export plant did not fail this config. A scratch module whose only extra symbol was exported `UnusedExport` in `package main` exited 0 with 0 issues. The same module with unexported `unusedHelper` exited 1: `func unusedHelper is unused`. The `unused` linter is enabled. It does not report that exported function.

## Deviation 2026-10-09, unreachable union marshal

`make pre-add-check` ran `go vet` and exited 1. `types_gen.go` reported unreachable code after `return _b, nil` in non-object union `MarshalJSON` methods (first site `types_gen.go:2041`). The generator emitted that return and then still emitted the object-shaping statements. Those statements never ran. `writeUnionMarshal` now returns from the emitter after the non-object return, so the dead statements are not written. Wire behavior is unchanged. `types_gen.go` lost those 12 unreachable tails. The two committed comment lines that a fresh run would rewrite (`// # ACP transport configuration`) stay as they are. A fresh generator run matches that file once those two lines are put back.

## Execution record 2026-10-09

* `python3 scripts/check_records.py --next` printed `0004` and exited 0. `--check-all` exited 0.
* `make markdownlint` exited 0. It linted 7 files and reported 0 issues.
* `make -n test` and `make -n pre-add-check` exited 0.
* `gofmt -l .` printed nothing. `go mod tidy -diff` exited 0 at the root and in `cmd/generate`.
* `go test ./...` exited 0 at the root and in `cmd/generate`. `go build ./example/...` exited 0.
* `make lint` exited 0. The host and `GOOS=windows`, at the root and in `cmd/generate`, each reported 0 issues.
* `make pre-add-check` exited 0: 43 files clean for `gofmt`, `golangci-lint`, `go vet`, `go test`, and `govulncheck`.
* `actionlint` v1.7.12 on `.github/workflows/ci.yml` exited 0.
* `cmp` matched every byte-copied file except `.markdownlint-cli2.jsonc`. That diff is the `.tools` ignore named above.
* The identifier scan of the changed paths reported no hits.
* Scratch plants: a missing link made `--check-all` print `broken relative link:` and exit 1. A star list made markdownlint report `MD004/ul-style`. The 224-character paragraph made it report `MD013` in `README.md` and in `AGENTS.md`. The same paragraph in `0099-MADR-plant.md` was excluded. `gofmt -l` printed the planted Go file. The account-name scan reported the planted name and was quiet on the clean copy.
* The wrapped-request-error test, run against the old type assert in a scratch module, printed `expected code -32602, got -32603` and exited 1. The same test in this tree exited 0 after `errors.As`.

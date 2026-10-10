---
status: accepted
date: 2026-10-09
decision-makers: Repository owner
---

# Install the fleet workspace and library CI

## Context and Problem Statement

The owner is taking over development of this repository. The fleet workspace is not installed: there are no harness pointers, no `scripts/check_records.py`, no markdownlint config, and no `.gitattributes`. `AGENTS.md` is the previous repository's "Repository Guidelines". CI is `.github/workflows/ci.yaml`, which runs `mise exec -- make check` and `mise exec -- make test` on one Linux runner. A second workflow, `.github/workflows/release-notes.yaml`, rewrites a published GitHub Release with an external model and requires an `ANTHROPIC_API_KEY` secret.

The workspace-scaffolding skill is the file set for agent instructions, records, markdownlint, and line endings. It does not by itself own Go lint or CI. The owner required both, and required them to follow the conventions of the sibling libraries `go-llmprovider-sdk` and `go-selfupdate-lib` where those two agree. Existing conventions in this tree yield where they differ: mise, treefmt, gofumpt, the product agent guidelines, and the release-notes workflow.

The Go 1.27.2 language floor and the `schema-v1.24.1` pin remain [0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md). The mise clause of that decision does not. See that record's amendment of 2026-10-09.

## Decision Drivers

* The owner will develop this tree and asked for workspace-scaffolding plus CI/CD in the shape of those sibling libraries.
* All five harness pointers land. `AGENTS.md` is the fleet template with this repository's slots. The previous "Repository Guidelines" are not kept.
* CI matches the shared library shape: three operating systems, pinned `checkout` and `setup-go`, Go from `go-version-file: go.mod`, `contents: read`, shuffled tests, and the Linux gates `gofmt`, `go vet`, `go mod tidy -diff`, `golangci-lint` v2.14.0, race, and `govulncheck` v1.8.0.
* Product-only jobs from either sibling stay out. This module has no installer, service backends, fuzz targets, coverage floors, or live-gateway build tag.
* The protocol generator, `schema/`, examples, and tests stay. They are the SDK, not the conventions being replaced.
* `cmd/generate` is a second module. The Go gates run there as well as at the root.

## Considered Options

* Install the fleet workspace, replace the product agent guidelines, and adopt the shared library CI gates.
* Install the fleet workspace and keep the existing product agent guidelines.
* Leave the repository without the fleet file set.

## Decision Outcome

Chosen option: "Install the fleet workspace, replace the product agent guidelines, and adopt the shared library CI gates", because the owner is taking the repository over and required those standards in place of the previous guidelines and CI.

`AGENTS.md` is replaced by the fleet template. Slots name this module, the default dependency rule plus `go mod tidy -diff`, and `make pre-add-check`. `.github/workflows/ci.yaml` is removed and `.github/workflows/ci.yml` is the library workflow below. `.github/workflows/release-notes.yaml` is removed. `mise.toml` and `treefmt.toml` are removed. `Makefile` keeps schema download, generation, and release preparation, and gains the library targets `test`, `fmt`, `vet`, `lint`, `tidy`, `vuln`, `pre-add-check`, and `help`. Formatting is `gofmt`. `scripts/go-precheck.sh` is the pre-add implementation. `.golangci.yml` takes the linter set from `go-llmprovider-sdk` without that repository's record citations, and without `go-selfupdate-lib`'s import bans.

The library workflow's weekly schedule is included so an advisory run does not wait for a push. `cancel-in-progress` is limited to pull requests, matching that library workflow, so a push to `main` is not cancelled by a later one.

These sibling jobs are not copied: release build and publish, installer containers, systemd, launchd, codesign, SCM, fuzz, shellcheck of installers, API diff against a tag, coverage floors, parity maps, and `depguard` rules. An API-compatibility gate is a later decision.

### Consequences

* Good, because every harness sees the mutate gate, and CI no longer depends on mise or a single custom runner label.
* Good, because `gofmt`, `golangci-lint` v2.14.0, race, and `govulncheck` v1.8.0 match the library gate.
* Bad, because hand-written Go that fails the linter set has to be fixed in this change. A finding that needs a design change stops the plan for an amendment. Linters are not turned off to get a green run.
* Neutral, because the lint amendment of 2026-10-09 keeps `SendRequest`'s parameter order, unwraps a wrapped `*RequestError` in `toReqErr`, and uses a line-scoped `//nolint` where a code change would break the signature or mis-read the generator input. The linters stay enabled.
* Bad, because published releases no longer get a model-written body. Nothing in this decision replaces that workflow.
* Neutral, because [0002-PLAN-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-PLAN-pin-go-1-27-2-and-acp-schema-1-24-1.md) still owns the `go` line and the schema pin. Until it lands, CI's `go-version-file` selects the current `go 1.21` line.

### Confirmation

Byte-copied workspace files match the skill templates. `AGENTS.md` has the fleet headings and does not contain "Repository Guidelines". `mise.toml`, `treefmt.toml`, `.github/workflows/ci.yaml`, and `.github/workflows/release-notes.yaml` are gone. `.github/workflows/ci.yml` uses the pinned actions and the gates named above. `python3 scripts/check_records.py --next` prints `0004`. `--check-all` and markdownlint exit 0. `make -n test` and `make -n pre-add-check` exit 0. Each new gate has failed on a planted input in a scratch copy.

## Pros and Cons of the Options

### Install the fleet workspace, replace the product agent guidelines, and adopt the shared library CI gates

* Good, because one change leaves the clone on the owner's agent rules and CI shape.
* Good, because the protocol generator and the schema stamp remain available for the schema pin.
* Bad, because the first lint pass can force edits in hand-written Go, and the release-notes workflow goes away.

### Install the fleet workspace and keep the existing product agent guidelines

* Good, because agents would still see the previous build and schema notes.
* Bad, because the owner rejected this on 2026-10-09. Those notes describe mise, treefmt, and Go 1.21, which are the conventions being replaced.

### Leave the repository without the fleet file set

* Good, because the current CI workflow would keep running as it does today.
* Bad, because the owner is taking over development and asked for the fleet workspace and the library CI shape.

## More Information

Workspace file set, pointers, slots, forbidden extras, and the record and markdown plants come from the workspace-scaffolding skill. The owner asked in this turn for CI as well, so the skill's "no CI in the scaffold commit" boundary does not apply to this decision.

Compared on 2026-10-09, both sibling libraries use `actions/checkout` at `3d3c42e5aac5ba805825da76410c181273ba90b1` (v7.0.1), `actions/setup-go` at `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` (v7.0.0), `go-version-file: go.mod`, a matrix of `ubuntu-24.04`, `macos-15`, and `windows-2025`, `golangci-lint` v2.14.0, and `govulncheck` v1.8.0. The library workflow also shuffles tests on every OS, runs race and the format gates on Linux, and schedules `17 6 * * 1`. The other sibling adds product jobs this module does not have. Its `.golangci.yml` adds import bans that name other modules; those bans are not copied. The shared linter enable-list is the one in `go-llmprovider-sdk`.

`*_gen.go` already starts with `Code generated by acp-go-generator; DO NOT EDIT.`, which is the header `generated: lax` recognizes. The fleet exclusion path `examples$` does not match this tree's `example/` directory. golangci-lint matches the path regexp against the relative file path, so a pattern ending in `$` does not match files inside the directory. The exclusion in force is `^example/`.

### Amendment 2026-10-09

Single-OS `golangci-lint` v2.14.0, after the fixes that do not change an API or a wire result, still reported three groups. The owner chose a resolution for each. No linter is disabled.

* `SendRequest` keeps the parameter order `(c *Connection, ctx context.Context, method string, params any)`. The signature carries `//nolint:revive` for `context-as-argument`. Moving `ctx` first would be a breaking change for every caller, including the generated client and agent methods.
* `toReqErr` uses `errors.As` for `*RequestError`. A wrapped `*RequestError` keeps that value and its code. A wrapped `context.Canceled` that does not contain a `*RequestError` still becomes request-cancelled (`-32800`). This matches the unwrap `toReqErr` already uses for `context.Canceled`.
* Each `os.ReadFile` of a schema file in `cmd/generate/internal/load/load.go` carries `//nolint:gosec` for G304. `schemaDir` is the generator's explicit input. The Go 1.21 floor has no `os.OpenRoot`, and G304 still flags a non-constant path after a prefix check. G304 stays enabled for every other file.

### Amendment 2026-10-09, markdownlint and `.tools`

`npx --yes markdownlint-cli2@0.23.2` exited 1. Every finding was under `.tools/`, an untracked cargo and rustup cache. `.git/info/exclude` already lists `/.tools/`. markdownlint-cli2 does not read that file, and the fleet config does not set `gitignore`. A scratch copy of the config with `"ignores": [".tools/**"]` linted 7 files and exited 0.

The question was left open. The resolution applied is that ignore. `.markdownlint-cli2.jsonc` keeps the fleet rules and adds that one key. That file is the named exception to the byte-copy. `.tools` stays on disk.

### Amendment 2026-10-09, schema stamp

The context paragraph that leaves the schema pin with 0002 names `schema-v1.24.1`. [0004-MADR-pin-acp-schema-1-25-0.md](0004-MADR-pin-acp-schema-1-25-0.md) supersedes that stamp. The G304 directives in the lint amendment stay until a later idiom record. The floor move does not replace them with `os.OpenRoot`.

Implementation is [0003-PLAN-install-fleet-agent-workspace.md](0003-PLAN-install-fleet-agent-workspace.md).

### Amendment 2026-10-09, commits on main

The owner said this repository is committed on `main` most of the time. `AGENTS.md` now carries one paragraph in its Commits section saying so,
and saying that an agent does not commit, push, or merge on `main` because the organization's git rule forbids it. That paragraph is a named
difference from the workspace-scaffolding `AGENTS.md` template, outside its three slots. A scaffold refresh keeps it.
[0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md) applies the same policy to the plans that are open.

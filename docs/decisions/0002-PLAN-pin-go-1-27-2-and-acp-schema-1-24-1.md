---
status: superseded
date: 2026-10-09
associated-madr: "0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md"
---

# Implement Pin Go 1.27.2 and ACP schema 1.24.1

Associated MADR: [0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md)

## Superseded 2026-10-09

Do not execute this plan. [0004-PLAN-pin-acp-schema-1-25-0.md](0004-PLAN-pin-acp-schema-1-25-0.md) replaces it. The schema stamp below is `1.24.1`. The latest stable schema on 2026-10-09 is `schema-v1.25.0`. The Go 1.27.2 floor is applied by that plan.

## Goal

Build this repository with Go 1.27.2 and generate its ACP API from stable schema release `schema-v1.24.1`.

## Scope

In scope:

* `go.mod` and `cmd/generate/go.mod`: `go 1.27.2`
* `mise.toml`: Go `1.27.2`, `golangci-lint` `2.14.0`, `gofumpt` `0.12.0`, and the newest stable `gopls` that loads a Go 1.27.2 module
* `Makefile` schema download URLs: tag prefix `schema-v`
* `schema/version` set to `1.24.1`, then `make version` so the schema JSON, generated Go, and `version` match
* Compile fixes in examples and tests required by the regenerated API
* `AGENTS.md` and `RELEASING.md` so they describe Go 1.27 and the `schema-v` tag
* `make test` and `make check` on the 1.27.2 toolchain

Out of scope:

* Protocol v2 (`schema-v2.0.0-alpha.7`) and any `protocolVersion: 2` API
* Switching JSON codecs to `encoding/json/v2` defaults
* Tagging, pushing, or publishing a GitHub release
* Hand-editing generated files ahead of the generator

## Implementation Steps

1. Raise the toolchain before the `go` line, so the module still loads at each step.
   * Set `mise.toml` `go` to `1.27.2`.
   * Set `golangci-lint` to `2.14.0` and `gofumpt` to `0.12.0`.
   * Resolve `gopls` with the 1.27.2 toolchain (`go list -m -versions golang.org/x/tools/gopls`) and pin the highest stable version. Record that version in this plan before editing `mise.toml`.
   * Run `mise install` for those tools.
2. Set `go 1.27.2` in `go.mod` and `cmd/generate/go.mod`.
3. Point every schema `curl` in the `Makefile` at `releases/download/schema-v$(ACP_VERSION)/`. Leave `schema/version` as the bare number `1.24.1`.
4. Write `1.24.1` to `schema/version` and run `make version`. That downloads `schema.json`, `schema.unstable.json`, `meta.json`, and `meta.unstable.json`, regenerates Go, and runs `gofumpt`.
5. Repair examples and tests until `go test ./...` and `go build ./example/...` pass. Keep the repairs to types and methods the generator emitted. When a stable method moves from an experimental interface onto `Agent` or `Client`, update the implementers. When an unstable method or field disappears or is renamed, update the call sites to the new generated name.
6. Update `AGENTS.md` so the documented language floor is Go 1.27. Update `RELEASING.md` so the schema tag in the download instructions is `schema-v` plus the number in `schema/version`.
7. Run `make test` and `make check`. `make check` must leave `git diff` empty.
8. Confirm the four schema files are byte-identical to the `schema-v1.24.1` assets, and that `schema/version` and `version` both read `1.24.1`.

## Verification

* `mise exec -- go version` prints `go1.27.2`.
* `go env GOVERSION` while building this module is `go1.27.2`.
* `cmp` of each schema file against the release asset exits 0.
* `make test` exits 0, including `go build ./example/...`.
* `make check` exits 0.
* `Agent` exposes `session/delete`. `Client` exposes `elicitation/create` and `elicitation/complete`. `ToolCall` and `ToolCallUpdate` include `name`.

## Rollout and Rollback

No tag and no push in this plan. Rollback is reverting the commit that contains the pin. A published `v1.24.1` tag is a later, separate decision. Consumers on Go older than 1.27.2 cannot import the module after the `go` line changes. Callers must update for the regenerated interfaces described in the MADR.

Commit once, at the end of this plan, after the verification commands pass. Use `git commit --no-edit` so the repository hook writes the message. Do not pass a message flag.

## Amendment 2026-10-09

[0003-MADR-install-fleet-agent-workspace.md](0003-MADR-install-fleet-agent-workspace.md) supersedes the mise toolchain. Withdrawn, do not execute:

* The `mise.toml` scope bullet and step 1, including `mise install`, `gofumpt`, and the `gopls` resolution.
* Step 6's edit to `AGENTS.md`. After 0003, that file is the fleet template and does not contain the Go 1.21 sentence. `RELEASING.md` in this plan still gains the `schema-v` download prefix.
* Verification lines that call `mise exec` or `make check`.

Still in force: both `go` lines become `1.27.2`, the Makefile schema URLs gain `schema-v`, `schema/version` becomes `1.24.1`, `make version` regenerates the API, examples and tests are repaired to the generated API, and there is no tag and no push. `make version` uses `gofmt`, which 0003 puts in that target. Confirmation is `go env GOVERSION` reporting `go1.27.2`, the schema byte compare, `make test`, and the 0003 Go gates.

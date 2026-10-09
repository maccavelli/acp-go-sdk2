---
status: complete
date: 2026-10-09
associated-madr: "0004-MADR-pin-acp-schema-1-25-0.md"
---

# Implement Pin ACP schema 1.25.0 on the Go 1.27.2 floor

Associated MADR: [0004-MADR-pin-acp-schema-1-25-0.md](0004-MADR-pin-acp-schema-1-25-0.md)

## Goal

Build this repository with Go 1.27.2 and generate its ACP API from stable schema release `schema-v1.25.0`.

## Scope

In scope:

* `go.mod` and `cmd/generate/go.mod`: `go 1.27.2`
* `Makefile` schema download URLs: tag prefix `schema-v`
* `schema/version` set to `1.25.0`, then a regeneration that does not run `mdsh`
* The four schema JSON files and the generated Go that `make version` would write
* The top-level `version` file, byte-for-byte the same number as `schema/version`
* Compile and test repairs required by the regenerated API
* The G304 comments in `cmd/generate/internal/load/load.go` that still say the floor is Go 1.21. The directives stay
* `RELEASING.md`, so the download tag is `schema-v` plus the bare number in `schema/version`

Out of scope:

* Executing [0002-PLAN-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-PLAN-pin-go-1-27-2-and-acp-schema-1-24-1.md)
* Protocol v2 (`schema-v2.0.0-alpha.8`) and any `protocolVersion: 2` API
* The Go 1.27.2 idiom refactor, including `encoding/json/v2` and replacing the G304 directives with `os.OpenRoot`
* Recreating `mise.toml`, or editing `AGENTS.md`
* Running `mdsh`, or changing the README `go get` line that names `v0.13.5`
* A `toolchain` directive in either `go.mod`
* Tagging, pushing, or publishing a GitHub release
* Hand-editing generated files ahead of the generator

## Deviation 2026-10-09, GNU make

`/usr/bin/make` is GNU Make 3.81. `Makefile` line 22 is `ACP_VERSION := $(file < ./schema/version)`. The `file` function arrived in GNU make 4.2. `make -p` showed `ACP_VERSION` empty. `make schema/meta.json` requested `releases/download/schema-v/meta.json`, and curl exited 56 with HTTP 404. `schema/version` contained `1.25.0` and a newline. The four schema JSON files were not modified. That assignment was already present from the fleet Makefile.

The owner chose to install GNU make new enough to implement `$(file)`, and to leave the assignment unchanged. Schema downloads and the `make` targets in this plan use that binary. `RELEASING.md` names GNU make 4.2 or newer, because the release recipes read `schema/version` with `$(file)`.

## Deviation 2026-10-09, elicitation union validator

`go build ./...` failed in generated code at `types_gen.go` `CreateElicitationRequest.Validate`: `v.Message` is undefined. `schema-v1.25.0` defines `CreateElicitationRequest` as an object with required `message` and an `anyOf` of form, url, and other modes. The emitter writes that `anyOf` as a wrapper whose fields are the variants. `message` is copied onto each variant. `emitValidateJen` still treated the wrapper as an object and read `v.Message`. `oneOf` unions already skip that path. `anyOf` unions did not. Hand-editing `types_gen.go` is outside this plan.

The emitter now skips `emitValidateJen` for an `anyOf` union that is emitted as a wrapper, and emits `Validate` on that wrapper. The method checks each shared required string or array on inline variants, which are the variants that merge those properties. A `$ref` variant keeps its own named type and does not gain the parent properties, so the check does not read them there. When no inline variant is set, and at least one inline variant exists, the method reports the first of those required properties. `ToolTerminalRef` passes `TerminalId(terminalID)` because the schema names that field as the `TerminalId` type.

## Implementation Steps

1. Set `go 1.27.2` in `go.mod` and `cmd/generate/go.mod`. The local `go version` already reports `go1.27.2`. Do not add a `toolchain` line. If `go mod tidy` inserts one, delete it and record that in the execution record.
2. Point every schema `curl` in the `Makefile` at `releases/download/schema-v$(ACP_VERSION)/`. Leave `schema/version` as the bare number `1.25.0`.
3. Write `1.25.0` to `schema/version`. Do not run `make version`. That target depends on `README.md`, and the `README.md` recipe runs `mdsh`, which this plan does not invoke. Download the four schema files with their `make` recipes, run `cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .`, run `gofmt -w .` at the root and in `cmd/generate`, and write `version` with the same contents as `schema/version`.
4. Repair examples and tests until `go test ./...` and `go build ./example/...` pass in both modules. Keep the repairs to types and methods the generator emitted. When a stable method moves from an experimental interface onto `Agent` or `Client`, update the implementers. When an unstable method or field disappears or is renamed, update the call sites to the new generated name.
5. In `cmd/generate/internal/load/load.go`, change the four G304 comments so they do not say the language floor is Go 1.21. Leave each `//nolint:gosec` in place.
6. Update `RELEASING.md` so a schema download uses the tag `schema-v` plus the number in `schema/version`. Leave the illustrative module-tag commands as illustrations. Do not change `README.md`.
7. Run `go mod tidy` in both modules. `go mod tidy -diff` must be empty at the commit. Run `make test`, `make pre-add-check`, `make markdownlint`, and `python3 scripts/check_records.py --check-all`.
8. Confirm the four schema files are byte-identical to the `schema-v1.25.0` assets, and that `schema/version` and `version` both read `1.25.0`. Confirm `go env GOVERSION` reports `go1.27.2` while building this module.

## Verification

* Both `go` lines read `1.27.2`, and neither file contains a `toolchain` directive.
* `go env GOVERSION` prints `go1.27.2`.
* `cmp` of each schema file against the release asset exits 0.
* `make test` exits 0, including `go build ./example/...`.
* `make pre-add-check` exits 0.
* `make markdownlint` and `python3 scripts/check_records.py --check-all` exit 0.
* Stable generation still exposes `session/delete` on `Agent` and `elicitation/create` plus `elicitation/complete` on `Client`.
* The generated tree contains the stable `Notice` and `Compaction` types named in the MADR.

## Rollout and Rollback

No tag and no push in this plan. Rollback is reverting the commit that contains the pin. A published `v1.25.0` tag is a later, separate decision. Consumers on Go older than 1.27.2 cannot import the module after the `go` line changes. Callers must update for the regenerated interfaces described in the MADR.

Commit once, at the end of this plan, after the verification commands pass. Use `git commit --no-edit` so the repository hook writes the message. Do not pass a message flag. Stay on the current `feature/` branch. Do not commit on `main`.

## Execution record 2026-10-09

* Homebrew GNU make 4.4.1 is installed as `gmake`. `/usr/bin/make` remains 3.81. Schema downloads and the `make` targets below used `gmake`.
* Both `go` lines are `1.27.2`. `go mod tidy` inserted no `toolchain` directive. `go mod tidy -diff` exited 0 in both modules.
* `schema/version` and `version` both read `1.25.0`. `cmp` of those two files exited 0.
* `make version` was not run. Its `README.md` prerequisite runs `mdsh`. The four schema recipes, `go run .` in `cmd/generate`, and `gofmt -w` at the root and in `cmd/generate` were run instead. `cmp` of each schema file against the `schema-v1.25.0` asset exited 0.
* The four G304 comments in `cmd/generate/internal/load/load.go` already do not name a Go 1.21 floor. Step 5 changed no lines. The directives stay.
* `go env GOVERSION` printed `go1.27.2`.
* `gmake test` exited 0, including `go build ./example/...` and the generator module tests.
* `gmake pre-add-check` exited 0 after the elicitation validator test was added: 43 files clean for `gofmt`, `golangci-lint`, `go vet`, `go test`, and `govulncheck`.
* `gmake markdownlint` exited 0. It linted 7 files and reported 0 issues. `python3 scripts/check_records.py --check-all` exited 0.
* `types_gen.go` contains `DeleteSession(`, `CreateElicitation(`, `CompleteElicitation(`, `type Notice struct`, `type CompactionUpdate struct`, and `type CompactionCapabilities struct`.
* Before the emitter fix, `go build ./...` failed at `CreateElicitationRequest.Validate` with `v.Message` undefined, then at `ElicitationUrlMode.Validate` with `v.Session.Url` undefined. After the fix, the build succeeded.
* `TestCreateElicitationRequest_ValidateRequiresMessage` exited 0. A scratch overlay that expected the empty request to pass printed `plant: empty request was rejected` and exited 1. The tree's test file was not changed for that run.

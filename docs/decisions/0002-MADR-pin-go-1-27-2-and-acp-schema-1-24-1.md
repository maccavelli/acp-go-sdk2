---
status: accepted
date: 2026-10-09
decision-makers: Repository owner
consulted:
informed:
---

# Pin Go 1.27.2 and ACP schema 1.24.1

## Context and Problem Statement

The library module and `cmd/generate` declare `go 1.21`. `mise.toml` installs Go 1.26.3 for development and CI. Go 1.27.2, released 2026-10-08, is the current patch of the current major and includes security fixes. The schema stamp in `schema/version` is `0.13.5`. The current stable ACP schema release is `schema-v1.24.1`, published 2026-09-30. The generated API is behind that schema on stable methods and fields, as recorded in [0001-REPORT-go-127-acp-parity.md](../reports/0001-REPORT-go-127-acp-parity.md).

The module version and the schema stamp are the same number. `RELEASING.md` says each Go release aligns with one upstream schema tag, and `Makefile` downloads `releases/download/v$(ACP_VERSION)/`. That URL matched historical tags such as `v0.13.5`. Current schema assets are published on tags of the form `schema-v1.24.1`.

## Decision Drivers

* Go 1.27.2 carries the security fixes that 1.27.1 does not.
* The `go` line is a minimum toolchain. Go 1.26.3 refuses to load `go 1.27.2`.
* Consumers and this repository should track the approved stable ACP schema, which is protocol v1.
* One stamp should keep the generated code, the schema files, and the module version together, which is the release rule already in `RELEASING.md`.

## Considered Options

* Pin Go 1.27.2 and stable schema `schema-v1.24.1`.
* Pin Go 1.27.2 and leave the schema at `0.13.5`.
* Pin the newest schema tag including prerelease `schema-v2.0.0-alpha.7`.
* Pin Go 1.27.1.

## Decision Outcome

Chosen option: "Pin Go 1.27.2 and stable schema `schema-v1.24.1`", because the owner required the security patch release and the latest ACP schema, and the latest approved schema is the stable v1 release `schema-v1.24.1`.

`schema/version` becomes `1.24.1`. Fetch URLs gain the `schema-v` tag prefix so `make version` downloads that release. Both `go` lines become `1.27.2`.

Annotated 2026-10-19 by [0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md): the Go 1.27.2 floor is superseded by a `go 1.26.9` line, once that
record's [0024-PLAN-go-1-26-9-floor.md](0024-PLAN-go-1-26-9-floor.md) step 2 lands. go1.26.9 was released the same day as
go1.27.2, with the same security fixes, so the reason given here still holds at the new floor. The schema stamp was already superseded by 0004.

~~Both `go` lines and the mise Go pin become `1.27.2`. Editor tools in `mise.toml` that cannot load a Go 1.27 module move to a release that can: `golangci-lint` 2.14.0 and `gofumpt` 0.12.0. `gopls` moves to the newest stable release that loads Go 1.27.2.~~ The mise, gofumpt, and gopls sentences are withdrawn by the amendment below. `golangci-lint` v2.14.0 remains the linter version.

Protocol v2 draft `schema-v2.0.0-alpha.7` stays out of this pin. It is a prerelease, and its migration guide keeps `protocolVersion: 1` as the deployed protocol.

### Consequences

* Good, because builds use the Go release that contains the 2026-10-08 security fixes to the `go` command, `crypto/tls`, `html/template`, `net/http`, `net/textproto`, and `os`.
* Good, because the generated v1 surface is regenerated from `schema-v1.24.1` instead of schema stamp `0.13.5`.
* Bad, because the next module tag under the existing release rule is `v1.24.1`, a major bump from `v0.13.5`. The import path stays `github.com/coder/acp-go-sdk` for v1. This decision does not create the tag.
* Bad, because regeneration moves `session/delete` and the elicitation methods onto the stable interfaces and drops or renames unstable methods and fields. Callers of `Agent`, `Client`, `AgentExperimental`, and `ClientExperimental` will need source changes.
* Neutral, because `encoding/json` on Go 1.27 keeps the v1 API and changes its implementation. Golden files under `testdata/json_golden` are the check. This decision does not switch call sites to `encoding/json/v2` defaults.

### Confirmation

~~`go version` under mise reports `go1.27.2`.~~ Both `go.mod` files contain `go 1.27.2`. `schema/version` and `version` contain `1.24.1`. The four schema files match the `schema-v1.24.1` release assets. `make test` passes, and the Go gates from [0003-MADR-install-fleet-agent-workspace.md](0003-MADR-install-fleet-agent-workspace.md) pass, on a 1.27.2 toolchain. `make check` is not part of confirmation after that amendment.

## Pros and Cons of the Options

### Pin Go 1.27.2 and stable schema `schema-v1.24.1`

* Good, because it takes the security patch and the current stable schema in one pin.
* Good, because it keeps the repository's one-stamp release rule.
* Bad, because the Go module version and the public generated API both move.

### Pin Go 1.27.2 and leave the schema at `0.13.5`

* Good, because the toolchain security fixes land without an API break.
* Bad, because the SDK remains behind stable ACP, which is the gap [0001-REPORT-go-127-acp-parity.md](../reports/0001-REPORT-go-127-acp-parity.md) recorded.

### Pin the newest schema tag including prerelease `schema-v2.0.0-alpha.7`

* Good, because that tag is the newest schema artifact by publication order.
* Bad, because the tag is a prerelease and the v2 book is still a draft.
* Bad, because v2 removes the v1 filesystem and terminal methods and changes `session/prompt`. Replacing the v1 pin with v2 drops the protocol this module implements.

### Pin Go 1.27.1

* Good, because 1.27.1 was the patch named in the earlier study.
* Bad, because 1.27.2, released the same day as that study, is the release that contains the security fixes. A `go 1.27.1` line still allows a 1.27.1 toolchain.

## More Information

Evidence for the schema gap, the `schema-v` tag layout, and the Go 1.27 behavior changes is in [0001-REPORT-go-127-acp-parity.md](../reports/0001-REPORT-go-127-acp-parity.md).

Go 1.27.2 release notes: <https://go.dev/doc/devel/release#go1.27.1>. The minor-revision entry for `go1.27.2` (released 2026-10-08) lists the security fixes.

Stable schema release: <https://github.com/agentclientprotocol/agent-client-protocol/releases/tag/schema-v1.24.1>.

`golangci-lint` 2.13.0 (2026-08-19) added Go 1.27 support. 2.14.0 (2026-09-24) is the latest release. ~~The pinned 2.12.2 binary predates that support. CI does not run the linter today. `mise.toml` still installs it, so the pin moves with the Go pin.~~ The amendment below moves the linter pin to CI's `go install` and drops mise.

Implementation is [0002-PLAN-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-PLAN-pin-go-1-27-2-and-acp-schema-1-24-1.md).

## Amendment 2026-10-09

The owner directed that previous repository conventions yield to the standards in [0003-MADR-install-fleet-agent-workspace.md](0003-MADR-install-fleet-agent-workspace.md). The Go 1.27.2 language floor and the schema pin `schema-v1.24.1` stand. These parts of the outcome no longer stand:

* `mise.toml` is not the toolchain pin, and this decision does not recreate it.
* `gofumpt` and a mise `gopls` pin are not required. Formatting is `gofmt`.
* Confirmation does not use `go version` under mise, and it does not use `make check`.

`golangci-lint` v2.14.0 remains the linter version. CI installs it with `go install`, which 0003 records. The `go` line stays the switch CI reads through `go-version-file`.

## Amendment 2026-10-09, schema stamp

The releases API on 2026-10-09 listed stable `schema-v1.25.0`, published `2026-10-09T14:17:40Z`, with `meta.json`, `schema.json`, `meta.unstable.json`, and `schema.unstable.json`. `schema-v1.24.1` is no longer the latest stable schema. The owner asked for the latest schema. [0004-MADR-pin-acp-schema-1-25-0.md](0004-MADR-pin-acp-schema-1-25-0.md) supersedes the schema stamp in this decision. The Go 1.27.2 outcome stands. Do not execute the `1.24.1` stamp in this decision's plan.

## Amendment 2026-10-09, module path

The sentence in this decision that the import path stays `github.com/coder/acp-go-sdk` for v1 no longer stands. [0005-MADR-separate-module-from-coder.md](0005-MADR-separate-module-from-coder.md) supersedes that sentence. The Go 1.27.2 outcome still stands.

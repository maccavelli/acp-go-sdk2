---
status: accepted
date: 2026-10-09
decision-makers: Repository owner
---

# Pin ACP schema 1.25.0 on the Go 1.27.2 floor

## Context and Problem Statement

[0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md) accepted Go 1.27.2 and stable schema `schema-v1.24.1`. That schema tag was the latest stable release when the record was written. Neither the `go` line nor the schema stamp has been changed. Both modules still declare `go 1.21`. `schema/version` still reads `0.13.5`.

On 2026-10-09 the GitHub releases API listed a newer stable schema tag, `schema-v1.25.0`, published `2026-10-09T14:17:40Z`. It is not a prerelease. Its assets are `meta.json`, `schema.json`, `meta.unstable.json`, and `schema.unstable.json`. The same publication includes prerelease `schema-v2.0.0-alpha.8` and Rust crate tag `v1.11.0`. The crate tag has an empty asset list. It is not a schema drop.

The owner asked to pin the latest schema and to set `go.mod` and `cmd/generate/go.mod` to `1.27.2`. Go 1.27.2, released 2026-10-08, is still the current patch of the current major. The owner also said the repository will be refactored to Go 1.27.2 idioms and will keep the workspace-scaffolding standards. That refactor is a later record. This decision is the pin.

## Decision Drivers

* The owner asked for the latest schema, and the latest stable schema is now `schema-v1.25.0`.
* The Go 1.27.2 floor already accepted in 0002 has not been applied. Both changes belong in one generated tree.
* Protocol v1 remains the approved stable protocol. A prerelease v2 tag is not that pin.
* Workspace scaffolding from [0003-MADR-install-fleet-agent-workspace.md](0003-MADR-install-fleet-agent-workspace.md) stays. This decision does not recreate `mise.toml`.

## Considered Options

* Pin stable schema `schema-v1.25.0` and apply the accepted Go 1.27.2 floor.
* Execute 0002 unchanged, pinning `schema-v1.24.1`.
* Pin prerelease `schema-v2.0.0-alpha.8`.
* Raise the `go` line and leave `schema/version` at `0.13.5`.

## Decision Outcome

Chosen option: "Pin stable schema `schema-v1.25.0` and apply the accepted Go 1.27.2 floor", because the owner asked for the latest schema and for Go 1.27.2, and the latest stable schema published on 2026-10-09 is `schema-v1.25.0`.

This record supersedes the schema stamp in 0002. It does not supersede the Go 1.27.2 choice. `schema/version` becomes `1.25.0`. Fetch URLs gain the `schema-v` tag prefix. Both `go` lines become `1.27.2`.

Annotated 2026-10-19 by [0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md): the Go 1.27.2 floor is superseded by a `go 1.26.9` line, once that
record's [0024-PLAN-go-1-26-9-floor.md](0024-PLAN-go-1-26-9-floor.md) step 2 lands. go1.26.9 was released the same day as
go1.27.2, with the same security fixes, so the reason given here still holds at the new floor. The schema `1.25.0` stamp stands.

`schema-v2.0.0-alpha.8` stays out. The release notes for `schema-v1.25.0` say that release stabilizes session notices and session compaction, adds an unstable error stop reason after prompt insertion, and rejects malformed values that receivers act on. Stable `meta.json` for that tag reports `version` 1 and includes `session_delete`, `elicitation_create`, and `elicitation_complete`. Stable `schema.json` defines `Notice`, `NoticeCapabilities`, `NoticeSeverity`, `CompactionCapabilities`, `CompactionId`, `CompactionStatus`, `CompactionSummaryChunk`, and `CompactionUpdate`.

### Consequences

* Good, because the generated v1 surface comes from the latest stable schema instead of stamp `0.13.5` or the older stable tag `schema-v1.24.1`.
* Good, because builds can use the Go release that contains the 2026-10-08 security fixes.
* Bad, because callers of the generated interfaces must follow the regenerated API, including methods and fields that move between stable and unstable.
* Bad, because the next module tag under the existing release rule would be `v1.25.0`. This decision does not create that tag. The README `go get` line stays on `v0.13.5` until a release exists.
* Neutral, because a later record owns the Go 1.27.2 idiom pass. This decision does not switch call sites to `encoding/json/v2` and does not replace the G304 `//nolint` directives with `os.OpenRoot`.

### Confirmation

Both `go.mod` files contain `go 1.27.2`. `schema/version` and `version` contain `1.25.0`. The four schema files match the `schema-v1.25.0` release assets. `go env GOVERSION` reports `go1.27.2`. `make test` passes, and the Go gates from 0003 pass, on that toolchain. There is no tag and no push.

## Pros and Cons of the Options

### Pin stable schema `schema-v1.25.0` and apply the accepted Go 1.27.2 floor

* Good, because it matches the owner's request for the latest schema and for Go 1.27.2.
* Good, because one regeneration consumes one schema tag.
* Bad, because the public generated API and the language floor both move.

### Execute 0002 unchanged, pinning `schema-v1.24.1`

* Good, because that stamp was already accepted.
* Bad, because `schema-v1.24.1` is no longer the latest stable schema. Executing it would miss the notices and compaction types stabilized in `schema-v1.25.0`.

### Pin prerelease `schema-v2.0.0-alpha.8`

* Good, because that tag is newer by publication order than `schema-v1.25.0`.
* Bad, because the tag is a prerelease. Replacing the v1 pin with v2 drops the protocol this module implements.

### Raise the `go` line and leave `schema/version` at `0.13.5`

* Good, because the toolchain floor moves without an API break.
* Bad, because the SDK remains behind stable ACP, which is the gap [0001-REPORT-go-127-acp-parity.md](../reports/0001-REPORT-go-127-acp-parity.md) recorded.

## More Information

Go 1.27.2 release notes: <https://go.dev/doc/devel/release>. The `go1.27.2` entry, released 2026-10-08, lists the security fixes. No later 1.27 patch was listed on 2026-10-09.

Stable schema release: <https://github.com/agentclientprotocol/agent-client-protocol/releases/tag/schema-v1.25.0>.

The owner said this repository will be refactored to Go 1.27.2 idioms, best practices, and standards, and that workspace scaffolding remains the standard. That refactor is not this decision. Workspace scaffolding stays 0003. The idiom pass, including whether `os.OpenRoot` replaces the G304 directives, waits for its own record.

Implementation is [0004-PLAN-pin-acp-schema-1-25-0.md](0004-PLAN-pin-acp-schema-1-25-0.md). 0002's plan is not executed.

## Amendment 2026-10-09, GNU make

`ACP_VERSION` is read with GNU make's `$(file)` function, which Make 3.81 does not implement. On this machine `/usr/bin/make` is 3.81, so the schema recipe expanded to `schema-v/meta.json` and the download returned HTTP 404. The owner chose to install GNU make 4.2 or newer and to keep the `$(file)` assignment. Confirmation of this pin uses that make.

## Amendment 2026-10-09, module path

This decision left the README `go get` line on `github.com/coder/acp-go-sdk@v0.13.5`. [0005-MADR-separate-module-from-coder.md](0005-MADR-separate-module-from-coder.md) replaces that module path. The schema stamp `1.25.0` stands. This amendment does not create a module tag.

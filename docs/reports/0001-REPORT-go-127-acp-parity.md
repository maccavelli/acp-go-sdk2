# Go 1.27.1 floor and ACP spec parity

Observed 2026-10-08. This report records what the tree contains, what Go 1.27.1 changes, and how that lines up with the Agent Client Protocol (ACP) documents and schema releases current on that date.
It chooses no floor, no schema pin, and no rewrite.

## Question

The module files declare `go 1.21`. The question is whether this SDK can move its language floor to Go 1.27.1 and, in the same effort, match current Go practice and the approved ACP spec.

Two answers came out of the reading, and they are separate:

- Raising the language floor to 1.27.1 is feasible for this repository. The library is already written in the style Go 1.21 made normal, it has no runtime third-party dependencies, and the language
additions in 1.27 do not fit the public interfaces.
- The SDK is a protocol v1 library pinned to schema stamp `0.13.5`. The current stable schema release is `schema-v1.24.1` (2026-09-30). Method names mostly match. Several stable methods and fields are
still generated as unstable, and some stable fields are absent. ACP v2 is a published draft, `schema-v2.0.0-alpha.7`, and is not the approved stable spec.

## How this was checked

Read in the tree: `go.mod`, `cmd/generate/go.mod`, `mise.toml`, `Makefile`, `.github/workflows/ci.yaml`, `AGENTS.md`, `schema/version`, `schema/meta.json`, `schema/meta.unstable.json`,
`schema/schema.json`, `schema/schema.unstable.json`, `agent.go`, `client.go`, `connection.go`, `errors.go`, `extensions.go`, and the generated `Agent`, `Client`, `SessionUpdate`, `ToolCall`,
`ContentChunk`, `ClientCapabilities`, and `SessionCapabilities` types.

Compared locally, with the schema files downloaded from the GitHub release assets:

- stable and unstable `meta.json` method maps
- stable and unstable `$defs` names, property keys, required keys, and `oneOf` / `anyOf` discriminators

External sources, fetched 2026-10-08:

- [Go release history](https://go.dev/doc/devel/release) for 1.27.1 and 1.27.2
- [Go 1.27 release notes](https://go.dev/doc/go1.27)
- [Go toolchains](https://go.dev/doc/toolchain)
- [ACP documentation index](https://agentclientprotocol.com/llms.txt)
- [ACP v1 overview](https://agentclientprotocol.com/protocol/v1/overview)
- [ACP v1 initialization](https://agentclientprotocol.com/protocol/v1/initialization)
- [ACP v1 transports](https://agentclientprotocol.com/protocol/v1/transports)
- [ACP v2 migration](https://agentclientprotocol.com/protocol/v2/migration) (draft)
- GitHub releases for `agentclientprotocol/agent-client-protocol`, including tags `schema-v1.24.1`, `schema-v2.0.0-alpha.7`, and `v1.10.2`
- golangci-lint changelog entry for v2.13.0 ("go1.27 support", released 2026-08-19) and the [supported-versions FAQ](https://golangci-lint.run/docs/welcome/faq/)

Not done: no install of Go 1.27.1, no `go test` on that toolchain, no byte-for-byte diff of schema description text, and no run of `gofumpt`, `gopls`, or `golangci-lint` against a `go 1.27.1` module.
Where a behavior change is plausible and untested, the report says so.

## What the repository pins today

| Pin | Value | Where |
| :--- | :--- | :--- |
| Language floor, library module | `go 1.21` | `go.mod` |
| Language floor, generator module | `go 1.21` | `cmd/generate/go.mod` |
| Developer and CI toolchain | Go 1.26.3 | `mise.toml`; CI runs `mise exec` |
| `gopls` | 0.22.0 | `mise.toml` |
| `golangci-lint` | 2.12.2 | `mise.toml`; not invoked by `make test` or CI |
| `gofumpt` | 0.10.0 | `mise.toml`; `make version` runs it |
| Formatter gate | `treefmt` then `git diff --exit-code` | `Makefile` `check` target |
| ACP schema stamp | `0.13.5` | `schema/version` |
| Generator dependency | `github.com/dave/jennifer` v1.7.1 | `cmd/generate/go.mod` |

`AGENTS.md` still says the code targets Go 1.21 idioms.

The `go` line is a minimum toolchain version, not a comment. From Go 1.21 onward the `go` command refuses to load a module whose `go` line is newer than itself, including the patch number. A module
that says `go 1.27.1` does not load under Go 1.26.3 or Go 1.27.0. Go 1.27.2 does load it. CI takes its Go from `mise.toml`, so the floor and the mise pin have to move together or `make test` cannot
start.

The language version in `go.mod` also selects language semantics and `GODEBUG` defaults. The tree is compiled with Go 1.26.3 today, and it still gets Go 1.21 loop-variable semantics and Go 1.21
`GODEBUG` defaults, because the `go` line says `1.21`. Raising the line opts this module into the semantic changes between 1.22 and 1.27 in one step.

## Go 1.27.1

Go 1.27.0 was released on 2026-08-19. Go 1.27.1 was released on 2026-09-01 and includes fixes to cgo, the compiler, the runtime, `go fix`, and the `database/sql`, `debug/elf`, `encoding/json`,
`net/http`, `os`, `simd`, and `simd/archsimd` packages.

Go 1.27.2 was released on 2026-10-08, the date of this report. It includes security fixes to the `go` command and to `crypto/tls`, `html/template`, `net/http`, `net/textproto`, and `os`, plus other
bug fixes, including `encoding/json` and `encoding/json/v2`. A `go 1.27.1` line allows a 1.27.1 toolchain. It does not force 1.27.2.

Under the Go release policy, a major release is supported until two newer majors exist. Go 1.27.0 (2026-08-19) ended support for Go 1.25. Supported majors on this date are 1.26 and 1.27. Go 1.21 left
support when Go 1.23 shipped (2024-08-13).

### Language and library changes that touch this SDK

Go 1.27 adds generic methods. A method may declare its own type parameters. Interface methods may not declare type parameters, and a generic method cannot implement an interface method. `Agent`,
`AgentLoader`, `AgentExperimental`, `Client`, `ClientExperimental`, and `ExtensionMethodHandler` are interfaces. Generic methods do not replace them. The generic surface that already exists, `Ptr[T]`
and `SendRequest[T]`, is the shape those operations need.

A struct literal key may name a nested or embedded field. Function type inference applies in more assignment contexts. Neither change alters the generated API.

`encoding/json` is now implemented on top of `encoding/json/v2`. The v1 functions keep their behavior. Error strings may differ. `GOEXPERIMENT=nojsonv2` restores the previous implementation at build
time, and the release notes say that opt-out is expected to go away later. The v2 package itself rejects invalid UTF-8 and duplicate object names unless the caller sets options. This SDK marshals
every JSON-RPC message with `encoding/json` (`connection.go`) and checks payloads in `testdata/json_golden` through `json_parity_test.go`. A floor bump uses the new implementation under the v1 API.
Moving call sites onto v2 defaults would change which payloads the codec accepts. Those are different changes. The golden tests were not run on Go 1.27.1 for this report.

`gofmt` column alignment can change for a small number of files, and the old alignment varied by CPU architecture. `make check` fails on any formatter diff. `make version` runs `gofumpt` 0.10.0, which
is pinned from before the 1.27 release. Whether that `gofumpt` matches Go 1.27 `gofmt` was not tested.

`go test` runs the `stdversion` vet check by default. With a `go 1.27.1` line, standard library symbols from 1.27 are allowed. The check matters if a file is later built under an older language
version. It does not block the bump itself.

`go mod tidy`, for modules at `go 1.27` or newer, merges `require` blocks into one direct block and one indirect block. The library module has no `require` block. The generator module has one require.

The `asynctimerchan` `GODEBUG` setting is gone. `time` channels are unbuffered. The library uses `time.After` as a test deadline and `time.NewTimer` in `example/agent`. Those call sites receive from
the channel. They were not re-run on Go 1.27.

These `GODEBUG` settings are removed: `gotypesalias`, `tlsunsafeekm`, `tlsrsakex`, `tls3des`, `tls10server`, and `x509keypairleaf`. The SDK does not set them. The generator does not import `go/types`.
The `gotypesalias` removal matters to tools that inspect alias nodes, not to the JSON-RPC path.

Go 1.27 requires macOS 13 Ventura or later for the Darwin port. The linker writes that minimum into the Mach-O load command. Linux CI is unaffected. A macOS builder older than 13 cannot run the 1.27
toolchain.

The `go` command no longer fetches modules from Bazaar. This module does not.

Other 1.27 library additions (`crypto/mldsa`, `uuid`, experimental `simd`, the goroutine-leak profile, `bytes.CutLast`, `strings.CutLast`) have no call site in this repository. They are available
after the floor moves. They do not need a sweep of the existing code.

### Changes accumulated since the 1.21 line

The floor has stayed at 1.21 while the toolchain moved to 1.26.3, so these semantics are not in effect for this module until the `go` line rises:

- Go 1.22 gives each loop iteration its own variables. One range loop in `acp_test.go` copies the index and the request before starting a goroutine. The goroutines read in `connection.go` take the
loop values as parameters. A full closure audit was not done.
- Go 1.22 adds integer `range`. Go 1.23 adds range over iterator functions. The generator walks maps and slices with index loops and sorts keys before emitting. Iterators would not change the wire
format.
- Go 1.24 finishes generic type aliases. The generator emits ordinary named types.

The code already uses the 1.18–1.21 vocabulary: `any` rather than `interface{}`, type parameters, `log/slog`, `errors.Is`, and `context`. There is no `io/ioutil` and no `golang.org/x` import in the
library module. `slices` and `maps` have been in the standard library since 1.21, which is the current floor, and the hand-written code has little use for them.

`testing/synctest` (stable in the 1.25 line) could replace some `time.After` deadlines in the connection tests. That is a test-harness change, independent of the floor.

## Floor bump, as a feasibility finding

A floor of 1.27.1 means all of the following, and nothing in the protocol generator has to change for the module to compile:

1. Set `go 1.27.1` in `go.mod` and in `cmd/generate/go.mod`.
2. Move the mise Go pin from 1.26.3 to a 1.27 toolchain at least as new as 1.27.1. CI follows that pin.
3. Re-run `make test` and `make check` on that toolchain. The JSON golden files and the `encoding/json` error strings are the likely failures, and they were not executed here.
4. Revisit the editor pins. `golangci-lint` 2.12.2 (2026-05-06) predates the v2.13.0 release that added Go 1.27 support (2026-08-19). The FAQ states that a binary supports Go versions up to the Go
version it was built with, and the 1.26-built binaries do not support 1.27. CI does not run the linter today. `gopls` is pinned at 0.22.0; `v0.24.0-pre.1` existed by 2026-09-30. This report did not
run either tool on a 1.27.1 module.
5. Expect `AGENTS.md` to be stale the moment the floor moves. It still names Go 1.21 idioms.

`github.com/dave/jennifer` v1.7.1 was not built or tested with Go 1.27 for this report.

A rewrite onto generic methods, iterators, or `encoding/json/v2` defaults is feasible as new code and is a poor match for the existing API. The public surface is schema-shaped interfaces and generated
unions. Hand-editing `types_gen.go`, `agent_gen.go`, or `client_gen.go` fights `make version`.

## ACP: what "current" and "approved" refer to

ACP has two version numbers, and this repository's stamp is a third.

The wire protocol version inside `initialize` is a single integer. It increases for breaking changes. The v1 documents say clients and agents exchange that integer and then follow the matching
specification. Non-breaking features arrive as capabilities. Omitted capabilities mean unsupported.

On 2026-10-08 the documentation index publishes two protocol books:

- Protocol v1, under `/protocol/v1/`, is the stable book. Its overview lists the baseline and optional methods below.
- Protocol v2, under `/protocol/v2/`, is a draft. The migration guide says the v2 schema baseline is stable enough to read and that the protocol surface is still draft. It tells implementers to keep
serving `protocolVersion: 1` and to gate v2 behind negotiation. The schema tag `schema-v2.0.0-alpha.7` is a GitHub prerelease (2026-09-30).

The schema artifacts are published as GitHub release assets. The newest stable schema tag found in the releases API is `schema-v1.24.1`, published 2026-09-30, with `meta.json`, `schema.json`,
`meta.unstable.json`, and `schema.unstable.json`. The same day published prerelease `schema-v2.0.0-alpha.7`. Rust crate tag `v1.10.2` (2026-10-01) has an empty asset list. It is not a schema drop.

This SDK's `schema/version` file contains `0.13.5`. `make version` downloads:

`https://github.com/agentclientprotocol/agent-client-protocol/releases/download/v$(ACP_VERSION)/schema.json`

That URL shape matches the historical tag `v0.13.5`. It does not match the tag `schema-v1.24.1`. Putting `1.24.1` in `schema/version` would request `.../download/v1.24.1/...`, which is a different tag
from the schema release. The fetch rule in the Makefile has to change before a schema bump can use the current assets. That was observed from the Makefile and the releases API. The `v1.24.1` URL was
not requested.

Official libraries listed on the documentation index are Kotlin, Java, Python, Rust, and TypeScript, plus a community page. This module is `github.com/coder/acp-go-sdk`. Parity here is against the
schema and the v1 documents, not against another language SDK.

### Stable v1 surface in the documents

Baseline agent methods: `initialize`, `authenticate`, `session/new`, `session/prompt`. Optional agent methods: `session/load`, `logout`, `session/set_mode`. Notification: `session/cancel`.

Baseline client method: `session/request_permission`. Optional client methods: `fs/read_text_file`, `fs/write_text_file`, `terminal/create`, `terminal/output`, `terminal/release`,
`terminal/wait_for_exit`, `terminal/kill`, `elicitation/create`. Notifications: `session/update`, `elicitation/complete`.

Initialization also documents capabilities that are easy to miss if one only diffs method names: client `auth.terminal`, client `elicitation`, client `session.configOptions.boolean`, agent
`session.delete`, agent `session.additionalDirectories`, and implementation `name` / `title` / `version`. File paths are absolute. Line numbers are 1-based. Extension methods use a `_` prefix. `_meta`
carries extension data, with `traceparent`, `tracestate`, and `baggage` reserved.

The v1 transport that is approved is stdio: newline-delimited JSON-RPC, UTF-8, no embedded newlines, logs on stderr. Streamable HTTP is marked as a draft on that same page. This SDK writes one JSON
value plus `\n` and reads with `bufio.Scanner`, capped at 10 MiB. The stdio section states the framing and does not state a size cap. The 10 MiB cap is an SDK limit. The connection is an `io.Reader` /
`io.Writer` pair, which is what the examples use for stdio. Streamable HTTP is not implemented.

### Stable schema-v1.24.1 method diff

Local stable `schema/meta.json` against `schema-v1.24.1` `meta.json`:

| Side | In schema-v1.24.1 stable, absent from local stable meta |
| :--- | :--- |
| Agent | `session/delete` |
| Client | `elicitation/create`, `elicitation/complete` |
| Protocol | `$/cancel_request` |

Every other stable method name matches, in both directions.

`session/delete` and the elicitation methods exist in this repo on `AgentExperimental` and `ClientExperimental`. The generator's own comment says those interfaces are outside the official spec. The v1
overview and `schema-v1.24.1` stable meta now include them. `SessionCapabilities.Delete` is generated with an "UNSTABLE" comment. The v1 initialization page describes `session.delete` as a stable
capability.

`$/cancel_request` is implemented in `connection.go` and covered by `connection_cancel_test.go`. `errors.go` uses code `-32800` for request cancellation. The stable schema's `ErrorCode` union lists
`-32700`, `-32600`, `-32601`, `-32602`, `-32603`, `-32000`, and `-32002`. The `schema-v1.24.1` union adds `-32800`. The connection path sends the integer itself, so the missing enum variant does not
stop the notification. The schema union is behind the code.

### Stable schema field and definition diff

`schema/schema.json` has 129 `$defs`. `schema-v1.24.1` `schema.json` has 170. No definition name exists only on the local side. Forty-one names exist only upstream. Many of those names already appear
in the local unstable schema and in `types_gen.go`, often with an "UNSTABLE" comment, because `cmd/generate` promotes an unstable definition onto the stable name when the two definitions differ. The
stable schema file and the generated comment can both be stale while the field is present in Go.

Definitions present only in `schema-v1.24.1` stable:

`AuthCapabilities`, `AuthMethodId`, `AuthMethodTerminal`, `BooleanConfigOptionCapabilities`, `BooleanPropertySchema`, `CancelRequestNotification`, `ClientSessionCapabilities`,
`CompleteElicitationNotification`, `Cost`, `CreateElicitationRequest`, `CreateElicitationResponse`, `DeleteSessionRequest`, `DeleteSessionResponse`, `ElicitationAcceptAction`,
`ElicitationCapabilities`, `ElicitationContentValue`, `ElicitationFormCapabilities`, `ElicitationFormMode`, `ElicitationId`, `ElicitationPropertySchema`, `ElicitationRequestScope`,
`ElicitationSchema`, `ElicitationSchemaType`, `ElicitationSessionScope`, `ElicitationUrlCapabilities`, `ElicitationUrlMode`, `EnumOption`, `IntegerPropertySchema`, `MessageId`, `MultiSelectItems`,
`MultiSelectPropertySchema`, `NumberPropertySchema`, `SessionConfigBoolean`, `SessionConfigOptionsCapabilities`, `SessionDeleteCapabilities`, `StringFormat`, `StringMultiSelectItems`,
`StringPropertySchema`, `TerminalId`, `TitledMultiSelectItems`, `UsageUpdate`.

Shared definitions whose property keys differ:

| Definition | Local stable only | schema-v1.24.1 only |
| :--- | :--- | :--- |
| `ClientCapabilities` | | `auth`, `elicitation`, `session` |
| `ContentChunk` | | `messageId` |
| `SessionCapabilities` | | `delete` |
| `SetSessionConfigOptionRequest` | `value` (also required locally) | |
| `ToolCall` | | `name` |
| `ToolCallUpdate` | | `name` |

Discriminator and enum drift on shared definitions:

- `SessionUpdate` locally ends at `session_info_update`. Upstream stable adds `usage_update`.
- `SessionConfigOptionCategory` locally has `mode`, `model`, `thought_level`. Upstream adds `model_config`. The generated constants match the local set.
- `ErrorCode` is missing `-32800`, as above.
- `Role` has no consts locally. Upstream lists `assistant` and `user`.

What the generated Go actually does with those gaps:

- `ContentChunk.MessageId` exists and is commented unstable. The announcement index and the stable schema treat message IDs as part of stable v1. The field is optional on the wire (`messageId` is not
in `ContentChunk`'s required list upstream).
- `SessionUpdate` unmarshals `usage_update` and comments that variant unstable. Upstream stable includes the variant.
- `ClientCapabilities` in Go has `auth` and `elicitation`, both commented unstable, plus unstable-only `nes`, `planCapabilities`, and `positionEncodings`. It has no `session` field. Upstream stable
`ClientCapabilities` properties are `_meta`, `auth`, `elicitation`, `fs`, `session`, and `terminal`. A typed client cannot advertise `clientCapabilities.session` (the boolean config-option capability)
because the field is not on the struct.
- `SetSessionConfigOptionRequest` in Go is already a union of a boolean variant and a value-id variant. That matches the upstream `anyOf` shape better than the committed stable schema, which still
requires a single `value` property. The generator promotion from the local unstable schema is why.
- `ToolCall` and `ToolCallUpdate` have no `name` field. Upstream stable adds optional `name`. The tool-call-name announcement says that field is stable. JSON from a peer that includes `name` is
dropped on unmarshal into these structs. JSON this SDK sends omits `name`.

`session/new`, `session/prompt`, `session/load`, `session/resume`, `session/close`, `session/list`, `session/set_mode`, `session/set_config_option`, `fs/read_text_file`, `fs/write_text_file`, and the
five `terminal/*` methods are on the stable `Agent` or `Client` interfaces. `additionalDirectories` is on the generated session lifecycle structs. Prompt content kinds `text`, `image`, `audio`,
`resource_link`, and `resource` are present. Stop reasons used by the examples include `end_turn`. Those parts of v1 are in the generated API.

Generated comments still link `https://agentclientprotocol.com/protocol/...` without the `/v1/` segment. The live book is under `/protocol/v1/`. Redirect behavior was not checked.

### Unstable schema drift

Local unstable methods against `schema-v1.24.1` unstable meta:

- Agent method names match.
- Local unstable client meta still has `mcp/connect` and `mcp/disconnect`. Upstream unstable meta does not. The 1.10.0 notes describe MCP-over-ACP becoming request-scoped. `ClientExperimental` still
has `UnstableConnectMcp` and `UnstableDisconnectMcp`.
- Both sides have `mcp/message`, the document `did*` notifications, NES, providers, `session/fork`, and `$/cancel_request`.

Unstable `$defs`: 255 local, 276 upstream. Names only local include `ConnectMcpRequest`, `ConnectMcpResponse`, `DisconnectMcpRequest`, `DisconnectMcpResponse`, and several elicitation and auth helper
names that upstream renamed or folded. Names only upstream include compaction, subagents, session notices, and session-message types (`CompactionUpdate`, `SubagentUpdate`, `Notice`, `SessionMessage`,
and their companions).

Forty-three shared unstable definitions differ in property keys. The renames that change the wire, not just `_meta`:

| Definition | Local unstable | schema-v1.24.1 unstable |
| :--- | :--- | :--- |
| `DisableProviderRequest`, `ProviderInfo`, `SetProviderRequest` | required `id` | required `providerId` |
| `McpServerAcp` | required `id` | required `serverId` |
| `MessageMcpRequest`, `MessageMcpNotification` | required `connectionId` | required `requestId` and `serverId` |
| `PlanFile`, `PlanItems`, `PlanMarkdown`, `PlanRemoved` | required `id` | required `planId` |

`PromptRequest` and `PromptResponse` locally have `messageId` and `userMessageId` on the unstable schema. Upstream unstable does not. Those two fields are local-only.

`SessionUpdate` on upstream unstable adds, beyond the local unstable list: `notice`, `compaction_update`, `compaction_summary_chunk`, `subagent_update`, `session_message`, `session_message_chunk`.

Unstable parity is behind in both directions. Regenerating from `schema-v1.24.1` would drop `mcp/connect` and `mcp/disconnect` and rename the fields above. Callers of `ClientExperimental` and the
unstable provider, plan, and MCP types would stop compiling.

### Draft v2, out of scope for a stable-parity claim

`schema-v2.0.0-alpha.7` stable meta (`version: 2`) uses `auth/login`, `auth/logout`, `initialize`, `session/cancel`, `session/close`, `session/delete`, `session/list`, `session/new`, `session/prompt`,
`session/resume`, `session/set_config_option`, `elicitation/create`, `elicitation/complete`, `session/request_permission`, `session/update`, and `$/cancel_request`.

Relative to v1, the migration guide removes `session/load`, `session/set_mode`, `fs/read_text_file`, `fs/write_text_file`, and all `terminal/*` execution methods. `authenticate` and `logout` move
under `auth/`. `session/prompt` acknowledges insertion and no longer carries the stop reason; foreground completion moves to a `state_update` notification. Tool calls collapse to upserts. Diffs gain a
structured `changes` list. Capability flags become objects. `info` becomes required.

This SDK negotiates protocol version 1 and implements the v1 filesystem and terminal methods on `Client`. It has no `protocolVersion: 2` surface. The draft text says v1 peers remain the common case
and that dropping v1 cuts an agent off from existing clients. Matching the v2 draft is a second protocol implementation beside v1. It is not required for compliance with the approved v1 spec, and it
is not a byproduct of the Go floor.

## Feasibility, stated as observations

The Go floor and the schema pin move independently.

The floor move is a two-module `go` line, a mise toolchain pin, a formatter and test pass on Go 1.27.1 or newer, and a look at `gopls` and `golangci-lint` pins. The source does not need a mechanical
idiom rewrite to compile. The risks that were identified and not executed are `encoding/json`'s new implementation against `testdata/json_golden`, `gofmt` / `gofumpt` alignment drift under `make
check`, loop-variable semantics in closures that were not enumerated, and unbuffered `time` channels in the example timer. Generic methods, iterators, and `encoding/json/v2` defaults do not unlock the
ACP API.

Schema parity with approved ACP is a regeneration against `schema-v1.24.1`, after the Makefile learns the `schema-v*` tag. The stable gaps that change the Go API, if the generator then places them on
`Agent` and `Client`, are `session/delete` and `elicitation/create` / `elicitation/complete`. The stable gaps that are additive fields are tool-call `name`, content `messageId` (already present,
mislabeled), `usage_update` (already decoded, mislabeled), `model_config`, client capability `session`, and error code `-32800` in the schema union. The unstable schema has wire-level renames and
removed methods. v2 stays a draft.

`make test` and `make check` were not run on Go 1.27.1. No claim here rests on a green test log from that toolchain.
